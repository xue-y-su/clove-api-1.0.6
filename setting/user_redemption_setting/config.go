package user_redemption_setting

import (
	"errors"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

const (
	EnabledKey               = "user_redemption.enabled"
	SingleEnabledKey         = "user_redemption.single_enabled"
	RandomEnabledKey         = "user_redemption.random_enabled"
	SubscriptionEnabledKey   = "user_redemption.subscription_enabled"
	MaxActiveCodesPerUserKey = "user_redemption.max_active_codes_per_user"
	MaxClaimsPerCodeKey      = "user_redemption.max_claims_per_code"
	MaxExpiryHoursKey        = "user_redemption.max_expiry_hours"

	DefaultMaxActiveCodesPerUser = 0
	DefaultMaxClaimsPerCode      = 100
	DefaultMaxExpiryHours        = int64(30 * 24)

	MaxActiveCodesPerUser = 1000
	MaxClaimsPerCode      = 100
	MaxExpiryHours        = int64(30 * 24)
)

type Settings struct {
	Enabled               bool  `json:"enabled"`
	SingleEnabled         bool  `json:"single_enabled"`
	RandomEnabled         bool  `json:"random_enabled"`
	SubscriptionEnabled   bool  `json:"subscription_enabled"`
	MaxActiveCodesPerUser int   `json:"max_active_codes_per_user"`
	MaxClaimsPerCode      int   `json:"max_claims_per_code"`
	MaxExpiryHours        int64 `json:"max_expiry_hours"`
}

func DefaultOptions() map[string]string {
	return map[string]string{
		EnabledKey:               "true",
		SingleEnabledKey:         "true",
		RandomEnabledKey:         "true",
		SubscriptionEnabledKey:   "true",
		MaxActiveCodesPerUserKey: strconv.Itoa(DefaultMaxActiveCodesPerUser),
		MaxClaimsPerCodeKey:      strconv.Itoa(DefaultMaxClaimsPerCode),
		MaxExpiryHoursKey:        strconv.FormatInt(DefaultMaxExpiryHours, 10),
	}
}

func GetSettings() Settings {
	common.OptionMapRWMutex.RLock()
	defer common.OptionMapRWMutex.RUnlock()

	return Settings{
		Enabled:               parseBoolWithDefault(common.OptionMap[EnabledKey], true),
		SingleEnabled:         parseBoolWithDefault(common.OptionMap[SingleEnabledKey], true),
		RandomEnabled:         parseBoolWithDefault(common.OptionMap[RandomEnabledKey], true),
		SubscriptionEnabled:   parseBoolWithDefault(common.OptionMap[SubscriptionEnabledKey], true),
		MaxActiveCodesPerUser: parseIntWithDefault(common.OptionMap[MaxActiveCodesPerUserKey], DefaultMaxActiveCodesPerUser, 0, MaxActiveCodesPerUser),
		MaxClaimsPerCode:      parseIntWithDefault(common.OptionMap[MaxClaimsPerCodeKey], DefaultMaxClaimsPerCode, 2, MaxClaimsPerCode),
		MaxExpiryHours:        parseInt64WithDefault(common.OptionMap[MaxExpiryHoursKey], DefaultMaxExpiryHours, 1, MaxExpiryHours),
	}
}

func ValidateSettings(settings Settings) error {
	if settings.MaxActiveCodesPerUser < 0 || settings.MaxActiveCodesPerUser > MaxActiveCodesPerUser {
		return errors.New("user redemption active code limit must be between 0 and 1000")
	}
	if settings.MaxClaimsPerCode < 2 || settings.MaxClaimsPerCode > MaxClaimsPerCode {
		return errors.New("user redemption maximum claims must be between 2 and 100")
	}
	if settings.MaxExpiryHours < 1 || settings.MaxExpiryHours > MaxExpiryHours {
		return errors.New("user redemption maximum expiry must be between 1 and 720 hours")
	}
	return nil
}

func SettingsToOptions(settings Settings) map[string]string {
	return map[string]string{
		EnabledKey:               strconv.FormatBool(settings.Enabled),
		SingleEnabledKey:         strconv.FormatBool(settings.SingleEnabled),
		RandomEnabledKey:         strconv.FormatBool(settings.RandomEnabled),
		SubscriptionEnabledKey:   strconv.FormatBool(settings.SubscriptionEnabled),
		MaxActiveCodesPerUserKey: strconv.Itoa(settings.MaxActiveCodesPerUser),
		MaxClaimsPerCodeKey:      strconv.Itoa(settings.MaxClaimsPerCode),
		MaxExpiryHoursKey:        strconv.FormatInt(settings.MaxExpiryHours, 10),
	}
}

func parseBoolWithDefault(value string, fallback bool) bool {
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

func parseIntWithDefault(value string, fallback, minimum, maximum int) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || parsed < minimum || parsed > maximum {
		return fallback
	}
	return parsed
}

func parseInt64WithDefault(value string, fallback, minimum, maximum int64) int64 {
	parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || parsed < minimum || parsed > maximum {
		return fallback
	}
	return parsed
}
