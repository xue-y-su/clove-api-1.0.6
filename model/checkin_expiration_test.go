package model

import (
	"fmt"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func createWalletWithActiveCheckinReward(t *testing.T, userID int, balance, reward int64) Checkin {
	t.Helper()
	require.NoError(t, DB.Create(&User{
		Id:       userID,
		Username: fmt.Sprintf("checkin-wallet-%d", userID),
		AffCode:  fmt.Sprintf("checkin-aff-%d", userID),
		Quota:    balance,
		Status:   common.UserStatusEnabled,
	}).Error)
	checkin := Checkin{
		UserId:         userID,
		CheckinDate:    "2026-07-29",
		QuotaAwarded:   int(reward),
		QuotaRemaining: reward,
		ExpiresAt:      time.Now().Add(time.Hour).Unix(),
		CreatedAt:      time.Now().Unix(),
	}
	require.NoError(t, DB.Create(&checkin).Error)
	return checkin
}

func adjustWalletForCheckinTest(t *testing.T, userID int, delta int64, operationID, category string) {
	t.Helper()
	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		_, err := AdjustWalletWithTx(tx, userID, delta, operationID, category, WalletAdjustReference{
			Type: "checkin_test", ID: operationID,
		})
		return err
	}))
}

func loadCheckinReward(t *testing.T, id int) Checkin {
	t.Helper()
	var checkin Checkin
	require.NoError(t, DB.First(&checkin, id).Error)
	return checkin
}

func TestWalletDebitPartiallyConsumesCheckinReward(t *testing.T) {
	setupTopupWalletTestDB(t)
	checkin := createWalletWithActiveCheckinReward(t, 31, 200, 100)

	adjustWalletForCheckinTest(t, 31, -30, "partial-checkin-debit", "relay")

	updated := loadCheckinReward(t, checkin.Id)
	require.EqualValues(t, 70, updated.QuotaRemaining)
	require.Zero(t, updated.SettledAt)
}

func TestWalletDebitFullyConsumesAndSettlesCheckinReward(t *testing.T) {
	setupTopupWalletTestDB(t)
	checkin := createWalletWithActiveCheckinReward(t, 32, 200, 100)

	adjustWalletForCheckinTest(t, 32, -30, "first-checkin-debit", "task")
	adjustWalletForCheckinTest(t, 32, -80, "second-checkin-debit", "task")

	updated := loadCheckinReward(t, checkin.Id)
	require.Zero(t, updated.QuotaRemaining)
	require.NotZero(t, updated.SettledAt)
}

func TestWalletCreditDoesNotRestoreConsumedCheckinReward(t *testing.T) {
	setupTopupWalletTestDB(t)
	checkin := createWalletWithActiveCheckinReward(t, 33, 200, 100)

	adjustWalletForCheckinTest(t, 33, -40, "consume-before-credit", "relay")
	adjustWalletForCheckinTest(t, 33, 25, "credit-after-consume", "topup")

	updated := loadCheckinReward(t, checkin.Id)
	require.EqualValues(t, 60, updated.QuotaRemaining)
	require.Zero(t, updated.SettledAt)
}

func TestWalletDebitCategoriesConsumeCheckinReward(t *testing.T) {
	categories := []string{"relay", "task", "user_redemption_reserve", "admin_adjust"}
	for index, category := range categories {
		t.Run(category, func(t *testing.T) {
			setupTopupWalletTestDB(t)
			userID := 40 + index
			checkin := createWalletWithActiveCheckinReward(t, userID, 200, 100)

			adjustWalletForCheckinTest(t, userID, -25, "category-"+category, category)

			updated := loadCheckinReward(t, checkin.Id)
			require.EqualValues(t, 75, updated.QuotaRemaining)
		})
	}
}

func expireCheckinRewardForTest(t *testing.T, checkinID int) {
	t.Helper()
	require.NoError(t, DB.Model(&Checkin{}).
		Where("id = ?", checkinID).
		UpdateColumn("expires_at", time.Now().Add(-time.Minute).Unix()).Error)
}

func loadWalletUser(t *testing.T, userID int) User {
	t.Helper()
	var user User
	require.NoError(t, DB.First(&user, userID).Error)
	return user
}

func TestSettleDueCheckinRewardReclaimsUnusedRemainder(t *testing.T) {
	setupTopupWalletTestDB(t)
	checkin := createWalletWithActiveCheckinReward(t, 51, 200, 60)
	expireCheckinRewardForTest(t, checkin.Id)

	settled, err := SettleDueCheckinRewards(10)

	require.NoError(t, err)
	require.Equal(t, 1, settled)
	require.EqualValues(t, 140, loadWalletUser(t, 51).Quota)
	updated := loadCheckinReward(t, checkin.Id)
	require.Zero(t, updated.QuotaRemaining)
	require.EqualValues(t, 60, updated.ReclaimedQuota)
	require.NotZero(t, updated.SettledAt)

	var entry WalletLedgerEntry
	require.NoError(t, DB.Where("operation_id = ?", fmt.Sprintf("checkin-expiry:%d", checkin.Id)).First(&entry).Error)
	require.Equal(t, "checkin_expiry", entry.Category)
	require.EqualValues(t, -60, entry.Delta)
	require.Equal(t, "checkin", entry.ReferenceType)
	require.Equal(t, checkin.CheckinDate, entry.ReferenceID)
	require.Equal(t, "unused_daily_reward", entry.Context)
}

