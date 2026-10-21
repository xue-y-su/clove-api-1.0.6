package helper

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newPlaygroundReasoningContext(body string, channelType int) *gin.Context {
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/pg/chat/completions", bytes.NewBufferString(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	common.SetContextKey(ctx, constant.ContextKeyChannelType, channelType)
	return ctx
}

func TestPlaygroundReasoningAcceptsSupportedModelAndRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx := newPlaygroundReasoningContext(`{"model":"gpt-5-mini","messages":[{"role":"user","content":"hi"}],"reasoning_effort":"high"}`, constant.ChannelTypeOpenAI)

	request, err := GetAndValidateTextRequest(ctx, relayconstant.RelayModeChatCompletions)

	require.NoError(t, err)
	require.Equal(t, "high", request.ReasoningEffort)
	require.False(t, ctx.GetBool("playground_reasoning_control_ignored"))
}

func TestPlaygroundReasoningNormalizesExplicitOff(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx := newPlaygroundReasoningContext(`{"model":"gpt-5-mini","messages":[{"role":"user","content":"hi"}],"reasoning_effort":"off"}`, constant.ChannelTypeOpenAI)

	request, err := GetAndValidateTextRequest(ctx, relayconstant.RelayModeChatCompletions)

	require.NoError(t, err)
	require.Equal(t, "none", request.ReasoningEffort)
}

func TestPlaygroundReasoningRejectsUnsupportedValue(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx := newPlaygroundReasoningContext(`{"model":"gpt-5-mini","messages":[{"role":"user","content":"hi"}],"reasoning_effort":"extreme"}`, constant.ChannelTypeOpenAI)

	_, err := GetAndValidateTextRequest(ctx, relayconstant.RelayModeChatCompletions)

	require.EqualError(t, err, "playground reasoning_effort must be one of: off, low, medium, high")
}

func TestPlaygroundReasoningIgnoresUnsupportedModel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx := newPlaygroundReasoningContext(`{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}],"reasoning_effort":"high"}`, constant.ChannelTypeOpenAI)

	request, err := GetAndValidateTextRequest(ctx, relayconstant.RelayModeChatCompletions)

	require.NoError(t, err)
	require.Empty(t, request.ReasoningEffort)
	require.True(t, ctx.GetBool("playground_reasoning_control_ignored"))
}

func TestPlaygroundReasoningIgnoresUnsupportedRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx := newPlaygroundReasoningContext(`{"model":"gpt-5-mini","messages":[{"role":"user","content":"hi"}],"reasoning_effort":"high"}`, constant.ChannelTypeGemini)

	request, err := GetAndValidateTextRequest(ctx, relayconstant.RelayModeChatCompletions)

	require.NoError(t, err)
	require.Empty(t, request.ReasoningEffort)
	require.True(t, ctx.GetBool("playground_reasoning_control_ignored"))
}
