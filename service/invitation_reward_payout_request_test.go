package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func enableInvitationPayoutCompliance(t *testing.T) {
	t.Helper()
	payment := operation_setting.GetPaymentSetting()
	previous := *payment
	payment.ComplianceConfirmed = true
	payment.ComplianceTermsVersion = operation_setting.CurrentComplianceTermsVersion
	t.Cleanup(func() { *payment = previous })
}

func seedAvailableInvitationReward(t *testing.T, userID, inviteeID, amount int, suffix string) model.InviteRewardPayout {
	t.Helper()
	use := model.InviteCodeUse{
		InviteCodeID: 1, InviterUserID: userID, RegisteredUserID: inviteeID,
		RegisteredAt: time.Now().Unix(), Source: "password", InviterReward: amount,
		InviterRewardStatus: model.InviteRewardStatusAvailable,
		InviteeRewardStatus: model.InviteRewardStatusSkipped,
		AuditEventID:        "payout-request-use-" + suffix,
	}
	require.NoError(t, model.DB.Create(&use).Error)
	reward := model.InviteRewardPayout{
		InviteCodeUseID: use.ID, TriggerType: model.InviteRewardTriggerRegistrationInviter,
		TriggerID: fmt.Sprint(use.ID), RecipientKind: model.InviteRewardRecipientInviter,
		RecipientUserID: userID, EligibleQuota: int64(amount), Amount: int64(amount),
		ReversalState: model.InviteRewardReversalNone, Status: model.InviteRewardStatusAvailable,
	}
	require.NoError(t, model.DB.Create(&reward).Error)
	return reward
}

func seedCashInvitationReward(t *testing.T, userID, inviteeID int, suffix string) model.InviteRewardPayout {
	t.Helper()
	reward := seedAvailableInvitationReward(t, userID, inviteeID, 100, suffix)
	require.NoError(t, model.DB.Model(&reward).Updates(map[string]any{
		"trigger_type":      model.InviteRewardTriggerCashTopUp,
		"trigger_id":        "cash-" + suffix,
		"eligible_quota":    int64(1_000),
		"rate_basis_points": int64(1_000),
	}).Error)
	reward.TriggerType = model.InviteRewardTriggerCashTopUp
	reward.TriggerID = "cash-" + suffix
	reward.EligibleQuota = 1_000
	reward.RateBasisPoints = 1_000
	return reward
}

func reverseCashInvitationReward(t *testing.T, reward model.InviteRewardPayout, sourceID string, refundedBase int64) {
	t.Helper()
	require.NoError(t, model.DB.Transaction(func(tx *gorm.DB) error {
		_, _, err := model.ReverseInvitationCommissionWithTx(tx, model.InvitationCommissionReversalInput{
			TriggerType: reward.TriggerType, TriggerID: reward.TriggerID,
			RefundSourceType: "cash_refund", RefundSourceID: sourceID, RefundedBase: refundedBase,
		})
		return err
	}))
}

func TestInvitationRewardRefundApprovalCreditsOnlyPendingNetAmount(t *testing.T) {
	db := setupInvitationServiceTest(t)
	enableInvitationPayoutCompliance(t)
	seedInvitationUser(t, db, 1, "primary-root", common.RoleRootUser)
	seedInvitationUser(t, db, 10, "inviter", common.RoleCommonUser)
	seedInvitationUser(t, db, 20, "invitee", common.RoleCommonUser)
	reward := seedCashInvitationReward(t, 10, 20, "pending-refund")
	request, err := SubmitInvitationRewardPayout(context.Background(), 10)
	require.NoError(t, err)

	reverseCashInvitationReward(t, reward, "pending-refund-1", 400)
	require.NoError(t, db.First(request, request.ID).Error)
	require.Equal(t, int64(100), request.Amount)
	require.Equal(t, model.InviteRewardPayoutRequestStatusPending, request.Status)
	require.NoError(t, ReviewInvitationRewardPayoutRequest(context.Background(), ReviewInvitationRewardPayoutRequestInput{
		RequestID: request.ID, ReviewerID: 1, Approve: true, ReviewReason: "approve current net",
	}))

	var inviter model.User
	require.NoError(t, db.First(&inviter, 10).Error)
	require.Equal(t, int64(60), inviter.Quota)
	var payoutLedger model.WalletLedgerEntry
	require.NoError(t, db.Where("operation_id = ?", request.OperationID).First(&payoutLedger).Error)
	require.Equal(t, int64(60), payoutLedger.Delta)
}

