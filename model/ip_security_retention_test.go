package model

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestDeleteOldLogAlsoBoundsIPSecurityProjectionStorage(t *testing.T) {
	mainDB, logDB := setupIPSecurityBackfillTest(t)
	target := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC).Unix()
	oldDay := utcDayBucket(target) - 86400
	currentDay := utcDayBucket(target)

	require.NoError(t, logDB.Create(&[]Log{
		{CreatedAt: target - 10, Type: LogTypeConsume, RequestId: "old-log"},
		{CreatedAt: target + 10, Type: LogTypeConsume, RequestId: "new-log"},
	}).Error)
	require.NoError(t, mainDB.Create(&[]IPSecurityEvent{
		{
			EventID: "old-projected", UserID: 1, CreatedAt: target - 10,
			IP: "203.0.113.1", MaskedIP: "203.0.113.*", Kind: IPSecurityKindAPIRequest,
			ProjectionStatus: IPSecurityEventProjectionProjected,
		},
		{
			EventID: "old-pending", UserID: 2, CreatedAt: target - 20,
			IP: "203.0.113.2", MaskedIP: "203.0.113.*", Kind: IPSecurityKindAPIRequest,
			ProjectionStatus: IPSecurityEventProjectionPending,
		},
		{
			EventID: "new-projected", UserID: 3, CreatedAt: target + 10,
			IP: "203.0.113.3", MaskedIP: "203.0.113.*", Kind: IPSecurityKindAPIRequest,
			ProjectionStatus: IPSecurityEventProjectionProjected,
		},
	}).Error)
	require.NoError(t, mainDB.Create(&[]IPAuditRollup{
		{Bucket: oldDay, UserID: 1, IP: "203.0.113.1", MaskedIP: "203.0.113.*", LastSeenAt: target - 10},
		{Bucket: currentDay, UserID: 3, IP: "203.0.113.3", MaskedIP: "203.0.113.*", LastSeenAt: target + 10},
		{Bucket: 0, UserID: 4, IP: "203.0.113.4", MaskedIP: "203.0.113.*", LastSeenAt: target - 10},
		{Bucket: 0, UserID: 5, IP: "203.0.113.5", MaskedIP: "203.0.113.*", LastSeenAt: target + 10},
	}).Error)
	require.NoError(t, mainDB.Create(&[]IPSecurityProjectionQuarantine{
		{SourceKey: "old-quarantine", SourceType: "log_relational", SourceIdentity: "1", Error: "invalid", AttemptCount: 1, FirstSeenAt: target - 20, LastSeenAt: target - 10},
		{SourceKey: "new-quarantine", SourceType: "log_relational", SourceIdentity: "2", Error: "invalid", AttemptCount: 1, FirstSeenAt: target + 5, LastSeenAt: target + 10},
	}).Error)

	count, err := CountOldLog(context.Background(), target)
	require.NoError(t, err)
	require.EqualValues(t, 6, count)
	deleted, err := DeleteOldLog(context.Background(), target, 2)
	require.NoError(t, err)
	require.EqualValues(t, 6, deleted)

	var countRemaining int64
	require.NoError(t, logDB.Model(&Log{}).Where("request_id = ?", "old-log").Count(&countRemaining).Error)
	require.Zero(t, countRemaining)
	require.NoError(t, logDB.Model(&Log{}).Where("request_id = ?", "new-log").Count(&countRemaining).Error)
	require.EqualValues(t, 1, countRemaining)
	require.NoError(t, mainDB.Model(&IPSecurityEvent{}).Where("event_id = ?", "old-projected").Count(&countRemaining).Error)
	require.Zero(t, countRemaining)
	require.NoError(t, mainDB.Model(&IPSecurityEvent{}).Where("event_id = ?", "old-pending").Count(&countRemaining).Error)
	require.Zero(t, countRemaining)
	require.NoError(t, mainDB.Model(&IPSecurityEvent{}).Where("event_id = ?", "new-projected").Count(&countRemaining).Error)
	require.EqualValues(t, 1, countRemaining)
	require.NoError(t, mainDB.Model(&IPAuditRollup{}).Where("bucket = ?", oldDay).Count(&countRemaining).Error)
	require.Zero(t, countRemaining)
	require.NoError(t, mainDB.Model(&IPAuditRollup{}).Where("bucket = ? OR (bucket = 0 AND last_seen_at >= ?)", currentDay, target).Count(&countRemaining).Error)
	require.EqualValues(t, 2, countRemaining)
	require.NoError(t, mainDB.Model(&IPSecurityProjectionQuarantine{}).Where("source_key = ?", "old-quarantine").Count(&countRemaining).Error)
	require.Zero(t, countRemaining)
	require.NoError(t, mainDB.Model(&IPSecurityProjectionQuarantine{}).Where("source_key = ?", "new-quarantine").Count(&countRemaining).Error)
	require.EqualValues(t, 1, countRemaining)
}

