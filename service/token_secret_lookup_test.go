package service

import (
	"context"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestTokenSecretLookupUsesFingerprintBeforeLegacyPlaintext(t *testing.T) {
	db := openTokenSecretMigrationTestDB(t)
	manager, err := NewTokenSecretManager(tokenSecretTestConfig(common.TokenSecretPhaseDualRead))
	require.NoError(t, err)
	record := model.Token{UserId: 7, Key: "custom-key-with-hyphens", Status: common.TokenStatusEnabled}
	require.NoError(t, manager.ProtectToken(&record))
	require.Empty(t, record.Key)
	require.NoError(t, db.Create(&record).Error)

	found, err := manager.LookupTokenByKey(context.Background(), "custom-key-with-hyphens")
	require.NoError(t, err)
	require.Equal(t, record.Id, found.Id)
	require.Equal(t, "custom-key-with-hyphens", found.Key)

	_, err = manager.LookupTokenByKey(context.Background(), "custom-key")
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestTokenSecretLookupAllowsOnePrefixStripAndPhaseScopedLegacyFallback(t *testing.T) {
	db := openTokenSecretMigrationTestDB(t)
	legacy := model.Token{UserId: 7, Key: "legacy-key-with-hyphens", Status: common.TokenStatusEnabled}
	require.NoError(t, db.Create(&legacy).Error)

	dualRead, err := NewTokenSecretManager(tokenSecretTestConfig(common.TokenSecretPhaseDualRead))
	require.NoError(t, err)
	found, err := dualRead.LookupTokenByKey(context.Background(), "sk-legacy-key-with-hyphens")
	require.NoError(t, err)
	require.Equal(t, legacy.Id, found.Id)
	require.Equal(t, legacy.Key, found.Key)

	cipherOnly, err := NewTokenSecretManager(tokenSecretTestConfig(common.TokenSecretPhaseCipherOnly))
	require.NoError(t, err)
	_, err = cipherOnly.LookupTokenByKey(context.Background(), legacy.Key)
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestTokenSecretLookupRejectsFingerprintRowThatDoesNotConstantTimeVerify(t *testing.T) {
	db := openTokenSecretMigrationTestDB(t)
	manager, err := NewTokenSecretManager(tokenSecretTestConfig(common.TokenSecretPhaseDualRead))
	require.NoError(t, err)
	fingerprint, err := manager.Fingerprint("requested-key-value")
	require.NoError(t, err)
	record := model.Token{
		UserId:         7,
		KeyFingerprint: &fingerprint,
		KeyCiphertext:  "corrupt-ciphertext",
		KeyVersion:     7,
		Status:         common.TokenStatusEnabled,
	}
	require.NoError(t, db.Create(&record).Error)

	found, err := manager.LookupTokenByKey(context.Background(), "requested-key-value")
	require.NoError(t, err)
	require.Equal(t, record.Id, found.Id)

	otherManager, err := NewTokenSecretManager(common.TokenSecretConfig{
		Phase:   common.TokenSecretPhaseCipherOnly,
		HMACKey: []byte("iiiiiiiiiiiiiiiiiiiiiiiiiiiiiiii"),
		EncryptionKeys: []common.TokenEncryptionKey{
			{Version: 7, Key: []byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")},
		},
	})
	require.NoError(t, err)
	_, err = otherManager.LookupTokenByKey(context.Background(), "requested-key-value")
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
}
