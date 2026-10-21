package announcement_setting

import (
	"errors"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/console_setting"
)

const (
	AnnouncementsEnabledKey       = "console_setting.announcements_enabled"
	PopupEnabledKey               = "announcement_setting.popup_enabled"
	ImageMaxBytesKey              = "announcement_setting.image_max_bytes"
	MiB                     int64 = 1024 * 1024
)

type Settings struct {
	Enabled       bool  `json:"enabled"`
	ImageMaxBytes int64 `json:"image_max_bytes"`
}

func DefaultOptions() map[string]string {
	return map[string]string{
		PopupEnabledKey:  "true",
		ImageMaxBytesKey: strconv.FormatInt(5*MiB, 10),
	}
}

func GetSettings() Settings {
	common.OptionMapRWMutex.RLock()
	defer common.OptionMapRWMutex.RUnlock()
	return Settings{
		Enabled:       console_setting.GetConsoleSetting().AnnouncementsEnabled,
		ImageMaxBytes: parseInt64WithDefault(common.OptionMap[ImageMaxBytesKey], 5*MiB),
	}
}

func ValidateSettings(settings Settings) error {
	if settings.ImageMaxBytes < MiB || settings.ImageMaxBytes > 5*MiB {
		return errors.New("announcement image size must be between 1 MiB and 5 MiB")
	}
	return nil
}

func SettingsToOptions(settings Settings) map[string]string {
	return map[string]string{
		AnnouncementsEnabledKey: strconv.FormatBool(settings.Enabled),
		ImageMaxBytesKey:        strconv.FormatInt(settings.ImageMaxBytes, 10),
	}
}

func parseInt64WithDefault(value string, fallback int64) int64 {
	parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || parsed < MiB || parsed > 5*MiB {
		return fallback
	}
	return parsed
}
