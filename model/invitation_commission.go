package model

import (
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrInvitationCommissionInvalid  = errors.New("invitation commission input is invalid")
	ErrInvitationCommissionConflict = errors.New("invitation commission source conflicts with an existing reward")
)

type InvitationCommissionInput struct {
	InviteeUserID   int
	TriggerType     string
	TriggerID       string
	EligibleQuota   int64
	RateBasisPoints int64
}

type InvitationCommissionReversalInput struct {
	TriggerType      string
	TriggerID        string
	RefundSourceType string
	RefundSourceID   string
	RefundedBase     int64
}

func CreateInvitationCommissionWithTx(tx *gorm.DB, input InvitationCommissionInput) (*InviteRewardPayout, bool, error) {
	if tx == nil || input.InviteeUserID <= 0 || input.EligibleQuota < 0 || input.RateBasisPoints < 0 {
		return nil, false, ErrInvitationCommissionInvalid
	}
	input.TriggerID = strings.TrimSpace(input.TriggerID)
	if input.TriggerID == "" || len(input.TriggerID) > 191 {
		return nil, false, ErrInvitationCommissionInvalid
	}

	setting := operation_setting.GetInvitationIncentiveSetting()
	enabled := false
	switch input.TriggerType {
	case InviteRewardTriggerCashTopUp, InviteRewardTriggerCashSubscription:
		enabled = setting.CashCommissionEnabled
	case InviteRewardTriggerAdminBalanceRedemption:
		enabled = setting.AdminBalanceRedemptionCommissionEnabled
	case InviteRewardTriggerAdminSubscriptionClaim:
		enabled = setting.AdminSubscriptionRedemptionCommissionEnabled
	default:
		return nil, false, ErrInvitationCommissionInvalid
	}
	if !setting.Enabled || !enabled || input.EligibleQuota == 0 || input.RateBasisPoints == 0 {
		return nil, false, nil
	}

	invitee, err := LockInvitationUser(tx, input.InviteeUserID)
	if err != nil {
		return nil, false, err
	}
	if invitee.InviterId <= 0 || invitee.InviterSource != InviterSourceInviteCode {
		return nil, false, nil
	}
	var inviteUse InviteCodeUse
	err = lockForUpdate(tx).
		Where("registered_user_id = ? AND inviter_user_id = ?", invitee.Id, invitee.InviterId).
		Order("id ASC").First(&inviteUse).Error
	if err != nil {
		return nil, false, err
	}

	product := new(big.Int).Mul(big.NewInt(input.EligibleQuota), big.NewInt(input.RateBasisPoints))
	amountValue := new(big.Int).Quo(product, big.NewInt(10_000))
	if !amountValue.IsInt64() {
		return nil, false, ErrInvitationCommissionInvalid
	}
	amount := amountValue.Int64()
	if amount <= 0 {
		return nil, false, nil
	}

	reward := InviteRewardPayout{
		InviteCodeUseID: inviteUse.ID,
		TriggerType:     input.TriggerType,
		TriggerID:       input.TriggerID,
		RecipientKind:   InviteRewardRecipientInviter,
		RecipientUserID: invitee.InviterId,
		EligibleQuota:   input.EligibleQuota,
		RateBasisPoints: input.RateBasisPoints,
		Amount:          amount,
		ReversalState:   InviteRewardReversalNone,
		Status:          InviteRewardStatusAvailable,
	}
	result := tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "trigger_type"}, {Name: "trigger_id"}, {Name: "recipient_kind"},
		},
		DoNothing: true,
	}).Create(&reward)
	if result.Error != nil {
		return nil, false, result.Error
	}
	created := result.RowsAffected == 1
	if !created {
		if err := tx.Where(
			"trigger_type = ? AND trigger_id = ? AND recipient_kind = ?",
			input.TriggerType, input.TriggerID, InviteRewardRecipientInviter,
		).First(&reward).Error; err != nil {
			return nil, false, err
		}
		if reward.InviteCodeUseID != inviteUse.ID || reward.RecipientUserID != invitee.InviterId ||
			reward.EligibleQuota != input.EligibleQuota || reward.RateBasisPoints != input.RateBasisPoints || reward.Amount != amount {
			return nil, false, ErrInvitationCommissionConflict
		}
		return &reward, false, nil
	}

	payload, err := common.Marshal(map[string]any{
		"trigger_type":      input.TriggerType,
		"eligible_quota":    strconv.FormatInt(input.EligibleQuota, 10),
		"rate_basis_points": strconv.FormatInt(input.RateBasisPoints, 10),
		"amount":            strconv.FormatInt(amount, 10),
	})
	if err != nil {
		return nil, false, err
	}
	event := AuditOutboxEvent{
		EventID:            "invitation:commission:" + strconv.FormatInt(reward.ID, 10),
		EventType:          "invitation.commission_created",
		Result:             "success",
		LogUserID:          invitee.InviterId,
		Target:             fmt.Sprintf("user:%d", invitee.InviterId),
		Payload:            string(payload),
		Status:             AuditOutboxStatusPending,
		LogSnapshotVersion: 1,
	}
	if err := tx.Create(&event).Error; err != nil {
		return nil, false, err
	}
	return &reward, true, nil
}

