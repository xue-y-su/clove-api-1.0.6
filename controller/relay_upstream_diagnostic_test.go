package controller

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAppendUpstreamDiagnosticSyncsIntoExistingFailedLogDetails(t *testing.T) {
	previous := *operation_setting.GetUpstreamDiagnosticSetting()
	t.Cleanup(func() {
		*operation_setting.GetUpstreamDiagnosticSetting() = previous
		operation_setting.RebuildUpstreamDiagnosticSnapshot()
	})
	*operation_setting.GetUpstreamDiagnosticSetting() = operation_setting.UpstreamDiagnosticSetting{
		Enabled:             true,
		CaptureMode:         operation_setting.UpstreamDiagnosticCaptureAllErrors,
		PreviewChars:        2048,
		SyncToUsageLog:      true,
		DedupeWindowSeconds: 0,
	}
	operation_setting.RebuildUpstreamDiagnosticSnapshot()

	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx.Set(common.RequestIdKey, "request-17")
	apiErr := service.RelayErrorHandler(ctx.Request.Context(), &http.Response{
		StatusCode: http.StatusBadGateway,
		Body:       io.NopCloser(strings.NewReader(`<html>bad gateway</html>`)),
	}, false)
	originalMessage := apiErr.Error()
	other := map[string]interface{}{}

	appendUpstreamDiagnostic(ctx, 17, apiErr, other)

	require.Equal(t, originalMessage, apiErr.Error())
	value, ok := other["upstream_diagnostic"].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, 17, value["channel_id"])
	require.Equal(t, "request-17", value["request_id"])
}

func TestProcessChannelErrorRecordsDiagnosticWhenDatabaseErrorLogsAreDisabled(t *testing.T) {
	previousErrorLogEnabled := constant.ErrorLogEnabled
	previousRecorder := recordUpstreamDiagnostic
	constant.ErrorLogEnabled = false
	t.Cleanup(func() {
		constant.ErrorLogEnabled = previousErrorLogEnabled
		recordUpstreamDiagnostic = previousRecorder
	})

	called := 0
	recordUpstreamDiagnostic = func(
		_ context.Context,
		_ *types.NewAPIError,
		channelID int,
		requestID string,
		other map[string]interface{},
		_ operation_setting.UpstreamDiagnosticSnapshot,
	) error {
		called++
		require.Equal(t, 23, channelID)
		require.Equal(t, "request-23", requestID)
		require.Nil(t, other)
		return nil
	}

	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx.Set(common.RequestIdKey, "request-23")
	apiErr := service.RelayErrorHandler(ctx.Request.Context(), &http.Response{
		StatusCode: http.StatusBadGateway,
		Body:       io.NopCloser(strings.NewReader(`<html>bad gateway</html>`)),
	}, false)

	processChannelError(ctx, *types.NewChannelError(23, 1, "test", false, "", false), apiErr)

	require.Equal(t, 1, called)
}
