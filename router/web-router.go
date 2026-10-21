package router

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/gin-contrib/gzip"
	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
)

// ThemeAssets holds the frontend assets for both themes.
type ThemeAssets struct {
	DefaultBuildFS   fs.FS
	DefaultIndexPage []byte
	ClassicBuildFS   fs.FS
	ClassicIndexPage []byte
	SnowBuildFS      fs.FS
	SnowIndexPage    []byte
}

func SetWebRouter(router *gin.Engine, assets ThemeAssets) {
	defaultFS := common.EmbedFolder(assets.DefaultBuildFS, "web/default/dist")
	classicFS := common.EmbedFolder(assets.ClassicBuildFS, "web/classic/dist")
	snowFS := common.EmbedFolder(assets.SnowBuildFS, "web/snow/dist")
	themeFS := common.NewThemeAwareFS(defaultFS, classicFS)

	router.Use(gzip.Gzip(gzip.DefaultCompression))
	router.Use(middleware.GlobalWebRateLimit())
	router.Use(middleware.Cache())
	router.Use(static.Serve("/snow-assets", snowFS))
	router.Use(static.Serve("/", themeFS))
	serveSnowIndex := func(c *gin.Context) {
		c.Set(middleware.RouteTagKey, "web")
		c.Header("Cache-Control", "no-cache")
		c.Data(http.StatusOK, "text/html; charset=utf-8", assets.SnowIndexPage)
	}
	router.GET("/snow-assets", serveSnowIndex)
	router.GET("/snow-assets/", serveSnowIndex)
	router.NoRoute(func(c *gin.Context) {
		c.Set(middleware.RouteTagKey, "web")
		if shouldReturnWebNotFound(c.Request.URL.Path) {
			controller.RelayNotFound(c)
			return
		}
		c.Header("Cache-Control", "no-cache")
		if common.GetTheme() == "classic" {
			c.Data(http.StatusOK, "text/html; charset=utf-8", assets.ClassicIndexPage)
		} else {
			c.Data(http.StatusOK, "text/html; charset=utf-8", assets.DefaultIndexPage)
		}
	})
}

func shouldReturnWebNotFound(path string) bool {
	for _, prefix := range []string{"/v1", "/api", "/assets", "/snow-assets", "/static"} {
		if path == prefix || strings.HasPrefix(path, prefix+"/") {
			return true
		}
	}
	return false
}
