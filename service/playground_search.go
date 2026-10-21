package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
)

const (
	PlaygroundSearchOptionEnabled          = "PlaygroundTavilyEnabled"
	PlaygroundSearchOptionAPIKey           = "PlaygroundTavilyAPIKey"
	PlaygroundSearchOptionDefaultDepth     = "PlaygroundTavilyDefaultDepth"
	PlaygroundSearchOptionMaxResults       = "PlaygroundTavilyMaxResults"
	PlaygroundSearchOptionMaxContextLength = "PlaygroundTavilyMaxContextLength"
	PlaygroundSearchOptionNativeModels     = "PlaygroundNativeWebSearchModels"
	PlaygroundSearchOptionNativeChannels   = "PlaygroundNativeWebSearchChannels"
)

const (
	defaultTavilySearchEndpoint     = "https://api.tavily.com/search"
	maxTavilyResponseBytes          = 1 << 20
	maxSearchTitleRunes             = 200
	maxSearchURLRunes               = 2048
	maxSearchSnippetRunes           = 800
	maxPlaygroundModelPolicyBytes   = 20000
	maxPlaygroundModelPolicyEntries = 256
	maxPlaygroundModelPatternBytes  = 255
)

const (
	PlaygroundSearchOutcomeNone        = "none"
	PlaygroundSearchOutcomeNative      = "native"
	PlaygroundSearchOutcomeTavily      = "tavily"
	PlaygroundSearchOutcomeUnavailable = "unavailable"
	PlaygroundSearchOutcomeFailed      = "failed"

	PlaygroundSearchUnavailableNotConfigured = "not_configured"
)

var (
	ErrPlaygroundSearchUnavailable = errors.New("playground search is not configured")
	ErrPlaygroundSearchNoResults   = errors.New("playground search returned no usable results")
	ErrPlaygroundSearchFailed      = errors.New("playground search provider failed")
)

type PlaygroundSearchRequest struct {
	Query string
	Depth string
}

type PlaygroundSearchResult struct {
	Title   string
	URL     string
	Content string
}

type PlaygroundSearchProvider interface {
	Search(ctx context.Context, req PlaygroundSearchRequest) ([]PlaygroundSearchResult, error)
}

type PlaygroundSearchSettings struct {
	Enabled            bool
	APIKey             string
	DefaultDepth       string
	NativeModels       []string
	NativeChannelIDs   map[int]struct{}
	NativeRouteCapable bool
	MaxResults         int
	MaxContextLen      int
	Timeout            time.Duration
	Provider           PlaygroundSearchProvider
}

type PlaygroundSearchOutcome struct {
	Status      string
	ContextSize string
	Sources     []PlaygroundSearchResult
	Error       string
	Cause       error
}

type PlaygroundSearchCapability struct {
	Available bool   `json:"available"`
	Native    bool   `json:"native"`
	Tavily    bool   `json:"tavily"`
	Reason    string `json:"reason"`
}

func PlaygroundSearchCapabilityForSettings(settings PlaygroundSearchSettings) PlaygroundSearchCapability {
	native := len(settings.NativeModels) > 0 && len(settings.NativeChannelIDs) > 0
	tavily := settings.Enabled && strings.TrimSpace(settings.APIKey) != ""
	capability := PlaygroundSearchCapability{
		Available: native || tavily,
		Native:    native,
		Tavily:    tavily,
	}
	if !capability.Available {
		capability.Reason = PlaygroundSearchUnavailableNotConfigured
	}
	return capability
}

func GetPlaygroundSearchCapabilityFromOptions() PlaygroundSearchCapability {
	return PlaygroundSearchCapabilityForSettings(GetPlaygroundSearchSettingsFromOptions())
}

