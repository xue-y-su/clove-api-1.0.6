package model

import (
	"errors"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func seedSubscriptionPurchaseLimitFixture(t *testing.T, userID, planID, limit int) *SubscriptionPlan {
	t.Helper()
	truncateTables(t)
	require.NoError(t, DB.Create(&User{
		Id: userID, Username: "purchase-limit-user", Status: common.UserStatusEnabled,
	}).Error)
	plan := &SubscriptionPlan{
		Id: planID, Title: "Lifetime limit", Enabled: true,
		DurationUnit: SubscriptionDurationMonth, DurationValue: 1,
		MaxPurchasePerUser: limit, AllowBalancePay: common.GetPointer(true),
		AllowWalletOverflow: common.GetPointer(true),
	}
	require.NoError(t, DB.Create(plan).Error)
	return plan
}

func TestSubscriptionPurchaseLimitSurvivesSubscriptionDeletion(t *testing.T) {
	plan := seedSubscriptionPurchaseLimitFixture(t, 3101, 3201, 1)
	first, err := CreateUserSubscriptionFromPlanTx(DB, 3101, plan, "admin")
	require.NoError(t, err)
	require.NoError(t, DB.Delete(first).Error)

	_, err = CreateUserSubscriptionFromPlanTx(DB, 3101, plan, "redemption")
	require.ErrorIs(t, err, ErrSubscriptionPurchaseLimit)
	count, err := CountSubscriptionPurchasesByPlan(3101, plan.Id)
	require.NoError(t, err)
	assert.EqualValues(t, 1, count)
}

func TestSubscriptionPurchaseLimitCountsAcquisitionsWhilePlanWasUnlimited(t *testing.T) {
	plan := seedSubscriptionPurchaseLimitFixture(t, 3102, 3202, 0)
	_, err := CreateUserSubscriptionFromPlanTx(DB, 3102, plan, "admin")
	require.NoError(t, err)
	_, err = CreateUserSubscriptionFromPlanTx(DB, 3102, plan, "redemption")
	require.NoError(t, err)

	plan.MaxPurchasePerUser = 2
	_, err = CreateUserSubscriptionFromPlanTx(DB, 3102, plan, "order")
	require.ErrorIs(t, err, ErrSubscriptionPurchaseLimit)
	count, err := CountSubscriptionPurchasesByPlan(3102, plan.Id)
	require.NoError(t, err)
	assert.EqualValues(t, 2, count)
}

func TestSubscriptionPurchaseLimitRaiseAllowsOnlyRemainingLifetimeSlot(t *testing.T) {
	plan := seedSubscriptionPurchaseLimitFixture(t, 3103, 3203, 1)
	_, err := CreateUserSubscriptionFromPlanTx(DB, 3103, plan, "admin")
	require.NoError(t, err)
	_, err = CreateUserSubscriptionFromPlanTx(DB, 3103, plan, "order")
	require.ErrorIs(t, err, ErrSubscriptionPurchaseLimit)

	plan.MaxPurchasePerUser = 2
	_, err = CreateUserSubscriptionFromPlanTx(DB, 3103, plan, "order")
	require.NoError(t, err)
	_, err = CreateUserSubscriptionFromPlanTx(DB, 3103, plan, "redemption")
	require.True(t, errors.Is(err, ErrSubscriptionPurchaseLimit))
}

func TestAdminBindSubscriptionUsesCurrentPlanLimitWhenCacheIsStale(t *testing.T) {
	plan := seedSubscriptionPurchaseLimitFixture(t, 3104, 3204, 0)
	InvalidateSubscriptionPlanCache(plan.Id)
	t.Cleanup(func() { InvalidateSubscriptionPlanCache(plan.Id) })

	cached, err := GetSubscriptionPlanById(plan.Id)
	require.NoError(t, err)
	require.Zero(t, cached.MaxPurchasePerUser)
	require.NoError(t, DB.Model(&SubscriptionPlan{}).
		Where("id = ?", plan.Id).
		Update("max_purchase_per_user", 1).Error)

	_, err = AdminBindSubscription(3104, plan.Id, "first")
	require.NoError(t, err)
	_, err = AdminBindSubscription(3104, plan.Id, "second")
	require.ErrorIs(t, err, ErrSubscriptionPurchaseLimit)

	count, err := CountSubscriptionPurchasesByPlan(3104, plan.Id)
	require.NoError(t, err)
	assert.EqualValues(t, 1, count)
}
