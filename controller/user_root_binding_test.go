package controller

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/common"
	appi18n "github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/service/authz"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupUserRootBindingControllerTest(t *testing.T) {
	t.Helper()

	require.NoError(t, appi18n.Init())
	db := openTokenControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(
		&model.User{}, &model.Checkin{}, &model.Log{}, &model.AuditOutboxEvent{}, &model.CasbinRule{}, &model.AuthzRole{},
		&model.UserOAuthBinding{}, &model.PasskeyCredential{},
		&model.IPSecurityEvent{}, &model.IPAuditRollup{}, &model.IPSecurityProjectionState{},
		&model.WalletLedgerEntry{},
	))
	previousMaster := common.IsMasterNode
	common.IsMasterNode = true
	t.Cleanup(func() {
		common.IsMasterNode = previousMaster
	})
	require.NoError(t, authz.Init(db))
}

func backfillIPSecurityProjectionForControllerTest(t *testing.T) {
	t.Helper()
	require.NoError(t, model.BackfillIPSecurityProjection(context.Background(), 50))
}

func seedUserRootBindingUser(t *testing.T, id int, username string, role int) {
	t.Helper()
	require.NoError(t, model.DB.Create(&model.User{
		Id:          id,
		Username:    username,
		Password:    "password-hash",
		DisplayName: username,
		Role:        role,
		Status:      common.UserStatusEnabled,
		Group:       "default",
		AffCode:     username + "-aff",
	}).Error)
}

func TestManageUserRootRoleChangesRequirePrimaryUser(t *testing.T) {
	setupUserRootBindingControllerTest(t)
	seedUserRootBindingUser(t, 1, "primary-root", common.RoleRootUser)
	seedUserRootBindingUser(t, 2, "secondary-root", common.RoleRootUser)
	seedUserRootBindingUser(t, 3, "target-user", common.RoleAdminUser)

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/user/manage", map[string]any{
		"id":     3,
		"action": "promote_root",
	}, 2)
	ctx.Set("username", "secondary-root")
	ctx.Set("role", common.RoleRootUser)

	ManageUser(ctx)

	response := decodeAPIResponse(t, recorder)
	require.False(t, response.Success)

	var target model.User
	require.NoError(t, model.DB.First(&target, 3).Error)
	require.Equal(t, common.RoleAdminUser, target.Role)
}

func TestSecondaryRootCannotReadPeerRoot(t *testing.T) {
	setupUserRootBindingControllerTest(t)
	seedUserRootBindingUser(t, 1, "primary-root", common.RoleRootUser)
	seedUserRootBindingUser(t, 2, "secondary-root", common.RoleRootUser)
	seedUserRootBindingUser(t, 3, "peer-root", common.RoleRootUser)

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/user/3", nil, 2)
	ctx.Set("username", "secondary-root")
	ctx.Set("role", common.RoleRootUser)
	ctx.Params = gin.Params{{Key: "id", Value: "3"}}

	GetUser(ctx)

	response := decodeAPIResponse(t, recorder)
	require.False(t, response.Success)
}

func TestPrimaryUserCanPromoteAndDemoteRootRole(t *testing.T) {
	setupUserRootBindingControllerTest(t)
	seedUserRootBindingUser(t, 1, "primary-root", common.RoleRootUser)
	seedUserRootBindingUser(t, 4, "target-user", common.RoleAdminUser)

	promoteCtx, promoteRecorder := newAuthenticatedContext(t, http.MethodPost, "/api/user/manage", map[string]any{
		"id":     4,
		"action": "promote_root",
	}, 1)
	promoteCtx.Set("username", "primary-root")
	promoteCtx.Set("role", common.RoleRootUser)

	ManageUser(promoteCtx)

	promoteResponse := decodeAPIResponse(t, promoteRecorder)
	require.True(t, promoteResponse.Success, promoteResponse.Message)

	var target model.User
	require.NoError(t, model.DB.First(&target, 4).Error)
	require.Equal(t, common.RoleRootUser, target.Role)

	demoteCtx, demoteRecorder := newAuthenticatedContext(t, http.MethodPost, "/api/user/manage", map[string]any{
		"id":     4,
		"action": "demote_root",
	}, 1)
	demoteCtx.Set("username", "primary-root")
	demoteCtx.Set("role", common.RoleRootUser)

	ManageUser(demoteCtx)

	demoteResponse := decodeAPIResponse(t, demoteRecorder)
	require.True(t, demoteResponse.Success, demoteResponse.Message)
	require.NoError(t, model.DB.First(&target, 4).Error)
	require.Equal(t, common.RoleAdminUser, target.Role)
}

