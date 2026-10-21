package model

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupIPSecurityBackfillTest(t *testing.T) (*gorm.DB, *gorm.DB) {
	t.Helper()
	open := func(name string) *gorm.DB {
		dsn := fmt.Sprintf("file:%s_%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"), name)
		db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
		require.NoError(t, err)
		sqlDB, err := db.DB()
		require.NoError(t, err)
		t.Cleanup(func() { _ = sqlDB.Close() })
		return db
	}
	mainDB := open("main")
	logDB := open("logs")
	require.NoError(t, mainDB.AutoMigrate(
		&User{}, &IPSecurityEvent{}, &IPAuditRollup{}, &IPSecurityProjectionState{}, &IPSecurityProjectionQuarantine{},
	))
	require.NoError(t, logDB.AutoMigrate(&Log{}))
	previousDB := DB
	previousLogDB := LOG_DB
	previousMainType := common.MainDatabaseType()
	previousLogType := common.LogDatabaseType()
	DB = mainDB
	LOG_DB = logDB
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	t.Cleanup(func() {
		DB = previousDB
		LOG_DB = previousLogDB
		common.SetDatabaseTypes(previousMainType, previousLogType)
	})
	return mainDB, logDB
}

func TestIPSecurityProjectionTransactionRetryRecognizesSupportedDatabaseDeadlocks(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		retryable bool
	}{
		{name: "mysql", err: errors.New("Error 1213 (40001): Deadlock found when trying to get lock"), retryable: true},
		{name: "postgres", err: errors.New("ERROR: deadlock detected (SQLSTATE 40P01)"), retryable: true},
		{name: "serialization", err: errors.New("SQLSTATE[40001]: serialization failure"), retryable: true},
		{name: "validation", err: errors.New("invalid projection event"), retryable: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.retryable, isRetryableIPSecurityProjectionTransactionError(test.err))
		})
	}
}

func TestBackfillIPSecurityProjectionIsBoundedIdempotentAndMarksReady(t *testing.T) {
	mainDB, logDB := setupIPSecurityBackfillTest(t)
	base := int64(1_752_500_500)
	require.NoError(t, mainDB.Create(&User{
		Id: 7, Username: "backfill-user", AffCode: "backfill-aff", Role: common.RoleCommonUser,
		Status: common.UserStatusEnabled, CreatedAt: base, RegistrationIP: "203.0.113.7",
	}).Error)
	require.NoError(t, logDB.Create(&[]Log{
		{UserId: 7, Username: "backfill-user", CreatedAt: base + 1, Type: LogTypeConsume, Ip: "203.0.113.7", RequestId: "request-1", Quota: 11},
		{UserId: 7, Username: "backfill-user", CreatedAt: base + 2, Type: LogTypeError, Ip: "203.0.113.7", RequestId: "request-2", Other: `{"status_code":401}`},
		{UserId: 7, Username: "backfill-user", CreatedAt: base, Type: LogTypeLogin, Content: "user.register", Ip: "203.0.113.7", RequestId: "registration-log"},
		{UserId: 7, Username: "backfill-user", CreatedAt: base + 3, Type: LogTypeManage, RequestId: "missing-ip"},
	}).Error)

	require.NoError(t, BackfillIPSecurityProjection(context.Background(), 2))

	ready, err := IPSecurityProjectionReady(context.Background())
	require.NoError(t, err)
	require.True(t, ready)
	var state IPSecurityProjectionState
	require.NoError(t, mainDB.First(&state, "id = ?", 1).Error)
	require.Equal(t, IPSecurityProjectionStatusReady, state.Status)
	require.EqualValues(t, 4, state.ProcessedLogs)
	require.EqualValues(t, 1, state.ProcessedUsers)
	var eventCount int64
	require.NoError(t, mainDB.Model(&IPSecurityEvent{}).Count(&eventCount).Error)
	require.EqualValues(t, 3, eventCount)
	var lifetime IPAuditRollup
	require.NoError(t, mainDB.First(&lifetime, "bucket = ? AND user_id = ? AND ip = ?", 0, 7, "203.0.113.7").Error)
	require.Equal(t, 1, lifetime.RegistrationCount)
	require.Equal(t, 1, lifetime.APIRequestCount)
	require.Equal(t, 1, lifetime.ErrorCount)
	require.Equal(t, 2, lifetime.RequestCount)

	require.NoError(t, BackfillIPSecurityProjection(context.Background(), 1))
	require.NoError(t, mainDB.Model(&IPSecurityEvent{}).Count(&eventCount).Error)
	require.EqualValues(t, 3, eventCount)
	require.NoError(t, mainDB.First(&lifetime, "bucket = ? AND user_id = ? AND ip = ?", 0, 7, "203.0.113.7").Error)
	require.Equal(t, 2, lifetime.RequestCount)
}

