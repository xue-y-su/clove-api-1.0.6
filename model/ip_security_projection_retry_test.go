package model

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type ipSecurityProjectionRetryMetadata struct {
	ProjectionStatus string
	ProjectionError  string
	AttemptCount     int
	NextAttemptAt    int64
	ProjectedAt      int64
}

func loadIPSecurityProjectionRetryMetadata(t *testing.T, db *gorm.DB, eventID string) ipSecurityProjectionRetryMetadata {
	t.Helper()
	var metadata ipSecurityProjectionRetryMetadata
	require.NoError(t, db.Model(&IPSecurityEvent{}).
		Select("projection_status", "projection_error", "attempt_count", "next_attempt_at", "projected_at").
		Where("event_id = ?", eventID).
		Take(&metadata).Error)
	return metadata
}

func TestProjectPendingIPSecurityEventsBacksOffPoisonedHead(t *testing.T) {
	db := openIPSecurityEventTestDB(t)
	previousDB := DB
	DB = db
	t.Cleanup(func() { DB = previousDB })

	poisoned := IPSecurityEvent{
		EventID: "poisoned-pending-event", UserID: 72, Username: "poisoned-user",
		CreatedAt: 1_752_600_200, IP: "203.0.113.72", MaskedIP: "203.0.113.*",
		Kind: IPSecurityKindAPIRequest, Quota: 7, RequestID: "poisoned-request",
		ProjectionStatus: IPSecurityEventProjectionPending,
	}
	healthy := IPSecurityEvent{
		EventID: "healthy-pending-event", UserID: 73, Username: "healthy-user",
		CreatedAt: 1_752_600_201, IP: "203.0.113.73", MaskedIP: "203.0.113.*",
		Kind: IPSecurityKindAPIRequest, Quota: 11, RequestID: "healthy-request",
		ProjectionStatus: IPSecurityEventProjectionPending,
	}
	require.NoError(t, db.Create(&poisoned).Error)
	require.NoError(t, db.Create(&healthy).Error)

	failure := errors.New("poisoned rollup remains unavailable")
	callbackName := "ip_security_projection_retry_test:fail_poisoned_rollup"
	require.NoError(t, db.Callback().Create().Before("gorm:create").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement == nil || tx.Statement.Schema == nil || tx.Statement.Schema.Table != "ip_audit_rollups" {
			return
		}
		rollup, ok := tx.Statement.Dest.(*IPAuditRollup)
		if ok && rollup.UserID == poisoned.UserID {
			tx.AddError(failure)
		}
	}))
	callbackRemoved := false
	t.Cleanup(func() {
		if !callbackRemoved {
			_ = db.Callback().Create().Remove(callbackName)
		}
	})

	firstAttemptAt := time.Now().Unix()
	projected, failed, err := ProjectPendingIPSecurityEvents(context.Background(), 1)
	require.NoError(t, err)
	require.Zero(t, projected)
	require.Equal(t, 1, failed)

	projected, failed, err = ProjectPendingIPSecurityEvents(context.Background(), 1)
	require.NoError(t, err)
	require.Equal(t, 1, projected)
	require.Zero(t, failed)

	poisonedMetadata := loadIPSecurityProjectionRetryMetadata(t, db, poisoned.EventID)
	require.Equal(t, IPSecurityEventProjectionPending, poisonedMetadata.ProjectionStatus)
	require.Equal(t, 1, poisonedMetadata.AttemptCount)
	require.Greater(t, poisonedMetadata.NextAttemptAt, firstAttemptAt)
	require.Contains(t, poisonedMetadata.ProjectionError, failure.Error())
	firstNextAttemptAt := poisonedMetadata.NextAttemptAt

	healthyMetadata := loadIPSecurityProjectionRetryMetadata(t, db, healthy.EventID)
	require.Equal(t, IPSecurityEventProjectionProjected, healthyMetadata.ProjectionStatus)
	require.Zero(t, healthyMetadata.AttemptCount)
	require.Zero(t, healthyMetadata.NextAttemptAt)
	require.Empty(t, healthyMetadata.ProjectionError)
	require.NotZero(t, healthyMetadata.ProjectedAt)

	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		replay := poisoned
		return RecordIPSecurityEventWithTx(tx, &replay)
	}))
	poisonedMetadata = loadIPSecurityProjectionRetryMetadata(t, db, poisoned.EventID)
	require.Equal(t, 1, poisonedMetadata.AttemptCount)
	require.Equal(t, firstNextAttemptAt, poisonedMetadata.NextAttemptAt)

	require.NoError(t, db.Callback().Create().Remove(callbackName))
	callbackRemoved = true
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		replay := poisoned
		return RecordIPSecurityEventWithTx(tx, &replay)
	}))
	poisonedMetadata = loadIPSecurityProjectionRetryMetadata(t, db, poisoned.EventID)
	require.Equal(t, IPSecurityEventProjectionPending, poisonedMetadata.ProjectionStatus)
	require.Equal(t, 1, poisonedMetadata.AttemptCount)
	require.Equal(t, firstNextAttemptAt, poisonedMetadata.NextAttemptAt)

	require.NoError(t, db.Model(&IPSecurityEvent{}).Where("event_id = ?", poisoned.EventID).
		Update("next_attempt_at", 0).Error)
	projected, failed, err = ProjectPendingIPSecurityEvents(context.Background(), 1)
	require.NoError(t, err)
	require.Equal(t, 1, projected)
	require.Zero(t, failed)
	poisonedMetadata = loadIPSecurityProjectionRetryMetadata(t, db, poisoned.EventID)
	require.Equal(t, IPSecurityEventProjectionProjected, poisonedMetadata.ProjectionStatus)
	require.Zero(t, poisonedMetadata.AttemptCount)
	require.Zero(t, poisonedMetadata.NextAttemptAt)
	require.Empty(t, poisonedMetadata.ProjectionError)
	require.NotZero(t, poisonedMetadata.ProjectedAt)

	var rollupCount int64
	require.NoError(t, db.Model(&IPAuditRollup{}).Count(&rollupCount).Error)
	require.EqualValues(t, 4, rollupCount)
}

