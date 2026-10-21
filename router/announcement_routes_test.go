package router

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAnnouncementStateRoutesAreRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	SetApiRouter(engine)

	routes := make(map[string]struct{})
	for _, route := range engine.Routes() {
		routes[route.Method+" "+route.Path] = struct{}{}
	}
	for _, route := range []string{
		http.MethodGet + " /api/announcement/popup-state",
		http.MethodPost + " /api/announcement/acknowledge",
		http.MethodPost + " /api/announcement/media",
		http.MethodGet + " /api/announcement-media/:filename",
	} {
		require.Contains(t, routes, route)
	}
}
