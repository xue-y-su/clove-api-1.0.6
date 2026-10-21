package model

import (
	"fmt"
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestAdminInvalidateLocksUserBeforeUpdatingSubscription(t *testing.T) {
	truncateTables(t)
	now := common.GetTimestamp()
	user := User{Id: 98900, Username: "lock-order-user", Group: "pro", Status: common.UserStatusEnabled}
	subscription := UserSubscription{
		Id:            98901,
		UserId:        user.Id,
		PlanId:        98902,
		Status:        "active",
		EndTime:       now + 3600,
		UpgradeGroup:  "pro",
		PrevUserGroup: "default",
	}
	require.NoError(t, DB.Create(&user).Error)
	require.NoError(t, DB.Create(&subscription).Error)

	var mu sync.Mutex
	operations := make([]string, 0, 8)
	record := func(operation string) func(*gorm.DB) {
		return func(tx *gorm.DB) {
			table := tx.Statement.Table
			if table != "users" && table != "user_subscriptions" {
				return
			}
			mu.Lock()
			operations = append(operations, operation+":"+table)
			mu.Unlock()
		}
	}
	callbackName := fmt.Sprintf("test:subscription-lock-order:%s", t.Name())
	require.NoError(t, DB.Callback().Query().Before("gorm:query").Register(callbackName, record("query")))
	require.NoError(t, DB.Callback().Update().Before("gorm:update").Register(callbackName, record("update")))
	t.Cleanup(func() {
		_ = DB.Callback().Query().Remove(callbackName)
		_ = DB.Callback().Update().Remove(callbackName)
	})

	_, err := AdminInvalidateUserSubscription(subscription.Id)
	require.NoError(t, err)

	mu.Lock()
	defer mu.Unlock()
	userQuery := operationIndex(operations, "query:users")
	subscriptionUpdate := operationIndex(operations, "update:user_subscriptions")
	require.NotEqual(t, -1, userQuery, operations)
	require.NotEqual(t, -1, subscriptionUpdate, operations)
	require.Less(t, userQuery, subscriptionUpdate, operations)
}

func TestExpireSubscriptionsLocksUserBeforeUpdatingSubscriptions(t *testing.T) {
	truncateTables(t)
	now := common.GetTimestamp()
	user := User{Id: 98910, Username: "expire-lock-order-user", Group: "pro", Status: common.UserStatusEnabled}
	subscription := UserSubscription{
		Id:            98911,
		UserId:        user.Id,
		PlanId:        98912,
		Status:        "active",
		EndTime:       now - 1,
		UpgradeGroup:  "pro",
		PrevUserGroup: "default",
	}
	require.NoError(t, DB.Create(&user).Error)
	require.NoError(t, DB.Create(&subscription).Error)

	var mu sync.Mutex
	operations := make([]string, 0, 8)
	record := func(operation string) func(*gorm.DB) {
		return func(tx *gorm.DB) {
			table := tx.Statement.Table
			if table != "users" && table != "user_subscriptions" {
				return
			}
			mu.Lock()
			operations = append(operations, operation+":"+table)
			mu.Unlock()
		}
	}
	callbackName := fmt.Sprintf("test:subscription-expire-lock-order:%s", t.Name())
	require.NoError(t, DB.Callback().Query().Before("gorm:query").Register(callbackName, record("query")))
	require.NoError(t, DB.Callback().Update().Before("gorm:update").Register(callbackName, record("update")))
	t.Cleanup(func() {
		_ = DB.Callback().Query().Remove(callbackName)
		_ = DB.Callback().Update().Remove(callbackName)
	})

	_, err := ExpireDueSubscriptions(10)
	require.NoError(t, err)

	mu.Lock()
	defer mu.Unlock()
	userQuery := operationIndex(operations, "query:users")
	subscriptionUpdate := operationIndex(operations, "update:user_subscriptions")
	require.NotEqual(t, -1, userQuery, operations)
	require.NotEqual(t, -1, subscriptionUpdate, operations)
	require.Less(t, userQuery, subscriptionUpdate, operations)
}

func TestSubscriptionResetLockHelperUsesUserReservationSubscriptionOrder(t *testing.T) {
	truncateTables(t)
	require.NoError(t, DB.AutoMigrate(&RelayFundingReservation{}))
	now := common.GetTimestamp()
	user := User{Id: 98920, Username: "reset-lock-order-user", Group: "pro", Status: common.UserStatusEnabled}
	subscription := UserSubscription{
		Id: 98921, UserId: user.Id, PlanId: 98922, Status: "active",
		EndTime: now + 3600, UpgradeGroup: "pro",
	}
	reservation := RelayFundingReservation{
		RequestID: "reset-lock-order", UserID: user.Id, UserSubscriptionID: subscription.Id,
		Status: relayFundingReservationActive,
	}
	require.NoError(t, DB.Create(&user).Error)
	require.NoError(t, DB.Create(&subscription).Error)
	require.NoError(t, DB.Create(&reservation).Error)

	var mu sync.Mutex
	operations := make([]string, 0, 8)
	callbackName := fmt.Sprintf("test:subscription-reset-lock-helper:%s", t.Name())
	require.NoError(t, DB.Callback().Query().Before("gorm:query").Register(callbackName, func(tx *gorm.DB) {
		switch tx.Statement.Table {
		case "users", "relay_funding_reservations", "user_subscriptions":
			mu.Lock()
			operations = append(operations, "query:"+tx.Statement.Table)
			mu.Unlock()
		}
	}))
	t.Cleanup(func() { _ = DB.Callback().Query().Remove(callbackName) })

	err := DB.Transaction(func(tx *gorm.DB) error {
		_, err := lockUserRelayReservationThenSubscriptionForUpdateTx(tx, user.Id, subscription.Id)
		return err
	})
	require.NoError(t, err)

	mu.Lock()
	defer mu.Unlock()
	userQuery := operationIndex(operations, "query:users")
	reservationQuery := operationIndex(operations, "query:relay_funding_reservations")
	subscriptionQuery := operationIndex(operations, "query:user_subscriptions")
	require.NotEqual(t, -1, userQuery, operations)
	require.NotEqual(t, -1, reservationQuery, operations)
	require.NotEqual(t, -1, subscriptionQuery, operations)
	require.Less(t, userQuery, reservationQuery, operations)
	require.Less(t, reservationQuery, subscriptionQuery, operations)
}

func TestSubscriptionResetPathsLockUserBeforeReservationMutation(t *testing.T) {
	tests := []struct {
		name string
		run  func(t *testing.T, user User, plan SubscriptionPlan, subscription UserSubscription) error
	}{
		{
			name: "admin user",
			run: func(t *testing.T, user User, plan SubscriptionPlan, subscription UserSubscription) error {
				_, err := AdminResetUserSubscriptionsByPlan(user.Id, plan.Id, false)
				return err
			},
		},
		{
			name: "admin plan",
			run: func(t *testing.T, user User, plan SubscriptionPlan, subscription UserSubscription) error {
				_, err := AdminResetPlanSubscriptions(plan.Id, false)
				return err
			},
		},
		{
			name: "scheduled",
			run: func(t *testing.T, user User, plan SubscriptionPlan, subscription UserSubscription) error {
				count, err := ResetDueSubscriptions(10)
				require.Equal(t, 1, count)
				return err
			},
		},
	}
	for index, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			truncateTables(t)
			require.NoError(t, DB.AutoMigrate(&RelayFundingReservation{}))
			now := common.GetTimestamp()
			base := 98930 + index*10
			user := User{Id: base, Username: fmt.Sprintf("reset-path-user-%d", index), Group: "pro", Status: common.UserStatusEnabled}
			plan := SubscriptionPlan{
				Id: base + 1, Title: fmt.Sprintf("Reset Path %d", index), DurationUnit: SubscriptionDurationMonth,
				DurationValue: 1, TotalAmount: 100, QuotaResetPeriod: SubscriptionResetDaily,
			}
			subscription := UserSubscription{
				Id: base + 2, UserId: user.Id, PlanId: plan.Id, AmountTotal: 100, AmountUsed: 50,
				StartTime: now - 172800, EndTime: now + 3600, Status: "active", UpgradeGroup: "pro",
				LastResetTime: now - 172800, NextResetTime: now - 1, QuotaResetPeriod: SubscriptionResetDaily,
			}
			reservation := RelayFundingReservation{
				RequestID: fmt.Sprintf("reset-path-%d", index), UserID: user.Id,
				UserSubscriptionID: subscription.Id, SubscriptionQuota: 10,
				SubscriptionAppliedQuota: 10, Status: relayFundingReservationActive,
			}
			require.NoError(t, DB.Create(&user).Error)
			require.NoError(t, DB.Create(&plan).Error)
			require.NoError(t, DB.Create(&subscription).Error)
			require.NoError(t, DB.Create(&reservation).Error)

			var mu sync.Mutex
			operations := make([]string, 0, 16)
			callbackName := fmt.Sprintf("test:subscription-reset-path-lock:%s", t.Name())
			record := func(operation string) func(*gorm.DB) {
				return func(tx *gorm.DB) {
					switch tx.Statement.Table {
					case "users", "relay_funding_reservations", "user_subscriptions":
						mu.Lock()
						operations = append(operations, operation+":"+tx.Statement.Table)
						mu.Unlock()
					}
				}
			}
			require.NoError(t, DB.Callback().Query().Before("gorm:query").Register(callbackName, record("query")))
			require.NoError(t, DB.Callback().Update().Before("gorm:update").Register(callbackName, record("update")))
			t.Cleanup(func() {
				_ = DB.Callback().Query().Remove(callbackName)
				_ = DB.Callback().Update().Remove(callbackName)
			})

			require.NoError(t, tc.run(t, user, plan, subscription))

			mu.Lock()
			defer mu.Unlock()
			userQuery := operationIndex(operations, "query:users")
			reservationQuery := operationIndex(operations, "query:relay_funding_reservations")
			subscriptionUpdate := operationIndex(operations, "update:user_subscriptions")
			require.NotEqual(t, -1, userQuery, operations)
			require.NotEqual(t, -1, reservationQuery, operations)
			require.NotEqual(t, -1, subscriptionUpdate, operations)
			require.Less(t, userQuery, reservationQuery, operations)
			require.Less(t, reservationQuery, subscriptionUpdate, operations)
		})
	}
}

