package service

import (
	"errors"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newBillingReservationContext() *gin.Context {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	return ctx
}

func setupBillingReservationTest(t *testing.T) {
	t.Helper()
	setupSubscriptionBillingGroupTest(t)
	require.NoError(t, model.DB.AutoMigrate(&model.RelayFundingReservation{}))
	t.Cleanup(func() { model.DB.Exec("DELETE FROM relay_funding_reservations") })
}

func seedBillingReservationUser(t *testing.T, id int, quota int64, preference string) model.User {
	t.Helper()
	user := model.User{
		Id:       id,
		Username: "billing-reservation-user",
		Quota:    quota,
		Status:   common.UserStatusEnabled,
		Group:    "pro",
	}
	user.SetSetting(dto.UserSetting{BillingPreference: preference})
	require.NoError(t, model.DB.Create(&user).Error)
	return user
}

func newBillingReservationInfo(user model.User, requestID string) *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		RequestId:       requestID,
		UserId:          user.Id,
		UserGroup:       user.Group,
		UsingGroup:      user.Group,
		OriginModelName: "gpt-billing-test",
		IsPlayground:    true,
		UserSetting:     user.GetSetting(),
	}
}

func TestZeroQuotaWalletSessionAllowsZeroBalance(t *testing.T) {
	setupBillingReservationTest(t)
	user := seedBillingReservationUser(t, 98600, 0, "wallet_only")
	info := newBillingReservationInfo(user, "billing-zero-wallet")

	session, apiErr := NewBillingSession(newBillingReservationContext(), info, 0)

	require.Nil(t, apiErr)
	require.NotNil(t, session)
	require.Zero(t, session.GetPreConsumedQuota())
	var ledgerCount int64
	require.NoError(t, model.DB.Model(&model.WalletLedgerEntry{}).
		Where("reference_type = ? AND reference_id = ?", "relay_request", info.RequestId).
		Count(&ledgerCount).Error)
	require.Zero(t, ledgerCount)
}

func TestNewBillingSessionRetriesTransientWalletReservationConflict(t *testing.T) {
	setupBillingReservationTest(t)
	user := seedBillingReservationUser(t, 98605, 100, "wallet_only")
	token := model.Token{
		Id:          98606,
		UserId:      user.Id,
		Key:         "billing-concurrent-reservation",
		Name:        "billing-concurrent-reservation",
		Status:      common.TokenStatusEnabled,
		RemainQuota: 100,
	}
	require.NoError(t, model.DB.Create(&token).Error)
	info := newBillingReservationInfo(user, "billing-concurrent-reservation")
	info.IsPlayground = false
	info.TokenId = token.Id
	info.TokenKey = token.Key

	var attempts atomic.Int32
	const callbackName = "test:billing_concurrent_reservation"
	require.NoError(t, model.DB.Callback().Update().Before("gorm:update").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement.Schema != nil && tx.Statement.Schema.Name == "User" && attempts.Add(1) == 1 {
			tx.AddError(model.ErrWalletConcurrentMutation)
		}
	}))
	defer func() { require.NoError(t, model.DB.Callback().Update().Remove(callbackName)) }()

	session, apiErr := NewBillingSession(newBillingReservationContext(), info, 20)
	require.Nil(t, apiErr)
	require.NotNil(t, session)
	require.EqualValues(t, 2, attempts.Load())

	var storedUser model.User
	var storedToken model.Token
	require.NoError(t, model.DB.First(&storedUser, user.Id).Error)
	require.NoError(t, model.DB.First(&storedToken, token.Id).Error)
	require.EqualValues(t, 80, storedUser.Quota)
	require.Equal(t, 80, storedToken.RemainQuota)
	require.Equal(t, 20, storedToken.UsedQuota)

	var reservationEntries int64
	require.NoError(t, model.DB.Model(&model.WalletLedgerEntry{}).
		Where("reference_type = ? AND reference_id = ? AND context = ?", "relay_request", info.RequestId, "pre").
		Count(&reservationEntries).Error)
	require.EqualValues(t, 1, reservationEntries)
}

func TestTrustedSessionReserveToTargetLocksRealFundsOnce(t *testing.T) {
	setupBillingReservationTest(t)
	user := seedBillingReservationUser(t, 98610, 100, "wallet_only")
	info := newBillingReservationInfo(user, "billing-trusted-reserve")
	session := &BillingSession{
		relayInfo: info,
		funding:   NewWalletFunding(user.Id, info.RequestId),
		trusted:   true,
	}

	require.NoError(t, session.ReserveToTarget(25))
	require.NoError(t, session.ReserveToTarget(25))
	require.Equal(t, 25, session.GetPreConsumedQuota())

	var stored model.User
	require.NoError(t, model.DB.First(&stored, user.Id).Error)
	require.EqualValues(t, 75, stored.Quota)
	var entries []model.WalletLedgerEntry
	require.NoError(t, model.DB.Where("reference_type = ? AND reference_id = ?", "relay_request", info.RequestId).Find(&entries).Error)
	require.Len(t, entries, 1)
	require.EqualValues(t, -25, entries[0].Delta)
}

