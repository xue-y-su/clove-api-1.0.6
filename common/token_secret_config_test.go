package common

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadTokenSecretConfigAllowsEmptySchemaPhase(t *testing.T) {
	t.Setenv("TOKEN_SECRET_PHASE", "")
	t.Setenv("TOKEN_HMAC_KEY", "")
	t.Setenv("TOKEN_ENCRYPTION_KEYS", "")

	config, err := LoadTokenSecretConfig()
	require.NoError(t, err)
	require.Equal(t, TokenSecretPhaseSchema, config.Phase)
	require.Empty(t, config.HMACKey)
	require.Empty(t, config.EncryptionKeys)
}

func TestLoadTokenSecretConfigParsesStableVersionedKeys(t *testing.T) {
	hmacKey := base64.StdEncoding.EncodeToString([]byte(strings.Repeat("h", 32)))
	currentKey := base64.StdEncoding.EncodeToString([]byte(strings.Repeat("a", 32)))
	previousKey := base64.StdEncoding.EncodeToString([]byte(strings.Repeat("b", 32)))
	t.Setenv("TOKEN_SECRET_PHASE", "dual-write")
	t.Setenv("TOKEN_HMAC_KEY", hmacKey)
	t.Setenv("TOKEN_ENCRYPTION_KEYS", "7:"+currentKey+",6:"+previousKey)

	config, err := LoadTokenSecretConfig()
	require.NoError(t, err)
	require.Equal(t, TokenSecretPhaseDualWrite, config.Phase)
	require.Equal(t, []byte(strings.Repeat("h", 32)), config.HMACKey)
	require.Len(t, config.EncryptionKeys, 2)
	require.Equal(t, uint32(7), config.EncryptionKeys[0].Version)
	require.Equal(t, uint32(6), config.EncryptionKeys[1].Version)
}

func TestLoadTokenSecretConfigRejectsUnsafeOrAmbiguousConfiguration(t *testing.T) {
	validHMAC := base64.StdEncoding.EncodeToString([]byte(strings.Repeat("h", 32)))
	validCipher := base64.StdEncoding.EncodeToString([]byte(strings.Repeat("a", 32)))

	tests := []struct {
		name       string
		phase      string
		hmacKey    string
		cipherKeys string
	}{
		{name: "unknown phase", phase: "enabled", hmacKey: validHMAC, cipherKeys: "1:" + validCipher},
		{name: "missing keys", phase: "cipher-only"},
		{name: "partial schema keys", phase: "schema", hmacKey: validHMAC},
		{name: "short hmac", phase: "dual-write", hmacKey: base64.StdEncoding.EncodeToString([]byte("short")), cipherKeys: "1:" + validCipher},
		{name: "bad cipher encoding", phase: "dual-read", hmacKey: validHMAC, cipherKeys: "1:not-base64"},
		{name: "duplicate versions", phase: "dual-write", hmacKey: validHMAC, cipherKeys: "1:" + validCipher + ",1:" + validCipher},
		{name: "zero version", phase: "dual-write", hmacKey: validHMAC, cipherKeys: "0:" + validCipher},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("TOKEN_SECRET_PHASE", test.phase)
			t.Setenv("TOKEN_HMAC_KEY", test.hmacKey)
			t.Setenv("TOKEN_ENCRYPTION_KEYS", test.cipherKeys)

			_, err := LoadTokenSecretConfig()
			require.Error(t, err)
			if test.hmacKey != "" {
				require.NotContains(t, err.Error(), test.hmacKey)
			}
			require.NotContains(t, err.Error(), validCipher)
		})
	}
}
