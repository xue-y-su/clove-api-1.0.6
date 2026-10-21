package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"

	"github.com/stretchr/testify/require"
)

// TestFormatUserLogsStripsQuotaSaturation verifies the admin-only quota
// saturation marker (nested under other.admin_info) is removed for non-admin
// log views, since formatUserLogs strips the whole admin_info object.
func TestFormatUserLogsStripsQuotaSaturation(t *testing.T) {
	other := common.MapToJsonStr(map[string]interface{}{
		"model_price": 0.004,
		"admin_info": map[string]interface{}{
			"quota_saturation": map[string]interface{}{
				"op":      "QuotaFromDecimal",
				"kind":    "overflow",
				"clamped": common.MaxQuota,
			},
		},
	})
	logs := []*Log{{Other: other}}

	formatUserLogs(logs, 0)

	parsed, err := common.StrToMap(logs[0].Other)
	require.NoError(t, err)
	_, hasAdminInfo := parsed["admin_info"]
	require.False(t, hasAdminInfo, "admin_info (and nested quota_saturation) must be stripped for non-admin views")
	// Non-admin billing fields remain visible.
	require.Contains(t, parsed, "model_price")
}

func TestFormatUserLogsKeepsStreamStatusForOwner(t *testing.T) {
	other := common.MapToJsonStr(map[string]interface{}{
		"stream_status": map[string]interface{}{
			"completed": false,
			"error":     "upstream disconnected",
		},
		"admin_info": map[string]interface{}{"channel_id": 12},
	})
	logs := []*Log{{Other: other}}

	formatUserLogs(logs, 0)

	parsed, err := common.StrToMap(logs[0].Other)
	require.NoError(t, err)
	require.Contains(t, parsed, "stream_status")
	require.NotContains(t, parsed, "admin_info")
}

func TestFormatUserLogsRedactsSourceAttributionTarget(t *testing.T) {
	other := common.MapToJsonStr(map[string]interface{}{
		"op": map[string]interface{}{
			"action": "source_attribution.confirm",
			"params": map[string]interface{}{
				"option_id":          "offline",
				"option_name":        "Offline referral",
				"target_inviter_id":  42,
				"target_username":    "private-inviter",
				"target_aff_code":    "private-code",
				"source_description": "private description",
			},
		},
	})
	logs := []*Log{{Other: other}}

	formatUserLogs(logs, 0)

	parsed, err := common.StrToMap(logs[0].Other)
	require.NoError(t, err)
	encoded, err := common.Marshal(parsed)
	require.NoError(t, err)
	require.Contains(t, string(encoded), "offline")
	require.Contains(t, string(encoded), "Offline referral")
	require.NotContains(t, string(encoded), "private-inviter")
	require.NotContains(t, string(encoded), "private-code")
	require.NotContains(t, string(encoded), "private description")
	require.NotContains(t, string(encoded), "target_inviter_id")
}

func TestFormatUserLogsRedactsUserRedemptionParticipantIDs(t *testing.T) {
	other := common.MapToJsonStr(map[string]interface{}{
		"op": map[string]interface{}{
			"action": "user.redemption.claim",
			"params": map[string]interface{}{
				"id":                   7,
				"quota":                500000,
				"target_user_id":       120,
				"claimant_user_id":     121,
				"requested_by_user_id": 122,
			},
		},
	})
	logs := []*Log{{Other: other}}

	formatUserLogs(logs, 0)

	parsed, err := common.StrToMap(logs[0].Other)
	require.NoError(t, err)
	encoded, err := common.Marshal(parsed)
	require.NoError(t, err)
	require.Contains(t, string(encoded), `"id":7`)
	require.Contains(t, string(encoded), `"quota":500000`)
	require.NotContains(t, string(encoded), "target_user_id")
	require.NotContains(t, string(encoded), "claimant_user_id")
	require.NotContains(t, string(encoded), "requested_by_user_id")
}