func TestSubscriptionFirstReservesSubscriptionAndWalletOverflowAtomically(t *testing.T) {
	setupBillingReservationTest(t)
	user := seedBillingReservationUser(t, 98620, 100, "subscription_first")
	seedSubscriptionBillingPlan(t, 98621, "Overflow Pro")
	subscription := model.UserSubscription{
		Id:                  98622,
		UserId:              user.Id,
		PlanId:              98621,
		AmountTotal:         40,
		EndTime:             common.GetTimestamp() + 3600,
		Status:              "active",
		UpgradeGroup:        "pro",
		AllowWalletOverflow: true,
	}
	require.NoError(t, model.DB.Create(&subscription).Error)
	info := newBillingReservationInfo(user, "billing-mixed-reserve")

	session, apiErr := NewBillingSession(newBillingReservationContext(), info, 100)

	require.Nil(t, apiErr)
	require.NotNil(t, session)
	require.Equal(t, 100, session.GetPreConsumedQuota())
	require.Equal(t, BillingSourceSubscription, info.BillingSource)
	require.Equal(t, subscription.Id, info.SubscriptionId)
	require.EqualValues(t, 40, info.SubscriptionPreConsumed)

	var storedUser model.User
	var storedSubscription model.UserSubscription
	require.NoError(t, model.DB.First(&storedUser, user.Id).Error)
	require.NoError(t, model.DB.First(&storedSubscription, subscription.Id).Error)
	require.EqualValues(t, 40, storedSubscription.AmountUsed)
	require.EqualValues(t, 40, storedUser.Quota)

	require.NoError(t, session.ReserveToTarget(120))
	require.NoError(t, session.ReserveToTarget(120))
	require.NoError(t, model.DB.First(&storedUser, user.Id).Error)
	require.EqualValues(t, 20, storedUser.Quota)
}

func TestSubscriptionFirstDoesNotOverflowWhenPlanDisallowsIt(t *testing.T) {
	setupBillingReservationTest(t)
	user := seedBillingReservationUser(t, 98630, 100, "subscription_first")
	seedSubscriptionBillingPlan(t, 98631, "Strict Pro")
	subscription := model.UserSubscription{
		Id:                  98632,
		UserId:              user.Id,
		PlanId:              98631,
		AmountTotal:         40,
		EndTime:             common.GetTimestamp() + 3600,
		Status:              "active",
		UpgradeGroup:        "pro",
		AllowWalletOverflow: false,
	}
	require.NoError(t, model.DB.Create(&subscription).Error)
	info := newBillingReservationInfo(user, "billing-strict-reserve")

	session, apiErr := NewBillingSession(newBillingReservationContext(), info, 100)

	require.Nil(t, session)
	require.NotNil(t, apiErr)
	require.Equal(t, types.ErrorCodeInsufficientUserQuota, apiErr.GetErrorCode())
	var storedUser model.User
	var storedSubscription model.UserSubscription
	require.NoError(t, model.DB.First(&storedUser, user.Id).Error)
	require.NoError(t, model.DB.First(&storedSubscription, subscription.Id).Error)
	require.EqualValues(t, 100, storedUser.Quota)
	require.Zero(t, storedSubscription.AmountUsed)
}

func TestSubscriptionOnlySkipsEarlierExhaustedSubscription(t *testing.T) {
	setupBillingReservationTest(t)
	user := seedBillingReservationUser(t, 98800, 0, "subscription_only")
	seedSubscriptionBillingPlan(t, 98801, "Exhausted Pro")
	seedSubscriptionBillingPlan(t, 98802, "Available Pro")
	now := common.GetTimestamp()
	exhausted := model.UserSubscription{
		Id: 98811, UserId: user.Id, PlanId: 98801, AmountTotal: 100, AmountUsed: 100,
		EndTime: now + 100, Status: "active", UpgradeGroup: "pro",
	}
	available := model.UserSubscription{
		Id: 98812, UserId: user.Id, PlanId: 98802, AmountTotal: 100, AmountUsed: 10,
		EndTime: now + 200, Status: "active", UpgradeGroup: "pro",
	}
	require.NoError(t, model.DB.Create(&exhausted).Error)
	require.NoError(t, model.DB.Create(&available).Error)
	info := newBillingReservationInfo(user, "billing-skip-exhausted")

	session, apiErr := NewBillingSession(newBillingReservationContext(), info, 50)

	require.Nil(t, apiErr)
	require.NotNil(t, session)
	require.Equal(t, available.Id, info.SubscriptionId)
	var storedExhausted, storedAvailable model.UserSubscription
	require.NoError(t, model.DB.First(&storedExhausted, exhausted.Id).Error)
	require.NoError(t, model.DB.First(&storedAvailable, available.Id).Error)
	require.EqualValues(t, 100, storedExhausted.AmountUsed)
	require.EqualValues(t, 60, storedAvailable.AmountUsed)
}

