package registration_setting

import (
	"errors"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
)

const AffRequiredKey = "registration.aff_required"
const InviteLimitUnlimitedKey = "registration.invite_limit_unlimited"
const InviteLimitCountKey = "registration.invite_limit_count"
const ApplicationsEnabledKey = "registration.invitation_applications_enabled"
const ReapplyCooldownSecondsKey = "registration.invitation_reapply_cooldown_seconds"
const InvitationPosterTitleKey = "registration.invitation_poster_title"
const InvitationPosterSubtitleKey = "registration.invitation_poster_subtitle"

const DefaultInvitationReapplyCooldownSeconds = int64(24 * 60 * 60)
const MaxInvitationReapplyCooldownSeconds = int64(30 * 24 * 60 * 60)
const InvitationPosterTitleMaxRunes = 80
const InvitationPosterSubtitleMaxRunes = 160

type Settings struct {
	AffRequired                      bool   `json:"aff_required"`
	InviteLimitUnlimited             bool   `json:"invite_limit_unlimited"`
	InviteLimitCount                 int    `json:"invite_limit_count"`
	InvitationApplicationsEnabled    bool   `json:"invitation_applications_enabled"`
	InvitationReapplyCooldownSeconds int64  `json:"invitation_reapply_cooldown_seconds"`
	InvitationPosterTitle            string `json:"invitation_poster_title"`
	InvitationPosterSubtitle         string `json:"invitation_poster_subtitle"`
}

func DefaultOptions() map[string]string {
	return map[string]string{
		AffRequiredKey:              "false",
		InviteLimitUnlimitedKey:     "true",
		InviteLimitCountKey:         "0",
		ApplicationsEnabledKey:      "true",
		ReapplyCooldownSecondsKey:   strconv.FormatInt(DefaultInvitationReapplyCooldownSeconds, 10),
		InvitationPosterTitleKey:    "",
		InvitationPosterSubtitleKey: "",
	}
}

func GetSettings() Settings {
	common.OptionMapRWMutex.RLock()
	defer common.OptionMapRWMutex.RUnlock()

	return NormalizeSettings(Settings{
		AffRequired:                   parseBoolOption(common.OptionMap[AffRequiredKey]),
		InviteLimitUnlimited:          parseBoolOptionWithDefault(common.OptionMap[InviteLimitUnlimitedKey], true),
		InviteLimitCount:              parseNonNegativeIntOption(common.OptionMap[InviteLimitCountKey]),
		InvitationApplicationsEnabled: parseBoolOptionWithDefault(common.OptionMap[ApplicationsEnabledKey], true),
		InvitationReapplyCooldownSeconds: parseBoundedInt64OptionWithDefault(
			common.OptionMap[ReapplyCooldownSecondsKey],
			DefaultInvitationReapplyCooldownSeconds,
			0,
			MaxInvitationReapplyCooldownSeconds,
		),
		InvitationPosterTitle:    common.OptionMap[InvitationPosterTitleKey],
		InvitationPosterSubtitle: common.OptionMap[InvitationPosterSubtitleKey],
	})
}

func NormalizeSettings(settings Settings) Settings {
	if settings.InviteLimitCount < 0 {
		settings.InviteLimitCount = 0
	}
	settings.InvitationPosterTitle = strings.TrimSpace(settings.InvitationPosterTitle)
	settings.InvitationPosterSubtitle = strings.TrimSpace(settings.InvitationPosterSubtitle)
	return settings
}

func ValidateSettings(settings Settings) error {
	if settings.InvitationReapplyCooldownSeconds < 0 || settings.InvitationReapplyCooldownSeconds > MaxInvitationReapplyCooldownSeconds {
		return errors.New("invitation reapply cooldown must be between 0 and 2592000 seconds")
	}
	if utf8.RuneCountInString(settings.InvitationPosterTitle) > InvitationPosterTitleMaxRunes {
		return errors.New("invitation poster title must not exceed 80 characters")
	}
	if utf8.RuneCountInString(settings.InvitationPosterSubtitle) > InvitationPosterSubtitleMaxRunes {
		return errors.New("invitation poster subtitle must not exceed 160 characters")
	}
	return nil
}

func SettingsToOptions(settings Settings) map[string]string {
	settings = NormalizeSettings(settings)
	return map[string]string{
		AffRequiredKey:              strconv.FormatBool(settings.AffRequired),
		InviteLimitUnlimitedKey:     strconv.FormatBool(settings.InviteLimitUnlimited),
		InviteLimitCountKey:         strconv.Itoa(settings.InviteLimitCount),
		ApplicationsEnabledKey:      strconv.FormatBool(settings.InvitationApplicationsEnabled),
		ReapplyCooldownSecondsKey:   strconv.FormatInt(settings.InvitationReapplyCooldownSeconds, 10),
		InvitationPosterTitleKey:    settings.InvitationPosterTitle,
		InvitationPosterSubtitleKey: settings.InvitationPosterSubtitle,
	}
}

func parseBoolOption(value string) bool {
	parsed, err := strconv.ParseBool(strings.TrimSpace(value))
	return err == nil && parsed
}

func parseBoolOptionWithDefault(value string, fallback bool) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func parseNonNegativeIntOption(value string) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || parsed < 0 {
		return 0
	}
	return parsed
}

func parseBoundedInt64OptionWithDefault(value string, fallback, minimum, maximum int64) int64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed < minimum || parsed > maximum {
		return fallback
	}
	return parsed
}
