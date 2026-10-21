package model

import (
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupTopUpCommissionTest(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s/topup-commission.db?_pragma=busy_timeout(5000)", t.TempDir())), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&User{}, &TopUp{}, &WalletLedgerEntry{}, &AuditOutboxEvent{}, &Checkin{},
		&InviteCode{}, &InviteCodeUse{}, &InviteRewardPayout{}, &InviteRewardReversal{},
	))
	previousDB := DB
	previousDatabaseType := common.MainDatabaseType()
	previousSetting := *operation_setting.GetInvitationIncentiveSetting()
	DB = db
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	setting := operation_setting.DefaultInvitationIncentiveSetting()
	setting.CashCommissionEnabled = true
	setting.CashCommissionRate = "10.00"
	*operation_setting.GetInvitationIncentiveSetting() = setting
	t.Cleanup(func() {
		DB = previousDB
		common.SetMainDatabaseType(previousDatabaseType)
		*operation_setting.GetInvitationIncentiveSetting() = previousSetting
		require.NoError(t, sqlDB.Close())
	})

	inviter := User{Id: 61, Username: "topup-inviter", AffCode: "topup-inviter-aff", Status: common.UserStatusEnabled}
	invitee := User{
		Id: 62, Username: "topup-invitee", AffCode: "topup-invitee-aff", Status: common.UserStatusEnabled,
		InviterId: inviter.Id, InviterSource: InviterSourceInviteCode,
	}
	require.NoError(t, db.Create(&inviter).Error)
	require.NoError(t, db.Create(&invitee).Error)
	code := InviteCode{
		OwnerUserID: inviter.Id, Code: "TOPUPCODE1", Kind: InviteCodeKindMulti,
		MaxUses: 20, UsedCount: 1, Status: InviteCodeStatusActive, IssuedBy: inviter.Id,
	}
	require.NoError(t, db.Create(&code).Error)
	require.NoError(t, db.Create(&InviteCodeUse{
		InviteCodeID: code.ID, InviterUserID: inviter.Id, RegisteredUserID: invitee.Id,
		Source: "password", InviterRewardStatus: InviteRewardStatusSkipped,
		InviteeRewardStatus: InviteRewardStatusSkipped, AuditEventID: "topup-commission-use",
	}).Error)
	return db
}

func TestCashPaymentEligibleQuotaUsesConfiguredCurrencyRate(t *testing.T) {
	previousPrice := operation_setting.Price
	previousCustomRate := operation_setting.GetGeneralSetting().CustomCurrencyExchangeRate
	t.Cleanup(func() {
		operation_setting.Price = previousPrice
		operation_setting.GetGeneralSetting().CustomCurrencyExchangeRate = previousCustomRate
	})
	operation_setting.Price = 7.2
	operation_setting.GetGeneralSetting().CustomCurrencyExchangeRate = 4

	cnyQuota, err := CalculateCashPaymentEligibleQuotaForCurrency("72.00", "CNY")
	require.NoError(t, err)
	usdQuota, err := CalculateCashPaymentEligibleQuota("10.00", "1")
	require.NoError(t, err)
	require.Equal(t, usdQuota, cnyQuota)
	customQuota, err := CalculateCashPaymentEligibleQuotaForCurrency("40.00", "CUSTOM")
	require.NoError(t, err)
	require.Equal(t, usdQuota, customQuota)
	_, err = CalculateCashPaymentEligibleQuotaForCurrency("10.00", "EUR")
	require.ErrorIs(t, err, ErrCashPaymentInvalid)
}