func TestSubscriptionFirstUsesLargestPartialSubscription(t *testing.T) {
	setupBillingReservationTest(t)
	user := seedBillingReservationUser(t, 98820, 100, "subscription_first")
	seedSubscriptionBillingPlan(t, 98821, "Small Partial Pro")
	seedSubscriptionBillingPlan(t, 98822, "Large Partial Pro")
	now := common.GetTimestamp()
	small := model.UserSubscription{
		Id: 98831, UserId: user.Id, PlanId: 98821, AmountTotal: 100, AmountUsed: 90,
		EndTime: now + 100, Status: "active", UpgradeGroup: "pro", AllowWalletOverflow: true,
	}
	large := model.UserSubscription{
		Id: 98832, UserId: user.Id, PlanId: 98822, AmountTotal: 100, AmountUsed: 60,
		EndTime: now + 200, Status: "active", UpgradeGroup: "pro", AllowWalletOverflow: true,
	}
	require.NoError(t, model.DB.Create(&small).Error)
	require.NoError(t, model.DB.Create(&large).Error)
	info := newBillingReservationInfo(user, "billing-largest-partial")

	session, apiErr := NewBillingSession(newBillingReservationContext(), info, 60)

	require.Nil(t, apiErr)
	require.NotNil(t, session)
	require.Equal(t, large.Id, info.SubscriptionId)
	require.EqualValues(t, 40, info.SubscriptionPreConsumed)
	var storedUser model.User
	require.NoError(t, model.DB.First(&storedUser, user.Id).Error)
	require.EqualValues(t, 80, storedUser.Quota)
}

func TestMixedReservationFailureRollsBackBothFundingSources(t *testing.T) {
	setupBillingReservationTest(t)
	user := seedBillingReservationUser(t, 98640, 50, "subscription_first")
	seedSubscriptionBillingPlan(t, 98641, "Rollback Pro")
	subscription := model.UserSubscription{
		Id:                  98642,
		UserId:              user.Id,
		PlanId:              98641,
		AmountTotal:         40,
		EndTime:             common.GetTimestamp() + 3600,
		Status:              "active",
		UpgradeGroup:        "pro",
		AllowWalletOverflow: true,
	}
	require.NoError(t, model.DB.Create(&subscription).Error)
	info := newBillingReservationInfo(user, "billing-mixed-rollback")

	session, apiErr := NewBillingSession(newBillingReservationContext(), info, 100)

	require.Nil(t, session)
	require.NotNil(t, apiErr)
	require.Equal(t, types.ErrorCodeInsufficientUserQuota, apiErr.GetErrorCode())
	var storedUser model.User
	var storedSubscription model.UserSubscription
	require.NoError(t, model.DB.First(&storedUser, user.Id).Error)
	require.NoError(t, model.DB.First(&storedSubscription, subscription.Id).Error)
	require.EqualValues(t, 50, storedUser.Quota)
	require.Zero(t, storedSubscription.AmountUsed)
	var reservationCount int64
	require.NoError(t, model.DB.Model(&model.RelayFundingReservation{}).
		Where("request_id = ?", info.RequestId).Count(&reservationCount).Error)
	require.Zero(t, reservationCount)
}

func TestMixedReservationRefundRestoresSubscriptionAndWalletTogether(t *testing.T) {
	setupBillingReservationTest(t)
	user := seedBillingReservationUser(t, 98650, 100, "subscription_first")
	seedSubscriptionBillingPlan(t, 98651, "Refund Pro")
	subscription := model.UserSubscription{
		Id:                  98652,
		UserId:              user.Id,
		PlanId:              98651,
		AmountTotal:         40,
		EndTime:             common.GetTimestamp() + 3600,
		Status:              "active",
		UpgradeGroup:        "pro",
		AllowWalletOverflow: true,
	}
	require.NoError(t, model.DB.Create(&subscription).Error)
	ctx := newBillingReservationContext()
	info := newBillingReservationInfo(user, "billing-mixed-refund")
	session, apiErr := NewBillingSession(ctx, info, 100)
	require.Nil(t, apiErr)
	require.NotNil(t, session)

	session.Refund(ctx)
	require.Eventually(t, func() bool {
		var storedUser model.User
		var storedSubscription model.UserSubscription
		if model.DB.First(&storedUser, user.Id).Error != nil || model.DB.First(&storedSubscription, subscription.Id).Error != nil {
			return false
		}
		return storedUser.Quota == 100 && storedSubscription.AmountUsed == 0
	}, 3*time.Second, 20*time.Millisecond)

	var reservation model.RelayFundingReservation
	require.NoError(t, model.DB.Where("request_id = ?", info.RequestId).First(&reservation).Error)
	require.Equal(t, "refunded", reservation.Status)
	require.Zero(t, reservation.SubscriptionQuota)
	require.Zero(t, reservation.WalletQuota)
}

