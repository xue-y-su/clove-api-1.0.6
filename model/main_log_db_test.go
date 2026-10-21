package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestInitLogDBReusesPrimaryForExplicitLocalSQLite(t *testing.T) {
	previousDB := DB
	previousLogDB := LOG_DB
	previousSQLitePath := common.SQLitePath
	previousMainType := common.MainDatabaseType()
	previousLogType := common.LogDatabaseType()
	previousMaster := common.IsMasterNode
	t.Setenv("SQL_DSN", "local")
	t.Setenv("LOG_SQL_DSN", "local")
	common.SQLitePath = t.TempDir() + "/shared.db"
	primary, err := gorm.Open(sqlite.Open(common.SQLitePath), &gorm.Config{})
	require.NoError(t, err)
	DB = primary
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	common.IsMasterNode = false
	t.Cleanup(func() {
		if LOG_DB != nil && LOG_DB != primary {
			if sqlDB, dbErr := LOG_DB.DB(); dbErr == nil {
				_ = sqlDB.Close()
			}
		}
		if sqlDB, dbErr := primary.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
		DB = previousDB
		LOG_DB = previousLogDB
		common.SQLitePath = previousSQLitePath
		common.SetDatabaseTypes(previousMainType, previousLogType)
		common.IsMasterNode = previousMaster
	})

	require.NoError(t, InitLogDB())
	require.Same(t, DB, LOG_DB)
}
