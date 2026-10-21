package user_redemption_setting

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func withUserRedemptionOptions(t *testing.T, options map[string]string) {
	t.Helper()
	common.OptionMapRWMutex.Lock()
	previous := common.OptionMap
	common.OptionMap = options
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = previous
		common.OptionMapRWMutex.Unlock()
	})
}

func TestUserRedemptionSettingsUseSafeDefaults(t *testing.T) {
	withUserRedemptionOptions(t, map[string]string{})

	settings := GetSettings()

	assert.True(t, settings.Enabled)
	assert.True(t, settings.SingleEnabled)
	assert.True(t, settings.RandomEnabled)
	assert.True(t, settings.SubscriptionEnabled)
	assert.Zero(t, settings.MaxActiveCodesPerUser)
	assert.Equal(t, DefaultMaxClaimsPerCode, settings.MaxClaimsPerCode)
	assert.Equal(t, DefaultMaxExpiryHours, settings.MaxExpiryHours)
}

func TestUserRedemptionSettingsValidateBounds(t *testing.T) {
	settings := Settings{
		Enabled:               true,
		SingleEnabled:         true,
		RandomEnabled:         true,
		SubscriptionEnabled:   true,
		MaxActiveCodesPerUser: 10,
		MaxClaimsPerCode:      50,
		MaxExpiryHours:        72,
	}
	require.NoError(t, ValidateSettings(settings))

	settings.MaxActiveCodesPerUser = -1
	require.Error(t, ValidateSettings(settings))
	settings.MaxActiveCodesPerUser = 10
	settings.MaxClaimsPerCode = MaxClaimsPerCode + 1
	require.Error(t, ValidateSettings(settings))
	settings.MaxClaimsPerCode = 50
	settings.MaxExpiryHours = MaxExpiryHours + 1
	require.Error(t, ValidateSettings(settings))
}

func TestUserRedemptionSettingsRoundTripOptions(t *testing.T) {
	settings := Settings{
		Enabled:               false,
		SingleEnabled:         true,
		RandomEnabled:         false,
		SubscriptionEnabled:   false,
		MaxActiveCodesPerUser: 3,
		MaxClaimsPerCode:      12,
		MaxExpiryHours:        48,
	}

	options := SettingsToOptions(settings)
	withUserRedemptionOptions(t, options)
	assert.Equal(t, settings, GetSettings())
}

func TestUserRedemptionSettingsExposeSubscriptionSwitch(t *testing.T) {
	withUserRedemptionOptions(t, map[string]string{SubscriptionEnabledKey: "false"})
	assert.False(t, GetSettings().SubscriptionEnabled)
}