func TestBackfillIPSecurityProjectionBatchAdvancesOneBoundedPage(t *testing.T) {
	mainDB, logDB := setupIPSecurityBackfillTest(t)
	base := int64(1_752_500_800)
	require.NoError(t, mainDB.Create(&User{
		Id: 8, Username: "batch-user", AffCode: "batch-aff", Role: common.RoleCommonUser,
		Status: common.UserStatusEnabled, CreatedAt: base, RegistrationIP: "8.8.8.8",
	}).Error)
	require.NoError(t, logDB.Create(&[]Log{
		{UserId: 8, Username: "batch-user", CreatedAt: base + 1, Type: LogTypeConsume, Ip: "8.8.8.8", RequestId: "batch-1"},
		{UserId: 8, Username: "batch-user", CreatedAt: base + 2, Type: LogTypeConsume, Ip: "8.8.4.4", RequestId: "batch-2"},
		{UserId: 8, Username: "batch-user", CreatedAt: base + 3, Type: LogTypeConsume, Ip: "1.1.1.1", RequestId: "batch-3"},
	}).Error)

	result, err := BackfillIPSecurityProjectionBatch(context.Background(), 1)
	require.NoError(t, err)
	require.True(t, result.Acquired)
	require.False(t, result.CaughtUp)

	var state IPSecurityProjectionState
	require.NoError(t, mainDB.First(&state, "id = ?", 1).Error)
	require.Equal(t, IPSecurityProjectionStatusRunning, state.Status)
	require.EqualValues(t, 1, state.ProcessedLogs)
	require.EqualValues(t, 1, state.ProcessedUsers)

	for attempt := 0; attempt < 3 && !result.CaughtUp; attempt++ {
		result, err = BackfillIPSecurityProjectionBatch(context.Background(), 1)
		require.NoError(t, err)
	}
	require.True(t, result.CaughtUp)
	require.NoError(t, mainDB.First(&state, "id = ?", 1).Error)
	require.Equal(t, IPSecurityProjectionStatusReady, state.Status)
	require.EqualValues(t, 3, state.ProcessedLogs)
}

func TestProjectIPSecurityLogBatchRetriesMySQLDeadlock(t *testing.T) {
	mainDB, _ := setupIPSecurityBackfillTest(t)
	const runID = "retry-deadlocked-ip-security-batch"
	now := common.GetTimestamp()
	require.NoError(t, mainDB.Create(&IPSecurityProjectionState{
		ID: 1, Status: IPSecurityProjectionStatusRunning, LockedBy: runID, LockedUntil: now + 60,
	}).Error)

	failures := 0
	callbackName := "ip_security_backfill_test:fail_state_update_with_mysql_deadlock"
	require.NoError(t, mainDB.Callback().Update().Before("gorm:update").Register(callbackName, func(tx *gorm.DB) {
		if failures != 0 || tx.Statement == nil || tx.Statement.Schema == nil || tx.Statement.Schema.Table != "ip_security_projection_states" {
			return
		}
		failures++
		tx.AddError(errors.New("Error 1213 (40001): Deadlock found when trying to get lock; try restarting transaction"))
	}))
	t.Cleanup(func() { _ = mainDB.Callback().Update().Remove(callbackName) })

	logs := []Log{{
		UserId: 51, Username: "deadlock-retry-user", CreatedAt: now,
		Type: LogTypeConsume, Ip: "203.0.113.51", RequestId: "deadlock-retry-request", Quota: 7,
	}}
	require.NoError(t, projectIPSecurityLogBatchAndUpdate(context.Background(), logs, runID, map[string]interface{}{
		"processed_logs": int64(1), "last_log_id": 1, "last_error": "",
	}))
	require.Equal(t, 1, failures)

	var eventCount int64
	require.NoError(t, mainDB.Model(&IPSecurityEvent{}).Count(&eventCount).Error)
	require.EqualValues(t, 1, eventCount)
	var rollupCount int64
	require.NoError(t, mainDB.Model(&IPAuditRollup{}).Count(&rollupCount).Error)
	require.EqualValues(t, 2, rollupCount)
	var state IPSecurityProjectionState
	require.NoError(t, mainDB.First(&state, 1).Error)
	require.EqualValues(t, 1, state.ProcessedLogs)
}

func TestBackfillIPSecurityRegistrationsRetriesDeadlockWithoutDoubleCounting(t *testing.T) {
	mainDB, _ := setupIPSecurityBackfillTest(t)
	const runID = "retry-deadlocked-registration-batch"
	now := common.GetTimestamp()
	state := IPSecurityProjectionState{
		ID: 1, Status: IPSecurityProjectionStatusRunning, LockedBy: runID, LockedUntil: now + 60,
	}
	require.NoError(t, mainDB.Create(&state).Error)
	require.NoError(t, mainDB.Create(&User{
		Id: 52, Username: "registration-deadlock-user", AffCode: "registration-deadlock-aff",
		CreatedAt: now, RegistrationIP: "203.0.113.52", Status: common.UserStatusEnabled,
	}).Error)

	failures := 0
	callbackName := "ip_security_backfill_test:fail_registration_state_update_with_mysql_deadlock"
	require.NoError(t, mainDB.Callback().Update().Before("gorm:update").Register(callbackName, func(tx *gorm.DB) {
		if failures != 0 || tx.Statement == nil || tx.Statement.Schema == nil || tx.Statement.Schema.Table != "ip_security_projection_states" {
			return
		}
		failures++
		tx.AddError(errors.New("Error 1213 (40001): Deadlock found when trying to get lock; try restarting transaction"))
	}))
	t.Cleanup(func() { _ = mainDB.Callback().Update().Remove(callbackName) })

	processed, err := backfillIPSecurityRegistrations(context.Background(), 10, &state, runID)
	require.NoError(t, err)
	require.EqualValues(t, 1, processed)
	require.Equal(t, 1, failures)

	require.NoError(t, mainDB.First(&state, 1).Error)
	require.EqualValues(t, 1, state.ProcessedUsers)
	var eventCount int64
	require.NoError(t, mainDB.Model(&IPSecurityEvent{}).Count(&eventCount).Error)
	require.EqualValues(t, 1, eventCount)
}

