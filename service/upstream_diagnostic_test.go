package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
)

func diagnosticResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func diagnosticSnapshot(mode string, previewChars int) operation_setting.UpstreamDiagnosticSnapshot {
	return operation_setting.UpstreamDiagnosticSnapshot{
		Enabled:      true,
		CaptureMode:  mode,
		PreviewChars: previewChars,
	}
}

func TestParseUpstreamErrorKeepsUsefulMessageAndRedactsStructuredPreview(t *testing.T) {
	body := `{"error":{"message":"upstream unavailable","type":"server_error","code":"overloaded"},` +
		`"authorization":"Bearer provider-secret","api_key":"sk-provider-secret",` +
		`"messages":[{"role":"user","content":"private prompt"}],` +
		`"nested":{"refresh_token":"refresh-secret"}}`

	apiErr, diagnostic := ParseUpstreamError(
		diagnosticResponse(http.StatusBadGateway, body),
		diagnosticSnapshot(operation_setting.UpstreamDiagnosticCaptureAllErrors, 2048),
	)

	require.Equal(t, "upstream unavailable", apiErr.Error())
	require.NotNil(t, diagnostic)
	require.True(t, diagnostic.Captured)
	require.Equal(t, "structured_error", diagnostic.ErrorCategory)
	require.Contains(t, diagnostic.Preview, "upstream unavailable")
	require.Contains(t, diagnostic.Preview, "server_error")
	require.Contains(t, diagnostic.Preview, "[REDACTED]")
	require.NotContains(t, diagnostic.Preview, "provider-secret")
	require.NotContains(t, diagnostic.Preview, "private prompt")
	require.NotContains(t, diagnostic.Preview, "refresh-secret")
	require.Len(t, diagnostic.Digest, 64)
}

func TestParseUpstreamErrorFallbackModeSkipsUsefulStructuredPreview(t *testing.T) {
	body := `{"error":{"message":"upstream unavailable","type":"server_error"}}`

	apiErr, diagnostic := ParseUpstreamError(
		diagnosticResponse(http.StatusBadGateway, body),
		diagnosticSnapshot(operation_setting.UpstreamDiagnosticCaptureFallbackOnly, 2048),
	)

	require.Equal(t, "upstream unavailable", apiErr.Error())
	require.NotNil(t, diagnostic)
	require.False(t, diagnostic.Captured)
	require.Empty(t, diagnostic.Preview)
}

func TestParseUpstreamErrorFallbackRedactsTextSecrets(t *testing.T) {
	body := `<html>Authorization: Bearer provider-secret api_key=sk-provider-secret ` +
		`prompt="private prompt" token eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxIn0.signature</html>`

	apiErr, diagnostic := ParseUpstreamError(
		diagnosticResponse(http.StatusBadGateway, body),
		diagnosticSnapshot(operation_setting.UpstreamDiagnosticCaptureFallbackOnly, 2048),
	)

	require.Equal(t, "bad response status code 502", apiErr.Error())
	require.True(t, diagnostic.Captured)
	require.Equal(t, "invalid_json", diagnostic.ErrorCategory)
	require.Contains(t, diagnostic.Preview, "[REDACTED]")
	require.NotContains(t, diagnostic.Preview, "provider-secret")
	require.NotContains(t, diagnostic.Preview, "private prompt")
	require.NotContains(t, diagnostic.Preview, "eyJhbGci")
}

func TestRedactUpstreamDiagnosticFallsBackToTextForMalformedTrailingJSON(t *testing.T) {
	body := []byte(`{"message":"upstream failed"} Authorization: Bearer trailing-secret {`)

	redacted := redactUpstreamDiagnostic(body)

	require.Contains(t, redacted, "upstream failed")
	require.Contains(t, redacted, "[REDACTED]")
	require.NotContains(t, redacted, "trailing-secret")
}

func TestRedactUpstreamDiagnosticMasksSensitiveJSONLikeValuesWhenTruncated(t *testing.T) {
	body := []byte(`{"messages":[{"role":"user","content":"private prompt"}],"authorization":"Basic provider-secret"`)

	redacted := redactUpstreamDiagnostic(body)

	require.Contains(t, redacted, "[REDACTED]")
	require.NotContains(t, redacted, "private prompt")
	require.NotContains(t, redacted, "provider-secret")
}

