package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPreConsumeUserSubscriptionByEligibilitySkipsUnrelatedGroup(t *testing.T) {
	truncateTables(t)
	require.NoError(t, DB.AutoMigrate(&SubscriptionPreConsumeRecord{}))
	t.Cleanup(func() { DB.Exec("DELETE FROM subscription_pre_consume_records") })

	now := GetDBTimestamp()
	standardPlan := SubscriptionPlan{Id: 98101, Title: "Standard", DurationUnit: SubscriptionDurationMonth, DurationValue: 1, TotalAmount: 1000}
	proPlan := SubscriptionPlan{Id: 98102, Title: "Pro", DurationUnit: SubscriptionDurationMonth, DurationValue: 1, TotalAmount: 1000}
	require.NoError(t, DB.Create(&standardPlan).Error)
	require.NoError(t, DB.Create(&proPlan).Error)
	standardSub := UserSubscription{Id: 98111, UserId: 98100, PlanId: standardPlan.Id, AmountTotal: 1000, EndTime: now + 100, Status: "active", UpgradeGroup: "standard", AllowWalletOverflow: false}
	proSub := UserSubscription{Id: 98112, UserId: 98100, PlanId: proPlan.Id, AmountTotal: 1000, EndTime: now + 200, Status: "active", UpgradeGroup: "pro", AllowWalletOverflow: true}
	require.NoError(t, DB.Create(&standardSub).Error)
	require.NoError(t, DB.Create(&proSub).Error)

	proOnly := SubscriptionEligibility(func(upgradeGroup string) bool { return upgradeGroup == "pro" })
	result, err := PreConsumeUserSubscriptionByEligibility("group-request-1", 98100, "gpt-test", 0, 100, proOnly)
	require.NoError(t, err)
	require.Equal(t, proSub.Id, result.UserSubscriptionId)

	var storedStandard UserSubscription
	var storedPro UserSubscription
	require.NoError(t, DB.First(&storedStandard, standardSub.Id).Error)
	require.NoError(t, DB.First(&storedPro, proSub.Id).Error)
	require.Zero(t, storedStandard.AmountUsed)
	require.EqualValues(t, 100, storedPro.AmountUsed)
}

func TestActiveSubscriptionChecksByEligibilityIgnoreUnrelatedStrictSubscription(t *testing.T) {
	truncateTables(t)
	now := GetDBTimestamp()
	require.NoError(t, DB.Create(&UserSubscription{Id: 98211, UserId: 98200, PlanId: 1, AmountTotal: 1000, EndTime: now + 100, Status: "active", UpgradeGroup: "standard", AllowWalletOverflow: false}).Error)
	require.NoError(t, DB.Create(&UserSubscription{Id: 98212, UserId: 98200, PlanId: 2, AmountTotal: 1000, EndTime: now + 200, Status: "active", UpgradeGroup: "pro", AllowWalletOverflow: true}).Error)

	proOnly := SubscriptionEligibility(func(upgradeGroup string) bool { return upgradeGroup == "pro" })
	hasPro, err := HasActiveUserSubscriptionByEligibility(98200, proOnly)
	require.NoError(t, err)
	require.True(t, hasPro)

	allowOverflow, err := UserActiveSubscriptionsAllowWalletOverflowByEligibility(98200, proOnly)
	require.NoError(t, err)
	require.True(t, allowOverflow)

	none := SubscriptionEligibility(func(string) bool { return false })
	hasMatch, err := HasActiveUserSubscriptionByEligibility(98200, none)
	require.NoError(t, err)
	require.False(t, hasMatch)
}

func TestPreConsumeUserSubscriptionByEligibilityChoosesSubscriptionThatCoversRequest(t *testing.T) {
	truncateTables(t)
	require.NoError(t, DB.AutoMigrate(&SubscriptionPreConsumeRecord{}))
	t.Cleanup(func() { DB.Exec("DELETE FROM subscription_pre_consume_records") })

	now := GetDBTimestamp()
	firstPlan := SubscriptionPlan{Id: 98221, Title: "Small", DurationUnit: SubscriptionDurationMonth, DurationValue: 1, TotalAmount: 40}
	secondPlan := SubscriptionPlan{Id: 98222, Title: "Large", DurationUnit: SubscriptionDurationMonth, DurationValue: 1, TotalAmount: 100}
	require.NoError(t, DB.Create(&firstPlan).Error)
	require.NoError(t, DB.Create(&secondPlan).Error)
	firstSub := UserSubscription{Id: 98231, UserId: 98220, PlanId: firstPlan.Id, AmountTotal: 40, EndTime: now + 100, Status: "active", UpgradeGroup: "pro"}
	secondSub := UserSubscription{Id: 98232, UserId: 98220, PlanId: secondPlan.Id, AmountTotal: 100, EndTime: now + 200, Status: "active", UpgradeGroup: "pro"}
	require.NoError(t, DB.Create(&firstSub).Error)
	require.NoError(t, DB.Create(&secondSub).Error)

	proOnly := SubscriptionEligibility(func(upgradeGroup string) bool { return upgradeGroup == "pro" })
	result, err := PreConsumeUserSubscriptionByEligibility("group-request-cover-1", 98220, "gpt-test", 0, 60, proOnly)

	require.NoError(t, err)
	require.Equal(t, secondSub.Id, result.UserSubscriptionId)
	require.EqualValues(t, 60, result.PreConsumed)

	var storedFirst UserSubscription
	var storedSecond UserSubscription
	require.NoError(t, DB.First(&storedFirst, firstSub.Id).Error)
	require.NoError(t, DB.First(&storedSecond, secondSub.Id).Error)
	require.Zero(t, storedFirst.AmountUsed)
	require.EqualValues(t, 60, storedSecond.AmountUsed)
}
