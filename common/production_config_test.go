package common

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func productionTestSecret() string {
	return strings.Repeat("x", minimumProductionSecretBytes)
}

func TestValidateProductionSecretsRequiresSessionSecret(t *testing.T) {
	t.Setenv("DEPLOYMENT_MODE", "production")
	t.Setenv("SESSION_SECRET", "")

	err := validateProductionSecrets()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "SESSION_SECRET")
}

func TestValidateProductionSecretsRejectsWeakSessionSecrets(t *testing.T) {
	t.Setenv("DEPLOYMENT_MODE", "production")

	for _, secret := range []string{
		"short",
		"random_string",
		"replace_with_at_least_32_random_bytes",
	} {
		t.Run(secret, func(t *testing.T) {
			t.Setenv("SESSION_SECRET", secret)
			assert.Error(t, validateProductionSecrets())
		})
	}
}

func TestValidateProductionSecretsAcceptsStrongSessionSecret(t *testing.T) {
	t.Setenv("DEPLOYMENT_MODE", "production")
	t.Setenv("SESSION_SECRET", productionTestSecret())

	assert.NoError(t, validateProductionSecrets())
}

func TestValidateProductionSecretsRejectsWeakExplicitCryptoSecret(t *testing.T) {
	t.Setenv("DEPLOYMENT_MODE", "production")
	t.Setenv("SESSION_SECRET", productionTestSecret())
	t.Setenv("CRYPTO_SECRET", "weak")

	assert.Error(t, validateProductionSecrets())
}

func TestValidateProductionSecretsDoesNotRestrictDevelopment(t *testing.T) {
	t.Setenv("DEPLOYMENT_MODE", "development")
	t.Setenv("SESSION_SECRET", "")

	assert.NoError(t, validateProductionSecrets())
}

func TestValidateProductionRuntimeSecurityRejectsUnsafeOverrides(t *testing.T) {
	unsafeSettings := map[string]string{
		"DEBUG":                         "true",
		"GIN_MODE":                      "debug",
		"TLS_INSECURE_SKIP_VERIFY":      "true",
		"SMTP_INSECURE_SKIP_VERIFY":     "true",
		"SMTP_TLS_INSECURE_SKIP_VERIFY": "true",
	}
	for name, value := range unsafeSettings {
		t.Run(name, func(t *testing.T) {
			t.Setenv("DEPLOYMENT_MODE", "production")
			t.Setenv(name, value)

			err := validateProductionRuntimeSecurity()

			require.Error(t, err)
			assert.Contains(t, err.Error(), name)
		})
	}
}

func TestValidateProductionRuntimeSecurityRestrictsPprofToLoopback(t *testing.T) {
	t.Setenv("DEPLOYMENT_MODE", "production")
	t.Setenv("ENABLE_PPROF", "true")

	t.Setenv("PPROF_ADDR", "0.0.0.0:8005")
	assert.Error(t, validateProductionRuntimeSecurity())

	t.Setenv("PPROF_ADDR", "127.0.0.1:8005")
	assert.NoError(t, validateProductionRuntimeSecurity())

	t.Setenv("PPROF_ADDR", "[::1]:8005")
	assert.NoError(t, validateProductionRuntimeSecurity())
}

func TestValidateProductionRuntimeSecurityDoesNotRestrictDevelopment(t *testing.T) {
	t.Setenv("DEPLOYMENT_MODE", "development")
	t.Setenv("DEBUG", "true")
	t.Setenv("TLS_INSECURE_SKIP_VERIFY", "true")
	t.Setenv("ENABLE_PPROF", "true")
	t.Setenv("PPROF_ADDR", "0.0.0.0:8005")

	assert.NoError(t, validateProductionRuntimeSecurity())
}
