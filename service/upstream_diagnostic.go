package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"
)

const (
	upstreamDiagnosticBodyLimitBytes = 64 << 10
	upstreamDiagnosticLogField       = "upstream_diagnostic"
	defaultDiagnosticSinkCapacity    = 1024
)

type UpstreamDiagnostic struct {
	ChannelID     int    `json:"channel_id"`
	Status        int    `json:"status"`
	RequestID     string `json:"request_id,omitempty"`
	ErrorCategory string `json:"error_category"`
	Preview       string `json:"preview,omitempty"`
	Digest        string `json:"digest"`
	RawLength     int64  `json:"raw_length"`
	Truncated     bool   `json:"truncated"`
	Captured      bool   `json:"-"`
}

var (
	diagnosticBearerPattern = regexp.MustCompile(`(?i)\bbearer\s+[a-z0-9._~+/=-]+`)
	diagnosticJWTpattern    = regexp.MustCompile(`\beyJ[a-zA-Z0-9_-]+\.[a-zA-Z0-9_-]+\.[a-zA-Z0-9_-]+\b`)
	diagnosticKeyPattern    = regexp.MustCompile(`(?i)\b(?:sk|rk|pk)-[a-z0-9_-]{8,}\b`)
	diagnosticHeaderPattern = regexp.MustCompile(`(?i)\b(authorization|proxy-authorization|cookie|set-cookie|api[_ -]?key)\s*[:=]\s*[^\r\n<>]+`)
	diagnosticPromptPattern = regexp.MustCompile(`(?i)\b(prompt|input|messages|request_body)\b\s*[:=]\s*[^\r\n<>]+`)
)

var diagnosticSensitiveKeys = map[string]struct{}{
	"authorization":      {},
	"proxyauthorization": {},
	"cookie":             {},
	"setcookie":          {},
	"apikey":             {},
	"password":           {},
	"secret":             {},
	"accesstoken":        {},
	"refreshtoken":       {},
	"prompt":             {},
	"input":              {},
	"messages":           {},
	"requestbody":        {},
}

func redactUpstreamDiagnostic(body []byte) string {
	var decoded any
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if err := decoder.Decode(&decoded); err == nil {
		var trailing any
		if trailingErr := decoder.Decode(&trailing); trailingErr == io.EOF {
			redactDiagnosticValue(decoded)
			if encoded, marshalErr := json.Marshal(decoded); marshalErr == nil {
				return redactDiagnosticText(string(encoded))
			}
		}
	}
	return redactDiagnosticText(strings.ToValidUTF8(string(body), "\uFFFD"))
}

func redactDiagnosticValue(value any) {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if isDiagnosticSensitiveKey(key) {
				typed[key] = "[REDACTED]"
				continue
			}
			redactDiagnosticValue(child)
			if text, ok := typed[key].(string); ok {
				typed[key] = redactDiagnosticText(text)
			}
		}
	case []any:
		for index, child := range typed {
			redactDiagnosticValue(child)
			if text, ok := typed[index].(string); ok {
				typed[index] = redactDiagnosticText(text)
			}
		}
	}
}

func isDiagnosticSensitiveKey(key string) bool {
	normalized := strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return unicode.ToLower(r)
		}
		return -1
	}, key)
	_, sensitive := diagnosticSensitiveKeys[normalized]
	return sensitive
}

func redactDiagnosticText(value string) string {
	value = redactJSONLikeSensitiveValues(value)
	value = diagnosticPromptPattern.ReplaceAllString(value, "$1=[REDACTED]")
	value = diagnosticHeaderPattern.ReplaceAllString(value, "$1=[REDACTED]")
	value = diagnosticBearerPattern.ReplaceAllString(value, "Bearer [REDACTED]")
	value = diagnosticJWTpattern.ReplaceAllString(value, "[REDACTED]")
	value = diagnosticKeyPattern.ReplaceAllString(value, "[REDACTED]")
	return value
}

func redactJSONLikeSensitiveValues(value string) string {
	var redacted strings.Builder
	last := 0
	for cursor := 0; cursor < len(value); {
		if value[cursor] != '"' {
			cursor++
			continue
		}
		keyEnd, complete := diagnosticJSONStringEnd(value, cursor)
		if !complete {
			break
		}
		separator := keyEnd
		for separator < len(value) && isDiagnosticJSONWhitespace(value[separator]) {
			separator++
		}
		key := value[cursor+1 : keyEnd-1]
		if separator >= len(value) || value[separator] != ':' || !isDiagnosticSensitiveKey(key) {
			cursor = keyEnd
			continue
		}

		valueStart := separator + 1
		for valueStart < len(value) && isDiagnosticJSONWhitespace(value[valueStart]) {
			valueStart++
		}
		valueEnd := diagnosticJSONLikeValueEnd(value, valueStart)
		redacted.WriteString(value[last:valueStart])
		redacted.WriteString(`"[REDACTED]"`)
		last = valueEnd
		cursor = valueEnd
	}
	if last == 0 {
		return value
	}
	redacted.WriteString(value[last:])
	return redacted.String()
}

