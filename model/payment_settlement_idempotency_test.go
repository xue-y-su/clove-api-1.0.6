package model

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type paymentSettlementCase struct {
	name           string
	provider       string
	amount         int64
	expectedCredit int64
	complete       func(tradeNo string) error
}

func paymentSettlementCases(t *testing.T) []paymentSettlementCase {
	t.Helper()
	stripeCredit, err := common.QuotaFromFloatStrict(common.QuotaPerUnit)
	require.NoError(t, err)
	return []paymentSettlementCase{
		{
			name: "stripe", provider: PaymentProviderStripe, amount: 1, expectedCredit: int64(stripeCredit),
			complete: func(tradeNo string) error {
				return RechargeWithSettlement(tradeNo, "cus_idempotent", "198.51.100.1", nil)
			},
		},
		{
			name: "creem", provider: PaymentProviderCreem, amount: 12_345, expectedCredit: 12_345,
			complete: func(tradeNo string) error {
				return RechargeCreemWithSettlement(tradeNo, "payer@example.com", "Payer", "198.51.100.2", nil)
			},
		},
	}
}

func installPaymentSettlementTestDB(t *testing.T, db *gorm.DB, databaseType common.DatabaseType) {
	t.Helper()
	previousDB := DB
	previousLogDB := LOG_DB
	previousMainType := common.MainDatabaseType()
	previousLogType := common.LogDatabaseType()
	previousRedisEnabled := common.RedisEnabled
	previousInvitationSetting := *operation_setting.GetInvitationIncentiveSetting()
	DB = db
	LOG_DB = db
	common.RedisEnabled = false
	common.SetDatabaseTypes(databaseType, databaseType)
	invitationSetting := operation_setting.DefaultInvitationIncentiveSetting()
	invitationSetting.Enabled = false
	invitationSetting.CashCommissionEnabled = false
	*operation_setting.GetInvitationIncentiveSetting() = invitationSetting
	t.Cleanup(func() {
		DB = previousDB
		LOG_DB = previousLogDB
		common.RedisEnabled = previousRedisEnabled
		common.SetDatabaseTypes(previousMainType, previousLogType)
		*operation_setting.GetInvitationIncentiveSetting() = previousInvitationSetting
	})
	require.NoError(t, db.AutoMigrate(
		&User{}, &TopUp{}, &Checkin{}, &WalletLedgerEntry{}, &AuditOutboxEvent{}, &Log{},
	))
}

func seedPaymentSettlement(t *testing.T, db *gorm.DB, tc paymentSettlementCase, suffix string) (User, TopUp) {
	t.Helper()
	identity := common.Sha1([]byte(suffix))[:16]
	user := User{
		Username: "settlement-" + identity,
		AffCode:  "aff-" + identity,
		Status:   common.UserStatusEnabled,
		Group:    "default",
	}
	require.NoError(t, db.Create(&user).Error)
	topUp := TopUp{
		UserId:          user.Id,
		Amount:          tc.amount,
		Money:           1,
		TradeNo:         "SETTLE-" + suffix,
		PaymentMethod:   tc.provider,
		PaymentProvider: tc.provider,
		EligibleQuota:   tc.expectedCredit,
		PaidAmount:      "1.00",
		PaidCurrency:    "USD",
		Status:          common.TopUpStatusPending,
		CreateTime:      common.GetTimestamp(),
	}
	require.NoError(t, db.Create(&topUp).Error)
	return user, topUp
}

func assertPaymentSettledOnce(t *testing.T, db *gorm.DB, user User, topUp TopUp, expectedCredit int64) {
	t.Helper()
	require.NoError(t, db.First(&user, user.Id).Error)
	require.Equal(t, expectedCredit, user.Quota)
	require.NoError(t, db.First(&topUp, topUp.Id).Error)
	require.Equal(t, common.TopUpStatusSuccess, topUp.Status)
	require.Positive(t, topUp.CompleteTime)

	var ledgerCount int64
	require.NoError(t, db.Model(&WalletLedgerEntry{}).
		Where("operation_id = ?", "topup:"+topUp.TradeNo).Count(&ledgerCount).Error)
	require.Equal(t, int64(1), ledgerCount)
	var auditCount int64
	require.NoError(t, db.Model(&AuditOutboxEvent{}).
		Where("event_id = ?", "wallet:topup:"+topUp.TradeNo).Count(&auditCount).Error)
	require.Equal(t, int64(1), auditCount)
	var logCount int64
	require.NoError(t, db.Model(&Log{}).
		Where("user_id = ? AND type = ?", user.Id, LogTypeTopup).Count(&logCount).Error)
	require.Equal(t, int64(1), logCount)
}