func ValidatePlaygroundModelListOption(value string) error {
	if len(value) > maxPlaygroundModelPolicyBytes {
		return fmt.Errorf("value exceeds %d bytes", maxPlaygroundModelPolicyBytes)
	}
	entries := splitOptionList(value)
	if len(entries) > maxPlaygroundModelPolicyEntries {
		return fmt.Errorf("value exceeds %d entries", maxPlaygroundModelPolicyEntries)
	}
	for _, entry := range entries {
		if len(entry) > maxPlaygroundModelPatternBytes {
			return fmt.Errorf("entry exceeds %d bytes", maxPlaygroundModelPatternBytes)
		}
	}
	return nil
}

type tavilySearchProvider struct {
	apiKey     string
	endpoint   string
	httpClient *http.Client
	maxResults int
}

type tavilySearchResponse struct {
	Results []struct {
		Title   string `json:"title"`
		URL     string `json:"url"`
		Content string `json:"content"`
	} `json:"results"`
}

func GetPlaygroundSearchSettingsFromOptions() PlaygroundSearchSettings {
	apiKey := getOptionString(PlaygroundSearchOptionAPIKey)
	settings := PlaygroundSearchSettings{
		Enabled:      playgroundTavilyEnabled(apiKey),
		APIKey:       apiKey,
		DefaultDepth: normalizeTavilyDepth(getOptionString(PlaygroundSearchOptionDefaultDepth)),
		NativeModels: splitOptionList(getOptionString(PlaygroundSearchOptionNativeModels)),
		NativeChannelIDs: parsePlaygroundNativeSearchChannels(
			getOptionString(PlaygroundSearchOptionNativeChannels),
		),
		MaxResults:    parseIntOption(PlaygroundSearchOptionMaxResults, 5, 1, 10),
		MaxContextLen: parseIntOption(PlaygroundSearchOptionMaxContextLength, 4000, 500, 20000),
		Timeout:       6 * time.Second,
	}
	if settings.DefaultDepth == "" {
		settings.DefaultDepth = "basic"
	}
	if settings.Enabled && settings.APIKey != "" {
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.DialContext = (&net.Dialer{
			Timeout:   2 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext
		transport.ResponseHeaderTimeout = 4 * time.Second
		transport.TLSHandshakeTimeout = 3 * time.Second
		settings.Provider = &tavilySearchProvider{
			apiKey:   settings.APIKey,
			endpoint: defaultTavilySearchEndpoint,
			httpClient: &http.Client{
				Transport: transport,
				Timeout:   settings.Timeout,
			},
			maxResults: settings.MaxResults,
		}
	}
	return settings
}

func ValidatePlaygroundNativeSearchChannelsOption(value string) error {
	if len(value) > maxPlaygroundModelPolicyBytes {
		return fmt.Errorf("value exceeds %d bytes", maxPlaygroundModelPolicyBytes)
	}
	entries := splitOptionList(value)
	if len(entries) > maxPlaygroundModelPolicyEntries {
		return fmt.Errorf("value exceeds %d entries", maxPlaygroundModelPolicyEntries)
	}
	for _, entry := range entries {
		channelID, err := strconv.Atoi(entry)
		if err != nil || channelID <= 0 {
			return fmt.Errorf("invalid channel id %q", entry)
		}
	}
	return nil
}

func parsePlaygroundNativeSearchChannels(value string) map[int]struct{} {
	channelIDs := make(map[int]struct{})
	for _, entry := range splitOptionList(value) {
		channelID, err := strconv.Atoi(entry)
		if err != nil || channelID <= 0 {
			continue
		}
		channelIDs[channelID] = struct{}{}
	}
	return channelIDs
}

func ApplyPlaygroundSearchFallback(ctx context.Context, req *dto.GeneralOpenAIRequest, settings PlaygroundSearchSettings) PlaygroundSearchOutcome {
	if req == nil || !requestWantsWebSearch(req) {
		return PlaygroundSearchOutcome{Status: PlaygroundSearchOutcomeNone}
	}

	contextSize := playgroundSearchContextSize(req)
	if settings.NativeRouteCapable && modelSupportsNativeSearch(req.Model, settings.NativeModels) {
		return PlaygroundSearchOutcome{Status: PlaygroundSearchOutcomeNative, ContextSize: contextSize}
	}

	removeNativeWebSearchFields(req)
	if !settings.Enabled || settings.Provider == nil {
		return PlaygroundSearchOutcome{
			Status: PlaygroundSearchOutcomeUnavailable, ContextSize: contextSize,
			Error: ErrPlaygroundSearchUnavailable.Error(), Cause: ErrPlaygroundSearchUnavailable,
		}
	}

	query := extractPlaygroundSearchQuery(req.Messages)
	if strings.TrimSpace(query) == "" {
		return PlaygroundSearchOutcome{
			Status: PlaygroundSearchOutcomeUnavailable, ContextSize: contextSize,
			Error: ErrPlaygroundSearchNoResults.Error(), Cause: ErrPlaygroundSearchNoResults,
		}
	}

	results, err := settings.Provider.Search(ctx, PlaygroundSearchRequest{
		Query: query,
		Depth: playgroundRequestedSearchDepth(contextSize, settings.DefaultDepth),
	})
	if err != nil {
		return PlaygroundSearchOutcome{
			Status: PlaygroundSearchOutcomeFailed, ContextSize: contextSize,
			Error: "search provider failed", Cause: fmt.Errorf("%w: %v", ErrPlaygroundSearchFailed, err),
		}
	}
	if len(results) == 0 {
		return PlaygroundSearchOutcome{
			Status: PlaygroundSearchOutcomeUnavailable, ContextSize: contextSize,
			Error: ErrPlaygroundSearchNoResults.Error(), Cause: ErrPlaygroundSearchNoResults,
		}
	}

	limited := limitPlaygroundSearchResults(normalizePlaygroundSearchResults(results), settings.MaxResults)
	contextMessage, injectedSources := buildPlaygroundSearchContext(limited, settings.MaxContextLen)
	if contextMessage == "" || len(injectedSources) == 0 {
		return PlaygroundSearchOutcome{
			Status: PlaygroundSearchOutcomeUnavailable, ContextSize: contextSize,
			Error: ErrPlaygroundSearchNoResults.Error(), Cause: ErrPlaygroundSearchNoResults,
		}
	}
	injectPlaygroundSearchContext(req, contextMessage)

	return PlaygroundSearchOutcome{
		Status:      PlaygroundSearchOutcomeTavily,
		ContextSize: contextSize,
		Sources:     injectedSources,
	}
}

func (p *tavilySearchProvider) Search(ctx context.Context, req PlaygroundSearchRequest) ([]PlaygroundSearchResult, error) {
	if p == nil || p.apiKey == "" {
		return nil, errors.New("tavily api key is not configured")
	}

	payload := map[string]any{
		"api_key":        p.apiKey,
		"query":          req.Query,
		"search_depth":   normalizeTavilyDepth(req.Depth),
		"max_results":    p.maxResults,
		"include_answer": false,
		"include_images": false,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	endpoint := strings.TrimSpace(p.endpoint)
	if endpoint == "" {
		endpoint = defaultTavilySearchEndpoint
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	client := p.httpClient
	if client == nil {
		client = &http.Client{Timeout: 6 * time.Second}
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("tavily returned status %d", resp.StatusCode)
	}

	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, maxTavilyResponseBytes+1))
	if err != nil {
		return nil, err
	}
	if len(responseBody) > maxTavilyResponseBytes {
		return nil, errors.New("tavily response exceeds size limit")
	}
	var parsed tavilySearchResponse
	if err := json.Unmarshal(responseBody, &parsed); err != nil {
		return nil, err
	}

	results := make([]PlaygroundSearchResult, 0, len(parsed.Results))
	for _, item := range parsed.Results {
		results = append(results, PlaygroundSearchResult{
			Title:   item.Title,
			URL:     item.URL,
			Content: item.Content,
		})
	}
	return limitPlaygroundSearchResults(normalizePlaygroundSearchResults(results), p.maxResults), nil
}

func requestWantsWebSearch(req *dto.GeneralOpenAIRequest) bool {
	if req.WebSearchOptions != nil {
		return true
	}
	for _, tool := range req.Tools {
		if tool.Type == "web_search_preview" || tool.Type == "web_search" {
			return true
		}
	}
	return false
}

func playgroundSearchContextSize(req *dto.GeneralOpenAIRequest) string {
	if req.WebSearchOptions != nil && req.WebSearchOptions.SearchContextSize != "" {
		return req.WebSearchOptions.SearchContextSize
	}
	return "medium"
}

func removeNativeWebSearchFields(req *dto.GeneralOpenAIRequest) {
	req.WebSearchOptions = nil
	if len(req.Tools) == 0 {
		return
	}
	filtered := req.Tools[:0]
	for _, tool := range req.Tools {
		if tool.Type == "web_search_preview" || tool.Type == "web_search" {
			continue
		}
		filtered = append(filtered, tool)
	}
	req.Tools = filtered
}

func extractPlaygroundSearchQuery(messages []dto.Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		message := messages[i]
		if message.Role != "user" {
			continue
		}
		return messageContentToText(message.Content)
	}
	return ""
}

func messageContentToText(content any) string {
	switch value := content.(type) {
	case string:
		return strings.TrimSpace(value)
	case []any:
		parts := make([]string, 0, len(value))
		for _, item := range value {
			if itemMap, ok := item.(map[string]any); ok {
				if text, ok := itemMap["text"].(string); ok && strings.TrimSpace(text) != "" {
					parts = append(parts, strings.TrimSpace(text))
				}
			}
		}
		return strings.Join(parts, "\n")
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", value))
	}
}

func injectPlaygroundSearchContext(req *dto.GeneralOpenAIRequest, contextMessage string) {
	insertAt := 0
	if len(req.Messages) > 0 && req.Messages[0].Role == "system" {
		insertAt = 1
	}
	searchMessage := dto.Message{Role: "system", Content: contextMessage}
	req.Messages = append(req.Messages, dto.Message{})
	copy(req.Messages[insertAt+1:], req.Messages[insertAt:])
	req.Messages[insertAt] = searchMessage
}

func buildPlaygroundSearchContext(results []PlaygroundSearchResult, maxLen int) (string, []PlaygroundSearchResult) {
	if maxLen <= 0 {
		maxLen = 4000
	}
	var builder strings.Builder
	prefix := "Use these search results as factual reference. Cite sources using [1], [2], etc. If the results do not answer the question, say that the web search did not provide enough evidence.\n\n"
	if len(prefix) >= maxLen {
		return "", nil
	}
	builder.WriteString(prefix)
	injected := make([]PlaygroundSearchResult, 0, len(results))
	for index, result := range results {
		title := strings.TrimSpace(result.Title)
		if title == "" {
			title = "Untitled"
		}
		line := fmt.Sprintf("[%d] %s\nURL: %s\nSnippet: %s\n\n", index+1, title, strings.TrimSpace(result.URL), strings.TrimSpace(result.Content))
		remaining := maxLen - builder.Len()
		if remaining <= 0 {
			break
		}
		piece := truncateUTF8Bytes(line, remaining)
		if strings.TrimSpace(piece) == "" {
			break
		}
		builder.WriteString(piece)
		injected = append(injected, result)
		if len(piece) < len(line) {
			break
		}
	}
	return strings.TrimSpace(builder.String()), injected
}

func buildPlaygroundSearchContextMessage(results []PlaygroundSearchResult, maxLen int) string {
	message, _ := buildPlaygroundSearchContext(results, maxLen)
	return message
}

func limitPlaygroundSearchResults(results []PlaygroundSearchResult, maxResults int) []PlaygroundSearchResult {
	if maxResults <= 0 {
		maxResults = 5
	}
	if len(results) <= maxResults {
		return results
	}
	return results[:maxResults]
}

func modelSupportsNativeSearch(model string, patterns []string) bool {
	model = strings.ToLower(strings.TrimSpace(model))
	if model == "" {
		return false
	}
	for _, pattern := range patterns {
		pattern = strings.ToLower(strings.TrimSpace(pattern))
		if pattern == "" {
			continue
		}
		if strings.HasSuffix(pattern, "*") && strings.HasPrefix(model, strings.TrimSuffix(pattern, "*")) {
			return true
		}
		if model == pattern {
			return true
		}
	}
	return false
}

func normalizeTavilyDepth(depth string) string {
	switch strings.ToLower(strings.TrimSpace(depth)) {
	case "advanced":
		return "advanced"
	case "basic":
		return "basic"
	default:
		return "basic"
	}
}

func playgroundRequestedSearchDepth(contextSize string, fallback string) string {
	switch strings.ToLower(strings.TrimSpace(contextSize)) {
	case "low":
		return "basic"
	case "high":
		return "advanced"
	default:
		return normalizeTavilyDepth(fallback)
	}
}

func normalizePlaygroundSearchResults(results []PlaygroundSearchResult) []PlaygroundSearchResult {
	normalized := make([]PlaygroundSearchResult, 0, len(results))
	for _, result := range results {
		safeURL := normalizePlaygroundSearchURL(result.URL)
		if safeURL == "" {
			continue
		}
		normalized = append(normalized, PlaygroundSearchResult{
			Title:   normalizePlaygroundSearchText(result.Title, maxSearchTitleRunes),
			URL:     safeURL,
			Content: normalizePlaygroundSearchText(result.Content, maxSearchSnippetRunes),
		})
	}
	return normalized
}

func normalizePlaygroundSearchURL(raw string) string {
	raw = normalizePlaygroundSearchText(raw, maxSearchURLRunes)
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "https" && parsed.Scheme != "http") {
		return ""
	}
	parsed.User = nil
	parsed.Fragment = ""
	return parsed.String()
}

