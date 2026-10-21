package service

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/ip_security_setting"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestIPSecurityProjectionWorkerStartupPolicyAllowsConfiguredSlave(t *testing.T) {
	previousMaster := common.IsMasterNode
	t.Cleanup(func() { common.IsMasterNode = previousMaster })
	common.IsMasterNode = false

	t.Setenv("IP_SECURITY_PROJECTION_WORKER_ENABLED", "true")
	require.True(t, ipSecurityProjectionWorkerEnabled())

	t.Setenv("IP_SECURITY_PROJECTION_WORKER_ENABLED", "false")
	require.False(t, ipSecurityProjectionWorkerEnabled())
}

func TestIPSecurityProjectionWorkerStartupPolicyKeepsMasterDefault(t *testing.T) {
	previousMaster := common.IsMasterNode
	t.Cleanup(func() { common.IsMasterNode = previousMaster })
	common.IsMasterNode = true
	t.Setenv("IP_SECURITY_PROJECTION_WORKER_ENABLED", "")

	require.True(t, ipSecurityProjectionWorkerEnabled())
}

func TestIPSecurityProjectionWorkerStartupPolicyAllowsExplicitMasterDisable(t *testing.T) {
	previousMaster := common.IsMasterNode
	t.Cleanup(func() { common.IsMasterNode = previousMaster })
	common.IsMasterNode = true
	t.Setenv("IP_SECURITY_PROJECTION_WORKER_ENABLED", "false")

	require.False(t, ipSecurityProjectionWorkerEnabled())
}

func TestIPSecurityProjectionWorkerReadsUpdatedOptionsForEachIteration(t *testing.T) {
	previousOptionMap := common.OptionMap
	common.OptionMapRWMutex.Lock()
	common.OptionMap = map[string]string{
		"ip_security.enabled":                     "true",
		"ip_security.retention_days":              "90",
		"ip_security.projection_interval_seconds": "5",
		"ip_security.projection_batch_size":       "17",
	}
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = previousOptionMap
		common.OptionMapRWMutex.Unlock()
	})

	first := ipSecurityProjectionWorkerPolicy(time.Second, 3)
	require.Equal(t, 5*time.Second, first.retryInterval)
	require.Equal(t, 17, first.batchSize)

	common.OptionMapRWMutex.Lock()
	common.OptionMap["ip_security.projection_interval_seconds"] = "11"
	common.OptionMap["ip_security.projection_batch_size"] = "29"
	common.OptionMapRWMutex.Unlock()

	second := ipSecurityProjectionWorkerPolicy(time.Second, 3)
	require.Equal(t, 11*time.Second, second.retryInterval)
	require.Equal(t, 29, second.batchSize)
}

func TestIPSecurityProjectionWorkerDoesNotShortPollWhileCollectionDisabled(t *testing.T) {
	policy := ipSecurityProjectionPolicy{
		enabled:       false,
		retryInterval: 37 * time.Second,
	}
	require.Equal(t, 37*time.Second, ipSecurityProjectionWaitInterval(policy))
}

func TestIPSecurityProjectionWorkerYieldsBrieflyBetweenBackfillBatches(t *testing.T) {
	policy := ipSecurityProjectionPolicy{enabled: true, retryInterval: 37 * time.Second}

	wait := ipSecurityProjectionBatchWaitInterval(policy, model.IPSecurityProjectionBatchResult{
		Acquired: true, CaughtUp: false,
	})

	require.Positive(t, wait)
	require.Less(t, wait, policy.retryInterval)
	require.Equal(t, policy.retryInterval, ipSecurityProjectionBatchWaitInterval(policy, model.IPSecurityProjectionBatchResult{
		Acquired: false, CaughtUp: false,
	}))
	require.Equal(t, policy.retryInterval, ipSecurityProjectionBatchWaitInterval(policy, model.IPSecurityProjectionBatchResult{
		Acquired: true, CaughtUp: true,
	}))
}

func TestIPSecurityProjectionWorkerUsesEnvironmentAsOptionDefaults(t *testing.T) {
	t.Setenv("IP_SECURITY_PROJECTION_RETRY_SECONDS", "17")
	t.Setenv("IP_SECURITY_PROJECTION_BATCH_SIZE", "29")

	previousOptionMap := common.OptionMap
	common.OptionMapRWMutex.Lock()
	common.OptionMap = ip_security_setting.DefaultOptions()
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = previousOptionMap
		common.OptionMapRWMutex.Unlock()
	})

	policy := ipSecurityProjectionWorkerPolicy(time.Second, 3)
	require.Equal(t, 17*time.Second, policy.retryInterval)
	require.Equal(t, 29, policy.batchSize)
}