func TestInvitationRewardRefundBeforeSubmitUsesNetAmountThroughoutReview(t *testing.T) {
	db := setupInvitationServiceTest(t)
	enableInvitationPayoutCompliance(t)
	seedInvitationUser(t, db, 1, "primary-root", common.RoleRootUser)
	seedInvitationUser(t, db, 10, "inviter", common.RoleCommonUser)
	seedInvitationUser(t, db, 20, "invitee", common.RoleCommonUser)
	reward := seedCashInvitationReward(t, 10, 20, "pre-submit-refund")

	reverseCashInvitationReward(t, reward, "pre-submit-refund-1", 400)
	request, err := SubmitInvitationRewardPayout(context.Background(), 10)
	require.NoError(t, err)
	require.Equal(t, int64(60), request.Amount)
	require.NoError(t, ReviewInvitationRewardPayoutRequest(context.Background(), ReviewInvitationRewardPayoutRequestInput{
		RequestID: request.ID, ReviewerID: 1, Approve: true, ReviewReason: "approve net commission",
	}))
	require.NoError(t, FreezeInvitationRewardPayoutRequest(context.Background(), InvitationRewardPayoutMutationInput{
		RequestID: request.ID, ActorID: 1, Reason: "verify refunded payout",
	}))
	require.NoError(t, UnfreezeInvitationRewardPayoutRequest(context.Background(), InvitationRewardPayoutMutationInput{
		RequestID: request.ID, ActorID: 1, Reason: "verification complete",
	}))

	var inviter model.User
	require.NoError(t, db.First(&inviter, 10).Error)
	require.Equal(t, int64(60), inviter.Quota)
}

func TestCashInvitationRewardPreservesRegistrationMetricsAndLegacyStatus(t *testing.T) {
	db := setupInvitationServiceTest(t)
	enableInvitationPayoutCompliance(t)
	seedInvitationUser(t, db, 1, "primary-root", common.RoleRootUser)
	seedInvitationUser(t, db, 10, "inviter", common.RoleCommonUser)
	seedInvitationUser(t, db, 20, "invitee", common.RoleCommonUser)
	reward := seedCashInvitationReward(t, 10, 20, "preserve-registration-state")

	request, err := SubmitInvitationRewardPayout(context.Background(), 10)
	require.NoError(t, err)
	var use model.InviteCodeUse
	require.NoError(t, db.First(&use, reward.InviteCodeUseID).Error)
	require.Equal(t, model.InviteRewardStatusAvailable, use.InviterRewardStatus)
	require.NoError(t, ReviewInvitationRewardPayoutRequest(context.Background(), ReviewInvitationRewardPayoutRequestInput{
		RequestID: request.ID, ReviewerID: 1, Approve: true, ReviewReason: "approve cash commission",
	}))

	require.NoError(t, db.First(&use, reward.InviteCodeUseID).Error)
	require.Equal(t, model.InviteRewardStatusAvailable, use.InviterRewardStatus)
	require.Zero(t, use.RewardsCompletedAt)
	var inviter model.User
	require.NoError(t, db.First(&inviter, 10).Error)
	require.Zero(t, inviter.AffCount)
	require.Equal(t, 100, inviter.AffHistoryQuota)
}

func TestInvitationRewardMutationsEmitOnlyCanonicalBusinessLogs(t *testing.T) {
	db := setupInvitationServiceTest(t)
	enableInvitationPayoutCompliance(t)
	seedInvitationUser(t, db, 1, "primary-root", common.RoleRootUser)
	seedInvitationUser(t, db, 10, "inviter", common.RoleCommonUser)
	seedInvitationUser(t, db, 20, "invitee", common.RoleCommonUser)
	seedAvailableInvitationReward(t, 10, 20, 100, "canonical-logs")

	request, err := SubmitInvitationRewardPayout(context.Background(), 10)
	require.NoError(t, err)
	require.NoError(t, ReviewInvitationRewardPayoutRequest(context.Background(), ReviewInvitationRewardPayoutRequestInput{
		RequestID: request.ID, ReviewerID: 1, Approve: true, ReviewReason: "approve reward",
	}))
	require.NoError(t, FreezeInvitationRewardPayoutRequest(context.Background(), InvitationRewardPayoutMutationInput{
		RequestID: request.ID, ActorID: 1, Reason: "review reward",
	}))
	require.NoError(t, UnfreezeInvitationRewardPayoutRequest(context.Background(), InvitationRewardPayoutMutationInput{
		RequestID: request.ID, ActorID: 1, Reason: "release reward",
	}))

	walletEventIDs := []string{
		"wallet:" + request.OperationID,
		fmt.Sprintf("wallet:invite-reward-freeze:%d:1", request.ID),
		fmt.Sprintf("wallet:invite-reward-unfreeze:%d:1", request.ID),
	}
	var walletAuditCount int64
	require.NoError(t, db.Model(&model.AuditOutboxEvent{}).Where("event_id IN ?", walletEventIDs).Count(&walletAuditCount).Error)
	require.Zero(t, walletAuditCount)
	var businessAuditCount int64
	require.NoError(t, db.Model(&model.AuditOutboxEvent{}).Where("event_type IN ?", []string{
		"invitation.reward_payout_reviewed",
		"invitation.reward_payout_frozen",
		"invitation.reward_payout_unfrozen",
	}).Count(&businessAuditCount).Error)
	require.EqualValues(t, 3, businessAuditCount)
}

