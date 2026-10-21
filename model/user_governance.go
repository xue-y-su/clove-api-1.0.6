package model

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strings"
	"unicode"

	"github.com/QuantumNous/new-api/common"
)

const (
	DefaultDisplayNameMaxLength       = 40
	DefaultDisplayNameCooldownSeconds = int64(7 * 24 * 60 * 60)
)

type AvatarUploadPolicy struct {
	Enabled        bool     `json:"enabled"`
	Endpoint       string   `json:"endpoint"`
	AllowedDomains []string `json:"allowed_domains"`
}

var usernamePattern = regexp.MustCompile(`^[\p{Han}A-Za-z0-9_-]{1,20}$`)

func governanceOption(key string) string {
	common.OptionMapRWMutex.RLock()
	defer common.OptionMapRWMutex.RUnlock()
	return common.OptionMap[key]
}

func GetAvatarUploadPolicy() AvatarUploadPolicy {
	common.OptionMapRWMutex.RLock()
	defer common.OptionMapRWMutex.RUnlock()
	policy := AvatarUploadPolicy{
		Enabled:  strings.EqualFold(strings.TrimSpace(common.OptionMap["AvatarUploadEnabled"]), "true"),
		Endpoint: strings.TrimSpace(common.OptionMap["AvatarUploadEndpoint"]),
	}
	seen := map[string]struct{}{}
	for _, value := range strings.Split(common.OptionMap["AvatarAllowedDomains"], ",") {
		domain := strings.ToLower(strings.TrimSpace(value))
		if domain == "" {
			continue
		}
		if _, exists := seen[domain]; exists {
			continue
		}
		seen[domain] = struct{}{}
		policy.AllowedDomains = append(policy.AllowedDomains, domain)
	}
	return policy
}

func displayNamePolicy() (int, int64) {
	maxLength := DefaultDisplayNameMaxLength
	cooldown := DefaultDisplayNameCooldownSeconds
	if value := governanceOption("UserDisplayNameMaxLength"); value != "" {
		if _, err := fmt.Sscanf(value, "%d", &maxLength); err != nil || maxLength < 1 || maxLength > 100 {
			maxLength = DefaultDisplayNameMaxLength
		}
	}
	if value := governanceOption("UserDisplayNameCooldownSeconds"); value != "" {
		if _, err := fmt.Sscanf(value, "%d", &cooldown); err != nil || cooldown < 0 || cooldown > 365*24*60*60 {
			cooldown = DefaultDisplayNameCooldownSeconds
		}
	}
	return maxLength, cooldown
}

func ValidateUsername(username string) error {
	if !usernamePattern.MatchString(username) {
		return errors.New("username must be 1-20 Chinese characters, ASCII letters, numbers, underscores, or hyphens")
	}
	return nil
}

func ValidateDisplayName(displayName string) error {
	displayName = strings.TrimSpace(displayName)
	maxLength, _ := displayNamePolicy()
	if displayName == "" || len([]rune(displayName)) > maxLength {
		return fmt.Errorf("display name must be between 1 and %d characters", maxLength)
	}
	for _, r := range displayName {
		if unicode.IsControl(r) || r == '\u200e' || r == '\u200f' || (r >= '\u202a' && r <= '\u202e') || (r >= '\u2066' && r <= '\u2069') {
			return errors.New("display name contains unsupported characters")
		}
	}
	lower := strings.ToLower(displayName)
	if strings.Contains(lower, "<script") || strings.Contains(displayName, "<") || strings.Contains(displayName, ">") {
		return errors.New("display name must not contain HTML")
	}
	if strings.Contains(displayName, "  ") || strings.HasPrefix(displayName, " ") || strings.HasSuffix(displayName, " ") {
		return errors.New("display name contains excessive whitespace")
	}
	return nil
}

func ValidateAvatarURL(rawURL string) error {
	if rawURL == "" {
		return nil
	}
	policy := GetAvatarUploadPolicy()
	if !policy.Enabled {
		return errors.New("avatar uploads are disabled")
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme != "https" || parsed.User != nil || parsed.Hostname() == "" {
		return errors.New("avatar URL must be an allowed HTTPS URL")
	}
	host := strings.ToLower(parsed.Hostname())
	if net.ParseIP(host) != nil || host == "localhost" || parsed.Port() != "" || parsed.Fragment != "" {
		return errors.New("avatar URL host is not allowed")
	}
	for _, domain := range policy.AllowedDomains {
		if host == domain {
			return nil
		}
	}
	return errors.New("avatar URL host is not allowed")
}
