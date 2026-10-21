package model

import (
	"bytes"
	"fmt"
	"log"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func openTokenModelTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	gin.SetMode(gin.TestMode)
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)

	previousDB := DB
	previousRedisEnabled := common.RedisEnabled
	common.RedisEnabled = false

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)

	DB = db
	require.NoError(t, db.AutoMigrate(&Token{}))

	t.Cleanup(func() {
		DB = previousDB
		common.RedisEnabled = previousRedisEnabled
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func TestTokenKeyLookupCandidatesPreserveHyphenatedKeys(t *testing.T) {
	require.Equal(t, []string{"abc-def-ghi"}, TokenKeyLookupCandidates(" abc-def-ghi "))
	require.Equal(t, []string{"sk-custom_key-123456", "custom_key-123456"}, TokenKeyLookupCandidates("sk-custom_key-123456"))
	require.Equal(t, []string{"SK-custom_key-123456", "custom_key-123456"}, TokenKeyLookupCandidates("SK-custom_key-123456"))
	require.Nil(t, TokenKeyLookupCandidates("  "))
}

func TestTokenKeyPresentationDistinguishesLegacyAndFullCredentials(t *testing.T) {
	legacy := &Token{
		Key:       "legacy_key_123456",
		KeyFormat: TokenKeyFormatLegacySK,
	}
	full := &Token{
		Key:       "custom_key_123456",
		KeyFormat: TokenKeyFormatFull,
	}
	prefixed := &Token{
		Key:       "sk-generated_key_123456",
		KeyFormat: TokenKeyFormatFull,
	}

	require.Equal(t, "sk-legacy_key_123456", legacy.GetFullKey())
	require.Equal(t, "sk-lega**********3456", legacy.GetMaskedKey())
	require.Equal(t, "custom_key_123456", full.GetFullKey())
	require.Equal(t, "cust**********3456", full.GetMaskedKey())
	require.Equal(t, "sk-generated_key_123456", prefixed.GetFullKey())
	require.Equal(t, "sk-g**********3456", prefixed.GetMaskedKey())
}

func TestMigrateTokenKeyFormatsMarksExistingPrefixedKeysAsFull(t *testing.T) {
	db := openTokenModelTestDB(t)
	legacy := &Token{UserId: 1, Key: "legacy_key_123456", KeyFormat: TokenKeyFormatLegacySK}
	full := &Token{UserId: 1, Key: "sk-generated_key_123456", KeyFormat: TokenKeyFormatLegacySK}
	mixedCaseFull := &Token{UserId: 1, Key: "Sk-custom_key_123456", KeyFormat: TokenKeyFormatLegacySK}
	require.NoError(t, db.Create(legacy).Error)
	require.NoError(t, db.Create(full).Error)
	require.NoError(t, db.Create(mixedCaseFull).Error)

	require.NoError(t, MigrateTokenKeyFormats(db))
	require.NoError(t, db.First(legacy, legacy.Id).Error)
	require.NoError(t, db.First(full, full.Id).Error)
	require.NoError(t, db.First(mixedCaseFull, mixedCaseFull.Id).Error)
	require.Equal(t, TokenKeyFormatLegacySK, legacy.KeyFormat)
	require.Equal(t, TokenKeyFormatFull, full.KeyFormat)
	require.Equal(t, TokenKeyFormatFull, mixedCaseFull.KeyFormat)
}

func TestDeletingTokensReplacesKeyMaterialWithDistinctTombstones(t *testing.T) {
	db := openTokenModelTestDB(t)
	firstFingerprint := strings.Repeat("a", 64)
	secondFingerprint := strings.Repeat("b", 64)
	first := &Token{
		UserId:         1,
		Key:            "first_custom_key_123",
		KeyFingerprint: &firstFingerprint,
		Status:         common.TokenStatusEnabled,
	}
	second := &Token{
		UserId:         1,
		Key:            "second_custom_key_123",
		KeyFingerprint: &secondFingerprint,
		Status:         common.TokenStatusEnabled,
	}
	require.NoError(t, db.Create(first).Error)
	require.NoError(t, db.Create(second).Error)

	require.NoError(t, first.Delete())
	require.NoError(t, second.Delete())

	var deleted []Token
	require.NoError(t, db.Unscoped().Where("id IN ?", []int{first.Id, second.Id}).Order("id ASC").Find(&deleted).Error)
	require.Len(t, deleted, 2)
	for _, token := range deleted {
		require.True(t, token.DeletedAt.Valid)
		require.NotEmpty(t, token.Key)
		require.NotContains(t, token.Key, "custom_key")
		require.Nil(t, token.KeyFingerprint)
		require.Empty(t, token.KeyCiphertext)
		require.Empty(t, token.KeyPrefix)
		require.Empty(t, token.KeySuffix)
	}
	require.NotEqual(t, deleted[0].Key, deleted[1].Key)

	require.NoError(t, db.Create(&Token{UserId: 1, Key: "first_custom_key_123", Status: common.TokenStatusEnabled}).Error)
}

func TestGetTokenByKeyWithPrefixFallbackDoesNotTruncateHyphenatedKeys(t *testing.T) {
	db := openTokenModelTestDB(t)

	require.NoError(t, db.Create(&Token{UserId: 1, Key: "abc", Status: common.TokenStatusEnabled}).Error)
	require.NoError(t, db.Create(&Token{UserId: 1, Key: "custom_key-123456", Status: common.TokenStatusEnabled}).Error)

	_, err := GetTokenByKeyWithPrefixFallback("abc-def-ghi", false)
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)

	token, err := GetTokenByKeyWithPrefixFallback("sk-custom_key-123456", false)
	require.NoError(t, err)
	require.Equal(t, "custom_key-123456", token.Key)

	token, err = GetTokenByKeyNoSQLLogWithPrefixFallback("SK-custom_key-123456")
	require.NoError(t, err)
	require.Equal(t, "custom_key-123456", token.Key)
}

func TestValidateUserTokenDoesNotLogExpectedUnknownKey(t *testing.T) {
	var output bytes.Buffer
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.New(log.New(&output, "", 0), logger.Config{
			LogLevel:                  logger.Warn,
			IgnoreRecordNotFoundError: false,
			Colorful:                  false,
		}),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Token{}))

	previousDB := DB
	previousRedisEnabled := common.RedisEnabled
	DB = db
	common.RedisEnabled = false
	t.Cleanup(func() {
		DB = previousDB
		common.RedisEnabled = previousRedisEnabled
		sqlDB, dbErr := db.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
	})
	output.Reset()

	token, err := ValidateUserToken("unknown-custom-key")
	require.Nil(t, token)
	require.ErrorIs(t, err, ErrTokenInvalid)
	require.NotContains(t, output.String(), "record not found")
}
