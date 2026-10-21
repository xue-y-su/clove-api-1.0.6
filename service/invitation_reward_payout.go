package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	ErrInviteRewardUnavailable     = errors.New("invitation reward is unavailable")
	ErrInviteRewardReviewInvalid   = errors.New("invitation reward review is invalid")
	ErrInviteRewardMutationInvalid = errors.New("invitation reward mutation is invalid")
)

type ReviewInvitationRewardPayoutRequestInput struct {
	RequestID    int64
	ReviewerID   int
	Approve      bool
	ReviewReason string
}

type InvitationRewardPayoutMutationInput struct {
	RequestID int64
	ActorID   int
	Reason    string
}

type InvitationRewardSummary struct {
	InvitedCount          int64  `json:"invited_count"`
	TotalEarned           int64  `json:"total_earned"`
	Available             int64  `json:"available"`
	PendingPayout         int64  `json:"pending_payout"`
	Paid                  int64  `json:"paid"`
	Frozen                int64  `json:"frozen"`
	CashCommissionVisible bool   `json:"cash_commission_visible"`
	CashCommissionRate    string `json:"cash_commission_rate"`
}

type InvitationRewardPayoutRequestListItem struct {
	model.InviteRewardPayoutRequest
	Username         string `json:"username" gorm:"column:username"`
	DisplayName      string `json:"display_name" gorm:"column:display_name"`
	UserRole         int    `json:"user_role" gorm:"column:user_role"`
	ReviewerUsername string `json:"reviewer_username" gorm:"column:reviewer_username"`
	ItemCount        int    `json:"item_count" gorm:"column:item_count"`
	ReversedAmount   int64  `json:"reversed_amount" gorm:"column:reversed_amount"`
	NetAmount        int64  `json:"net_amount" gorm:"column:net_amount"`
}

type InvitationRewardPayoutRequestListResult struct {
	Items    []InvitationRewardPayoutRequestListItem `json:"items"`
	Total    int64                                   `json:"total"`
	Page     int                                     `json:"page"`
	PageSize int                                     `json:"page_size"`
}

type InvitationRewardPayoutSource struct {
	RewardID          int64  `json:"reward_id" gorm:"column:reward_id"`
	InviteCodeUseID   int64  `json:"invite_code_use_id" gorm:"column:invite_code_use_id"`
	TriggerType       string `json:"trigger_type" gorm:"column:trigger_type"`
	TriggerID         string `json:"trigger_id" gorm:"column:trigger_id"`
	EligibleQuota     int64  `json:"eligible_quota" gorm:"column:eligible_quota"`
	RateBasisPoints   int64  `json:"rate_basis_points" gorm:"column:rate_basis_points"`
	Amount            int64  `json:"amount" gorm:"column:amount"`
	ReversedAmount    int64  `json:"reversed_amount" gorm:"column:reversed_amount"`
	NetAmount         int64  `json:"net_amount" gorm:"column:net_amount"`
	ReversalState     string `json:"reversal_state" gorm:"column:reversal_state"`
	Status            string `json:"status" gorm:"column:status"`
	SourceUserID      int    `json:"source_user_id" gorm:"column:source_user_id"`
	SourceUsername    string `json:"source_username" gorm:"column:source_username"`
	SourceDisplayName string `json:"source_display_name" gorm:"column:source_display_name"`
	RegistrationIP    string `json:"registration_ip" gorm:"column:registration_ip"`
	RegisteredAt      int64  `json:"registered_at" gorm:"column:registered_at"`
	InviteCode        string `json:"invite_code" gorm:"column:invite_code"`
}

type InvitationRewardPayoutActionListItem struct {
	model.InviteRewardPayoutAction
	ActorUsername    string `json:"actor_username" gorm:"column:actor_username"`
	ActorDisplayName string `json:"actor_display_name" gorm:"column:actor_display_name"`
}

type InvitationRewardPayoutRequestDetail struct {
	Request InvitationRewardPayoutRequestListItem  `json:"request"`
	Rewards []InvitationRewardPayoutSource         `json:"rewards"`
	Actions []InvitationRewardPayoutActionListItem `json:"actions"`
}

func createInvitationRewardPayoutsWithTx(tx *gorm.DB, use model.InviteCodeUse) error {
	if err := upsertInvitationInviterRewardWithTx(tx, use); err != nil {
		return err
	}
	return settleInvitationInviteeRewardWithTx(tx, use)
}