func TestIPSecurityProjectionReadyRequiresNoPendingEvents(t *testing.T) {
	mainDB, _ := setupIPSecurityBackfillTest(t)
	require.NoError(t, mainDB.Create(&IPSecurityProjectionState{
		ID: 1, Status: IPSecurityProjectionStatusReady,
	}).Error)
	require.NoError(t, mainDB.Create(&IPSecurityEvent{
		EventID: "readiness-pending", UserID: 7, Username: "pending-user",
		CreatedAt: 1_752_500_550, IP: "203.0.113.7", MaskedIP: "203.0.113.*",
		Kind: IPSecurityKindAPIError, ProjectionStatus: IPSecurityEventProjectionPending,
	}).Error)

	ready, err := IPSecurityProjectionReady(context.Background())
	require.NoError(t, err)
	require.False(t, ready)

	require.NoError(t, mainDB.Model(&IPSecurityEvent{}).
		Where("event_id = ?", "readiness-pending").
		Update("projection_status", IPSecurityEventProjectionProjected).Error)
	ready, err = IPSecurityProjectionReady(context.Background())
	require.NoError(t, err)
	require.True(t, ready)
}

func TestBackfillIPSecurityProjectionDowngradesReadyAfterReconcileFailureAndRecovers(t *testing.T) {
	mainDB, logDB := setupIPSecurityBackfillTest(t)
	require.NoError(t, mainDB.Create(&IPSecurityProjectionState{
		ID: 1, Status: IPSecurityProjectionStatusReady,
	}).Error)
	require.NoError(t, logDB.Create(&Log{
		UserId: 8, Username: "reconcile-user", CreatedAt: 1_752_500_560,
		Type: LogTypeConsume, Ip: "203.0.113.8", RequestId: "reconcile-failure",
	}).Error)

	failure := errors.New("reconcile event insert unavailable")
	callbackName := "ip_security_backfill_test:fail_ready_reconcile_event_insert"
	require.NoError(t, mainDB.Callback().Create().Before("gorm:create").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Schema != nil && tx.Statement.Schema.Table == "ip_security_events" {
			tx.AddError(failure)
		}
	}))
	callbackRegistered := true
	t.Cleanup(func() {
		if callbackRegistered {
			_ = mainDB.Callback().Create().Remove(callbackName)
		}
	})

	require.ErrorIs(t, BackfillIPSecurityProjection(context.Background(), 10), failure)
	var state IPSecurityProjectionState
	require.NoError(t, mainDB.First(&state, "id = ?", 1).Error)
	require.Equal(t, IPSecurityProjectionStatusFailed, state.Status)
	require.Contains(t, state.LastError, failure.Error())
	ready, err := IPSecurityProjectionReady(context.Background())
	require.NoError(t, err)
	require.False(t, ready)

	require.NoError(t, mainDB.Callback().Create().Remove(callbackName))
	callbackRegistered = false
	require.NoError(t, BackfillIPSecurityProjection(context.Background(), 10))
	require.NoError(t, mainDB.First(&state, "id = ?", 1).Error)
	require.Equal(t, IPSecurityProjectionStatusReady, state.Status)
	ready, err = IPSecurityProjectionReady(context.Background())
	require.NoError(t, err)
	require.True(t, ready)
}

func TestBackfillIPSecurityProjectionPersistsUserCursorAndReconcilesReadyRegistrations(t *testing.T) {
	mainDB, _ := setupIPSecurityBackfillTest(t)
	require.NoError(t, mainDB.Create(&IPSecurityProjectionState{
		ID: 1, Status: IPSecurityProjectionStatusReady,
	}).Error)
	first := User{
		Id: 14, Username: "registration-cursor-1", AffCode: "registration-cursor-aff-1",
		Role: common.RoleCommonUser, Status: common.UserStatusEnabled,
		CreatedAt: 1_752_500_600, RegistrationIP: "203.0.113.14",
	}
	require.NoError(t, mainDB.Create(&first).Error)
	require.NoError(t, BackfillIPSecurityProjection(context.Background(), 1))

	var state IPSecurityProjectionState
	require.NoError(t, mainDB.First(&state, "id = ?", 1).Error)
	require.Equal(t, first.Id, state.LastUserID)
	require.EqualValues(t, 1, state.ProcessedUsers)

	second := User{
		Id: 15, Username: "registration-cursor-2", AffCode: "registration-cursor-aff-2",
		Role: common.RoleCommonUser, Status: common.UserStatusEnabled,
		CreatedAt: 1_752_500_601, RegistrationIP: "203.0.113.15",
	}
	require.NoError(t, mainDB.Create(&second).Error)
	require.NoError(t, BackfillIPSecurityProjection(context.Background(), 1))
	require.NoError(t, mainDB.First(&state, "id = ?", 1).Error)
	require.Equal(t, second.Id, state.LastUserID)
	require.EqualValues(t, 2, state.ProcessedUsers)
	var registrationCount int64
	require.NoError(t, mainDB.Model(&IPSecurityEvent{}).Where("kind = ?", IPSecurityKindRegistration).Count(&registrationCount).Error)
	require.EqualValues(t, 2, registrationCount)
}

