package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestPlaygroundMetadataWriterEmitsMetadataBeforeFirstSSEChunkOnce(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/pg/chat/completions", nil)
	ctx.Writer.Header().Set("Content-Type", "text/event-stream")
	ctx.Writer = newPlaygroundMetadataWriter(ctx.Writer, service.PlaygroundSearchOutcome{
		Status: service.PlaygroundSearchOutcomeTavily,
		Sources: []service.PlaygroundSearchResult{{
			Title: "Source", URL: "https://example.test/a", Content: "Evidence",
		}},
	})

	_, err := ctx.Writer.Write([]byte("data: {\"choices\":[]}\n\n"))
	require.NoError(t, err)
	_, err = ctx.Writer.Write([]byte("data: [DONE]\n\n"))
	require.NoError(t, err)

	body := recorder.Body.String()
	require.Equal(t, 1, strings.Count(body, `"type":"playground.metadata"`))
	require.Less(t, strings.Index(body, `"type":"playground.metadata"`), strings.Index(body, `"choices"`))
	require.Contains(t, body, `"domain":"example.test"`)
	require.Contains(t, body, `"snippet":"Evidence"`)
}

func TestPlaygroundMetadataWriterDoesNotChangeJSONResponses(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Writer.Header().Set("Content-Type", "application/json")
	ctx.Writer = newPlaygroundMetadataWriter(ctx.Writer, service.PlaygroundSearchOutcome{
		Status:  service.PlaygroundSearchOutcomeTavily,
		Sources: []service.PlaygroundSearchResult{{Title: "Source", URL: "https://example.test", Content: "Evidence"}},
	})

	payload := map[string]any{"choices": []any{}}
	body, err := json.Marshal(payload)
	require.NoError(t, err)
	_, err = ctx.Writer.Write(body)
	require.NoError(t, err)

	require.JSONEq(t, string(body), recorder.Body.String())
	require.NotContains(t, recorder.Body.String(), "playground.metadata")
}

func TestPlaygroundMetadataWriterDoesNotPrependMetadataToJSONErrorAfterSSESetup(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Writer.Header().Set("Content-Type", "text/event-stream")
	ctx.Writer = newPlaygroundMetadataWriter(ctx.Writer, service.PlaygroundSearchOutcome{
		Status: service.PlaygroundSearchOutcomeUnavailable,
	})

	body := []byte(`{"error":{"message":"bad response status code 502","code":"bad_response_status_code"}}`)
	_, err := ctx.Writer.Write(body)
	require.NoError(t, err)

	require.JSONEq(t, string(body), recorder.Body.String())
	require.NotContains(t, recorder.Body.String(), "playground.metadata")
}

func TestPlaygroundMetadataLimitsSourcesToFive(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Writer.Header().Set("Content-Type", "text/event-stream")
	sources := make([]service.PlaygroundSearchResult, 0, 8)
	for i := 0; i < 8; i++ {
		sources = append(sources, service.PlaygroundSearchResult{Title: "Source", URL: "https://example.test", Content: "Evidence"})
	}
	ctx.Writer = newPlaygroundMetadataWriter(ctx.Writer, service.PlaygroundSearchOutcome{
		Status:  service.PlaygroundSearchOutcomeTavily,
		Sources: sources,
	})

	_, err := ctx.Writer.Write([]byte("data: {}\n\n"))
	require.NoError(t, err)

	require.Equal(t, 5, strings.Count(recorder.Body.String(), `"domain":"example.test"`))
}
