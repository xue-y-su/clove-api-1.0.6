package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"
	appi18n "github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/oauth"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type fakeBindingOAuthProvider struct{}

func (fakeBindingOAuthProvider) GetName() string {
	return "Fake OAuth"
}

func (fakeBindingOAuthProvider) IsEnabled() bool {
	return true
}

func (fakeBindingOAuthProvider) ExchangeToken(context.Context, string, *gin.Context) (*oauth.OAuthToken, error) {
	return &oauth.OAuthToken{AccessToken: "token"}, nil
}

func (fakeBindingOAuthProvider) GetUserInfo(context.Context, *oauth.OAuthToken) (*oauth.OAuthUser, error) {
	return &oauth.OAuthUser{ProviderUserID: "provider-user-1"}, nil
}

func (fakeBindingOAuthProvider) IsUserIDTaken(string) bool {
	return false
}

func (fakeBindingOAuthProvider) FillUserByProviderID(*model.User, string) error {
	return nil
}

func (fakeBindingOAuthProvider) SetProviderUserID(user *model.User, providerUserID string) {
	user.GitHubId = providerUserID
}

func (fakeBindingOAuthProvider) GetProviderPrefix() string {
	return "fake_"
}

func setupOAuthBindingControllerTest(t *testing.T) {
	t.Helper()

	require.NoError(t, appi18n.Init())
	db := openTokenControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(
		&model.User{}, &model.UserOAuthBinding{}, &model.PasskeyCredential{}, &model.Log{}, &model.AuditOutboxEvent{},
	))
}