func normalizePlaygroundSearchText(value string, maxRunes int) string {
	value = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && !unicode.IsSpace(r) {
			return -1
		}
		return r
	}, value)
	value = strings.Join(strings.Fields(value), " ")
	if maxRunes <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) > maxRunes {
		runes = runes[:maxRunes]
	}
	return string(runes)
}

func truncateUTF8Bytes(value string, maxBytes int) string {
	if maxBytes <= 0 {
		return ""
	}
	if len(value) <= maxBytes {
		return value
	}
	value = value[:maxBytes]
	for !utf8.ValidString(value) && len(value) > 0 {
		value = value[:len(value)-1]
	}
	return value
}

func getOptionString(key string) string {
	common.OptionMapRWMutex.RLock()
	defer common.OptionMapRWMutex.RUnlock()
	return common.OptionMap[key]
}

func playgroundTavilyEnabled(apiKey string) bool {
	common.OptionMapRWMutex.RLock()
	value, configured := common.OptionMap[PlaygroundSearchOptionEnabled]
	common.OptionMapRWMutex.RUnlock()
	if !configured {
		// Older installations enabled Tavily by storing only the API key.
		return strings.TrimSpace(apiKey) != ""
	}
	return parseBoolValue(value, false)
}

func parseBoolOption(key string, fallback bool) bool {
	return parseBoolValue(getOptionString(key), fallback)
}

func parseBoolValue(value string, fallback bool) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "true", "1", "yes", "on":
		return true
	case "false", "0", "no", "off":
		return false
	default:
		return fallback
	}
}

func parseIntOption(key string, fallback int, minValue int, maxValue int) int {
	value := strings.TrimSpace(getOptionString(key))
	if value == "" {
		return fallback
	}
	var parsed int
	if _, err := fmt.Sscanf(value, "%d", &parsed); err != nil {
		return fallback
	}
	if parsed < minValue {
		return minValue
	}
	if parsed > maxValue {
		return maxValue
	}
	return parsed
}

func splitOptionList(value string) []string {
	fields := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == '\n' || r == ';'
	})
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		trimmed := strings.TrimSpace(field)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