func TestBackfillIPSecurityProjectionUsesFixedRegistrationHighWater(t *testing.T) {
	mainDB, _ := setupIPSecurityBackfillTest(t)
	first := User{
		Id: 16, Username: "registration-high-water-1", AffCode: "registration-high-water-aff-1",
		Role: common.RoleCommonUser, Status: common.UserStatusEnabled,
		CreatedAt: 1_752_500_700, RegistrationIP: "203.0.113.16",
	}
	second := User{
		Id: 17, Username: "registration-high-water-2", AffCode: "registration-high-water-aff-2",
		Role: common.RoleCommonUser, Status: common.UserStatusEnabled,
		CreatedAt: 1_752_500_701, RegistrationIP: "203.0.113.17",
	}
	require.NoError(t, mainDB.Create(&first).Error)

	insertedSecond := false
	var insertErr error
	callbackName := "ip_security_backfill_test:insert_registration_after_first_page"
	require.NoError(t, mainDB.Callback().Query().After("gorm:query").Register(callbackName, func(tx *gorm.DB) {
		if insertedSecond || tx.Statement == nil || tx.Statement.Table != "users" {
			return
		}
		if _, pageQuery := tx.Statement.Dest.(*[]User); !pageQuery {
			return
		}
		insertedSecond = true
		insertErr = mainDB.Create(&second).Error
		if insertErr != nil {
			tx.AddError(insertErr)
		}
	}))
	t.Cleanup(func() { _ = mainDB.Callback().Query().Remove(callbackName) })

	require.NoError(t, BackfillIPSecurityProjection(context.Background(), 1))
	require.True(t, insertedSecond)
	require.NoError(t, insertErr)
	var state IPSecurityProjectionState
	require.NoError(t, mainDB.First(&state, "id = ?", 1).Error)
	require.Equal(t, first.Id, state.LastUserID)
	require.EqualValues(t, 1, state.ProcessedUsers)
	ready, err := IPSecurityProjectionReady(context.Background())
	require.NoError(t, err)
	require.True(t, ready)

	require.NoError(t, BackfillIPSecurityProjection(context.Background(), 1))
	require.NoError(t, mainDB.First(&state, "id = ?", 1).Error)
	require.Equal(t, second.Id, state.LastUserID)
	require.EqualValues(t, 2, state.ProcessedUsers)
}

func TestBackfillIPSecurityProjectionAdvancesUserCursorAcrossMissingRegistrationIPs(t *testing.T) {
	mainDB, _ := setupIPSecurityBackfillTest(t)
	require.NoError(t, mainDB.Create(&IPSecurityProjectionState{
		ID: 1, Status: IPSecurityProjectionStatusReady,
	}).Error)
	user := User{
		Id: 18, Username: "registration-without-ip", AffCode: "registration-without-ip-aff",
		Role: common.RoleCommonUser, Status: common.UserStatusEnabled, CreatedAt: 1_752_500_800,
	}
	require.NoError(t, mainDB.Create(&user).Error)

	require.NoError(t, BackfillIPSecurityProjection(context.Background(), 1))
	var state IPSecurityProjectionState
	require.NoError(t, mainDB.First(&state, "id = ?", 1).Error)
	require.Equal(t, user.Id, state.LastUserID)
	require.Zero(t, state.ProcessedUsers)
	var eventCount int64
	require.NoError(t, mainDB.Model(&IPSecurityEvent{}).Count(&eventCount).Error)
	require.Zero(t, eventCount)
}

func TestBackfillIPSecurityProjectionUsesRelationalIDKeyset(t *testing.T) {
	mainDB, logDB := setupIPSecurityBackfillTest(t)
	base := int64(1_752_500_800)
	logs := []Log{
		{UserId: 8, Username: "keyset-user", CreatedAt: base, Type: LogTypeConsume, Ip: "203.0.113.8", RequestId: "keyset-1"},
		{UserId: 8, Username: "keyset-user", CreatedAt: base + 1, Type: LogTypeError, Ip: "203.0.113.8", RequestId: "keyset-2", Other: `{"status_code":401}`},
	}
	require.NoError(t, logDB.Create(&logs).Error)

	deletedFirstPage := false
	var deleteErr error
	require.NoError(t, logDB.Callback().Query().After("gorm:query").Register("ip_security_backfill_test:delete_first_page", func(tx *gorm.DB) {
		if deletedFirstPage || tx.Statement == nil || tx.Statement.Table != "logs" {
			return
		}
		if _, pageQuery := tx.Statement.Dest.(*[]Log); !pageQuery {
			return
		}
		deletedFirstPage = true
		deleteErr = logDB.Delete(&Log{}, logs[0].Id).Error
		if deleteErr != nil {
			tx.AddError(deleteErr)
		}
	}))
	t.Cleanup(func() {
		_ = logDB.Callback().Query().Remove("ip_security_backfill_test:delete_first_page")
	})

	require.NoError(t, BackfillIPSecurityProjection(context.Background(), 1))
	require.True(t, deletedFirstPage)
	require.NoError(t, deleteErr)

	var eventCount int64
	require.NoError(t, mainDB.Model(&IPSecurityEvent{}).Count(&eventCount).Error)
	require.EqualValues(t, 2, eventCount)
	var state IPSecurityProjectionState
	require.NoError(t, mainDB.First(&state, "id = ?", 1).Error)
	require.EqualValues(t, 2, state.ProcessedLogs)
}

