package router

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestInvitationRoutesAreRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	SetApiRouter(engine)

	routes := make(map[string]string)
	for _, route := range engine.Routes() {
		routes[route.Method+" "+route.Path] = route.Handler
	}
	expected := []string{
		http.MethodPost + " /api/redemption/claim",
		http.MethodPost + " /api/invitation/validate",
		http.MethodGet + " /api/invitation/poster-logo",
		http.MethodGet + " /api/invitation/self",
		http.MethodGet + " /api/invitation/rewards",
		http.MethodPost + " /api/invitation/rewards/payout-requests",
		http.MethodPost + " /api/invitation/applications",
		http.MethodPost + " /api/invitation/applications/:id/withdraw",
		http.MethodGet + " /api/invitation/admin/applications",
		http.MethodPost + " /api/invitation/admin/applications/:id/review",
		http.MethodGet + " /api/invitation/admin/codes",
		http.MethodPost + " /api/invitation/admin/codes",
		http.MethodPost + " /api/invitation/admin/codes/:id/revoke",
		http.MethodGet + " /api/invitation/admin/uses",
		http.MethodGet + " /api/invitation/admin/reward-payout-requests",
		http.MethodPost + " /api/invitation/admin/reward-payout-requests/:id/review",
		http.MethodGet + " /api/invitation/admin/reward-payout-requests/:id",
		http.MethodPost + " /api/invitation/admin/reward-payout-requests/:id/freeze",
		http.MethodPost + " /api/invitation/admin/reward-payout-requests/:id/unfreeze",
	}
	for _, route := range expected {
		require.Contains(t, routes, route)
	}
	require.NotContains(t, routes, http.MethodPost+" /api/invitation/rewards/withdraw")
	require.NotContains(t, routes, http.MethodGet+" /api/invitation/admin/rewards")
	require.NotContains(t, routes, http.MethodPost+" /api/invitation/admin/rewards/:id/review")
}

func TestInvitationPosterLogoRouteIsPublicAndUsesGlobalReadLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previousRateLimit := common.CriticalRateLimitEnable
	common.CriticalRateLimitEnable = false
	common.OptionMapRWMutex.Lock()
	previousLogo := common.Logo
	common.Logo = ""
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.CriticalRateLimitEnable = previousRateLimit
		common.OptionMapRWMutex.Lock()
		common.Logo = previousLogo
		common.OptionMapRWMutex.Unlock()
	})

	engine := gin.New()
	SetApiRouter(engine)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/invitation/poster-logo", nil)
	request.Header.Set("Accept-Encoding", "gzip")
	engine.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusNotFound, recorder.Code)
	require.NotEqual(t, http.StatusUnauthorized, recorder.Code)
	require.Empty(t, recorder.Header().Get("Content-Encoding"))

	source, err := os.ReadFile("api-router.go")
	require.NoError(t, err)
	normalizedSource := strings.ReplaceAll(string(source), "\r\n", "\n")
	require.Contains(t, normalizedSource, "apiRouter.Use(middleware.GlobalAPIRateLimit())")
	require.Contains(t, normalizedSource, `apiRouter.GET("/invitation/poster-logo", controller.GetInvitationPosterLogo)`)
	require.NotContains(t, normalizedSource, `CriticalIPRateLimit("invitation.poster-logo.read")`)
}