func TestRefundAfterReserveToTargetDoesNotSubtractUnrelatedSubscriptionUsage(t *testing.T) {
	setupBillingReservationTest(t)
	user := seedBillingReservationUser(t, 98660, 100, "subscription_only")
	seedSubscriptionBillingPlan(t, 98661, "Shared Usage Pro")
	subscription := model.UserSubscription{
		Id:                  98662,
		UserId:              user.Id,
		PlanId:              98661,
		AmountTotal:         1000,
		EndTime:             common.GetTimestamp() + 3600,
		Status:              "active",
		UpgradeGroup:        "pro",
		AllowWalletOverflow: false,
	}
	require.NoError(t, model.DB.Create(&subscription).Error)
	ctx := newBillingReservationContext()
	info := newBillingReservationInfo(user, "billing-extra-reserve-refund")
	session, apiErr := NewBillingSession(ctx, info, 100)
	require.Nil(t, apiErr)
	require.NoError(t, session.ReserveToTarget(150))
	require.NoError(t, model.PostConsumeUserSubscriptionDelta(subscription.Id, 50))

	session.Refund(ctx)
	require.Eventually(t, func() bool {
		var stored model.UserSubscription
		return model.DB.First(&stored, subscription.Id).Error == nil && stored.AmountUsed == 50
	}, 3*time.Second, 20*time.Millisecond)
}

func TestSettleRejectsSessionAfterRefundStarts(t *testing.T) {
	setupBillingReservationTest(t)
	user := seedBillingReservationUser(t, 98670, 100, "wallet_only")
	ctx := newBillingReservationContext()
	info := newBillingReservationInfo(user, "billing-refund-settle-race")
	session, apiErr := NewBillingSession(ctx, info, 25)
	require.Nil(t, apiErr)

	session.Refund(ctx)
	err := session.Settle(25)

	require.ErrorIs(t, err, ErrBillingSessionRefunded)
}

func TestBillingSessionSettlementRollsBackTokenWhenFundingAndCompensationFail(t *testing.T) {
	setupBillingReservationTest(t)
	user := seedBillingReservationUser(t, 98680, 100, "wallet_only")
	token := model.Token{
		Id:          98681,
		UserId:      user.Id,
		Key:         "billing-atomic-token",
		Name:        "billing-atomic-token",
		Status:      common.TokenStatusEnabled,
		RemainQuota: 100,
	}
	require.NoError(t, model.DB.Create(&token).Error)
	ctx := newBillingReservationContext()
	info := newBillingReservationInfo(user, "billing-atomic-settle")
	info.IsPlayground = false
	info.TokenId = token.Id
	info.TokenKey = token.Key
	session, apiErr := NewBillingSession(ctx, info, 20)
	require.Nil(t, apiErr)

	fundingErr := errors.New("forced funding settlement failure")
	const fundingCallback = "test:billing_atomic_funding_failure"
	require.NoError(t, model.DB.Callback().Create().Before("gorm:create").Register(fundingCallback, func(tx *gorm.DB) {
		if tx.Statement.Schema != nil && tx.Statement.Schema.Name == "WalletLedgerEntry" {
			tx.AddError(fundingErr)
		}
	}))
	defer func() { require.NoError(t, model.DB.Callback().Create().Remove(fundingCallback)) }()

	rollbackErr := errors.New("forced token compensation failure")
	var tokenUpdates atomic.Int32
	const tokenCallback = "test:billing_atomic_token_compensation_failure"
	require.NoError(t, model.DB.Callback().Update().Before("gorm:update").Register(tokenCallback, func(tx *gorm.DB) {
		if tx.Statement.Schema != nil && tx.Statement.Schema.Name == "Token" && tokenUpdates.Add(1) > 1 {
			tx.AddError(rollbackErr)
		}
	}))
	defer func() { require.NoError(t, model.DB.Callback().Update().Remove(tokenCallback)) }()

	err := session.Settle(25)

	require.ErrorIs(t, err, fundingErr)
	var storedUser model.User
	var storedToken model.Token
	require.NoError(t, model.DB.First(&storedUser, user.Id).Error)
	require.NoError(t, model.DB.First(&storedToken, token.Id).Error)
	require.EqualValues(t, 80, storedUser.Quota)
	require.Equal(t, 80, storedToken.RemainQuota,
		"a failed settlement must not depend on a second best-effort token compensation")
	require.Equal(t, 20, storedToken.UsedQuota)
	require.True(t, session.NeedsRefund())
}