func TestInvitationRewardRefundWhileFrozenRestoresOnlyUnreversedNet(t *testing.T) {
	db := setupInvitationServiceTest(t)
	enableInvitationPayoutCompliance(t)
	seedInvitationUser(t, db, 1, "primary-root", common.RoleRootUser)
	seedInvitationUser(t, db, 10, "inviter", common.RoleCommonUser)
	seedInvitationUser(t, db, 20, "invitee", common.RoleCommonUser)
	reward := seedCashInvitationReward(t, 10, 20, "frozen-refund")
	request, err := SubmitInvitationRewardPayout(context.Background(), 10)
	require.NoError(t, err)
	require.NoError(t, ReviewInvitationRewardPayoutRequest(context.Background(), ReviewInvitationRewardPayoutRequestInput{
		RequestID: request.ID, ReviewerID: 1, Approve: true, ReviewReason: "initial approval",
	}))
	require.NoError(t, FreezeInvitationRewardPayoutRequest(context.Background(), InvitationRewardPayoutMutationInput{
		RequestID: request.ID, ActorID: 1, Reason: "hold during refund review",
	}))

	reverseCashInvitationReward(t, reward, "frozen-refund-1", 400)
	var reversalLedgerCount int64
	require.NoError(t, db.Model(&model.WalletLedgerEntry{}).Where("category = ?", "invitation_commission_reversal").Count(&reversalLedgerCount).Error)
	require.Zero(t, reversalLedgerCount)
	require.NoError(t, UnfreezeInvitationRewardPayoutRequest(context.Background(), InvitationRewardPayoutMutationInput{
		RequestID: request.ID, ActorID: 1, Reason: "refund evidence settled",
	}))

	var inviter model.User
	require.NoError(t, db.First(&inviter, 10).Error)
	require.Equal(t, int64(60), inviter.Quota)
	var actions []model.InviteRewardPayoutAction
	require.NoError(t, db.Where("request_id = ?", request.ID).Order("id ASC").Find(&actions).Error)
	require.Len(t, actions, 2)
	require.Equal(t, []int64{100, 60}, []int64{actions[0].Amount, actions[1].Amount})
}

func TestInvitationRewardSubmitAndReviewCreditsExactlyOnce(t *testing.T) {
	db := setupInvitationServiceTest(t)
	enableInvitationPayoutCompliance(t)
	seedInvitationUser(t, db, 1, "primary-root", common.RoleRootUser)
	seedInvitationUser(t, db, 10, "inviter", common.RoleCommonUser)
	seedInvitationUser(t, db, 20, "invitee-one", common.RoleCommonUser)
	seedInvitationUser(t, db, 21, "invitee-two", common.RoleCommonUser)
	first := seedAvailableInvitationReward(t, 10, 20, 10, "first")
	second := seedAvailableInvitationReward(t, 10, 21, 20, "second")

	request, err := SubmitInvitationRewardPayout(context.Background(), 10)
	require.NoError(t, err)
	require.EqualValues(t, 30, request.Amount)
	require.Equal(t, model.InviteRewardPayoutRequestStatusPending, request.Status)

	var rewards []model.InviteRewardPayout
	require.NoError(t, db.Where("id IN ?", []int64{first.ID, second.ID}).Order("id ASC").Find(&rewards).Error)
	require.Len(t, rewards, 2)
	require.Equal(t, model.InviteRewardStatusPendingPayout, rewards[0].Status)
	require.Equal(t, model.InviteRewardStatusPendingPayout, rewards[1].Status)

	review := ReviewInvitationRewardPayoutRequestInput{
		RequestID: request.ID, ReviewerID: 1, Approve: true,
	}
	require.NoError(t, ReviewInvitationRewardPayoutRequest(context.Background(), review))
	require.ErrorIs(t, ReviewInvitationRewardPayoutRequest(context.Background(), review), ErrInviteRewardReviewInvalid)

	var inviter model.User
	require.NoError(t, db.First(&inviter, 10).Error)
	require.EqualValues(t, 30, inviter.Quota)
	require.Equal(t, 2, inviter.AffCount)
	require.Equal(t, 30, inviter.AffHistoryQuota)
	var ledgerEntries []model.WalletLedgerEntry
	require.NoError(t, db.Find(&ledgerEntries).Error)
	require.Len(t, ledgerEntries, 1)
	require.Equal(t, request.OperationID, ledgerEntries[0].OperationID)
	require.Equal(t, "invitation_reward_payout", ledgerEntries[0].Category)
}

