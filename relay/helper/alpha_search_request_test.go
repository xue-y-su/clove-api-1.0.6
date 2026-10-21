package helper

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGetAndValidateAlphaSearchRequestPreservesUnknownFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	raw := []byte(`{"model":"gpt-5.1","commands":{"search_query":[{"q":"weather"}]},"future_field":{"nested":true}}`)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/alpha/search", bytes.NewReader(raw))
	ctx.Request.Header.Set("Content-Type", "application/json")
	t.Cleanup(func() { common.CleanupBodyStorage(ctx) })

	request, err := GetAndValidateRequest(ctx, types.RelayFormatOpenAIAlphaSearch)
	require.NoError(t, err)
	alphaRequest, ok := request.(*dto.AlphaSearchRequest)
	require.True(t, ok)
	require.Equal(t, "gpt-5.1", alphaRequest.Model)
	require.Equal(t, raw, []byte(alphaRequest.RawBody))
	require.False(t, alphaRequest.IsStream(ctx))
}

func TestGetAndValidateAlphaSearchRequestRequiresModel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/alpha/search", bytes.NewBufferString(`{"query":"hello"}`))
	ctx.Request.Header.Set("Content-Type", "application/json")
	t.Cleanup(func() { common.CleanupBodyStorage(ctx) })

	_, err := GetAndValidateRequest(ctx, types.RelayFormatOpenAIAlphaSearch)
	require.ErrorContains(t, err, "model is required")
}