func TestNewRelayReservationLocksActiveReservationsBeforeSubscriptionCandidates(t *testing.T) {
	truncateTables(t)
	require.NoError(t, DB.AutoMigrate(&RelayFundingReservation{}))
	now := common.GetTimestamp()
	user := User{Id: 98970, Username: "relay-candidate-lock-user", Group: "pro", Status: common.UserStatusEnabled}
	plan := SubscriptionPlan{
		Id: 98971, Title: "Relay Candidate Lock", DurationUnit: SubscriptionDurationMonth,
		DurationValue: 1, TotalAmount: 100, QuotaResetPeriod: SubscriptionResetDaily,
	}
	subscription := UserSubscription{
		Id: 98972, UserId: user.Id, PlanId: plan.Id, AmountTotal: 100, AmountUsed: 10,
		StartTime: now - 172800, EndTime: now + 3600, Status: "active", UpgradeGroup: "pro",
		LastResetTime: now - 172800, NextResetTime: now - 1, QuotaResetPeriod: SubscriptionResetDaily,
	}
	existing := RelayFundingReservation{
		RequestID: "relay-existing-active", UserID: user.Id, UserSubscriptionID: subscription.Id,
		SubscriptionQuota: 10, SubscriptionAppliedQuota: 10, Status: relayFundingReservationActive,
	}
	require.NoError(t, DB.Create(&user).Error)
	require.NoError(t, DB.Create(&plan).Error)
	require.NoError(t, DB.Create(&subscription).Error)
	require.NoError(t, DB.Create(&existing).Error)

	var mu sync.Mutex
	operations := make([]string, 0, 12)
	callbackName := fmt.Sprintf("test:relay-candidate-lock-order:%s", t.Name())
	require.NoError(t, DB.Callback().Query().Before("gorm:query").Register(callbackName, func(tx *gorm.DB) {
		operation := ""
		switch tx.Statement.Table {
		case "users":
			operation = "user"
		case "relay_funding_reservations":
			if len(tx.Statement.Selects) == 1 && tx.Statement.Selects[0] == "request_id" {
				operation = "active-reservations"
			}
		case "user_subscriptions":
			operation = "subscriptions"
		}
		if operation != "" {
			mu.Lock()
			operations = append(operations, operation)
			mu.Unlock()
		}
	}))
	t.Cleanup(func() { _ = DB.Callback().Query().Remove(callbackName) })

	_, err := SetRelayFundingReservationTarget("relay-new-request", user.Id, 5, nil, false)
	require.NoError(t, err)

	mu.Lock()
	defer mu.Unlock()
	userQuery := operationIndex(operations, "user")
	reservationQuery := operationIndex(operations, "active-reservations")
	subscriptionQuery := operationIndex(operations, "subscriptions")
	require.NotEqual(t, -1, userQuery, operations)
	require.NotEqual(t, -1, reservationQuery, operations)
	require.NotEqual(t, -1, subscriptionQuery, operations)
	require.Less(t, userQuery, reservationQuery, operations)
	require.Less(t, reservationQuery, subscriptionQuery, operations)
}

func operationIndex(operations []string, target string) int {
	for i, operation := range operations {
		if operation == target {
			return i
		}
	}
	return -1
}