func TestInvitationRewardRejectedRequestRestoresAvailability(t *testing.T) {
	db := setupInvitationServiceTest(t)
	enableInvitationPayoutCompliance(t)
	seedInvitationUser(t, db, 1, "primary-root", common.RoleRootUser)
	seedInvitationUser(t, db, 10, "inviter", common.RoleCommonUser)
	seedInvitationUser(t, db, 20, "invitee", common.RoleCommonUser)
	reward := seedAvailableInvitationReward(t, 10, 20, 25, "rejected")

	request, err := SubmitInvitationRewardPayout(context.Background(), 10)
	require.NoError(t, err)
	require.NoError(t, ReviewInvitationRewardPayoutRequest(context.Background(), ReviewInvitationRewardPayoutRequestInput{
		RequestID: request.ID, ReviewerID: 1, ReviewReason: "registration was reversed",
	}))

	require.NoError(t, db.First(&reward, reward.ID).Error)
	require.Equal(t, model.InviteRewardStatusAvailable, reward.Status)
	var storedRequest model.InviteRewardPayoutRequest
	require.NoError(t, db.First(&storedRequest, request.ID).Error)
	require.Equal(t, model.InviteRewardPayoutRequestStatusRejected, storedRequest.Status)
	require.Equal(t, "registration was reversed", storedRequest.Reason)

	retry, err := SubmitInvitationRewardPayout(context.Background(), 10)
	require.NoError(t, err)
	require.NotEqual(t, request.ID, retry.ID)
	require.EqualValues(t, 25, retry.Amount)
	var ledgerCount int64
	require.NoError(t, db.Model(&model.WalletLedgerEntry{}).Count(&ledgerCount).Error)
	require.Zero(t, ledgerCount)
}

func TestInviteeRewardDoesNotEnterSummaryOrPayoutRequest(t *testing.T) {
	db := setupInvitationServiceTest(t)
	enableInvitationPayoutCompliance(t)
	seedInvitationUser(t, db, 1, "inviter", common.RoleCommonUser)
	seedInvitationUser(t, db, 10, "invitee", common.RoleCommonUser)
	use := model.InviteCodeUse{
		InviteCodeID: 1, InviterUserID: 1, RegisteredUserID: 10,
		RegisteredAt: time.Now().Unix(), Source: "password",
		InviterRewardStatus: model.InviteRewardStatusSkipped,
		InviteeReward:       25, InviteeRewardStatus: model.InviteRewardStatusAvailable,
		AuditEventID: "invitee-reward-not-withdrawable",
	}
	require.NoError(t, db.Create(&use).Error)
	require.NoError(t, db.Create(&model.InviteRewardPayout{
		InviteCodeUseID: use.ID, TriggerType: model.InviteRewardTriggerRegistrationInviter,
		TriggerID: fmt.Sprint(use.ID), RecipientKind: model.InviteRewardRecipientInvitee,
		RecipientUserID: 10, EligibleQuota: 25, Amount: 25,
		ReversalState: model.InviteRewardReversalNone, Status: model.InviteRewardStatusAvailable,
	}).Error)

	summary, err := GetInvitationRewardSummary(context.Background(), 10)
	require.NoError(t, err)
	require.Equal(t, InvitationRewardSummary{CashCommissionRate: "0.00"}, summary)
	_, err = SubmitInvitationRewardPayout(context.Background(), 10)
	require.ErrorIs(t, err, ErrInviteRewardUnavailable)
}

