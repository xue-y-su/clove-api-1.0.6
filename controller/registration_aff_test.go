package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	appi18n "github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/oauth"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/registration_setting"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func setupRegistrationAffControllerTest(t *testing.T) {
	t.Helper()

	require.NoError(t, appi18n.Init())
	db := openTokenControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(
		&model.Option{}, &model.User{}, &model.Checkin{}, &model.Log{}, &model.AuditOutboxEvent{},
		&model.InviteCodeApplication{}, &model.InviteCode{}, &model.InviteCodeUse{}, &model.InviteRewardPayout{},
		&model.InviteRewardPayoutRequest{}, &model.InviteRewardPayoutRequestItem{}, &model.WalletLedgerEntry{},
		&model.InviteRewardPayoutAction{},
		&model.IPSecurityEvent{}, &model.IPAuditRollup{}, &model.IPSecurityProjectionState{},
	))

	previousRegisterEnabled := common.RegisterEnabled
	previousPasswordRegisterEnabled := common.PasswordRegisterEnabled
	previousEmailVerificationEnabled := common.EmailVerificationEnabled
	common.RegisterEnabled = true
	common.PasswordRegisterEnabled = true
	common.EmailVerificationEnabled = false

	common.OptionMapRWMutex.Lock()
	previousOptionMap := common.OptionMap
	common.OptionMap = map[string]string{
		registration_setting.AffRequiredKey:          "false",
		registration_setting.InviteLimitUnlimitedKey: "true",
		registration_setting.InviteLimitCountKey:     "0",
	}
	common.OptionMapRWMutex.Unlock()

	t.Cleanup(func() {
		common.RegisterEnabled = previousRegisterEnabled
		common.PasswordRegisterEnabled = previousPasswordRegisterEnabled
		common.EmailVerificationEnabled = previousEmailVerificationEnabled
		common.OptionMapRWMutex.Lock()
		common.OptionMap = previousOptionMap
		common.OptionMapRWMutex.Unlock()
	})
}

func seedInviterForAffTest(t *testing.T) model.User {
	t.Helper()
	inviter := model.User{
		Id:          1,
		Username:    "inviter",
		Password:    "password",
		DisplayName: "Inviter",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		AffCode:     "abcd",
	}
	require.NoError(t, model.DB.Create(&inviter).Error)
	ownerID := inviter.Id
	require.NoError(t, model.DB.Create(&model.InviteCode{
		OwnerUserID: ownerID, Code: inviter.AffCode, ActiveOwnerSlot: &ownerID,
		Kind: model.InviteCodeKindMulti, MaxUses: 100, Status: model.InviteCodeStatusActive,
		ExpiresAt: time.Now().Add(24 * time.Hour).Unix(), IssuedBy: 1,
	}).Error)
	return inviter
}

func setRegistrationAffOptions(t *testing.T, affRequired string, unlimited string, count string) {
	t.Helper()
	require.NoError(t, model.UpdateOptionsBulk(map[string]string{
		registration_setting.AffRequiredKey:          affRequired,
		registration_setting.InviteLimitUnlimitedKey: unlimited,
		registration_setting.InviteLimitCountKey:     count,
	}))
}

func TestRegisterKeepsAffOptionalByDefault(t *testing.T) {
	setupRegistrationAffControllerTest(t)
	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/user/register", map[string]any{
		"username": "alice",
		"password": "password123",
	}, 0)

	Register(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	var user model.User
	require.NoError(t, model.DB.First(&user, "username = ?", "alice").Error)
	require.Equal(t, 0, user.InviterId)
}

func TestRegisterReturnsFriendlyPasswordLengthError(t *testing.T) {
	setupRegistrationAffControllerTest(t)
	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/user/register", map[string]any{
		"username": "password-too-long",
		"password": "123456789012345678901",
	}, 0)
	ctx.Request.Header.Set("Accept-Language", "en-US")

	Register(ctx)

	response := decodeAPIResponse(t, recorder)
	require.False(t, response.Success)
	require.Equal(t, "Password must be between 8 and 20 characters", response.Message)
	require.NotContains(t, response.Message, "Field validation")
}