func TestBillingSessionRetriesTransientWalletSettlementConflict(t *testing.T) {
	setupBillingReservationTest(t)
	user := seedBillingReservationUser(t, 98735, 100, "wallet_only")
	token := model.Token{
		Id:          98736,
		UserId:      user.Id,
		Key:         "billing-concurrent-settlement",
		Name:        "billing-concurrent-settlement",
		Status:      common.TokenStatusEnabled,
		RemainQuota: 100,
	}
	require.NoError(t, model.DB.Create(&token).Error)
	info := newBillingReservationInfo(user, "billing-concurrent-settlement")
	info.IsPlayground = false
	info.TokenId = token.Id
	info.TokenKey = token.Key
	session, apiErr := NewBillingSession(newBillingReservationContext(), info, 20)
	require.Nil(t, apiErr)
	require.NotNil(t, session)

	var attempts atomic.Int32
	const callbackName = "test:billing_concurrent_settlement"
	require.NoError(t, model.DB.Callback().Update().Before("gorm:update").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement.Schema != nil && tx.Statement.Schema.Name == "User" && attempts.Add(1) == 1 {
			tx.AddError(model.ErrWalletConcurrentMutation)
		}
	}))
	defer func() { require.NoError(t, model.DB.Callback().Update().Remove(callbackName)) }()

	require.NoError(t, session.Settle(25))
	require.EqualValues(t, 2, attempts.Load())

	var storedUser model.User
	var storedToken model.Token
	require.NoError(t, model.DB.First(&storedUser, user.Id).Error)
	require.NoError(t, model.DB.First(&storedToken, token.Id).Error)
	require.EqualValues(t, 75, storedUser.Quota)
	require.Equal(t, 75, storedToken.RemainQuota)
	require.Equal(t, 25, storedToken.UsedQuota)

	var settlementEntries int64
	require.NoError(t, model.DB.Model(&model.WalletLedgerEntry{}).
		Where("reference_type = ? AND reference_id = ? AND context = ?", "relay_request", info.RequestId, "settle_debit").
		Count(&settlementEntries).Error)
	require.EqualValues(t, 1, settlementEntries)
}

func TestBillingSessionStopsAfterWalletSettlementConflictRetryBudget(t *testing.T) {
	setupBillingReservationTest(t)
	user := seedBillingReservationUser(t, 98737, 100, "wallet_only")
	token := model.Token{
		Id:          98738,
		UserId:      user.Id,
		Key:         "billing-persistent-settlement-conflict",
		Name:        "billing-persistent-settlement-conflict",
		Status:      common.TokenStatusEnabled,
		RemainQuota: 100,
	}
	require.NoError(t, model.DB.Create(&token).Error)
	info := newBillingReservationInfo(user, "billing-persistent-settlement-conflict")
	info.IsPlayground = false
	info.TokenId = token.Id
	info.TokenKey = token.Key
	session, apiErr := NewBillingSession(newBillingReservationContext(), info, 20)
	require.Nil(t, apiErr)
	require.NotNil(t, session)

	var attempts atomic.Int32
	const callbackName = "test:billing_persistent_settlement_conflict"
	require.NoError(t, model.DB.Callback().Update().Before("gorm:update").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement.Schema != nil && tx.Statement.Schema.Name == "User" {
			attempts.Add(1)
			tx.AddError(model.ErrWalletConcurrentMutation)
		}
	}))
	defer func() { require.NoError(t, model.DB.Callback().Update().Remove(callbackName)) }()

	err := session.Settle(25)
	require.ErrorIs(t, err, model.ErrWalletConcurrentMutation)
	require.EqualValues(t, 5, attempts.Load())
	require.True(t, session.NeedsRefund())

	var storedUser model.User
	var storedToken model.Token
	require.NoError(t, model.DB.First(&storedUser, user.Id).Error)
	require.NoError(t, model.DB.First(&storedToken, token.Id).Error)
	require.EqualValues(t, 80, storedUser.Quota)
	require.Equal(t, 80, storedToken.RemainQuota)
	require.Equal(t, 20, storedToken.UsedQuota)

	var settlementEntries int64
	require.NoError(t, model.DB.Model(&model.WalletLedgerEntry{}).
		Where("reference_type = ? AND reference_id = ? AND context = ?", "relay_request", info.RequestId, "settle_debit").
		Count(&settlementEntries).Error)
	require.Zero(t, settlementEntries)
}

func TestBillingSessionDoesNotRetryNonConcurrencySettlementError(t *testing.T) {
	setupBillingReservationTest(t)
	user := seedBillingReservationUser(t, 98739, 100, "wallet_only")
	info := newBillingReservationInfo(user, "billing-non-concurrency-settlement-error")
	session, apiErr := NewBillingSession(newBillingReservationContext(), info, 20)
	require.Nil(t, apiErr)
	require.NotNil(t, session)

	expected := errors.New("forced non-concurrency wallet error")
	var attempts atomic.Int32
	const callbackName = "test:billing_non_concurrency_settlement_error"
	require.NoError(t, model.DB.Callback().Update().Before("gorm:update").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement.Schema != nil && tx.Statement.Schema.Name == "User" {
			attempts.Add(1)
			tx.AddError(expected)
		}
	}))
	defer func() { require.NoError(t, model.DB.Callback().Update().Remove(callbackName)) }()

	err := session.Settle(25)
	require.ErrorIs(t, err, expected)
	require.EqualValues(t, 1, attempts.Load())
}

