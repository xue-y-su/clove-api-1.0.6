package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	appi18n "github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGetPlaygroundRequestGroupReadsImageGenerationGroup(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/pg/images/generations", strings.NewReader(`{"model":"gpt-image-2","group":"drawing"}`))
	ctx.Request.Header.Set("Content-Type", "application/json")

	group, playgroundRequest, err := getPlaygroundRequestGroup(ctx)

	require.NoError(t, err)
	require.True(t, playgroundRequest)
	require.Equal(t, "drawing", group)
}

func TestGetPlaygroundRequestGroupIgnoresNonPlaygroundRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", strings.NewReader(`{"model":"gpt-image-2","group":"drawing"}`))
	ctx.Request.Header.Set("Content-Type", "application/json")

	group, playgroundRequest, err := getPlaygroundRequestGroup(ctx)

	require.NoError(t, err)
	require.False(t, playgroundRequest)
	require.Empty(t, group)
}

func TestPlaygroundGroupAccessUsesUserGroupForAutoToken(t *testing.T) {
	previousGroups := setting.UserUsableGroups2JSONString()
	t.Cleanup(func() {
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(previousGroups))
	})
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default","creative":"Creative"}`))

	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	common.SetContextKey(ctx, constant.ContextKeyUserGroup, "default")

	require.True(t, playgroundGroupAllowed(ctx, "auto", "creative"))
	require.False(t, playgroundGroupAllowed(ctx, "auto", "restricted"))
}

func TestPlaygroundChannelFailureMessageHidesDistributorDetails(t *testing.T) {
	require.NoError(t, appi18n.Init())
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/pg/chat/completions", nil)
	ctx.Request.Header.Set("Accept-Language", "zh-CN")

	message := channelSelectionFailureMessage(ctx, "creative", "gpt-image-2", errors.New("distributor: channel 17 failed"))

	require.Equal(t, "当前分组没有此模型的可用路由。", message)
	require.NotContains(t, message, "distributor")
	require.NotContains(t, message, "channel")
}

func TestStandardAPIChannelFailureMessageKeepsExistingDiagnostic(t *testing.T) {
	require.NoError(t, appi18n.Init())
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx.Request.Header.Set("Accept-Language", "zh-CN")

	message := channelSelectionFailureMessage(ctx, "default", "gpt-4.1", errors.New("route unavailable"))

	require.Contains(t, message, "default")
	require.Contains(t, message, "gpt-4.1")
	require.Contains(t, message, "route unavailable")
}