func TestLegacyInviteePayoutRequestCanStillBeApproved(t *testing.T) {
	db := setupInvitationServiceTest(t)
	enableInvitationPayoutCompliance(t)
	seedInvitationUser(t, db, 1, "primary-root", common.RoleRootUser)
	seedInvitationUser(t, db, 10, "legacy-invitee", common.RoleCommonUser)
	use := model.InviteCodeUse{
		InviteCodeID: 1, InviterUserID: 20, RegisteredUserID: 10,
		RegisteredAt: time.Now().Unix(), Source: "password",
		InviterRewardStatus: model.InviteRewardStatusSkipped,
		InviteeReward:       25, InviteeRewardStatus: model.InviteRewardStatusPendingPayout,
		AuditEventID: "legacy-invitee-payout-request",
	}
	require.NoError(t, db.Create(&use).Error)
	reward := model.InviteRewardPayout{
		InviteCodeUseID: use.ID, TriggerType: model.InviteRewardTriggerRegistrationInviter,
		TriggerID: fmt.Sprint(use.ID), RecipientKind: model.InviteRewardRecipientInvitee,
		RecipientUserID: 10, EligibleQuota: 25, Amount: 25,
		ReversalState: model.InviteRewardReversalNone, Status: model.InviteRewardStatusPendingPayout,
	}
	require.NoError(t, db.Create(&reward).Error)
	pendingUserSlot := 10
	request := model.InviteRewardPayoutRequest{
		OperationID: "legacy-invitee-payout", UserID: 10, PendingUserSlot: &pendingUserSlot,
		Amount: 25, Status: model.InviteRewardPayoutRequestStatusPending,
	}
	require.NoError(t, db.Create(&request).Error)
	require.NoError(t, db.Create(&model.InviteRewardPayoutRequestItem{
		RequestID: request.ID, RewardID: reward.ID,
	}).Error)

	require.NoError(t, ReviewInvitationRewardPayoutRequest(context.Background(), ReviewInvitationRewardPayoutRequestInput{
		RequestID: request.ID, ReviewerID: 1, Approve: true,
	}))
	require.NoError(t, db.First(&reward, reward.ID).Error)
	require.Equal(t, model.InviteRewardStatusPaid, reward.Status)
	require.NoError(t, db.First(&use, use.ID).Error)
	require.Equal(t, model.InviteRewardStatusPaid, use.InviteeRewardStatus)
	var invitee model.User
	require.NoError(t, db.First(&invitee, 10).Error)
	require.EqualValues(t, 25, invitee.Quota)
	require.Zero(t, invitee.AffCount)
	require.Zero(t, invitee.AffHistoryQuota)
}

func TestInvitationRewardSummaryAndRequestListUseAggregateStates(t *testing.T) {
	db := setupInvitationServiceTest(t)
	seedInvitationUser(t, db, 1, "primary-root", common.RoleRootUser)
	seedInvitationUser(t, db, 10, "inviter", common.RoleCommonUser)
	for index, reward := range []struct {
		amount int
		status string
	}{
		{amount: 10, status: model.InviteRewardStatusAvailable},
		{amount: 20, status: model.InviteRewardStatusPendingPayout},
		{amount: 30, status: model.InviteRewardStatusPaid},
	} {
		seedInvitationUser(t, db, 20+index, fmt.Sprintf("invitee-%d", index), common.RoleCommonUser)
		use := model.InviteCodeUse{
			InviteCodeID: 1, InviterUserID: 10, RegisteredUserID: 20 + index,
			RegisteredAt: time.Now().Unix(), Source: "password", InviterReward: reward.amount,
			InviterRewardStatus: reward.status, InviteeRewardStatus: model.InviteRewardStatusSkipped,
			AuditEventID: fmt.Sprintf("summary-use-%d", index),
		}
		require.NoError(t, db.Create(&use).Error)
		require.NoError(t, db.Create(&model.InviteRewardPayout{
			InviteCodeUseID: use.ID, TriggerType: model.InviteRewardTriggerRegistrationInviter,
			TriggerID: fmt.Sprint(use.ID), RecipientKind: model.InviteRewardRecipientInviter,
			RecipientUserID: 10, EligibleQuota: int64(reward.amount), Amount: int64(reward.amount),
			ReversalState: model.InviteRewardReversalNone, Status: reward.status,
		}).Error)
	}

	summary, err := GetInvitationRewardSummary(context.Background(), 10)
	require.NoError(t, err)
	require.EqualValues(t, 3, summary.InvitedCount)
	require.EqualValues(t, 60, summary.TotalEarned)
	require.EqualValues(t, 10, summary.Available)
	require.EqualValues(t, 20, summary.PendingPayout)
	require.EqualValues(t, 30, summary.Paid)

	var rewards []model.InviteRewardPayout
	require.NoError(t, db.Order("id ASC").Find(&rewards).Error)
	pendingRequest := model.InviteRewardPayoutRequest{
		OperationID: "summary-request-pending", UserID: 10, Amount: 10,
		Status: model.InviteRewardPayoutRequestStatusPending,
	}
	require.NoError(t, db.Create(&pendingRequest).Error)
	require.NoError(t, db.Create(&model.InviteRewardPayoutRequestItem{
		RequestID: pendingRequest.ID, RewardID: rewards[0].ID,
	}).Error)
	paidRequest := model.InviteRewardPayoutRequest{
		OperationID: "summary-request-paid", UserID: 10, Amount: 30,
		Status: model.InviteRewardPayoutRequestStatusPaid,
	}
	require.NoError(t, db.Create(&paidRequest).Error)
	require.NoError(t, db.Create(&model.InviteRewardPayoutRequestItem{
		RequestID: paidRequest.ID, RewardID: rewards[2].ID,
	}).Error)

	list, err := ListInvitationRewardPayoutRequests(context.Background(), InvitationAdminListQuery{
		ActorID: 1, Page: 1, PageSize: 20,
	})
	require.NoError(t, err)
	require.EqualValues(t, 2, list.Total)
	require.Equal(t, pendingRequest.ID, list.Items[0].ID)
	require.Equal(t, "inviter", list.Items[0].Username)
	require.Equal(t, 1, list.Items[0].ItemCount)
}

