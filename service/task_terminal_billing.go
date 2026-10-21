package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

// FinalizeTaskTerminal commits one asynchronous task terminal transition and
// its billing mutation as one database transaction. The task row is the
// serialization point: a success and failure callback racing for the same row
// can have only one winner, while a retry after a terminal status has already
// been persisted can safely finish an interrupted legacy refund.
func FinalizeTaskTerminal(ctx context.Context, taskID int64, mutation model.TaskTerminalMutation) (bool, error) {
	if taskID <= 0 {
		return false, errors.New("invalid task id")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if mutation.ToStatus != model.TaskStatusSuccess && mutation.ToStatus != model.TaskStatusFailure {
		return false, fmt.Errorf("invalid terminal task status %q", mutation.ToStatus)
	}

	var (
		applied    bool
		userID     int
		tokenKey   string
		tokenDelta int
	)
	err := model.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var task model.Task
		if err := model.LockForUpdate(tx).Where("id = ?", taskID).First(&task).Error; err != nil {
			return err
		}

		fromStatus := mutation.FromStatus
		if fromStatus == "" {
			fromStatus = task.Status
		}
		// A callback that lost the race is a no-op. A callback retrying the same
		// terminal status is allowed to complete a legacy status-first refund.
		if task.Status != fromStatus && task.Status != mutation.ToStatus {
			return nil
		}
		if (task.Status == model.TaskStatusFailure || task.Status == model.TaskStatusSuccess) && task.Status != mutation.ToStatus {
			return nil
		}

		actualQuota := mutation.ActualQuota
		if actualQuota < 0 {
			actualQuota = task.Quota
		}
		if mutation.ToStatus == model.TaskStatusFailure && !mutation.SkipBilling {
			actualQuota = 0
		}
		if actualQuota < 0 {
			return errors.New("invalid terminal task quota")
		}
		if mutation.SkipBilling {
			// Historical rows were never safely pre-charged. Preserve their quota
			// while still recording the terminal status.
			actualQuota = task.Quota
		}

		// If the row already has the requested terminal state and quota, the
		// transaction was completed by an earlier attempt.
		if task.Status == mutation.ToStatus && task.Quota == actualQuota {
			return nil
		}

		preConsumedQuota := task.Quota
		quotaDelta := actualQuota - preConsumedQuota
		operationID := taskTerminalOperationID(task.TaskID, mutation.ToStatus, actualQuota)
		billingApplied := mutation.SkipBilling || quotaDelta == 0
		if !billingApplied {
			var err error
			billingApplied, err = terminalOperationExists(tx, operationID)
			if err != nil {
				return err
			}
		}
		if !billingApplied {
			if err := adjustTaskFundingWithTx(tx, &task, quotaDelta, operationID); err != nil {
				return err
			}
			var tokenErr error
			tokenKey, tokenDelta, tokenErr = adjustTaskTokenQuotaWithTx(tx, &task, quotaDelta)
			if tokenErr != nil {
				return tokenErr
			}

			logType := model.LogTypeRefund
			logQuota := -quotaDelta
			if quotaDelta > 0 {
				logType = model.LogTypeConsume
				logQuota = quotaDelta
			}
			other := taskBillingOther(&task)
			other["task_id"] = task.TaskID
			other["pre_consumed_quota"] = preConsumedQuota
			other["actual_quota"] = actualQuota
			other["operation_id"] = operationID
			if err := model.RecordTaskBillingLogWithTx(tx, model.RecordTaskBillingLogParams{
				UserId:    task.UserId,
				LogType:   logType,
				Content:   mutation.Reason,
				ChannelId: task.ChannelId,
				ModelName: taskModelName(&task),
				Quota:     logQuota,
				TokenId:   task.PrivateData.TokenId,
				Group:     task.Group,
				Other:     other,
				NodeName:  task.PrivateData.NodeName,
			}, operationID); err != nil {
				return err
			}
			if quotaDelta > 0 {
				// Usage counters are intentionally updated after the terminal
				// funding mutation, but remain in this transaction when available.
				if err := tx.Model(&model.User{}).Where("id = ?", task.UserId).
					Updates(map[string]any{
						"used_quota":    gorm.Expr("used_quota + ?", quotaDelta),
						"request_count": gorm.Expr("request_count + ?", 1),
					}).Error; err != nil {
					return err
				}
				if task.ChannelId > 0 {
					if err := tx.Model(&model.Channel{}).Where("id = ?", task.ChannelId).
						UpdateColumn("used_quota", gorm.Expr("used_quota + ?", quotaDelta)).Error; err != nil {
						return err
					}
				}
			}
		}

		task.Status = mutation.ToStatus
		task.Progress = "100%"
		if task.FinishTime == 0 {
			task.FinishTime = time.Now().Unix()
		}
		if mutation.ToStatus == model.TaskStatusFailure && mutation.Reason != "" {
			task.FailReason = mutation.Reason
		}
		if mutation.ToStatus == model.TaskStatusSuccess && mutation.ResultURL != "" {
			task.PrivateData.ResultURL = mutation.ResultURL
		}
		if mutation.Data != nil {
			task.Data = mutation.Data
		}
		task.Quota = actualQuota
		task.UpdatedAt = time.Now().Unix()
		if err := tx.Save(&task).Error; err != nil {
			return err
		}
		userID = task.UserId
		applied = true
		return nil
	})
	if err != nil {
		return false, err
	}
	if applied {
		_ = model.InvalidateUserCache(userID)
		if tokenDelta != 0 {
			model.SyncTokenQuotaCacheDelta(tokenKey, tokenDelta)
		}
	}
	return applied, nil
}