func runConcurrentPaymentCompletion(t *testing.T, attempts int, complete func() error) {
	t.Helper()
	start := make(chan struct{})
	errorsByAttempt := make(chan error, attempts)
	var workers sync.WaitGroup
	workers.Add(attempts)
	for range attempts {
		go func() {
			defer workers.Done()
			<-start
			errorsByAttempt <- complete()
		}()
	}
	close(start)
	workers.Wait()
	close(errorsByAttempt)
	for err := range errorsByAttempt {
		require.NoError(t, err)
	}
}

func TestCashTopUpSettlementIsIdempotentSQLite(t *testing.T) {
	for _, tc := range paymentSettlementCases(t) {
		t.Run(tc.name, func(t *testing.T) {
			db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s/payment.db?_pragma=busy_timeout(10000)", t.TempDir())), &gorm.Config{
				Logger: logger.Default.LogMode(logger.Silent),
			})
			require.NoError(t, err)
			sqlDB, err := db.DB()
			require.NoError(t, err)
			sqlDB.SetMaxOpenConns(1)
			t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
			installPaymentSettlementTestDB(t, db, common.DatabaseTypeSQLite)

			suffix := fmt.Sprintf("%s-%d", tc.name, time.Now().UnixNano())
			user, topUp := seedPaymentSettlement(t, db, tc, suffix)
			runConcurrentPaymentCompletion(t, 12, func() error { return tc.complete(topUp.TradeNo) })
			assertPaymentSettledOnce(t, db, user, topUp, tc.expectedCredit)
		})
	}
}

func TestPaymentSettlementConcurrencyMySQL(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("TEST_MYSQL_DSN"))
	if dsn == "" {
		t.Skip("set TEST_MYSQL_DSN to run the MySQL payment settlement concurrency test")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(24)
	sqlDB.SetMaxIdleConns(24)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	installPaymentSettlementTestDB(t, db, common.DatabaseTypeMySQL)

	for _, tc := range paymentSettlementCases(t) {
		t.Run(tc.name, func(t *testing.T) {
			suffix := fmt.Sprintf("mysql-%s-%d", tc.name, time.Now().UnixNano())
			user, topUp := seedPaymentSettlement(t, db, tc, suffix)
			t.Cleanup(func() {
				_ = db.Where("user_id = ?", user.Id).Delete(&Log{}).Error
				_ = db.Where("actor_id = ? OR log_user_id = ?", user.Id, user.Id).Delete(&AuditOutboxEvent{}).Error
				_ = db.Where("user_id = ?", user.Id).Delete(&WalletLedgerEntry{}).Error
				_ = db.Where("user_id = ?", user.Id).Delete(&Checkin{}).Error
				_ = db.Where("user_id = ?", user.Id).Delete(&TopUp{}).Error
				_ = db.Unscoped().Delete(&User{}, user.Id).Error
			})
			runConcurrentPaymentCompletion(t, 24, func() error { return tc.complete(topUp.TradeNo) })
			assertPaymentSettledOnce(t, db, user, topUp, tc.expectedCredit)
		})
	}

	require.NoError(t, db.AutoMigrate(
		&SubscriptionPlan{}, &SubscriptionOrder{}, &UserSubscription{}, &SubscriptionPurchaseCounter{},
	))
	t.Run("subscription", func(t *testing.T) {
		identity := common.Sha1([]byte(fmt.Sprintf("mysql-subscription-%d", time.Now().UnixNano())))[:16]
		user := User{
			Username: "sub-" + identity, AffCode: "sub-aff-" + identity,
			Status: common.UserStatusEnabled, Group: "default",
		}
		require.NoError(t, db.Create(&user).Error)
		allowOverflow := false
		plan := SubscriptionPlan{
			Title: "Concurrent subscription", PriceAmount: 3, Currency: "USD", Enabled: true,
			DurationUnit: SubscriptionDurationDay, DurationValue: 7, TotalAmount: 30_000,
			QuotaResetPeriod: SubscriptionResetDaily, AllowWalletOverflow: &allowOverflow,
		}
		require.NoError(t, db.Create(&plan).Error)
		order := SubscriptionOrder{
			UserId: user.Id, PlanId: plan.Id, TradeNo: "MYSQL-SUB-" + identity,
			PaymentMethod: PaymentMethodStripe, PaymentProvider: PaymentProviderStripe,
			Status: common.TopUpStatusPending,
		}
		require.NoError(t, order.Insert())
		t.Cleanup(func() {
			_ = db.Where("actor_id = ? OR log_user_id = ?", user.Id, user.Id).Delete(&AuditOutboxEvent{}).Error
			_ = db.Where("trade_no = ?", order.TradeNo).Delete(&TopUp{}).Error
			_ = db.Where("trade_no = ?", order.TradeNo).Delete(&SubscriptionOrder{}).Error
			_ = db.Where("user_id = ?", user.Id).Delete(&UserSubscription{}).Error
			_ = db.Delete(&SubscriptionPlan{}, plan.Id).Error
			_ = db.Unscoped().Delete(&User{}, user.Id).Error
		})

		runConcurrentPaymentCompletion(t, 24, func() error {
			return CompleteSubscriptionOrderWithSettlement(
				order.TradeNo, `{"source":"mysql-concurrency"}`, PaymentProviderStripe, PaymentMethodStripe, nil,
			)
		})
		require.NoError(t, db.First(&order, order.Id).Error)
		require.Equal(t, common.TopUpStatusSuccess, order.Status)
		var subscriptionCount, topUpCount, auditCount int64
		require.NoError(t, db.Model(&UserSubscription{}).Where("user_id = ?", user.Id).Count(&subscriptionCount).Error)
		require.NoError(t, db.Model(&TopUp{}).Where("trade_no = ?", order.TradeNo).Count(&topUpCount).Error)
		require.NoError(t, db.Model(&AuditOutboxEvent{}).
			Where("event_id = ?", "subscription:purchase:"+fmt.Sprint(order.Id)).Count(&auditCount).Error)
		require.Equal(t, int64(1), subscriptionCount)
		require.Equal(t, int64(1), topUpCount)
		require.Equal(t, int64(1), auditCount)
	})
}