func TestReversePaymentInvitationCommissionUsesOriginalPaymentSnapshot(t *testing.T) {
	db := setupTopUpCommissionTest(t)
	previousPrice := operation_setting.Price
	operation_setting.Price = 7.2
	t.Cleanup(func() { operation_setting.Price = previousPrice })
	eligibleQuota, err := CalculateCashPaymentEligibleQuotaForCurrency("72.00", "CNY")
	require.NoError(t, err)
	topUp := TopUp{
		UserId: 62, TradeNo: "cny-refund-order", PaymentProvider: PaymentProviderCreem,
		EligibleQuota: eligibleQuota, PaidAmount: "72.00", PaidCurrency: "CNY", Status: common.TopUpStatusSuccess,
	}
	require.NoError(t, db.Create(&topUp).Error)
	use := InviteCodeUse{}
	require.NoError(t, db.Where("registered_user_id = ?", 62).First(&use).Error)
	reward := InviteRewardPayout{
		InviteCodeUseID: use.ID, TriggerType: InviteRewardTriggerCashTopUp, TriggerID: topUp.TradeNo,
		RecipientKind: InviteRewardRecipientInviter, RecipientUserID: 61,
		EligibleQuota: eligibleQuota, RateBasisPoints: 1_000, Amount: eligibleQuota / 10,
		ReversalState: InviteRewardReversalNone, Status: InviteRewardStatusAvailable,
	}
	require.NoError(t, db.Create(&reward).Error)

	created, err := ReversePaymentInvitationCommission(PaymentCommissionReversalInput{
		TradeNo: topUp.TradeNo, ExpectedProvider: PaymentProviderCreem,
		RefundSourceType: "creem_refund", RefundSourceID: "refund-cny-half",
		RefundedAmount: "36.00", RefundedCurrency: "CNY",
	})
	require.NoError(t, err)
	require.True(t, created)
	require.NoError(t, db.First(&reward, reward.ID).Error)
	require.Equal(t, reward.Amount/2, reward.ReversedAmount)

	created, err = ReversePaymentInvitationCommission(PaymentCommissionReversalInput{
		TradeNo: topUp.TradeNo, ExpectedProvider: PaymentProviderCreem,
		RefundSourceType: "creem_refund", RefundSourceID: "refund-cny-half",
		RefundedAmount: "36.00", RefundedCurrency: "CNY",
	})
	require.NoError(t, err)
	require.False(t, created)
}

func TestReversePaymentInvitationCommissionSkipsOrderWithoutReward(t *testing.T) {
	db := setupTopUpCommissionTest(t)
	require.NoError(t, db.Create(&TopUp{
		UserId: 62, TradeNo: "refund-without-commission", PaymentProvider: PaymentProviderWaffo,
		EligibleQuota: 500_000, PaidAmount: "1.00", PaidCurrency: "USD", Status: common.TopUpStatusSuccess,
	}).Error)
	created, err := ReversePaymentInvitationCommission(PaymentCommissionReversalInput{
		TradeNo: "refund-without-commission", ExpectedProvider: PaymentProviderWaffo,
		RefundSourceType: "waffo_refund", RefundSourceID: "refund-no-reward",
		RefundedAmount: "1.00", RefundedCurrency: "USD",
	})
	require.NoError(t, err)
	require.False(t, created)
}

func TestCompleteCashTopUpCommissionUsesPaidBasisAndReplaysOnce(t *testing.T) {
	db := setupTopUpCommissionTest(t)
	topUp := TopUp{
		UserId: 62, Amount: 1, Money: 1, TradeNo: "discounted-topup",
		PaymentMethod: PaymentMethodStripe, PaymentProvider: PaymentProviderStripe,
		EligibleQuota: 900, PaidAmount: "0.0018", PaidCurrency: "USD",
		Status: common.TopUpStatusPending,
	}
	require.NoError(t, db.Create(&topUp).Error)

	for range 2 {
		require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
			return CompleteCashTopUpWithCommissionTx(tx, &topUp, 1_000, PaymentProviderStripe, nil)
		}))
	}

	var user User
	require.NoError(t, db.First(&user, 62).Error)
	require.Equal(t, int64(1_000), user.Quota)
	var reward InviteRewardPayout
	require.NoError(t, db.Where("trigger_type = ? AND trigger_id = ?", InviteRewardTriggerCashTopUp, topUp.TradeNo).First(&reward).Error)
	require.Equal(t, int64(900), reward.EligibleQuota)
	require.Equal(t, int64(90), reward.Amount)
	var rewardCount, commissionEventCount int64
	require.NoError(t, db.Model(&InviteRewardPayout{}).Where("trigger_type = ?", InviteRewardTriggerCashTopUp).Count(&rewardCount).Error)
	require.NoError(t, db.Model(&AuditOutboxEvent{}).Where("event_type = ?", "invitation.commission_created").Count(&commissionEventCount).Error)
	require.Equal(t, int64(1), rewardCount)
	require.Equal(t, int64(1), commissionEventCount)
}

