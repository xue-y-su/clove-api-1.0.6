package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVersionHeaderUsesDisplayVersion(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(Version())
	router.GET("/", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	router.ServeHTTP(recorder, request)

	require.Equal(t, "Clove API v1.0.5-beta", recorder.Header().Get("X-New-Api-Version"))
	require.NotEqual(t, common.Version, recorder.Header().Get("X-New-Api-Version"))
}

func performCORSPreflight(t *testing.T, origin string, requestedHeaders ...string) *httptest.ResponseRecorder {
	return performCORSPreflightForMethod(t, origin, http.MethodGet, requestedHeaders...)
}

func performCORSPreflightForMethod(t *testing.T, origin, method string, requestedHeaders ...string) *httptest.ResponseRecorder {
	t.Helper()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(CORS())
	router.OPTIONS("/v1/models", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	request := httptest.NewRequest(http.MethodOptions, "/v1/models", nil)
	request.Header.Set("Origin", origin)
	request.Header.Set("Access-Control-Request-Method", method)
	if len(requestedHeaders) > 0 {
		request.Header.Set("Access-Control-Request-Headers", strings.Join(requestedHeaders, ", "))
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

func TestCORSAllowsPatchForSubscriptionAdministration(t *testing.T) {
	t.Setenv("CORS_ALLOW_ORIGINS", "https://app.example")

	recorder := performCORSPreflightForMethod(t, "https://app.example", http.MethodPatch,
		"new-api-user", "content-type")

	require.Equal(t, http.StatusNoContent, recorder.Code)
	assert.Contains(t, recorder.Header().Get("Access-Control-Allow-Methods"), http.MethodPatch)
}

func TestCORSDefaultDoesNotAllowCredentialsWithWildcard(t *testing.T) {
	t.Setenv("CORS_ALLOW_ORIGINS", "")

	recorder := performCORSPreflight(t, "https://evil.example")

	require.Equal(t, http.StatusNoContent, recorder.Code)
	assert.Equal(t, "*", recorder.Header().Get("Access-Control-Allow-Origin"))
	assert.Empty(t, recorder.Header().Get("Access-Control-Allow-Credentials"))
}

func TestCORSAllowlistAllowsCredentialsOnlyForTrustedOrigins(t *testing.T) {
	t.Setenv("CORS_ALLOW_ORIGINS", "https://app.example, https://admin.example")

	trustedRecorder := performCORSPreflight(t, "https://app.example")
	require.Equal(t, http.StatusNoContent, trustedRecorder.Code)
	assert.Equal(t, "https://app.example", trustedRecorder.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "true", trustedRecorder.Header().Get("Access-Control-Allow-Credentials"))

	untrustedRecorder := performCORSPreflight(t, "https://evil.example")
	require.Equal(t, http.StatusForbidden, untrustedRecorder.Code)
	assert.Empty(t, untrustedRecorder.Header().Get("Access-Control-Allow-Origin"))
	assert.Empty(t, untrustedRecorder.Header().Get("Access-Control-Allow-Credentials"))
}

func TestCORSDefaultPreservesRelayHeaderCompatibility(t *testing.T) {
	t.Setenv("CORS_ALLOW_ORIGINS", "")

	recorder := performCORSPreflight(t, "https://client.example",
		"x-api-key", "anthropic-version", "openai-beta")

	require.Equal(t, http.StatusNoContent, recorder.Code)
	assert.Equal(t, "*", recorder.Header().Get("Access-Control-Allow-Headers"))
}

func TestCORSAllowlistSupportsDashboardAndProviderHeaders(t *testing.T) {
	t.Setenv("CORS_ALLOW_ORIGINS", "https://app.example")

	recorder := performCORSPreflight(t, "https://app.example",
		"new-api-user", "x-api-key", "anthropic-version", "x-goog-api-key")

	require.Equal(t, http.StatusNoContent, recorder.Code)
	allowedHeaders := recorder.Header().Get("Access-Control-Allow-Headers")
	for _, header := range []string{"New-Api-User", "X-Api-Key", "Anthropic-Version", "X-Goog-Api-Key"} {
		assert.Contains(t, allowedHeaders, header)
	}
}

func TestSecurityHeadersSetBrowserHardeningHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(SecurityHeaders())
	router.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "nosniff", recorder.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "DENY", recorder.Header().Get("X-Frame-Options"))
	assert.Equal(t, "strict-origin-when-cross-origin", recorder.Header().Get("Referrer-Policy"))
	permissionsPolicy := recorder.Header().Get("Permissions-Policy")
	assert.Contains(t, permissionsPolicy, "camera=(self)")
	assert.Contains(t, permissionsPolicy, "microphone=()")
	assert.Contains(t, permissionsPolicy, "geolocation=()")
	assert.Equal(t, "frame-ancestors 'self'", recorder.Header().Get("Content-Security-Policy"))
}
