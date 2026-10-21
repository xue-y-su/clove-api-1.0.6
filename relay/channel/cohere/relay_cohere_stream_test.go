package cohere

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type trackedCohereBody struct {
	io.Reader
	closed bool
}

type cohereStreamRecorder struct {
	*httptest.ResponseRecorder
	closed chan bool
}

func (recorder *cohereStreamRecorder) CloseNotify() <-chan bool {
	return recorder.closed
}

func (body *trackedCohereBody) Close() error {
	body.closed = true
	return nil
}

func TestCohereStreamHandlerClosesBodyAndReportsTotalUsage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := &cohereStreamRecorder{ResponseRecorder: httptest.NewRecorder(), closed: make(chan bool)}
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	body := &trackedCohereBody{Reader: strings.NewReader(
		"{\"is_finished\":false,\"text\":\"hello\"}\n" +
			"{\"is_finished\":true,\"finish_reason\":\"COMPLETE\",\"response\":{\"meta\":{\"billed_units\":{\"input_tokens\":2,\"output_tokens\":3}}}}\n",
	)}
	response := &http.Response{StatusCode: http.StatusOK, Body: body}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "command-r"},
	}

	usage, apiErr := cohereStreamHandler(context, info, response)

	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	assert.Equal(t, 2, usage.PromptTokens)
	assert.Equal(t, 3, usage.CompletionTokens)
	assert.Equal(t, 5, usage.TotalTokens)
	assert.True(t, body.closed)
	assert.Contains(t, recorder.Body.String(), "hello")
	assert.Contains(t, recorder.Body.String(), "data: [DONE]")
}

func TestCohereStreamHandlerRejectsMalformedEvent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := &cohereStreamRecorder{ResponseRecorder: httptest.NewRecorder(), closed: make(chan bool)}
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	body := &trackedCohereBody{Reader: strings.NewReader("not-json\n")}
	response := &http.Response{StatusCode: http.StatusOK, Body: body}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "command-r"}}

	usage, apiErr := cohereStreamHandler(context, info, response)

	require.NotNil(t, apiErr)
	assert.Nil(t, usage)
	assert.True(t, body.closed)
}
