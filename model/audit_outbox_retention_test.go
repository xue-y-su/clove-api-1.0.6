package model

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupAuditOutboxRetentionTest(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/audit-outbox-retention.db"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&AuditOutboxEvent{}))
	sqlDB, err := db.DB()
	require.NoError(t, err)
	previousDB := DB
	DB = db
	t.Cleanup(func() {
		DB = previousDB
		require.NoError(t, sqlDB.Close())
	})
	return db
}

func TestDeleteDeliveredAuditOutboxBatchIsBoundedAndPreservesUndeliveredEvents(t *testing.T) {
	db := setupAuditOutboxRetentionTest(t)
	cutoff := time.Date(2026, 7, 16, 0, 0, 0, 0, time.UTC).Unix()
	require.NoError(t, db.Create(&[]AuditOutboxEvent{
		{EventID: "delivered-old-1", EventType: "wallet.adjust", Status: AuditOutboxStatusDelivered, DeliveredAt: cutoff - 20},
		{EventID: "delivered-old-2", EventType: "wallet.adjust", Status: AuditOutboxStatusDelivered, DeliveredAt: cutoff - 10},
		{EventID: "delivered-recent", EventType: "wallet.adjust", Status: AuditOutboxStatusDelivered, DeliveredAt: cutoff + 10},
		{EventID: "delivered-without-time", EventType: "wallet.adjust", Status: AuditOutboxStatusDelivered},
		{EventID: "pending-old", EventType: "wallet.adjust", Status: AuditOutboxStatusPending, DeliveredAt: cutoff - 30},
		{EventID: "processing-old", EventType: "wallet.adjust", Status: AuditOutboxStatusPending, DeliveredAt: cutoff - 30, LockedBy: "worker-1", LockedUntil: cutoff + 60},
	}).Error)

	deleted, err := DeleteDeliveredAuditOutboxBatch(context.Background(), cutoff, 1)
	require.NoError(t, err)
	require.EqualValues(t, 1, deleted)
	var oldDelivered int64
	require.NoError(t, db.Model(&AuditOutboxEvent{}).
		Where("status = ? AND delivered_at > 0 AND delivered_at < ?", AuditOutboxStatusDelivered, cutoff).
		Count(&oldDelivered).Error)
	require.EqualValues(t, 1, oldDelivered)

	deleted, err = DeleteDeliveredAuditOutboxBatch(context.Background(), cutoff, 10)
	require.NoError(t, err)
	require.EqualValues(t, 1, deleted)
	var retained []AuditOutboxEvent
	require.NoError(t, db.Order("event_id asc").Find(&retained).Error)
	require.Len(t, retained, 4)
	require.Equal(t, []string{
		"delivered-recent", "delivered-without-time", "pending-old", "processing-old",
	}, []string{retained[0].EventID, retained[1].EventID, retained[2].EventID, retained[3].EventID})
}

func TestAuditOutboxRetentionMigrationCreatesLookupIndex(t *testing.T) {
	db := setupAuditOutboxRetentionTest(t)
	require.True(t, db.Migrator().HasIndex(&AuditOutboxEvent{}, "idx_audit_outbox_delivered_retention"))
}

func TestDeleteDeliveredAuditOutboxBatchCandidateQueryUsesRetentionIndexOrder(t *testing.T) {
	db := setupAuditOutboxRetentionTest(t)
	cutoff := time.Date(2026, 7, 16, 0, 0, 0, 0, time.UTC).Unix()
	require.NoError(t, db.Create(&[]AuditOutboxEvent{
		{EventID: "query-plan-old-1", EventType: "wallet.adjust", Status: AuditOutboxStatusDelivered, DeliveredAt: cutoff - 20},
		{EventID: "query-plan-old-2", EventType: "wallet.adjust", Status: AuditOutboxStatusDelivered, DeliveredAt: cutoff - 10},
	}).Error)

	var candidateSQL string
	var candidateArgs []interface{}
	const callbackName = "audit_outbox_retention_test:capture_candidate_query"
	require.NoError(t, db.Callback().Query().After("gorm:query").Register(callbackName, func(tx *gorm.DB) {
		query := tx.Statement.SQL.String()
		if strings.Contains(query, "audit_outbox_events") && strings.Contains(query, "delivered_at") && strings.Contains(query, "ORDER BY") {
			candidateSQL = query
			candidateArgs = append([]interface{}(nil), tx.Statement.Vars...)
		}
	}))
	t.Cleanup(func() { _ = db.Callback().Query().Remove(callbackName) })

	_, err := DeleteDeliveredAuditOutboxBatch(context.Background(), cutoff, 1)
	require.NoError(t, err)
	require.NotEmpty(t, candidateSQL)

	var plan []struct {
		Detail string `gorm:"column:detail"`
	}
	require.NoError(t, db.Raw("EXPLAIN QUERY PLAN "+candidateSQL, candidateArgs...).Scan(&plan).Error)
	require.NotEmpty(t, plan)
	usesRetentionIndex := false
	for _, step := range plan {
		if strings.Contains(step.Detail, "idx_audit_outbox_delivered_retention") {
			usesRetentionIndex = true
		}
		require.NotContains(t, strings.ToUpper(step.Detail), "TEMP B-TREE FOR ORDER BY", plan)
	}
	require.True(t, usesRetentionIndex, plan)
}
