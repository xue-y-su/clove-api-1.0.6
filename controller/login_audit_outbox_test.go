package controller

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	appi18n "github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/oauth"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type failingLoginAuditOAuthProvider struct{}

func (failingLoginAuditOAuthProvider) GetName() string { return "Audit OAuth" }

func (failingLoginAuditOAuthProvider) IsEnabled() bool { return true }

func (failingLoginAuditOAuthProvider) ExchangeToken(context.Context, string, *gin.Context) (*oauth.OAuthToken, error) {
	return nil, errors.New("provider token exchange failed")
}

func (failingLoginAuditOAuthProvider) GetUserInfo(context.Context, *oauth.OAuthToken) (*oauth.OAuthUser, error) {
	return nil, errors.New("unexpected user info request")
}

func (failingLoginAuditOAuthProvider) IsUserIDTaken(string) bool { return false }

func (failingLoginAuditOAuthProvider) FillUserByProviderID(*model.User, string) error { return nil }

func (failingLoginAuditOAuthProvider) SetProviderUserID(*model.User, string) {}

func (failingLoginAuditOAuthProvider) GetProviderPrefix() string { return "audit_" }

func setupLoginAuditRouter(t *testing.T) *gin.Engine {
	t.Helper()
	setupUserRootBindingControllerTest(t)
	require.NoError(t, model.DB.AutoMigrate(&model.TwoFA{}))
	previousPasswordLoginEnabled := common.PasswordLoginEnabled
	common.PasswordLoginEnabled = true
	t.Cleanup(func() { common.PasswordLoginEnabled = previousPasswordLoginEnabled })
	router := gin.New()
	router.Use(sessions.Sessions("session", cookie.NewStore([]byte("login-audit-test-secret"))))
	router.POST("/api/user/login", Login)
	router.POST("/api/user/logout", Logout)
	router.POST("/api/user/login/2fa", Verify2FALogin)
	router.POST("/api/user/passkey/login/finish", PasskeyLoginFinish)
	router.GET("/api/oauth/wechat", WeChatAuth)
	router.GET("/api/oauth/telegram/login", TelegramLogin)
	router.GET("/api/oauth/:provider", HandleOAuth)
	return router
}

func performLoginAuditRequest(t *testing.T, router *gin.Engine, username string, password string) (*httptest.ResponseRecorder, tokenAPIResponse) {
	t.Helper()
	payload, err := common.Marshal(LoginRequest{Username: username, Password: password})
	require.NoError(t, err)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/user/login", bytes.NewReader(payload))
	request.Header.Set("Content-Type", "application/json")
	request.RemoteAddr = "203.0.113.91:4021"
	router.ServeHTTP(recorder, request)
	return recorder, decodeAPIResponse(t, recorder)
}

func requireLoginFailureAudit(t *testing.T, method string, category string, targetUserID int, forbidden ...string) model.AuditOutboxEvent {
	t.Helper()
	var event model.AuditOutboxEvent
	require.NoError(t, model.DB.First(&event, "event_type = ?", "user.login_failed").Error)
	require.Equal(t, "failure", event.Result)
	require.Zero(t, event.ActorID)
	var payload map[string]any
	require.NoError(t, common.UnmarshalJsonStr(event.Payload, &payload))
	require.Equal(t, method, payload["login_method"])
	require.Equal(t, category, payload["failure_category"])
	require.NotEmpty(t, payload["attempt_identifier"])
	if targetUserID > 0 {
		require.EqualValues(t, targetUserID, payload["target_user_id"])
		require.Equal(t, "user:"+strconv.Itoa(targetUserID), event.Target)
	} else {
		_, present := payload["target_user_id"]
		require.False(t, present)
	}
	auditText := strings.ToLower(event.Target + "\n" + event.Payload)
	for _, value := range forbidden {
		require.NotContains(t, auditText, strings.ToLower(value))
	}
	return event
}

func TestLoginFailureEnqueuesRedactedAuditEvent(t *testing.T) {
	router := setupLoginAuditRouter(t)
	passwordHash, err := common.Password2Hash("CorrectPassword123")
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.User{
		Id: 31, Username: "login-sensitive-user", Password: passwordHash, DisplayName: "Login User",
		Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: "default", AffCode: "login-audit-aff",
	}).Error)

	_, response := performLoginAuditRequest(t, router, "login-sensitive-user", "WrongPassword123")
	require.False(t, response.Success)

	event := requireLoginFailureAudit(t, "password", "invalid_credentials", 31,
		"login-sensitive-user", "WrongPassword123")
	require.Equal(t, "203.0.113.91", event.IP)
	require.NotContains(t, strings.ToLower(event.Payload), `"password":`)
}