func TestInvitationRewardSummaryReturnsCurrentCashPolicyAndNetAmounts(t *testing.T) {
	db := setupInvitationServiceTest(t)
	seedInvitationUser(t, db, 10, "inviter", common.RoleCommonUser)
	seedInvitationUser(t, db, 20, "invitee", common.RoleCommonUser)
	reward := seedCashInvitationReward(t, 10, 20, "summary-net")
	require.NoError(t, db.Model(&reward).Updates(map[string]any{
		"reversed_amount": int64(25), "reversal_state": model.InviteRewardReversalPartial,
	}).Error)
	setting := operation_setting.GetInvitationIncentiveSetting()
	setting.Enabled = true
	setting.CashCommissionEnabled = true
	setting.CashCommissionRate = "200"

	summary, err := GetInvitationRewardSummary(context.Background(), 10)
	require.NoError(t, err)
	require.True(t, summary.CashCommissionVisible)
	require.Equal(t, "200.00", summary.CashCommissionRate)
	require.Equal(t, int64(75), summary.Available)
	require.Equal(t, int64(75), summary.TotalEarned)

	setting.Enabled = false
	summary, err = GetInvitationRewardSummary(context.Background(), 10)
	require.NoError(t, err)
	require.False(t, summary.CashCommissionVisible)
	require.Equal(t, "200.00", summary.CashCommissionRate)
}

func TestInvitationRewardPayoutHistoryUsesServerSideStatusSet(t *testing.T) {
	db := setupInvitationServiceTest(t)
	seedInvitationUser(t, db, 1, "root", common.RoleRootUser)
	seedInvitationUser(t, db, 10, "inviter", common.RoleCommonUser)

	statuses := []string{
		model.InviteRewardPayoutRequestStatusPending,
		model.InviteRewardPayoutRequestStatusPaid,
		model.InviteRewardPayoutRequestStatusRejected,
		model.InviteRewardPayoutRequestStatusFrozen,
		model.InviteRewardPayoutRequestStatusCancelled,
	}
	for index, status := range statuses {
		require.NoError(t, db.Create(&model.InviteRewardPayoutRequest{
			OperationID: fmt.Sprintf("history-request-%d", index),
			UserID:      10, Amount: int64(index + 1), Status: status,
		}).Error)
	}

	firstPage, err := ListInvitationRewardPayoutRequests(context.Background(), InvitationAdminListQuery{
		ActorID: 1, Page: 1, PageSize: 2, Status: "history",
	})
	require.NoError(t, err)
	require.EqualValues(t, 4, firstPage.Total)
	require.Len(t, firstPage.Items, 2)
	require.Equal(t, model.InviteRewardPayoutRequestStatusCancelled, firstPage.Items[0].Status)
	require.Equal(t, model.InviteRewardPayoutRequestStatusFrozen, firstPage.Items[1].Status)

	secondPage, err := ListInvitationRewardPayoutRequests(context.Background(), InvitationAdminListQuery{
		ActorID: 1, Page: 2, PageSize: 2, Status: "history",
	})
	require.NoError(t, err)
	require.EqualValues(t, 4, secondPage.Total)
	require.Len(t, secondPage.Items, 2)
	require.Equal(t, model.InviteRewardPayoutRequestStatusRejected, secondPage.Items[0].Status)
	require.Equal(t, model.InviteRewardPayoutRequestStatusPaid, secondPage.Items[1].Status)
}

func TestInvitationRewardConcurrentSubmitCreatesOneRequest(t *testing.T) {
	db := setupInvitationServiceTest(t)
	enableInvitationPayoutCompliance(t)
	seedInvitationUser(t, db, 10, "inviter", common.RoleCommonUser)
	seedInvitationUser(t, db, 20, "invitee", common.RoleCommonUser)
	seedAvailableInvitationReward(t, 10, 20, 10, "concurrent")

	start := make(chan struct{})
	errorsByAttempt := make(chan error, 2)
	var waitGroup sync.WaitGroup
	for attempt := 0; attempt < 2; attempt++ {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			<-start
			_, err := SubmitInvitationRewardPayout(context.Background(), 10)
			errorsByAttempt <- err
		}()
	}
	close(start)
	waitGroup.Wait()
	close(errorsByAttempt)

	successes := 0
	unavailable := 0
	for err := range errorsByAttempt {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, ErrInviteRewardUnavailable):
			unavailable++
		default:
			t.Fatalf("unexpected submit error: %v", err)
		}
	}
	require.Equal(t, 1, successes)
	require.Equal(t, 1, unavailable)
	var requestCount int64
	require.NoError(t, db.Model(&model.InviteRewardPayoutRequest{}).Count(&requestCount).Error)
	require.EqualValues(t, 1, requestCount)
}