func TestBackfillIPSecurityProjectionPersistsRelationalCursorAcrossReadyRuns(t *testing.T) {
	mainDB, logDB := setupIPSecurityBackfillTest(t)
	base := int64(1_752_500_900)
	logs := []Log{
		{UserId: 9, Username: "cursor-user", CreatedAt: base, Type: LogTypeConsume, Ip: "203.0.113.9", RequestId: "cursor-1"},
		{UserId: 9, Username: "cursor-user", CreatedAt: base + 1, Type: LogTypeError, Ip: "203.0.113.9", RequestId: "cursor-2", Other: `{"status_code":401}`},
	}
	require.NoError(t, logDB.Create(&logs).Error)
	require.NoError(t, BackfillIPSecurityProjection(context.Background(), 1))

	var state IPSecurityProjectionState
	require.NoError(t, mainDB.First(&state, "id = ?", 1).Error)
	require.Equal(t, logs[1].Id, state.LastLogID)
	require.EqualValues(t, 2, state.ProcessedLogs)

	lateLog := Log{
		UserId: 9, Username: "cursor-user", CreatedAt: base + 2,
		Type: LogTypeConsume, Ip: "203.0.113.9", RequestId: "cursor-3",
	}
	require.NoError(t, logDB.Create(&lateLog).Error)
	require.NoError(t, BackfillIPSecurityProjection(context.Background(), 1))

	require.NoError(t, mainDB.First(&state, "id = ?", 1).Error)
	require.Equal(t, lateLog.Id, state.LastLogID)
	require.EqualValues(t, 3, state.ProcessedLogs)
	var eventCount int64
	require.NoError(t, mainDB.Model(&IPSecurityEvent{}).Count(&eventCount).Error)
	require.EqualValues(t, 3, eventCount)
}

func TestBackfillIPSecurityProjectionPreservesCursorWhenTailLogsAreDeleted(t *testing.T) {
	mainDB, logDB := setupIPSecurityBackfillTest(t)
	base := int64(1_752_501_200)
	logs := []Log{
		{Id: 99, UserId: 13, Username: "tail-delete-user", CreatedAt: base, Type: LogTypeConsume, Ip: "203.0.113.13", RequestId: "tail-delete-99"},
		{Id: 100, UserId: 13, Username: "tail-delete-user", CreatedAt: base + 1, Type: LogTypeConsume, Ip: "203.0.113.13", RequestId: "tail-delete-100"},
	}
	require.NoError(t, logDB.Create(&logs).Error)
	require.NoError(t, BackfillIPSecurityProjection(context.Background(), 10))
	require.NoError(t, logDB.Delete(&logs[1]).Error)
	require.NoError(t, BackfillIPSecurityProjection(context.Background(), 10))

	var state IPSecurityProjectionState
	require.NoError(t, mainDB.First(&state, "id = ?", 1).Error)
	require.Equal(t, 100, state.LastLogID)
	require.EqualValues(t, 2, state.ProcessedLogs)

	lateLog := Log{
		Id: 101, UserId: 13, Username: "tail-delete-user", CreatedAt: base + 2,
		Type: LogTypeConsume, Ip: "203.0.113.13", RequestId: "tail-delete-101",
	}
	require.NoError(t, logDB.Create(&lateLog).Error)
	require.NoError(t, BackfillIPSecurityProjection(context.Background(), 10))
	require.NoError(t, mainDB.First(&state, "id = ?", 1).Error)
	require.Equal(t, 101, state.LastLogID)
	require.EqualValues(t, 3, state.ProcessedLogs)
}

func TestBackfillIPSecurityProjectionResetsCursorWhenLogSourceGenerationChanges(t *testing.T) {
	mainDB, logDB := setupIPSecurityBackfillTest(t)
	t.Setenv("IP_SECURITY_LOG_SOURCE_GENERATION", "new-generation")
	require.NoError(t, mainDB.Create(&IPSecurityProjectionState{
		ID: 1, Status: IPSecurityProjectionStatusReady,
		ProcessedLogs: 100, LastLogID: 100,
		LogSourceType: "sqlite", LogSourceGeneration: "old-generation",
	}).Error)
	logs := []Log{
		{Id: 1, UserId: 23, Username: "generation-user", CreatedAt: 1_752_501_400, Type: LogTypeConsume, Ip: "203.0.113.23", RequestId: "generation-low"},
		{Id: 101, UserId: 23, Username: "generation-user", CreatedAt: 1_752_501_401, Type: LogTypeConsume, Ip: "203.0.113.23", RequestId: "generation-high"},
	}
	require.NoError(t, logDB.Create(&logs).Error)

	require.NoError(t, BackfillIPSecurityProjection(context.Background(), 1))
	var eventCount int64
	require.NoError(t, mainDB.Model(&IPSecurityEvent{}).Count(&eventCount).Error)
	require.EqualValues(t, 2, eventCount)
	var state IPSecurityProjectionState
	require.NoError(t, mainDB.First(&state, "id = ?", 1).Error)
	require.Equal(t, "sqlite", state.LogSourceType)
	require.Equal(t, "new-generation", state.LogSourceGeneration)
	require.Equal(t, 101, state.LastLogID)
	require.EqualValues(t, 2, state.ProcessedLogs)
}