func TestCompleteCashTopUpCommissionUsesVerifiedSettlement(t *testing.T) {
	db := setupTopUpCommissionTest(t)
	topUp := TopUp{
		UserId: 62, Amount: 1, Money: 10, TradeNo: "settled-topup",
		PaymentMethod: PaymentMethodStripe, PaymentProvider: PaymentProviderStripe,
		EligibleQuota: 1_000, PaidAmount: "10.00", PaidCurrency: "USD",
		Status: common.TopUpStatusPending,
	}
	require.NoError(t, db.Create(&topUp).Error)
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		return CompleteCashTopUpWithCommissionTx(tx, &topUp, 1_000, PaymentProviderStripe, &CashPaymentSettlement{
			PaidAmount: "8.50", PaidCurrency: "USD",
		})
	}))

	var stored TopUp
	require.NoError(t, db.First(&stored, topUp.Id).Error)
	require.Equal(t, "8.50", stored.PaidAmount)
	require.Equal(t, "USD", stored.PaidCurrency)
	require.Equal(t, int64(850), stored.EligibleQuota)
	var reward InviteRewardPayout
	require.NoError(t, db.Where("trigger_id = ?", topUp.TradeNo).First(&reward).Error)
	require.Equal(t, int64(850), reward.EligibleQuota)
	require.Equal(t, int64(85), reward.Amount)
}

func TestCompleteCashTopUpCommissionCoversProvidersAndExcludesInternalCredits(t *testing.T) {
	db := setupTopUpCommissionTest(t)
	providers := []string{
		PaymentProviderEpay, PaymentProviderStripe, PaymentProviderCreem,
		PaymentProviderWaffo, PaymentProviderWaffoPancake,
	}
	for index, provider := range providers {
		topUp := TopUp{
			UserId: 62, TradeNo: fmt.Sprintf("provider-%d", index), PaymentProvider: provider,
			EligibleQuota: 1_000, PaidAmount: "1.00", PaidCurrency: "USD",
		}
		require.NoError(t, db.Create(&topUp).Error)
		require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
			return CompleteCashTopUpWithCommissionTx(tx, &topUp, 1_000, provider, nil)
		}))
	}
	for index, provider := range []string{PaymentProviderBalance, "admin"} {
		topUp := TopUp{
			UserId: 62, TradeNo: fmt.Sprintf("internal-%d", index), PaymentProvider: provider,
			EligibleQuota: 1_000, PaidAmount: "1.00", PaidCurrency: "USD",
		}
		require.NoError(t, db.Create(&topUp).Error)
		require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
			return CompleteCashTopUpWithCommissionTx(tx, &topUp, 1_000, provider, nil)
		}))
	}

	var rewardCount int64
	require.NoError(t, db.Model(&InviteRewardPayout{}).Where("trigger_type = ?", InviteRewardTriggerCashTopUp).Count(&rewardCount).Error)
	require.Equal(t, int64(len(providers)), rewardCount)
	var ledgerCount int64
	require.NoError(t, db.Model(&WalletLedgerEntry{}).Where("category = ?", "topup").Count(&ledgerCount).Error)
	require.Equal(t, int64(len(providers)+2), ledgerCount)
}
