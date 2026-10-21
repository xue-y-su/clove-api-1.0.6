package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

func FinalizePendingSubscriptionRedemptionRefunds(ctx context.Context, limit int) (int, error) {
	if model.DB == nil {
		return 0, ErrUserRedemptionInvalid
	}
	if limit <= 0 {
		limit = 100
	}
	var ids []int64
	if err := model.DB.WithContext(ctx).Model(&model.UserRedemptionCode{}).
		Where("reward_type = ? AND status = ?", model.UserRedemptionRewardSubscription, model.UserRedemptionStatusRefundPending).
		Order("id asc").
		Limit(limit).
		Pluck("id", &ids).Error; err != nil {
		return 0, err
	}

	finalized := 0
	for _, id := range ids {
		var creatorID int
		var didFinalize bool
		var candidate model.UserRedemptionCode
		if err := model.DB.WithContext(ctx).Select("creator_user_id").First(&candidate, id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				continue
			}
			return finalized, err
		}
		err := model.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			creatorExists := true
			if err := model.LockUserRedemptionCreatorUnscoped(tx, candidate.CreatorUserID); err != nil {
				if !errors.Is(err, gorm.ErrRecordNotFound) {
					return err
				}
				creatorExists = false
			}
			row, err := model.LockUserRedemptionCodeByID(tx, id)
			if err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return nil
				}
				return err
			}
			if normalizeUserRedemptionRewardType(row.RewardType) != model.UserRedemptionRewardSubscription || row.Status != model.UserRedemptionStatusRefundPending {
				return nil
			}
			refundable, err := userRedemptionRefundableQuotaWithTx(tx, row)
			if err != nil {
				return err
			}
			if !creatorExists {
				row.Status = model.UserRedemptionStatusForfeited
				row.ForfeitedQuota += refundable
				row.TerminalReason = "creator_deleted"
				if err := tx.Model(&model.UserRedemptionCode{}).Where("id = ?", row.ID).Updates(map[string]any{
					"status": row.Status, "forfeited_quota": row.ForfeitedQuota, "terminal_reason": row.TerminalReason,
				}).Error; err != nil {
					return err
				}
				if err := enqueueUserRedemptionBusinessAuditWithTx(tx, fmt.Sprintf("user-redemption:%d:subscription-forfeit:creator-deleted", row.ID), "user.redemption.subscription.forfeit", 0, row.CreatorUserID, row, refundable, 0); err != nil {
					return err
				}
				didFinalize = true
				return nil
			}
			if err := refundUserRedemptionWithTx(tx, &row, row.TerminalAt); err != nil {
				return err
			}
			if err := enqueueUserRedemptionBusinessAuditWithTx(tx, fmt.Sprintf("user-redemption:%d:subscription-refund:audit", row.ID), "user.redemption.subscription.refund", 0, row.CreatorUserID, row, refundable, 0); err != nil {
				return err
			}
			creatorID = row.CreatorUserID
			didFinalize = true
			return nil
		})
		if err != nil {
			return finalized, err
		}
		if didFinalize {
			if creatorID > 0 {
				_ = invalidateUserRedemptionCache(creatorID)
			}
			finalized++
		}
	}
	return finalized, nil
}
