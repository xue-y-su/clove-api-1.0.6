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
	"github.com/QuantumNous/new-api/setting/ip_security_setting"
)

type IPSecurityProjectionWorker struct {
	cancel   context.CancelFunc
	done     chan struct{}
	stopOnce sync.Once
}

func StartIPSecurityProjectionWorker() *IPSecurityProjectionWorker {
	intervalSeconds := common.GetEnvOrDefault("IP_SECURITY_PROJECTION_RETRY_SECONDS", 60)
	if intervalSeconds < 5 {
		intervalSeconds = 5
	}
	if intervalSeconds > 3600 {
		intervalSeconds = 3600
	}
	batchSize := common.GetEnvOrDefault("IP_SECURITY_PROJECTION_BATCH_SIZE", 500)
	if batchSize < 10 {
		batchSize = 10
	}
	if batchSize > 5000 {
		batchSize = 5000
	}
	if !ipSecurityProjectionWorkerEnabled() {
		worker := &IPSecurityProjectionWorker{cancel: func() {}, done: make(chan struct{})}
		close(worker.done)
		return worker
	}
	return startIPSecurityProjectionWorker(context.Background(), time.Duration(intervalSeconds)*time.Second, batchSize)
}

func ipSecurityProjectionWorkerEnabled() bool {
	return common.GetEnvOrDefaultBool("IP_SECURITY_PROJECTION_WORKER_ENABLED", common.IsMasterNode)
}

type ipSecurityProjectionPolicy struct {
	enabled       bool
	retryInterval time.Duration
	batchSize     int
	retentionDays int
}

func ipSecurityProjectionWorkerPolicy(fallbackInterval time.Duration, fallbackBatch int) ipSecurityProjectionPolicy {
	settings := ip_security_setting.GetSettings()
	policy := ipSecurityProjectionPolicy{
		enabled:       settings.Enabled,
		retryInterval: fallbackInterval,
		batchSize:     fallbackBatch,
	}
	if policy.retryInterval <= 0 {
		policy.retryInterval = 60 * time.Second
	}
	if policy.batchSize < 1 {
		policy.batchSize = 500
	}
	if policy.batchSize > 5000 {
		policy.batchSize = 5000
	}
	common.OptionMapRWMutex.RLock()
	_, intervalConfigured := common.OptionMap[ip_security_setting.ProjectionIntervalSecondsKey]
	_, batchConfigured := common.OptionMap[ip_security_setting.ProjectionBatchSizeKey]
	_, retentionConfigured := common.OptionMap[ip_security_setting.RetentionDaysKey]
	common.OptionMapRWMutex.RUnlock()
	if intervalConfigured {
		policy.retryInterval = time.Duration(settings.ProjectionIntervalSeconds) * time.Second
	}
	if batchConfigured {
		policy.batchSize = settings.ProjectionBatchSize
	}
	if retentionConfigured {
		policy.retentionDays = settings.RetentionDays
	}
	return policy
}

func ipSecurityProjectionWaitInterval(policy ipSecurityProjectionPolicy) time.Duration {
	return policy.retryInterval
}

func ipSecurityProjectionBatchWaitInterval(policy ipSecurityProjectionPolicy, result model.IPSecurityProjectionBatchResult) time.Duration {
	if !policy.enabled || !result.Acquired || result.CaughtUp {
		return ipSecurityProjectionWaitInterval(policy)
	}
	return min(100*time.Millisecond, policy.retryInterval)
}

func startIPSecurityProjectionWorker(parent context.Context, retryInterval time.Duration, batchSize int) *IPSecurityProjectionWorker {
	ctx, cancel := context.WithCancel(parent) // #nosec G118 -- cancel is retained and called by worker.Stop.
	worker := &IPSecurityProjectionWorker{cancel: cancel, done: make(chan struct{})}
	go func() { // #nosec G118 -- this owned worker exits when worker.Stop cancels ctx.
		defer close(worker.done)
		if err := runIPSecurityProjectionWorker(ctx, retryInterval, batchSize); err != nil && !errors.Is(err, context.Canceled) {
			logger.LogError(context.Background(), fmt.Sprintf("IP security projection worker stopped: %v", err))
		}
	}()
	return worker
}

func runIPSecurityProjectionWorker(ctx context.Context, retryInterval time.Duration, batchSize int) error {
	if err := ctx.Err(); err != nil {
		return nil
	}
	if retryInterval <= 0 {
		return errors.New("IP security projection retry interval must be positive")
	}
	if batchSize < 1 {
		batchSize = 500
	}
	if batchSize > 5000 {
		batchSize = 5000
	}
	for {
		policy := ipSecurityProjectionWorkerPolicy(retryInterval, batchSize)
		batchResult := model.IPSecurityProjectionBatchResult{CaughtUp: true}
		if policy.enabled {
			_, failed, retryErr := model.ProjectPendingIPSecurityEvents(ctx, policy.batchSize)
			if ctx.Err() != nil || errors.Is(retryErr, context.Canceled) {
				return nil
			}
			if retryErr != nil {
				logger.LogError(context.Background(), fmt.Sprintf("IP security projection retry failed: %v", retryErr))
			} else if failed > 0 {
				logger.LogError(context.Background(), fmt.Sprintf("IP security projection retry left %d events pending", failed))
			}
			var reconcileErr error
			batchResult, reconcileErr = model.BackfillIPSecurityProjectionBatch(ctx, policy.batchSize)
			if ctx.Err() != nil || errors.Is(reconcileErr, context.Canceled) {
				return nil
			}
			if reconcileErr != nil {
				logger.LogError(context.Background(), fmt.Sprintf("IP security projection backfill failed: %v", reconcileErr))
			}
		}
		if cutoff, enabled := model.IPSecurityProjectionRetentionCutoffForDays(common.GetTimestamp(), policy.retentionDays); enabled {
			for {
				deleted, cleanupErr := model.DeleteOldIPSecurityProjectionBatch(ctx, cutoff, policy.batchSize)
				if errors.Is(cleanupErr, context.Canceled) {
					return nil
				}
				if cleanupErr != nil {
					logger.LogError(context.Background(), fmt.Sprintf("IP security projection retention failed: %v", cleanupErr))
					break
				}
				if deleted < int64(policy.batchSize) {
					break
				}
			}
		}
		waitInterval := ipSecurityProjectionBatchWaitInterval(policy, batchResult)
		timer := time.NewTimer(waitInterval)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return nil
		case <-timer.C:
		}
	}
}

func (worker *IPSecurityProjectionWorker) Stop(ctx context.Context) error {
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
