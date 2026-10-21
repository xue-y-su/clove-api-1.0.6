package operation_setting

import (
	"testing"

	"github.com/QuantumNous/new-api/setting/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInvitationIncentiveDefaultsPreserveRegistrationAndDisableNewCommission(t *testing.T) {
	got := DefaultInvitationIncentiveSetting()
	require.True(t, got.Enabled)
	require.True(t, got.InviterRegistrationRewardEnabled)
	require.True(t, got.InviteeRegistrationGiftEnabled)
	require.False(t, got.CashCommissionEnabled)
	require.Equal(t, "0.00", got.CashCommissionRate)
	require.False(t, got.AdminBalanceRedemptionCommissionEnabled)
	require.Equal(t, "0.00", got.AdminBalanceRedemptionCommissionRate)
	require.False(t, got.AdminSubscriptionRedemptionCommissionEnabled)
	require.Equal(t, "0.00", got.AdminSubscriptionRedemptionCommissionRate)
}

func TestParseInvitationCommissionRate(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantBPS  int64
		wantRate string
		wantErr  bool
	}{
		{name: "zero", input: "0", wantRate: "0.00"},
		{name: "fractional", input: "5.25", wantBPS: 525, wantRate: "5.25"},
		{name: "over one hundred", input: "200", wantBPS: 20_000, wantRate: "200.00"},
		{name: "large valid rate", input: "1250.75", wantBPS: 125_075, wantRate: "1250.75"},
		{name: "int64 boundary", input: "92233720368547758.07", wantBPS: 9_223_372_036_854_775_807, wantRate: "92233720368547758.07"},
		{name: "negative", input: "-0.01", wantErr: true},
		{name: "too precise", input: "5.251", wantErr: true},
		{name: "not a number", input: "five", wantErr: true},
		{name: "overflow", input: "92233720368547758.08", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			bps, normalized, err := ParseInvitationCommissionRate(test.input)
			if test.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, test.wantBPS, bps)
			assert.Equal(t, test.wantRate, normalized)
		})
	}
}

func TestInvitationIncentiveSettingGlobalConfigRoundTrip(t *testing.T) {
	previous := *GetInvitationIncentiveSetting()
	t.Cleanup(func() { *GetInvitationIncentiveSetting() = previous })

	require.Same(
		t,
		GetInvitationIncentiveSetting(),
		config.GlobalConfig.Get(InvitationIncentiveSettingConfigName),
	)
	require.NoError(t, config.UpdateConfigFromMap(GetInvitationIncentiveSetting(), map[string]string{
		"enabled":                                          "false",
		"inviter_registration_reward_enabled":              "false",
		"invitee_registration_gift_enabled":                "false",
		"cash_commission_enabled":                          "true",
		"cash_commission_rate":                             "200.00",
		"admin_balance_redemption_commission_enabled":      "true",
		"admin_balance_redemption_commission_rate":         "5.25",
		"admin_subscription_redemption_commission_enabled": "true",
		"admin_subscription_redemption_commission_rate":    "1250.75",
	}))

	got := GetInvitationIncentiveSetting()
	assert.False(t, got.Enabled)
	assert.False(t, got.InviterRegistrationRewardEnabled)
	assert.False(t, got.InviteeRegistrationGiftEnabled)
	assert.True(t, got.CashCommissionEnabled)
	assert.Equal(t, "200.00", got.CashCommissionRate)
	assert.True(t, got.AdminBalanceRedemptionCommissionEnabled)
	assert.Equal(t, "5.25", got.AdminBalanceRedemptionCommissionRate)
	assert.True(t, got.AdminSubscriptionRedemptionCommissionEnabled)
	assert.Equal(t, "1250.75", got.AdminSubscriptionRedemptionCommissionRate)

	policy, err := GetInvitationIncentivePublicPolicy()
	require.NoError(t, err)
	assert.Equal(t, InvitationIncentivePublicPolicy{
		Enabled:               false,
		CashCommissionEnabled: true,
		CashCommissionRate:    "200.00",
	}, policy)
}
