package service

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	mysqlDriver "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/require"
	gormMySQL "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func setupAuditOutboxMySQLTest(t *testing.T) (*gorm.DB, *gorm.DB) {
	t.Helper()
	rawDSN := os.Getenv("TEST_MYSQL_DSN")
	if rawDSN == "" {
		t.Skip("set TEST_MYSQL_DSN to run MySQL clientFoundRows compatibility test")
	}
	config, err := mysqlDriver.ParseDSN(rawDSN)
	require.NoError(t, err)
	config.ClientFoundRows = true
	prefix := fmt.Sprintf("audit_cf_%d_", time.Now().UnixNano())
	gormConfig := &gorm.Config{NamingStrategy: schema.NamingStrategy{TablePrefix: prefix}}
	mainDB, err := gorm.Open(gormMySQL.Open(config.FormatDSN()), gormConfig)
	require.NoError(t, err)
	logDB, err := gorm.Open(gormMySQL.Open(config.FormatDSN()), gormConfig)
	require.NoError(t, err)
	require.NoError(t, mainDB.AutoMigrate(
		&model.User{}, &model.AuditOutboxEvent{}, &model.IPSecurityEvent{},
		&model.IPAuditRollup{}, &model.IPSecurityProjectionState{},
	))
	require.NoError(t, logDB.AutoMigrate(&model.Log{}))

	previousDB := model.DB
	previousLogDB := model.LOG_DB
	previousMainType := common.MainDatabaseType()
	previousLogType := common.LogDatabaseType()
	model.DB = mainDB
	model.LOG_DB = logDB
	common.SetDatabaseTypes(common.DatabaseTypeMySQL, common.DatabaseTypeMySQL)
	t.Cleanup(func() {
		model.DB = previousDB
		model.LOG_DB = previousLogDB
		common.SetDatabaseTypes(previousMainType, previousLogType)
		require.NoError(t, logDB.Migrator().DropTable(&model.Log{}))
		require.NoError(t, mainDB.Migrator().DropTable(
			&model.IPAuditRollup{}, &model.IPSecurityEvent{}, &model.IPSecurityProjectionState{},
			&model.AuditOutboxEvent{}, &model.User{},
		))
		if sqlDB, dbErr := logDB.DB(); dbErr == nil {
			require.NoError(t, sqlDB.Close())
		}
		if sqlDB, dbErr := mainDB.DB(); dbErr == nil {
			require.NoError(t, sqlDB.Close())
		}
	})
	return mainDB, logDB
}

func TestAuditOutboxMySQLClientFoundRowsValidatesCanonicalRows(t *testing.T) {
	mainDB, logDB := setupAuditOutboxMySQLTest(t)
	first := AuditEvent{
		EventID: "mysql-real-enqueue", Type: "user.role_change", Result: "success",
		ActorID: 1, Target: "user:9", Params: map[string]any{"target_user_id": 9},
	}
	require.NoError(t, mainDB.Transaction(func(tx *gorm.DB) error {
		return EnqueueWithTx(tx, first)
	}))
	conflictingOutbox := first
	conflictingOutbox.Target = "user:10"
	require.ErrorIs(t, mainDB.Transaction(func(tx *gorm.DB) error {
		return EnqueueWithTx(tx, conflictingOutbox)
	}), ErrAuditEventConflict)

	require.NoError(t, mainDB.Transaction(func(tx *gorm.DB) error {
		return EnqueueWithTx(tx, AuditEvent{
			EventID: "mysql-real-log", Type: "user.role_change", Result: "success",
			ActorID: 1, Target: "user:9", RequestID: "canonical-request",
		})
	}))
	conflictingLogEventID := "mysql-real-log"
	require.NoError(t, logDB.Create(&model.Log{
		EventId: &conflictingLogEventID, UserId: 99, CreatedAt: 1, Type: model.LogTypeManage,
		Content: "conflicting.event", RequestId: "conflicting-request",
	}).Error)
	var outbox model.AuditOutboxEvent
	require.NoError(t, mainDB.First(&outbox, "event_id = ?", conflictingLogEventID).Error)
	const workerID = "mysql-canonical-conflict-worker"
	claimed, err := claimAuditOutboxEvent(context.Background(), outbox.ID, workerID, common.GetTimestamp())
	require.NoError(t, err)
	require.True(t, claimed)
	require.ErrorIs(t, deliverClaimedAuditOutboxEvent(context.Background(), outbox.ID, workerID), ErrAuditEventConflict)
}
