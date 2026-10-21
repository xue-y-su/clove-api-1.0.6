package service

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResponseConverterFacades(t *testing.T) {
	cache5m, cache1h := NormalizeCacheCreationSplit(10, 3, 2)
	assert.Equal(t, 8, cache5m)
	assert.Equal(t, 2, cache1h)

	chatResp := &dto.OpenAITextResponse{
		Id:    "chatcmpl_1",
		Model: "gpt-test",
		Choices: []dto.OpenAITextResponseChoice{{
			Message:      dto.Message{Role: "assistant", Content: "hello"},
			FinishReason: "stop",
		}},
	}

	claudeResp := ResponseOpenAI2Claude(chatResp, &relaycommon.RelayInfo{})
	require.NotNil(t, claudeResp)
	assert.Equal(t, "message", claudeResp.Type)

	geminiResp := ResponseOpenAI2Gemini(chatResp, &relaycommon.RelayInfo{})
	require.NotNil(t, geminiResp)
	require.Len(t, geminiResp.Candidates, 1)
}

func TestStreamResponseConverterFacades(t *testing.T) {
	info := &relaycommon.RelayInfo{
		SendResponseCount: 1,
		ClaudeConvertInfo: &relaycommon.ClaudeConvertInfo{
			LastMessagesType: relaycommon.LastMessageTypeNone,
		},
	}
	streamResp := &dto.ChatCompletionsStreamResponse{
		Id:    "chatcmpl_1",
		Model: "gpt-test",
		Choices: []dto.ChatCompletionsStreamResponseChoice{{
			Delta: dto.ChatCompletionsStreamResponseChoiceDelta{Content: ptrValue("hello")},
		}},
	}

	claudeResponses := StreamResponseOpenAI2Claude(streamResp, info)
	require.NotEmpty(t, claudeResponses)

	geminiResp := StreamResponseOpenAI2Gemini(streamResp, &relaycommon.RelayInfo{})
	require.NotNil(t, geminiResp)
	require.Len(t, geminiResp.Candidates, 1)
}

func TestGeminiToOpenAIRequestMatchesFunctionResponseToolCallID(t *testing.T) {
	request := &dto.GeminiChatRequest{
		Contents: []dto.GeminiChatContent{
			{
				Role: "model",
				Parts: []dto.GeminiPart{{
					FunctionCall: &dto.FunctionCall{
						FunctionName: "lookup_weather",
						Arguments:    map[string]any{"city": "Shanghai"},
					},
				}},
			},
			{
				Role: "user",
				Parts: []dto.GeminiPart{{
					FunctionResponse: &dto.GeminiFunctionResponse{
						Name:     "lookup_weather",
						Response: map[string]any{"temperature": 31},
					},
				}},
			},
		},
	}

	converted, err := GeminiToOpenAIRequest(request, &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "gemini-test"},
	})

	require.NoError(t, err)
	require.Len(t, converted.Messages, 2)
	toolCalls := converted.Messages[0].ParseToolCalls()
	require.Len(t, toolCalls, 1)
	require.Equal(t, "call_1", toolCalls[0].ID)
	require.Equal(t, "call_1", converted.Messages[1].ToolCallId)
}

func TestClaudeToOpenAIRequestKeepsTextWhenToolUseExists(t *testing.T) {
	request := dto.ClaudeRequest{
		Model: "claude-test",
		Messages: []dto.ClaudeMessage{{
			Role: "assistant",
			Content: []dto.ClaudeMediaMessage{
				{Type: "text", Text: common.GetPointer("I should call a tool first.")},
				{
					Type:  "tool_use",
					Id:    "toolu_1",
					Name:  "lookup_weather",
					Input: map[string]any{"city": "Shanghai"},
				},
			},
		}},
	}

	converted, err := ClaudeToOpenAIRequest(request, &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{},
	})

	require.NoError(t, err)
	require.Len(t, converted.Messages, 1)
	require.Equal(t, "I should call a tool first.", converted.Messages[0].StringContent())
	require.Len(t, converted.Messages[0].ParseToolCalls(), 1)
}

func TestStreamResponseOpenAI2ClaudeKeepsContentWhenReasoningExists(t *testing.T) {
	reasoning := "thinking"
	content := "answer"
	response := &dto.ChatCompletionsStreamResponse{
		Choices: []dto.ChatCompletionsStreamResponseChoice{{
			Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
				ReasoningContent: &reasoning,
				Content:          &content,
			},
		}},
	}
	info := &relaycommon.RelayInfo{
		SendResponseCount: 2,
		ClaudeConvertInfo: &relaycommon.ClaudeConvertInfo{
			LastMessagesType: relaycommon.LastMessageTypeNone,
		},
	}

	claudeResponses := StreamResponseOpenAI2Claude(response, info)

	var hasThinkingDelta bool
	var hasTextDelta bool
	for _, claudeResponse := range claudeResponses {
		if claudeResponse.Delta == nil {
			continue
		}
		if claudeResponse.Delta.Type == "thinking_delta" && claudeResponse.Delta.Thinking != nil && *claudeResponse.Delta.Thinking == reasoning {
			hasThinkingDelta = true
		}
		if claudeResponse.Delta.Type == "text_delta" && claudeResponse.Delta.Text != nil && *claudeResponse.Delta.Text == content {
			hasTextDelta = true
		}
	}
	require.True(t, hasThinkingDelta)
	require.True(t, hasTextDelta)
}

func ptrValue[T any](value T) *T {
	return &value
}
