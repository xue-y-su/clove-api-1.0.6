package cloudflare

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type trackedCloudflareBody struct {
	io.Reader
	closed bool
}

func (body *trackedCloudflareBody) Close() error {
	body.closed = true
	return nil
}

func TestCloudflareStreamHandlerPropagatesScannerErrorAndClosesBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	originalMaxBufferMB := constant.StreamScannerMaxBufferMB
	constant.StreamScannerMaxBufferMB = 1
	t.Cleanup(func() {
		constant.StreamScannerMaxBufferMB = originalMaxBufferMB
	})

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	body := &trackedCloudflareBody{Reader: strings.NewReader("data: " + strings.Repeat("x", (1<<20)+1))}
	response := &http.Response{StatusCode: http.StatusOK, Body: body}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "cf-model"}}

	apiErr, usage := cfStreamHandler(context, info, response)

	require.NotNil(t, apiErr)
	assert.Nil(t, usage)
	assert.True(t, body.closed)
}

func TestCloudflareStreamHandlerSetsAssistantRole(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	body := &trackedCloudflareBody{Reader: strings.NewReader("data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"hello\"}}]}\n\ndata: [DONE]\n")}
	response := &http.Response{StatusCode: http.StatusOK, Body: body}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "cf-model"}}

	apiErr, usage := cfStreamHandler(context, info, response)

	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	assert.True(t, body.closed)
	assert.Contains(t, recorder.Body.String(), `"role":"assistant"`)
}

func TestCloudflareStreamHandlerRejectsMalformedEvent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	body := &trackedCloudflareBody{Reader: strings.NewReader("data: not-json\n")}
	response := &http.Response{StatusCode: http.StatusOK, Body: body}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "cf-model"}}

	apiErr, usage := cfStreamHandler(context, info, response)

	require.NotNil(t, apiErr)
	assert.Nil(t, usage)
	assert.True(t, body.closed)
}