func ReverseInvitationCommissionWithTx(tx *gorm.DB, input InvitationCommissionReversalInput) (*InviteRewardReversal, bool, error) {
	input.TriggerType = strings.TrimSpace(input.TriggerType)
	input.TriggerID = strings.TrimSpace(input.TriggerID)
	input.RefundSourceType = strings.TrimSpace(input.RefundSourceType)
	input.RefundSourceID = strings.TrimSpace(input.RefundSourceID)
	if tx == nil || input.RefundedBase <= 0 || input.TriggerID == "" || len(input.TriggerID) > 191 ||
		input.RefundSourceType == "" || len(input.RefundSourceType) > 32 ||
		input.RefundSourceID == "" || len(input.RefundSourceID) > 191 {
		return nil, false, ErrInvitationCommissionInvalid
	}
	if input.TriggerType != InviteRewardTriggerCashTopUp && input.TriggerType != InviteRewardTriggerCashSubscription {
		return nil, false, ErrInvitationCommissionInvalid
	}

	var sourceReward InviteRewardPayout
	if err := tx.Where(
		"trigger_type = ? AND trigger_id = ? AND recipient_kind = ?",
		input.TriggerType, input.TriggerID, InviteRewardRecipientInviter,
	).First(&sourceReward).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, false, nil
		}
		return nil, false, err
	}
	if _, err := LockInvitationUser(tx, sourceReward.RecipientUserID); err != nil {
		return nil, false, err
	}
	if err := tx.First(&sourceReward, sourceReward.ID).Error; err != nil {
		return nil, false, err
	}

	request, rewards, err := lockInvitationCommissionReversalRewards(tx, sourceReward)
	if err != nil {
		return nil, false, err
	}
	reward := sourceReward
	rewardFound := false
	for _, candidate := range rewards {
		if candidate.ID == sourceReward.ID {
			reward = candidate
			rewardFound = true
			break
		}
	}
	if !rewardFound || reward.Amount <= 0 || reward.ReversedAmount < 0 || reward.ReversedAmount > reward.Amount || reward.RateBasisPoints < 0 {
		return nil, false, ErrInvitationCommissionConflict
	}
	switch reward.Status {
	case InviteRewardStatusAvailable, InviteRewardStatusPendingPayout, InviteRewardStatusPaid, InviteRewardStatusFrozen:
	default:
		return nil, false, ErrInvitationCommissionConflict
	}

	existingReversals := make([]InviteRewardReversal, 0)
	if err := lockForUpdate(tx).Where("reward_id = ?", reward.ID).Order("id ASC").Find(&existingReversals).Error; err != nil {
		return nil, false, err
	}
	totalRefundedBase := big.NewInt(input.RefundedBase)
	for i := range existingReversals {
		existing := &existingReversals[i]
		if existing.RefundSourceType == input.RefundSourceType && existing.RefundSourceID == input.RefundSourceID {
			if existing.RefundedBase != input.RefundedBase {
				return nil, false, ErrInvitationCommissionConflict
			}
			return existing, false, nil
		}
		if existing.RefundedBase <= 0 {
			return nil, false, ErrInvitationCommissionConflict
		}
		totalRefundedBase.Add(totalRefundedBase, big.NewInt(existing.RefundedBase))
	}
	targetValue := new(big.Int).Quo(
		new(big.Int).Mul(totalRefundedBase, big.NewInt(reward.RateBasisPoints)),
		big.NewInt(10_000),
	)
	if targetValue.Cmp(big.NewInt(reward.Amount)) > 0 {
		targetValue.SetInt64(reward.Amount)
	}
	if !targetValue.IsInt64() || targetValue.Int64() < reward.ReversedAmount {
		return nil, false, ErrInvitationCommissionConflict
	}
	cumulative := targetValue.Int64()
	reversalAmount := cumulative - reward.ReversedAmount
	walletOperationID := ""
	if reward.Status == InviteRewardStatusPaid && reversalAmount > 0 {
		walletOperationID = invitationCommissionReversalOperationID(reward.ID, input.RefundSourceType, input.RefundSourceID)
	}
	reversal := InviteRewardReversal{
		RewardID: reward.ID, RefundSourceType: input.RefundSourceType, RefundSourceID: input.RefundSourceID,
		RefundedBase: input.RefundedBase, Amount: reversalAmount, CumulativeReversedAmount: cumulative,
		RewardStatusBefore: reward.Status, WalletOperationID: walletOperationID,
	}
	result := tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "reward_id"}, {Name: "refund_source_type"}, {Name: "refund_source_id"}},
		DoNothing: true,
	}).Create(&reversal)
	if result.Error != nil {
		return nil, false, result.Error
	}
	if result.RowsAffected == 0 {
		if err := tx.Where(
			"reward_id = ? AND refund_source_type = ? AND refund_source_id = ?",
			reward.ID, input.RefundSourceType, input.RefundSourceID,
		).First(&reversal).Error; err != nil {
			return nil, false, err
		}
		if reversal.RefundedBase != input.RefundedBase {
			return nil, false, ErrInvitationCommissionConflict
		}
		return &reversal, false, nil
	}

	if reversalAmount > 0 && reward.Status == InviteRewardStatusPaid {
		if _, err := AdjustWalletAllowDebtWithTx(tx, reward.RecipientUserID, -reversalAmount, walletOperationID,
			"invitation_commission_reversal", WalletAdjustReference{
				Type: "invite_reward", ID: strconv.FormatInt(reward.ID, 10), Context: "refund:" + input.RefundSourceType,
			}); err != nil {
			return nil, false, err
		}
	}

	reversalState := InviteRewardReversalNone
	if cumulative > 0 {
		reversalState = InviteRewardReversalPartial
	}
	if cumulative >= reward.Amount {
		reversalState = InviteRewardReversalFull
	}
	update := tx.Model(&InviteRewardPayout{}).
		Where("id = ? AND reversed_amount = ?", reward.ID, reward.ReversedAmount).
		Updates(map[string]any{"reversed_amount": cumulative, "reversal_state": reversalState})
	if update.Error != nil {
		return nil, false, update.Error
	}
	if update.RowsAffected != 1 {
		return nil, false, ErrInvitationCommissionConflict
	}

	if request != nil && request.Status == InviteRewardPayoutRequestStatusPending {
		netAmount, err := invitationCommissionRequestNet(rewards, reward.ID, cumulative)
		if err != nil {
			return nil, false, err
		}
		if netAmount == 0 {
			update = tx.Model(&InviteRewardPayoutRequest{}).
				Where("id = ? AND status = ?", request.ID, InviteRewardPayoutRequestStatusPending).
				Updates(map[string]any{
					"status": InviteRewardPayoutRequestStatusCancelled, "pending_user_slot": nil,
					"reason": "commission_refunded", "reviewed_at": common.GetTimestamp(),
				})
			if update.Error != nil {
				return nil, false, update.Error
			}
			if update.RowsAffected != 1 {
				return nil, false, ErrInvitationCommissionConflict
			}
			if err := tx.Model(&InviteRewardPayout{}).
				Where("id IN ? AND status = ?", invitationCommissionRewardIDs(rewards), InviteRewardStatusPendingPayout).
				Update("status", InviteRewardStatusAvailable).Error; err != nil {
				return nil, false, err
			}
		}
	}

	if reversalAmount > 0 {
		payload, err := common.Marshal(map[string]any{
			"trigger_type":      reward.TriggerType,
			"rate_basis_points": strconv.FormatInt(reward.RateBasisPoints, 10),
			"amount":            strconv.FormatInt(reversalAmount, 10),
		})
		if err != nil {
			return nil, false, err
		}
		if err := tx.Create(&AuditOutboxEvent{
			EventID:   "invitation:commission-reversal:" + strconv.FormatInt(reversal.ID, 10),
			EventType: "invitation.commission_reversed", Result: "success",
			LogUserID: reward.RecipientUserID, Target: fmt.Sprintf("user:%d", reward.RecipientUserID),
			Payload: string(payload), Status: AuditOutboxStatusPending, LogSnapshotVersion: 1,
		}).Error; err != nil {
			return nil, false, err
		}
	}
	return &reversal, true, nil
}