func TestInitialReservationRollsBackTokenWithoutCompensationWhenFundingFails(t *testing.T) {
	setupBillingReservationTest(t)
	user := seedBillingReservationUser(t, 98770, 100, "wallet_only")
	token := model.Token{
		Id:          98771,
		UserId:      user.Id,
		Key:         "billing-atomic-initial-token",
		Name:        "billing-atomic-initial-token",
		Status:      common.TokenStatusEnabled,
		RemainQuota: 100,
	}
	require.NoError(t, model.DB.Create(&token).Error)
	ctx := newBillingReservationContext()
	info := newBillingReservationInfo(user, "billing-atomic-initial")
	info.IsPlayground = false
	info.TokenId = token.Id
	info.TokenKey = token.Key

	fundingErr := errors.New("forced initial funding failure")
	const fundingCallback = "test:billing_atomic_initial_funding_failure"
	require.NoError(t, model.DB.Callback().Create().Before("gorm:create").Register(fundingCallback, func(tx *gorm.DB) {
		if tx.Statement.Schema != nil && tx.Statement.Schema.Name == "WalletLedgerEntry" {
			tx.AddError(fundingErr)
		}
	}))
	defer func() { require.NoError(t, model.DB.Callback().Create().Remove(fundingCallback)) }()

	compensationErr := errors.New("forced initial token compensation failure")
	var tokenUpdates atomic.Int32
	const tokenCallback = "test:billing_atomic_initial_token_compensation_failure"
	require.NoError(t, model.DB.Callback().Update().Before("gorm:update").Register(tokenCallback, func(tx *gorm.DB) {
		if tx.Statement.Schema != nil && tx.Statement.Schema.Name == "Token" && tokenUpdates.Add(1) > 1 {
			tx.AddError(compensationErr)
		}
	}))
	defer func() { require.NoError(t, model.DB.Callback().Update().Remove(tokenCallback)) }()

	session, apiErr := NewBillingSession(ctx, info, 25)

	require.Nil(t, session)
	require.NotNil(t, apiErr)
	require.ErrorIs(t, apiErr.Err, fundingErr)
	var storedUser model.User
	var storedToken model.Token
	require.NoError(t, model.DB.First(&storedUser, user.Id).Error)
	require.NoError(t, model.DB.First(&storedToken, token.Id).Error)
	require.EqualValues(t, 100, storedUser.Quota)
	require.Equal(t, 100, storedToken.RemainQuota,
		"a failed initial reservation must roll back in one transaction without token compensation")
	require.Zero(t, storedToken.UsedQuota)
}

func TestReserveToTargetRollsBackFundingWithoutCompensationWhenTokenFails(t *testing.T) {
	setupBillingReservationTest(t)
	user := seedBillingReservationUser(t, 98780, 100, "wallet_only")
	token := model.Token{
		Id:          98781,
		UserId:      user.Id,
		Key:         "billing-atomic-target-token",
		Name:        "billing-atomic-target-token",
		Status:      common.TokenStatusEnabled,
		RemainQuota: 100,
	}
	require.NoError(t, model.DB.Create(&token).Error)
	info := newBillingReservationInfo(user, "billing-atomic-target")
	info.IsPlayground = false
	info.TokenId = token.Id
	info.TokenKey = token.Key
	session := &BillingSession{
		relayInfo: info,
		funding:   NewWalletFunding(user.Id, info.RequestId),
		trusted:   true,
	}

	rollbackErr := errors.New("forced reserve funding compensation failure")
	var ledgerCreates atomic.Int32
	const fundingCallback = "test:billing_atomic_target_funding_compensation_failure"
	require.NoError(t, model.DB.Callback().Create().Before("gorm:create").Register(fundingCallback, func(tx *gorm.DB) {
		if tx.Statement.Schema != nil && tx.Statement.Schema.Name == "WalletLedgerEntry" && ledgerCreates.Add(1) > 1 {
			tx.AddError(rollbackErr)
		}
	}))
	defer func() { require.NoError(t, model.DB.Callback().Create().Remove(fundingCallback)) }()

	tokenErr := errors.New("forced reserve token failure")
	const tokenCallback = "test:billing_atomic_target_token_failure"
	require.NoError(t, model.DB.Callback().Update().Before("gorm:update").Register(tokenCallback, func(tx *gorm.DB) {
		if tx.Statement.Schema != nil && tx.Statement.Schema.Name == "Token" {
			tx.AddError(tokenErr)
		}
	}))
	defer func() { require.NoError(t, model.DB.Callback().Update().Remove(tokenCallback)) }()

	err := session.ReserveToTarget(25)

	require.ErrorIs(t, err, tokenErr)
	var storedUser model.User
	var storedToken model.Token
	require.NoError(t, model.DB.First(&storedUser, user.Id).Error)
	require.NoError(t, model.DB.First(&storedToken, token.Id).Error)
	require.EqualValues(t, 100, storedUser.Quota,
		"a failed target reservation must roll back funding in the same transaction")
	require.Equal(t, 100, storedToken.RemainQuota)
	var ledgerCount int64
	require.NoError(t, model.DB.Model(&model.WalletLedgerEntry{}).
		Where("reference_type = ? AND reference_id = ?", "relay_request", info.RequestId).
		Count(&ledgerCount).Error)
	require.Zero(t, ledgerCount)
}

