package dto

import "github.com/QuantumNous/new-api/setting/operation_setting"

type AlphaSearchDocument struct {
	Enabled bool `json:"enabled"`
}

type RelayBillingSettings struct {
	AlphaSearch        AlphaSearchDocument                         `json:"alpha_search"`
	ToolPricing        operation_setting.ToolPriceSetting          `json:"tool_pricing"`
	UpstreamDiagnostic operation_setting.UpstreamDiagnosticSetting `json:"upstream_diagnostic"`
}
