package gemini

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfirmGeminiGroundingToolsChargesConfirmedCandidatesOnce(t *testing.T) {
	snapshot, err := operation_setting.NewToolPricingSnapshot(operation_setting.ToolPriceSetting{
		Enabled: true, Prices: map[string]float64{"google_search": 14},
		MultiplierMode:   operation_setting.ToolMultiplierGroupOnly,
		ImagePricingMode: operation_setting.ImagePricingMatrix,
	})
	require.NoError(t, err)
	meter := relaycommon.NewToolBillingMeter(relaycommon.ToolBillingContext{
		Pricing: snapshot, PublicModel: "gemini-2.5-pro",
		GroupRatio: decimal.NewFromInt(1), OtherRatioMultiplier: decimal.NewFromInt(1),
		QuotaPerUnit: decimal.NewFromInt(500000),
	})
	info := &relaycommon.RelayInfo{OriginModelName: "gemini-2.5-pro", ToolBillingMeter: meter}
	info.BeginToolBillingAttempt()
	response := &dto.GeminiChatResponse{Candidates: []dto.GeminiChatCandidate{
		{Index: 0, GroundingMetadata: &dto.GeminiGroundingMetadata{WebSearchQueries: []string{"latest news"}}},
		{Index: 1},
	}}
	require.NoError(t, confirmGeminiGroundingTools(info, response))
	require.NoError(t, confirmGeminiGroundingTools(info, response))
	require.NoError(t, info.CommitToolBillingAttempt())

	result, err := info.ToolBillingMeter.TargetToolQuota()
	require.NoError(t, err)
	require.Len(t, result.Items, 1)
	assert.Equal(t, "google_search", result.Items[0].Name)
	assert.Equal(t, 1, result.Items[0].Count)
}