func TestIPSecurityEventSurvivesRollupFailureAndRetriesExactlyOnce(t *testing.T) {
	db := openIPSecurityEventTestDB(t)
	failure := errors.New("rollup unavailable")
	callbackName := "ip_security_projection_retry_test:fail_rollup"
	require.NoError(t, db.Callback().Create().Before("gorm:create").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Schema != nil && tx.Statement.Schema.Table == "ip_audit_rollups" {
			tx.AddError(failure)
		}
	}))
	callbackRemoved := false
	t.Cleanup(func() {
		if !callbackRemoved {
			_ = db.Callback().Create().Remove(callbackName)
		}
	})

	original := IPSecurityEvent{
		EventID: "retry-rollup-event", UserID: 71, Username: "retry-user",
		CreatedAt: 1_752_600_100, IP: "203.0.113.71",
		Kind: IPSecurityKindAPIRequest, Quota: 9, RequestID: "retry-rollup-request",
	}
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		event := original
		return RecordIPSecurityEventWithTx(tx, &event)
	}))

	var eventCount int64
	require.NoError(t, db.Model(&IPSecurityEvent{}).Count(&eventCount).Error)
	require.EqualValues(t, 1, eventCount)
	var rollupCount int64
	require.NoError(t, db.Model(&IPAuditRollup{}).Count(&rollupCount).Error)
	require.Zero(t, rollupCount)

	require.NoError(t, db.Callback().Create().Remove(callbackName))
	callbackRemoved = true
	require.NoError(t, db.Model(&IPSecurityEvent{}).Where("event_id = ?", original.EventID).
		Update("next_attempt_at", 0).Error)
	for range 2 {
		require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
			event := original
			return RecordIPSecurityEventWithTx(tx, &event)
		}))
	}

	require.NoError(t, db.Model(&IPSecurityEvent{}).Count(&eventCount).Error)
	require.EqualValues(t, 1, eventCount)
	require.NoError(t, db.Model(&IPAuditRollup{}).Count(&rollupCount).Error)
	require.EqualValues(t, 2, rollupCount)
	var lifetime IPAuditRollup
	require.NoError(t, db.First(&lifetime, "bucket = ? AND user_id = ? AND ip = ?", 0, original.UserID, original.IP).Error)
	require.Equal(t, 1, lifetime.RequestCount)
	require.Equal(t, 1, lifetime.APIRequestCount)
	require.Equal(t, 9, lifetime.Quota)
}