func TestBackfillIPSecurityProjectionDoesNotAdvanceCursorWhenEventInsertFails(t *testing.T) {
	mainDB, logDB := setupIPSecurityBackfillTest(t)
	require.NoError(t, mainDB.Create(&IPSecurityProjectionState{
		ID: 1, Status: IPSecurityProjectionStatusReady,
	}).Error)
	log := Log{
		UserId: 10, Username: "retry-cursor-user", CreatedAt: 1_752_501_000,
		Type: LogTypeConsume, Ip: "203.0.113.10", RequestId: "retry-cursor",
	}
	require.NoError(t, logDB.Create(&log).Error)

	failure := errors.New("security event insert unavailable")
	callbackName := "ip_security_backfill_test:fail_security_event_insert"
	require.NoError(t, mainDB.Callback().Create().Before("gorm:create").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Schema != nil && tx.Statement.Schema.Table == "ip_security_events" {
			tx.AddError(failure)
		}
	}))
	callbackRemoved := false
	t.Cleanup(func() {
		if !callbackRemoved {
			_ = mainDB.Callback().Create().Remove(callbackName)
		}
	})

	require.ErrorIs(t, BackfillIPSecurityProjection(context.Background(), 1), failure)
	var state IPSecurityProjectionState
	require.NoError(t, mainDB.First(&state, "id = ?", 1).Error)
	require.Zero(t, state.LastLogID)
	require.Zero(t, state.ProcessedLogs)

	require.NoError(t, mainDB.Callback().Create().Remove(callbackName))
	callbackRemoved = true
	require.NoError(t, BackfillIPSecurityProjection(context.Background(), 1))
	require.NoError(t, mainDB.First(&state, "id = ?", 1).Error)
	require.Equal(t, log.Id, state.LastLogID)
	require.EqualValues(t, 1, state.ProcessedLogs)
	var eventCount int64
	require.NoError(t, mainDB.Model(&IPSecurityEvent{}).Count(&eventCount).Error)
	require.EqualValues(t, 1, eventCount)
}

func TestBackfillIPSecurityProjectionQuarantinesInvalidSourceLogAndContinues(t *testing.T) {
	mainDB, logDB := setupIPSecurityBackfillTest(t)
	require.NoError(t, mainDB.Create(&IPSecurityProjectionState{
		ID: 1, Status: IPSecurityProjectionStatusReady,
	}).Error)
	logs := []Log{
		{
			UserId: 20, Username: "invalid-source-user", CreatedAt: 1_752_501_020,
			Type: LogTypeConsume, Ip: "203.0.113.20", RequestId: strings.Repeat("x", 65),
		},
		{
			UserId: 21, Username: "healthy-source-user", CreatedAt: 1_752_501_021,
			Type: LogTypeConsume, Ip: "203.0.113.21", RequestId: "healthy-after-invalid",
		},
	}
	require.NoError(t, logDB.Create(&logs).Error)

	require.NoError(t, BackfillIPSecurityProjection(context.Background(), 1))
	var state IPSecurityProjectionState
	require.NoError(t, mainDB.First(&state, "id = ?", 1).Error)
	require.Equal(t, logs[1].Id, state.LastLogID)
	require.EqualValues(t, 2, state.ProcessedLogs)
	var eventCount int64
	require.NoError(t, mainDB.Model(&IPSecurityEvent{}).Count(&eventCount).Error)
	require.EqualValues(t, 1, eventCount)
	var quarantine IPSecurityProjectionQuarantine
	require.NoError(t, mainDB.First(&quarantine).Error)
	require.Equal(t, "log:relational:"+strconv.Itoa(logs[0].Id), quarantine.SourceKey)
	require.Equal(t, ErrInvalidIPSecurityEvent.Error(), quarantine.Error)
	require.Equal(t, 1, quarantine.AttemptCount)
}

