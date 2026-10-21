package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/stretchr/testify/require"
)

func TestTokenRevealWindowExpiresAtThirtyMinutes(t *testing.T) {
	verifiedAt := time.Unix(1_000, 0)
	require.True(t, TokenRevealWindowValid(verifiedAt, verifiedAt.Add(30*time.Minute-time.Second)))
	require.False(t, TokenRevealWindowValid(verifiedAt, verifiedAt.Add(30*time.Minute)))
	require.False(t, TokenRevealWindowValid(verifiedAt, verifiedAt.Add(-time.Second)))
}

func TestTokenRevealRequiredRejectsMissingWindow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(sessions.Sessions("session", cookie.NewStore([]byte("token-reveal-middleware-test"))))
	router.Use(func(c *gin.Context) {
		c.Set("id", 7)
		c.Next()
	})
	router.GET("/protected", TokenRevealRequired(), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/protected", nil)
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusForbidden, recorder.Code)
	require.Contains(t, recorder.Body.String(), "TOKEN_REVEAL_VERIFICATION_REQUIRED")
}
