package model

import (
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func openComplianceModelTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	gin.SetMode(gin.TestMode)
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	common.RedisEnabled = false

	previousDB := DB
	previousLogDB := LOG_DB

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)

	DB = db
	LOG_DB = db
	require.NoError(t, db.AutoMigrate(&ComplianceStatementSnapshot{}, &ComplianceConfirmation{}))

	t.Cleanup(func() {
		DB = previousDB
		LOG_DB = previousLogDB
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func TestComplianceConfirmationIsIdempotentByUserVersionAndHash(t *testing.T) {
	db := openComplianceModelTestDB(t)

	require.NoError(t, RecordComplianceConfirmation(7, "alice", "2026-07", "hash-a", "203.0.113.10", "ua-one", 100))
	require.NoError(t, RecordComplianceConfirmation(7, "alice", "2026-07", "hash-a", "203.0.113.11", "ua-two", 200))

	var confirmations []ComplianceConfirmation
	require.NoError(t, db.Find(&confirmations).Error)
	require.Len(t, confirmations, 1)
	require.Equal(t, "203.0.113.11", confirmations[0].Ip)
	require.Equal(t, "ua-two", confirmations[0].UserAgent)
	require.EqualValues(t, 200, confirmations[0].ConfirmedAt)

	confirmed, err := HasComplianceConfirmation(7, "2026-07", "hash-a")
	require.NoError(t, err)
	require.True(t, confirmed)
}

func TestComplianceConfirmationListSupportsAdminFilters(t *testing.T) {
	openComplianceModelTestDB(t)

	require.NoError(t, RecordComplianceConfirmation(7, "alice", "2026-07", "hash-a", "203.0.113.10", "ua-one", 100))
	require.NoError(t, RecordComplianceConfirmation(8, "bob", "2026-08", "hash-b", "203.0.113.11", "ua-two", 200))

	items, total, err := GetComplianceConfirmations(ComplianceConfirmationFilter{
		Username: "bob",
		Ip:       "203.0.113.11",
	}, 0, 10)

	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, items, 1)
	require.Equal(t, "bob", items[0].Username)
	require.Equal(t, "hash-b", items[0].ContentHash)
}

func TestComplianceSnapshotConfirmationHasDatabaseUniqueConstraint(t *testing.T) {
	db := openComplianceModelTestDB(t)

	confirmation := ComplianceConfirmation{
		UserId:      7,
		Username:    "alice",
		SnapshotId:  11,
		Version:     "2026-07",
		ContentHash: "hash-a",
		Ip:          "203.0.113.10",
		ConfirmedAt: 100,
	}
	require.NoError(t, db.Create(&confirmation).Error)

	duplicate := ComplianceConfirmation{
		UserId:      7,
		Username:    "alice",
		SnapshotId:  11,
		Version:     "2026-07",
		ContentHash: "hash-a",
		Ip:          "203.0.113.11",
		ConfirmedAt: 200,
	}
	require.Error(t, db.Create(&duplicate).Error)
}

func TestHasAllComplianceSnapshotConfirmationsCountsDistinctSnapshots(t *testing.T) {
	db := openComplianceModelTestDB(t)

	snapshots := []ComplianceStatementSnapshot{
		{Id: 11, ItemKey: "terms", Version: "2026-07", ContentHash: "hash-a"},
		{Id: 12, ItemKey: "privacy", Version: "2026-07", ContentHash: "hash-b"},
	}
	for _, snapshot := range snapshots {
		require.NoError(t, db.Create(&snapshot).Error)
	}
	require.NoError(t, db.Exec("DROP INDEX IF EXISTS idx_compliance_confirmation_snapshot").Error)
	require.NoError(t, db.Create(&ComplianceConfirmation{
		UserId:      7,
		SnapshotId:  11,
		Version:     "2026-07",
		ContentHash: "hash-a",
		ConfirmedAt: 100,
	}).Error)
	require.NoError(t, db.Create(&ComplianceConfirmation{
		UserId:      7,
		SnapshotId:  11,
		Version:     "2026-07",
		ContentHash: "hash-a",
		ConfirmedAt: 101,
	}).Error)

	confirmed, err := HasAllComplianceSnapshotConfirmations(7, snapshots)

	require.NoError(t, err)
	require.False(t, confirmed)
}

func TestComplianceConfirmationPreservesIPv6LoopbackIP(t *testing.T) {
	openComplianceModelTestDB(t)

	require.NoError(t, RecordComplianceConfirmation(7, "alice", "2026-07", "hash-a", "::1", "ua", 100))

	var confirmation ComplianceConfirmation
	require.NoError(t, DB.First(&confirmation).Error)
	require.Equal(t, "::1", confirmation.Ip)
}

func TestComplianceConfirmationListNormalizesAndFiltersLegacyLoopbackIP(t *testing.T) {
	openComplianceModelTestDB(t)
	require.NoError(t, DB.Create(&ComplianceConfirmation{
		UserId:      7,
		Username:    "alice",
		Version:     "2026-07",
		ContentHash: "hash-a",
		Ip:          "::1",
		ConfirmedAt: 100,
	}).Error)

	items, total, err := GetComplianceConfirmations(ComplianceConfirmationFilter{
		Ip: "127.0.0.1",
	}, 0, 10)

	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, items, 1)
	require.Equal(t, "::1", items[0].Ip)
}