func TestSubscriptionSettlementAcrossResetPreservesNewPeriodUsage(t *testing.T) {
	setupBillingReservationTest(t)
	user := seedBillingReservationUser(t, 98690, 0, "subscription_only")
	seedSubscriptionBillingPlan(t, 98691, "Reset Boundary Pro")
	subscription := model.UserSubscription{
		Id:                  98692,
		UserId:              user.Id,
		PlanId:              98691,
		AmountTotal:         1000,
		StartTime:           common.GetTimestamp() - 7200,
		EndTime:             common.GetTimestamp() + 3600,
		LastResetTime:       common.GetTimestamp() - 3600,
		Status:              "active",
		UpgradeGroup:        "pro",
		AllowWalletOverflow: false,
	}
	require.NoError(t, model.DB.Create(&subscription).Error)
	ctx := newBillingReservationContext()
	info := newBillingReservationInfo(user, "billing-reset-boundary")
	session, apiErr := NewBillingSession(ctx, info, 100)
	require.Nil(t, apiErr)

	_, resetErr := model.AdminResetUserSubscriptionsByPlan(user.Id, subscription.PlanId, false)
	require.NoError(t, resetErr)
	require.NoError(t, model.DB.Model(&model.UserSubscription{}).Where("id = ?", subscription.Id).
		Update("amount_used", 30).Error)

	require.NoError(t, session.Settle(80))

	var stored model.UserSubscription
	require.NoError(t, model.DB.First(&stored, subscription.Id).Error)
	require.EqualValues(t, 30, stored.AmountUsed,
		"releasing a reservation from the prior reset period must not consume unrelated current-period usage")
	var reservation model.RelayFundingReservation
	require.NoError(t, model.DB.First(&reservation, "request_id = ?", info.RequestId).Error)
	require.EqualValues(t, 80, reservation.SubscriptionQuota)
}

func TestRefundFailureKeepsBillingSessionRetryable(t *testing.T) {
	setupBillingReservationTest(t)
	user := seedBillingReservationUser(t, 98730, 100, "wallet_only")
	ctx := newBillingReservationContext()
	info := newBillingReservationInfo(user, "billing-refund-retry")
	session, apiErr := NewBillingSession(ctx, info, 25)
	require.Nil(t, apiErr)

	refundErr := errors.New("forced refund persistence failure")
	var attempts atomic.Int32
	const callbackName = "test:billing_refund_retry"
	require.NoError(t, model.DB.Callback().Create().Before("gorm:create").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement.Schema != nil && tx.Statement.Schema.Name == "WalletLedgerEntry" {
			attempts.Add(1)
			tx.AddError(refundErr)
		}
	}))

	session.Refund(ctx)
	require.Eventually(t, func() bool { return attempts.Load() > 0 }, time.Second, 10*time.Millisecond)
	require.True(t, session.NeedsRefund(), "failed persistence must not permanently mark the session refunded")
	var afterFailure model.User
	require.NoError(t, model.DB.First(&afterFailure, user.Id).Error)
	require.EqualValues(t, 75, afterFailure.Quota)

	require.NoError(t, model.DB.Callback().Create().Remove(callbackName))
	session.Refund(ctx)
	require.Eventually(t, func() bool {
		var stored model.User
		return model.DB.First(&stored, user.Id).Error == nil && stored.Quota == 100
	}, time.Second, 10*time.Millisecond)
	require.False(t, session.NeedsRefund())
}

func TestSubscriptionReservationRefundAfterQuotaResetRestoresWalletOnly(t *testing.T) {
	setupBillingReservationTest(t)
	user := seedBillingReservationUser(t, 98680, 100, "subscription_first")
	seedSubscriptionBillingPlan(t, 98681, "Reset Boundary Pro")
	subscription := model.UserSubscription{
		Id:                  98682,
		UserId:              user.Id,
		PlanId:              98681,
		AmountTotal:         40,
		EndTime:             common.GetTimestamp() + 3600,
		Status:              "active",
		UpgradeGroup:        "pro",
		AllowWalletOverflow: true,
	}
	require.NoError(t, model.DB.Create(&subscription).Error)
	ctx := newBillingReservationContext()
	info := newBillingReservationInfo(user, "billing-reset-refund")
	session, apiErr := NewBillingSession(ctx, info, 100)
	require.Nil(t, apiErr)

	_, err := model.AdminResetUserSubscriptionsByPlan(user.Id, subscription.PlanId, false)
	require.NoError(t, err)
	session.Refund(ctx)

	require.Eventually(t, func() bool {
		var storedUser model.User
		var storedSubscription model.UserSubscription
		if model.DB.First(&storedUser, user.Id).Error != nil || model.DB.First(&storedSubscription, subscription.Id).Error != nil {
			return false
		}
		return storedUser.Quota == 100 && storedSubscription.AmountUsed == 0
	}, 3*time.Second, 20*time.Millisecond)
}

