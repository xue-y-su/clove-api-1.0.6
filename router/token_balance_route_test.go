package router

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestTokenBalanceRouteIsPublicPostEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	SetApiRouter(engine)

	found := false
	for _, route := range engine.Routes() {
		if route.Method == http.MethodPost && route.Path == "/api/token-balance" {
			found = true
			break
		}
	}
	require.True(t, found, "POST /api/token-balance route not found")
}