func lockInvitationCommissionReversalRewards(tx *gorm.DB, sourceReward InviteRewardPayout) (*InviteRewardPayoutRequest, []InviteRewardPayout, error) {
	expectedRequestStatus := ""
	switch sourceReward.Status {
	case InviteRewardStatusPendingPayout:
		expectedRequestStatus = InviteRewardPayoutRequestStatusPending
	case InviteRewardStatusPaid:
		expectedRequestStatus = InviteRewardPayoutRequestStatusPaid
	case InviteRewardStatusFrozen:
		expectedRequestStatus = InviteRewardPayoutRequestStatusFrozen
	}
	if expectedRequestStatus == "" {
		rewards, err := LockInviteRewardPayoutsByID(tx, []int64{sourceReward.ID})
		return nil, rewards, err
	}

	var requestID int64
	result := tx.Table("invite_reward_payout_requests AS requests").
		Select("requests.id").
		Joins("JOIN invite_reward_payout_request_items AS items ON items.request_id = requests.id").
		Where("items.reward_id = ? AND requests.status = ?", sourceReward.ID, expectedRequestStatus).
		Order("requests.id DESC").Limit(1).Scan(&requestID)
	if result.Error != nil {
		return nil, nil, result.Error
	}
	if result.RowsAffected == 0 {
		if sourceReward.Status == InviteRewardStatusPaid {
			rewards, err := LockInviteRewardPayoutsByID(tx, []int64{sourceReward.ID})
			return nil, rewards, err
		}
		return nil, nil, ErrInvitationCommissionConflict
	}
	request, err := LockInviteRewardPayoutRequest(tx, requestID)
	if err != nil {
		return nil, nil, err
	}
	items, err := LockInviteRewardPayoutRequestItems(tx, request.ID)
	if err != nil || len(items) == 0 {
		return nil, nil, ErrInvitationCommissionConflict
	}
	rewardIDs := make([]int64, 0, len(items))
	for _, item := range items {
		rewardIDs = append(rewardIDs, item.RewardID)
	}
	rewards, err := LockInviteRewardPayoutsByID(tx, rewardIDs)
	if err != nil || len(rewards) != len(rewardIDs) {
		return nil, nil, ErrInvitationCommissionConflict
	}
	return &request, rewards, nil
}