func TestRedactUpstreamDiagnosticMasksWholePlainTextSensitiveValues(t *testing.T) {
	body := []byte("Authorization: Basic basic-secret\n" +
		"Cookie: session=one; refresh=two\n" +
		"prompt=private multi word prompt\n" +
		"status=502")

	redacted := redactUpstreamDiagnostic(body)

	require.NotContains(t, redacted, "basic-secret")
	require.NotContains(t, redacted, "refresh=two")
	require.NotContains(t, redacted, "private multi word prompt")
	require.Contains(t, redacted, "status=502")
}

func TestParseUpstreamErrorBoundsReadAndUnicodePreview(t *testing.T) {
	body := strings.Repeat("界", upstreamDiagnosticBodyLimitBytes) + "Bearer trailing-secret"

	apiErr, diagnostic := ParseUpstreamError(
		diagnosticResponse(http.StatusInternalServerError, body),
		diagnosticSnapshot(operation_setting.UpstreamDiagnosticCaptureAllErrors, 256),
	)

	require.Equal(t, "bad response status code 500", apiErr.Error())
	require.True(t, diagnostic.Truncated)
	require.EqualValues(t, upstreamDiagnosticBodyLimitBytes+1, diagnostic.RawLength)
	require.LessOrEqual(t, utf8.RuneCountInString(diagnostic.Preview), 256)
	require.True(t, utf8.ValidString(diagnostic.Preview))
	require.Contains(t, diagnostic.Preview, "[truncated]")
}

func TestParseUpstreamErrorTreatsEmptyMessageJSONAsFallback(t *testing.T) {
	apiErr, diagnostic := ParseUpstreamError(
		diagnosticResponse(http.StatusServiceUnavailable, `{"error":{"message":""},"detail":""}`),
		diagnosticSnapshot(operation_setting.UpstreamDiagnosticCaptureFallbackOnly, 2048),
	)

	require.Equal(t, "bad response status code 503", apiErr.Error())
	require.True(t, diagnostic.Captured)
	require.Equal(t, "empty_message", diagnostic.ErrorCategory)
}

func TestDiagnosticSinkCoalescesDuplicatesAndSyncsExistingLogDetails(t *testing.T) {
	now := time.Unix(100, 0)
	var mu sync.Mutex
	var messages []string
	sink := newDiagnosticSink(diagnosticSinkOptions{
		Capacity: 4,
		Now:      func() time.Time { return now },
		Log: func(_ context.Context, message string) error {
			mu.Lock()
			defer mu.Unlock()
			messages = append(messages, message)
			return nil
		},
	})
	snapshot := diagnosticSnapshot(operation_setting.UpstreamDiagnosticCaptureAllErrors, 2048)
	snapshot.SyncToUsageLog = true
	snapshot.DedupeWindowSeconds = 60
	diagnostic := UpstreamDiagnostic{
		ChannelID: 7, Status: http.StatusBadGateway, RequestID: "req-1",
		Preview: "redacted preview", Digest: strings.Repeat("a", 64), Captured: true,
	}

	firstOther := map[string]interface{}{}
	require.NoError(t, sink.Record(context.Background(), diagnostic, firstOther, snapshot))
	require.Contains(t, firstOther, upstreamDiagnosticLogField)
	require.Len(t, messages, 1)

	secondOther := map[string]interface{}{}
	require.NoError(t, sink.Record(context.Background(), diagnostic, secondOther, snapshot))
	require.Contains(t, secondOther, upstreamDiagnosticLogField)
	require.Len(t, messages, 1)

	now = now.Add(61 * time.Second)
	require.NoError(t, sink.Record(context.Background(), diagnostic, map[string]interface{}{}, snapshot))
	require.Len(t, messages, 3)
	require.Contains(t, messages[1], "suppressed=1")
	require.Contains(t, messages[2], "redacted preview")
}

func TestDiagnosticSinkZeroWindowAndCapacityBound(t *testing.T) {
	var messages []string
	sink := newDiagnosticSink(diagnosticSinkOptions{
		Capacity: 2,
		Log: func(_ context.Context, message string) error {
			messages = append(messages, message)
			return nil
		},
	})
	snapshot := diagnosticSnapshot(operation_setting.UpstreamDiagnosticCaptureAllErrors, 2048)
	snapshot.DedupeWindowSeconds = 0

	for i, digest := range []string{"a", "a", "b", "c"} {
		diagnostic := UpstreamDiagnostic{
			ChannelID: 1, Status: 500 + i, Digest: digest, Preview: "preview", Captured: true,
		}
		require.NoError(t, sink.Record(context.Background(), diagnostic, nil, snapshot))
	}

	require.Len(t, messages, 4)
	require.Zero(t, sink.entryCount())

	snapshot.DedupeWindowSeconds = 60
	for i, digest := range []string{"d", "e", "f"} {
		diagnostic := UpstreamDiagnostic{ChannelID: i, Status: 500, Digest: digest, Captured: true}
		require.NoError(t, sink.Record(context.Background(), diagnostic, nil, snapshot))
	}
	require.LessOrEqual(t, sink.entryCount(), 2)
}

