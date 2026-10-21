package user_governance_setting

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func withGovernanceOptions(t *testing.T, options map[string]string) {
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

func TestGetSettingsReadsExistingUserGovernanceKeys(t *testing.T) {
	withGovernanceOptions(t, map[string]string{
		AvatarUploadEnabledKey:        "false",
		AvatarUploadEndpointKey:       "https://upload.example.test/image",
		AvatarAllowedDomainsKey:       "IMG.EXAMPLE.TEST, cdn.example.test,img.example.test",
		DisplayNameMaxLengthKey:       "32",
		DisplayNameCooldownSecondsKey: "86400",
	})

	settings := GetSettings()

	assert.False(t, settings.AvatarUploadEnabled)
	assert.Equal(t, "https://upload.example.test/image", settings.AvatarUploadEndpoint)
	assert.Equal(t, []string{"img.example.test", "cdn.example.test"}, settings.AvatarAllowedDomains)
	assert.Equal(t, 32, settings.DisplayNameMaxLength)
	assert.Equal(t, int64(86400), settings.DisplayNameCooldownSeconds)
}

func TestUserGovernanceSettingsValidatePolicy(t *testing.T) {
	settings := Settings{
		AvatarUploadEnabled:        true,
		AvatarUploadEndpoint:       "https://upload.example.test/image",
		AvatarAllowedDomains:       []string{"img.example.test"},
		DisplayNameMaxLength:       40,
		DisplayNameCooldownSeconds: 604800,
	}
	require.NoError(t, ValidateSettings(settings))

	invalidEndpoint := settings
	invalidEndpoint.AvatarUploadEndpoint = "http://upload.example.test/image"
	require.Error(t, ValidateSettings(invalidEndpoint))

	invalidLength := settings
	invalidLength.DisplayNameMaxLength = 101
	require.Error(t, ValidateSettings(invalidLength))

	invalidCooldown := settings
	invalidCooldown.DisplayNameCooldownSeconds = MaxDisplayNameCooldownSeconds + 1
	require.Error(t, ValidateSettings(invalidCooldown))
}

func TestUserGovernanceSettingsToOptionsUsesCompatibleKeys(t *testing.T) {
	options := SettingsToOptions(Settings{
		AvatarUploadEnabled:        true,
		AvatarUploadEndpoint:       "https://upload.example.test/image",
		AvatarAllowedDomains:       []string{"IMG.EXAMPLE.TEST", "img.example.test", "cdn.example.test"},
		DisplayNameMaxLength:       30,
		DisplayNameCooldownSeconds: 0,
	})

	assert.Equal(t, "true", options[AvatarUploadEnabledKey])
	assert.Equal(t, "img.example.test,cdn.example.test", options[AvatarAllowedDomainsKey])
	assert.Equal(t, "30", options[DisplayNameMaxLengthKey])
	assert.Equal(t, "0", options[DisplayNameCooldownSecondsKey])
}
