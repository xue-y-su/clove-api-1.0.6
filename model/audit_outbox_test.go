package model

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type auditOutboxBeforeLogSnapshot struct {
	ID             int64  `gorm:"primaryKey;autoIncrement"`
	EventID        string `gorm:"type:varchar(191);not null;uniqueIndex"`
	EventType      string `gorm:"type:varchar(128);not null"`
	Result         string `gorm:"type:varchar(32);not null;default:''"`
	ActorID        int    `gorm:"not null;default:0"`
	Target         string `gorm:"type:varchar(255);not null;default:''"`
	IP             string `gorm:"type:varchar(64);not null;default:''"`
	RequestID      string `gorm:"type:varchar(64);not null;default:''"`
	Payload        string `gorm:"type:text;not null"`
	Status         string `gorm:"type:varchar(32);not null"`
	AttemptCount   int    `gorm:"not null;default:0"`
	NextAttemptAt  int64  `gorm:"bigint;not null;default:0"`
	LastError      string `gorm:"type:text;not null"`
	LockedBy       string `gorm:"type:varchar(128);not null;default:''"`
	LockedUntil    int64  `gorm:"bigint;not null;default:0"`
	CreatedAt      int64  `gorm:"bigint;autoCreateTime"`
	UpdatedAt      int64  `gorm:"bigint;autoUpdateTime"`
	DeliveredAt    int64  `gorm:"bigint;not null;default:0"`
}

func (auditOutboxBeforeLogSnapshot) TableName() string {
	return "audit_outbox_events"
}

func TestAuditOutboxMigrationEnforcesUniqueEventID(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/audit-outbox.db"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.AutoMigrate(&AuditOutboxEvent{}))
	require.NoError(t, db.AutoMigrate(&AuditOutboxEvent{}))

	first := &AuditOutboxEvent{EventID: "event-1", EventType: "wallet.adjust", Status: AuditOutboxStatusPending}
	require.NoError(t, db.Create(first).Error)
	duplicate := &AuditOutboxEvent{EventID: "event-1", EventType: "wallet.adjust", Status: AuditOutboxStatusPending}
	require.Error(t, db.Create(duplicate).Error)
}

func TestAuditOutboxMigrationPreservesPendingRowsBeforeLogSnapshots(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/audit-outbox-upgrade.db"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })

	require.NoError(t, db.AutoMigrate(&auditOutboxBeforeLogSnapshot{}))
	require.NoError(t, db.Create(&auditOutboxBeforeLogSnapshot{
		EventID: "legacy-pending-snapshot", EventType: "user.role_change",
		Payload: "{}", Status: AuditOutboxStatusPending,
	}).Error)
	require.NoError(t, db.AutoMigrate(&AuditOutboxEvent{}))
	require.NoError(t, db.AutoMigrate(&AuditOutboxEvent{}))

	var event AuditOutboxEvent
	require.NoError(t, db.First(&event, "event_id = ?", "legacy-pending-snapshot").Error)
	require.Equal(t, AuditOutboxStatusPending, event.Status)
	require.Zero(t, event.LogSnapshotVersion)
	require.Empty(t, event.LogSnapshot)
}

func TestFastMigrationCreatesAuditOutboxTable(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/audit-outbox-fast.db"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	previousDB := DB
	previousMainType := common.MainDatabaseType()
	DB = db
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	t.Cleanup(func() {
		DB = previousDB
		common.SetMainDatabaseType(previousMainType)
	})

	require.NoError(t, migrateDBFast())
	require.True(t, db.Migrator().HasTable(&AuditOutboxEvent{}))
	require.True(t, db.Migrator().HasTable(&InviteCodeApplication{}))
	require.True(t, db.Migrator().HasTable(&InviteCode{}))
	require.True(t, db.Migrator().HasTable(&InviteCodeUse{}))
}
