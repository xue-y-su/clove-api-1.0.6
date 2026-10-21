package model

import (
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestSQLiteLogMigrationAddsUniqueEventIDToLegacyTable(t *testing.T) {
	originalLogDB := LOG_DB
	originalMainType := common.MainDatabaseType()
	originalLogType := common.LogDatabaseType()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	LOG_DB = db
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	t.Cleanup(func() {
		LOG_DB = originalLogDB
		common.SetDatabaseTypes(originalMainType, originalLogType)
	})

	require.NoError(t, db.Exec("CREATE TABLE logs (id integer primary key, content text)").Error)

	require.NoError(t, migrateLOGDB())
	require.True(t, db.Migrator().HasColumn(&Log{}, "EventId"))

	require.NoError(t, db.Create(&Log{Content: "legacy-a"}).Error)
	require.NoError(t, db.Create(&Log{Content: "legacy-b"}).Error)
	eventID := "audit-event-1"
	require.NoError(t, db.Create(&Log{EventId: &eventID, Content: "audit-a"}).Error)
	require.Error(t, db.Create(&Log{EventId: &eventID, Content: "audit-b"}).Error)
}

func TestSQLiteLogMigrationCreatesCoveringUsageStatsIndex(t *testing.T) {
	originalLogDB := LOG_DB
	originalMainType := common.MainDatabaseType()
	originalLogType := common.LogDatabaseType()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	LOG_DB = db
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	t.Cleanup(func() {
		LOG_DB = originalLogDB
		common.SetDatabaseTypes(originalMainType, originalLogType)
	})

	require.NoError(t, migrateLOGDB())
	require.True(t, db.Migrator().HasIndex(&Log{}, "idx_logs_stats_covering"))

	var indexColumns []struct {
		Name string `gorm:"column:name"`
	}
	require.NoError(t, db.Raw("PRAGMA index_info('idx_logs_stats_covering')").Scan(&indexColumns).Error)
	columnNames := make([]string, len(indexColumns))
	for index, column := range indexColumns {
		columnNames[index] = column.Name
	}
	require.Equal(
		t,
		[]string{"type", "created_at", "quota", "prompt_tokens", "completion_tokens"},
		columnNames,
	)

	require.True(t, db.Migrator().HasIndex(&Log{}, "idx_logs_created_at_id"))
	indexColumns = nil
	require.NoError(t, db.Raw("PRAGMA index_info('idx_logs_created_at_id')").Scan(&indexColumns).Error)
	columnNames = make([]string, len(indexColumns))
	for index, column := range indexColumns {
		columnNames[index] = column.Name
	}
	require.Equal(t, []string{"created_at", "id"}, columnNames)
}