func TestSettleDueCheckinRewardNeverCreatesNegativeBalance(t *testing.T) {
	setupTopupWalletTestDB(t)
	checkin := createWalletWithActiveCheckinReward(t, 52, 20, 60)
	expireCheckinRewardForTest(t, checkin.Id)

	settled, err := SettleDueCheckinRewards(10)

	require.NoError(t, err)
	require.Equal(t, 1, settled)
	require.Zero(t, loadWalletUser(t, 52).Quota)
	updated := loadCheckinReward(t, checkin.Id)
	require.Zero(t, updated.QuotaRemaining)
	require.EqualValues(t, 20, updated.ReclaimedQuota)
	require.NotZero(t, updated.SettledAt)
}

func TestSettleDueCheckinRewardsIsIdempotent(t *testing.T) {
	setupTopupWalletTestDB(t)
	checkin := createWalletWithActiveCheckinReward(t, 53, 200, 60)
	expireCheckinRewardForTest(t, checkin.Id)

	first, err := SettleDueCheckinRewards(10)
	require.NoError(t, err)
	second, err := SettleDueCheckinRewards(10)
	require.NoError(t, err)

	require.Equal(t, 1, first)
	require.Zero(t, second)
	require.EqualValues(t, 140, loadWalletUser(t, 53).Quota)
	var count int64
	require.NoError(t, DB.Model(&WalletLedgerEntry{}).
		Where("operation_id = ?", fmt.Sprintf("checkin-expiry:%d", checkin.Id)).
		Count(&count).Error)
	require.EqualValues(t, 1, count)
}

func TestSettleDueCheckinRewardAfterFullUseIsNoOp(t *testing.T) {
	setupTopupWalletTestDB(t)
	checkin := createWalletWithActiveCheckinReward(t, 54, 200, 60)
	adjustWalletForCheckinTest(t, 54, -60, "fully-use-checkin", "relay")
	expireCheckinRewardForTest(t, checkin.Id)

	settled, err := SettleDueCheckinRewards(10)

	require.NoError(t, err)
	require.Zero(t, settled)
	require.EqualValues(t, 140, loadWalletUser(t, 54).Quota)
	updated := loadCheckinReward(t, checkin.Id)
	require.Zero(t, updated.ReclaimedQuota)
}

func TestMidnightDebitSettlesExpiredRewardBeforeRequestedDebit(t *testing.T) {
	setupTopupWalletTestDB(t)
	checkin := createWalletWithActiveCheckinReward(t, 55, 200, 60)
	expireCheckinRewardForTest(t, checkin.Id)

	adjustWalletForCheckinTest(t, 55, -30, "after-midnight-debit", "relay")

	require.EqualValues(t, 110, loadWalletUser(t, 55).Quota)
	updated := loadCheckinReward(t, checkin.Id)
	require.Zero(t, updated.QuotaRemaining)
	require.EqualValues(t, 60, updated.ReclaimedQuota)
	require.NotZero(t, updated.SettledAt)
}

func TestWalletDebitLeavesPermanentHistoricalCheckinUntouched(t *testing.T) {
	setupTopupWalletTestDB(t)
	checkin := createWalletWithActiveCheckinReward(t, 56, 200, 60)
	require.NoError(t, DB.Model(&Checkin{}).Where("id = ?", checkin.Id).
		Updates(map[string]any{"expires_at": 0, "quota_remaining": 0}).Error)

	adjustWalletForCheckinTest(t, 56, -30, "permanent-checkin-debit", "relay")

	updated := loadCheckinReward(t, checkin.Id)
	require.Zero(t, updated.QuotaRemaining)
	require.Zero(t, updated.ExpiresAt)
	require.Zero(t, updated.SettledAt)
}

func TestOverrideWalletQuotaRecomputesDebitAfterExpiredRewardSettlement(t *testing.T) {
	setupTopupWalletTestDB(t)
	checkin := createWalletWithActiveCheckinReward(t, 57, 200, 60)
	expireCheckinRewardForTest(t, checkin.Id)

	var previous int64
	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		var err error
		previous, err = OverrideWalletQuotaWithTx(tx, 57, 150, "admin-expired-checkin-override")
		return err
	}))

	require.EqualValues(t, 200, previous)
	require.EqualValues(t, 150, loadWalletUser(t, 57).Quota)
	updated := loadCheckinReward(t, checkin.Id)
	require.EqualValues(t, 60, updated.ReclaimedQuota)
	require.NotZero(t, updated.SettledAt)
}