func TestRegisterDefaultTokenUsesAutomaticAPIKeyPrefix(t *testing.T) {
	setupRegistrationAffControllerTest(t)
	require.NoError(t, model.DB.AutoMigrate(&model.Token{}))
	previousGenerateDefaultToken := constant.GenerateDefaultToken
	constant.GenerateDefaultToken = true
	t.Cleanup(func() { constant.GenerateDefaultToken = previousGenerateDefaultToken })

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/user/register", map[string]any{
		"username": "default-token-user",
		"password": "password123",
	}, 0)

	Register(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	var user model.User
	require.NoError(t, model.DB.First(&user, "username = ?", "default-token-user").Error)
	var token model.Token
	require.NoError(t, model.DB.First(&token, "user_id = ?", user.Id).Error)
	require.True(t, strings.HasPrefix(token.Key, "sk-"), "generated key %q must use the automatic API key prefix", token.Key)
	require.Len(t, token.Key, 51)
}

func TestRegisterRequiresValidAffWhenEnabled(t *testing.T) {
	setupRegistrationAffControllerTest(t)
	inviter := seedInviterForAffTest(t)
	require.NoError(t, model.UpdateOptionsBulk(map[string]string{
		registration_setting.AffRequiredKey: "true",
	}))

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/user/register", map[string]any{
		"username": "missing",
		"password": "password123",
	}, 0)
	Register(ctx)
	require.False(t, decodeAPIResponse(t, recorder).Success)

	ctx, recorder = newAuthenticatedContext(t, http.MethodPost, "/api/user/register", map[string]any{
		"username": "invalid",
		"password": "password123",
		"aff_code": "nope",
	}, 0)
	Register(ctx)
	require.False(t, decodeAPIResponse(t, recorder).Success)

	ctx, recorder = newAuthenticatedContext(t, http.MethodPost, "/api/user/register", map[string]any{
		"username": "valid",
		"password": "password123",
		"aff_code": "abcd",
	}, 0)
	Register(ctx)
	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)

	var user model.User
	require.NoError(t, model.DB.First(&user, "username = ?", "valid").Error)
	require.Equal(t, inviter.Id, user.InviterId)
}

func TestRegisterWithAffWritesInviteAuditLog(t *testing.T) {
	setupRegistrationAffControllerTest(t)
	inviter := seedInviterForAffTest(t)
	require.NoError(t, model.UpdateOptionsBulk(map[string]string{
		registration_setting.AffRequiredKey: "true",
	}))

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/user/register", map[string]any{
		"username": "invite-audited",
		"password": "password123",
		"aff_code": "abcd",
	}, 0)
	ctx.Request.RemoteAddr = "203.0.113.80:12345"
	Register(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)

	var user model.User
	require.NoError(t, model.DB.First(&user, "username = ?", "invite-audited").Error)
	require.Equal(t, inviter.Id, user.InviterId)
	require.Equal(t, model.InviterSourceInviteCode, user.InviterSource)

	var event model.AuditOutboxEvent
	require.NoError(t, model.DB.First(&event, "event_type = ? AND actor_id = ?", "invitation.code_used", user.Id).Error)
	require.NotContains(t, event.Payload, "abcd")
	delivered, failed, err := service.DeliverPending(context.Background(), 10)
	require.NoError(t, err)
	require.Equal(t, 2, delivered)
	require.Zero(t, failed)
	var auditLog model.Log
	require.NoError(t, model.LOG_DB.First(&auditLog, "event_id = ?", event.EventID).Error)
	require.Contains(t, auditLog.Other, "invitation.code_used")
	require.Contains(t, auditLog.Other, `"inviter_id":1`)
}

