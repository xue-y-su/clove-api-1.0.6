package common

import (
	"fmt"
	"net/url"
	"os"
	"strings"
)

func InitSessionCookieSettings() error {
	secureRaw := strings.TrimSpace(os.Getenv("SESSION_COOKIE_SECURE"))
	trustedURLsRaw := strings.TrimSpace(os.Getenv("SESSION_COOKIE_TRUSTED_URL"))
	frontendBaseURL := strings.TrimSpace(os.Getenv("FRONTEND_BASE_URL"))

	SessionCookieSecure = false
	SessionCookieTrustedURLs = nil

	if secureRaw == "" && trustedURLsRaw == "" && isHTTPSURL(frontendBaseURL) {
		secureRaw = "true"
		trustedURLsRaw = frontendBaseURL
	}

	if secureRaw == "" || strings.EqualFold(secureRaw, "false") {
		if trustedURLsRaw != "" {
			return fmt.Errorf("SESSION_COOKIE_TRUSTED_URL requires SESSION_COOKIE_SECURE=true")
		}
		return nil
	}

	if !strings.EqualFold(secureRaw, "true") {
		return fmt.Errorf("SESSION_COOKIE_SECURE must be true or false")
	}

	if trustedURLsRaw == "" {
		return fmt.Errorf("SESSION_COOKIE_SECURE=true requires SESSION_COOKIE_TRUSTED_URL")
	}

	trustedURLs, err := parseSessionCookieTrustedURLs(trustedURLsRaw)
	if err != nil {
		return err
	}
	SessionCookieTrustedURLs = trustedURLs
	SessionCookieSecure = true
	return nil
}

func parseSessionCookieTrustedURLs(value string) ([]string, error) {
	trustedURLs := strings.Split(value, ",")
	parsedURLs := make([]string, 0, len(trustedURLs))
	for _, trustedURL := range trustedURLs {
		trustedURL = strings.TrimSpace(trustedURL)
		if trustedURL == "" {
			return nil, fmt.Errorf("SESSION_COOKIE_TRUSTED_URL contains an empty URL")
		}
		parsedURL, err := url.Parse(trustedURL)
		if err != nil {
			return nil, fmt.Errorf("invalid SESSION_COOKIE_TRUSTED_URL: %w", err)
		}
		if parsedURL.Scheme != "https" || parsedURL.Host == "" {
			return nil, fmt.Errorf("SESSION_COOKIE_TRUSTED_URL must contain only https URLs with hosts")
		}
		parsedURLs = append(parsedURLs, trustedURL)
	}
	return parsedURLs, nil
}

func isHTTPSURL(value string) bool {
	parsedURL, err := url.Parse(value)
	return err == nil && parsedURL.Scheme == "https" && parsedURL.Host != ""
}