func upsertInvitationInviterRewardWithTx(tx *gorm.DB, use model.InviteCodeUse) error {
	status := invitationRewardPayoutStatus(use.InviterReward, use.InviterRewardStatus)
	if use.InviterReward <= 0 || status == model.InviteRewardStatusSkipped {
		if use.InviterRewardStatus == model.InviteRewardStatusSkipped {
			return nil
		}
		return tx.Model(&model.InviteCodeUse{}).Where("id = ?", use.ID).
			Update("inviter_reward_status", model.InviteRewardStatusSkipped).Error
	}
	triggerID := strconv.FormatInt(use.ID, 10)
	var payout model.InviteRewardPayout
	err := tx.Where("trigger_type = ? AND trigger_id = ? AND recipient_kind = ?", model.InviteRewardTriggerRegistrationInviter, triggerID, model.InviteRewardRecipientInviter).First(&payout).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		payout = model.InviteRewardPayout{
			InviteCodeUseID: use.ID,
			TriggerType:     model.InviteRewardTriggerRegistrationInviter,
			TriggerID:       triggerID,
			RecipientKind:   model.InviteRewardRecipientInviter,
			RecipientUserID: use.InviterUserID,
			EligibleQuota:   int64(use.InviterReward),
			Amount:          int64(use.InviterReward),
			ReversalState:   model.InviteRewardReversalNone,
			Status:          status,
		}
		if err := tx.Create(&payout).Error; err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else if payout.Status != status {
		if err := tx.Model(&model.InviteRewardPayout{}).Where("id = ?", payout.ID).Update("status", status).Error; err != nil {
			return err
		}
	}
	if use.InviterRewardStatus == status {
		return nil
	}
	return tx.Model(&model.InviteCodeUse{}).Where("id = ?", use.ID).Update("inviter_reward_status", status).Error
}

func settleInvitationInviteeRewardWithTx(tx *gorm.DB, use model.InviteCodeUse) error {
	status := invitationRewardPayoutStatus(use.InviteeReward, use.InviteeRewardStatus)
	triggerID := strconv.FormatInt(use.ID, 10)
	var payout model.InviteRewardPayout
	err := tx.Where("trigger_type = ? AND trigger_id = ? AND recipient_kind = ?", model.InviteRewardTriggerRegistrationInviter, triggerID, model.InviteRewardRecipientInvitee).First(&payout).Error
	payoutFound := err == nil
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	if payoutFound {
		switch payout.Status {
		case model.InviteRewardStatusPaid:
			status = model.InviteRewardStatusPaid
		case model.InviteRewardStatusPendingPayout:
			status = model.InviteRewardStatusPendingPayout
		case model.InviteRewardStatusSkipped:
			status = model.InviteRewardStatusSkipped
		}
	}

	if status != model.InviteRewardStatusPaid &&
		status != model.InviteRewardStatusPendingPayout &&
		status != model.InviteRewardStatusSkipped {
		if _, err := AdjustWithTx(tx, use.RegisteredUserID, int64(use.InviteeReward),
			fmt.Sprintf("invitee_gift:%d", use.RegisteredUserID), "invitation_gift", WalletReference{
				Type: "user", ID: fmt.Sprintf("%d", use.RegisteredUserID), Context: "invitee",
			}); err != nil {
			return err
		}
		status = model.InviteRewardStatusPaid
	}

	if payoutFound && payout.Status != status {
		if err := tx.Model(&model.InviteRewardPayout{}).Where("id = ?", payout.ID).Update("status", status).Error; err != nil {
			return err
		}
	}
	if use.InviteeRewardStatus == status {
		return nil
	}
	return tx.Model(&model.InviteCodeUse{}).Where("id = ?", use.ID).Update("invitee_reward_status", status).Error
}

func invitationRewardPayoutStatus(amount int, status string) string {
	if amount <= 0 {
		return model.InviteRewardStatusSkipped
	}
	switch status {
	case model.InviteRewardStatusAvailable,
		model.InviteRewardStatusPendingPayout,
		model.InviteRewardStatusRejected,
		model.InviteRewardStatusPaid,
		model.InviteRewardStatusFrozen,
		model.InviteRewardStatusSkipped:
		return status
	case model.InviteRewardStatusWithdrawn, model.InviteRewardStatusCompleted:
		return model.InviteRewardStatusPaid
	default:
		return model.InviteRewardStatusAvailable
	}
}