func TestOAuthStateOnlyRequiresAffForRegistrationStart(t *testing.T) {
	setupRegistrationAffControllerTest(t)
	seedInviterForAffTest(t)
	require.NoError(t, model.UpdateOptionsBulk(map[string]string{
		registration_setting.AffRequiredKey: "true",
	}))

	router := gin.New()
	router.Use(sessions.Sessions("session", cookie.NewStore([]byte("secret"))))
	router.GET("/api/oauth/state", GenerateOAuthCode)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/oauth/state", nil)
	router.ServeHTTP(recorder, request)
	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success)
	var state string
	require.NoError(t, common.Unmarshal(response.Data, &state))
	require.GreaterOrEqual(t, len(state), 32)

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, "/api/oauth/state?register=true", nil)
	router.ServeHTTP(recorder, request)
	require.False(t, decodeAPIResponse(t, recorder).Success)

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, "/api/oauth/state?register=true&aff=abcd", nil)
	router.ServeHTTP(recorder, request)
	require.True(t, decodeAPIResponse(t, recorder).Success)
}

func TestApprovedInvitationIgnoresLegacyZeroSystemLimit(t *testing.T) {
	setupRegistrationAffControllerTest(t)
	seedInviterForAffTest(t)
	setRegistrationAffOptions(t, "true", "false", "0")

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/user/register", map[string]any{
		"username": "blocked",
		"password": "password123",
		"aff_code": "abcd",
	}, 0)
	Register(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
}

func TestApprovedInvitationUsageDoesNotCountLegacyRelationships(t *testing.T) {
	setupRegistrationAffControllerTest(t)
	inviter := seedInviterForAffTest(t)
	require.NoError(t, model.DB.Create(&model.User{
		Id:        2,
		Username:  "already-invited",
		Password:  "password",
		Role:      common.RoleCommonUser,
		Status:    common.UserStatusEnabled,
		AffCode:   "efgh",
		InviterId: inviter.Id,
	}).Error)
	setRegistrationAffOptions(t, "true", "false", "1")

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/user/register", map[string]any{
		"username": "overlimit",
		"password": "password123",
		"aff_code": "abcd",
	}, 0)
	Register(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
}

func TestApprovedInvitationIgnoresLegacyUserLimitOverride(t *testing.T) {
	setupRegistrationAffControllerTest(t)
	inviter := seedInviterForAffTest(t)
	setRegistrationAffOptions(t, "true", "false", "0")
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", inviter.Id).Updates(map[string]any{
		"invite_limit_mode": "unlimited",
	}).Error)

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/user/register", map[string]any{
		"username": "override-unlimited",
		"password": "password123",
		"aff_code": "abcd",
	}, 0)
	Register(ctx)
	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)

	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", inviter.Id).Updates(map[string]any{
		"invite_limit_mode":  "limited",
		"invite_limit_count": 0,
	}).Error)
	setRegistrationAffOptions(t, "true", "true", "0")

	ctx, recorder = newAuthenticatedContext(t, http.MethodPost, "/api/user/register", map[string]any{
		"username": "override-limited",
		"password": "password123",
		"aff_code": "abcd",
	}, 0)
	Register(ctx)
	response = decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
}

func TestRegisterAllowsAdminInviterPastLimit(t *testing.T) {
	setupRegistrationAffControllerTest(t)
	inviter := seedInviterForAffTest(t)
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", inviter.Id).Update("role", common.RoleAdminUser).Error)
	setRegistrationAffOptions(t, "true", "false", "0")

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/user/register", map[string]any{
		"username": "admin-invite",
		"password": "password123",
		"aff_code": "abcd",
	}, 0)
	Register(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
}

func TestApprovedInvitationIgnoresLegacyAdminLimitOverride(t *testing.T) {
	setupRegistrationAffControllerTest(t)
	inviter := seedInviterForAffTest(t)
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", inviter.Id).Updates(map[string]any{
		"role":               common.RoleAdminUser,
		"invite_limit_mode":  model.InviteLimitModeLimited,
		"invite_limit_count": 0,
	}).Error)
	setRegistrationAffOptions(t, "true", "true", "0")

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/user/register", map[string]any{
		"username": "admin-explicit-limit",
		"password": "password123",
		"aff_code": "abcd",
	}, 0)
	Register(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
}

