package router

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestFeatureSettingRoutesAreRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	SetApiRouter(engine)

	routes := make(map[string]struct{})
	for _, route := range engine.Routes() {
		routes[route.Method+" "+route.Path] = struct{}{}
	}
	for _, route := range []string{
		http.MethodGet + " /api/announcement/settings",
		http.MethodPut + " /api/announcement/settings",
		http.MethodGet + " /api/security/settings",
		http.MethodPut + " /api/security/settings",
		http.MethodGet + " /api/invitation/incentive-settings",
		http.MethodPut + " /api/invitation/incentive-settings",
	} {
		require.Contains(t, routes, route)
	}
}
