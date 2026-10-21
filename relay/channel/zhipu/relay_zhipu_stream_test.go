package zhipu

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

type trackedZhipuBody struct {
	io.Reader
	closed bool
}

func (body *trackedZhipuBody) Close() error {
	body.closed = true
	return nil
}

type zhipuStreamRecorder struct {
	*httptest.ResponseRecorder
	closed chan bool
}

func (recorder *zhipuStreamRecorder) CloseNotify() <-chan bool {
	return recorder.closed
}

func TestZhipuStreamHandlerPropagatesScannerErrorAndClosesBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	originalMaxBufferMB := constant.StreamScannerMaxBufferMB
	constant.StreamScannerMaxBufferMB = 1
	t.Cleanup(func() {
		constant.StreamScannerMaxBufferMB = originalMaxBufferMB
	})

	recorder := &zhipuStreamRecorder{ResponseRecorder: httptest.NewRecorder(), closed: make(chan bool)}
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	body := &trackedZhipuBody{Reader: strings.NewReader("data:" + strings.Repeat("x", (1<<20)+1))}
	response := &http.Response{StatusCode: http.StatusOK, Body: body}

	usage, apiErr := zhipuStreamHandler(context, &relaycommon.RelayInfo{}, response)

	require.NotNil(t, apiErr)
	assert.Nil(t, usage)
	assert.True(t, body.closed)
}