func TestUpdateUserCanEditBuiltInOAuthBindings(t *testing.T) {
	setupUserRootBindingControllerTest(t)
	seedUserRootBindingUser(t, 1, "primary-root", common.RoleRootUser)
	seedUserRootBindingUser(t, 5, "binding-user", common.RoleCommonUser)

	ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/user/", map[string]any{
		"id":           5,
		"username":     "binding-user",
		"display_name": "Binding User",
		"role":         common.RoleCommonUser,
		"group":        "default",
		"github_id":    "github-5",
		"discord_id":   "discord-5",
		"oidc_id":      "oidc-5",
		"wechat_id":    "wechat-5",
		"telegram_id":  "telegram-5",
		"linux_do_id":  "linuxdo-5",
		"email":        "Binding@Example.com",
	}, 1)
	ctx.Set("username", "primary-root")
	ctx.Set("role", common.RoleRootUser)

	UpdateUser(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)

	var user model.User
	require.NoError(t, model.DB.First(&user, 5).Error)
	require.Equal(t, "github-5", user.GitHubId)
	require.Equal(t, "discord-5", user.DiscordId)
	require.Equal(t, "oidc-5", user.OidcId)
	require.Equal(t, "wechat-5", user.WeChatId)
	require.Equal(t, "telegram-5", user.TelegramId)
	require.Equal(t, "linuxdo-5", user.LinuxDOId)
	require.Equal(t, "binding@example.com", user.Email)
}

func TestUpdateUserRejectsDuplicateBuiltInOAuthBinding(t *testing.T) {
	setupUserRootBindingControllerTest(t)
	seedUserRootBindingUser(t, 1, "primary-root", common.RoleRootUser)
	seedUserRootBindingUser(t, 6, "existing-binding", common.RoleCommonUser)
	seedUserRootBindingUser(t, 7, "target-binding", common.RoleCommonUser)
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", 6).Update("github_id", "github-taken").Error)

	ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/user/", map[string]any{
		"id":           7,
		"username":     "target-binding",
		"display_name": "Target Binding",
		"role":         common.RoleCommonUser,
		"group":        "default",
		"github_id":    "github-taken",
	}, 1)
	ctx.Set("username", "primary-root")
	ctx.Set("role", common.RoleRootUser)

	UpdateUser(ctx)

	response := decodeAPIResponse(t, recorder)
	require.False(t, response.Success)
	require.Contains(t, response.Message, "GitHub")
}

func TestPrimaryUserCannotDemoteSelfFromRoot(t *testing.T) {
	setupUserRootBindingControllerTest(t)
	seedUserRootBindingUser(t, 1, "primary-root", common.RoleRootUser)

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/user/manage", map[string]any{
		"id":     1,
		"action": "demote_root",
	}, 1)
	ctx.Set("username", "primary-root")
	ctx.Set("role", common.RoleRootUser)

	ManageUser(ctx)

	response := decodeAPIResponse(t, recorder)
	require.False(t, response.Success)

	var primary model.User
	require.NoError(t, model.DB.First(&primary, 1).Error)
	require.Equal(t, common.RoleRootUser, primary.Role)
}