func seedAuditedSubscriptionOrder(t *testing.T, suffix string) (User, SubscriptionOrder) {
	t.Helper()
	user := User{
		Username: "subscription-audit-" + suffix,
		AffCode:  "subscription-audit-aff-" + suffix,
		Status:   common.UserStatusEnabled,
		Group:    "default",
	}
	require.NoError(t, DB.Create(&user).Error)
	allowOverflow := false
	plan := SubscriptionPlan{
		Title: "Audited plan", PriceAmount: 2.5, Currency: "USD", Enabled: true,
		DurationUnit: SubscriptionDurationDay, DurationValue: 7, TotalAmount: 25_000,
		QuotaResetPeriod: SubscriptionResetDaily, AllowWalletOverflow: &allowOverflow,
	}
	require.NoError(t, DB.Create(&plan).Error)
	order := SubscriptionOrder{
		UserId: user.Id, PlanId: plan.Id, TradeNo: "SUB-AUDIT-" + suffix,
		PaymentMethod: PaymentMethodStripe, PaymentProvider: PaymentProviderStripe,
		Status: common.TopUpStatusPending,
	}
	require.NoError(t, order.Insert())
	return user, order
}

func TestSubscriptionCompletionCommitsDurableAuditWithBusinessState(t *testing.T) {
	truncateTables(t)
	user, order := seedAuditedSubscriptionOrder(t, fmt.Sprintf("commit-%d", time.Now().UnixNano()))
	require.NoError(t, CompleteSubscriptionOrder(order.TradeNo, "{}", PaymentProviderStripe, PaymentMethodStripe))
	require.NoError(t, CompleteSubscriptionOrder(order.TradeNo, "{}", PaymentProviderStripe, PaymentMethodStripe))

	require.NoError(t, DB.First(&order, order.Id).Error)
	require.Equal(t, common.TopUpStatusSuccess, order.Status)
	var subscriptionCount int64
	require.NoError(t, DB.Model(&UserSubscription{}).Where("user_id = ?", user.Id).Count(&subscriptionCount).Error)
	require.Equal(t, int64(1), subscriptionCount)
	var auditEvents []AuditOutboxEvent
	require.NoError(t, DB.Where("event_id = ?", "subscription:purchase:"+fmt.Sprint(order.Id)).Find(&auditEvents).Error)
	require.Len(t, auditEvents, 1)
	require.Equal(t, "subscription.purchase_completed", auditEvents[0].EventType)
	require.Equal(t, AuditOutboxStatusPending, auditEvents[0].Status)
}