func taskTerminalOperationID(taskID string, status model.TaskStatus, actualQuota int) string {
	if status == model.TaskStatusFailure {
		return fmt.Sprintf("task:%s:failure-refund", taskID)
	}
	return fmt.Sprintf("task:%s:success-settle:%d", taskID, actualQuota)
}

func terminalOperationExists(tx *gorm.DB, operationID string) (bool, error) {
	var ledger model.WalletLedgerEntry
	result := tx.Where("operation_id = ?", operationID).Limit(1).Find(&ledger)
	if result.Error != nil {
		return false, result.Error
	}
	if result.RowsAffected > 0 {
		return true, nil
	}
	var log model.Log
	result = tx.Where("event_id = ?", operationID).Limit(1).Find(&log)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

func adjustTaskFundingWithTx(tx *gorm.DB, task *model.Task, quotaDelta int, operationID string) error {
	if quotaDelta == 0 {
		return nil
	}
	if taskIsSubscription(task) {
		return model.PostConsumeUserSubscriptionDeltaWithTx(tx, task.PrivateData.SubscriptionId, int64(quotaDelta))
	}
	walletDelta := -int64(quotaDelta)
	phase := "settle"
	if quotaDelta < 0 {
		phase = "refund"
	}
	_, err := model.AdjustWalletWithTx(tx, task.UserId, walletDelta, operationID, "task", model.WalletAdjustReference{
		Type: "task", ID: task.TaskID, Context: phase,
	})
	return err
}

// adjustTaskTokenQuotaWithTx returns the cache key and signed remaining-quota
// delta. Missing or deleted tokens are intentionally ignored: the durable
// wallet/subscription adjustment and task transition still settle correctly.
func adjustTaskTokenQuotaWithTx(tx *gorm.DB, task *model.Task, quotaDelta int) (string, int, error) {
	if task.PrivateData.TokenId <= 0 || quotaDelta == 0 {
		return "", 0, nil
	}
	var token model.Token
	if err := tx.Where("id = ? AND deleted_at IS NULL", task.PrivateData.TokenId).First(&token).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", 0, nil
		}
		return "", 0, err
	}
	tokenDelta := -quotaDelta
	if err := model.AdjustTokenQuotaWithTx(tx, token.Id, tokenDelta); err != nil {
		return "", 0, err
	}
	return token.Key, tokenDelta, nil
}

// FinalizeTaskTerminal is deliberately the only terminal billing entry point.
// Keep this alias short for callers that already use a service-local helper.
func finalizeTaskTerminal(ctx context.Context, taskID int64, mutation model.TaskTerminalMutation) (bool, error) {
	return FinalizeTaskTerminal(ctx, taskID, mutation)
}

func normalizeTerminalReason(reason string) string {
	return strings.TrimSpace(reason)
}