func TestApprovedInvitationUsageDoesNotCountDeletedLegacyRelationships(t *testing.T) {
	setupRegistrationAffControllerTest(t)
	inviter := seedInviterForAffTest(t)
	deleted := model.User{
		Id:        2,
		Username:  "deleted-invited",
		Password:  "password",
		Role:      common.RoleCommonUser,
		Status:    common.UserStatusEnabled,
		AffCode:   "efgh",
		InviterId: inviter.Id,
	}
	require.NoError(t, model.DB.Create(&deleted).Error)
	require.NoError(t, model.DB.Delete(&deleted).Error)
	setRegistrationAffOptions(t, "true", "false", "1")

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/user/register", map[string]any{
		"username": "deleted-still-counts",
		"password": "password123",
		"aff_code": "abcd",
	}, 0)
	Register(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
}

func TestCreateUserConsumesApprovedInvitation(t *testing.T) {
	setupRegistrationAffControllerTest(t)
	seedInviterForAffTest(t)
	setRegistrationAffOptions(t, "true", "false", "0")

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/user/", map[string]any{
		"username": "admin-created",
		"password": "password123",
		"role":     common.RoleCommonUser,
		"aff_code": "abcd",
	}, 1)
	ctx.Request.Header.Set("Accept-Language", "zh-CN")
	ctx.Set("role", common.RoleAdminUser)
	CreateUser(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	var use model.InviteCodeUse
	require.NoError(t, model.DB.Joins("JOIN users ON users.id = invite_code_uses.registered_user_id").
		Where("users.username = ?", "admin-created").First(&use).Error)
}

func TestCreateUserReturnsFriendlyDuplicateUsernameMessage(t *testing.T) {
	setupRegistrationAffControllerTest(t)
	require.NoError(t, model.DB.Create(&model.User{
		Id:          2,
		Username:    "duplicate",
		Password:    "password",
		DisplayName: "duplicate",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		AffCode:     "dupe",
	}).Error)

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/user/", map[string]any{
		"username":        "duplicate",
		"password":        "password123",
		"role":            common.RoleCommonUser,
		"aff_skip_reason": "manual admin creation",
	}, 1)
	ctx.Set("role", common.RoleAdminUser)
	ctx.Request.Header.Set("Accept-Language", "zh-CN")
	CreateUser(ctx)

	response := decodeAPIResponse(t, recorder)
	require.False(t, response.Success)
	require.Contains(t, response.Message, "用户名已存在")
	require.NotContains(t, response.Message, "Duplicate entry")
}

func TestCreateUserReturnsFriendlyUsernameLengthMessage(t *testing.T) {
	setupRegistrationAffControllerTest(t)

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/user/", map[string]any{
		"username": "username-is-too-long-21",
		"password": "password123",
		"role":     common.RoleCommonUser,
	}, 1)
	ctx.Set("role", common.RoleAdminUser)
	ctx.Request.Header.Set("Accept-Language", "zh-CN")
	CreateUser(ctx)

	response := decodeAPIResponse(t, recorder)
	require.False(t, response.Success)
	require.Contains(t, response.Message, "用户名最多 20 个字符")
	require.NotContains(t, response.Message, "User.Username")
	require.NotContains(t, response.Message, "failed on the 'max' tag")
}

func TestCreateUserPersistsAffSkipReason(t *testing.T) {
	setupRegistrationAffControllerTest(t)
	setRegistrationAffOptions(t, "true", "true", "0")

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/user/", map[string]any{
		"username":        "skip-reason-user",
		"password":        "password123",
		"role":            common.RoleCommonUser,
		"aff_skip_reason": "manual enterprise import",
	}, 1)
	ctx.Set("username", "admin")
	ctx.Set("role", common.RoleAdminUser)
	CreateUser(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)

	var user model.User
	require.NoError(t, model.DB.First(&user, "username = ?", "skip-reason-user").Error)
	require.Equal(t, "manual enterprise import", user.AffSkipReason)
	require.Equal(t, 1, user.AffSkipOperatorId)
	require.NotZero(t, user.AffSkipAt)

	var auditLog model.Log
	require.NoError(t, model.LOG_DB.First(&auditLog, "user_id = ? AND type = ?", 1, model.LogTypeManage).Error)
	require.Contains(t, auditLog.Other, "user.create.invite_skip")
	require.Contains(t, auditLog.Other, "manual enterprise import")
}