func TestOAuthBindCallbackReturnsBindActionWithoutRefreshingLogin(t *testing.T) {
	setupOAuthBindingControllerTest(t)
	oauth.Register("fake-bind", fakeBindingOAuthProvider{})
	t.Cleanup(func() {
		oauth.Unregister("fake-bind")
	})
	require.NoError(t, model.DB.Create(&model.User{
		Id:          1,
		Username:    "alice",
		DisplayName: "Alice",
		Password:    "",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		AffCode:     "abcd",
		LastLoginAt: 0,
	}).Error)
	var beforeBind model.User
	require.NoError(t, model.DB.First(&beforeBind, 1).Error)

	router := gin.New()
	router.Use(sessions.Sessions("session", cookie.NewStore([]byte("secret"))))
	router.GET("/seed-session", func(c *gin.Context) {
		session := sessions.Default(c)
		session.Set("id", 1)
		session.Set("username", "alice")
		session.Set("oauth_state", "state-1")
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
	request = httptest.NewRequest(http.MethodGet, "/api/oauth/fake-bind?state=state-1&code=code-1", nil)
	request.Header.Set("Cookie", cookieHeader)
	router.ServeHTTP(recorder, request)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	require.Equal(t, "bind", response.Message)
	require.JSONEq(t, `{"action":"bind"}`, string(response.Data))

	var user model.User
	require.NoError(t, model.DB.First(&user, 1).Error)
	require.Equal(t, int64(0), user.LastLoginAt)
	require.Equal(t, "provider-user-1", user.GitHubId)
	require.Greater(t, user.SecurityVersion, beforeBind.SecurityVersion)
}

func TestOAuthCallbackConsumesStateAfterSuccessfulValidation(t *testing.T) {
	setupOAuthBindingControllerTest(t)
	oauth.Register("fake-state-consume", fakeBindingOAuthProvider{})
	t.Cleanup(func() {
		oauth.Unregister("fake-state-consume")
	})
	require.NoError(t, model.DB.Create(&model.User{
		Id:       11,
		Username: "state-user",
		Password: "password-hash",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		AffCode:  "st11",
	}).Error)

	router := gin.New()
	router.Use(sessions.Sessions("session", cookie.NewStore([]byte("secret"))))
	router.GET("/seed-session", func(c *gin.Context) {
		session := sessions.Default(c)
		session.Set("id", 11)
		session.Set("username", "state-user")
		session.Set("oauth_state", "state-consume")
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
	request = httptest.NewRequest(http.MethodGet, "/api/oauth/fake-state-consume?state=state-consume&code=code-1", nil)
	request.Header.Set("Cookie", cookieHeader)
	router.ServeHTTP(recorder, request)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	consumedStateCookie := recorder.Header().Get("Set-Cookie")
	require.NotEmpty(t, consumedStateCookie)

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, "/api/oauth/fake-state-consume?state=state-consume&code=code-2", nil)
	request.Header.Set("Cookie", consumedStateCookie)
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusForbidden, recorder.Code)
	response = decodeAPIResponse(t, recorder)
	require.False(t, response.Success)
}

func TestOAuthBindRejectsMissingSessionUserIdWithoutPanic(t *testing.T) {
	setupOAuthBindingControllerTest(t)
	oauth.Register("fake-missing-session-id", fakeBindingOAuthProvider{})
	t.Cleanup(func() {
		oauth.Unregister("fake-missing-session-id")
	})

	router := gin.New()
	router.Use(sessions.Sessions("session", cookie.NewStore([]byte("secret"))))
	router.GET("/seed-session", func(c *gin.Context) {
		session := sessions.Default(c)
		session.Set("username", "missing-id")
		session.Set("oauth_state", "state-missing-id")
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
	request = httptest.NewRequest(http.MethodGet, "/api/oauth/fake-missing-session-id?state=state-missing-id&code=code-1", nil)
	request.Header.Set("Cookie", cookieHeader)
	require.NotPanics(t, func() {
		router.ServeHTTP(recorder, request)
	})

	response := decodeAPIResponse(t, recorder)
	require.False(t, response.Success)
}

func TestOAuthBindRejectsDisabledSessionUser(t *testing.T) {
	setupOAuthBindingControllerTest(t)
	oauth.Register("fake-disabled-bind", fakeBindingOAuthProvider{})
	t.Cleanup(func() {
		oauth.Unregister("fake-disabled-bind")
	})
	require.NoError(t, model.DB.Create(&model.User{
		Id:       12,
		Username: "disabled-binding-user",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusDisabled,
		AffCode:  "db12",
	}).Error)

	router := gin.New()
	router.Use(sessions.Sessions("session", cookie.NewStore([]byte("secret"))))
	router.GET("/seed-session", func(c *gin.Context) {
		session := sessions.Default(c)
		session.Set("id", 12)
		session.Set("username", "disabled-binding-user")
		session.Set("oauth_state", "state-disabled")
		require.NoError(t, session.Save())
		c.Status(http.StatusNoContent)
	})
	router.GET("/api/oauth/:provider", HandleOAuth)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/seed-session", nil))
	require.Equal(t, http.StatusNoContent, recorder.Code)
	cookieHeader := recorder.Header().Get("Set-Cookie")
	require.NotEmpty(t, cookieHeader)

	recorder = httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/oauth/fake-disabled-bind?state=state-disabled&code=code-1", nil)
	request.Header.Set("Cookie", cookieHeader)
	router.ServeHTTP(recorder, request)

	response := decodeAPIResponse(t, recorder)
	require.False(t, response.Success)
	var user model.User
	require.NoError(t, model.DB.First(&user, 12).Error)
	require.Empty(t, user.GitHubId)
}

func TestAdminClearUserBindingRejectsLastBuiltInLoginMethod(t *testing.T) {
	setupOAuthBindingControllerTest(t)
	require.NoError(t, model.DB.Create(&model.User{
		Id:       2,
		Username: "github-only",
		Password: "",
		GitHubId: "gh-2",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		AffCode:  "abcd",
	}).Error)

	ctx, recorder := newAuthenticatedContext(t, http.MethodDelete, "/api/user/2/bindings/github", nil, 1)
	ctx.Set("role", common.RoleAdminUser)
	ctx.Params = gin.Params{
		{Key: "id", Value: "2"},
		{Key: "binding_type", Value: "github"},
	}

	AdminClearUserBinding(ctx)

	response := decodeAPIResponse(t, recorder)
	require.False(t, response.Success)
	require.Contains(t, response.Message, "last login method")

	var user model.User
	require.NoError(t, model.DB.First(&user, 2).Error)
	require.Equal(t, "gh-2", user.GitHubId)
}

func TestAdminUnbindCustomOAuthRejectsLastLoginMethod(t *testing.T) {
	setupOAuthBindingControllerTest(t)
	require.NoError(t, model.DB.Create(&model.User{
		Id:       3,
		Username: "custom-only",
		Password: "",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		AffCode:  "abcd",
	}).Error)
	require.NoError(t, model.DB.Create(&model.UserOAuthBinding{
		UserId:         3,
		ProviderId:     9,
		ProviderUserId: "custom-3",
	}).Error)

	ctx, recorder := newAuthenticatedContext(t, http.MethodDelete, "/api/user/3/oauth/bindings/9", nil, 1)
	ctx.Set("role", common.RoleAdminUser)
	ctx.Params = gin.Params{
		{Key: "id", Value: "3"},
		{Key: "provider_id", Value: "9"},
	}

	UnbindCustomOAuthByAdmin(ctx)

	response := decodeAPIResponse(t, recorder)
	require.False(t, response.Success)
	require.Contains(t, response.Message, "last login method")

	var count int64
	require.NoError(t, model.DB.Model(&model.UserOAuthBinding{}).Where("user_id = ? AND provider_id = ?", 3, 9).Count(&count).Error)
	require.Equal(t, int64(1), count)
}

func bindingSessionRouter(t *testing.T, user model.User) *gin.Engine {
	t.Helper()
	router := gin.New()
	router.Use(sessions.Sessions("session", cookie.NewStore([]byte("binding-session-secret"))))
	router.GET("/seed-session", func(c *gin.Context) {
		session := sessions.Default(c)
		session.Set("id", user.Id)
		session.Set("username", user.Username)
		session.Set("role", user.Role)
		session.Set("status", user.Status)
		session.Set("group", user.Group)
		session.Set("security_version", user.SecurityVersion)
		require.NoError(t, session.Save())
		c.Status(http.StatusNoContent)
	})
	router.GET("/api/user/self", middleware.UserAuth(), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	return router
}

func performBindingSessionRequest(
	router http.Handler,
	method string,
	target string,
	cookies []*http.Cookie,
	userId int,
) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, target, nil)
	for _, item := range cookies {
		request.AddCookie(item)
	}
	if userId > 0 {
		request.Header.Set("New-Api-User", strconv.Itoa(userId))
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

func responseSessionCookies(previous []*http.Cookie, recorder *httptest.ResponseRecorder) []*http.Cookie {
	if cookies := recorder.Result().Cookies(); len(cookies) > 0 {
		return cookies
	}
	return previous
}

func TestClearSelfBindingRefreshesCurrentSessionSecurityVersion(t *testing.T) {
	setupOAuthBindingControllerTest(t)
	user := model.User{
		Id:       41,
		Username: "self-built-in-unbind",
		Password: "password-hash",
		GitHubId: "github-41",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
		AffCode:  "bind-41",
	}
	require.NoError(t, model.DB.Create(&user).Error)

	router := bindingSessionRouter(t, user)
	router.DELETE("/api/user/self/bindings/:binding_type", func(c *gin.Context) {
		c.Set("id", user.Id)
		ClearSelfBinding(c)
	})

	seedRecorder := performBindingSessionRequest(router, http.MethodGet, "/seed-session", nil, 0)
	require.Equal(t, http.StatusNoContent, seedRecorder.Code)
	cookies := seedRecorder.Result().Cookies()
	require.NotEmpty(t, cookies)

	deleteRecorder := performBindingSessionRequest(router, http.MethodDelete, "/api/user/self/bindings/github", cookies, 0)
	response := decodeAPIResponse(t, deleteRecorder)
	require.True(t, response.Success, response.Message)
	cookies = responseSessionCookies(cookies, deleteRecorder)

	protectedRecorder := performBindingSessionRequest(router, http.MethodGet, "/api/user/self", cookies, user.Id)
	require.Equal(t, http.StatusNoContent, protectedRecorder.Code, protectedRecorder.Body.String())
}

func TestUnbindCustomOAuthRefreshesCurrentSessionSecurityVersion(t *testing.T) {
	setupOAuthBindingControllerTest(t)
	user := model.User{
		Id:       42,
		Username: "self-custom-unbind",
		Password: "password-hash",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
		AffCode:  "bind-42",
	}
	require.NoError(t, model.DB.Create(&user).Error)
	require.NoError(t, model.DB.Create(&model.UserOAuthBinding{UserId: user.Id, ProviderId: 9, ProviderUserId: "custom-42"}).Error)

	router := bindingSessionRouter(t, user)
	router.DELETE("/api/user/self/oauth/bindings/:provider_id", func(c *gin.Context) {
		c.Set("id", user.Id)
		UnbindCustomOAuth(c)
	})

	seedRecorder := performBindingSessionRequest(router, http.MethodGet, "/seed-session", nil, 0)
	require.Equal(t, http.StatusNoContent, seedRecorder.Code)
	cookies := seedRecorder.Result().Cookies()
	require.NotEmpty(t, cookies)

	deleteRecorder := performBindingSessionRequest(router, http.MethodDelete, "/api/user/self/oauth/bindings/9", cookies, 0)
	response := decodeAPIResponse(t, deleteRecorder)
	require.True(t, response.Success, response.Message)
	cookies = responseSessionCookies(cookies, deleteRecorder)

	protectedRecorder := performBindingSessionRequest(router, http.MethodGet, "/api/user/self", cookies, user.Id)
	require.Equal(t, http.StatusNoContent, protectedRecorder.Code, protectedRecorder.Body.String())
}

func TestAdminOAuthUnbindDoesNotReplaceAdministratorSessionVersion(t *testing.T) {
	setupOAuthBindingControllerTest(t)
	admin := model.User{
		Id:       51,
		Username: "binding-admin",
		Password: "password-hash",
		Role:     common.RoleAdminUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
		AffCode:  "bind-51",
	}
	target := model.User{
		Id:       52,
		Username: "binding-target",
		Password: "password-hash",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
		AffCode:  "bind-52",
	}
	require.NoError(t, model.DB.Create(&admin).Error)
	require.NoError(t, model.DB.Create(&target).Error)
	require.NoError(t, model.DB.Create(&model.UserOAuthBinding{UserId: target.Id, ProviderId: 9, ProviderUserId: "custom-52"}).Error)

	router := bindingSessionRouter(t, admin)
	router.DELETE("/api/user/:id/oauth/bindings/:provider_id", func(c *gin.Context) {
		c.Set("id", admin.Id)
		c.Set("username", admin.Username)
		c.Set("role", admin.Role)
		UnbindCustomOAuthByAdmin(c)
	})

	seedRecorder := performBindingSessionRequest(router, http.MethodGet, "/seed-session", nil, 0)
	require.Equal(t, http.StatusNoContent, seedRecorder.Code)
	cookies := seedRecorder.Result().Cookies()
	require.NotEmpty(t, cookies)

	deleteRecorder := performBindingSessionRequest(router, http.MethodDelete, "/api/user/52/oauth/bindings/9", cookies, 0)
	response := decodeAPIResponse(t, deleteRecorder)
	require.True(t, response.Success, response.Message)
	cookies = responseSessionCookies(cookies, deleteRecorder)

	protectedRecorder := performBindingSessionRequest(router, http.MethodGet, "/api/user/self", cookies, admin.Id)
	require.Equal(t, http.StatusNoContent, protectedRecorder.Code, protectedRecorder.Body.String())
}