func TestInvitationRewardStatusMigrationPreservesPaidAndRejectedHistory(t *testing.T) {
	db := setupInvitationServiceTest(t)
	seedInvitationUser(t, db, 10, "inviter", common.RoleCommonUser)
	seedInvitationUser(t, db, 20, "invitee", common.RoleCommonUser)
	statuses := []string{
		model.InviteRewardStatusPendingReview,
		model.InviteRewardStatusApproved,
		model.InviteRewardStatusWithdrawn,
		model.InviteRewardStatusRejected,
	}
	for index, status := range statuses {
		use := model.InviteCodeUse{
			InviteCodeID: 1, InviterUserID: 10, RegisteredUserID: 20 + index,
			RegisteredAt: time.Now().Unix(), Source: "password", InviterReward: 10,
			InviterRewardStatus: status, InviteeRewardStatus: model.InviteRewardStatusSkipped,
			AuditEventID: fmt.Sprintf("migration-use-%d", index),
		}
		require.NoError(t, db.Create(&use).Error)
		require.NoError(t, db.Create(&model.InviteRewardPayout{
			InviteCodeUseID: use.ID, TriggerType: model.InviteRewardTriggerRegistrationInviter,
			TriggerID: fmt.Sprint(use.ID), RecipientKind: model.InviteRewardRecipientInviter,
			RecipientUserID: 10, EligibleQuota: 10, Amount: 10,
			ReversalState: model.InviteRewardReversalNone, Status: status,
		}).Error)
	}

	require.NoError(t, model.MigrateInvitationRewardStatuses(db))
	var rewards []model.InviteRewardPayout
	require.NoError(t, db.Order("id ASC").Find(&rewards).Error)
	require.Equal(t, []string{
		model.InviteRewardStatusAvailable,
		model.InviteRewardStatusAvailable,
		model.InviteRewardStatusPaid,
		model.InviteRewardStatusRejected,
	}, []string{rewards[0].Status, rewards[1].Status, rewards[2].Status, rewards[3].Status})
}

