package model

import (
	"errors"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupHardDeleteUserTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/hard-delete-user.db"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&User{},
		&Token{},
		&UserOAuthBinding{},
		&TwoFA{},
		&TwoFABackupCode{},
		&PasskeyCredential{},
		&WalletLedgerEntry{},
		&Log{},
	))

	previousDB, previousLogDB := DB, LOG_DB
	previousRedisEnabled := common.RedisEnabled
	DB, LOG_DB = db, db
	common.RedisEnabled = false
	t.Cleanup(func() {
		sqlDB, sqlErr := db.DB()
		if sqlErr == nil {
			_ = sqlDB.Close()
		}
		DB, LOG_DB = previousDB, previousLogDB
		common.RedisEnabled = previousRedisEnabled
	})
	return db
}

func seedHardDeleteCredentials(t *testing.T, db *gorm.DB, userID int) {
	t.Helper()
	require.NoError(t, db.Create(&User{
		Id: userID, Username: "hard-delete-user", Password: "password-hash", AffCode: "hard-delete-aff",
	}).Error)
	require.NoError(t, db.Create(&Token{UserId: userID, Key: "hard-delete-token", Name: "credential"}).Error)
	require.NoError(t, db.Create(&UserOAuthBinding{UserId: userID, ProviderId: 17, ProviderUserId: "provider-user"}).Error)
	require.NoError(t, db.Create(&TwoFA{UserId: userID, Secret: "totp-secret", IsEnabled: true}).Error)
	require.NoError(t, db.Create(&TwoFABackupCode{UserId: userID, CodeHash: "backup-code-hash"}).Error)
	require.NoError(t, db.Create(&PasskeyCredential{UserID: userID, CredentialID: "credential-id", PublicKey: "public-key"}).Error)
}

func requireHardDeleteCredentialCount(t *testing.T, db *gorm.DB, userID int, expected int64) {
	t.Helper()
	targets := []struct {
		model  any
		column string
	}{
		{model: &User{}, column: "id"},
		{model: &Token{}, column: "user_id"},
		{model: &UserOAuthBinding{}, column: "user_id"},
		{model: &TwoFA{}, column: "user_id"},
		{model: &TwoFABackupCode{}, column: "user_id"},
		{model: &PasskeyCredential{}, column: "user_id"},
	}
	for _, target := range targets {
		var count int64
		require.NoError(t, db.Unscoped().Model(target.model).Where(target.column+" = ?", userID).Count(&count).Error)
		require.Equal(t, expected, count, "%T", target.model)
	}
}

func TestHardDeleteUserPurgesAuthenticationDataAndKeepsFinancialHistory(t *testing.T) {
	db := setupHardDeleteUserTestDB(t)
	const userID = 901
	seedHardDeleteCredentials(t, db, userID)
	require.NoError(t, db.Create(&WalletLedgerEntry{
		UserID: userID, OperationID: "hard-delete-ledger", Category: "topup", Delta: 100,
	}).Error)
	require.NoError(t, db.Create(&Log{UserId: userID, Type: LogTypeTopup, Content: "historical topup"}).Error)

	require.NoError(t, HardDeleteUserById(userID))
	require.NoError(t, HardDeleteUserById(userID))

	requireHardDeleteCredentialCount(t, db, userID, 0)
	var ledgerCount, logCount int64
	require.NoError(t, db.Model(&WalletLedgerEntry{}).Where("user_id = ?", userID).Count(&ledgerCount).Error)
	require.NoError(t, db.Model(&Log{}).Where("user_id = ?", userID).Count(&logCount).Error)
	require.EqualValues(t, 1, ledgerCount)
	require.EqualValues(t, 1, logCount)
}

func TestHardDeleteUserRollsBackAllCredentialDeletesOnFailure(t *testing.T) {
	db := setupHardDeleteUserTestDB(t)
	const userID = 902
	seedHardDeleteCredentials(t, db, userID)
	expected := errors.New("passkey delete failed")
	require.NoError(t, db.Callback().Delete().Before("gorm:delete").Register("test:fail_passkey_delete", func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Schema != nil && tx.Statement.Schema.Table == "passkey_credentials" {
			tx.AddError(expected)
		}
	}))
	t.Cleanup(func() { _ = db.Callback().Delete().Remove("test:fail_passkey_delete") })

	err := HardDeleteUserById(userID)

	require.ErrorIs(t, err, expected)
	requireHardDeleteCredentialCount(t, db, userID, 1)
}
