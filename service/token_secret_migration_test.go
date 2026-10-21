package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func openTokenSecretMigrationTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	previousDB := model.DB
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Token{}))
	model.DB = db
	t.Cleanup(func() {
		model.DB = previousDB
		sqlDB, dbErr := db.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func TestTokenSecretMigratorBackfillsLegacyRecordWithoutClearingDualReadPlaintext(t *testing.T) {
	db := openTokenSecretMigrationTestDB(t)
	manager, err := NewTokenSecretManager(tokenSecretTestConfig(common.TokenSecretPhaseDualRead))
	require.NoError(t, err)
	record := model.Token{UserId: 1, Key: "legacy-key-value", Status: common.TokenStatusEnabled}
	require.NoError(t, db.Create(&record).Error)

	result := NewTokenSecretMigrator(manager).BackfillBatch(context.Background(), db, 0, 10)
	require.NoError(t, result.Error)
	require.Equal(t, 1, result.Scanned)
	require.Equal(t, 1, result.Migrated)
	require.Empty(t, result.FailedIDs)

	var reloaded model.Token
	require.NoError(t, db.First(&reloaded, record.Id).Error)
	require.Equal(t, "legacy-key-value", reloaded.Key)
	require.NotNil(t, reloaded.KeyFingerprint)
	require.NotEmpty(t, reloaded.KeyCiphertext)
	revealed, err := manager.RevealTokenKey(&reloaded)
	require.NoError(t, err)
	require.Equal(t, "legacy-key-value", revealed)
}

func TestTokenSecretMigratorLeavesLegacyKeyReadableOnEncryptionFailure(t *testing.T) {
	db := openTokenSecretMigrationTestDB(t)
	manager, err := NewTokenSecretManager(tokenSecretTestConfig(common.TokenSecretPhaseDualWrite))
	require.NoError(t, err)
	manager.random = failingTokenSecretReader{}
	record := model.Token{UserId: 1, Key: "legacy-key-value", Status: common.TokenStatusEnabled}
	require.NoError(t, db.Create(&record).Error)

	result := NewTokenSecretMigrator(manager).BackfillBatch(context.Background(), db, 0, 1)
	require.Error(t, result.Error)
	require.Equal(t, 1, result.Scanned)
	require.Equal(t, 0, result.Migrated)
	require.Equal(t, []int{record.Id}, result.FailedIDs)

	var reloaded model.Token
	require.NoError(t, db.First(&reloaded, record.Id).Error)
	require.Equal(t, "legacy-key-value", reloaded.Key)
	require.Nil(t, reloaded.KeyFingerprint)
	require.Empty(t, reloaded.KeyCiphertext)
}

func TestPrepareCipherOnlyRejectsUnprotectedLegacyRecords(t *testing.T) {
	db := openTokenSecretMigrationTestDB(t)
	manager, err := NewTokenSecretManager(tokenSecretTestConfig(common.TokenSecretPhaseCipherOnly))
	require.NoError(t, err)
	record := model.Token{UserId: 1, Key: "legacy-key-value", Status: common.TokenStatusEnabled}
	require.NoError(t, db.Create(&record).Error)

	err = PrepareCipherOnlyTokenSecrets(context.Background(), db, manager, 10)
	require.Error(t, err)
	require.Contains(t, err.Error(), fmt.Sprintf("token %d", record.Id))

	var reloaded model.Token
	require.NoError(t, db.First(&reloaded, record.Id).Error)
	require.Equal(t, "legacy-key-value", reloaded.Key)
}

func TestPrepareCipherOnlyVerifiesAllRecordsBeforeClearingPlaintext(t *testing.T) {
	db := openTokenSecretMigrationTestDB(t)
	dualWrite, err := NewTokenSecretManager(tokenSecretTestConfig(common.TokenSecretPhaseDualWrite))
	require.NoError(t, err)
	first := model.Token{UserId: 1, Key: "first-key-value-1234", Status: common.TokenStatusEnabled}
	second := model.Token{UserId: 1, Key: "second-key-value-123", Status: common.TokenStatusEnabled}
	require.NoError(t, dualWrite.ProtectToken(&first))
	require.NoError(t, dualWrite.ProtectToken(&second))
	require.NoError(t, db.Create(&first).Error)
	require.NoError(t, db.Create(&second).Error)
	require.NoError(t, db.Model(&second).Update("key_version", 99).Error)

	cipherOnly, err := NewTokenSecretManager(tokenSecretTestConfig(common.TokenSecretPhaseCipherOnly))
	require.NoError(t, err)
	err = PrepareCipherOnlyTokenSecrets(context.Background(), db, cipherOnly, 1)
	require.Error(t, err)

	var unchanged model.Token
	require.NoError(t, db.First(&unchanged, first.Id).Error)
	require.Equal(t, "first-key-value-1234", unchanged.Key)

	require.NoError(t, db.Model(&second).Update("key_version", 7).Error)
	require.NoError(t, PrepareCipherOnlyTokenSecrets(context.Background(), db, cipherOnly, 1))
	var nullCount int64
	require.NoError(t, db.Model(&model.Token{}).Where("key IS NULL").Count(&nullCount).Error)
	require.EqualValues(t, 2, nullCount)
}

func TestPrepareCipherOnlySupportsMySQLReservedKeyColumn(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("TEST_MYSQL_DSN"))
	if dsn == "" {
		t.Skip("set TEST_MYSQL_DSN to run MySQL token secret migration compatibility test")
	}

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Token{}))
	require.NoError(t, db.Unscoped().Where("name = ?", "mysql-cipher-only-secret-query-test").Delete(&model.Token{}).Error)
	t.Cleanup(func() {
		_ = db.Unscoped().Where("name = ?", "mysql-cipher-only-secret-query-test").Delete(&model.Token{}).Error
		_ = sqlDB.Close()
	})

	dualWrite, err := NewTokenSecretManager(tokenSecretTestConfig(common.TokenSecretPhaseDualWrite))
	require.NoError(t, err)
	record := model.Token{
		UserId: 1,
		Name:   "mysql-cipher-only-secret-query-test",
		Key:    "mysql-cipher-only-legacy-key",
		Status: common.TokenStatusEnabled,
	}
	require.NoError(t, dualWrite.ProtectToken(&record))
	require.NoError(t, db.Create(&record).Error)

	cipherOnly, err := NewTokenSecretManager(tokenSecretTestConfig(common.TokenSecretPhaseCipherOnly))
	require.NoError(t, err)
	require.NoError(t, PrepareCipherOnlyTokenSecrets(context.Background(), db, cipherOnly, 10))

	var reloaded model.Token
	require.NoError(t, db.First(&reloaded, record.Id).Error)
	require.Empty(t, reloaded.Key)
}

type failingTokenSecretReader struct{}

func (failingTokenSecretReader) Read([]byte) (int, error) {
	return 0, errors.New("nonce source failed")
}
