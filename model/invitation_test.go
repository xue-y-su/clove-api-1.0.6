package model

import (
	"strconv"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type legacyInviteRewardPayout struct {
	ID              int64  `gorm:"primaryKey;autoIncrement"`
	InviteCodeUseID int64  `gorm:"not null;uniqueIndex:uniq_invite_reward_recipient,priority:1;index"`
	RecipientKind   string `gorm:"type:varchar(16);not null;uniqueIndex:uniq_invite_reward_recipient,priority:2;index"`
	RecipientUserID int    `gorm:"not null;index"`
	Amount          int    `gorm:"not null"`
	Status          string `gorm:"type:varchar(32);not null;index"`
	ReviewerID      int    `gorm:"not null;default:0;index"`
	ReviewReason    string `gorm:"type:varchar(500);not null;default:''"`
	ReviewedAt      int64  `gorm:"bigint;not null;default:0"`
	WithdrawnAt     int64  `gorm:"bigint;not null;default:0;index"`
	CreatedAt       int64  `gorm:"bigint;autoCreateTime;index"`
	UpdatedAt       int64  `gorm:"bigint;autoUpdateTime"`
}

func (legacyInviteRewardPayout) TableName() string {
	return "invite_reward_payouts"
}

func TestInvitationModelsUseNullableUniqueActiveSlots(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/invitation-model.db"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.AutoMigrate(&InviteCodeApplication{}, &InviteCode{}, &InviteCodeUse{}))
	require.NoError(t, db.AutoMigrate(&InviteCodeApplication{}, &InviteCode{}, &InviteCodeUse{}))

	userID := 7
	require.NoError(t, db.Create(&InviteCodeApplication{
		UserID: userID, Reason: "first", Status: InviteApplicationStatusPending, PendingUserSlot: &userID,
	}).Error)
	require.Error(t, db.Create(&InviteCodeApplication{
		UserID: userID, Reason: "duplicate", Status: InviteApplicationStatusPending, PendingUserSlot: &userID,
	}).Error)
	require.NoError(t, db.Create(&InviteCodeApplication{
		UserID: userID, Reason: "terminal one", Status: InviteApplicationStatusRejected,
	}).Error)
	require.NoError(t, db.Create(&InviteCodeApplication{
		UserID: userID, Reason: "terminal two", Status: InviteApplicationStatusWithdrawn,
	}).Error)

	ownerID := 8
	require.NoError(t, db.Create(&InviteCode{
		OwnerUserID: ownerID, Code: "invite-one", Kind: InviteCodeKindSingle, MaxUses: 1,
		Status: InviteCodeStatusActive, ActiveOwnerSlot: &ownerID,
	}).Error)
	require.Error(t, db.Create(&InviteCode{
		OwnerUserID: ownerID, Code: "invite-two", Kind: InviteCodeKindSingle, MaxUses: 1,
		Status: InviteCodeStatusActive, ActiveOwnerSlot: &ownerID,
	}).Error)
	require.NoError(t, db.Create(&InviteCode{
		OwnerUserID: ownerID, Code: "invite-terminal", Kind: InviteCodeKindSingle, MaxUses: 1,
		Status: InviteCodeStatusRevoked,
	}).Error)
}

func TestInvitationRewardFreezeModelsUseRevisionedActions(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/invitation-reward-freeze.db"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.AutoMigrate(&InviteRewardPayoutRequest{}, &InviteRewardPayoutAction{}))

	require.Equal(t, "frozen", InviteRewardStatusFrozen)
	require.Equal(t, "frozen", InviteRewardPayoutRequestStatusFrozen)

	request := InviteRewardPayoutRequest{
		OperationID: "reward-request-1", UserID: 7, Amount: 25,
		Status: InviteRewardPayoutRequestStatusPaid, FreezeRevision: 1,
	}
	require.NoError(t, db.Create(&request).Error)
	freeze := InviteRewardPayoutAction{
		RequestID: request.ID, Action: InviteRewardPayoutActionFreeze, Revision: 1,
		ActorID: 1, Reason: "fraud review", Amount: 25,
		WalletOperationID: "invitation-reward-freeze:1:1",
	}
	require.NoError(t, db.Create(&freeze).Error)
	require.Error(t, db.Create(&InviteRewardPayoutAction{
		RequestID: request.ID, Action: InviteRewardPayoutActionFreeze, Revision: 1,
		ActorID: 2, Reason: "duplicate", Amount: 25,
		WalletOperationID: "invitation-reward-freeze:1:duplicate",
	}).Error)
	require.NoError(t, db.Create(&InviteRewardPayoutAction{
		RequestID: request.ID, Action: InviteRewardPayoutActionUnfreeze, Revision: 1,
		ActorID: 1, Reason: "review cleared", Amount: 25,
		WalletOperationID: "invitation-reward-unfreeze:1:1",
	}).Error)
}

func TestInvitationRewardFactsAllowMultipleTransactionSourcesPerInviteUse(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/invitation-reward-facts.db"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.AutoMigrate(&InviteRewardPayout{}))

	first := InviteRewardPayout{
		InviteCodeUseID: 7,
		RecipientKind:   InviteRewardRecipientInviter,
		RecipientUserID: 3,
		TriggerType:     InviteRewardTriggerCashTopUp,
		TriggerID:       "pay-1",
		EligibleQuota:   100,
		RateBasisPoints: 1_000,
		Amount:          10,
		Status:          InviteRewardStatusAvailable,
		ReversalState:   InviteRewardReversalNone,
	}
	second := first
	second.ID = 0
	second.TriggerID = "pay-2"
	require.NoError(t, db.Create(&first).Error)
	require.NoError(t, db.Create(&second).Error)

	duplicate := first
	duplicate.ID = 0
	duplicate.InviteCodeUseID = 8
	require.Error(t, db.Create(&duplicate).Error)
}

func TestInvitationRewardMigrationBackfillsLegacyRegistrationSources(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/invitation-reward-migration.db"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.AutoMigrate(
		&legacyInviteRewardPayout{},
		&InviteRewardPayoutRequest{},
		&InviteRewardPayoutRequestItem{},
	))

	rewardStatuses := []string{
		InviteRewardStatusPaid,
		InviteRewardStatusFrozen,
		InviteRewardStatusRejected,
		InviteRewardStatusPendingPayout,
	}
	requestStatuses := []string{
		InviteRewardPayoutRequestStatusPaid,
		InviteRewardPayoutRequestStatusFrozen,
		InviteRewardPayoutRequestStatusRejected,
		InviteRewardPayoutRequestStatusPending,
	}
	legacyRewards := make([]legacyInviteRewardPayout, 0, len(rewardStatuses))
	requests := make([]InviteRewardPayoutRequest, 0, len(requestStatuses))
	for index, status := range rewardStatuses {
		recipientKind := InviteRewardRecipientInviter
		if index == 1 {
			recipientKind = InviteRewardRecipientInvitee
		}
		reward := legacyInviteRewardPayout{
			InviteCodeUseID: int64(index + 7),
			RecipientKind:   recipientKind,
			RecipientUserID: index + 3,
			Amount:          (index + 1) * 10,
			Status:          status,
		}
		require.NoError(t, db.Create(&reward).Error)
		legacyRewards = append(legacyRewards, reward)

		request := InviteRewardPayoutRequest{
			OperationID: "legacy-request-" + strconv.Itoa(index),
			UserID:      index + 3,
			Amount:      int64(reward.Amount),
			Status:      requestStatuses[index],
		}
		require.NoError(t, db.Create(&request).Error)
		require.NoError(t, db.Create(&InviteRewardPayoutRequestItem{
			RequestID: request.ID,
			RewardID:  reward.ID,
		}).Error)
		requests = append(requests, request)
	}

	require.NoError(t, MigrateInvitationRewardFacts(db))
	require.NoError(t, MigrateInvitationRewardFacts(db))
	require.False(t, db.Migrator().HasIndex(&InviteRewardPayout{}, "uniq_invite_reward_recipient"))
	require.True(t, db.Migrator().HasIndex(&InviteRewardPayout{}, "uniq_invite_reward_source"))
	require.True(t, db.Migrator().HasTable(&InviteRewardReversal{}))
	require.Equal(t, "cancelled", InviteRewardPayoutRequestStatusCancelled)

	for index, legacy := range legacyRewards {
		var reward InviteRewardPayout
		require.NoError(t, db.First(&reward, legacy.ID).Error)
		require.Equal(t, InviteRewardTriggerRegistrationInviter, reward.TriggerType)
		require.Equal(t, strconv.FormatInt(reward.InviteCodeUseID, 10), reward.TriggerID)
		require.Equal(t, int64(legacy.Amount), reward.EligibleQuota)
		require.Equal(t, int64(legacy.Amount), reward.Amount)
		require.Equal(t, legacy.Status, reward.Status)
		require.Zero(t, reward.ReversedAmount)
		require.Equal(t, InviteRewardReversalNone, reward.ReversalState)

		var request InviteRewardPayoutRequest
		require.NoError(t, db.First(&request, requests[index].ID).Error)
		require.Equal(t, requestStatuses[index], request.Status)
		var item InviteRewardPayoutRequestItem
		require.NoError(t, db.First(&item, "request_id = ? AND reward_id = ?", request.ID, reward.ID).Error)
	}

	reversal := InviteRewardReversal{
		RewardID:                 legacyRewards[0].ID,
		RefundSourceType:         "cash_refund",
		RefundSourceID:           "refund-1",
		RefundedBase:             50,
		Amount:                   5,
		CumulativeReversedAmount: 5,
		RewardStatusBefore:       InviteRewardStatusPaid,
		WalletOperationID:        "invite-reversal:refund-1",
	}
	require.NoError(t, db.Create(&reversal).Error)
	duplicate := reversal
	duplicate.ID = 0
	require.Error(t, db.Create(&duplicate).Error)
}