func TestManageUserStoresBanReasonsAndAuditsThem(t *testing.T) {
	setupUserRootBindingControllerTest(t)
	seedUserRootBindingUser(t, 1, "primary-root", common.RoleRootUser)
	seedUserRootBindingUser(t, 8, "target-ban", common.RoleCommonUser)

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/user/manage", map[string]any{
		"id":                  8,
		"action":              "disable",
		"ban_reason_public":   "policy violation",
		"ban_reason_internal": "chargeback investigation",
	}, 1)
	ctx.Set("username", "primary-root")
	ctx.Set("role", common.RoleRootUser)

	ManageUser(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)

	var user model.User
	require.NoError(t, model.DB.First(&user, 8).Error)
	require.Equal(t, common.UserStatusDisabled, user.Status)
	require.Equal(t, "policy violation", user.BanReasonPublic)
	require.Equal(t, "chargeback investigation", user.BanReasonInternal)

	var event model.AuditOutboxEvent
	require.NoError(t, model.DB.First(&event, "event_type = ?", "user.disable").Error)
	require.Equal(t, model.AuditOutboxStatusPending, event.Status)
	require.Contains(t, event.Payload, "policy violation")
	require.Contains(t, event.Payload, "chargeback investigation")

	delivered, failed, err := service.DeliverPending(ctx.Request.Context(), 10)
	require.NoError(t, err)
	require.Equal(t, 1, delivered)
	require.Zero(t, failed)
	var audit model.Log
	require.NoError(t, model.LOG_DB.First(&audit, "event_id = ?", event.EventID).Error)
	require.Contains(t, audit.Other, "user.disable")
	require.Contains(t, audit.Other, "policy violation")
	require.Contains(t, audit.Other, "chargeback investigation")
}

func TestManageUserRollsBackWhenCriticalAuditCannotBeEnqueued(t *testing.T) {
	setupUserRootBindingControllerTest(t)
	seedUserRootBindingUser(t, 1, "primary-root", common.RoleRootUser)
	seedUserRootBindingUser(t, 18, "target-rollback", common.RoleCommonUser)
	failure := errors.New("outbox unavailable")
	require.NoError(t, model.DB.Callback().Create().Before("gorm:create").Register("manage_user_test:fail_outbox", func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Schema != nil && tx.Statement.Schema.Table == "audit_outbox_events" {
			tx.AddError(failure)
		}
	}))
	t.Cleanup(func() { _ = model.DB.Callback().Create().Remove("manage_user_test:fail_outbox") })

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/user/manage", map[string]any{
		"id":                18,
		"action":            "disable",
		"ban_reason_public": "must roll back",
	}, 1)
	ctx.Set("username", "primary-root")
	ctx.Set("role", common.RoleRootUser)

	ManageUser(ctx)

	response := decodeAPIResponse(t, recorder)
	require.False(t, response.Success)
	var user model.User
	require.NoError(t, model.DB.First(&user, 18).Error)
	require.Equal(t, common.UserStatusEnabled, user.Status)
	var count int64
	require.NoError(t, model.DB.Model(&model.AuditOutboxEvent{}).Count(&count).Error)
	require.Zero(t, count)
}

func TestLoginDisabledUserShowsPublicBanReasonOnly(t *testing.T) {
	setupUserRootBindingControllerTest(t)
	previousPasswordLoginEnabled := common.PasswordLoginEnabled
	common.PasswordLoginEnabled = true
	t.Cleanup(func() { common.PasswordLoginEnabled = previousPasswordLoginEnabled })

	password, err := common.Password2Hash("ValidPassword123")
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.User{
		Id:                9,
		Username:          "banned-user",
		Password:          password,
		DisplayName:       "Banned User",
		Role:              common.RoleCommonUser,
		Status:            common.UserStatusDisabled,
		BanReasonPublic:   "terms abuse",
		BanReasonInternal: "risk team note",
		Group:             "default",
		AffCode:           "banned-aff",
	}).Error)

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/user/login", map[string]any{
		"username": "banned-user",
		"password": "ValidPassword123",
	}, 0)

	Login(ctx)

	response := decodeAPIResponse(t, recorder)
	require.False(t, response.Success)
	require.Contains(t, response.Message, "terms abuse")
	require.NotContains(t, response.Message, "risk team note")
}

func TestUpdateUserRenamesUsernameAndPreservesCustomDisplayName(t *testing.T) {
	setupUserRootBindingControllerTest(t)
	seedUserRootBindingUser(t, 1, "primary-root", common.RoleRootUser)
	seedUserRootBindingUser(t, 10, "old-name", common.RoleCommonUser)
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", 10).Update("display_name", "Custom Nick").Error)

	ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/user/", map[string]any{
		"id":           10,
		"username":     "new-name",
		"display_name": "Custom Nick",
		"role":         common.RoleCommonUser,
		"group":        "default",
	}, 1)
	ctx.Set("username", "primary-root")
	ctx.Set("role", common.RoleRootUser)

	UpdateUser(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)

	var user model.User
	require.NoError(t, model.DB.First(&user, 10).Error)
	require.Equal(t, "new-name", user.Username)
	require.Equal(t, "Custom Nick", user.DisplayName)

	var audit model.Log
	require.NoError(t, model.LOG_DB.First(&audit, "type = ?", model.LogTypeManage).Error)
	require.Contains(t, audit.Other, "username_from")
	require.Contains(t, audit.Other, "old-name")
	require.Contains(t, audit.Other, "username_to")
	require.Contains(t, audit.Other, "new-name")
}

