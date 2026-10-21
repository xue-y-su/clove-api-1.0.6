package model

import (
	"bytes"
	"log"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestGetTwoFAByUserIdTreatsMissingConfigurationAsExpected(t *testing.T) {
	var output bytes.Buffer
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/twofa.db"), &gorm.Config{
		Logger: logger.New(log.New(&output, "", 0), logger.Config{
			LogLevel:                  logger.Warn,
			IgnoreRecordNotFoundError: false,
			Colorful:                  false,
		}),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&TwoFA{}))

	previousDB := DB
	DB = db
	t.Cleanup(func() {
		DB = previousDB
		sqlDB, closeErr := db.DB()
		require.NoError(t, closeErr)
		require.NoError(t, sqlDB.Close())
	})

	twoFA, err := GetTwoFAByUserId(42)
	require.NoError(t, err)
	require.Nil(t, twoFA)
	require.NotContains(t, output.String(), "record not found")
}

func TestTwoFAFailedAttemptsUseCurrentLockedRecord(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/twofa-attempts.db"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&TwoFA{}))

	previousDB := DB
	previousType := common.MainDatabaseType()
	DB = db
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	t.Cleanup(func() {
		DB = previousDB
		common.SetMainDatabaseType(previousType)
		sqlDB, closeErr := db.DB()
		require.NoError(t, closeErr)
		require.NoError(t, sqlDB.Close())
	})

	record := TwoFA{UserId: 91, Secret: "test-secret", IsEnabled: true}
	require.NoError(t, db.Create(&record).Error)
	staleCopies := make([]TwoFA, common.MaxFailAttempts)
	for index := range staleCopies {
		require.NoError(t, db.First(&staleCopies[index], record.Id).Error)
	}
	for index := range staleCopies {
		require.NoError(t, staleCopies[index].IncrementFailedAttempts())
	}

	var stored TwoFA
	require.NoError(t, db.First(&stored, record.Id).Error)
	require.Equal(t, common.MaxFailAttempts, stored.FailedAttempts)
	require.NotNil(t, stored.LockedUntil)

	require.NoError(t, staleCopies[0].ResetFailedAttempts())
	stored = TwoFA{}
	require.NoError(t, db.First(&stored, record.Id).Error)
	require.Zero(t, stored.FailedAttempts)
	require.Nil(t, stored.LockedUntil)
}