func TestDiagnosticSinkCancelsTimerWhenCapacityEvictsEntry(t *testing.T) {
	activeTimers := 0
	cancelledTimers := 0
	sink := newDiagnosticSink(diagnosticSinkOptions{
		Capacity: 2,
		Log:      func(context.Context, string) error { return nil },
		Schedule: func(time.Duration, func()) func() {
			activeTimers++
			return func() {
				activeTimers--
				cancelledTimers++
			}
		},
	})
	snapshot := diagnosticSnapshot(operation_setting.UpstreamDiagnosticCaptureAllErrors, 2048)
	snapshot.DedupeWindowSeconds = 3600

	for _, digest := range []string{"a", "b", "c"} {
		require.NoError(t, sink.Record(context.Background(), UpstreamDiagnostic{
			ChannelID: 1, Status: 500, Digest: digest, Captured: true,
		}, nil, snapshot))
	}

	require.Equal(t, 2, activeTimers)
	require.Equal(t, 1, cancelledTimers)
}

func TestDiagnosticSinkReturnsFailureWithoutMutatingOriginalError(t *testing.T) {
	expected := errors.New("forced diagnostic sink failure")
	sink := newDiagnosticSink(diagnosticSinkOptions{
		Capacity: 1,
		Log: func(context.Context, string) error {
			return expected
		},
	})
	snapshot := diagnosticSnapshot(operation_setting.UpstreamDiagnosticCaptureAllErrors, 2048)
	apiErr, diagnostic := ParseUpstreamError(
		diagnosticResponse(http.StatusBadGateway, `<html>bad gateway</html>`),
		snapshot,
	)
	originalMessage := apiErr.Error()

	err := sink.Record(context.Background(), *diagnostic, nil, snapshot)

	require.ErrorIs(t, err, expected)
	require.Equal(t, originalMessage, apiErr.Error())
}

func TestRecordUpstreamDiagnosticExtractsCarrierAndAddsAttemptIdentity(t *testing.T) {
	originalSink := defaultUpstreamDiagnosticSink
	t.Cleanup(func() { defaultUpstreamDiagnosticSink = originalSink })
	defaultUpstreamDiagnosticSink = newDiagnosticSink(diagnosticSinkOptions{
		Capacity: 2,
		Log:      func(context.Context, string) error { return nil },
	})
	snapshot := diagnosticSnapshot(operation_setting.UpstreamDiagnosticCaptureAllErrors, 2048)
	snapshot.SyncToUsageLog = true
	snapshot.DedupeWindowSeconds = 0
	apiErr := RelayErrorHandler(
		context.Background(),
		diagnosticResponse(http.StatusBadGateway, `<html>bad gateway</html>`),
		false,
	)
	other := map[string]interface{}{}

	require.NoError(t, RecordUpstreamDiagnostic(
		context.Background(), apiErr, 42, "request-42", other, snapshot,
	))

	value, ok := other[upstreamDiagnosticLogField].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, 42, value["channel_id"])
	require.Equal(t, "request-42", value["request_id"])
}

func TestNormalizeViolationFeeErrorPreservesUpstreamDiagnosticCarrier(t *testing.T) {
	apiErr := RelayErrorHandler(
		context.Background(),
		diagnosticResponse(http.StatusBadRequest, `{"error":{"message":"Failed check: SAFETY_CHECK_TYPE"}}`),
		false,
	)
	expected, ok := UpstreamDiagnosticFromError(apiErr)
	require.True(t, ok)

	normalized := NormalizeViolationFeeError(apiErr)

	actual, ok := UpstreamDiagnosticFromError(normalized)
	require.True(t, ok)
	require.Equal(t, expected.Digest, actual.Digest)
	require.Equal(t, types.ErrorCodeViolationFeeGrokCSAM, normalized.GetErrorCode())
}
