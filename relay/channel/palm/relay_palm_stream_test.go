package palm

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type failingPalmBody struct {
	closed bool
}

func (body *failingPalmBody) Read([]byte) (int, error) {
	return 0, errors.New("upstream read failed")
}

func (body *failingPalmBody) Close() error {
	body.closed = true
	return nil
}

type palmStreamRecorder struct {
	*httptest.ResponseRecorder
	closed chan bool
}

func (recorder *palmStreamRecorder) CloseNotify() <-chan bool {
	return recorder.closed
}

func TestPalmStreamHandlerPropagatesReadErrorAndClosesBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := &palmStreamRecorder{ResponseRecorder: httptest.NewRecorder(), closed: make(chan bool)}
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	body := &failingPalmBody{}
	response := &http.Response{StatusCode: http.StatusOK, Body: body}

	apiErr, responseText := palmStreamHandler(context, response)

	require.NotNil(t, apiErr)
	assert.Empty(t, responseText)
	assert.True(t, body.closed)
}