func TestInvitationRewardFreezeAndUnfreezeAreRevisionedAndFullyAudited(t *testing.T) {
	db := setupInvitationServiceTest(t)
	enableInvitationPayoutCompliance(t)
	seedInvitationUser(t, db, 1, "primary-root", common.RoleRootUser)
	seedInvitationUser(t, db, 10, "inviter", common.RoleCommonUser)
	seedInvitationUser(t, db, 20, "invitee-one", common.RoleCommonUser)
	seedInvitationUser(t, db, 21, "invitee-two", common.RoleCommonUser)
	seedAvailableInvitationReward(t, 10, 20, 10, "freeze-first")
	seedAvailableInvitationReward(t, 10, 21, 20, "freeze-second")

	request, err := SubmitInvitationRewardPayout(context.Background(), 10)
	require.NoError(t, err)
	require.NoError(t, ReviewInvitationRewardPayoutRequest(context.Background(), ReviewInvitationRewardPayoutRequestInput{
		RequestID: request.ID, ReviewerID: 1, Approve: true, ReviewReason: "initial approval",
	}))
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		_, adjustErr := AdjustWithTx(tx, 10, -25, "freeze-test-spend", "test_spend", WalletReference{
			Type: "test", ID: "freeze", Context: "spent_after_approval",
		})
		return adjustErr
	}))

	freeze := InvitationRewardPayoutMutationInput{
		RequestID: request.ID, ActorID: 1, Reason: "confirmed fraudulent invitations",
	}
	require.NoError(t, FreezeInvitationRewardPayoutRequest(context.Background(), freeze))
	require.ErrorIs(t, FreezeInvitationRewardPayoutRequest(context.Background(), freeze), ErrInviteRewardMutationInvalid)

	var target model.User
	require.NoError(t, db.First(&target, 10).Error)
	require.EqualValues(t, -25, target.Quota)
	var storedRequest model.InviteRewardPayoutRequest
	require.NoError(t, db.First(&storedRequest, request.ID).Error)
	require.Equal(t, model.InviteRewardPayoutRequestStatusFrozen, storedRequest.Status)
	require.Equal(t, 1, storedRequest.FreezeRevision)
	var rewards []model.InviteRewardPayout
	require.NoError(t, db.Joins("JOIN invite_reward_payout_request_items AS items ON items.reward_id = invite_reward_payouts.id").
		Where("items.request_id = ?", request.ID).Order("invite_reward_payouts.id ASC").Find(&rewards).Error)
	require.Len(t, rewards, 2)
	for _, reward := range rewards {
		require.Equal(t, model.InviteRewardStatusFrozen, reward.Status)
	}
	var freezeAction model.InviteRewardPayoutAction
	require.NoError(t, db.Where("request_id = ? AND action = ? AND revision = ?", request.ID, model.InviteRewardPayoutActionFreeze, 1).First(&freezeAction).Error)
	require.Equal(t, freeze.Reason, freezeAction.Reason)
	require.EqualValues(t, 30, freezeAction.Amount)
	require.Equal(t, fmt.Sprintf("invite-reward-freeze:%d:1", request.ID), freezeAction.WalletOperationID)

	summary, err := GetInvitationRewardSummary(context.Background(), 10)
	require.NoError(t, err)
	require.EqualValues(t, 30, summary.Frozen)
	require.Zero(t, summary.Paid)
	require.EqualValues(t, 30, summary.TotalEarned)

	unfreeze := InvitationRewardPayoutMutationInput{
		RequestID: request.ID, ActorID: 1, Reason: "manual review cleared the account",
	}
	require.NoError(t, UnfreezeInvitationRewardPayoutRequest(context.Background(), unfreeze))
	require.ErrorIs(t, UnfreezeInvitationRewardPayoutRequest(context.Background(), unfreeze), ErrInviteRewardMutationInvalid)
	require.NoError(t, db.First(&target, 10).Error)
	require.EqualValues(t, 5, target.Quota)
	require.NoError(t, db.First(&storedRequest, request.ID).Error)
	require.Equal(t, model.InviteRewardPayoutRequestStatusPaid, storedRequest.Status)
	require.Equal(t, 1, storedRequest.FreezeRevision)

	require.NoError(t, FreezeInvitationRewardPayoutRequest(context.Background(), InvitationRewardPayoutMutationInput{
		RequestID: request.ID, ActorID: 1, Reason: "new evidence requires another freeze",
	}))
	require.NoError(t, db.First(&storedRequest, request.ID).Error)
	require.Equal(t, 2, storedRequest.FreezeRevision)

	var actions []model.InviteRewardPayoutAction
	require.NoError(t, db.Where("request_id = ?", request.ID).Order("id ASC").Find(&actions).Error)
	require.Len(t, actions, 3)
	require.Equal(t, []string{
		model.InviteRewardPayoutActionFreeze,
		model.InviteRewardPayoutActionUnfreeze,
		model.InviteRewardPayoutActionFreeze,
	}, []string{actions[0].Action, actions[1].Action, actions[2].Action})
	require.Equal(t, []int{1, 1, 2}, []int{actions[0].Revision, actions[1].Revision, actions[2].Revision})

	var freezeLedgerCount int64
	require.NoError(t, db.Model(&model.WalletLedgerEntry{}).
		Where("category IN ?", []string{"invitation_reward_freeze", "invitation_reward_unfreeze"}).Count(&freezeLedgerCount).Error)
	require.EqualValues(t, 3, freezeLedgerCount)
	var businessAuditCount int64
	require.NoError(t, db.Model(&model.AuditOutboxEvent{}).
		Where("event_type IN ?", []string{"invitation.reward_payout_frozen", "invitation.reward_payout_unfrozen"}).Count(&businessAuditCount).Error)
	require.EqualValues(t, 3, businessAuditCount)
}

func TestInvitationRewardFreezeUsesApprovalPermissionMatrix(t *testing.T) {
	db := setupInvitationServiceTest(t)
	seedInvitationUser(t, db, 1, "primary-root", common.RoleRootUser)
	seedInvitationUser(t, db, 2, "admin-actor", common.RoleAdminUser)
	seedInvitationUser(t, db, 10, "admin-target", common.RoleAdminUser)
	request := model.InviteRewardPayoutRequest{
		OperationID: "permission-paid-request", UserID: 10, Amount: 10,
		Status: model.InviteRewardPayoutRequestStatusPaid, ReviewerID: 1,
	}
	require.NoError(t, db.Create(&request).Error)

	err := FreezeInvitationRewardPayoutRequest(context.Background(), InvitationRewardPayoutMutationInput{
		RequestID: request.ID, ActorID: 2, Reason: "admin cannot manage peer admin",
	})
	require.ErrorIs(t, err, ErrInviteTargetForbidden)
}
