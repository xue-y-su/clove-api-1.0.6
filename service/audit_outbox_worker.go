package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
)

const (
	auditOutboxRetentionDaysEnv     = "AUDIT_OUTBOX_RETENTION_DAYS"
	defaultAuditOutboxRetentionDays = 30
	maxAuditOutboxRetentionDays     = 3650
)

type AuditOutboxMetrics struct {
	PendingCount            int64  `json:"pending_count"`
	OldestPendingAgeSeconds int64  `json:"oldest_pending_age_seconds"`
	FailureCount            int64  `json:"failure_count"`
	LastError               string `json:"last_error,omitempty"`
}

type AuditOutboxWorker struct {
	cancel   context.CancelFunc
	done     chan struct{}
	stopOnce sync.Once
}

func auditOutboxRetentionCutoff(now int64) (int64, bool) {
	if now <= 0 {
		return 0, false
	}
	days := common.GetEnvOrDefault(auditOutboxRetentionDaysEnv, defaultAuditOutboxRetentionDays)
	if days == 0 {
		return 0, false
	}
	if days < 0 {
		days = defaultAuditOutboxRetentionDays
	}
	if days > maxAuditOutboxRetentionDays {
		days = maxAuditOutboxRetentionDays
	}
	return now - int64(days)*86400, true
}

func GetAuditOutboxMetrics(ctx context.Context, now int64) (AuditOutboxMetrics, error) {
	metrics := AuditOutboxMetrics{}
	if model.DB == nil {
		return metrics, errors.New("audit outbox database is not initialized")
	}
	if now <= 0 {
		now = common.GetTimestamp()
	}

	pending := model.DB.WithContext(ctx).Model(&model.AuditOutboxEvent{}).
		Where("status = ?", model.AuditOutboxStatusPending)
	if err := pending.Count(&metrics.PendingCount).Error; err != nil {
		return metrics, err
	}
	if metrics.PendingCount > 0 {
		var oldestCreatedAt int64
		if err := pending.Select("COALESCE(MIN(created_at), 0)").Scan(&oldestCreatedAt).Error; err != nil {
			return metrics, err
		}
		if oldestCreatedAt > 0 && now > oldestCreatedAt {
			metrics.OldestPendingAgeSeconds = now - oldestCreatedAt
		}
	}

	failed := model.DB.WithContext(ctx).Model(&model.AuditOutboxEvent{}).
		Where("status = ? AND attempt_count > 0 AND last_error <> ?", model.AuditOutboxStatusPending, "")
	if err := failed.Count(&metrics.FailureCount).Error; err != nil {
		return metrics, err
	}
	if metrics.FailureCount > 0 {
		var latest model.AuditOutboxEvent
		if err := failed.Select("last_error", "updated_at", "id").Order("updated_at desc, id desc").First(&latest).Error; err != nil {
			return metrics, err
		}
		metrics.LastError = latest.LastError
	}
	return metrics, nil
}

func StartAuditOutboxWorker() *AuditOutboxWorker {
	intervalSeconds := common.GetEnvOrDefault("AUDIT_OUTBOX_INTERVAL_SECONDS", 2)
	if intervalSeconds < 1 {
		intervalSeconds = 1
	}
	batchSize := common.GetEnvOrDefault("AUDIT_OUTBOX_BATCH_SIZE", 100)
	return startAuditOutboxWorker(context.Background(), time.Duration(intervalSeconds)*time.Second, batchSize)
}

func startAuditOutboxWorker(parent context.Context, interval time.Duration, batchSize int) *AuditOutboxWorker {
	ctx, cancel := context.WithCancel(parent) // #nosec G118 -- cancel is retained and called by worker.Stop.
	worker := &AuditOutboxWorker{
		cancel: cancel,
		done:   make(chan struct{}),
	}
	go func() { // #nosec G118 -- this owned worker exits when worker.Stop cancels ctx.
		defer close(worker.done)
		if err := runAuditOutboxWorker(ctx, interval, batchSize); err != nil && !errors.Is(err, context.Canceled) {
			logger.LogError(context.Background(), fmt.Sprintf("audit outbox worker stopped: %v", err))
		}
	}()
	return worker
}

func runAuditOutboxWorker(ctx context.Context, interval time.Duration, batchSize int) error {
	if err := ctx.Err(); err != nil {
		return nil
	}
	if interval <= 0 {
		return errors.New("audit outbox worker interval must be positive")
	}
	deliver := func() {
		delivered, failed, err := DeliverPending(ctx, batchSize)
		if err != nil && !errors.Is(err, context.Canceled) {
			logger.LogError(context.Background(), fmt.Sprintf("audit outbox delivery failed: %v", err))
			return
		}
		if failed > 0 {
			logger.LogWarn(context.Background(), fmt.Sprintf("audit outbox delivery completed with failures: delivered=%d failed=%d", delivered, failed))
		}
	}
	cleanup := func() {
		cutoff, enabled := auditOutboxRetentionCutoff(common.GetTimestamp())
		if !enabled {
			return
		}
		if _, err := model.DeleteDeliveredAuditOutboxBatch(ctx, cutoff, batchSize); err != nil && !errors.Is(err, context.Canceled) {
			logger.LogError(context.Background(), fmt.Sprintf("audit outbox retention failed: %v", err))
		}
	}
	runOnce := func() {
		deliver()
		cleanup()
	}

	runOnce()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			runOnce()
		}
	}
}

func (worker *AuditOutboxWorker) Stop(ctx context.Context) error {
	if worker == nil {
		return nil
	}
	worker.stopOnce.Do(worker.cancel)
	select {
	case <-worker.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
