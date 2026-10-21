package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRegisterAlphaSearchRouteRejectsDisabledFeatureBeforeDistribution(t *testing.T) {
	previous := *operation_setting.GetAlphaSearchSetting()
	operation_setting.GetAlphaSearchSetting().Enabled = false
	operation_setting.RebuildAlphaSearchSnapshot()
	t.Cleanup(func() {
		*operation_setting.GetAlphaSearchSetting() = previous
		operation_setting.RebuildAlphaSearchSnapshot()
	})

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	v1 := engine.Group("/v1")
	distributed := false
	handled := false
	registerAlphaSearchRoute(v1, func(c *gin.Context) {
		distributed = true
		c.Next()
	}, func(c *gin.Context) {
		handled = true
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/alpha/search", nil)
	engine.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	require.False(t, distributed)
	require.False(t, handled)
	require.JSONEq(t, `{"error":{"message":"alpha search is disabled","type":"new_api_error","param":"","code":"alpha_search_disabled"}}`, recorder.Body.String())
}

func TestRegisterAlphaSearchRouteRunsDistributionWhenEnabled(t *testing.T) {
	previous := *operation_setting.GetAlphaSearchSetting()
	operation_setting.GetAlphaSearchSetting().Enabled = true
	operation_setting.RebuildAlphaSearchSnapshot()
	t.Cleanup(func() {
		*operation_setting.GetAlphaSearchSetting() = previous
		operation_setting.RebuildAlphaSearchSnapshot()
	})

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	v1 := engine.Group("/v1")
	distributed := false
	registerAlphaSearchRoute(v1, func(c *gin.Context) {
		distributed = true
		c.Next()
	}, func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/alpha/search", nil)
	engine.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusNoContent, recorder.Code)
	require.True(t, distributed)
}
