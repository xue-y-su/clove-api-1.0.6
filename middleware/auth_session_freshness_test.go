package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRootAuthRejectsStaleRootSessionAfterRoleDemotion(t *testing.T) {
	openComplianceMiddlewareTestDB(t)
	withComplianceOptions(t, map[string]string{})
	withSourceAttributionOptions(t, false, false)
	require.NoError(t, model.DB.Create(&model.User{
		Id:       51,
		Username: "demoted-root",
		Role:     common.RoleAdminUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
	}).Error)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(sessions.Sessions("session", cookie.NewStore([]byte("stale-root-session-test"))))
	router.GET("/login", func(c *gin.Context) {
		session := sessions.Default(c)
		session.Set("username", "demoted-root")
		session.Set("role", common.RoleRootUser)
		session.Set("id", 51)
		session.Set("status", common.UserStatusEnabled)
		session.Set("group", "default")
		require.NoError(t, session.Save())
		c.Status(http.StatusNoContent)
	})
	router.GET("/root", RootAuth(), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	loginRecorder := httptest.NewRecorder()
	router.ServeHTTP(loginRecorder, httptest.NewRequest(http.MethodGet, "/login", nil))
	require.Equal(t, http.StatusNoContent, loginRecorder.Code)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/root", nil)
	request.Header.Set("New-Api-User", "51")
	for _, sessionCookie := range loginRecorder.Result().Cookies() {
		request.AddCookie(sessionCookie)
	}
	router.ServeHTTP(recorder, request)

	require.NotEqual(t, http.StatusNoContent, recorder.Code)
}

func TestTokenOrUserAuthRejectsStaleEnabledSessionAfterUserDisabled(t *testing.T) {
	openComplianceMiddlewareTestDB(t)
	withComplianceOptions(t, map[string]string{})
	withSourceAttributionOptions(t, false, false)
	require.NoError(t, model.DB.Create(&model.User{
		Id:       52,
		Username: "disabled-session-user",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusDisabled,
		Group:    "default",
	}).Error)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(sessions.Sessions("session", cookie.NewStore([]byte("stale-enabled-session-test"))))
	router.GET("/login", func(c *gin.Context) {
		session := sessions.Default(c)
		session.Set("username", "disabled-session-user")
		session.Set("role", common.RoleCommonUser)
		session.Set("id", 52)
		session.Set("status", common.UserStatusEnabled)
		session.Set("group", "default")
		require.NoError(t, session.Save())
		c.Status(http.StatusNoContent)
	})
	router.GET("/video", TokenOrUserAuth(), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	loginRecorder := httptest.NewRecorder()
	router.ServeHTTP(loginRecorder, httptest.NewRequest(http.MethodGet, "/login", nil))
	require.Equal(t, http.StatusNoContent, loginRecorder.Code)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/video", nil)
	for _, sessionCookie := range loginRecorder.Result().Cookies() {
		request.AddCookie(sessionCookie)
	}
	router.ServeHTTP(recorder, request)

	require.NotEqual(t, http.StatusNoContent, recorder.Code)
}

func TestCurrentSessionUserRejectsSessionWhenDatabaseIsUnavailable(t *testing.T) {
	previousDB := model.DB
	model.DB = nil
	t.Cleanup(func() {
		model.DB = previousDB
	})

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(sessions.Sessions("session", cookie.NewStore([]byte("missing-db-session-test"))))
	router.GET("/login", func(c *gin.Context) {
		session := sessions.Default(c)
		session.Set("username", "stale-user")
		session.Set("role", common.RoleRootUser)
		session.Set("id", 99)
		session.Set("status", common.UserStatusEnabled)
		require.NoError(t, session.Save())
		c.Status(http.StatusNoContent)
	})
	router.GET("/optional", TryUserAuth(), func(c *gin.Context) {
		_, authenticated := c.Get("id")
		c.JSON(http.StatusOK, gin.H{"authenticated": authenticated})
	})

	loginRecorder := httptest.NewRecorder()
	router.ServeHTTP(loginRecorder, httptest.NewRequest(http.MethodGet, "/login", nil))
	require.Equal(t, http.StatusNoContent, loginRecorder.Code)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/optional", nil)
	for _, sessionCookie := range loginRecorder.Result().Cookies() {
		request.AddCookie(sessionCookie)
	}
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.JSONEq(t, `{"authenticated":false}`, recorder.Body.String())
}

func TestRootAuthRejectsSessionWithStaleSecurityVersion(t *testing.T) {
	openComplianceMiddlewareTestDB(t)
	withComplianceOptions(t, map[string]string{})
	withSourceAttributionOptions(t, false, false)
	require.NoError(t, model.DB.Create(&model.User{
		Id:              53,
		Username:        "versioned-root",
		Role:            common.RoleRootUser,
		Status:          common.UserStatusEnabled,
		SecurityVersion: 2,
		Group:           "default",
	}).Error)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(sessions.Sessions("session", cookie.NewStore([]byte("stale-security-version-test"))))
	router.GET("/login", func(c *gin.Context) {
		session := sessions.Default(c)
		session.Set("username", "versioned-root")
		session.Set("role", common.RoleRootUser)
		session.Set("id", 53)
		session.Set("status", common.UserStatusEnabled)
		session.Set("group", "default")
		session.Set("security_version", int64(1))
		require.NoError(t, session.Save())
		c.Status(http.StatusNoContent)
	})
	router.GET("/root", RootAuth(), func(c *gin.Context) { c.Status(http.StatusNoContent) })

	loginRecorder := httptest.NewRecorder()
	router.ServeHTTP(loginRecorder, httptest.NewRequest(http.MethodGet, "/login", nil))
	require.Equal(t, http.StatusNoContent, loginRecorder.Code)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/root", nil)
	request.Header.Set("New-Api-User", "53")
	for _, sessionCookie := range loginRecorder.Result().Cookies() {
		request.AddCookie(sessionCookie)
	}
	router.ServeHTTP(recorder, request)

	require.NotEqual(t, http.StatusNoContent, recorder.Code)
}

func TestUserAuthRefreshesGroupWithoutExpiringSession(t *testing.T) {
	openComplianceMiddlewareTestDB(t)
	withComplianceOptions(t, map[string]string{})
	withSourceAttributionOptions(t, false, false)
	require.NoError(t, model.DB.Create(&model.User{
		Id:              54,
		Username:        "moved-group-user",
		Role:            common.RoleCommonUser,
		Status:          common.UserStatusEnabled,
		SecurityVersion: 1,
		Group:           "grok",
	}).Error)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(sessions.Sessions("session", cookie.NewStore([]byte("group-refresh-session-test"))))
	router.GET("/login", func(c *gin.Context) {
		session := sessions.Default(c)
		session.Set("username", "moved-group-user")
		session.Set("role", common.RoleCommonUser)
		session.Set("id", 54)
		session.Set("status", common.UserStatusEnabled)
		session.Set("group", "default")
		session.Set("security_version", int64(1))
		require.NoError(t, session.Save())
		c.Status(http.StatusNoContent)
	})
	router.GET("/self", UserAuth(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"group": c.GetString("group")})
	})

	loginRecorder := httptest.NewRecorder()
	router.ServeHTTP(loginRecorder, httptest.NewRequest(http.MethodGet, "/login", nil))
	require.Equal(t, http.StatusNoContent, loginRecorder.Code)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/self", nil)
	request.Header.Set("New-Api-User", "54")
	for _, sessionCookie := range loginRecorder.Result().Cookies() {
		request.AddCookie(sessionCookie)
	}
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.JSONEq(t, `{"group":"grok"}`, recorder.Body.String())
	require.Equal(t, "grok", recorder.Header().Get("Auth-User-Group"))
}
