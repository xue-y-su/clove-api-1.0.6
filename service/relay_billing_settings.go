package service

import (
	"encoding/json"
	"fmt"
	"strconv"
	"sync"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

var relayBillingSettingsMu sync.RWMutex

func CurrentRelayBillingSettings() dto.RelayBillingSettings {
	relayBillingSettingsMu.RLock()
	defer relayBillingSettingsMu.RUnlock()
	return dto.RelayBillingSettings{
		AlphaSearch: dto.AlphaSearchDocument{
			Enabled: operation_setting.GetAlphaSearchSetting().Enabled,
		},
		ToolPricing:        operation_setting.CurrentToolPriceSetting(),
		UpstreamDiagnostic: *operation_setting.GetUpstreamDiagnosticSetting(),
	}
}

func UpdateRelayBillingSettings(input dto.RelayBillingSettings) (dto.RelayBillingSettings, error) {
	normalized, err := normalizeRelayBillingSettings(input)
	if err != nil {
		return dto.RelayBillingSettings{}, err
	}
	values, err := relayBillingOptionValues(normalized)
	if err != nil {
		return dto.RelayBillingSettings{}, err
	}

	relayBillingSettingsMu.Lock()
	defer relayBillingSettingsMu.Unlock()
	if err := model.UpdateOptionsBulk(values); err != nil {
		return dto.RelayBillingSettings{}, err
	}
	// Publish complete snapshots after the option transaction has committed.
	operation_setting.RebuildToolPriceIndex()
	operation_setting.RebuildAlphaSearchSnapshot()
	operation_setting.RebuildUpstreamDiagnosticSnapshot()
	return normalized, nil
}

func normalizeRelayBillingSettings(input dto.RelayBillingSettings) (dto.RelayBillingSettings, error) {
	toolPricing := input.ToolPricing
	if toolPricing.Prices == nil {
		toolPricing.Prices = map[string]float64{}
	}
	if toolPricing.ImagePriceMatrix == nil {
		toolPricing.ImagePriceMatrix = map[string]map[string]float64{}
	}
	if toolPricing.ImageModelModeOverrides == nil {
		toolPricing.ImageModelModeOverrides = map[string]string{}
	}
	if toolPricing.MultiplierMode == "" {
		toolPricing.MultiplierMode = operation_setting.ToolMultiplierGroupOnly
	}
	if toolPricing.ImagePricingMode == "" {
		toolPricing.ImagePricingMode = operation_setting.ImagePricingMatrix
	}
	if _, err := operation_setting.NewToolPricingSnapshot(toolPricing); err != nil {
		return dto.RelayBillingSettings{}, err
	}

	diagnostic := input.UpstreamDiagnostic
	if diagnostic.CaptureMode == "" {
		diagnostic.CaptureMode = operation_setting.UpstreamDiagnosticCaptureFallbackOnly
	}
	if diagnostic.PreviewChars == 0 {
		diagnostic.PreviewChars = 2048
	}
	if err := operation_setting.ValidateUpstreamDiagnosticSetting(diagnostic); err != nil {
		return dto.RelayBillingSettings{}, err
	}

	input.ToolPricing = toolPricing
	input.UpstreamDiagnostic = diagnostic
	return input, nil
}

func relayBillingOptionValues(settings dto.RelayBillingSettings) (map[string]string, error) {
	marshal := func(value any) (string, error) {
		encoded, err := json.Marshal(value)
		if err != nil {
			return "", fmt.Errorf("encode relay billing setting: %w", err)
		}
		return string(encoded), nil
	}
	prices, err := marshal(settings.ToolPricing.Prices)
	if err != nil {
		return nil, err
	}
	imageMatrix, err := marshal(settings.ToolPricing.ImagePriceMatrix)
	if err != nil {
		return nil, err
	}
	imageOverrides, err := marshal(settings.ToolPricing.ImageModelModeOverrides)
	if err != nil {
		return nil, err
	}
	return map[string]string{
		"tool_price_setting.enabled":                              strconv.FormatBool(settings.ToolPricing.Enabled),
		"tool_price_setting.prices":                               prices,
		"tool_price_setting.multiplier_mode":                      settings.ToolPricing.MultiplierMode,
		"tool_price_setting.image_pricing_mode":                   settings.ToolPricing.ImagePricingMode,
		"tool_price_setting.image_price_matrix":                   imageMatrix,
		"tool_price_setting.image_model_mode_overrides":           imageOverrides,
		"alpha_search_setting.enabled":                            strconv.FormatBool(settings.AlphaSearch.Enabled),
		"upstream_error_diagnostic_setting.enabled":               strconv.FormatBool(settings.UpstreamDiagnostic.Enabled),
		"upstream_error_diagnostic_setting.capture_mode":          settings.UpstreamDiagnostic.CaptureMode,
		"upstream_error_diagnostic_setting.preview_chars":         strconv.Itoa(settings.UpstreamDiagnostic.PreviewChars),
		"upstream_error_diagnostic_setting.sync_to_usage_log":     strconv.FormatBool(settings.UpstreamDiagnostic.SyncToUsageLog),
		"upstream_error_diagnostic_setting.dedupe_window_seconds": strconv.Itoa(settings.UpstreamDiagnostic.DedupeWindowSeconds),
	}, nil
}