func TestSubscriptionCompletionRollsBackWhenDurableAuditFails(t *testing.T) {
	truncateTables(t)
	user, order := seedAuditedSubscriptionOrder(t, fmt.Sprintf("rollback-%d", time.Now().UnixNano()))
	callbackName := "subscription_settlement_test:fail_audit_outbox"
	require.NoError(t, DB.Callback().Create().Before("gorm:create").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Schema != nil && tx.Statement.Schema.Table == "audit_outbox_events" {
			tx.AddError(errors.New("forced subscription audit outbox failure"))
		}
	}))
	t.Cleanup(func() { _ = DB.Callback().Create().Remove(callbackName) })

	require.Error(t, CompleteSubscriptionOrder(order.TradeNo, "{}", PaymentProviderStripe, PaymentMethodStripe))
	require.NoError(t, DB.First(&order, order.Id).Error)
	require.Equal(t, common.TopUpStatusPending, order.Status)
	var subscriptionCount, topUpCount, auditCount int64
	require.NoError(t, DB.Model(&UserSubscription{}).Where("user_id = ?", user.Id).Count(&subscriptionCount).Error)
	require.NoError(t, DB.Model(&TopUp{}).Where("trade_no = ?", order.TradeNo).Count(&topUpCount).Error)
	require.NoError(t, DB.Model(&AuditOutboxEvent{}).Where("event_id = ?", "subscription:purchase:"+fmt.Sprint(order.Id)).Count(&auditCount).Error)
	require.Zero(t, subscriptionCount)
	require.Zero(t, topUpCount)
	require.Zero(t, auditCount)
}

func TestBalanceSubscriptionCommitsDurableAuditWithPurchase(t *testing.T) {
	truncateTables(t)
	user := User{
		Username: "balance-subscription-audit", AffCode: "balance-sub-audit",
		Status: common.UserStatusEnabled, Group: "default", Quota: int64(common.QuotaPerUnit * 5),
	}
	require.NoError(t, DB.Create(&user).Error)
	plan := SubscriptionPlan{
		Title: "Balance audited plan", PriceAmount: 2, Currency: "USD", Enabled: true,
		DurationUnit: SubscriptionDurationDay, DurationValue: 3, TotalAmount: 10_000,
	}
	require.NoError(t, DB.Create(&plan).Error)
	require.NoError(t, PurchaseSubscriptionWithBalance(user.Id, plan.Id))

	var order SubscriptionOrder
	require.NoError(t, DB.Where("user_id = ? AND payment_provider = ?", user.Id, PaymentProviderBalance).First(&order).Error)
	require.Positive(t, order.UserSubscriptionID)
	var event AuditOutboxEvent
	require.NoError(t, DB.First(&event, "event_id = ?", "subscription:purchase:"+fmt.Sprint(order.Id)).Error)
	require.Equal(t, "subscription.purchase_completed", event.EventType)
	require.Contains(t, event.Payload, `"method":"balance"`)
	require.Contains(t, event.Payload, `"quota":"`)
}

func TestFreeBalanceSubscriptionRollsBackWhenDurableAuditFails(t *testing.T) {
	truncateTables(t)
	user := User{
		Username: "free-subscription-audit", AffCode: "free-sub-audit",
		Status: common.UserStatusEnabled, Group: "default", Quota: 100,
	}
	require.NoError(t, DB.Create(&user).Error)
	plan := SubscriptionPlan{
		Title: "Free audited plan", PriceAmount: 0, Currency: "USD", Enabled: true,
		DurationUnit: SubscriptionDurationDay, DurationValue: 1, TotalAmount: 1_000,
	}
	require.NoError(t, DB.Create(&plan).Error)
	callbackName := "balance_subscription_test:fail_purchase_audit"
	require.NoError(t, DB.Callback().Create().Before("gorm:create").Register(callbackName, func(tx *gorm.DB) {
		if event, ok := tx.Statement.Dest.(*AuditOutboxEvent); ok && event.EventType == "subscription.purchase_completed" {
			tx.AddError(errors.New("forced balance subscription audit failure"))
		}
	}))
	t.Cleanup(func() { _ = DB.Callback().Create().Remove(callbackName) })

	require.Error(t, PurchaseSubscriptionWithBalance(user.Id, plan.Id))
	var subscriptionCount, orderCount, auditCount int64
	require.NoError(t, DB.Model(&UserSubscription{}).Where("user_id = ?", user.Id).Count(&subscriptionCount).Error)
	require.NoError(t, DB.Model(&SubscriptionOrder{}).Where("user_id = ?", user.Id).Count(&orderCount).Error)
	require.NoError(t, DB.Model(&AuditOutboxEvent{}).Where("actor_id = ?", user.Id).Count(&auditCount).Error)
	require.Zero(t, subscriptionCount)
	require.Zero(t, orderCount)
	require.Zero(t, auditCount)
	require.NoError(t, DB.First(&user, user.Id).Error)
	require.Equal(t, int64(100), user.Quota)
}
