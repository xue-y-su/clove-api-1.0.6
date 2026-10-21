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

func performTryUserAuthRequest(t *testing.T, status int) *httptest.ResponseRecorder {
	t.Helper()
	openComplianceMiddlewareTestDB(t)
	require.NoError(t, model.DB.Create(&model.User{
		Id:       11,
		Username: "tester",
		Role:     common.RoleCommonUser,
		Status:   status,
		Group:    "default",
	}).Error)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(sessions.Sessions("session", cookie.NewStore([]byte("try-user-auth-test"))))
	router.GET("/login", func(c *gin.Context) {
		session := sessions.Default(c)
		session.Set("username", "tester")
		session.Set("role", common.RoleCommonUser)
		session.Set("id", 11)
		session.Set("status", status)
		session.Set("group", "default")
		require.NoError(t, session.Save())
		c.Status(http.StatusNoContent)
	})
	router.GET("/optional", TryUserAuth(), func(c *gin.Context) {
		_, exists := c.Get("id")
		c.JSON(http.StatusOK, gin.H{"authenticated": exists})
	})

	loginRecorder := httptest.NewRecorder()
	router.ServeHTTP(loginRecorder, httptest.NewRequest(http.MethodGet, "/login", nil))
	require.Equal(t, http.StatusNoContent, loginRecorder.Code)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/optional", nil)
	for _, cookie := range loginRecorder.Result().Cookies() {
		request.AddCookie(cookie)
	}
	router.ServeHTTP(recorder, request)
	return recorder
}

func TestTryUserAuthIgnoresDisabledSession(t *testing.T) {
	recorder := performTryUserAuthRequest(t, common.UserStatusDisabled)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.JSONEq(t, `{"authenticated":false}`, recorder.Body.String())
}

func TestTryUserAuthAcceptsEnabledSession(t *testing.T) {
	recorder := performTryUserAuthRequest(t, common.UserStatusEnabled)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.JSONEq(t, `{"authenticated":true}`, recorder.Body.String())
}