func invitationCommissionRequestNet(rewards []InviteRewardPayout, updatedRewardID, updatedReversedAmount int64) (int64, error) {
	var net int64
	for _, reward := range rewards {
		reversedAmount := reward.ReversedAmount
		if reward.ID == updatedRewardID {
			reversedAmount = updatedReversedAmount
		}
		if reward.Amount < 0 || reversedAmount < 0 || reversedAmount > reward.Amount {
			return 0, ErrInvitationCommissionConflict
		}
		itemNet := reward.Amount - reversedAmount
		if itemNet > 0 && net > int64(^uint64(0)>>1)-itemNet {
			return 0, ErrInvitationCommissionInvalid
		}
		net += itemNet
	}
	return net, nil
}

func invitationCommissionRewardIDs(rewards []InviteRewardPayout) []int64 {
	ids := make([]int64, 0, len(rewards))
	for _, reward := range rewards {
		ids = append(ids, reward.ID)
	}
	return ids
}

func invitationCommissionReversalOperationID(rewardID int64, sourceType, sourceID string) string {
	operationID := fmt.Sprintf("invitation-commission-reversal:%d:%s:%s", rewardID, sourceType, sourceID)
	if len(operationID) <= 191 {
		return operationID
	}
	return fmt.Sprintf("invitation-commission-reversal:%d:sha1:%s", rewardID, common.Sha1([]byte(sourceType+"\x00"+sourceID)))
}
