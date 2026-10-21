package controller

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestPlaygroundErrorResponseHidesInfrastructureDetails(t *testing.T) {
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/api/playground", nil)
	context.Request.Header.Set("Accept-Language", "zh-CN")
	context.Set(common.RequestIdKey, "playground-request")
	errorWithDetails := types.NewError(
		errors.New("query channel 42 failed: SELECT * FROM users at C:\\private\\db.go"),
		types.ErrorCodeQueryDataError,
		types.ErrOptionWithSkipRetry(),
	)

	writePlaygroundErrorResponse(context, errorWithDetails)

	var response struct {
		Success   bool              `json:"success"`
		Code      string            `json:"code"`
		Message   string            `json:"message"`
		RequestID string            `json:"request_id"`
		Error     types.OpenAIError `json:"error"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.False(t, response.Success)
	require.Equal(t, "playground_request_failed", response.Code)
	require.Equal(t, "playground-request", response.RequestID)
	require.NotEmpty(t, response.Message)
	require.Equal(t, response.Message, response.Error.Message)
	require.NotContains(t, recorder.Body.String(), "channel 42")
	require.NotContains(t, recorder.Body.String(), "SELECT *")
	require.NotContains(t, recorder.Body.String(), "C:\\\\private")
}

func TestPlaygroundNativeSearchRequiresCompatibleSelectedChannel(t *testing.T) {
	allowedChannelIDs := map[int]struct{}{101: {}}
	for _, testCase := range []struct {
		name        string
		channelType int
		channelID   int
		expected    bool
	}{
		{name: "explicit OpenAI channel", channelType: constant.ChannelTypeOpenAI, channelID: 101, expected: true},
		{name: "unlisted OpenAI channel", channelType: constant.ChannelTypeOpenAI, channelID: 102, expected: false},
		{name: "explicit Azure channel", channelType: constant.ChannelTypeAzure, channelID: 101, expected: true},
		{name: "unsupported channel type", channelType: constant.ChannelTypeGemini, channelID: 101, expected: false},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			relayInfo := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{ChannelType: testCase.channelType, ChannelId: testCase.channelID}}
			require.Equal(t, testCase.expected, playgroundNativeSearchRouteCapable(relayInfo, allowedChannelIDs))
		})
	}
}

func TestGetOptionsReportsTavilyConfigurationWithoutReturningKey(t *testing.T) {
	common.OptionMapRWMutex.Lock()
	originalMap := common.OptionMap
	common.OptionMap = map[string]string{service.PlaygroundSearchOptionAPIKey: "tvly-production-secret"}
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = originalMap
		common.OptionMapRWMutex.Unlock()
	})
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	GetOptions(ctx)

	body := recorder.Body.String()
	require.NotContains(t, body, "tvly-production-secret")
	require.NotContains(t, body, `"key":"PlaygroundTavilyAPIKey"`)
	require.Contains(t, body, `"key":"PlaygroundTavilyAPIKeyConfigured"`)
	require.Contains(t, body, `"value":"true"`)
}

func TestUpdateOptionRejectsInvalidPlaygroundSearchSettings(t *testing.T) {
	setupComplianceControllerTest(t)
	for _, testCase := range []struct {
		key   string
		value any
	}{
		{key: service.PlaygroundSearchOptionDefaultDepth, value: "deep"},
		{key: service.PlaygroundSearchOptionMaxResults, value: 0},
		{key: service.PlaygroundSearchOptionMaxContextLength, value: 499},
	} {
		t.Run(testCase.key, func(t *testing.T) {
			ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/option/", map[string]any{
				"key": testCase.key, "value": testCase.value,
			}, 1)

			UpdateOption(ctx)

			response := decodeAPIResponse(t, recorder)
			require.False(t, response.Success)
			require.Contains(t, response.Message, "Playground")
		})
	}
}

func TestUpdateOptionRejectsClearingEnabledTavilyKey(t *testing.T) {
	setupComplianceControllerTest(t)
	common.OptionMapRWMutex.Lock()
	originalEnabled := common.OptionMap[service.PlaygroundSearchOptionEnabled]
	originalKey := common.OptionMap[service.PlaygroundSearchOptionAPIKey]
	common.OptionMap[service.PlaygroundSearchOptionEnabled] = "true"
	common.OptionMap[service.PlaygroundSearchOptionAPIKey] = "tvly-production-secret"
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap[service.PlaygroundSearchOptionEnabled] = originalEnabled
		common.OptionMap[service.PlaygroundSearchOptionAPIKey] = originalKey
		common.OptionMapRWMutex.Unlock()
	})

	ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/option/", map[string]any{
		"key": service.PlaygroundSearchOptionAPIKey, "value": "",
	}, 1)

	UpdateOption(ctx)

	response := decodeAPIResponse(t, recorder)
	require.False(t, response.Success)
	require.Contains(t, response.Message, "Tavily")
}

func TestUpdateOptionRejectsOversizedPlaygroundModelPolicies(t *testing.T) {
	setupComplianceControllerTest(t)
	for _, testCase := range []struct {
		name  string
		key   string
		value string
	}{
		{
			name:  "too many native patterns",
			key:   service.PlaygroundSearchOptionNativeModels,
			value: strings.Repeat("model,", 257),
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/option/", map[string]any{
				"key": testCase.key, "value": testCase.value,
			}, 1)

			UpdateOption(ctx)

			response := decodeAPIResponse(t, recorder)
			require.False(t, response.Success)
			require.Contains(t, response.Message, "Playground")
		})
	}
}

func TestBuildPlaygroundAuditEventDetectsWebSearch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set("id", 17)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/pg/chat/completions", strings.NewReader(`{
		"model":"gpt-4.1",
		"tools":[{"type":"web_search_preview"}],
		"web_search_options":{"search_context_size":"high"}
	}`))
	ctx.Request.Header.Set("Content-Type", "application/json")

	action, params := buildPlaygroundAuditEvent(ctx, relayconstant.RelayModeChatCompletions, "default")

	require.Equal(t, "playground.web_search", action)
	require.Equal(t, "chat", params["feature"])
	require.Equal(t, "default", params["group"])
	require.Equal(t, "gpt-4.1", params["model"])
	require.Equal(t, "high", params["web_search_context_size"])
	require.Equal(t, "/pg/chat/completions", params["route"])
}

func TestBuildPlaygroundAuditEventDetectsImageGeneration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set("id", 17)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/pg/images/generations", strings.NewReader(`{
		"model":"gpt-image-1",
		"size":"1024x1024",
		"n":2,
		"response_format":"url"
	}`))
	ctx.Request.Header.Set("Content-Type", "application/json")

	action, params := buildPlaygroundAuditEvent(ctx, relayconstant.RelayModeImagesGenerations, "creative")

	require.Equal(t, "playground.image_generation", action)
	require.Equal(t, "image_generation", params["feature"])
	require.Equal(t, "creative", params["group"])
	require.Equal(t, "gpt-image-1", params["model"])
	require.Equal(t, "1024x1024", params["size"])
	require.Equal(t, float64(2), params["n"])
	require.Equal(t, "url", params["response_format"])
	require.Equal(t, "/pg/images/generations", params["route"])
}

func TestBuildPlaygroundAuditEventSkipsPlainChat(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/pg/chat/completions", strings.NewReader(`{"model":"gpt-4.1"}`))
	ctx.Request.Header.Set("Content-Type", "application/json")

	action, params := buildPlaygroundAuditEvent(ctx, relayconstant.RelayModeChatCompletions, "default")

	require.Empty(t, action)
	require.Nil(t, params)
}

func TestBuildPlaygroundAuditEventUsesFallbackSearchContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/pg/chat/completions", strings.NewReader(`{"model":"custom-chat"}`))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Set("playground_web_search_path", "tavily")
	ctx.Set("playground_web_search_context_size", "low")
	ctx.Set("playground_web_search_source_count", 2)

	action, params := buildPlaygroundAuditEvent(ctx, relayconstant.RelayModeChatCompletions, "default")

	require.Equal(t, "playground.web_search", action)
	require.Equal(t, "chat", params["feature"])
	require.Equal(t, "tavily", params["web_search_path"])
	require.Equal(t, "low", params["web_search_context_size"])
	require.Equal(t, 2, params["web_search_source_count"])
}

func TestApplyPlaygroundSearchFallbackToContextRewritesCachedBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	body := `{"model":"custom-chat","messages":[{"role":"user","content":"latest redis release"}],"tools":[{"type":"web_search_preview"}],"web_search_options":{"search_context_size":"low"}}`
	ctx.Request = httptest.NewRequest(http.MethodPost, "/pg/chat/completions", strings.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")

	outcome, fallbackErr := applyPlaygroundSearchFallbackToContext(ctx, service.PlaygroundSearchSettings{
		Enabled: true,
		Provider: &fakeControllerSearchProvider{results: []service.PlaygroundSearchResult{
			{Title: "Redis 8", URL: "https://redis.io/blog/redis-8", Content: "Redis 8 includes query improvements."},
		}},
		DefaultDepth:  "basic",
		MaxResults:    3,
		MaxContextLen: 1200,
	})

	require.Nil(t, fallbackErr)
	require.Equal(t, service.PlaygroundSearchOutcomeTavily, outcome.Status)
	require.True(t, ctx.GetBool("playground_tavily_search"))
	require.Equal(t, "tavily", ctx.GetString("playground_web_search_path"))
	require.Equal(t, "low", ctx.GetString("playground_web_search_context_size"))

	storage, storageErr := common.GetBodyStorage(ctx)
	require.NoError(t, storageErr)
	updatedBody, bytesErr := storage.Bytes()
	require.NoError(t, bytesErr)

	var updated map[string]any
	require.NoError(t, json.Unmarshal(updatedBody, &updated))
	require.NotContains(t, updated, "web_search_options")
	require.Empty(t, updated["tools"])

	messages, ok := updated["messages"].([]any)
	require.True(t, ok)
	require.Len(t, messages, 2)
	searchContext, ok := messages[0].(map[string]any)["content"].(string)
	require.True(t, ok)
	require.Contains(t, searchContext, "[1] Redis 8")
}

func TestApplyPlaygroundSearchFallbackStopsWhenSearchIsUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	body := `{"model":"custom-chat","messages":[{"role":"user","content":"latest release"}],"web_search_options":{"search_context_size":"low"}}`
	ctx.Request = httptest.NewRequest(http.MethodPost, "/pg/chat/completions", strings.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")

	outcome, fallbackErr := applyPlaygroundSearchFallbackToContext(ctx, service.PlaygroundSearchSettings{})

	require.Equal(t, service.PlaygroundSearchOutcomeUnavailable, outcome.Status)
	require.ErrorIs(t, fallbackErr, service.ErrPlaygroundSearchUnavailable)
	require.Equal(t, http.StatusServiceUnavailable, fallbackErr.StatusCode)
}

func TestSetupPlaygroundRelayContextKeepsRequestedGroupAsSourceOfTruth(t *testing.T) {
	setupComplianceControllerTest(t)
	require.NoError(t, model.DB.Create(&model.User{
		Id: 731, Username: "playground-group-user", Group: "default",
		Status: common.UserStatusEnabled, AffCode: "pg-group-731",
	}).Error)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/pg/chat/completions", nil)
	ctx.Set("id", 731)
	common.SetContextKey(ctx, constant.ContextKeyUsingGroup, "stale")

	setupErr := setupPlaygroundRelayContext(ctx, "creative")

	require.Nil(t, setupErr)
	require.Equal(t, "creative", common.GetContextKeyString(ctx, constant.ContextKeyUsingGroup))
	require.Equal(t, "creative", common.GetContextKeyString(ctx, constant.ContextKeyTokenGroup))
}

type fakeControllerSearchProvider struct {
	results []service.PlaygroundSearchResult
}

func (p *fakeControllerSearchProvider) Search(ctx context.Context, req service.PlaygroundSearchRequest) ([]service.PlaygroundSearchResult, error) {
	return p.results, nil
}
