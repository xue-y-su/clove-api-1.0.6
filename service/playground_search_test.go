package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/require"
)

type fakePlaygroundSearchProvider struct {
	results []PlaygroundSearchResult
	err     error
	query   string
	depth   string
}

func (p *fakePlaygroundSearchProvider) Search(ctx context.Context, req PlaygroundSearchRequest) ([]PlaygroundSearchResult, error) {
	p.query = req.Query
	p.depth = req.Depth
	return p.results, p.err
}

func TestApplyPlaygroundSearchFallbackLeavesNativeSearchUntouched(t *testing.T) {
	req := &dto.GeneralOpenAIRequest{
		Model:    "gpt-4.1",
		Messages: []dto.Message{{Role: "user", Content: "latest ai news"}},
		Tools: []dto.ToolCallRequest{{
			Type: "web_search_preview",
		}},
		WebSearchOptions: &dto.WebSearchOptions{SearchContextSize: "high"},
		Stream:           boolPtr(false),
		Temperature:      floatPtr(0.7),
		TopP:             floatPtr(1),
		FrequencyPenalty: floatPtr(0),
		PresencePenalty:  floatPtr(0),
		ReasoningEffort:  "low",
	}
	provider := &fakePlaygroundSearchProvider{}

	outcome := ApplyPlaygroundSearchFallback(context.Background(), req, PlaygroundSearchSettings{
		Enabled:            true,
		NativeModels:       []string{"gpt-4.1"},
		NativeRouteCapable: true,
		Provider:           provider,
		MaxResults:         3,
		MaxContextLen:      1200,
	})

	require.Equal(t, PlaygroundSearchOutcomeNative, outcome.Status)
	require.NotNil(t, req.WebSearchOptions)
	require.Len(t, req.Tools, 1)
	require.Empty(t, provider.query)
	require.NotNil(t, req.Stream)
	require.NotNil(t, req.Temperature)
	require.NotNil(t, req.TopP)
	require.NotNil(t, req.FrequencyPenalty)
	require.NotNil(t, req.PresencePenalty)
	require.Equal(t, "low", req.ReasoningEffort)
}

func TestApplyPlaygroundSearchFallbackRequiresNativeRouteCapability(t *testing.T) {
	req := &dto.GeneralOpenAIRequest{
		Model:            "gpt-4.1",
		Messages:         []dto.Message{{Role: "user", Content: "latest ai news"}},
		WebSearchOptions: &dto.WebSearchOptions{SearchContextSize: "low"},
	}
	provider := &fakePlaygroundSearchProvider{results: []PlaygroundSearchResult{{
		Title: "Current source", URL: "https://example.test/news", Content: "Current facts.",
	}}}

	outcome := ApplyPlaygroundSearchFallback(context.Background(), req, PlaygroundSearchSettings{
		Enabled:            true,
		NativeModels:       []string{"gpt-4.1"},
		NativeRouteCapable: false,
		Provider:           provider,
		MaxResults:         3,
		MaxContextLen:      1200,
	})

	require.Equal(t, PlaygroundSearchOutcomeTavily, outcome.Status)
	require.Equal(t, "latest ai news", provider.query)
}

func TestApplyPlaygroundSearchFallbackUsesRequestedDepth(t *testing.T) {
	for _, testCase := range []struct {
		name        string
		contextSize string
		fallback    string
		expected    string
	}{
		{name: "basic", contextSize: "low", fallback: "advanced", expected: "basic"},
		{name: "advanced", contextSize: "high", fallback: "basic", expected: "advanced"},
		{name: "default", contextSize: "medium", fallback: "advanced", expected: "advanced"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			req := &dto.GeneralOpenAIRequest{
				Model:            "custom-chat",
				Messages:         []dto.Message{{Role: "user", Content: "search this"}},
				WebSearchOptions: &dto.WebSearchOptions{SearchContextSize: testCase.contextSize},
			}
			provider := &fakePlaygroundSearchProvider{results: []PlaygroundSearchResult{{
				Title: "Source", URL: "https://example.test", Content: "Evidence",
			}}}

			outcome := ApplyPlaygroundSearchFallback(context.Background(), req, PlaygroundSearchSettings{
				Enabled:       true,
				Provider:      provider,
				DefaultDepth:  testCase.fallback,
				MaxResults:    3,
				MaxContextLen: 1200,
			})

			require.Equal(t, PlaygroundSearchOutcomeTavily, outcome.Status)
			require.Equal(t, testCase.expected, provider.depth)
		})
	}
}