func diagnosticJSONStringEnd(value string, start int) (int, bool) {
	for cursor := start + 1; cursor < len(value); cursor++ {
		switch value[cursor] {
		case '\\':
			cursor++
		case '"':
			return cursor + 1, true
		}
	}
	return len(value), false
}

func diagnosticJSONLikeValueEnd(value string, start int) int {
	if start >= len(value) {
		return len(value)
	}
	if value[start] == '"' {
		if end, complete := diagnosticJSONStringEnd(value, start); complete {
			return end
		}
		return len(value)
	}
	if value[start] == '{' || value[start] == '[' {
		stack := []byte{value[start]}
		for cursor := start + 1; cursor < len(value); cursor++ {
			switch value[cursor] {
			case '"':
				end, complete := diagnosticJSONStringEnd(value, cursor)
				if !complete {
					return len(value)
				}
				cursor = end - 1
			case '{', '[':
				stack = append(stack, value[cursor])
			case '}', ']':
				opening := stack[len(stack)-1]
				if (opening == '{' && value[cursor] == '}') || (opening == '[' && value[cursor] == ']') {
					stack = stack[:len(stack)-1]
					if len(stack) == 0 {
						return cursor + 1
					}
				}
			}
		}
		return len(value)
	}
	for cursor := start; cursor < len(value); cursor++ {
		switch value[cursor] {
		case ',', '}', ']', '\r', '\n':
			return cursor
		}
	}
	return len(value)
}

func isDiagnosticJSONWhitespace(value byte) bool {
	return value == ' ' || value == '\t' || value == '\r' || value == '\n'
}

func truncateDiagnosticPreview(value string, limit int, bodyTruncated bool) string {
	if limit < 1 {
		return ""
	}
	value = strings.ToValidUTF8(value, "\uFFFD")
	runes := []rune(value)
	needsSuffix := bodyTruncated || len(runes) > limit
	if !needsSuffix {
		return value
	}
	suffix := []rune("... [truncated]")
	if len(suffix) >= limit {
		return string(suffix[:limit])
	}
	keep := limit - len(suffix)
	if keep > len(runes) {
		keep = len(runes)
	}
	return string(runes[:keep]) + string(suffix)
}

func diagnosticDigest(body []byte) string {
	digest := sha256.Sum256(body)
	return hex.EncodeToString(digest[:])
}

type diagnosticDedupeKey struct {
	ChannelID int
	Status    int
	Digest    string
}

type diagnosticDedupeEntry struct {
	ExpiresAt  time.Time
	Suppressed int
	RequestID  string
	Cancel     func()
}

type diagnosticSinkOptions struct {
	Capacity int
	Now      func() time.Time
	Log      func(context.Context, string) error
	Schedule func(time.Duration, func()) func()
}

type DiagnosticSink struct {
	mu       sync.Mutex
	entries  map[diagnosticDedupeKey]diagnosticDedupeEntry
	capacity int
	now      func() time.Time
	log      func(context.Context, string) error
	schedule func(time.Duration, func()) func()
}

func newDiagnosticSink(options diagnosticSinkOptions) *DiagnosticSink {
	capacity := options.Capacity
	if capacity <= 0 {
		capacity = defaultDiagnosticSinkCapacity
	}
	now := options.Now
	schedule := options.Schedule
	if now == nil {
		now = time.Now
		if schedule == nil {
			schedule = func(delay time.Duration, fn func()) func() {
				timer := time.AfterFunc(delay, fn)
				return func() { timer.Stop() }
			}
		}
	}
	logFn := options.Log
	if logFn == nil {
		logFn = func(ctx context.Context, message string) error {
			logger.LogError(ctx, message)
			return nil
		}
	}
	return &DiagnosticSink{
		entries:  make(map[diagnosticDedupeKey]diagnosticDedupeEntry),
		capacity: capacity,
		now:      now,
		log:      logFn,
		schedule: schedule,
	}
}

func (s *DiagnosticSink) entryCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.entries)
}