func SubmitInvitationRewardPayout(ctx context.Context, userID int) (*model.InviteRewardPayoutRequest, error) {
	if model.DB == nil || userID <= 0 || !operation_setting.IsPaymentComplianceConfirmed() {
		return nil, ErrInviteRewardUnavailable
	}
	for attempt := 0; attempt < 4; attempt++ {
		request, err := submitInvitationRewardPayoutOnce(ctx, userID)
		if err == nil {
			return request, nil
		}
		if errors.Is(err, ErrInviteRewardUnavailable) || isInvitationUniqueConflict(err) {
			return nil, ErrInviteRewardUnavailable
		}
		if !isSQLiteBusyError(err) {
			return nil, err
		}
		time.Sleep(time.Duration(attempt+1) * 10 * time.Millisecond)
	}
	return nil, ErrInviteRewardUnavailable
}

func submitInvitationRewardPayoutOnce(ctx context.Context, userID int) (*model.InviteRewardPayoutRequest, error) {
	var request model.InviteRewardPayoutRequest
	err := model.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		user, err := model.LockInvitationUser(tx, userID)
		if err != nil || user.Status != common.UserStatusEnabled {
			return ErrInviteRewardUnavailable
		}
		_, err = model.LockPendingInviteRewardPayoutRequest(tx, userID)
		if err == nil {
			return ErrInviteRewardUnavailable
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		rewards, err := model.LockAvailableInviteRewardPayouts(tx, userID)
		if err != nil {
			return err
		}
		if len(rewards) == 0 {
			return ErrInviteRewardUnavailable
		}
		var amount int64
		rewardIDs := make([]int64, 0, len(rewards))
		for _, reward := range rewards {
			netAmount, err := invitationRewardNetAmount(reward)
			if err != nil {
				return ErrInviteRewardUnavailable
			}
			if netAmount == 0 {
				continue
			}
			if amount > int64(^uint64(0)>>1)-netAmount {
				return ErrInviteRewardUnavailable
			}
			amount += netAmount
			rewardIDs = append(rewardIDs, reward.ID)
		}
		if len(rewardIDs) == 0 || amount <= 0 {
			return ErrInviteRewardUnavailable
		}
		pendingUserID := userID
		request = model.InviteRewardPayoutRequest{
			OperationID: "invite-reward-payout:" + uuid.NewString(), UserID: userID,
			PendingUserSlot: &pendingUserID, Amount: amount,
			Status: model.InviteRewardPayoutRequestStatusPending,
		}
		if err := tx.Create(&request).Error; err != nil {
			return err
		}
		items := make([]model.InviteRewardPayoutRequestItem, 0, len(rewardIDs))
		for _, rewardID := range rewardIDs {
			items = append(items, model.InviteRewardPayoutRequestItem{RequestID: request.ID, RewardID: rewardID})
		}
		if err := tx.Create(&items).Error; err != nil {
			return err
		}
		result := tx.Model(&model.InviteRewardPayout{}).
			Where("id IN ? AND recipient_user_id = ? AND status = ?", rewardIDs, userID, model.InviteRewardStatusAvailable).
			Update("status", model.InviteRewardStatusPendingPayout)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != int64(len(rewardIDs)) {
			return ErrInviteRewardUnavailable
		}
		for _, reward := range rewards {
			if reward.TriggerType == model.InviteRewardTriggerRegistrationInviter {
				if err := updateInvitationRewardUseStatusWithTx(tx, reward, model.InviteRewardStatusPendingPayout); err != nil {
					return err
				}
			}
		}
		return EnqueueWithTx(tx, AuditEvent{
			EventID: fmt.Sprintf("invitation:reward-payout-request:%d:submitted", request.ID),
			Type:    "invitation.reward_payout_submitted", Result: "success", ActorID: userID,
			Target: fmt.Sprintf("invite-reward-payout-request:%d", request.ID),
			Params: map[string]any{
				"request_id": request.ID, "target_user_id": userID, "amount": amount,
				"item_count": len(rewardIDs), "operation_id": request.OperationID,
			},
		})
	})
	if err != nil {
		return nil, err
	}
	return &request, nil
}