type fakeRegistrationOAuthProvider struct {
	userID string
}

func (p fakeRegistrationOAuthProvider) GetName() string {
	return "Fake Registration OAuth"
}

func (p fakeRegistrationOAuthProvider) IsEnabled() bool {
	return true
}

func (p fakeRegistrationOAuthProvider) ExchangeToken(context.Context, string, *gin.Context) (*oauth.OAuthToken, error) {
	return &oauth.OAuthToken{AccessToken: "token"}, nil
}

func (p fakeRegistrationOAuthProvider) GetUserInfo(context.Context, *oauth.OAuthToken) (*oauth.OAuthUser, error) {
	return &oauth.OAuthUser{
		ProviderUserID: p.userID,
		Username:       "oauth-capped",
		DisplayName:    "OAuth Capped",
	}, nil
}

func (p fakeRegistrationOAuthProvider) IsUserIDTaken(string) bool {
	return false
}

func (p fakeRegistrationOAuthProvider) FillUserByProviderID(*model.User, string) error {
	return nil
}

func (p fakeRegistrationOAuthProvider) SetProviderUserID(user *model.User, providerUserID string) {
	user.GitHubId = providerUserID
}

func (p fakeRegistrationOAuthProvider) GetProviderPrefix() string {
	return "fake_"
}

func TestOAuthRegistrationConsumesApprovedInvitation(t *testing.T) {
	setupRegistrationAffControllerTest(t)
	seedInviterForAffTest(t)
	setRegistrationAffOptions(t, "true", "false", "0")
	providerName := "fake-register-limit"
	oauth.Register(providerName, fakeRegistrationOAuthProvider{userID: "oauth-capped-1"})
	t.Cleanup(func() {
		oauth.Unregister(providerName)
	})

	router := gin.New()
	router.Use(sessions.Sessions("session", cookie.NewStore([]byte("secret"))))
	router.GET("/seed-session", func(c *gin.Context) {
		session := sessions.Default(c)
		session.Set("oauth_state", "state-capped")
		session.Set("aff", "abcd")
		require.NoError(t, session.Save())
		c.Status(http.StatusNoContent)
	})
	router.GET("/api/oauth/:provider", HandleOAuth)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/seed-session", nil)
	router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusNoContent, recorder.Code)
	cookieHeader := recorder.Header().Get("Set-Cookie")
	require.NotEmpty(t, cookieHeader)

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, "/api/oauth/"+providerName+"?state=state-capped&code=code-1", nil)
	request.Header.Set("Cookie", cookieHeader)
	router.ServeHTTP(recorder, request)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	var use model.InviteCodeUse
	require.NoError(t, model.DB.Joins("JOIN users ON users.id = invite_code_uses.registered_user_id").
		Where("users.username = ?", "oauth-capped").First(&use).Error)
}

func TestUpdateUserPersistsInviteLimitOverride(t *testing.T) {
	setupRegistrationAffControllerTest(t)
	require.NoError(t, model.DB.Create(&model.User{
		Id:          2,
		Username:    "managed-user",
		Password:    "password",
		DisplayName: "Managed",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		Group:       "default",
		AffCode:     "mngd",
	}).Error)

	ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/user/", map[string]any{
		"id":                 2,
		"username":           "managed-user",
		"display_name":       "Managed User",
		"role":               common.RoleCommonUser,
		"group":              "default",
		"invite_limit_mode":  "limited",
		"invite_limit_count": 2,
	}, 1)
	ctx.Set("role", common.RoleAdminUser)
	UpdateUser(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)

	var updated model.User
	require.NoError(t, model.DB.First(&updated, 2).Error)
	require.Equal(t, model.InviteLimitModeLimited, updated.InviteLimitMode)
	require.Equal(t, 2, updated.InviteLimitCount)
}