func TestDeleteOldIPSecurityProjectionBatchBoundsPendingEventCleanup(t *testing.T) {
	mainDB, _ := setupIPSecurityBackfillTest(t)
	target := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC).Unix()
	require.NoError(t, mainDB.Create(&[]IPSecurityEvent{
		{EventID: "pending-old-1", UserID: 1, CreatedAt: target - 30, IP: "203.0.113.1", MaskedIP: "203.0.113.*", Kind: IPSecurityKindAPIRequest, ProjectionStatus: IPSecurityEventProjectionPending},
		{EventID: "pending-old-2", UserID: 2, CreatedAt: target - 20, IP: "203.0.113.2", MaskedIP: "203.0.113.*", Kind: IPSecurityKindAPIRequest, ProjectionStatus: IPSecurityEventProjectionPending},
		{EventID: "pending-old-3", UserID: 3, CreatedAt: target - 10, IP: "203.0.113.3", MaskedIP: "203.0.113.*", Kind: IPSecurityKindAPIRequest, ProjectionStatus: IPSecurityEventProjectionPending},
		{EventID: "pending-recent", UserID: 4, CreatedAt: target + 10, IP: "203.0.113.4", MaskedIP: "203.0.113.*", Kind: IPSecurityKindAPIRequest, ProjectionStatus: IPSecurityEventProjectionPending},
	}).Error)

	deleted, err := DeleteOldIPSecurityProjectionBatch(context.Background(), target, 2)
	require.NoError(t, err)
	require.EqualValues(t, 2, deleted)
	var oldCount int64
	require.NoError(t, mainDB.Model(&IPSecurityEvent{}).
		Where("created_at < ? AND projection_status = ?", target, IPSecurityEventProjectionPending).
		Count(&oldCount).Error)
	require.EqualValues(t, 1, oldCount)

	deleted, err = DeleteOldIPSecurityProjectionBatch(context.Background(), target, 2)
	require.NoError(t, err)
	require.EqualValues(t, 1, deleted)
	var recentCount int64
	require.NoError(t, mainDB.Model(&IPSecurityEvent{}).
		Where("event_id = ?", "pending-recent").Count(&recentCount).Error)
	require.EqualValues(t, 1, recentCount)
}

func TestDeleteOldLogRebuildsMixedLifetimeRollupFromRetainedEvents(t *testing.T) {
	mainDB, _ := setupIPSecurityBackfillTest(t)
	target := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC).Unix()
	oldAt := target - 86410
	newAt := target + 10
	for _, event := range []IPSecurityEvent{
		{
			EventID: "retention-old", UserID: 7, Username: "old-name", CreatedAt: oldAt,
			IP: "203.0.113.7", Kind: IPSecurityKindAPIRequest, Quota: 3,
		},
		{
			EventID: "retention-unauthorized", UserID: 7, Username: "new-name", CreatedAt: newAt,
			IP: "203.0.113.7", Kind: IPSecurityKindAPIError, HTTPStatus: http.StatusUnauthorized, Quota: 5,
		},
	} {
		event := event
		require.NoError(t, RecordIPSecurityEventWithTx(mainDB, &event))
	}
	require.NoError(t, mainDB.Create(&IPSecurityEvent{
		EventID: "retention-upstream-error", UserID: 7, Username: "ignored-name", CreatedAt: newAt + 1,
		IP: "203.0.113.7", MaskedIP: "203.0.113.*", Kind: IPSecurityKindAPIError,
		HTTPStatus: http.StatusServiceUnavailable, Quota: 7, ProjectionStatus: IPSecurityEventProjectionProjected,
	}).Error)

	count, err := CountOldLog(context.Background(), target)
	require.NoError(t, err)
	require.EqualValues(t, 2, count)
	deleted, err := DeleteOldLog(context.Background(), target, 1)
	require.NoError(t, err)
	require.EqualValues(t, 2, deleted)

	var lifetime IPAuditRollup
	require.NoError(t, mainDB.First(&lifetime, "bucket = ? AND user_id = ? AND ip = ?", 0, 7, "203.0.113.7").Error)
	require.Equal(t, "new-name", lifetime.Username)
	require.Equal(t, 1, lifetime.RequestCount)
	require.Zero(t, lifetime.APIRequestCount)
	require.Equal(t, 1, lifetime.ErrorCount)
	require.Equal(t, 1, lifetime.UnauthorizedCount)
	require.Equal(t, 1, lifetime.AnomalyCount)
	require.Equal(t, 5, lifetime.Quota)
	require.Equal(t, newAt, lifetime.FirstSeenAt)
	require.Equal(t, newAt, lifetime.LastSeenAt)

	var eventCount int64
	require.NoError(t, mainDB.Model(&IPSecurityEvent{}).Where("event_id = ?", "retention-old").Count(&eventCount).Error)
	require.Zero(t, eventCount)
	require.NoError(t, mainDB.Model(&IPSecurityEvent{}).Where("event_id = ?", "retention-unauthorized").Count(&eventCount).Error)
	require.EqualValues(t, 1, eventCount)
	require.NoError(t, mainDB.Model(&IPSecurityEvent{}).Where("event_id = ?", "retention-upstream-error").Count(&eventCount).Error)
	require.EqualValues(t, 1, eventCount)
	var oldDailyCount int64
	require.NoError(t, mainDB.Model(&IPAuditRollup{}).Where("bucket = ?", utcDayBucket(oldAt)).Count(&oldDailyCount).Error)
	require.Zero(t, oldDailyCount)
}