func ReviewInvitationRewardPayoutRequest(ctx context.Context, input ReviewInvitationRewardPayoutRequestInput) error {
	input.ReviewReason = strings.TrimSpace(input.ReviewReason)
	if input.RequestID <= 0 || input.ReviewerID <= 0 || len(input.ReviewReason) > 500 || (!input.Approve && input.ReviewReason == "") {
		return ErrInviteRewardReviewInvalid
	}
	targetUserID := 0
	err := model.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		actor, err := model.LockInvitationUser(tx, input.ReviewerID)
		if err != nil || actor.Status != common.UserStatusEnabled || actor.Role < common.RoleAdminUser {
			return ErrInviteTargetForbidden
		}
		var requestSnapshot model.InviteRewardPayoutRequest
		if err := tx.Select("id", "user_id").First(&requestSnapshot, input.RequestID).Error; err != nil {
			return ErrInviteRewardReviewInvalid
		}
		target, err := model.LockInvitationUser(tx, requestSnapshot.UserID)
		if err != nil || !canReviewInvitationRewardTarget(actor, target) {
			return ErrInviteTargetForbidden
		}
		request, err := model.LockInviteRewardPayoutRequest(tx, input.RequestID)
		if err != nil || request.UserID != target.Id || request.Status != model.InviteRewardPayoutRequestStatusPending {
			return ErrInviteRewardReviewInvalid
		}
		targetUserID = target.Id
		items, err := model.LockInviteRewardPayoutRequestItems(tx, request.ID)
		if err != nil || len(items) == 0 {
			return ErrInviteRewardReviewInvalid
		}
		rewardIDs := make([]int64, 0, len(items))
		for _, item := range items {
			rewardIDs = append(rewardIDs, item.RewardID)
		}
		rewards, err := model.LockInviteRewardPayoutsByID(tx, rewardIDs)
		if err != nil || len(rewards) != len(rewardIDs) {
			return ErrInviteRewardReviewInvalid
		}
		var amount int64
		inviterRewardCount := 0
		var inviterRewardAmount int64
		for _, reward := range rewards {
			if reward.RecipientUserID != request.UserID || reward.Status != model.InviteRewardStatusPendingPayout || reward.Amount <= 0 {
				return ErrInviteRewardReviewInvalid
			}
			netAmount, err := invitationRewardNetAmount(reward)
			if err != nil {
				return ErrInviteRewardReviewInvalid
			}
			amount += netAmount
			if reward.RecipientKind == model.InviteRewardRecipientInviter && netAmount > 0 {
				inviterRewardAmount += netAmount
				if reward.TriggerType == model.InviteRewardTriggerRegistrationInviter {
					inviterRewardCount++
				}
			}
		}
		if amount <= 0 || amount > request.Amount {
			return ErrInviteRewardReviewInvalid
		}

		rewardStatus := model.InviteRewardStatusAvailable
		requestStatus := model.InviteRewardPayoutRequestStatusRejected
		if input.Approve {
			if _, err := AdjustWithTx(tx, request.UserID, amount, request.OperationID, "invitation_reward_payout", WalletReference{
				Type: "invite_reward_payout_request", ID: fmt.Sprint(request.ID), Context: "approved_invitation_rewards",
			}); err != nil {
				return err
			}
			rewardStatus = model.InviteRewardStatusPaid
			requestStatus = model.InviteRewardPayoutRequestStatusPaid
		}
		result := tx.Model(&model.InviteRewardPayout{}).
			Where("id IN ? AND status = ?", rewardIDs, model.InviteRewardStatusPendingPayout).
			Update("status", rewardStatus)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != int64(len(rewardIDs)) {
			return ErrInviteRewardReviewInvalid
		}
		now := common.GetTimestamp()
		for _, reward := range rewards {
			if reward.TriggerType == model.InviteRewardTriggerRegistrationInviter {
				if err := updateInvitationRewardUseStatusWithTx(tx, reward, rewardStatus); err != nil {
					return err
				}
				if input.Approve {
					if err := markInvitationUseRewardsCompletedWithTx(tx, reward.InviteCodeUseID, now); err != nil {
						return err
					}
				}
			}
		}
		if input.Approve && inviterRewardAmount > 0 {
			if err := tx.Model(&model.User{}).Where("id = ?", request.UserID).Updates(map[string]any{
				"aff_count":   gorm.Expr("aff_count + ?", inviterRewardCount),
				"aff_history": gorm.Expr("aff_history + ?", inviterRewardAmount),
			}).Error; err != nil {
				return err
			}
		}
		result = tx.Model(&model.InviteRewardPayoutRequest{}).
			Where("id = ? AND status = ?", request.ID, model.InviteRewardPayoutRequestStatusPending).
			Updates(map[string]any{
				"status": requestStatus, "pending_user_slot": nil, "reason": input.ReviewReason,
				"reviewer_id": actor.Id, "reviewed_at": now,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrInviteRewardReviewInvalid
		}
		return EnqueueWithTx(tx, AuditEvent{
			EventID: fmt.Sprintf("invitation:reward-payout-request:%d:reviewed", request.ID),
			Type:    "invitation.reward_payout_reviewed", Result: "success", ActorID: actor.Id,
			LogUserID: request.UserID, Target: fmt.Sprintf("invite-reward-payout-request:%d", request.ID),
			Params: map[string]any{
				"request_id": request.ID, "target_user_id": request.UserID, "reviewer_id": actor.Id,
				"status": requestStatus, "review_reason": input.ReviewReason, "amount": amount,
				"item_count": len(rewardIDs), "operation_id": request.OperationID,
			},
		})
	})
	if err != nil {
		return err
	}
	if targetUserID > 0 {
		if err := model.InvalidateUserCache(targetUserID); err != nil {
			common.SysError("failed to invalidate invitation payout user cache: " + err.Error())
		}
	}
	return nil
}