func TestIPSecurityProjectionQuarantineUpdatesRetryWhenUpsertReportsMatchedRow(t *testing.T) {
	mainDB, _ := setupIPSecurityBackfillTest(t)
	require.NoError(t, mainDB.Transaction(func(tx *gorm.DB) error {
		return recordIPSecurityProjectionQuarantineWithTx(
			tx, "log:relational:99", "log_relational", "99", ErrInvalidIPSecurityEvent,
		)
	}))

	callbackName := "ip_security_backfill_test:simulate_quarantine_client_found_rows"
	require.NoError(t, mainDB.Callback().Create().After("gorm:create").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Schema != nil && tx.Statement.Schema.Table == "ip_security_projection_quarantines" {
			tx.RowsAffected = 1
		}
	}))
	t.Cleanup(func() { _ = mainDB.Callback().Create().Remove(callbackName) })
	require.NoError(t, mainDB.Transaction(func(tx *gorm.DB) error {
		return recordIPSecurityProjectionQuarantineWithTx(
			tx, "log:relational:99", "log_relational", "99", ErrInvalidIPSecurityEvent,
		)
	}))

	var quarantine IPSecurityProjectionQuarantine
	require.NoError(t, mainDB.First(&quarantine, "source_key = ?", "log:relational:99").Error)
	require.Equal(t, 2, quarantine.AttemptCount)
}

func TestBackfillIPSecurityProjectionDoesNotMarkReadyWhileEventsRemainPending(t *testing.T) {
	mainDB, logDB := setupIPSecurityBackfillTest(t)
	require.NoError(t, logDB.Create(&Log{
		UserId: 19, Username: "pending-readiness-user", CreatedAt: 1_752_501_050,
		Type: LogTypeConsume, Ip: "203.0.113.19", RequestId: "pending-readiness",
	}).Error)

	failure := errors.New("rollup unavailable")
	callbackName := "ip_security_backfill_test:fail_initial_rollup"
	require.NoError(t, mainDB.Callback().Create().Before("gorm:create").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Schema != nil && tx.Statement.Schema.Table == "ip_audit_rollups" {
			tx.AddError(failure)
		}
	}))
	t.Cleanup(func() { _ = mainDB.Callback().Create().Remove(callbackName) })

	err := BackfillIPSecurityProjection(context.Background(), 1)
	require.Error(t, err)
	ready, readyErr := IPSecurityProjectionReady(context.Background())
	require.NoError(t, readyErr)
	require.False(t, ready)
	var state IPSecurityProjectionState
	require.NoError(t, mainDB.First(&state, "id = ?", 1).Error)
	require.Equal(t, IPSecurityProjectionStatusFailed, state.Status)
	var pendingCount int64
	require.NoError(t, mainDB.Model(&IPSecurityEvent{}).
		Where("projection_status = ?", IPSecurityEventProjectionPending).Count(&pendingCount).Error)
	require.EqualValues(t, 1, pendingCount)
}

func TestBackfillIPSecurityProjectionSkipsAReadyStateWithAnActiveLease(t *testing.T) {
	mainDB, logDB := setupIPSecurityBackfillTest(t)
	require.NoError(t, mainDB.Create(&IPSecurityProjectionState{
		ID: 1, Status: IPSecurityProjectionStatusReady,
		LockedBy: "other-worker", LockedUntil: time.Now().Add(time.Minute).Unix(),
	}).Error)
	log := Log{
		UserId: 11, Username: "leased-user", CreatedAt: 1_752_501_100,
		Type: LogTypeConsume, Ip: "203.0.113.11", RequestId: "leased-request",
	}
	require.NoError(t, logDB.Create(&log).Error)

	require.NoError(t, BackfillIPSecurityProjection(context.Background(), 1))
	var eventCount int64
	require.NoError(t, mainDB.Model(&IPSecurityEvent{}).Count(&eventCount).Error)
	require.Zero(t, eventCount)

	require.NoError(t, mainDB.Model(&IPSecurityProjectionState{}).Where("id = ?", 1).
		Update("locked_until", time.Now().Add(-time.Minute).Unix()).Error)
	require.NoError(t, BackfillIPSecurityProjection(context.Background(), 1))
	require.NoError(t, mainDB.Model(&IPSecurityEvent{}).Count(&eventCount).Error)
	require.EqualValues(t, 1, eventCount)
}

func TestIPSecurityProjectionLeaseRenewalAlwaysAdvancesVersion(t *testing.T) {
	mainDB, _ := setupIPSecurityBackfillTest(t)
	runID := "lease-version-worker"
	now := common.GetTimestamp()
	require.NoError(t, mainDB.Create(&IPSecurityProjectionState{
		ID: 1, Status: IPSecurityProjectionStatusReady,
		LockedBy: runID, LockedUntil: now + ipSecurityProjectionLeaseSeconds,
	}).Error)

	require.NoError(t, renewIPSecurityProjectionRun(context.Background(), runID))
	require.NoError(t, renewIPSecurityProjectionRun(context.Background(), runID))
	var state IPSecurityProjectionState
	require.NoError(t, mainDB.First(&state, "id = ?", 1).Error)
	require.EqualValues(t, 2, state.LeaseVersion)
}

func TestIPSecurityProjectionCleanupContextHasBoundedDeadline(t *testing.T) {
	ctx, cancel := ipSecurityProjectionCleanupContext()
	defer cancel()
	deadline, ok := ctx.Deadline()
	require.True(t, ok)
	remaining := time.Until(deadline)
	require.Greater(t, remaining, time.Duration(0))
	require.LessOrEqual(t, remaining, 2*time.Second)
}

