package model

import (
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupPaymentRefundTest(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s/payment-refund.db?_pragma=busy_timeout(5000)", t.TempDir())), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&User{}, &TopUp{}, &PaymentRefund{}, &WalletLedgerEntry{}, &AuditOutboxEvent{}, &Checkin{},
		&SubscriptionOrder{}, &UserSubscription{}, &RelayFundingReservation{},
		&InviteRewardPayout{}, &InviteRewardReversal{}, &InviteRewardPayoutRequest{}, &InviteRewardPayoutRequestItem{},
	))
	previousDB := DB
	previousDatabaseType := common.MainDatabaseType()
	DB = db
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	t.Cleanup(func() {
		DB = previousDB
		common.SetMainDatabaseType(previousDatabaseType)
		require.NoError(t, sqlDB.Close())
	})
	return db
}

func seedRefundableTopUp(t *testing.T, db *gorm.DB, tradeNo string, quota, balance int64) TopUp {
	t.Helper()
	user := User{Id: 7101, Username: "refund-user", Quota: balance, Group: "default", Status: common.UserStatusEnabled}
	require.NoError(t, db.Create(&user).Error)
	topUp := TopUp{
		UserId: user.Id, Amount: quota, Money: 2.5, TradeNo: tradeNo,
		PaymentMethod: PaymentMethodCreem, PaymentProvider: PaymentProviderCreem,
		PaidAmount: "2.50", PaidCurrency: "USD", EligibleQuota: quota,
		Status: common.TopUpStatusSuccess,
	}
	require.NoError(t, db.Create(&topUp).Error)
	require.NoError(t, db.Create(&WalletLedgerEntry{
		UserID: user.Id, OperationID: "topup:" + tradeNo, Category: "topup", Delta: quota,
		BalanceAfter: quota, ReferenceType: "topup", ReferenceID: tradeNo,
	}).Error)
	return topUp
}

func processTestRefund(t *testing.T, tradeNo, sourceID, amount string) bool {
	t.Helper()
	created, err := ProcessPaymentRefund(PaymentCommissionReversalInput{
		TradeNo: tradeNo, ExpectedProvider: PaymentProviderCreem,
		RefundSourceType: "test_refund", RefundSourceID: sourceID,
		RefundedAmount: amount, RefundedCurrency: "USD",
	})
	require.NoError(t, err)
	return created
}

func TestProcessPaymentRefundUsesDecimalCumulativeRatio(t *testing.T) {
	db := setupPaymentRefundTest(t)
	topUp := seedRefundableTopUp(t, db, "decimal-refund", 1_000, 1_000)

	require.True(t, processTestRefund(t, topUp.TradeNo, "partial-1", "0.50"))
	require.True(t, processTestRefund(t, topUp.TradeNo, "partial-2", "0.75"))

	var user User
	require.NoError(t, db.First(&user, topUp.UserId).Error)
	require.Equal(t, int64(500), user.Quota)
	var refunds []PaymentRefund
	require.NoError(t, db.Where("trade_no = ?", topUp.TradeNo).Order("id asc").Find(&refunds).Error)
	require.Len(t, refunds, 2)
	require.Equal(t, int64(200), refunds[0].ReversedQuota)
	require.Equal(t, int64(300), refunds[1].ReversedQuota)

	require.True(t, processTestRefund(t, topUp.TradeNo, "over-refund", "10.00"))
	require.NoError(t, db.First(&user, topUp.UserId).Error)
	require.Zero(t, user.Quota)
	var cappedRefund PaymentRefund
	require.NoError(t, db.Where("source_id = ?", "over-refund").First(&cappedRefund).Error)
	require.Equal(t, "1.25", cappedRefund.AppliedAmount)
	require.Equal(t, int64(500), cappedRefund.ReversedQuota)
}

func TestProcessPaymentRefundIsIdempotentAndRejectsSourceConflict(t *testing.T) {
	db := setupPaymentRefundTest(t)
	topUp := seedRefundableTopUp(t, db, "idempotent-refund", 1_000, 1_000)

	require.True(t, processTestRefund(t, topUp.TradeNo, "same-source", "1.00"))
	require.False(t, processTestRefund(t, topUp.TradeNo, "same-source", "1.0"))
	_, err := ProcessPaymentRefund(PaymentCommissionReversalInput{
		TradeNo: topUp.TradeNo, ExpectedProvider: PaymentProviderCreem,
		RefundSourceType: "test_refund", RefundSourceID: "same-source",
		RefundedAmount: "1.01", RefundedCurrency: "USD",
	})
	require.ErrorIs(t, err, ErrPaymentRefundConflict)

	var user User
	require.NoError(t, db.First(&user, topUp.UserId).Error)
	require.Equal(t, int64(600), user.Quota)
	var count int64
	require.NoError(t, db.Model(&PaymentRefund{}).Count(&count).Error)
	require.Equal(t, int64(1), count)
}

