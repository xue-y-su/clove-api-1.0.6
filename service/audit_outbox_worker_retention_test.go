package service

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestAuditOutboxRetentionCutoffDefaultsDisablesAndCaps(t *testing.T) {
	previous, existed := os.LookupEnv(auditOutboxRetentionDaysEnv)
	require.NoError(t, os.Unsetenv(auditOutboxRetentionDaysEnv))
	t.Cleanup(func() {
		if existed {
			require.NoError(t, os.Setenv(auditOutboxRetentionDaysEnv, previous))
		} else {
			require.NoError(t, os.Unsetenv(auditOutboxRetentionDaysEnv))
		}
	})
	now := time.Date(2026, 7, 16, 0, 0, 0, 0, time.UTC).Unix()

	cutoff, enabled := auditOutboxRetentionCutoff(now)
	require.True(t, enabled)
	require.Equal(t, now-30*86400, cutoff)

	require.NoError(t, os.Setenv(auditOutboxRetentionDaysEnv, "0"))
	_, enabled = auditOutboxRetentionCutoff(now)
	require.False(t, enabled)

	require.NoError(t, os.Setenv(auditOutboxRetentionDaysEnv, "-1"))
	cutoff, enabled = auditOutboxRetentionCutoff(now)
	require.True(t, enabled)
	require.Equal(t, now-30*86400, cutoff)

	require.NoError(t, os.Setenv(auditOutboxRetentionDaysEnv, "5000"))
	cutoff, enabled = auditOutboxRetentionCutoff(now)
	require.True(t, enabled)
	require.Equal(t, now-3650*86400, cutoff)
}

func TestAuditOutboxWorkerCleansOneDeliveredBatchAndPreservesProcessing(t *testing.T) {
	mainDB, _ := setupAuditOutboxServiceTest(t)
	t.Setenv(auditOutboxRetentionDaysEnv, "1")
	now := common.GetTimestamp()
	require.NoError(t, mainDB.Create(&[]model.AuditOutboxEvent{
		{EventID: "worker-retention-old-1", EventType: "wallet.adjust", Status: model.AuditOutboxStatusDelivered, DeliveredAt: now - 2*86400},
		{EventID: "worker-retention-old-2", EventType: "wallet.adjust", Status: model.AuditOutboxStatusDelivered, DeliveredAt: now - 2*86400},
		{EventID: "worker-retention-processing", EventType: "wallet.adjust", Status: model.AuditOutboxStatusPending, CreatedAt: now - 2*86400, LockedBy: "worker-1", LockedUntil: now + 3600},
	}).Error)

	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	require.NoError(t, runAuditOutboxWorker(ctx, time.Hour, 1))

	var oldDelivered int64
	require.NoError(t, mainDB.Model(&model.AuditOutboxEvent{}).
		Where("status = ? AND delivered_at > 0 AND delivered_at < ?", model.AuditOutboxStatusDelivered, now-86400).
		Count(&oldDelivered).Error)
	require.EqualValues(t, 1, oldDelivered)
	var processing int64
	require.NoError(t, mainDB.Model(&model.AuditOutboxEvent{}).
		Where("event_id = ? AND status = ? AND locked_by = ?", "worker-retention-processing", model.AuditOutboxStatusPending, "worker-1").
		Count(&processing).Error)
	require.EqualValues(t, 1, processing)
}