func TestDeleteOldIPSecurityProjectionRechecksLifetimeCutoffBeforeDelete(t *testing.T) {
	mainDB, _ := setupIPSecurityBackfillTest(t)
	target := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC).Unix()
	lifetime := IPAuditRollup{
		Bucket: 0, UserID: 9, Username: "active-user", IP: "203.0.113.9", MaskedIP: "203.0.113.*",
		RequestCount: 1, LastSeenAt: target - 10,
	}
	require.NoError(t, mainDB.Create(&lifetime).Error)

	advancedAfterQuery := false
	var advanceErr error
	const callbackName = "ip_security_retention_test:advance_lifetime_after_query"
	require.NoError(t, mainDB.Callback().Query().After("gorm:query").Register(callbackName, func(tx *gorm.DB) {
		if advancedAfterQuery || tx.Statement == nil || tx.Statement.Table != "ip_audit_rollups" {
			return
		}
		query := tx.Statement.SQL.String()
		if !strings.Contains(query, "bucket = 0") || !strings.Contains(query, "last_seen_at <") {
			return
		}
		advancedAfterQuery = true
		advanceErr = mainDB.Model(&IPAuditRollup{}).Where("id = ?", lifetime.ID).
			UpdateColumn("last_seen_at", target+10).Error
	}))
	t.Cleanup(func() {
		_ = mainDB.Callback().Query().Remove(callbackName)
	})

	deleted, err := deleteOldIPSecurityProjectionBatch(context.Background(), target, 10)
	require.True(t, advancedAfterQuery)
	require.NoError(t, advanceErr)
	require.NoError(t, err)
	require.Zero(t, deleted)

	var retained IPAuditRollup
	require.NoError(t, mainDB.First(&retained, lifetime.ID).Error)
	require.Equal(t, target+10, retained.LastSeenAt)
}

func TestIPSecurityProjectionRetentionCutoffUsesClickHouseLogTTL(t *testing.T) {
	previousLogType := common.LogDatabaseType()
	previousOptionMap := common.OptionMap
	common.SetLogDatabaseType(common.DatabaseTypeClickHouse)
	common.OptionMapRWMutex.Lock()
	common.OptionMap = map[string]string{"ip_security.retention_days": "0"}
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.SetLogDatabaseType(previousLogType)
		common.OptionMapRWMutex.Lock()
		common.OptionMap = previousOptionMap
		common.OptionMapRWMutex.Unlock()
	})
	t.Setenv("LOG_SQL_CLICKHOUSE_TTL_DAYS", "7")

	now := time.Date(2026, 7, 16, 0, 0, 0, 0, time.UTC).Unix()
	cutoff, enabled := IPSecurityProjectionRetentionCutoff(now)
	require.True(t, enabled)
	require.Equal(t, now-7*86400, cutoff)

	t.Setenv("LOG_SQL_CLICKHOUSE_TTL_DAYS", "0")
	_, enabled = IPSecurityProjectionRetentionCutoff(now)
	require.False(t, enabled)
	common.SetLogDatabaseType(common.DatabaseTypeSQLite)
	t.Setenv("LOG_SQL_CLICKHOUSE_TTL_DAYS", "7")
	_, enabled = IPSecurityProjectionRetentionCutoff(now)
	require.False(t, enabled)
}

func TestIPSecurityProjectionRetentionCutoffUsesConfiguredRelationalRetention(t *testing.T) {
	previousLogType := common.LogDatabaseType()
	common.SetLogDatabaseType(common.DatabaseTypeSQLite)
	previousOptionMap := common.OptionMap
	common.OptionMapRWMutex.Lock()
	common.OptionMap = map[string]string{
		"ip_security.enabled":                     "true",
		"ip_security.retention_days":              "7",
		"ip_security.projection_interval_seconds": "60",
		"ip_security.projection_batch_size":       "500",
	}
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.SetLogDatabaseType(previousLogType)
		common.OptionMapRWMutex.Lock()
		common.OptionMap = previousOptionMap
		common.OptionMapRWMutex.Unlock()
	})

	now := time.Date(2026, 7, 16, 0, 0, 0, 0, time.UTC).Unix()
	cutoff, enabled := IPSecurityProjectionRetentionCutoff(now)
	require.True(t, enabled)
	require.Equal(t, now-7*86400, cutoff)

	common.OptionMapRWMutex.Lock()
	common.OptionMap["ip_security.retention_days"] = "0"
	common.OptionMapRWMutex.Unlock()
	_, enabled = IPSecurityProjectionRetentionCutoff(now)
	require.False(t, enabled)
}