func TestUpdateUserRenamesUsernameAndSyncsDefaultDisplayName(t *testing.T) {
	setupUserRootBindingControllerTest(t)
	seedUserRootBindingUser(t, 1, "primary-root", common.RoleRootUser)
	seedUserRootBindingUser(t, 13, "old-default-name", common.RoleCommonUser)

	ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/user/", map[string]any{
		"id":           13,
		"username":     "new-default-name",
		"display_name": "old-default-name",
		"role":         common.RoleCommonUser,
		"group":        "default",
	}, 1)
	ctx.Set("username", "primary-root")
	ctx.Set("role", common.RoleRootUser)

	UpdateUser(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)

	var user model.User
	require.NoError(t, model.DB.First(&user, 13).Error)
	require.Equal(t, "new-default-name", user.Username)
	require.Equal(t, "new-default-name", user.DisplayName)
}

func TestUpdateSelfCannotChangeUsernameButCanUpdateProfileFields(t *testing.T) {
	setupUserRootBindingControllerTest(t)
	setupOwnProfilePolicy(t, "example.com")
	seedUserRootBindingUser(t, 14, "self-user", common.RoleCommonUser)

	ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/user/self", map[string]any{
		"username":     "attempted-rename",
		"display_name": "Self Nick",
		"avatar_url":   "https://example.com/avatar.png",
	}, 14)
	ctx.Set("username", "self-user")
	ctx.Set("role", common.RoleCommonUser)

	UpdateSelf(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)

	var user model.User
	require.NoError(t, model.DB.First(&user, 14).Error)
	require.Equal(t, "self-user", user.Username)
	require.Equal(t, "Self Nick", user.DisplayName)
	require.Equal(t, "https://example.com/avatar.png", user.AvatarURL)

	var audit model.Log
	require.NoError(t, model.LOG_DB.First(&audit, "type = ?", model.LogTypeManage).Error)
	require.Contains(t, audit.Other, "user.profile_update")
	require.Contains(t, audit.Other, "Self Nick")
	require.NotContains(t, audit.Other, "attempted-rename")
}

func TestGetSelfInviterInfoUsesInviteCode(t *testing.T) {
	setupUserRootBindingControllerTest(t)
	seedUserRootBindingUser(t, 11, "inviter", common.RoleCommonUser)
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", 11).Updates(map[string]any{
		"display_name": "Inviter Nick",
		"aff_code":     "INV123",
	}).Error)
	require.NoError(t, model.DB.Create(&model.User{
		Id:          12,
		Username:    "invitee",
		Password:    "password-hash",
		DisplayName: "Invitee",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		Group:       "default",
		AffCode:     "INVITEE",
		InviterId:   11,
	}).Error)

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/user/self", nil, 12)
	ctx.Set("role", common.RoleCommonUser)

	GetSelf(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	var data map[string]interface{}
	require.NoError(t, common.Unmarshal(response.Data, &data))
	require.Equal(t, "default", data["group_name"])
	require.Equal(t, float64(11), data["inviter_user_id"])
	require.Equal(t, "Inviter Nick", data["inviter_display_name"])
	inviterInfo, ok := data["inviter_info"].(map[string]interface{})
	require.True(t, ok)
	require.NotContains(t, inviterInfo, "aff_code")
	require.NotContains(t, inviterInfo, "role")
	require.NotContains(t, inviterInfo, "role_label")
	require.NotContains(t, inviterInfo, "label")
	require.NotContains(t, inviterInfo, "profile_subtitle_item")
	require.Equal(t, "inviter", inviterInfo["username"])
	require.Equal(t, "Inviter Nick", inviterInfo["display_name"])
}
