package openai

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newOpenAIToolMeterInfo(t *testing.T) *relaycommon.RelayInfo {
	t.Helper()
	snapshot, err := operation_setting.NewToolPricingSnapshot(operation_setting.ToolPriceSetting{
		Enabled: true,
		Prices: map[string]float64{
			"web_search_preview": 10,
			"file_search":        2.5,
			"lookup_customer":    1,
		},
		MultiplierMode:   operation_setting.ToolMultiplierGroupOnly,
		ImagePricingMode: operation_setting.ImagePricingMatrix,
		ImagePriceMatrix: map[string]map[string]float64{"high": {"1024x1024": 0.5}},
	})
	require.NoError(t, err)
	meter := relaycommon.NewToolBillingMeter(relaycommon.ToolBillingContext{
		Pricing: snapshot, PublicModel: "gpt-4.1-mini",
		GroupRatio: decimal.NewFromInt(1), OtherRatioMultiplier: decimal.NewFromInt(1),
		QuotaPerUnit: decimal.NewFromInt(500000),
	})
	info := &relaycommon.RelayInfo{OriginModelName: "gpt-4.1-mini", ToolBillingMeter: meter}
	info.BeginToolBillingAttempt()
	return info
}

func TestConfirmResponsesOutputUsesActualCompletedOutputsAndDeduplicates(t *testing.T) {
	info := newOpenAIToolMeterInfo(t)
	outputs := []dto.ResponsesOutput{
		{Type: dto.BuildInCallWebSearchCall, ID: "web-1", Status: "completed"},
		{Type: dto.BuildInCallWebSearchCall, ID: "web-1", Status: "completed"},
		{Type: dto.BuildInCallFileSearchCall, CallId: "file-1", Status: "completed"},
		{Type: dto.BuildInCallFunctionCall, CallId: "fn-1", Name: "lookup_customer", Status: "completed"},
		{Type: dto.ResponsesOutputTypeImageGenerationCall, ID: "image-1", Status: "completed", Result: "base64", Quality: "high", Size: "1024x1024"},
		{Type: dto.ResponsesOutputTypeImageGenerationCall, ID: "image-failed", Status: "failed", Result: "partial", Quality: "high", Size: "1024x1024"},
	}
	for index := range outputs {
		require.NoError(t, confirmResponsesOutput(info, &outputs[index], &index))
	}
	require.NoError(t, info.CommitToolBillingAttempt())

	result, err := info.ToolBillingMeter.TargetToolQuota()
	require.NoError(t, err)
	require.Len(t, result.Items, 4)
	counts := map[string]int{}
	for _, item := range result.Items {
		counts[item.Name] = item.Count
	}
	assert.Equal(t, 1, counts["web_search_preview"])
	assert.Equal(t, 1, counts["file_search"])
	assert.Equal(t, 1, counts["lookup_customer"])
	assert.Equal(t, 1, counts["image_generation"])
}

func TestConfirmResponsesOutputRejectsIncompleteCustomFunction(t *testing.T) {
	info := newOpenAIToolMeterInfo(t)
	item := dto.ResponsesOutput{Type: dto.BuildInCallFunctionCall, ID: "fn", Name: "lookup_customer", Status: "incomplete"}
	require.NoError(t, confirmResponsesOutput(info, &item, nil))
	require.NoError(t, info.CommitToolBillingAttempt())
	result, err := info.ToolBillingMeter.TargetToolQuota()
	require.NoError(t, err)
	assert.Empty(t, result.Items)
}