func TestLoginFailureFailsClosedWhenAuditOutboxCannotBeWritten(t *testing.T) {
	router := setupLoginAuditRouter(t)
	passwordHash, err := common.Password2Hash("CorrectPassword123")
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.User{
		Id: 33, Username: "login-audit-fail-closed", Password: passwordHash, DisplayName: "Login Audit Failure",
		Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: "default", AffCode: "login-audit-fail-closed-aff",
	}).Error)

	auditFailure := errors.New("audit outbox database details must stay private")
	const callbackName = "login_audit_test:fail_outbox"
	require.NoError(t, model.DB.Callback().Create().Before("gorm:create").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Schema != nil && tx.Statement.Schema.Table == "audit_outbox_events" {
			tx.AddError(auditFailure)
		}
	}))
	t.Cleanup(func() { _ = model.DB.Callback().Create().Remove(callbackName) })

	_, response := performLoginAuditRequest(t, router, "login-audit-fail-closed", "WrongPassword123")

	require.False(t, response.Success)
	require.Equal(t, appi18n.Translate(appi18n.LangEn, appi18n.MsgRetryLater), response.Message)
	require.NotContains(t, response.Message, auditFailure.Error())
	require.NotEqual(t, appi18n.Translate(appi18n.LangEn, appi18n.MsgUserUsernameOrPasswordError), response.Message)
}

func TestLoginSuccessEnqueuesAuditBeforeReturningSession(t *testing.T) {
	router := setupLoginAuditRouter(t)
	passwordHash, err := common.Password2Hash("CorrectPassword123")
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.User{
		Id: 32, Username: "login-success-user", Password: passwordHash, DisplayName: "Login Success",
		Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: "default", AffCode: "login-success-aff",
	}).Error)

	recorder, response := performLoginAuditRequest(t, router, "login-success-user", "CorrectPassword123")
	require.True(t, response.Success, response.Message)
	require.NotEmpty(t, recorder.Header().Values("Set-Cookie"))

	var event model.AuditOutboxEvent
	require.NoError(t, model.DB.First(&event, "event_type = ?", "user.login_success").Error)
	require.Equal(t, model.AuditOutboxStatusPending, event.Status)
	require.Equal(t, 32, event.ActorID)
	require.Contains(t, event.Payload, `"login_method":"password"`)
}

func TestLogoutEnqueuesAuditWithSourceIP(t *testing.T) {
	router := setupLoginAuditRouter(t)
	router.GET("/seed-logout-session", func(c *gin.Context) {
		session := sessions.Default(c)
		session.Set("id", 32)
		session.Set("username", "logout-user")
		require.NoError(t, session.Save())
		c.Status(http.StatusNoContent)
	})
	seedRecorder := httptest.NewRecorder()
	router.ServeHTTP(seedRecorder, httptest.NewRequest(http.MethodGet, "/seed-logout-session", nil))
	request := httptest.NewRequest(http.MethodPost, "/api/user/logout", nil)
	request.Header.Set("Cookie", seedRecorder.Header().Get("Set-Cookie"))
	request.RemoteAddr = "203.0.113.97:4027"
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	require.True(t, decodeAPIResponse(t, recorder).Success)
	var event model.AuditOutboxEvent
	require.NoError(t, model.DB.First(&event, "event_type = ?", "user.logout").Error)
	require.Equal(t, 32, event.ActorID)
	require.Equal(t, "user:32", event.Target)
	require.Equal(t, "203.0.113.97", event.IP)
}

func TestLoginEmptyCredentialsStillEnqueueRedactedFailure(t *testing.T) {
	router := setupLoginAuditRouter(t)

	_, response := performLoginAuditRequest(t, router, "attempted-user", "")
	require.False(t, response.Success)

	var event model.AuditOutboxEvent
	require.NoError(t, model.DB.First(&event, "event_type = ?", "user.login_failed").Error)
	require.Contains(t, event.Payload, "invalid_request")
	require.NotContains(t, event.Payload, "attempted-user")
}

func TestDisabledPasswordLoginEnqueuesMethodDisabledAudit(t *testing.T) {
	router := setupLoginAuditRouter(t)
	common.PasswordLoginEnabled = false

	_, response := performLoginAuditRequest(t, router, "disabled-method-user", "SensitivePassword123")
	require.False(t, response.Success)

	requireLoginFailureAudit(t, "password", "method_disabled", 0,
		"disabled-method-user", "SensitivePassword123")
}

func TestTwoFALoginFailureEnqueuesRedactedAuditForKnownUser(t *testing.T) {
	router := setupLoginAuditRouter(t)
	passwordHash, err := common.Password2Hash("CorrectPassword123")
	require.NoError(t, err)
	require.NoError(t, model.DB.Create(&model.User{
		Id: 41, Username: "twofa-sensitive-user", Password: passwordHash, DisplayName: "2FA User",
		Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: "default", AffCode: "twofa-audit-aff",
	}).Error)
	require.NoError(t, model.DB.Create(&model.TwoFA{
		UserId: 41, Secret: "JBSWY3DPEHPK3PXP", IsEnabled: true,
	}).Error)

	loginRecorder, loginResponse := performLoginAuditRequest(t, router, "twofa-sensitive-user", "CorrectPassword123")
	require.True(t, loginResponse.Success, loginResponse.Message)
	cookieHeader := loginRecorder.Header().Get("Set-Cookie")
	require.NotEmpty(t, cookieHeader)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/user/login/2fa", strings.NewReader(`{"code":"twofa-secret-code"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Cookie", cookieHeader)
	request.RemoteAddr = "203.0.113.92:4022"
	router.ServeHTTP(recorder, request)
	require.False(t, decodeAPIResponse(t, recorder).Success)

	event := requireLoginFailureAudit(t, "2fa", "invalid_code", 41,
		"twofa-sensitive-user", "twofa-secret-code")
	require.Equal(t, "203.0.113.92", event.IP)
}

