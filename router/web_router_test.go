package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testDefaultBuildFS = fstest.MapFS{
	"web/default/dist/assets/snow-only.txt": &fstest.MapFile{
		Data: []byte("default-only-asset"),
	},
}

var testClassicBuildFS = fstest.MapFS{
	"web/classic/dist/assets/classic-only.txt": &fstest.MapFile{
		Data: []byte("classic-only-asset"),
	},
}

var testSnowBuildFS = fstest.MapFS{
	"web/snow/dist/assets/snow-only.txt": &fstest.MapFile{
		Data: []byte("snow-only-asset"),
	},
}

func TestSnowModeKeepsNewAPIApplicationShellForAllRoutes(t *testing.T) {
	previousMode := common.HomePageMode
	previousTheme := common.GetTheme()
	common.HomePageMode = "snow"
	t.Cleanup(func() {
		common.HomePageMode = previousMode
		common.SetTheme(previousTheme)
	})

	assets := ThemeAssets{
		DefaultBuildFS:   testDefaultBuildFS,
		DefaultIndexPage: []byte("default-index"),
		ClassicBuildFS:   testClassicBuildFS,
		ClassicIndexPage: []byte("classic-index"),
		SnowBuildFS:      testSnowBuildFS,
		SnowIndexPage:    []byte("snow-index"),
	}

	for _, test := range []struct {
		name          string
		theme         string
		expectedIndex string
	}{
		{name: "default theme", theme: "default", expectedIndex: "default-index"},
		{name: "classic theme", theme: "classic", expectedIndex: "classic-index"},
	} {
		t.Run(test.name, func(t *testing.T) {
			common.SetTheme(test.theme)
			engine := gin.New()
			SetWebRouter(engine, assets)

			for _, path := range []string{
				"/",
				"/login",
				"/console",
				"/dashboard",
				"/token",
				"/pricing",
				"/unknown-new-api-route",
			} {
				response := performWebRequest(engine, path)
				require.Equal(t, http.StatusOK, response.Code, path)
				assert.Equal(t, test.expectedIndex, response.Body.String(), path)
				assert.NotContains(t, response.Body.String(), "snow-index", path)
			}
		})
	}
}

func TestSnowAssetsStayUnderDedicatedPrefix(t *testing.T) {
	previousMode := common.HomePageMode
	previousTheme := common.GetTheme()
	common.HomePageMode = "snow"
	common.SetTheme("default")
	t.Cleanup(func() {
		common.HomePageMode = previousMode
		common.SetTheme(previousTheme)
	})

	engine := gin.New()
	SetWebRouter(engine, ThemeAssets{
		DefaultBuildFS:   testDefaultBuildFS,
		DefaultIndexPage: []byte("default-index"),
		ClassicBuildFS:   testClassicBuildFS,
		ClassicIndexPage: []byte("classic-index"),
		SnowBuildFS:      testSnowBuildFS,
		SnowIndexPage:    []byte("snow-index"),
	})

	for _, path := range []string{"/snow-assets", "/snow-assets/"} {
		snowPage := performWebRequest(engine, path)
		require.Equal(t, http.StatusOK, snowPage.Code, path)
		assert.Equal(t, "snow-index", snowPage.Body.String(), path)
	}

	prefixedAsset := performWebRequest(engine, "/snow-assets/assets/snow-only.txt")
	require.Equal(t, http.StatusOK, prefixedAsset.Code)
	assert.Equal(t, "snow-only-asset", strings.TrimSpace(prefixedAsset.Body.String()))

	leakedAsset := performWebRequest(engine, "/assets/snow-only.txt")
	require.Equal(t, http.StatusOK, leakedAsset.Code)
	assert.Equal(t, "default-only-asset", strings.TrimSpace(leakedAsset.Body.String()))

	missingAsset := performWebRequest(engine, "/snow-assets/assets/missing.txt")
	assert.Equal(t, http.StatusNotFound, missingAsset.Code)
}

func performWebRequest(handler http.Handler, path string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, path, nil)
	handler.ServeHTTP(recorder, request)
	return recorder
}

func TestShouldReturnWebNotFoundForMissingBuildAssets(t *testing.T) {
	t.Parallel()

	for _, path := range []string{
		"/api/missing",
		"/assets/missing.js",
		"/static/js/old-build.js",
		"/static/css/old-build.css",
		"/v1/missing",
	} {
		require.True(t, shouldReturnWebNotFound(path), path)
	}
}

func TestShouldKeepSPAHistoryFallbackForApplicationRoutes(t *testing.T) {
	t.Parallel()

	for _, path := range []string{
		"/",
		"/pricing",
		"/dashboard/overview",
		"/static-analysis",
	} {
		require.False(t, shouldReturnWebNotFound(path), path)
	}
}