func FreezeInvitationRewardPayoutRequest(ctx context.Context, input InvitationRewardPayoutMutationInput) error {
	return mutateInvitationRewardPayoutRequest(ctx, input, true)
}

func UnfreezeInvitationRewardPayoutRequest(ctx context.Context, input InvitationRewardPayoutMutationInput) error {
	return mutateInvitationRewardPayoutRequest(ctx, input, false)
}

func mutateInvitationRewardPayoutRequest(ctx context.Context, input InvitationRewardPayoutMutationInput, freeze bool) error {
	input.Reason = strings.TrimSpace(input.Reason)
	if model.DB == nil || input.RequestID <= 0 || input.ActorID <= 0 || input.Reason == "" || len(input.Reason) > 500 {
		return ErrInviteRewardMutationInvalid
	}
	targetUserID := 0
	err := model.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		actor, err := model.LockInvitationUser(tx, input.ActorID)
		if err != nil || actor.Status != common.UserStatusEnabled || actor.Role < common.RoleAdminUser {
			return ErrInviteTargetForbidden
		}
		var requestSnapshot model.InviteRewardPayoutRequest
		if err := tx.Select("id", "user_id").First(&requestSnapshot, input.RequestID).Error; err != nil {
			return ErrInviteRewardMutationInvalid
		}
		target, err := model.LockInvitationUser(tx, requestSnapshot.UserID)
		if err != nil || !canReviewInvitationRewardTarget(actor, target) {
			return ErrInviteTargetForbidden
		}
		request, err := model.LockInviteRewardPayoutRequest(tx, input.RequestID)
		if err != nil || request.UserID != target.Id {
			return ErrInviteRewardMutationInvalid
		}
		targetUserID = target.Id

		expectedRequestStatus := model.InviteRewardPayoutRequestStatusFrozen
		expectedRewardStatus := model.InviteRewardStatusFrozen
		nextRequestStatus := model.InviteRewardPayoutRequestStatusPaid
		nextRewardStatus := model.InviteRewardStatusPaid
		action := model.InviteRewardPayoutActionUnfreeze
		category := "invitation_reward_unfreeze"
		revision := request.FreezeRevision
		var netAmount int64
		delta := int64(0)
		eventType := "invitation.reward_payout_unfrozen"
		if freeze {
			expectedRequestStatus = model.InviteRewardPayoutRequestStatusPaid
			expectedRewardStatus = model.InviteRewardStatusPaid
			nextRequestStatus = model.InviteRewardPayoutRequestStatusFrozen
			nextRewardStatus = model.InviteRewardStatusFrozen
			action = model.InviteRewardPayoutActionFreeze
			category = "invitation_reward_freeze"
			revision++
			eventType = "invitation.reward_payout_frozen"
		}
		if request.Status != expectedRequestStatus || request.Amount <= 0 || revision <= 0 {
			return ErrInviteRewardMutationInvalid
		}

		items, err := model.LockInviteRewardPayoutRequestItems(tx, request.ID)
		if err != nil || len(items) == 0 {
			return ErrInviteRewardMutationInvalid
		}
		rewardIDs := make([]int64, 0, len(items))
		for _, item := range items {
			rewardIDs = append(rewardIDs, item.RewardID)
		}
		rewards, err := model.LockInviteRewardPayoutsByID(tx, rewardIDs)
		if err != nil || len(rewards) != len(rewardIDs) {
			return ErrInviteRewardMutationInvalid
		}
		for _, reward := range rewards {
			if reward.RecipientUserID != request.UserID || reward.Status != expectedRewardStatus || reward.Amount <= 0 {
				return ErrInviteRewardMutationInvalid
			}
			rewardNet, err := invitationRewardNetAmount(reward)
			if err != nil {
				return ErrInviteRewardMutationInvalid
			}
			netAmount += rewardNet
		}
		if netAmount > request.Amount || (freeze && netAmount <= 0) {
			return ErrInviteRewardMutationInvalid
		}
		delta = netAmount
		if freeze {
			delta = -netAmount
		}

		operationID := fmt.Sprintf("invite-reward-%s:%d:%d", action, request.ID, revision)
		reference := WalletReference{
			Type: "invite_reward_payout_request", ID: fmt.Sprint(request.ID), Context: fmt.Sprintf("%s:%d", action, revision),
		}
		if delta == 0 {
			// A fully reversed frozen request changes state without restoring wallet value.
		} else if freeze {
			if _, err := adjustAllowDebtWithTx(tx, request.UserID, delta, operationID, category, reference); err != nil {
				return err
			}
		} else if _, err := AdjustWithTx(tx, request.UserID, delta, operationID, category, reference); err != nil {
			return err
		}

		result := tx.Model(&model.InviteRewardPayout{}).
			Where("id IN ? AND status = ?", rewardIDs, expectedRewardStatus).
			Update("status", nextRewardStatus)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != int64(len(rewardIDs)) {
			return ErrInviteRewardMutationInvalid
		}
		for _, reward := range rewards {
			if reward.TriggerType == model.InviteRewardTriggerRegistrationInviter {
				if err := updateInvitationRewardUseStatusWithTx(tx, reward, nextRewardStatus); err != nil {
					return err
				}
			}
		}

		requestUpdates := map[string]any{"status": nextRequestStatus}
		if freeze {
			requestUpdates["freeze_revision"] = revision
		}
		result = tx.Model(&model.InviteRewardPayoutRequest{}).
			Where("id = ? AND status = ? AND freeze_revision = ?", request.ID, expectedRequestStatus, request.FreezeRevision).
			Updates(requestUpdates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrInviteRewardMutationInvalid
		}
		actionRecord := model.InviteRewardPayoutAction{
			RequestID: request.ID, Action: action, Revision: revision, ActorID: actor.Id,
			Reason: input.Reason, Amount: netAmount, WalletOperationID: operationID,
		}
		if err := tx.Create(&actionRecord).Error; err != nil {
			return ErrInviteRewardMutationInvalid
		}
		return EnqueueWithTx(tx, AuditEvent{
			EventID: fmt.Sprintf("invitation:reward-payout-request:%d:%s:%d", request.ID, action, revision),
			Type:    eventType, Result: "success", ActorID: actor.Id, LogUserID: request.UserID,
			Target: fmt.Sprintf("invite-reward-payout-request:%d", request.ID),
			Params: map[string]any{
				"request_id": request.ID, "target_user_id": request.UserID, "action": action,
				"reason": input.Reason, "amount": netAmount, "revision": revision,
				"item_count": len(rewardIDs), "operation_id": operationID,
			},
		})
	})
	if err != nil {
		return err
	}
	if targetUserID > 0 {
		if err := model.InvalidateUserCache(targetUserID); err != nil {
			common.SysError("failed to invalidate invitation payout user cache: " + err.Error())
		}
	}
	return nil
}

