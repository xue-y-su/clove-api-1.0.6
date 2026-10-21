package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func performRequest(handler http.Handler, method string, target string, body io.Reader) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, target, body)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}

func useTestAnalyticsPages(t *testing.T, umamiPlaceholder, googlePlaceholder string) {
	t.Helper()
	originalIndex := indexPage
	originalClassic := classicIndexPage
	indexPage = []byte(umamiPlaceholder + googlePlaceholder)
	classicIndexPage = []byte(umamiPlaceholder + googlePlaceholder)
	t.Cleanup(func() {
		indexPage = originalIndex
		classicIndexPage = originalClassic
	})
}

func TestInjectUmamiAnalyticsRejectsHTMLInjection(t *testing.T) {
	useTestAnalyticsPages(t, "<!--umami-->\n", "")
	t.Setenv("UMAMI_WEBSITE_ID", `site"><script>alert(1)</script>`)
	t.Setenv("UMAMI_SCRIPT_URL", "https://analytics.example/script.js")

	InjectUmamiAnalytics()

	assert.NotContains(t, string(indexPage), "alert(1)")
	assert.NotContains(t, string(indexPage), "analytics.example")
}

func TestInjectUmamiAnalyticsRejectsNonHTTPSScriptURL(t *testing.T) {
	useTestAnalyticsPages(t, "<!--umami-->\n", "")
	t.Setenv("UMAMI_WEBSITE_ID", "site-123")
	t.Setenv("UMAMI_SCRIPT_URL", "data:text/javascript,alert(1)")

	InjectUmamiAnalytics()

	assert.NotContains(t, string(indexPage), "data:text/javascript")
	assert.NotContains(t, string(indexPage), "alert(1)")
}

func TestInjectGoogleAnalyticsRejectsScriptInjection(t *testing.T) {
	useTestAnalyticsPages(t, "", "<!--Google Analytics-->\n")
	t.Setenv("GOOGLE_ANALYTICS_ID", `G-test');alert(1);//`)

	InjectGoogleAnalytics()

	assert.NotContains(t, string(indexPage), "alert(1)")
	assert.NotContains(t, string(indexPage), "googletagmanager.com")
}

func TestInjectAnalyticsAcceptsValidConfiguration(t *testing.T) {
	useTestAnalyticsPages(t, "<!--umami-->\n", "<!--Google Analytics-->\n")
	t.Setenv("UMAMI_WEBSITE_ID", "site-123")
	t.Setenv("UMAMI_SCRIPT_URL", "https://analytics.example/script.js")
	t.Setenv("GOOGLE_ANALYTICS_ID", "G-ABC123")

	InjectUmamiAnalytics()
	InjectGoogleAnalytics()

	output := string(indexPage)
	assert.Contains(t, output, `src="https://analytics.example/script.js"`)
	assert.Contains(t, output, `data-website-id="site-123"`)
	assert.Contains(t, output, "googletagmanager.com/gtag/js?id=G-ABC123")
	assert.Contains(t, output, "gtag('config', 'G-ABC123')")
}

func TestNewHTTPServerHasProductionTimeouts(t *testing.T) {
	server := newHTTPServer(":3000", http.NewServeMux())

	assert.Equal(t, ":3000", server.Addr)
	assert.NotNil(t, server.Handler)
	assert.GreaterOrEqual(t, server.ReadHeaderTimeout, 5*time.Second)
	assert.GreaterOrEqual(t, server.ReadTimeout, 30*time.Second)
	assert.GreaterOrEqual(t, server.IdleTimeout, 30*time.Second)
	assert.GreaterOrEqual(t, server.MaxHeaderBytes, 1<<20)
}

func TestNewPprofServerDefaultsToLoopbackWithTimeouts(t *testing.T) {
	t.Setenv("PPROF_ADDR", "")

	server := newPprofServer()

	assert.Equal(t, "127.0.0.1:8005", server.Addr)
	assert.NotNil(t, server.Handler)
	assert.GreaterOrEqual(t, server.ReadHeaderTimeout, 5*time.Second)
	assert.GreaterOrEqual(t, server.ReadTimeout, 30*time.Second)
	assert.GreaterOrEqual(t, server.IdleTimeout, 30*time.Second)
}

func TestNewPprofServerAllowsExplicitInternalAddress(t *testing.T) {
	t.Setenv("PPROF_ADDR", "10.0.0.10:9000")

	server := newPprofServer()

	assert.Equal(t, "10.0.0.10:9000", server.Addr)
}

