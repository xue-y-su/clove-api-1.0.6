package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubscriptionOrderSnapshotIsUsedAtCompletion(t *testing.T) {
	truncateTables(t)
	user := User{Id: 3301, Username: "snapshot-order-user", Group: "default", Status: common.UserStatusEnabled}
	require.NoError(t, DB.Create(&user).Error)
	allowOverflow := false
	plan := SubscriptionPlan{
		Id: 3302, Title: "Original plan", PriceAmount: 8.5, Currency: "USD", Enabled: true,
		DurationUnit: SubscriptionDurationDay, DurationValue: 7, TotalAmount: 12_345,
		QuotaResetPeriod: SubscriptionResetDaily, AllowWalletOverflow: &allowOverflow,
		UpgradeGroup: "vip", DowngradeGroup: "default", MaxPurchasePerUser: 2,
		StripePriceId: "price_original",
	}
	require.NoError(t, DB.Create(&plan).Error)
	order := SubscriptionOrder{
		UserId: user.Id, PlanId: plan.Id, TradeNo: "snapshot-order",
		PaymentMethod: PaymentMethodStripe, PaymentProvider: PaymentProviderStripe, ProviderStoreID: "STO_order_snapshot",
		Status: common.TopUpStatusPending,
	}
	require.NoError(t, order.Insert())
	assert.Equal(t, "Original plan", order.PlanTitle)
	assert.Equal(t, "price_original", order.ProviderProductID)
	assert.Equal(t, int64(12_345), order.TotalAmount)
	assert.False(t, order.AllowWalletOverflow)

	require.NoError(t, DB.Model(&SubscriptionPlan{}).Where("id = ?", plan.Id).Updates(map[string]any{
		"title": "Changed plan", "duration_unit": SubscriptionDurationMonth, "duration_value": 12,
		"total_amount": 99_999, "quota_reset_period": SubscriptionResetNever,
		"upgrade_group": "changed", "downgrade_group": "changed-back", "stripe_price_id": "price_changed",
	}).Error)

	require.NoError(t, CompleteSubscriptionOrder(order.TradeNo, "{}", PaymentProviderStripe, PaymentMethodStripe))
	require.NoError(t, DB.Where("trade_no = ?", order.TradeNo).First(&order).Error)
	require.Positive(t, order.UserSubscriptionID)
	var subscription UserSubscription
	require.NoError(t, DB.First(&subscription, order.UserSubscriptionID).Error)
	assert.Equal(t, int64(12_345), subscription.AmountTotal)
	assert.Equal(t, SubscriptionResetDaily, subscription.QuotaResetPeriod)
	assert.Equal(t, "vip", subscription.UpgradeGroup)
	assert.Equal(t, "default", subscription.DowngradeGroup)
	assert.False(t, subscription.AllowWalletOverflow)
	require.NoError(t, DB.First(&user, user.Id).Error)
	assert.Equal(t, "vip", user.Group)
	var topUp TopUp
	require.NoError(t, DB.Where("trade_no = ?", order.TradeNo).First(&topUp).Error)
	assert.Equal(t, "STO_order_snapshot", topUp.ProviderStoreID)
}

func TestSubscriptionOrderPendingReservationEnforcesPurchaseLimit(t *testing.T) {
	truncateTables(t)
	user := User{Id: 3311, Username: "pending-limit-user", Status: common.UserStatusEnabled}
	require.NoError(t, DB.Create(&user).Error)
	plan := SubscriptionPlan{
		Id: 3312, Title: "Limited plan", PriceAmount: 1, Currency: "USD", Enabled: true,
		DurationUnit: SubscriptionDurationMonth, DurationValue: 1, MaxPurchasePerUser: 1,
	}
	require.NoError(t, DB.Create(&plan).Error)
	first := SubscriptionOrder{
		UserId: user.Id, PlanId: plan.Id, TradeNo: "pending-limit-1",
		PaymentProvider: PaymentProviderCreem, Status: common.TopUpStatusPending,
	}
	require.NoError(t, first.Insert())
	second := SubscriptionOrder{
		UserId: user.Id, PlanId: plan.Id, TradeNo: "pending-limit-2",
		PaymentProvider: PaymentProviderCreem, Status: common.TopUpStatusPending,
	}
	require.ErrorIs(t, second.Insert(), ErrSubscriptionPurchaseLimit)

	require.NoError(t, ExpireSubscriptionOrder(first.TradeNo, PaymentProviderCreem))
	require.NoError(t, second.Insert())
}

func TestEpaySubscriptionOrderSnapshotsConvertedCNYAmount(t *testing.T) {
	truncateTables(t)
	previousPrice := operation_setting.Price
	operation_setting.Price = 7.2
	t.Cleanup(func() { operation_setting.Price = previousPrice })
	user := User{Id: 3321, Username: "epay-snapshot-user", Status: common.UserStatusEnabled}
	require.NoError(t, DB.Create(&user).Error)
	plan := SubscriptionPlan{
		Id: 3322, Title: "Epay plan", PriceAmount: 10, Currency: "USD", Enabled: true,
		DurationUnit: SubscriptionDurationMonth, DurationValue: 1,
	}
	require.NoError(t, DB.Create(&plan).Error)
	order := SubscriptionOrder{
		UserId: user.Id, PlanId: plan.Id, TradeNo: "epay-cny-order",
		PaymentMethod: "alipay", PaymentProvider: PaymentProviderEpay, Status: common.TopUpStatusPending,
	}
	require.NoError(t, order.Insert())
	assert.Equal(t, "72.00", order.PaidAmount)
	assert.Equal(t, "CNY", order.PaidCurrency)
	expectedQuota, err := CalculateCashPaymentEligibleQuota("72", "7.2")
	require.NoError(t, err)
	assert.Equal(t, expectedQuota, order.EligibleQuota)
}

func TestNonEpaySubscriptionSnapshotUsesConfiguredCurrencyRate(t *testing.T) {
	truncateTables(t)
	previousPrice := operation_setting.Price
	operation_setting.Price = 7.2
	t.Cleanup(func() { operation_setting.Price = previousPrice })
	user := User{Id: 3325, Username: "cny-snapshot-user", Status: common.UserStatusEnabled}
	require.NoError(t, DB.Create(&user).Error)
	plan := SubscriptionPlan{
		Id: 3326, Title: "CNY plan", PriceAmount: 72, Currency: "CNY", Enabled: true,
		DurationUnit: SubscriptionDurationMonth, DurationValue: 1,
	}
	require.NoError(t, DB.Create(&plan).Error)
	order := SubscriptionOrder{
		UserId: user.Id, PlanId: plan.Id, TradeNo: "creem-cny-order",
		PaymentProvider: PaymentProviderCreem, Status: common.TopUpStatusPending,
	}
	require.NoError(t, order.Insert())
	expectedQuota, err := CalculateCashPaymentEligibleQuota("72", "7.2")
	require.NoError(t, err)
	assert.Equal(t, expectedQuota, order.EligibleQuota)
}

func TestCashTopUpCreditedQuotaUsesProviderAmountUnits(t *testing.T) {
	creemQuota, err := cashTopUpCreditedQuota(&TopUp{Amount: 1_234, PaymentProvider: PaymentProviderCreem})
	require.NoError(t, err)
	assert.Equal(t, 1_234, creemQuota)

	unitQuota, err := cashTopUpCreditedQuota(&TopUp{Amount: 2, PaymentProvider: PaymentProviderEpay})
	require.NoError(t, err)
	expected, err := common.QuotaFromFloatStrict(2 * common.QuotaPerUnit)
	require.NoError(t, err)
	assert.Equal(t, expected, unitQuota)
}
