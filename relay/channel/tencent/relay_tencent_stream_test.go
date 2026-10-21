package tencent

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

type trackedTencentBody struct {
	io.Reader
	closed bool
}

func (body *trackedTencentBody) Close() error {
	body.closed = true
	return nil
}

func TestTencentStreamHandlerPropagatesScannerErrorAndClosesBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	originalMaxBufferMB := constant.StreamScannerMaxBufferMB
	constant.StreamScannerMaxBufferMB = 1
	t.Cleanup(func() {
		constant.StreamScannerMaxBufferMB = originalMaxBufferMB
	})

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	body := &trackedTencentBody{Reader: strings.NewReader("data:" + strings.Repeat("x", (1<<20)+1))}
	response := &http.Response{StatusCode: http.StatusOK, Body: body}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "hunyuan"}}

	usage, apiErr := tencentStreamHandler(context, info, response)

	require.NotNil(t, apiErr)
	assert.Nil(t, usage)
	assert.True(t, body.closed)
}