func TestSubscriptionReservationCanSettleBelowPreConsumeAfterQuotaReset(t *testing.T) {
	setupBillingReservationTest(t)
	user := seedBillingReservationUser(t, 98690, 0, "subscription_only")
	seedSubscriptionBillingPlan(t, 98691, "Reset Settle Pro")
	subscription := model.UserSubscription{
		Id:           98692,
		UserId:       user.Id,
		PlanId:       98691,
		AmountTotal:  1000,
		EndTime:      common.GetTimestamp() + 3600,
		Status:       "active",
		UpgradeGroup: "pro",
	}
	require.NoError(t, model.DB.Create(&subscription).Error)
	ctx := newBillingReservationContext()
	info := newBillingReservationInfo(user, "billing-reset-settle")
	session, apiErr := NewBillingSession(ctx, info, 100)
	require.Nil(t, apiErr)

	_, err := model.AdminResetUserSubscriptionsByPlan(user.Id, subscription.PlanId, false)
	require.NoError(t, err)
	require.NoError(t, session.Settle(50))

	var stored model.UserSubscription
	require.NoError(t, model.DB.First(&stored, subscription.Id).Error)
	require.Zero(t, stored.AmountUsed)
}

func TestSubscriptionZeroDeltaSettlementMarksReservationSettled(t *testing.T) {
	setupBillingReservationTest(t)
	user := seedBillingReservationUser(t, 98840, 0, "subscription_only")
	seedSubscriptionBillingPlan(t, 98841, "Terminal Reservation Pro")
	subscription := model.UserSubscription{
		Id: 98842, UserId: user.Id, PlanId: 98841, AmountTotal: 100,
		EndTime: common.GetTimestamp() + 3600, Status: "active", UpgradeGroup: "pro",
	}
	require.NoError(t, model.DB.Create(&subscription).Error)
	info := newBillingReservationInfo(user, "billing-zero-delta-terminal")
	session, apiErr := NewBillingSession(newBillingReservationContext(), info, 25)
	require.Nil(t, apiErr)

	require.NoError(t, session.Settle(25))

	var reservation model.RelayFundingReservation
	require.NoError(t, model.DB.First(&reservation, "request_id = ?", info.RequestId).Error)
	require.Equal(t, "settled", reservation.Status)
}

func TestPreserveReservationAfterDeliveryMarksReservationSettled(t *testing.T) {
	setupBillingReservationTest(t)
	user := seedBillingReservationUser(t, 98845, 0, "subscription_only")
	seedSubscriptionBillingPlan(t, 98846, "Delivered Reservation Pro")
	subscription := model.UserSubscription{
		Id: 98847, UserId: user.Id, PlanId: 98846, AmountTotal: 100,
		EndTime: common.GetTimestamp() + 3600, Status: "active", UpgradeGroup: "pro",
	}
	require.NoError(t, model.DB.Create(&subscription).Error)
	info := newBillingReservationInfo(user, "billing-delivered-terminal")
	session, apiErr := NewBillingSession(newBillingReservationContext(), info, 25)
	require.Nil(t, apiErr)

	session.PreserveReservationAfterDelivery()

	var reservation model.RelayFundingReservation
	require.NoError(t, model.DB.First(&reservation, "request_id = ?", info.RequestId).Error)
	require.Equal(t, "settled", reservation.Status)
	require.False(t, session.NeedsRefund())
}

func TestRelayFundingCleanupKeepsActiveReservations(t *testing.T) {
	setupBillingReservationTest(t)
	rows := []model.RelayFundingReservation{
		{RequestID: "cleanup-active", UserID: 98850, Status: "active"},
		{RequestID: "cleanup-settled", UserID: 98850, Status: "settled"},
		{RequestID: "cleanup-refunded", UserID: 98850, Status: "refunded"},
	}
	require.NoError(t, model.DB.Create(&rows).Error)
	old := model.GetDBTimestamp() - 7200
	require.NoError(t, model.DB.Model(&model.RelayFundingReservation{}).
		Where("request_id IN ?", []string{"cleanup-active", "cleanup-settled", "cleanup-refunded"}).
		UpdateColumn("updated_at", old).Error)

	deleted, err := model.CleanupRelayFundingReservations(3600)

	require.NoError(t, err)
	require.EqualValues(t, 2, deleted)
	var activeCount, terminalCount int64
	require.NoError(t, model.DB.Model(&model.RelayFundingReservation{}).
		Where("request_id = ?", "cleanup-active").Count(&activeCount).Error)
	require.NoError(t, model.DB.Model(&model.RelayFundingReservation{}).
		Where("request_id IN ?", []string{"cleanup-settled", "cleanup-refunded"}).Count(&terminalCount).Error)
	require.EqualValues(t, 1, activeCount)
	require.Zero(t, terminalCount)
}
