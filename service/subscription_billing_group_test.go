package service

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func setupSubscriptionBillingGroupTest(t *testing.T) {
	t.Helper()
	truncate(t)
	if !model.DB.Migrator().HasTable(&model.SubscriptionPlan{}) {
		require.NoError(t, model.DB.AutoMigrate(&model.SubscriptionPlan{}))
	}
	if !model.DB.Migrator().HasTable(&model.SubscriptionPreConsumeRecord{}) {
		require.NoError(t, model.DB.AutoMigrate(&model.SubscriptionPreConsumeRecord{}))
	}
	if !model.DB.Migrator().HasTable(&model.Checkin{}) {
		require.NoError(t, model.DB.AutoMigrate(&model.Checkin{}))
	}
	if !model.DB.Migrator().HasTable(&model.RelayFundingReservation{}) {
		require.NoError(t, model.DB.AutoMigrate(&model.RelayFundingReservation{}))
	}
	t.Cleanup(func() {
		model.DB.Exec("DELETE FROM checkins")
		model.DB.Exec("DELETE FROM relay_funding_reservations")
		model.DB.Exec("DELETE FROM subscription_pre_consume_records")
		model.DB.Exec("DELETE FROM subscription_plans")
	})
}

func seedSubscriptionBillingPlan(t *testing.T, id int, title string) {
	t.Helper()
	require.NoError(t, model.DB.Create(&model.SubscriptionPlan{
		Id:               id,
		Title:            title,
		DurationUnit:     model.SubscriptionDurationMonth,
		DurationValue:    1,
		TotalAmount:      1000,
		QuotaResetPeriod: model.SubscriptionResetNever,
	}).Error)
}

func TestNewBillingSessionUsesOnlySubscriptionMatchingRequestGroup(t *testing.T) {
	setupSubscriptionBillingGroupTest(t)
	gin.SetMode(gin.TestMode)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())

	user := model.User{Id: 98300, Username: "group-billing-user", Quota: 0, Status: common.UserStatusEnabled, Group: "pro"}
	user.SetSetting(dto.UserSetting{BillingPreference: "subscription_only"})
	require.NoError(t, model.DB.Create(&user).Error)
	seedSubscriptionBillingPlan(t, 98301, "Standard")
	seedSubscriptionBillingPlan(t, 98302, "Pro")
	now := common.GetTimestamp()
	require.NoError(t, model.DB.Create(&model.UserSubscription{Id: 98311, UserId: user.Id, PlanId: 98301, AmountTotal: 1000, EndTime: now + 100, Status: "active", UpgradeGroup: "standard", AllowWalletOverflow: false}).Error)
	require.NoError(t, model.DB.Create(&model.UserSubscription{Id: 98312, UserId: user.Id, PlanId: 98302, AmountTotal: 1000, EndTime: now + 200, Status: "active", UpgradeGroup: "pro", AllowWalletOverflow: true}).Error)

	info := &relaycommon.RelayInfo{
		RequestId:       "billing-group-match-1",
		UserId:          user.Id,
		UserGroup:       user.Group,
		UsingGroup:      "pro",
		OriginModelName: "gpt-test",
		IsPlayground:    true,
		UserSetting:     user.GetSetting(),
	}
	session, apiErr := NewBillingSession(context, info, 100)
	require.Nil(t, apiErr)
	require.NotNil(t, session)
	require.Equal(t, 98312, info.SubscriptionId)
	require.NoError(t, session.Settle(100))
}

func TestSubscriptionFirstIgnoresUnrelatedStrictSubscriptionForWalletFallback(t *testing.T) {
	setupSubscriptionBillingGroupTest(t)
	gin.SetMode(gin.TestMode)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())

	user := model.User{Id: 98400, Username: "group-fallback-user", Quota: 500, Status: common.UserStatusEnabled, Group: "pro"}
	user.SetSetting(dto.UserSetting{BillingPreference: "subscription_first"})
	require.NoError(t, model.DB.Create(&user).Error)
	seedSubscriptionBillingPlan(t, 98401, "Strict Standard")
	now := common.GetTimestamp()
	require.NoError(t, model.DB.Create(&model.UserSubscription{Id: 98411, UserId: user.Id, PlanId: 98401, AmountTotal: 100, AmountUsed: 100, EndTime: now + 100, Status: "active", UpgradeGroup: "standard", AllowWalletOverflow: false}).Error)

	info := &relaycommon.RelayInfo{
		RequestId:       "billing-group-fallback-1",
		UserId:          user.Id,
		UserGroup:       user.Group,
		UsingGroup:      "pro",
		OriginModelName: "gpt-test",
		IsPlayground:    true,
		UserSetting:     user.GetSetting(),
	}
	session, apiErr := NewBillingSession(context, info, 100)
	require.Nil(t, apiErr)
	require.NotNil(t, session)
	require.Equal(t, BillingSourceWallet, info.BillingSource)
	require.NoError(t, session.Settle(100))
}

func TestSubscriptionOnlyDoesNotUseWalletToCompletePartialSubscription(t *testing.T) {
	setupSubscriptionBillingGroupTest(t)
	gin.SetMode(gin.TestMode)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())

	user := model.User{Id: 98500, Username: "subscription-only-user", Quota: 500, Status: common.UserStatusEnabled, Group: "pro"}
	user.SetSetting(dto.UserSetting{BillingPreference: "subscription_only"})
	require.NoError(t, model.DB.Create(&user).Error)
	seedSubscriptionBillingPlan(t, 98501, "Small Pro")
	now := common.GetTimestamp()
	subscription := model.UserSubscription{Id: 98511, UserId: user.Id, PlanId: 98501, AmountTotal: 40, EndTime: now + 100, Status: "active", UpgradeGroup: "pro", AllowWalletOverflow: true}
	require.NoError(t, model.DB.Create(&subscription).Error)

	info := &relaycommon.RelayInfo{
		RequestId:       "billing-no-partial-wallet-1",
		UserId:          user.Id,
		UserGroup:       user.Group,
		UsingGroup:      "pro",
		OriginModelName: "gpt-test",
		IsPlayground:    true,
		UserSetting:     user.GetSetting(),
	}
	session, apiErr := NewBillingSession(context, info, 100)

	require.Nil(t, session)
	require.NotNil(t, apiErr)

	var storedUser model.User
	var storedSubscription model.UserSubscription
	require.NoError(t, model.DB.First(&storedUser, user.Id).Error)
	require.NoError(t, model.DB.First(&storedSubscription, subscription.Id).Error)
	require.Equal(t, int64(500), storedUser.Quota)
	require.Zero(t, storedSubscription.AmountUsed)
}
