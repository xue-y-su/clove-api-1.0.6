package controller

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRelayResponseBufferHoldsNonStreamBodyUntilCommit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	buffer := newRelayResponseBuffer(ctx.Writer, 1024)
	ctx.Writer = buffer

	ctx.Data(http.StatusOK, "application/json", []byte(`{"ok":true}`))

	require.Empty(t, recorder.Body.String())
	require.False(t, recorder.Flushed)
	require.NoError(t, buffer.Commit())
	require.Equal(t, http.StatusOK, recorder.Code)
	require.JSONEq(t, `{"ok":true}`, recorder.Body.String())
}

func TestRelayResponseBufferDiscardPreventsUpstreamBytesFromReachingClient(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	original := ctx.Writer
	buffer := newRelayResponseBuffer(original, 1024)
	ctx.Writer = buffer

	ctx.Data(http.StatusOK, "application/json", []byte(`{"upstream":"secret"}`))
	buffer.Discard()
	ctx.Writer = original
	ctx.JSON(http.StatusPaymentRequired, gin.H{"error": "settlement failed"})

	require.NotContains(t, recorder.Body.String(), "upstream")
	require.JSONEq(t, `{"error":"settlement failed"}`, recorder.Body.String())
}

func TestRelayResponseBufferStreamsEventSourceImmediately(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	buffer := newRelayResponseBuffer(ctx.Writer, 1024)
	ctx.Writer = buffer
	ctx.Header("Content-Type", "text/event-stream")

	_, err := ctx.Writer.Write([]byte("data: ready\n\n"))
	require.NoError(t, err)
	ctx.Writer.Flush()

	require.True(t, buffer.IsPassthrough())
	require.Equal(t, "data: ready\n\n", recorder.Body.String())
}

func TestRelayResponseBufferWithholdsIOCopyBytesUntilCommit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	buffer := newRelayResponseBuffer(ctx.Writer, 1024)
	ctx.Writer = buffer
	response := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"ok":true}`)),
	}

	service.IOCopyBytesGracefully(ctx, response, []byte(`{"ok":true}`))

	require.Empty(t, recorder.Body.String())
	require.False(t, buffer.IsPassthrough())
	require.NoError(t, buffer.Commit())
	require.JSONEq(t, `{"ok":true}`, recorder.Body.String())
}
