package service

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func tokenSecretTestConfig(phase common.TokenSecretPhase) common.TokenSecretConfig {
	return common.TokenSecretConfig{
		Phase:   phase,
		HMACKey: []byte(strings.Repeat("h", 32)),
		EncryptionKeys: []common.TokenEncryptionKey{
			{Version: 7, Key: []byte(strings.Repeat("a", 32))},
			{Version: 6, Key: []byte(strings.Repeat("b", 32))},
		},
	}
}

func TestTokenSecretManagerDualWriteRoundTrip(t *testing.T) {
	manager, err := NewTokenSecretManager(tokenSecretTestConfig(common.TokenSecretPhaseDualWrite))
	require.NoError(t, err)
	token := &model.Token{Key: "custom_key-123456"}

	require.NoError(t, manager.ProtectToken(token))
	require.Equal(t, "custom_key-123456", token.Key)
	require.NotNil(t, token.KeyFingerprint)
	require.Len(t, *token.KeyFingerprint, 64)
	require.NotEmpty(t, token.KeyCiphertext)
	require.NotContains(t, token.KeyCiphertext, token.Key)
	require.Equal(t, uint32(7), token.KeyVersion)
	require.Equal(t, "cust", token.KeyPrefix)
	require.Equal(t, "3456", token.KeySuffix)
	require.True(t, manager.MatchesFingerprint("custom_key-123456", *token.KeyFingerprint))
	require.False(t, manager.MatchesFingerprint("custom_key-123457", *token.KeyFingerprint))

	revealed, err := manager.RevealTokenKey(token)
	require.NoError(t, err)
	require.Equal(t, "custom_key-123456", revealed)
}

func TestTokenSecretManagerUsesFreshNonce(t *testing.T) {
	manager, err := NewTokenSecretManager(tokenSecretTestConfig(common.TokenSecretPhaseDualWrite))
	require.NoError(t, err)
	first := &model.Token{Key: "same_key_12345678"}
	second := &model.Token{Key: "same_key_12345678"}

	require.NoError(t, manager.ProtectToken(first))
	require.NoError(t, manager.ProtectToken(second))
	require.Equal(t, first.KeyFingerprint, second.KeyFingerprint)
	require.NotEqual(t, first.KeyCiphertext, second.KeyCiphertext)
}

func TestTokenSecretManagerPhaseControlsPlaintextFallback(t *testing.T) {
	dualRead, err := NewTokenSecretManager(tokenSecretTestConfig(common.TokenSecretPhaseDualRead))
	require.NoError(t, err)
	token := &model.Token{Key: "phase_key_123456"}
	require.NoError(t, dualRead.ProtectToken(token))
	require.Empty(t, token.Key)
	revealed, err := dualRead.RevealTokenKey(token)
	require.NoError(t, err)
	require.Equal(t, "phase_key_123456", revealed)

	legacy := &model.Token{Key: "legacy_key_123456"}
	revealed, err = dualRead.RevealTokenKey(legacy)
	require.NoError(t, err)
	require.Equal(t, legacy.Key, revealed)

	cipherOnly, err := NewTokenSecretManager(tokenSecretTestConfig(common.TokenSecretPhaseCipherOnly))
	require.NoError(t, err)
	_, err = cipherOnly.RevealTokenKey(legacy)
	require.ErrorIs(t, err, ErrTokenSecretUnavailable)
}

func TestTokenSecretManagerRejectsUnknownCipherVersion(t *testing.T) {
	manager, err := NewTokenSecretManager(tokenSecretTestConfig(common.TokenSecretPhaseDualRead))
	require.NoError(t, err)
	token := &model.Token{KeyCiphertext: "not-secret", KeyVersion: 99}

	_, err = manager.RevealTokenKey(token)
	require.ErrorIs(t, err, ErrTokenSecretVersionUnsupported)
}

func TestTokenSecretManagerChecksRevocationByFingerprintAcrossKeyAliases(t *testing.T) {
	db := openTokenSecretMigrationTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.RevokedTokenFingerprint{}))
	manager, err := NewTokenSecretManager(tokenSecretTestConfig(common.TokenSecretPhaseDualWrite))
	require.NoError(t, err)

	require.NoError(t, manager.RevokeTokenKey(db, "custom_key-123456", "compromised"))
	revoked, err := manager.IsTokenKeyRevoked(db, "sk-custom_key-123456")
	require.NoError(t, err)
	require.True(t, revoked)

	var stored model.RevokedTokenFingerprint
	require.NoError(t, db.First(&stored).Error)
	require.NotEqual(t, "custom_key-123456", stored.Fingerprint)
}