func TestIPSecurityProjectionBackfillDoesNotAdvanceWhileDisabled(t *testing.T) {
	mainDB, logDB := setupAuditOutboxServiceTest(t)
	setIPSecurityEnabledForServiceTest(t, false)
	require.NoError(t, logDB.Create(&model.Log{
		UserId: 83, Username: "disabled-backfill-user", CreatedAt: common.GetTimestamp(),
		Type: model.LogTypeConsume, Ip: "203.0.113.83", RequestId: "disabled-backfill-request",
	}).Error)

	require.NoError(t, model.BackfillIPSecurityProjection(context.Background(), 10))
	var eventCount int64
	require.NoError(t, mainDB.Model(&model.IPSecurityEvent{}).Count(&eventCount).Error)
	require.Zero(t, eventCount)
	var stateCount int64
	require.NoError(t, mainDB.Model(&model.IPSecurityProjectionState{}).Count(&stateCount).Error)
	require.Zero(t, stateCount)
}

func TestIPSecurityProjectionWorkerRetainsCleanupWhileCollectionDisabled(t *testing.T) {
	mainDB, _ := setupAuditOutboxServiceTest(t)
	oldTimestamp := common.GetTimestamp() - 91*86400
	require.NoError(t, mainDB.Create(&model.IPSecurityEvent{
		EventID: "disabled-retention-event", UserID: 84, Username: "retention-user",
		CreatedAt: oldTimestamp, IP: "203.0.113.84", MaskedIP: "203.0.113.*",
		Kind: model.IPSecurityKindAPIRequest, ProjectionStatus: model.IPSecurityEventProjectionProjected,
	}).Error)
	setIPSecurityEnabledForServiceTest(t, false)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	require.NoError(t, runIPSecurityProjectionWorker(ctx, 10*time.Millisecond, 10))

	var eventCount int64
	require.NoError(t, mainDB.Model(&model.IPSecurityEvent{}).
		Where("event_id = ?", "disabled-retention-event").Count(&eventCount).Error)
	require.Zero(t, eventCount)
}

func TestIPSecurityProjectionWorkerBackfillsImmediatelyAndStopsIdempotently(t *testing.T) {
	mainDB, logDB := setupAuditOutboxServiceTest(t)
	require.NoError(t, logDB.Create(&model.Log{
		UserId: 77, Username: "worker-user", CreatedAt: 1_752_500_700,
		Type: model.LogTypeConsume, Ip: "203.0.113.77", RequestId: "worker-request",
	}).Error)

	worker := startIPSecurityProjectionWorker(context.Background(), time.Hour, 2)
	require.Eventually(t, func() bool {
		ready, err := model.IPSecurityProjectionReady(context.Background())
		return err == nil && ready
	}, 5*time.Second, 20*time.Millisecond)
	var count int64
	require.NoError(t, mainDB.Model(&model.IPSecurityEvent{}).Count(&count).Error)
	require.EqualValues(t, 1, count)

	stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, worker.Stop(stopCtx))
	require.NoError(t, worker.Stop(stopCtx))
}

func TestIPSecurityProjectionWorkerRepairsPendingEventsAfterReady(t *testing.T) {
	mainDB, _ := setupAuditOutboxServiceTest(t)
	require.NoError(t, mainDB.Create(&model.IPSecurityProjectionState{
		ID: 1, Status: model.IPSecurityProjectionStatusReady,
	}).Error)
	require.NoError(t, mainDB.Create(&model.IPSecurityEvent{
		EventID: "ready-pending-event", UserID: 78, Username: "pending-user",
		CreatedAt: 1_752_600_200, IP: "203.0.113.78", MaskedIP: "203.0.113.*",
		Kind: model.IPSecurityKindAPIError, HTTPStatus: http.StatusUnauthorized, Quota: 4, RequestID: "pending-request",
		ProjectionStatus: model.IPSecurityEventProjectionPending,
	}).Error)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	require.NoError(t, runIPSecurityProjectionWorker(ctx, 10*time.Millisecond, 10))

	var rollups []model.IPAuditRollup
	require.NoError(t, mainDB.Order("bucket asc").Find(&rollups).Error)
	require.Len(t, rollups, 2)
	for _, rollup := range rollups {
		require.Equal(t, 1, rollup.RequestCount)
		require.Equal(t, 1, rollup.ErrorCount)
		require.Equal(t, 1, rollup.AnomalyCount)
		require.Equal(t, 4, rollup.Quota)
	}
}

