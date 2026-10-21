package operation_setting

import (
	"fmt"
	"strconv"
	"sync/atomic"

	"github.com/QuantumNous/new-api/setting/config"
)

const (
	UpstreamDiagnosticSettingConfigName = "upstream_error_diagnostic_setting"

	UpstreamDiagnosticCaptureFallbackOnly = "fallback_only"
	UpstreamDiagnosticCaptureAllErrors    = "all_errors"

	MinUpstreamDiagnosticPreviewChars  = 256
	MaxUpstreamDiagnosticPreviewChars  = 8192
	MaxUpstreamDiagnosticDedupeSeconds = 3600
)

type UpstreamDiagnosticSetting struct {
	Enabled             bool   `json:"enabled"`
	CaptureMode         string `json:"capture_mode"`
	PreviewChars        int    `json:"preview_chars"`
	SyncToUsageLog      bool   `json:"sync_to_usage_log"`
	DedupeWindowSeconds int    `json:"dedupe_window_seconds"`
}

type UpstreamDiagnosticSnapshot struct {
	Enabled             bool
	CaptureMode         string
	PreviewChars        int
	SyncToUsageLog      bool
	DedupeWindowSeconds int
}

func ValidateUpstreamDiagnosticSetting(setting UpstreamDiagnosticSetting) error {
	if setting.CaptureMode != UpstreamDiagnosticCaptureFallbackOnly && setting.CaptureMode != UpstreamDiagnosticCaptureAllErrors {
		return fmt.Errorf("capture_mode must be %s or %s", UpstreamDiagnosticCaptureFallbackOnly, UpstreamDiagnosticCaptureAllErrors)
	}
	if setting.PreviewChars < MinUpstreamDiagnosticPreviewChars || setting.PreviewChars > MaxUpstreamDiagnosticPreviewChars {
		return fmt.Errorf("preview_chars must be between %d and %d", MinUpstreamDiagnosticPreviewChars, MaxUpstreamDiagnosticPreviewChars)
	}
	if setting.DedupeWindowSeconds < 0 || setting.DedupeWindowSeconds > MaxUpstreamDiagnosticDedupeSeconds {
		return fmt.Errorf("dedupe_window_seconds must be between 0 and %d", MaxUpstreamDiagnosticDedupeSeconds)
	}
	return nil
}

var upstreamDiagnosticSetting = UpstreamDiagnosticSetting{
	Enabled:             true,
	CaptureMode:         UpstreamDiagnosticCaptureFallbackOnly,
	PreviewChars:        2048,
	SyncToUsageLog:      false,
	DedupeWindowSeconds: 60,
}

var upstreamDiagnosticSnapshot atomic.Pointer[UpstreamDiagnosticSnapshot]

func init() {
	config.GlobalConfig.Register(UpstreamDiagnosticSettingConfigName, &upstreamDiagnosticSetting)
	RebuildUpstreamDiagnosticSnapshot()
}

func GetUpstreamDiagnosticSetting() *UpstreamDiagnosticSetting {
	return &upstreamDiagnosticSetting
}

func CurrentUpstreamDiagnosticSnapshot() UpstreamDiagnosticSnapshot {
	if snapshot := upstreamDiagnosticSnapshot.Load(); snapshot != nil {
		return *snapshot
	}
	RebuildUpstreamDiagnosticSnapshot()
	return *upstreamDiagnosticSnapshot.Load()
}

func RebuildUpstreamDiagnosticSnapshot() {
	setting := upstreamDiagnosticSetting
	if setting.CaptureMode != UpstreamDiagnosticCaptureAllErrors {
		setting.CaptureMode = UpstreamDiagnosticCaptureFallbackOnly
	}
	if setting.PreviewChars < MinUpstreamDiagnosticPreviewChars || setting.PreviewChars > MaxUpstreamDiagnosticPreviewChars {
		setting.PreviewChars = 2048
	}
	if setting.DedupeWindowSeconds < 0 || setting.DedupeWindowSeconds > MaxUpstreamDiagnosticDedupeSeconds {
		setting.DedupeWindowSeconds = 60
	}
	upstreamDiagnosticSnapshot.Store(&UpstreamDiagnosticSnapshot{
		Enabled:             setting.Enabled,
		CaptureMode:         setting.CaptureMode,
		PreviewChars:        setting.PreviewChars,
		SyncToUsageLog:      setting.SyncToUsageLog,
		DedupeWindowSeconds: setting.DedupeWindowSeconds,
	})
}

func ValidateUpstreamDiagnosticOption(key, value string) error {
	switch key {
	case "enabled", "sync_to_usage_log":
		if value != "true" && value != "false" {
			return fmt.Errorf("%s must be true or false", key)
		}
	case "capture_mode":
		if value != UpstreamDiagnosticCaptureFallbackOnly && value != UpstreamDiagnosticCaptureAllErrors {
			return fmt.Errorf("capture_mode must be %s or %s", UpstreamDiagnosticCaptureFallbackOnly, UpstreamDiagnosticCaptureAllErrors)
		}
	case "preview_chars":
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < MinUpstreamDiagnosticPreviewChars || parsed > MaxUpstreamDiagnosticPreviewChars {
			return fmt.Errorf("preview_chars must be between %d and %d", MinUpstreamDiagnosticPreviewChars, MaxUpstreamDiagnosticPreviewChars)
		}
	case "dedupe_window_seconds":
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 0 || parsed > MaxUpstreamDiagnosticDedupeSeconds {
			return fmt.Errorf("dedupe_window_seconds must be between 0 and %d", MaxUpstreamDiagnosticDedupeSeconds)
		}
	default:
		return fmt.Errorf("unknown upstream diagnostic option %q", key)
	}
	return nil
}
