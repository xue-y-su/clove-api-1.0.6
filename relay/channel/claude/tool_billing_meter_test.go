package claude

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newClaudeToolMeterInfo(t *testing.T) *relaycommon.RelayInfo {
	t.Helper()
	snapshot, err := operation_setting.NewToolPricingSnapshot(operation_setting.ToolPriceSetting{
		Enabled:          true,
		Prices:           map[string]float64{"web_search": 10, "lookup_customer": 1},
		MultiplierMode:   operation_setting.ToolMultiplierGroupOnly,
		ImagePricingMode: operation_setting.ImagePricingMatrix,
	})
	require.NoError(t, err)
	meter := relaycommon.NewToolBillingMeter(relaycommon.ToolBillingContext{
		Pricing: snapshot, PublicModel: "claude-sonnet-4",
		GroupRatio: decimal.NewFromInt(1), OtherRatioMultiplier: decimal.NewFromInt(1),
		QuotaPerUnit: decimal.NewFromInt(500000),
	})
	info := &relaycommon.RelayInfo{OriginModelName: "claude-sonnet-4", ToolBillingMeter: meter}
	info.BeginToolBillingAttempt()
	return info
}

func TestConfirmClaudeBillableToolsUsesCumulativeServerUsageAndToolUse(t *testing.T) {
	info := newClaudeToolMeterInfo(t)
	first := &dto.ClaudeResponse{
		Usage:   &dto.ClaudeUsage{ServerToolUse: &dto.ClaudeServerToolUse{WebSearchRequests: 2}},
		Content: []dto.ClaudeMediaMessage{{Type: "tool_use", Id: "tool-1", Name: "lookup_customer"}},
	}
	second := &dto.ClaudeResponse{
		Usage:        &dto.ClaudeUsage{ServerToolUse: &dto.ClaudeServerToolUse{WebSearchRequests: 2}},
		ContentBlock: &dto.ClaudeMediaMessage{Type: "tool_use", Id: "tool-1", Name: "lookup_customer"},
	}
	require.NoError(t, confirmClaudeBillableTools(info, first))
	require.NoError(t, confirmClaudeBillableTools(info, second))
	require.NoError(t, info.CommitToolBillingAttempt())

	result, err := info.ToolBillingMeter.TargetToolQuota()
	require.NoError(t, err)
	counts := map[string]int{}
	for _, item := range result.Items {
		counts[item.Name] = item.Count
	}
	assert.Equal(t, 2, counts["web_search"])
	assert.Equal(t, 1, counts["lookup_customer"])
}
