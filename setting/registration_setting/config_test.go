package registration_setting

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func withRegistrationOptions(t *testing.T, options map[string]string) {
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

func TestGetSettingsIncludesInvitationApplicationPolicy(t *testing.T) {
	withRegistrationOptions(t, map[string]string{
		AffRequiredKey:            "true",
		InviteLimitUnlimitedKey:   "false",
		InviteLimitCountKey:       "3",
		ApplicationsEnabledKey:    "false",
		ReapplyCooldownSecondsKey: "7200",
	})

	settings := GetSettings()

	assert.True(t, settings.AffRequired)
	assert.False(t, settings.InviteLimitUnlimited)
	assert.Equal(t, 3, settings.InviteLimitCount)
	assert.False(t, settings.InvitationApplicationsEnabled)
	assert.Equal(t, int64(7200), settings.InvitationReapplyCooldownSeconds)
}

func TestRegistrationSettingsUseSafeInvitationDefaults(t *testing.T) {
	withRegistrationOptions(t, map[string]string{})

	settings := GetSettings()

	assert.True(t, settings.InvitationApplicationsEnabled)
	assert.Equal(t, DefaultInvitationReapplyCooldownSeconds, settings.InvitationReapplyCooldownSeconds)
}

func TestRegistrationSettingsValidateInvitationCooldown(t *testing.T) {
	settings := Settings{
		InviteLimitUnlimited:             true,
		InvitationApplicationsEnabled:    true,
		InvitationReapplyCooldownSeconds: 3600,
	}
	require.NoError(t, ValidateSettings(settings))

	settings.InvitationReapplyCooldownSeconds = -1
	require.Error(t, ValidateSettings(settings))

	settings.InvitationReapplyCooldownSeconds = MaxInvitationReapplyCooldownSeconds + 1
	require.Error(t, ValidateSettings(settings))
}

func TestRegistrationSettingsLimitInvitationCooldownToThirtyDays(t *testing.T) {
	assert.Equal(t, int64(30*24*60*60), MaxInvitationReapplyCooldownSeconds)
}

func TestRegistrationSettingsToOptionsIncludesInvitationPolicy(t *testing.T) {
	options := SettingsToOptions(Settings{
		AffRequired:                      true,
		InviteLimitUnlimited:             false,
		InviteLimitCount:                 2,
		InvitationApplicationsEnabled:    false,
		InvitationReapplyCooldownSeconds: 1800,
	})

	assert.Equal(t, "false", options[ApplicationsEnabledKey])
	assert.Equal(t, "1800", options[ReapplyCooldownSecondsKey])
}

func TestInvitationPosterSettingsTrimText(t *testing.T) {
	settings := NormalizeSettings(Settings{
		InvitationPosterTitle:    "  Welcome to {站点名称}  ",
		InvitationPosterSubtitle: "  Scan to register  ",
	})

	assert.Equal(t, "Welcome to {站点名称}", settings.InvitationPosterTitle)
	assert.Equal(t, "Scan to register", settings.InvitationPosterSubtitle)
}

func TestInvitationPosterSettingsValidateUnicodeLength(t *testing.T) {
	settings := Settings{
		InvitationPosterTitle:            strings.Repeat("邀", InvitationPosterTitleMaxRunes),
		InvitationPosterSubtitle:         strings.Repeat("请", InvitationPosterSubtitleMaxRunes),
		InvitationReapplyCooldownSeconds: DefaultInvitationReapplyCooldownSeconds,
	}
	require.NoError(t, ValidateSettings(settings))

	settings.InvitationPosterTitle += "请"
	require.Error(t, ValidateSettings(settings))
	settings.InvitationPosterTitle = ""
	settings.InvitationPosterSubtitle += "码"
	require.Error(t, ValidateSettings(settings))
}

func TestInvitationPosterSettingsToOptionsIncludesCopy(t *testing.T) {
	options := SettingsToOptions(Settings{
		InvitationPosterTitle:    "  Join {站点名称}  ",
		InvitationPosterSubtitle: "  Scan now  ",
	})

	assert.Equal(t, "Join {站点名称}", options[InvitationPosterTitleKey])
	assert.Equal(t, "Scan now", options[InvitationPosterSubtitleKey])
}