func TestSessionCookieSecureDefaultsToHTTPSFrontendBaseURL(t *testing.T) {
	t.Setenv("SESSION_COOKIE_SECURE", "")
	t.Setenv("SESSION_COOKIE_TRUSTED_URL", "")
	t.Setenv("FRONTEND_BASE_URL", "https://app.example")
	t.Cleanup(func() {
		common.SessionCookieSecure = false
		common.SessionCookieTrustedURLs = nil
	})

	require.NoError(t, common.InitSessionCookieSettings())
	assert.True(t, common.SessionCookieSecure)
}

func TestSessionCookieSecureAllowsExplicitLocalDevelopmentOverride(t *testing.T) {
	t.Setenv("SESSION_COOKIE_SECURE", "false")
	t.Setenv("SESSION_COOKIE_TRUSTED_URL", "")
	t.Setenv("FRONTEND_BASE_URL", "https://app.example")
	t.Cleanup(func() {
		common.SessionCookieSecure = false
		common.SessionCookieTrustedURLs = nil
	})

	require.NoError(t, common.InitSessionCookieSettings())
	assert.False(t, common.SessionCookieSecure)
}

func TestTrustedProxiesDefaultDisablesForwardedHeaderTrust(t *testing.T) {
	t.Setenv("TRUSTED_PROXIES", "")

	proxies, err := trustedProxiesFromEnv()

	require.NoError(t, err)
	assert.Empty(t, proxies)
}

func TestTrustedProxiesParsesCommaSeparatedList(t *testing.T) {
	t.Setenv("TRUSTED_PROXIES", "127.0.0.1, 10.0.0.0/8")

	proxies, err := trustedProxiesFromEnv()

	require.NoError(t, err)
	assert.Equal(t, []string{"127.0.0.1", "10.0.0.0/8"}, proxies)
}

func TestTrustedProxiesRejectsOpenNetworks(t *testing.T) {
	for _, value := range []string{"0.0.0.0/0", "::/0", "10.0.0.0/8, 0.0.0.0/0"} {
		t.Run(value, func(t *testing.T) {
			t.Setenv("TRUSTED_PROXIES", value)

			proxies, err := trustedProxiesFromEnv()

			require.Error(t, err)
			assert.Empty(t, proxies)
		})
	}
}

func TestConfigureTrustedProxiesUsesForwardedIPOnlyFromConfiguredProxy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("TRUSTED_PROXIES", "172.18.0.1")
	router := gin.New()
	require.NoError(t, configureTrustedProxies(router))
	router.GET("/client-ip", func(c *gin.Context) {
		c.String(http.StatusOK, c.ClientIP())
	})

	trustedRequest := httptest.NewRequest(http.MethodGet, "/client-ip", nil)
	trustedRequest.RemoteAddr = "172.18.0.1:43210"
	trustedRequest.Header.Set("X-Forwarded-For", "203.0.113.10")
	trustedRecorder := httptest.NewRecorder()
	router.ServeHTTP(trustedRecorder, trustedRequest)
	require.Equal(t, "203.0.113.10", trustedRecorder.Body.String())

	untrustedRequest := httptest.NewRequest(http.MethodGet, "/client-ip", nil)
	untrustedRequest.RemoteAddr = "198.51.100.25:43210"
	untrustedRequest.Header.Set("X-Forwarded-For", "203.0.113.11")
	untrustedRecorder := httptest.NewRecorder()
	router.ServeHTTP(untrustedRecorder, untrustedRequest)
	require.Equal(t, "198.51.100.25", untrustedRecorder.Body.String())
}

func TestServerListenAddressDefaultsToAllInterfaces(t *testing.T) {
	t.Setenv("BIND_ADDRESS", "")

	assert.Equal(t, ":3000", serverListenAddress("3000"))
}

func TestServerListenAddressSupportsLoopbackOnlyBinding(t *testing.T) {
	t.Setenv("BIND_ADDRESS", "127.0.0.1")

	assert.Equal(t, "127.0.0.1:3000", serverListenAddress("3000"))
}

func TestRecoveryResponseDoesNotExposePanicDetail(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(gin.CustomRecovery(func(c *gin.Context, err any) {
		respondWithPanicError(c)
	}))
	router.GET("/panic", func(c *gin.Context) {
		panic("database password leaked")
	})

	recorder := performRequest(router, http.MethodGet, "/panic", nil)

	require.Equal(t, http.StatusInternalServerError, recorder.Code)
	var body map[string]map[string]string
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	assert.Equal(t, "An internal server error occurred. Please contact the administrator with the request id.", body["error"]["message"])
	assert.Equal(t, "new_api_panic", body["error"]["type"])
	assert.NotContains(t, recorder.Body.String(), "database password leaked")
}
