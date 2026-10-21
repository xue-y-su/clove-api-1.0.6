package router

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestUserSecurityAuditRouteIsRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	SetApiRouter(engine)

	for _, route := range engine.Routes() {
		if route.Method == http.MethodGet && route.Path == "/api/user/:id/security-audit" {
			return
		}
	}
	require.Fail(t, "GET /api/user/:id/security-audit route not found")
}

func TestGlobalIPAuditDetailRouteIsRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	SetApiRouter(engine)

	for _, route := range engine.Routes() {
		if route.Method == http.MethodGet && route.Path == "/api/log/ip-audit/detail" {
			return
		}
	}
	require.Fail(t, "GET /api/log/ip-audit/detail route not found")
}