func TestApplyPlaygroundSearchFallbackPropagatesCancellation(t *testing.T) {
	req := &dto.GeneralOpenAIRequest{
		Model:            "custom-chat",
		Messages:         []dto.Message{{Role: "user", Content: "search this"}},
		WebSearchOptions: &dto.WebSearchOptions{SearchContextSize: "low"},
	}
	provider := &cancellingPlaygroundSearchProvider{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	outcome := ApplyPlaygroundSearchFallback(ctx, req, PlaygroundSearchSettings{
		Enabled:       true,
		Provider:      provider,
		MaxResults:    3,
		MaxContextLen: 1200,
	})

	require.Equal(t, PlaygroundSearchOutcomeFailed, outcome.Status)
	require.ErrorIs(t, provider.err, context.Canceled)
}

func TestApplyPlaygroundSearchFallbackReportsOnlySourcesActuallyInjected(t *testing.T) {
	req := &dto.GeneralOpenAIRequest{
		Model:            "custom-chat",
		Messages:         []dto.Message{{Role: "user", Content: "search this"}},
		WebSearchOptions: &dto.WebSearchOptions{SearchContextSize: "low"},
	}
	provider := &fakePlaygroundSearchProvider{results: []PlaygroundSearchResult{
		{Title: "First", URL: "https://one.example/a", Content: strings.Repeat("one ", 80)},
		{Title: "Second", URL: "https://two.example/b", Content: strings.Repeat("two ", 80)},
	}}

	outcome := ApplyPlaygroundSearchFallback(context.Background(), req, PlaygroundSearchSettings{
		Enabled:       true,
		Provider:      provider,
		MaxResults:    5,
		MaxContextLen: 320,
	})

	require.Equal(t, PlaygroundSearchOutcomeTavily, outcome.Status)
	require.Len(t, outcome.Sources, 1)
	require.Equal(t, "First", outcome.Sources[0].Title)
}

func TestTavilySearchProviderRejectsOversizedResponses(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"results":[]}`+strings.Repeat(" ", (1<<20)+1))
	}))
	t.Cleanup(server.Close)
	provider := &tavilySearchProvider{
		apiKey:     "test-key",
		endpoint:   server.URL,
		httpClient: server.Client(),
		maxResults: 5,
	}

	results, err := provider.Search(context.Background(), PlaygroundSearchRequest{Query: "q", Depth: "basic"})

	require.Error(t, err)
	require.Nil(t, results)
	require.NotContains(t, err.Error(), "test-key")
}

func TestTavilySearchProviderNormalizesUntrustedResults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"results":[`+
			`{"title":"bad","url":"javascript:alert(1)","content":"ignore"},`+
			`{"title":"  Useful\u0000 title  ","url":"https://user:pass@example.test/path#fragment","content":"  bounded evidence  "}`+
			`]}`)
	}))
	t.Cleanup(server.Close)
	provider := &tavilySearchProvider{
		apiKey:     "test-key",
		endpoint:   server.URL,
		httpClient: server.Client(),
		maxResults: 5,
	}

	results, err := provider.Search(context.Background(), PlaygroundSearchRequest{Query: "q", Depth: "advanced"})

	require.NoError(t, err)
	require.Equal(t, []PlaygroundSearchResult{{
		Title: "Useful title", URL: "https://example.test/path", Content: "bounded evidence",
	}}, results)
}

func TestTavilySearchProviderHonorsRequestCancellation(t *testing.T) {
	requestStarted := make(chan struct{})
	releaseHandler := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(requestStarted)
		select {
		case <-r.Context().Done():
		case <-releaseHandler:
		}
	}))
	t.Cleanup(func() {
		close(releaseHandler)
		server.Close()
	})
	provider := &tavilySearchProvider{
		apiKey:     "test-key",
		endpoint:   server.URL,
		httpClient: server.Client(),
		maxResults: 5,
	}
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		_, err := provider.Search(ctx, PlaygroundSearchRequest{Query: "q", Depth: "basic"})
		result <- err
	}()
	<-requestStarted
	cancel()

	select {
	case err := <-result:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(2 * time.Second):
		t.Fatal("provider did not stop after request cancellation")
	}
}

func TestApplyPlaygroundSearchFallbackInjectsTavilyContext(t *testing.T) {
	req := &dto.GeneralOpenAIRequest{
		Model: "custom-chat",
		Messages: []dto.Message{
			{Role: "system", Content: "Be precise."},
			{Role: "user", Content: "What changed in Redis 8?"},
		},
		Tools:            []dto.ToolCallRequest{{Type: "web_search_preview"}},
		WebSearchOptions: &dto.WebSearchOptions{SearchContextSize: "low"},
	}
	provider := &fakePlaygroundSearchProvider{results: []PlaygroundSearchResult{
		{Title: "Redis 8 release", URL: "https://redis.io/blog/redis-8", Content: "Redis 8 adds query and vector improvements."},
		{Title: "Docs", URL: "https://redis.io/docs/latest/", Content: "Redis Query Engine is included."},
	}}

	outcome := ApplyPlaygroundSearchFallback(context.Background(), req, PlaygroundSearchSettings{
		Enabled:       true,
		Provider:      provider,
		DefaultDepth:  "advanced",
		MaxResults:    5,
		MaxContextLen: 2000,
	})

	require.Equal(t, PlaygroundSearchOutcomeTavily, outcome.Status)
	require.Equal(t, "What changed in Redis 8?", provider.query)
	require.Equal(t, "basic", provider.depth)
	require.Nil(t, req.WebSearchOptions)
	require.Empty(t, req.Tools)
	require.Len(t, req.Messages, 3)
	require.Equal(t, "system", req.Messages[1].Role)
	contextText, ok := req.Messages[1].Content.(string)
	require.True(t, ok)
	require.Contains(t, contextText, "[1] Redis 8 release")
	require.Contains(t, contextText, "https://redis.io/blog/redis-8")
	require.Contains(t, contextText, "Use these search results")
	require.Len(t, outcome.Sources, 2)
	require.Equal(t, "low", outcome.ContextSize)
}

