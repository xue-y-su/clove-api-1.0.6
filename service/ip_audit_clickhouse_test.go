package service

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestIPAuditExecutesAgainstClickHouse(t *testing.T) {
	dsn := os.Getenv("CLOVEAPI_TEST_CLICKHOUSE_DSN")
	if dsn == "" {
		t.Skip("CLOVEAPI_TEST_CLICKHOUSE_DSN is not configured")
	}

	previousMainDB := model.DB
	previousLogDB := model.LOG_DB
	previousMainType := common.MainDatabaseType()
	previousLogType := common.LogDatabaseType()
	previousMaster := common.IsMasterNode
	previousDSN, hadDSN := os.LookupEnv("LOG_SQL_DSN")
	t.Cleanup(func() {
		if model.LOG_DB != nil {
			_ = model.LOG_DB.Exec("DROP TABLE IF EXISTS logs").Error
			if sqlDB, err := model.LOG_DB.DB(); err == nil {
				_ = sqlDB.Close()
			}
		}
		model.DB = previousMainDB
		model.LOG_DB = previousLogDB
		common.SetMainDatabaseType(previousMainType)
		common.SetLogDatabaseType(previousLogType)
		common.IsMasterNode = previousMaster
		if hadDSN {
			_ = os.Setenv("LOG_SQL_DSN", previousDSN)
		} else {
			_ = os.Unsetenv("LOG_SQL_DSN")
		}
	})

	mainDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	model.DB = mainDB
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	require.NoError(t, mainDB.AutoMigrate(
		&model.User{}, &model.AuditOutboxEvent{}, &model.IPSecurityEvent{}, &model.IPAuditRollup{}, &model.IPSecurityProjectionState{},
		&model.IPSecurityProjectionQuarantine{},
	))
	require.NoError(t, mainDB.Create(&[]model.User{
		{Id: 1, Username: "root", AffCode: "clickhouse-root", Role: common.RoleRootUser, Status: common.UserStatusEnabled},
		{Id: 7, Username: "target", AffCode: "clickhouse-target", Role: common.RoleCommonUser, Status: common.UserStatusEnabled},
	}).Error)

	require.NoError(t, os.Setenv("LOG_SQL_DSN", dsn))
	t.Setenv("LOG_SQL_CLICKHOUSE_TTL_DAYS", "7")
	common.IsMasterNode = true
	require.NoError(t, model.InitLogDB())
	require.True(t, common.UsingLogDatabase(common.DatabaseTypeClickHouse))
	var createTableSQL string
	require.NoError(t, model.LOG_DB.Raw("SHOW CREATE TABLE logs").Scan(&createTableSQL).Error)
	require.Contains(t, createTableSQL, "TTL toDateTime(created_at) + toIntervalDay(7)")
	require.NoError(t, model.LOG_DB.Exec("TRUNCATE TABLE logs").Error)

	base := int64(1_789_000_000)
	duplicateEventID := "clickhouse-duplicate"
	otherEventID := "clickhouse-other"
	tieEventID := "clickhouse-tie"
	require.NoError(t, model.LOG_DB.Create(&[]model.Log{
		{EventId: &duplicateEventID, UserId: 7, Username: "target", CreatedAt: base, Type: model.LogTypeConsume, Ip: "203.0.113.70", RequestId: "a"},
		{EventId: &duplicateEventID, UserId: 7, Username: "target", CreatedAt: base, Type: model.LogTypeConsume, Ip: "203.0.113.71", RequestId: "z"},
		{EventId: &otherEventID, UserId: 7, Username: "target", CreatedAt: base, Type: model.LogTypeConsume, Ip: "203.0.113.71", RequestId: "z"},
		{EventId: &tieEventID, UserId: 7, Username: "target", CreatedAt: base, Type: model.LogTypeConsume, Ip: "203.0.113.71", RequestId: "z"},
	}).Error)
	require.NoError(t, model.BackfillIPSecurityProjection(context.Background(), 1))
	var projectionState model.IPSecurityProjectionState
	require.NoError(t, mainDB.First(&projectionState, "id = ?", 1).Error)
	require.EqualValues(t, 3, projectionState.ProcessedLogs)
	require.Equal(t, base, projectionState.LastLogCreatedAt)
	require.Equal(t, "z", projectionState.LastLogRequestID)
	require.NotEmpty(t, projectionState.LastLogEventIdentity)

	lateEventID := "clickhouse-late"
	require.NoError(t, model.LOG_DB.Create(&model.Log{
		EventId: &lateEventID, UserId: 7, Username: "target", CreatedAt: base,
		Type: model.LogTypeConsume, Ip: "203.0.113.71", RequestId: "a",
	}).Error)
	oversizedEventID := strings.Repeat("oversized-clickhouse-event-id-", 10)
	healthyAfterOversizedID := "clickhouse-after-oversized"
	require.Greater(t, len(oversizedEventID), 191)
	require.NoError(t, model.LOG_DB.Create(&[]model.Log{
		{
			EventId: &oversizedEventID, UserId: 7, Username: "target", CreatedAt: base + 1,
			Type: model.LogTypeConsume, Ip: "203.0.113.71", RequestId: "oversized",
		},
		{
			EventId: &healthyAfterOversizedID, UserId: 7, Username: "target", CreatedAt: base + 2,
			Type: model.LogTypeConsume, Ip: "203.0.113.71", RequestId: "healthy-after-oversized",
		},
	}).Error)
	require.NoError(t, model.BackfillIPSecurityProjection(context.Background(), 1))
	require.NoError(t, mainDB.First(&projectionState, "id = ?", 1).Error)
	require.EqualValues(t, 5, projectionState.ProcessedLogs, "overlap replays must not inflate forward progress")
	var lateEvent model.IPSecurityEvent
	require.NoError(t, mainDB.First(&lateEvent, "event_id = ?", lateEventID).Error)
	var healthyAfterOversized model.IPSecurityEvent
	require.NoError(t, mainDB.First(&healthyAfterOversized, "event_id = ?", healthyAfterOversizedID).Error)
	var quarantine model.IPSecurityProjectionQuarantine
	require.NoError(t, mainDB.First(&quarantine, "source_type = ?", "log_clickhouse").Error)
	require.LessOrEqual(t, len(quarantine.SourceKey), 255)
	require.LessOrEqual(t, len(quarantine.SourceIdentity), 191)

	security, err := GetUserSecurityAudit(context.Background(), 1, 7, IPAuditQuery{
		Range: IPAuditRangeCustom, StartTimestamp: base, EndTimestamp: base, Page: 1, PageSize: 20,
	})
	require.NoError(t, err)
	require.Equal(t, 4, security.Summary.APIRequestCount)
	require.Equal(t, "203.0.113.71", security.Summary.RecentIP)
	require.Equal(t, 4, security.Total)
	require.Len(t, security.Timeline, 4)
	securityRequestIDs := map[string]int{}
	for _, item := range security.Timeline {
		securityRequestIDs[item.RequestID]++
	}
	require.Equal(t, map[string]int{"a": 1, "z": 3}, securityRequestIDs)

	page, err := GetGlobalIPAudit(context.Background(), 1, IPAuditQuery{
		Range: IPAuditRangeCustom, StartTimestamp: base, EndTimestamp: base, Page: 1, PageSize: 20,
	})
	require.NoError(t, err)
	require.Equal(t, 1, page.Total)
	require.Len(t, page.Items, 1)
	require.Equal(t, 4, page.Items[0].APIRequestCount)
	require.Len(t, page.Items[0].Timeline, 4)
	globalRequestIDs := map[string]int{}
	for _, item := range page.Items[0].Timeline {
		globalRequestIDs[item.RequestId]++
	}
	require.Equal(t, map[string]int{"a": 1, "z": 3}, globalRequestIDs)

	fixedHighWater := projectionState
	delayedEventID := "clickhouse-after-fixed-high-water"
	require.NoError(t, model.LOG_DB.Create(&[]model.Log{
		{
			EventId: &delayedEventID, UserId: 7, Username: "target", CreatedAt: base + 1,
			Type: model.LogTypeConsume, Ip: "203.0.113.80", RequestId: "before-fixed-high-water",
		},
		{
			EventId: &delayedEventID, UserId: 7, Username: "target", CreatedAt: base + 3,
			Type: model.LogTypeConsume, Ip: "203.0.113.81", RequestId: "after-fixed-high-water",
		},
	}).Error)
	const cursorTuple = "tuple(logs.created_at, logs.request_id, logs.audit_event_identity)"
	var fixedSnapshotCount int64
	require.NoError(t, model.LogReadDB().
		Where(cursorTuple+" <= tuple(?, ?, ?)", fixedHighWater.LastLogCreatedAt, fixedHighWater.LastLogRequestID, fixedHighWater.LastLogEventIdentity).
		Where("logs.event_id = ?", delayedEventID).
		Count(&fixedSnapshotCount).Error)
	require.Zero(t, fixedSnapshotCount, "a canonical copy beyond the fixed high-water belongs to the next run")
	require.NoError(t, model.BackfillIPSecurityProjection(context.Background(), 1))
	require.NoError(t, mainDB.First(&projectionState, "id = ?", 1).Error)
	require.EqualValues(t, 6, projectionState.ProcessedLogs)
	var delayedEvent model.IPSecurityEvent
	require.NoError(t, mainDB.First(&delayedEvent, "event_id = ?", delayedEventID).Error)
	require.Equal(t, base+3, delayedEvent.CreatedAt)
	require.Equal(t, "203.0.113.81", delayedEvent.IP)

	seedAuditOutboxWalletUser(t, mainDB, 53)
	require.NoError(t, mainDB.Transaction(func(tx *gorm.DB) error {
		return EnqueueWithTx(tx, AuditEvent{
			EventID: "clickhouse-outbox-projection-retry", Type: "user.logout", Result: "success",
			ActorID: 53, Target: "user:53", IP: "203.0.113.53", RequestID: "clickhouse-outbox-retry",
		})
	}))
	failure := errors.New("clickhouse projection unavailable")
	callbackName := "audit_outbox_clickhouse_test:fail_ip_security_projection"
	require.NoError(t, mainDB.Callback().Create().Before("gorm:create").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Schema != nil && tx.Statement.Schema.Table == "ip_security_events" {
			tx.AddError(failure)
		}
	}))
	delivered, failed, err := DeliverPending(context.Background(), 10)
	require.NoError(t, err)
	require.Zero(t, delivered)
	require.Equal(t, 1, failed)
	var failedEvent model.AuditOutboxEvent
	require.NoError(t, mainDB.First(&failedEvent, "event_id = ?", "clickhouse-outbox-projection-retry").Error)
	require.Greater(t, failedEvent.LogPersistedAt, int64(0))
	require.NoError(t, mainDB.Callback().Create().Remove(callbackName))
	require.NoError(t, mainDB.Model(&model.AuditOutboxEvent{}).
		Where("event_id = ?", "clickhouse-outbox-projection-retry").Update("next_attempt_at", 0).Error)
	delivered, failed, err = DeliverPending(context.Background(), 10)
	require.NoError(t, err)
	require.Equal(t, 1, delivered)
	require.Zero(t, failed)
	var rawLogCount int64
	require.NoError(t, model.LOG_DB.Table("logs").
		Where("event_id = ?", "clickhouse-outbox-projection-retry").Count(&rawLogCount).Error)
	require.EqualValues(t, 1, rawLogCount, "a projection retry must not physically append the audit log again")
}