func invitationRewardNetAmount(reward model.InviteRewardPayout) (int64, error) {
	if reward.Amount <= 0 || reward.ReversedAmount < 0 || reward.ReversedAmount > reward.Amount {
		return 0, ErrInviteRewardUnavailable
	}
	return reward.Amount - reward.ReversedAmount, nil
}

func canReviewInvitationRewardTarget(actor, target model.User) bool {
	if actor.Id == 1 {
		return true
	}
	if actor.Role >= common.RoleRootUser {
		return target.Role < common.RoleRootUser
	}
	return target.Role < common.RoleAdminUser
}

func updateInvitationRewardUseStatusWithTx(tx *gorm.DB, reward model.InviteRewardPayout, status string) error {
	column, err := invitationRewardUseStatusColumn(reward.RecipientKind)
	if err != nil {
		return err
	}
	return tx.Model(&model.InviteCodeUse{}).Where("id = ?", reward.InviteCodeUseID).Update(column, status).Error
}

func markInvitationUseRewardsCompletedWithTx(tx *gorm.DB, useID, completedAt int64) error {
	terminal := []string{model.InviteRewardStatusPaid, model.InviteRewardStatusRejected, model.InviteRewardStatusSkipped}
	return tx.Model(&model.InviteCodeUse{}).
		Where("id = ? AND inviter_reward_status IN ? AND invitee_reward_status IN ?", useID, terminal, terminal).
		Update("rewards_completed_at", completedAt).Error
}

