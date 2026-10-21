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

type InvitationRewardWorker struct {
	cancel   context.CancelFunc
	done     chan struct{}
	stopOnce sync.Once
}

func ProcessPendingInvitationRewards(ctx context.Context, batchSize int) (processed int, failed int, err error) {
	if model.DB == nil {
		return 0, 0, errors.New("invitation reward database is not initialized")
	}
	if batchSize <= 0 {
		return 0, 0, errors.New("invitation reward batch size must be positive")
	}
	if batchSize > 500 {
		batchSize = 500
	}

	var useIDs []int64
	err = model.DB.WithContext(ctx).Table("invite_code_uses AS uses").
		Where(`(
			uses.inviter_reward > 0 AND
			uses.inviter_reward_status <> ? AND
			NOT EXISTS (
				SELECT 1 FROM invite_reward_payouts AS payouts
				WHERE payouts.invite_code_use_id = uses.id
					AND payouts.trigger_type = ?
					AND payouts.recipient_kind = ?
			)
		) OR uses.invitee_reward_status IN ?`,
			model.InviteRewardStatusSkipped,
			model.InviteRewardTriggerRegistrationInviter,
			model.InviteRewardRecipientInviter,
			[]string{
				model.InviteRewardStatusPending,
				model.InviteRewardStatusPendingReview,
				model.InviteRewardStatusApproved,
				model.InviteRewardStatusRejected,
				model.InviteRewardStatusFailed,
				model.InviteRewardStatusAvailable,
			}).
		Order("uses.id ASC").Limit(batchSize).Pluck("uses.id", &useIDs).Error
	if err != nil {
		return 0, 0, err
	}
	for _, useID := range useIDs {
		if err := ctx.Err(); err != nil {
			return processed, failed, err
		}
		if err := ProcessInvitationUseRewards(ctx, useID); err != nil {
			failed++
			logger.LogWarn(context.Background(), fmt.Sprintf("invitation reward processing failed for use %d: %v", useID, err))
			continue
		}
		processed++
	}
	return processed, failed, nil
}

func StartInvitationRewardWorker() *InvitationRewardWorker {
	intervalSeconds := common.GetEnvOrDefault("INVITATION_REWARD_INTERVAL_SECONDS", 30)
	if intervalSeconds < 1 {
		intervalSeconds = 1
	}
	batchSize := common.GetEnvOrDefault("INVITATION_REWARD_BATCH_SIZE", 100)
	if batchSize < 1 {
		batchSize = 100
	}
	return startInvitationRewardWorker(context.Background(), time.Duration(intervalSeconds)*time.Second, batchSize)
}

func startInvitationRewardWorker(parent context.Context, interval time.Duration, batchSize int) *InvitationRewardWorker {
	ctx, cancel := context.WithCancel(parent) // #nosec G118 -- cancel is retained and called by worker.Stop.
	worker := &InvitationRewardWorker{
		cancel: cancel,
		done:   make(chan struct{}),
	}
	go func() { // #nosec G118 -- this owned worker exits when worker.Stop cancels ctx.
		defer close(worker.done)
		if err := runInvitationRewardWorker(ctx, interval, batchSize); err != nil && !errors.Is(err, context.Canceled) {
			logger.LogError(context.Background(), fmt.Sprintf("invitation reward worker stopped: %v", err))
		}
	}()
	return worker
}

func runInvitationRewardWorker(ctx context.Context, interval time.Duration, batchSize int) error {
	if ctx.Err() != nil {
		return nil
	}
	if interval <= 0 {
		return errors.New("invitation reward worker interval must be positive")
	}
	process := func() {
		processed, failed, err := ProcessPendingInvitationRewards(ctx, batchSize)
		if err != nil && !errors.Is(err, context.Canceled) {
			logger.LogError(context.Background(), fmt.Sprintf("invitation reward batch failed: %v", err))
			return
		}
		if failed > 0 {
			logger.LogWarn(context.Background(), fmt.Sprintf("invitation reward batch completed with failures: processed=%d failed=%d", processed, failed))
		}
	}

	process()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			process()
		}
	}
}

func (worker *InvitationRewardWorker) Stop(ctx context.Context) error {
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
