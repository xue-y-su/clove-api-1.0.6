package user_governance_setting

import (
	"errors"
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

const (
	AvatarUploadEnabledKey        = "AvatarUploadEnabled"
	AvatarUploadEndpointKey       = "AvatarUploadEndpoint"
	AvatarAllowedDomainsKey       = "AvatarAllowedDomains"
	DisplayNameMaxLengthKey       = "UserDisplayNameMaxLength"
	DisplayNameCooldownSecondsKey = "UserDisplayNameCooldownSeconds"

	DefaultAvatarUploadEndpoint       = "https://img.scdn.io/api/v1.php"
	DefaultAvatarAllowedDomains       = "img.scdn.io,img.cdn1.vip"
	DefaultDisplayNameMaxLength       = 40
	DefaultDisplayNameCooldownSeconds = int64(7 * 24 * 60 * 60)
	MaxDisplayNameCooldownSeconds     = int64(365 * 24 * 60 * 60)
)

type Settings struct {
	AvatarUploadEnabled        bool     `json:"avatar_upload_enabled"`
	AvatarUploadEndpoint       string   `json:"avatar_upload_endpoint"`
	AvatarAllowedDomains       []string `json:"avatar_allowed_domains"`
	DisplayNameMaxLength       int      `json:"display_name_max_length"`
	DisplayNameCooldownSeconds int64    `json:"display_name_cooldown_seconds"`
}

func GetSettings() Settings {
	common.OptionMapRWMutex.RLock()
	defer common.OptionMapRWMutex.RUnlock()

	return NormalizeSettings(Settings{
		AvatarUploadEnabled: parseBoolWithDefault(common.OptionMap[AvatarUploadEnabledKey], true),
		AvatarUploadEndpoint: valueWithDefault(
			common.OptionMap[AvatarUploadEndpointKey],
			DefaultAvatarUploadEndpoint,
		),
		AvatarAllowedDomains: strings.Split(valueWithDefault(
			common.OptionMap[AvatarAllowedDomainsKey],
			DefaultAvatarAllowedDomains,
		), ","),
		DisplayNameMaxLength: parseIntWithDefault(
			common.OptionMap[DisplayNameMaxLengthKey],
			DefaultDisplayNameMaxLength,
			1,
			100,
		),
		DisplayNameCooldownSeconds: parseInt64WithDefault(
			common.OptionMap[DisplayNameCooldownSecondsKey],
			DefaultDisplayNameCooldownSeconds,
			0,
			MaxDisplayNameCooldownSeconds,
		),
	})
}

func NormalizeSettings(settings Settings) Settings {
	settings.AvatarUploadEndpoint = strings.TrimSpace(settings.AvatarUploadEndpoint)
	seen := make(map[string]struct{}, len(settings.AvatarAllowedDomains))
	domains := make([]string, 0, len(settings.AvatarAllowedDomains))
	for _, value := range settings.AvatarAllowedDomains {
		domain := strings.ToLower(strings.TrimSpace(value))
		if domain == "" {
			continue
		}
		if _, exists := seen[domain]; exists {
			continue
		}
		seen[domain] = struct{}{}
		domains = append(domains, domain)
	}
	settings.AvatarAllowedDomains = domains
	return settings
}

func ValidateSettings(settings Settings) error {
	settings = NormalizeSettings(settings)
	if settings.DisplayNameMaxLength < 1 || settings.DisplayNameMaxLength > 100 {
		return errors.New("display name maximum length must be between 1 and 100")
	}
	if settings.DisplayNameCooldownSeconds < 0 || settings.DisplayNameCooldownSeconds > MaxDisplayNameCooldownSeconds {
		return errors.New("display name cooldown must be between 0 and 31536000 seconds")
	}
	if !settings.AvatarUploadEnabled {
		return nil
	}
	endpoint, err := url.Parse(settings.AvatarUploadEndpoint)
	if err != nil || endpoint.Scheme != "https" || endpoint.User != nil || endpoint.Hostname() == "" {
		return errors.New("avatar upload endpoint must be a valid HTTPS URL")
	}
	if len(settings.AvatarAllowedDomains) == 0 {
		return errors.New("avatar allowed domains cannot be empty while uploads are enabled")
	}
	for _, domain := range settings.AvatarAllowedDomains {
		if net.ParseIP(domain) != nil || domain == "localhost" || strings.ContainsAny(domain, "/:@") {
			return errors.New("avatar allowed domain is invalid")
		}
		parsed, parseErr := url.Parse("https://" + domain)
		if parseErr != nil || parsed.Hostname() != domain || parsed.Port() != "" {
			return errors.New("avatar allowed domain is invalid")
		}
	}
	return nil
}

func SettingsToOptions(settings Settings) map[string]string {
	settings = NormalizeSettings(settings)
	return map[string]string{
		AvatarUploadEnabledKey:        strconv.FormatBool(settings.AvatarUploadEnabled),
		AvatarUploadEndpointKey:       settings.AvatarUploadEndpoint,
		AvatarAllowedDomainsKey:       strings.Join(settings.AvatarAllowedDomains, ","),
		DisplayNameMaxLengthKey:       strconv.Itoa(settings.DisplayNameMaxLength),
		DisplayNameCooldownSecondsKey: strconv.FormatInt(settings.DisplayNameCooldownSeconds, 10),
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

func valueWithDefault(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
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