func invitationRewardUseStatusColumn(recipientKind string) (string, error) {
	switch recipientKind {
	case model.InviteRewardRecipientInviter:
		return "inviter_reward_status", nil
	case model.InviteRewardRecipientInvitee:
		return "invitee_reward_status", nil
	default:
		return "", ErrInviteRewardReviewInvalid
	}
}

func GetInvitationRewardSummary(ctx context.Context, userID int) (InvitationRewardSummary, error) {
	summary := InvitationRewardSummary{}
	if model.DB == nil || userID <= 0 {
		return summary, ErrInviteRewardUnavailable
	}
	if err := model.DB.WithContext(ctx).Model(&model.InviteCodeUse{}).
		Where("inviter_user_id = ?", userID).
		Distinct("registered_user_id").
		Count(&summary.InvitedCount).Error; err != nil {
		return summary, err
	}
	policy, err := operation_setting.GetInvitationIncentivePublicPolicy()
	if err != nil {
		return summary, err
	}
	rateBasisPoints, normalizedRate, err := operation_setting.ParseInvitationCommissionRate(policy.CashCommissionRate)
	if err != nil {
		return summary, err
	}
	summary.CashCommissionRate = normalizedRate
	summary.CashCommissionVisible = policy.Enabled && policy.CashCommissionEnabled && rateBasisPoints > 0
	rows := make([]struct {
		Status string `gorm:"column:status"`
		Amount int64  `gorm:"column:amount"`
	}, 0)
	if err := model.DB.WithContext(ctx).Model(&model.InviteRewardPayout{}).
		Select("status, COALESCE(SUM(CASE WHEN amount >= reversed_amount THEN amount - reversed_amount ELSE 0 END), 0) AS amount").
		Where("recipient_user_id = ? AND recipient_kind = ?", userID, model.InviteRewardRecipientInviter).
		Group("status").Scan(&rows).Error; err != nil {
		return summary, err
	}
	for _, row := range rows {
		switch row.Status {
		case model.InviteRewardStatusAvailable:
			summary.Available = row.Amount
		case model.InviteRewardStatusPendingPayout:
			summary.PendingPayout = row.Amount
		case model.InviteRewardStatusPaid:
			summary.Paid = row.Amount
		case model.InviteRewardStatusFrozen:
			summary.Frozen = row.Amount
		}
	}
	summary.TotalEarned = summary.Available + summary.PendingPayout + summary.Paid + summary.Frozen
	return summary, nil
}

func ListInvitationRewardPayoutRequests(ctx context.Context, query InvitationAdminListQuery) (InvitationRewardPayoutRequestListResult, error) {
	query.normalize()
	result := InvitationRewardPayoutRequestListResult{
		Items: make([]InvitationRewardPayoutRequestListItem, 0), Page: query.Page, PageSize: query.PageSize,
	}
	actor, err := invitationListActor(ctx, query.ActorID)
	if err != nil {
		return result, err
	}
	itemCounts := invitationRewardRequestItemAggregates(model.DB.WithContext(ctx))
	db := model.DB.WithContext(ctx).Table("invite_reward_payout_requests AS requests").
		Joins("JOIN users AS request_user ON request_user.id = requests.user_id").
		Joins("LEFT JOIN users AS reviewer ON reviewer.id = requests.reviewer_id").
		Joins("LEFT JOIN (?) AS item_counts ON item_counts.request_id = requests.id", itemCounts)
	db = invitationActorScope(db, actor, "request_user.role")
	if query.Status == "history" {
		db = db.Where("requests.status IN ?", []string{
			model.InviteRewardPayoutRequestStatusPaid,
			model.InviteRewardPayoutRequestStatusRejected,
			model.InviteRewardPayoutRequestStatusFrozen,
			model.InviteRewardPayoutRequestStatusCancelled,
		})
	} else if query.Status != "" {
		db = db.Where("requests.status = ?", query.Status)
	}
	if query.Keyword != "" {
		pattern := "%" + strings.ToLower(query.Keyword) + "%"
		db = db.Where("LOWER(request_user.username) LIKE ? OR LOWER(request_user.display_name) LIKE ? OR LOWER(requests.operation_id) LIKE ? OR LOWER(requests.reason) LIKE ?", pattern, pattern, pattern, pattern)
	}
	if err := db.Count(&result.Total).Error; err != nil {
		return result, err
	}
	offset, pastEnd := invitationListOffset(query.Page, query.PageSize, result.Total)
	if pastEnd {
		return result, nil
	}
	err = db.Select("requests.*, request_user.username AS username, request_user.display_name AS display_name, request_user.role AS user_role, COALESCE(reviewer.username, '') AS reviewer_username, COALESCE(item_counts.item_count, 0) AS item_count, COALESCE(item_counts.reversed_amount, 0) AS reversed_amount, COALESCE(item_counts.net_amount, 0) AS net_amount").
		Order("CASE WHEN requests.status = 'pending' THEN 0 ELSE 1 END ASC").
		Order("requests.id DESC").Limit(query.PageSize).Offset(offset).Scan(&result.Items).Error
	return result, err
}

