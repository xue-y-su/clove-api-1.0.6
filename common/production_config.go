package common

import (
	"fmt"
	"net"
	"os"
	"strings"
)

const minimumProductionSecretBytes = 32

func validateProductionSecrets() error {
	if !strings.EqualFold(strings.TrimSpace(os.Getenv("DEPLOYMENT_MODE")), "production") {
		return nil
	}

	sessionSecret := os.Getenv("SESSION_SECRET")
	if err := validateProductionSecret("SESSION_SECRET", sessionSecret); err != nil {
		return err
	}
	if cryptoSecret := os.Getenv("CRYPTO_SECRET"); cryptoSecret != "" {
		if err := validateProductionSecret("CRYPTO_SECRET", cryptoSecret); err != nil {
			return err
		}
	}
	return nil
}

func validateProductionSecret(name, value string) error {
	placeholder := strings.ToLower(strings.TrimSpace(value))
	if placeholder == "" {
		return fmt.Errorf("%s is required when DEPLOYMENT_MODE=production", name)
	}
	if len([]byte(value)) < minimumProductionSecretBytes {
		return fmt.Errorf("%s must contain at least %d bytes when DEPLOYMENT_MODE=production", name, minimumProductionSecretBytes)
	}
	if placeholder == "random_string" || strings.HasPrefix(placeholder, "replace_with_") {
		return fmt.Errorf("%s must not use a documented placeholder value", name)
	}
	return nil
}

func validateProductionCookieSettings() error {
	if !strings.EqualFold(strings.TrimSpace(os.Getenv("DEPLOYMENT_MODE")), "production") {
		return nil
	}
	if !SessionCookieSecure {
		return fmt.Errorf("SESSION_COOKIE_SECURE must be true when DEPLOYMENT_MODE=production. Set SESSION_COOKIE_SECURE=true with SESSION_COOKIE_TRUSTED_URL=https://your-frontend-domain, or set FRONTEND_BASE_URL to an https URL")
	}
	return nil
}

func validateProductionRuntimeSecurity() error {
	if !strings.EqualFold(strings.TrimSpace(os.Getenv("DEPLOYMENT_MODE")), "production") {
		return nil
	}

	for _, name := range []string{
		"DEBUG",
		"TLS_INSECURE_SKIP_VERIFY",
		"SMTP_INSECURE_SKIP_VERIFY",
		"SMTP_TLS_INSECURE_SKIP_VERIFY",
	} {
		if strings.EqualFold(strings.TrimSpace(os.Getenv(name)), "true") {
			return fmt.Errorf("%s must not be true when DEPLOYMENT_MODE=production", name)
		}
	}
	if strings.EqualFold(strings.TrimSpace(os.Getenv("GIN_MODE")), "debug") {
		return fmt.Errorf("GIN_MODE must not be debug when DEPLOYMENT_MODE=production")
	}
	if strings.EqualFold(strings.TrimSpace(os.Getenv("ENABLE_PPROF")), "true") && !isLoopbackListenAddress(os.Getenv("PPROF_ADDR")) {
		return fmt.Errorf("PPROF_ADDR must use a loopback address when ENABLE_PPROF=true and DEPLOYMENT_MODE=production")
	}
	return nil
}

func isLoopbackListenAddress(address string) bool {
	address = strings.TrimSpace(address)
	if address == "" {
		return true
	}
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return false
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