func TestBackfillIPSecurityProjectionRollsBackEventsWhenCursorUpdateFails(t *testing.T) {
	mainDB, logDB := setupIPSecurityBackfillTest(t)
	require.NoError(t, mainDB.Create(&IPSecurityProjectionState{
		ID: 1, Status: IPSecurityProjectionStatusReady,
	}).Error)
	require.NoError(t, logDB.Create(&Log{
		UserId: 12, Username: "atomic-user", CreatedAt: 1_752_501_200,
		Type: LogTypeConsume, Ip: "203.0.113.12", RequestId: "atomic-request",
	}).Error)

	failure := errors.New("projection cursor update unavailable")
	callbackName := "ip_security_backfill_test:fail_cursor_update"
	require.NoError(t, mainDB.Callback().Update().Before("gorm:update").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement == nil || tx.Statement.Schema == nil || tx.Statement.Schema.Table != "ip_security_projection_states" {
			return
		}
		updates, ok := tx.Statement.Dest.(map[string]interface{})
		if _, advancesCursor := updates["last_log_id"]; ok && advancesCursor {
			tx.AddError(failure)
		}
	}))
	t.Cleanup(func() { _ = mainDB.Callback().Update().Remove(callbackName) })

	require.ErrorIs(t, BackfillIPSecurityProjection(context.Background(), 1), failure)
	var eventCount int64
	require.NoError(t, mainDB.Model(&IPSecurityEvent{}).Count(&eventCount).Error)
	require.Zero(t, eventCount)
	var state IPSecurityProjectionState
	require.NoError(t, mainDB.First(&state, "id = ?", 1).Error)
	require.Zero(t, state.LastLogID)
	require.Zero(t, state.ProcessedLogs)
}

func TestClickHouseIPSecurityBackfillRowsUsesStableKeysetWithoutOffset(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{DryRun: true})
	require.NoError(t, err)
	previousLogDB := LOG_DB
	previousLogType := common.LogDatabaseType()
	LOG_DB = db
	common.SetLogDatabaseType(common.DatabaseTypeClickHouse)
	t.Cleanup(func() {
		LOG_DB = previousLogDB
		common.SetLogDatabaseType(previousLogType)
	})

	query := clickHouseIPSecurityBackfillRows(
		context.Background(),
		clickHouseIPSecurityCursor{CreatedAt: 100, RequestID: "cursor-request", EventIdentity: "event-100"},
		clickHouseIPSecurityCursor{CreatedAt: 200, RequestID: "high-request", EventIdentity: "event-200"},
	).Find(&[]Log{}).Statement.SQL.String()
	upperQuery := strings.ToUpper(query)
	require.NotContains(t, upperQuery, " OFFSET ")
	require.NotContains(t, upperQuery, " LIMIT ", "the canonical result must be streamed once and batched by the caller")
	require.Regexp(t,
		`(?s)FROM logs\s+WHERE tuple\(created_at, request_id, .*?\) > tuple\(.*?\)\s*\) WHERE audit_event_rank = 1`,
		query,
		"the lower keyset bound must be applied before the canonical window",
	)
	require.Contains(t, query, "tuple(logs.created_at, logs.request_id, logs.audit_event_identity) > tuple(")
	require.Contains(t, query, "tuple(logs.created_at, logs.request_id, logs.audit_event_identity) <= tuple(")
	require.Contains(t, query, "logs.created_at asc, logs.request_id asc, logs.audit_event_identity asc")
}

func TestClickHouseIPSecurityReconciliationStartUsesOverlapWindow(t *testing.T) {
	cursor := clickHouseIPSecurityCursor{
		CreatedAt: 1_000, RequestID: "z", EventIdentity: "event-z",
	}
	require.Equal(t, clickHouseIPSecurityCursor{CreatedAt: 700}, clickHouseIPSecurityReconciliationStart(cursor, 300))
	require.Equal(t, clickHouseIPSecurityCursor{}, clickHouseIPSecurityReconciliationStart(cursor, 1_000))
	require.Equal(t, clickHouseIPSecurityCursor{}, clickHouseIPSecurityReconciliationStart(clickHouseIPSecurityCursor{}, 300))
}

func TestIPSecurityLogSourceIdentityBoundsClickHouseIdentity(t *testing.T) {
	previousLogType := common.LogDatabaseType()
	common.SetLogDatabaseType(common.DatabaseTypeClickHouse)
	t.Cleanup(func() { common.SetLogDatabaseType(previousLogType) })

	rawIdentity := strings.Repeat("x", 500)
	sourceKey, sourceType, sourceIdentity := ipSecurityLogSourceIdentity(Log{AuditEventIdentity: rawIdentity})
	require.Equal(t, "log_clickhouse", sourceType)
	require.LessOrEqual(t, len(sourceKey), 255)
	require.LessOrEqual(t, len(sourceIdentity), 191)
	require.NotEqual(t, rawIdentity, sourceIdentity)
	require.Equal(t, sourceKey, "log:clickhouse:"+sourceIdentity)

	repeatedKey, _, repeatedIdentity := ipSecurityLogSourceIdentity(Log{AuditEventIdentity: rawIdentity})
	require.Equal(t, sourceKey, repeatedKey)
	require.Equal(t, sourceIdentity, repeatedIdentity)
	differentKey, _, _ := ipSecurityLogSourceIdentity(Log{AuditEventIdentity: strings.Repeat("y", 500)})
	require.NotEqual(t, sourceKey, differentKey)
}