func TestPasskeyLoginFailureEnqueuesRedactedAuditForUnknownUser(t *testing.T) {
	router := setupLoginAuditRouter(t)
	settings := system_setting.GetPasskeySettings()
	previousSettings := *settings
	*settings = system_setting.PasskeySettings{
		Enabled: true, RPDisplayName: "Login Audit", RPID: "example.com", Origins: "https://example.com",
	}
	t.Cleanup(func() { *settings = previousSettings })

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "https://example.com/api/user/passkey/login/finish", strings.NewReader(`{"credential":"passkey-provider-secret"}`))
	request.Header.Set("Content-Type", "application/json")
	request.RemoteAddr = "203.0.113.93:4023"
	router.ServeHTTP(recorder, request)
	require.False(t, decodeAPIResponse(t, recorder).Success)

	event := requireLoginFailureAudit(t, "passkey", "session_missing", 0, "passkey-provider-secret")
	require.Equal(t, "203.0.113.93", event.IP)
}

func TestOAuthLoginFailureEnqueuesRedactedAudit(t *testing.T) {
	const providerName = "audit-failure"
	oauth.Register(providerName, failingLoginAuditOAuthProvider{})
	t.Cleanup(func() { oauth.Unregister(providerName) })
	router := setupLoginAuditRouter(t)
	router.GET("/seed-oauth-audit-state", func(c *gin.Context) {
		session := sessions.Default(c)
		session.Set("oauth_state", "audit-state")
		require.NoError(t, session.Save())
		c.Status(http.StatusNoContent)
	})

	seedRecorder := httptest.NewRecorder()
	router.ServeHTTP(seedRecorder, httptest.NewRequest(http.MethodGet, "/seed-oauth-audit-state", nil))
	require.Equal(t, http.StatusNoContent, seedRecorder.Code)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/oauth/"+providerName+"?state=audit-state&code=oauth-provider-secret-code", nil)
	request.Header.Set("Cookie", seedRecorder.Header().Get("Set-Cookie"))
	request.RemoteAddr = "203.0.113.94:4024"
	router.ServeHTTP(recorder, request)
	require.False(t, decodeAPIResponse(t, recorder).Success)

	event := requireLoginFailureAudit(t, "oauth:"+providerName, "token_exchange_failed", 0,
		"oauth-provider-secret-code", "provider token exchange failed")
	require.Equal(t, "203.0.113.94", event.IP)
}

func TestWeChatLoginFailureEnqueuesRedactedAudit(t *testing.T) {
	previousEnabled := common.WeChatAuthEnabled
	previousAddress := common.WeChatServerAddress
	previousToken := common.WeChatServerToken
	common.WeChatAuthEnabled = true
	common.WeChatServerToken = "wechat-provider-token"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":false,"message":"wechat provider rejected code","data":""}`))
	}))
	common.WeChatServerAddress = server.URL
	t.Cleanup(func() {
		server.Close()
		common.WeChatAuthEnabled = previousEnabled
		common.WeChatServerAddress = previousAddress
		common.WeChatServerToken = previousToken
	})
	router := setupLoginAuditRouter(t)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/oauth/wechat?code=wechat-provider-secret-code", nil)
	request.RemoteAddr = "203.0.113.95:4025"
	router.ServeHTTP(recorder, request)
	require.False(t, decodeAPIResponse(t, recorder).Success)

	event := requireLoginFailureAudit(t, "wechat", "provider_verification_failed", 0,
		"wechat-provider-secret-code", "wechat-provider-token", "wechat provider rejected code")
	require.Equal(t, "203.0.113.95", event.IP)
}

func TestTelegramLoginFailureEnqueuesRedactedAudit(t *testing.T) {
	previousEnabled := common.TelegramOAuthEnabled
	previousToken := common.TelegramBotToken
	common.TelegramOAuthEnabled = true
	common.TelegramBotToken = "telegram-provider-token"
	t.Cleanup(func() {
		common.TelegramOAuthEnabled = previousEnabled
		common.TelegramBotToken = previousToken
	})
	router := setupLoginAuditRouter(t)

	recorder := httptest.NewRecorder()
	authDate := strconv.FormatInt(time.Now().Unix(), 10)
	request := httptest.NewRequest(http.MethodGet, "/api/oauth/telegram/login?id=telegram-sensitive-id&auth_date="+authDate+"&hash=telegram-provider-secret-hash", nil)
	request.RemoteAddr = "203.0.113.96:4026"
	router.ServeHTTP(recorder, request)
	require.False(t, decodeAPIResponse(t, recorder).Success)

	event := requireLoginFailureAudit(t, "telegram", "invalid_authorization", 0,
		"telegram-sensitive-id", "telegram-provider-token", "telegram-provider-secret-hash")
	require.Equal(t, "203.0.113.96", event.IP)
}