func TestProcessPaymentRefundAllowsWalletDebt(t *testing.T) {
	db := setupPaymentRefundTest(t)
	topUp := seedRefundableTopUp(t, db, "debt-refund", 1_000, 100)

	require.True(t, processTestRefund(t, topUp.TradeNo, "full-refund", "2.50"))
	var user User
	require.NoError(t, db.First(&user, topUp.UserId).Error)
	require.Equal(t, int64(-900), user.Quota)
}

func TestProcessPaymentRefundReversesPaidInvitationCommission(t *testing.T) {
	db := setupPaymentRefundTest(t)
	topUp := seedRefundableTopUp(t, db, "commission-refund", 1_000, 1_000)
	inviter := User{Id: 7102, Username: "refund-inviter", AffCode: "refund-inviter-aff", Quota: 100, Status: common.UserStatusEnabled}
	require.NoError(t, db.Create(&inviter).Error)
	reward := InviteRewardPayout{
		TriggerType: InviteRewardTriggerCashTopUp, TriggerID: topUp.TradeNo,
		RecipientKind: InviteRewardRecipientInviter, RecipientUserID: inviter.Id,
		EligibleQuota: 1_000, RateBasisPoints: 1_000, Amount: 100,
		Status: InviteRewardStatusPaid, ReversalState: InviteRewardReversalNone,
	}
	require.NoError(t, db.Create(&reward).Error)

	require.True(t, processTestRefund(t, topUp.TradeNo, "commission-half", "1.25"))
	require.NoError(t, db.First(&inviter, inviter.Id).Error)
	require.Equal(t, int64(50), inviter.Quota)
	require.NoError(t, db.First(&reward, reward.ID).Error)
	require.Equal(t, int64(50), reward.ReversedAmount)
	require.Equal(t, InviteRewardReversalPartial, reward.ReversalState)
}

func TestProcessPaymentRefundCancelsSubscriptionAndRestoresGroup(t *testing.T) {
	db := setupPaymentRefundTest(t)
	user := User{Id: 7201, Username: "subscription-refund-user", Group: "vip", Status: common.UserStatusEnabled}
	require.NoError(t, db.Create(&user).Error)
	sub := UserSubscription{
		UserId: user.Id, PlanId: 81, AmountTotal: 5_000, Status: "active", Source: "order",
		StartTime: 100, EndTime: 9_999_999_999, UpgradeGroup: "vip", PrevUserGroup: "default",
	}
	require.NoError(t, db.Create(&sub).Error)
	order := SubscriptionOrder{
		UserId: user.Id, PlanId: sub.PlanId, Money: 2.5, TradeNo: "subscription-refund",
		PaymentProvider: PaymentProviderCreem, PaidAmount: "2.50", PaidCurrency: "USD",
		EligibleQuota: 1_000, Status: common.TopUpStatusSuccess, UserSubscriptionID: int64(sub.Id),
	}
	require.NoError(t, db.Create(&order).Error)
	require.NoError(t, db.Create(&TopUp{
		UserId: user.Id, TradeNo: order.TradeNo, PaymentProvider: PaymentProviderCreem,
		PaidAmount: order.PaidAmount, PaidCurrency: order.PaidCurrency, EligibleQuota: order.EligibleQuota,
		Status: common.TopUpStatusSuccess,
	}).Error)

	require.True(t, processTestRefund(t, order.TradeNo, "subscription-source", "0.50"))
	require.NoError(t, db.First(&sub, sub.Id).Error)
	require.Equal(t, "cancelled", sub.Status)
	require.NoError(t, db.First(&user, user.Id).Error)
	require.Equal(t, "default", user.Group)
	var refund PaymentRefund
	require.NoError(t, db.Where("source_id = ?", "subscription-source").First(&refund).Error)
	require.Zero(t, refund.ReversedQuota)
	require.Equal(t, int64(sub.Id), refund.UserSubscriptionID)
}