func TestApplyPlaygroundSearchFallbackReportsUnavailableWithoutInjection(t *testing.T) {
	req := &dto.GeneralOpenAIRequest{
		Model:            "custom-chat",
		Messages:         []dto.Message{{Role: "user", Content: "search this"}},
		WebSearchOptions: &dto.WebSearchOptions{SearchContextSize: "medium"},
	}

	outcome := ApplyPlaygroundSearchFallback(context.Background(), req, PlaygroundSearchSettings{Enabled: false})

	require.Equal(t, PlaygroundSearchOutcomeUnavailable, outcome.Status)
	require.Nil(t, req.WebSearchOptions)
	require.Empty(t, req.Tools)
	require.Len(t, req.Messages, 1)
}

func TestApplyPlaygroundSearchFallbackReportsProviderFailureWithoutLeakingDetails(t *testing.T) {
	req := &dto.GeneralOpenAIRequest{
		Model:            "custom-chat",
		Messages:         []dto.Message{{Role: "user", Content: "search this"}},
		WebSearchOptions: &dto.WebSearchOptions{SearchContextSize: "medium"},
	}

	outcome := ApplyPlaygroundSearchFallback(context.Background(), req, PlaygroundSearchSettings{
		Enabled:  true,
		Provider: &fakePlaygroundSearchProvider{err: errors.New("provider secret exploded")},
	})

	require.Equal(t, PlaygroundSearchOutcomeFailed, outcome.Status)
	require.Equal(t, "search provider failed", outcome.Error)
	require.Len(t, req.Messages, 1)
	require.Error(t, outcome.Cause)
	require.NotContains(t, outcome.Error, "provider secret")
}

func TestPlaygroundSearchCapabilityReportsConfiguredPathsWithoutSecrets(t *testing.T) {
	capability := PlaygroundSearchCapabilityForSettings(PlaygroundSearchSettings{
		Enabled:          true,
		APIKey:           "tvly-secret",
		NativeModels:     []string{"gpt-4.1*"},
		NativeChannelIDs: map[int]struct{}{7: {}},
	})

	require.True(t, capability.Available)
	require.True(t, capability.Native)
	require.True(t, capability.Tavily)
	require.Empty(t, capability.Reason)
	require.NotContains(t, fmt.Sprintf("%+v", capability), "tvly-secret")
}

func TestPlaygroundSearchCapabilityExplainsMissingConfiguration(t *testing.T) {
	capability := PlaygroundSearchCapabilityForSettings(PlaygroundSearchSettings{})

	require.False(t, capability.Available)
	require.False(t, capability.Native)
	require.False(t, capability.Tavily)
	require.Equal(t, PlaygroundSearchUnavailableNotConfigured, capability.Reason)
}

func TestPlaygroundTavilyEnabledPreservesLegacyAPIKeyConfiguration(t *testing.T) {
	common.OptionMapRWMutex.Lock()
	if common.OptionMap == nil {
		common.OptionMap = make(map[string]string)
	}
	previous, existed := common.OptionMap[PlaygroundSearchOptionEnabled]
	delete(common.OptionMap, PlaygroundSearchOptionEnabled)
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		defer common.OptionMapRWMutex.Unlock()
		if existed {
			common.OptionMap[PlaygroundSearchOptionEnabled] = previous
		} else {
			delete(common.OptionMap, PlaygroundSearchOptionEnabled)
		}
	})

	require.True(t, playgroundTavilyEnabled("tvly-existing-key"))
	require.False(t, playgroundTavilyEnabled(""))
}

func TestPlaygroundTavilyEnabledHonorsExplicitDisable(t *testing.T) {
	common.OptionMapRWMutex.Lock()
	if common.OptionMap == nil {
		common.OptionMap = make(map[string]string)
	}
	previous, existed := common.OptionMap[PlaygroundSearchOptionEnabled]
	common.OptionMap[PlaygroundSearchOptionEnabled] = "false"
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		defer common.OptionMapRWMutex.Unlock()
		if existed {
			common.OptionMap[PlaygroundSearchOptionEnabled] = previous
		} else {
			delete(common.OptionMap, PlaygroundSearchOptionEnabled)
		}
	})

	require.False(t, playgroundTavilyEnabled("tvly-existing-key"))
}

func boolPtr(value bool) *bool        { return &value }
func floatPtr(value float64) *float64 { return &value }

type cancellingPlaygroundSearchProvider struct {
	err error
}

func (p *cancellingPlaygroundSearchProvider) Search(ctx context.Context, req PlaygroundSearchRequest) ([]PlaygroundSearchResult, error) {
	p.err = ctx.Err()
	return nil, p.err
}