func TestUpdateUserPreservesInviteLimitOverrideWhenOmitted(t *testing.T) {
	setupRegistrationAffControllerTest(t)
	require.NoError(t, model.DB.Create(&model.User{
		Id:               2,
		Username:         "managed-user",
		Password:         "password",
		DisplayName:      "Managed",
		Role:             common.RoleCommonUser,
		Status:           common.UserStatusEnabled,
		Group:            "default",
		AffCode:          "mngd",
		InviteLimitMode:  model.InviteLimitModeLimited,
		InviteLimitCount: 4,
	}).Error)

	ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/user/", map[string]any{
		"id":           2,
		"username":     "managed-user",
		"display_name": "Managed User",
		"role":         common.RoleCommonUser,
		"group":        "default",
	}, 1)
	ctx.Set("role", common.RoleAdminUser)
	UpdateUser(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)

	var updated model.User
	require.NoError(t, model.DB.First(&updated, 2).Error)
	require.Equal(t, model.InviteLimitModeLimited, updated.InviteLimitMode)
	require.Equal(t, 4, updated.InviteLimitCount)
}

func TestGetSelfReturnsInviteLimitStatus(t *testing.T) {
	setupRegistrationAffControllerTest(t)
	inviter := seedInviterForAffTest(t)
	require.NoError(t, model.DB.Create(&model.User{
		Id:        2,
		Username:  "invitee-one",
		Password:  "password",
		Role:      common.RoleCommonUser,
		Status:    common.UserStatusEnabled,
		AffCode:   "efgh",
		InviterId: inviter.Id,
	}).Error)
	require.NoError(t, model.DB.Create(&model.User{
		Id:        3,
		Username:  "invitee-two",
		Password:  "password",
		Role:      common.RoleCommonUser,
		Status:    common.UserStatusEnabled,
		AffCode:   "ijkl",
		InviterId: inviter.Id,
	}).Error)
	setRegistrationAffOptions(t, "false", "false", "3")

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/user/self", nil, inviter.Id)
	ctx.Set("role", common.RoleCommonUser)
	GetSelf(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	var data map[string]any
	require.NoError(t, common.Unmarshal(response.Data, &data))
	inviteLimit, ok := data["invite_limit"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, false, inviteLimit["unlimited"])
	require.Equal(t, false, inviteLimit["exempt"])
	require.Equal(t, float64(3), inviteLimit["limit"])
	require.Equal(t, float64(2), inviteLimit["used"])
	require.Equal(t, float64(1), inviteLimit["remaining"])
	require.Equal(t, model.InviteLimitModeInherit, inviteLimit["mode"])
}

func TestGetSelfDoesNotExposeInviterPrivateFields(t *testing.T) {
	setupRegistrationAffControllerTest(t)
	inviter := seedInviterForAffTest(t)
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", inviter.Id).Update("email", "private-inviter@example.com").Error)
	require.NoError(t, model.DB.Create(&model.User{
		Id:        12,
		Username:  "invited-profile-user",
		Password:  "password",
		Role:      common.RoleCommonUser,
		Status:    common.UserStatusEnabled,
		Group:     "default",
		AffCode:   "prof",
		InviterId: inviter.Id,
	}).Error)

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/user/self", nil, 12)
	ctx.Set("role", common.RoleCommonUser)
	GetSelf(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	var data map[string]any
	require.NoError(t, common.Unmarshal(response.Data, &data))
	inviterInfo, ok := data["inviter_info"].(map[string]any)
	require.True(t, ok)
	require.NotContains(t, inviterInfo, "id")
	require.NotContains(t, inviterInfo, "email")
	require.NotContains(t, inviterInfo, "aff_code")
}