func GetInvitationRewardPayoutRequestDetail(ctx context.Context, actorID int, requestID int64) (InvitationRewardPayoutRequestDetail, error) {
	detail := InvitationRewardPayoutRequestDetail{
		Rewards: make([]InvitationRewardPayoutSource, 0),
		Actions: make([]InvitationRewardPayoutActionListItem, 0),
	}
	if model.DB == nil || requestID <= 0 {
		return detail, ErrInviteRewardMutationInvalid
	}
	actor, err := invitationListActor(ctx, actorID)
	if err != nil {
		return detail, err
	}
	itemCounts := invitationRewardRequestItemAggregates(model.DB.WithContext(ctx))
	requestQuery := model.DB.WithContext(ctx).Table("invite_reward_payout_requests AS requests").
		Joins("JOIN users AS request_user ON request_user.id = requests.user_id").
		Joins("LEFT JOIN users AS reviewer ON reviewer.id = requests.reviewer_id").
		Joins("LEFT JOIN (?) AS item_counts ON item_counts.request_id = requests.id", itemCounts).
		Where("requests.id = ?", requestID)
	requestQuery = invitationActorScope(requestQuery, actor, "request_user.role")
	result := requestQuery.Select("requests.*, request_user.username AS username, request_user.display_name AS display_name, request_user.role AS user_role, COALESCE(reviewer.username, '') AS reviewer_username, COALESCE(item_counts.item_count, 0) AS item_count, COALESCE(item_counts.reversed_amount, 0) AS reversed_amount, COALESCE(item_counts.net_amount, 0) AS net_amount").
		Scan(&detail.Request)
	if result.Error != nil {
		return detail, result.Error
	}
	if result.RowsAffected != 1 {
		return detail, ErrInviteTargetForbidden
	}
	if err := model.DB.WithContext(ctx).Table("invite_reward_payout_request_items AS items").
		Joins("JOIN invite_reward_payouts AS rewards ON rewards.id = items.reward_id").
		Joins("JOIN invite_code_uses AS uses ON uses.id = rewards.invite_code_use_id").
		Joins("JOIN users AS source_user ON source_user.id = uses.registered_user_id").
		Joins("LEFT JOIN invite_codes AS codes ON codes.id = uses.invite_code_id").
		Where("items.request_id = ?", requestID).
		Select("rewards.id AS reward_id, rewards.invite_code_use_id, rewards.trigger_type, rewards.trigger_id, rewards.eligible_quota, rewards.rate_basis_points, rewards.amount, rewards.reversed_amount, (rewards.amount - rewards.reversed_amount) AS net_amount, rewards.reversal_state, rewards.status, uses.registered_user_id AS source_user_id, source_user.username AS source_username, source_user.display_name AS source_display_name, uses.registration_ip, uses.registered_at, COALESCE(codes.code, '') AS invite_code").
		Order("rewards.id ASC").Scan(&detail.Rewards).Error; err != nil {
		return detail, err
	}
	if err := model.DB.WithContext(ctx).Table("invite_reward_payout_actions AS actions").
		Joins("JOIN users AS actor ON actor.id = actions.actor_id").
		Where("actions.request_id = ?", requestID).
		Select("actions.*, actor.username AS actor_username, actor.display_name AS actor_display_name").
		Order("actions.id ASC").Scan(&detail.Actions).Error; err != nil {
		return detail, err
	}
	return detail, nil
}
