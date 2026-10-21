package router

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestUserRedemptionGovernanceRoutesAreRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	SetApiRouter(engine)

	routes := make(map[string]struct{})
	for _, route := range engine.Routes() {
		routes[route.Method+" "+route.Path] = struct{}{}
	}
	for _, route := range []string{
		http.MethodPost + " /api/user-redemption/admin/:id/disable",
		http.MethodPost + " /api/user-redemption/admin/:id/enable",
		http.MethodPost + " /api/user-redemption/admin/:id/terminate",
		http.MethodPost + " /api/user-redemption/admin/:id/delete",
	} {
		require.Contains(t, routes, route)
	}
}
