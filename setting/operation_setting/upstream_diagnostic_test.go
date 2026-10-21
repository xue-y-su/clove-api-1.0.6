package operation_setting

import (
	"testing"

	"github.com/QuantumNous/new-api/setting/config"
	"github.com/stretchr/testify/require"
)

func TestUpstreamDiagnosticSettingDefaultsAndRoundTrip(t *testing.T) {
	previous := *GetUpstreamDiagnosticSetting()
	t.Cleanup(func() {
		*GetUpstreamDiagnosticSetting() = previous
		RebuildUpstreamDiagnosticSnapshot()
	})

	setting := GetUpstreamDiagnosticSetting()
	require.True(t, setting.Enabled)
	require.Equal(t, UpstreamDiagnosticCaptureFallbackOnly, setting.CaptureMode)
	require.Equal(t, 2048, setting.PreviewChars)
	require.False(t, setting.SyncToUsageLog)
	require.Equal(t, 60, setting.DedupeWindowSeconds)
	require.Same(t, setting, config.GlobalConfig.Get(UpstreamDiagnosticSettingConfigName))

	require.NoError(t, config.UpdateConfigFromMap(setting, map[string]string{
		"enabled":               "false",
		"capture_mode":          UpstreamDiagnosticCaptureAllErrors,
		"preview_chars":         "4096",
		"sync_to_usage_log":     "true",
		"dedupe_window_seconds": "0",
	}))
	RebuildUpstreamDiagnosticSnapshot()

	snapshot := CurrentUpstreamDiagnosticSnapshot()
	require.False(t, snapshot.Enabled)
	require.Equal(t, UpstreamDiagnosticCaptureAllErrors, snapshot.CaptureMode)
	require.Equal(t, 4096, snapshot.PreviewChars)
	require.True(t, snapshot.SyncToUsageLog)
	require.Zero(t, snapshot.DedupeWindowSeconds)
}

func TestValidateUpstreamDiagnosticOptionRejectsInvalidDocumentFields(t *testing.T) {
	valid := map[string]string{
		"enabled":               "true",
		"capture_mode":          UpstreamDiagnosticCaptureFallbackOnly,
		"preview_chars":         "256",
		"sync_to_usage_log":     "false",
		"dedupe_window_seconds": "3600",
	}
	for key, value := range valid {
		require.NoError(t, ValidateUpstreamDiagnosticOption(key, value), key)
	}

	invalid := map[string]string{
		"enabled":               "yes",
		"capture_mode":          "everything",
		"preview_chars":         "255",
		"sync_to_usage_log":     "1",
		"dedupe_window_seconds": "3601",
		"unknown":               "true",
	}
	for key, value := range invalid {
		require.Error(t, ValidateUpstreamDiagnosticOption(key, value), key)
	}
	require.Error(t, ValidateUpstreamDiagnosticOption("preview_chars", " 256 "))
	require.Error(t, ValidateUpstreamDiagnosticOption("dedupe_window_seconds", " 60 "))
}

func TestValidateUpstreamDiagnosticSettingRejectsOutOfRangeDocument(t *testing.T) {
	setting := UpstreamDiagnosticSetting{
		CaptureMode:         UpstreamDiagnosticCaptureFallbackOnly,
		PreviewChars:        MinUpstreamDiagnosticPreviewChars,
		DedupeWindowSeconds: MaxUpstreamDiagnosticDedupeSeconds,
	}
	require.NoError(t, ValidateUpstreamDiagnosticSetting(setting))
	setting.PreviewChars = MinUpstreamDiagnosticPreviewChars - 1
	require.Error(t, ValidateUpstreamDiagnosticSetting(setting))
	setting.PreviewChars = MinUpstreamDiagnosticPreviewChars
	setting.DedupeWindowSeconds = MaxUpstreamDiagnosticDedupeSeconds + 1
	require.Error(t, ValidateUpstreamDiagnosticSetting(setting))
}