func (s *DiagnosticSink) Record(ctx context.Context, diagnostic UpstreamDiagnostic, other map[string]interface{}, snapshot operation_setting.UpstreamDiagnosticSnapshot) error {
	if snapshot.SyncToUsageLog && other != nil {
		other[upstreamDiagnosticLogField] = diagnostic.logValue(snapshot.Enabled && diagnostic.Captured)
	}
	if snapshot.Enabled && !diagnostic.Captured {
		return nil
	}

	message := diagnostic.logMessage(snapshot.Enabled && diagnostic.Captured)
	window := time.Duration(snapshot.DedupeWindowSeconds) * time.Second
	if window <= 0 {
		return s.log(ctx, message)
	}

	now := s.now()
	key := diagnosticDedupeKey{ChannelID: diagnostic.ChannelID, Status: diagnostic.Status, Digest: diagnostic.Digest}
	var pending []string
	shouldLog := false

	s.mu.Lock()
	pending = append(pending, s.collectExpiredLocked(now)...)
	if entry, exists := s.entries[key]; exists {
		entry.Suppressed++
		s.entries[key] = entry
	} else {
		if len(s.entries) >= s.capacity {
			pending = append(pending, s.evictOldestLocked()...)
		}
		expiresAt := now.Add(window)
		entry := diagnosticDedupeEntry{ExpiresAt: expiresAt, RequestID: diagnostic.RequestID}
		s.entries[key] = entry
		shouldLog = true
		if s.schedule != nil {
			entry.Cancel = s.schedule(window, func() { s.expireScheduled(key, expiresAt) })
			s.entries[key] = entry
		}
	}
	s.mu.Unlock()

	for _, summary := range pending {
		if err := s.log(ctx, summary); err != nil {
			return err
		}
	}
	if shouldLog {
		return s.log(ctx, message)
	}
	return nil
}

func (s *DiagnosticSink) collectExpiredLocked(now time.Time) []string {
	var summaries []string
	for key, entry := range s.entries {
		if now.Before(entry.ExpiresAt) {
			continue
		}
		if entry.Cancel != nil {
			entry.Cancel()
		}
		delete(s.entries, key)
		if entry.Suppressed > 0 {
			summaries = append(summaries, diagnosticSummaryMessage(key, entry))
		}
	}
	return summaries
}

func (s *DiagnosticSink) evictOldestLocked() []string {
	var oldestKey diagnosticDedupeKey
	var oldest diagnosticDedupeEntry
	found := false
	for key, entry := range s.entries {
		if !found || entry.ExpiresAt.Before(oldest.ExpiresAt) {
			oldestKey = key
			oldest = entry
			found = true
		}
	}
	if !found {
		return nil
	}
	if oldest.Cancel != nil {
		oldest.Cancel()
	}
	delete(s.entries, oldestKey)
	if oldest.Suppressed == 0 {
		return nil
	}
	return []string{diagnosticSummaryMessage(oldestKey, oldest)}
}

func (s *DiagnosticSink) expireScheduled(key diagnosticDedupeKey, expiresAt time.Time) {
	s.mu.Lock()
	entry, exists := s.entries[key]
	if !exists || !entry.ExpiresAt.Equal(expiresAt) || s.now().Before(expiresAt) {
		s.mu.Unlock()
		return
	}
	delete(s.entries, key)
	s.mu.Unlock()
	if entry.Suppressed > 0 {
		_ = s.log(context.Background(), diagnosticSummaryMessage(key, entry))
	}
}

func diagnosticSummaryMessage(key diagnosticDedupeKey, entry diagnosticDedupeEntry) string {
	return fmt.Sprintf(
		"upstream diagnostic duplicates channel_id=%d status=%d digest=%s request_id=%s suppressed=%d",
		key.ChannelID, key.Status, key.Digest, entry.RequestID, entry.Suppressed,
	)
}

func (diagnostic UpstreamDiagnostic) logMessage(includePreview bool) string {
	message := fmt.Sprintf(
		"upstream diagnostic channel_id=%d status=%d request_id=%s category=%s digest=%s raw_length=%d truncated=%t",
		diagnostic.ChannelID,
		diagnostic.Status,
		diagnostic.RequestID,
		diagnostic.ErrorCategory,
		diagnostic.Digest,
		diagnostic.RawLength,
		diagnostic.Truncated,
	)
	if includePreview && diagnostic.Preview != "" {
		message += fmt.Sprintf(" preview=%q", diagnostic.Preview)
	}
	return message
}

func (diagnostic UpstreamDiagnostic) logValue(includePreview bool) map[string]interface{} {
	value := map[string]interface{}{
		"channel_id":     diagnostic.ChannelID,
		"status":         diagnostic.Status,
		"request_id":     diagnostic.RequestID,
		"error_category": diagnostic.ErrorCategory,
		"digest":         diagnostic.Digest,
		"raw_length":     diagnostic.RawLength,
		"truncated":      diagnostic.Truncated,
	}
	if includePreview && diagnostic.Preview != "" {
		value["preview"] = diagnostic.Preview
	}
	return value
}

var defaultUpstreamDiagnosticSink = newDiagnosticSink(diagnosticSinkOptions{})

func RecordUpstreamDiagnostic(
	ctx context.Context,
	apiErr *types.NewAPIError,
	channelID int,
	requestID string,
	other map[string]interface{},
	snapshot operation_setting.UpstreamDiagnosticSnapshot,
) error {
	diagnostic, ok := UpstreamDiagnosticFromError(apiErr)
	if !ok {
		return nil
	}
	diagnostic.ChannelID = channelID
	diagnostic.RequestID = requestID
	return defaultUpstreamDiagnosticSink.Record(ctx, diagnostic, other, snapshot)
}
