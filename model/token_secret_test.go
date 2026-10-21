package model

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestGetTokenByFingerprintUsesExactFingerprint(t *testing.T) {
	db := openTokenModelTestDB(t)
	fingerprint := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	record := Token{
		UserId:         7,
		Key:            "legacy-key-must-not-be-queried",
		KeyFingerprint: &fingerprint,
		Status:         common.TokenStatusEnabled,
	}
	require.NoError(t, db.Create(&record).Error)

	token, err := GetTokenByFingerprint(context.Background(), fingerprint)
	require.NoError(t, err)
	require.Equal(t, record.Id, token.Id)

	_, err = GetTokenByFingerprint(context.Background(), "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestUpdateTokenSecretProjectionPreservesLegacyKeyUnlessExplicitlyCleared(t *testing.T) {
	db := openTokenModelTestDB(t)
	record := Token{UserId: 7, Key: "legacy-key-value", Status: common.TokenStatusEnabled}
	require.NoError(t, db.Create(&record).Error)
	fingerprint := "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"

	err := UpdateTokenSecretProjection(context.Background(), db, record.Id, TokenSecretProjection{
		Fingerprint: fingerprint,
		Ciphertext:  "ciphertext",
		Version:     3,
		Prefix:      "lega",
		Suffix:      "alue",
	}, false)
	require.NoError(t, err)

	var reloaded Token
	require.NoError(t, db.First(&reloaded, record.Id).Error)
	require.Equal(t, "legacy-key-value", reloaded.Key)
	require.Equal(t, fingerprint, *reloaded.KeyFingerprint)

	require.NoError(t, UpdateTokenSecretProjection(context.Background(), db, record.Id, TokenSecretProjection{
		Fingerprint: fingerprint,
		Ciphertext:  "ciphertext",
		Version:     3,
		Prefix:      "lega",
		Suffix:      "alue",
	}, true))
	reloaded = Token{}
	require.NoError(t, db.First(&reloaded, record.Id).Error)
	require.Empty(t, reloaded.Key)
}

func TestCipherOnlyRecordsAllowMultipleNullLegacyKeys(t *testing.T) {
	db := openTokenModelTestDB(t)
	require.NoError(t, EnsureTokenSecretIndexes(db))
	firstFingerprint := "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"
	secondFingerprint := "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"

	require.NoError(t, db.Create(&Token{UserId: 1, KeyFingerprint: &firstFingerprint, KeyCiphertext: "one", KeyVersion: 1}).Error)
	require.NoError(t, db.Create(&Token{UserId: 2, KeyFingerprint: &secondFingerprint, KeyCiphertext: "two", KeyVersion: 1}).Error)

	var nullCount int64
	require.NoError(t, db.Model(&Token{}).Where("key IS NULL").Count(&nullCount).Error)
	require.EqualValues(t, 2, nullCount)
}

func TestTokenFingerprintUniqueIndexRejectsDuplicateNonNullValues(t *testing.T) {
	db := openTokenModelTestDB(t)
	require.NoError(t, EnsureTokenSecretIndexes(db))
	fingerprint := "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
	require.NoError(t, db.Create(&Token{UserId: 1, Key: "first-legacy-key", KeyFingerprint: &fingerprint}).Error)
	require.Error(t, db.Create(&Token{UserId: 2, Key: "second-legacy-key", KeyFingerprint: &fingerprint}).Error)
}

func TestEncryptedTokenMaskedKeyUsesStoredAffixes(t *testing.T) {
	token := Token{KeyPrefix: "cust", KeySuffix: "3456"}
	require.Equal(t, "cust**********3456", token.GetMaskedKey())
}

func TestListLegacyTokenSecretsSupportsMySQLReservedKeyColumn(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("TEST_MYSQL_DSN"))
	if dsn == "" {
		t.Skip("set TEST_MYSQL_DSN to run MySQL token secret query compatibility test")
	}

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Token{}))
	require.NoError(t, db.Unscoped().Where("name = ?", "mysql-legacy-secret-query-test").Delete(&Token{}).Error)
	t.Cleanup(func() {
		_ = db.Unscoped().Where("name = ?", "mysql-legacy-secret-query-test").Delete(&Token{}).Error
		_ = sqlDB.Close()
	})

	record := Token{
		UserId: 1,
		Name:   "mysql-legacy-secret-query-test",
		Key:    "legacy-key-value",
		Status: common.TokenStatusEnabled,
	}
	require.NoError(t, db.Create(&record).Error)

	tokens, err := ListLegacyTokenSecrets(context.Background(), db, record.Id-1, 10)
	require.NoError(t, err)
	require.Len(t, tokens, 1)
	require.Equal(t, record.Id, tokens[0].Id)
}