func TestIPSecurityProjectionWorkerRepairsPendingEventsBeforeBackfillCancellation(t *testing.T) {
	mainDB, logDB := setupAuditOutboxServiceTest(t)
	require.NoError(t, mainDB.Create(&model.IPSecurityProjectionState{
		ID: 1, Status: model.IPSecurityProjectionStatusReady,
	}).Error)
	require.NoError(t, mainDB.Create(&model.IPSecurityEvent{
		EventID: "pending-before-backfill-cancellation", UserID: 82, Username: "pending-user",
		CreatedAt: 1_752_600_500, IP: "203.0.113.82", MaskedIP: "203.0.113.*",
		Kind: model.IPSecurityKindAPIError, RequestID: "pending-before-backfill-cancellation",
		ProjectionStatus: model.IPSecurityEventProjectionPending,
	}).Error)

	ctx, cancel := context.WithCancel(context.Background())
	callbackName := "ip_security_projection_worker_test:cancel_source_reconciliation"
	require.NoError(t, logDB.Callback().Query().Before("gorm:query").Register(callbackName, func(tx *gorm.DB) {
		cancel()
		tx.AddError(context.Canceled)
	}))
	t.Cleanup(func() { _ = logDB.Callback().Query().Remove(callbackName) })
	require.NoError(t, runIPSecurityProjectionWorker(ctx, time.Hour, 10))

	var event model.IPSecurityEvent
	require.NoError(t, mainDB.First(&event, "event_id = ?", "pending-before-backfill-cancellation").Error)
	require.Equal(t, model.IPSecurityEventProjectionProjected, event.ProjectionStatus)
}

func TestIPSecurityProjectionWorkerReconcilesMissedSourceLogsAfterReady(t *testing.T) {
	mainDB, logDB := setupAuditOutboxServiceTest(t)
	require.NoError(t, mainDB.Create(&model.IPSecurityProjectionState{
		ID: 1, Status: model.IPSecurityProjectionStatusReady,
	}).Error)
	require.NoError(t, logDB.Create(&model.Log{
		UserId: 79, Username: "missed-source-user", CreatedAt: 1_752_600_300,
		Type: model.LogTypeConsume, Ip: "203.0.113.79", RequestId: "missed-source-request",
	}).Error)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	require.NoError(t, runIPSecurityProjectionWorker(ctx, 10*time.Millisecond, 10))

	var event model.IPSecurityEvent
	require.NoError(t, mainDB.First(&event, "user_id = ?", 79).Error)
	require.Equal(t, "203.0.113.79", event.IP)
	require.Equal(t, model.IPSecurityEventProjectionProjected, event.ProjectionStatus)
	var state model.IPSecurityProjectionState
	require.NoError(t, mainDB.First(&state, "id = ?", 1).Error)
	require.EqualValues(t, 1, state.ProcessedLogs)
}

func TestIPSecurityProjectionWorkerRepairsPendingEventsWhenSourceReconciliationFails(t *testing.T) {
	mainDB, logDB := setupAuditOutboxServiceTest(t)
	require.NoError(t, mainDB.Create(&model.IPSecurityProjectionState{
		ID: 1, Status: model.IPSecurityProjectionStatusReady,
	}).Error)
	require.NoError(t, mainDB.Create(&model.IPSecurityEvent{
		EventID: "pending-during-source-failure", UserID: 80, Username: "pending-user",
		CreatedAt: 1_752_600_400, IP: "203.0.113.80", MaskedIP: "203.0.113.*",
		Kind: model.IPSecurityKindAPIError, RequestID: "pending-during-source-failure",
		ProjectionStatus: model.IPSecurityEventProjectionPending,
	}).Error)
	require.NoError(t, logDB.Create(&model.Log{
		UserId: 81, Username: "missed-source-user", CreatedAt: 1_752_600_401,
		Type: model.LogTypeConsume, Ip: "203.0.113.81", RequestId: "source-insert-failure",
	}).Error)

	failure := errors.New("security event insert unavailable")
	callbackName := "ip_security_projection_worker_test:fail_source_event_insert"
	require.NoError(t, mainDB.Callback().Create().Before("gorm:create").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Schema != nil && tx.Statement.Schema.Table == "ip_security_events" {
			tx.AddError(failure)
		}
	}))
	t.Cleanup(func() { _ = mainDB.Callback().Create().Remove(callbackName) })

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	require.NoError(t, runIPSecurityProjectionWorker(ctx, 10*time.Millisecond, 10))

	var pending model.IPSecurityEvent
	require.NoError(t, mainDB.First(&pending, "event_id = ?", "pending-during-source-failure").Error)
	require.Equal(t, model.IPSecurityEventProjectionProjected, pending.ProjectionStatus)
}
