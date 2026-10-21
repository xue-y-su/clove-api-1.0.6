package coze

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type trackedCozeBody struct {
	io.Reader
	closed bool
}

func TestPollCozeUntilCompleteHonorsCancellationBeforePolling(t *testing.T) {
	requestContext, cancel := context.WithCancel(context.Background())
	cancel()
	pollCount := 0

	err := pollCozeUntilComplete(requestContext, time.Millisecond, func() (bool, error) {
		pollCount++
		return false, nil
	})

	require.ErrorIs(t, err, context.Canceled)
	assert.Zero(t, pollCount)
}

func TestPollCozeUntilCompleteStopsAtCompletion(t *testing.T) {
	pollCount := 0

	err := pollCozeUntilComplete(context.Background(), 0, func() (bool, error) {
		pollCount++
		return pollCount == 2, nil
	})

	require.NoError(t, err)
	assert.Equal(t, 2, pollCount)
}

func (body *trackedCozeBody) Close() error {
	body.closed = true
	return nil
}

func TestCozeStreamHandlerClosesBodyWhenScannerFails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	originalMaxBufferMB := constant.StreamScannerMaxBufferMB
	constant.StreamScannerMaxBufferMB = 1
	t.Cleanup(func() {
		constant.StreamScannerMaxBufferMB = originalMaxBufferMB
	})

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	body := &trackedCozeBody{Reader: strings.NewReader("event:" + strings.Repeat("x", (1<<20)+1))}
	response := &http.Response{StatusCode: http.StatusOK, Body: body}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "coze"}}

	usage, apiErr := cozeChatStreamHandler(context, info, response)

	require.NotNil(t, apiErr)
	assert.Nil(t, usage)
	assert.True(t, body.closed)
}

func TestCozeStreamHandlerRejectsMalformedEvent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	body := &trackedCozeBody{Reader: strings.NewReader("event: conversation.message.delta\ndata: not-json\n\n")}
	response := &http.Response{StatusCode: http.StatusOK, Body: body}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "coze"}}

	usage, apiErr := cozeChatStreamHandler(context, info, response)

	require.NotNil(t, apiErr)
	assert.Nil(t, usage)
	assert.True(t, body.closed)
}
