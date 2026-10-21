package announcement_setting

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/console_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func withAnnouncementOptions(t *testing.T, options map[string]string) {
	t.Helper()
	common.OptionMapRWMutex.Lock()
	previous := common.OptionMap
	common.OptionMap = options
	common.OptionMapRWMutex.Unlock()
	previousConsole := *console_setting.GetConsoleSetting()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = previous
		common.OptionMapRWMutex.Unlock()
		*console_setting.GetConsoleSetting() = previousConsole
	})
}

func TestAnnouncementSettingsUseSafeDefaults(t *testing.T) {
	withAnnouncementOptions(t, map[string]string{})
	console_setting.GetConsoleSetting().AnnouncementsEnabled = true

	settings := GetSettings()

	assert.True(t, settings.Enabled)
	assert.Equal(t, int64(5*MiB), settings.ImageMaxBytes)
	payload, err := common.Marshal(settings)
	require.NoError(t, err)
	assert.NotContains(t, string(payload), "popup_enabled")
	assert.Contains(t, DefaultOptions(), PopupEnabledKey)
}

func TestAnnouncementSettingsValidateImageLimit(t *testing.T) {
	settings := Settings{Enabled: true, ImageMaxBytes: MiB}
	require.NoError(t, ValidateSettings(settings))

	settings.ImageMaxBytes = MiB - 1
	require.Error(t, ValidateSettings(settings))

	settings.ImageMaxBytes = 5*MiB + 1
	require.Error(t, ValidateSettings(settings))
}

func TestAnnouncementSettingsToOptionsIncludesConsoleAndMediaPolicy(t *testing.T) {
	options := SettingsToOptions(Settings{Enabled: false, ImageMaxBytes: 2 * MiB})

	assert.Equal(t, "false", options[AnnouncementsEnabledKey])
	assert.NotContains(t, options, PopupEnabledKey)
	assert.Equal(t, "2097152", options[ImageMaxBytesKey])
}
