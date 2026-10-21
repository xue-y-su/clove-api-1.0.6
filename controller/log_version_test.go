package controller

import (
	"encoding/json"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/stretchr/testify/require"
)

func TestNormalizeVisibleLogVersionsReplacesLegacyDisplayValue(t *testing.T) {
	logs := []*model.Log{{
		Other: "{\"admin_info\":{\"version\":\"v1.0.0-rc.21.clove-enhanced\",\"action\":\"review\"}}",
	}}

	normalizeVisibleLogVersions(logs)

	other, err := common.StrToMap(logs[0].Other)
	require.NoError(t, err)
	adminInfo, ok := other["admin_info"].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, common.DisplayVersion, adminInfo["version"])
	require.Equal(t, "review", adminInfo["action"])
}

func TestEnrichLegacyLogBillingModesRecognizesZeroPricedPerCallModel(t *testing.T) {
	savedPrices := ratio_setting.ModelPrice2JSONString()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(savedPrices))
	})
	prices := map[string]float64{"free-per-call-model": 0}
	rawPrices, err := json.Marshal(prices)
	require.NoError(t, err)
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(string(rawPrices)))

	logs := []*model.Log{{
		Type:      model.LogTypeConsume,
		ModelName: "free-per-call-model",
		Other:     `{"model_price":0,"model_ratio":0}`,
	}}

	enrichLegacyLogBillingModes(logs)

	other, err := common.StrToMap(logs[0].Other)
	require.NoError(t, err)
	require.Equal(t, "per_call", other["billing_mode"])
}

func TestEnrichLegacyLogBillingModesPreservesRecordedMode(t *testing.T) {
	logs := []*model.Log{{
		Type:      model.LogTypeConsume,
		ModelName: "historical-model",
		Other:     `{"billing_mode":"per_token","model_price":0}`,
	}}

	enrichLegacyLogBillingModes(logs)

	other, err := common.StrToMap(logs[0].Other)
	require.NoError(t, err)
	require.Equal(t, "per_token", other["billing_mode"])
}

func TestNormalizeVisibleLogVersionsPreservesUnknownVersionsAndInvalidJSON(t *testing.T) {
	logs := []*model.Log{
		{Other: "{\"admin_info\":{\"version\":\"v2.0.0-custom\"}}"},
		{Other: "not-json"},
	}

	normalizeVisibleLogVersions(logs)

	require.Contains(t, logs[0].Other, "v2.0.0-custom")
	require.Equal(t, "not-json", logs[1].Other)
}
