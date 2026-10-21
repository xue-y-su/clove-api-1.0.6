package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestInvitationRewardWorkerBackfillsInviterAndCreditsInvitee(t *testing.T) {
	db := setupInvitationServiceTest(t)
	seedInvitationUser(t, db, 1, "inviter", common.RoleCommonUser)
	seedInvitationUser(t, db, 99, "invitee", common.RoleCommonUser)
	invite := seedInviteCode(t, db, 1, "worker-reward-code", 3)
	use := model.InviteCodeUse{
		InviteCodeID: invite.ID, InviterUserID: 1, RegisteredUserID: 99,
		RegisteredAt: time.Now().Unix(), Source: "password",
		InviterReward: 10, InviteeReward: 5,
		InviterRewardStatus: model.InviteRewardStatusPending,
		InviteeRewardStatus: model.InviteRewardStatusPending,
		AuditEventID:        "worker-reward-use",
	}
	require.NoError(t, db.Create(&use).Error)

	processed, failed, err := ProcessPendingInvitationRewards(context.Background(), 10)
	require.NoError(t, err)
	require.Equal(t, 1, processed)
	require.Zero(t, failed)
	require.NoError(t, db.First(&use, use.ID).Error)
	require.Equal(t, model.InviteRewardStatusAvailable, use.InviterRewardStatus)
	require.Equal(t, model.InviteRewardStatusPaid, use.InviteeRewardStatus)
	var payouts []model.InviteRewardPayout
	require.NoError(t, db.Find(&payouts).Error)
	require.Len(t, payouts, 1)
	require.Equal(t, model.InviteRewardRecipientInviter, payouts[0].RecipientKind)
	require.Equal(t, model.InviteRewardStatusAvailable, payouts[0].Status)

	processed, failed, err = ProcessPendingInvitationRewards(context.Background(), 10)
	require.NoError(t, err)
	require.Zero(t, processed)
	require.Zero(t, failed)

	var inviter model.User
	require.NoError(t, db.First(&inviter, 1).Error)
	require.Zero(t, inviter.Quota)
	require.Zero(t, inviter.AffCount)
	var invitee model.User
	require.NoError(t, db.First(&invitee, 99).Error)
	require.EqualValues(t, 5, invitee.Quota)
	var entries []model.WalletLedgerEntry
	require.NoError(t, db.Find(&entries).Error)
	require.Len(t, entries, 1)
	require.Equal(t, "invitation_gift", entries[0].Category)
}

func TestInvitationRewardWorkerDoesNotBackfillDisabledRegistrationRewards(t *testing.T) {
	db := setupInvitationServiceTest(t)
	seedInvitationUser(t, db, 1, "inviter", common.RoleCommonUser)
	seedInvitationUser(t, db, 99, "invitee", common.RoleCommonUser)
	invite := seedInviteCode(t, db, 1, "worker-disabled-code", 3)
	use := model.InviteCodeUse{
		InviteCodeID: invite.ID, InviterUserID: 1, RegisteredUserID: 99,
		RegisteredAt: time.Now().Unix(), Source: "password",
		InviterRewardStatus: model.InviteRewardStatusSkipped,
		InviteeRewardStatus: model.InviteRewardStatusSkipped,
		AuditEventID:        "worker-disabled-use",
	}
	require.NoError(t, db.Create(&use).Error)

	processed, failed, err := ProcessPendingInvitationRewards(context.Background(), 10)
	require.NoError(t, err)
	require.Zero(t, processed)
	require.Zero(t, failed)
	var payoutCount int64
	require.NoError(t, db.Model(&model.InviteRewardPayout{}).Count(&payoutCount).Error)
	require.Zero(t, payoutCount)
}

func TestInvitationRewardWorkerBackfillsByRegistrationTriggerIdentity(t *testing.T) {
	db := setupInvitationServiceTest(t)
	seedInvitationUser(t, db, 1, "inviter", common.RoleCommonUser)
	seedInvitationUser(t, db, 99, "invitee", common.RoleCommonUser)
	invite := seedInviteCode(t, db, 1, "worker-source-code", 3)
	use := model.InviteCodeUse{
		InviteCodeID: invite.ID, InviterUserID: 1, RegisteredUserID: 99,
		RegisteredAt: time.Now().Unix(), Source: "password", InviterReward: 10,
		InviterRewardStatus: model.InviteRewardStatusPending,
		InviteeRewardStatus: model.InviteRewardStatusSkipped,
		AuditEventID:        "worker-source-use",
	}
	require.NoError(t, db.Create(&use).Error)
	require.NoError(t, db.Create(&model.InviteRewardPayout{
		InviteCodeUseID: use.ID,
		TriggerType:     model.InviteRewardTriggerCashTopUp,
		TriggerID:       "payment-1",
		RecipientKind:   model.InviteRewardRecipientInviter,
		RecipientUserID: 1,
		EligibleQuota:   100,
		RateBasisPoints: 1_000,
		Amount:          10,
		ReversalState:   model.InviteRewardReversalNone,
		Status:          model.InviteRewardStatusAvailable,
	}).Error)

	processed, failed, err := ProcessPendingInvitationRewards(context.Background(), 10)
	require.NoError(t, err)
	require.Equal(t, 1, processed)
	require.Zero(t, failed)
	var registrationReward model.InviteRewardPayout
	require.NoError(t, db.Where(
		"trigger_type = ? AND trigger_id = ? AND recipient_kind = ?",
		model.InviteRewardTriggerRegistrationInviter,
		fmt.Sprint(use.ID),
		model.InviteRewardRecipientInviter,
	).First(&registrationReward).Error)
}

func TestInvitationRewardWorkerStopIsIdempotent(t *testing.T) {
	setupInvitationServiceTest(t)
	worker := startInvitationRewardWorker(context.Background(), time.Hour, 10)
	stopContext, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	require.NoError(t, worker.Stop(stopContext))
	require.NoError(t, worker.Stop(stopContext))
}

func TestInvitationRewardWorkerReturnsWhenContextIsAlreadyCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	require.NoError(t, runInvitationRewardWorker(ctx, time.Hour, 10))
}
