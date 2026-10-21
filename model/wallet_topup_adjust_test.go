package model

import (
	"bytes"
	"fmt"
	"log"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupTopupWalletTestDB(t *testing.T) {
	t.Helper()
	prev := DB
	prevLogDB := LOG_DB
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:topup_wallet_%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	DB = db
	LOG_DB = db
	t.Cleanup(func() {
		DB = prev
		LOG_DB = prevLogDB
		_ = sqlDB.Close()
	})
	require.NoError(t, db.AutoMigrate(&User{}, &TopUp{}, &Checkin{}, &WalletLedgerEntry{}, &AuditOutboxEvent{}, &Log{}))
}

func TestCreditTopUpWithTxIsIdempotent(t *testing.T) {
	setupTopupWalletTestDB(t)
	require.NoError(t, DB.Create(&User{Id: 1, Username: "u1", AffCode: "a1", Quota: 100, Status: common.UserStatusEnabled}).Error)

	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		_, err := CreditTopUpWithTx(tx, 1, 50, "TNO-1", "stripe")
		return err
	}))
	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		_, err := CreditTopUpWithTx(tx, 1, 50, "TNO-1", "stripe")
		return err
	}))

	var user User
	require.NoError(t, DB.First(&user, 1).Error)
	require.Equal(t, int64(150), user.Quota)
	var count int64
	require.NoError(t, DB.Model(&WalletLedgerEntry{}).Count(&count).Error)
	require.EqualValues(t, 1, count)
}

func TestAdjustWalletWithTxDoesNotLogExpectedMissingOperation(t *testing.T) {
	var output bytes.Buffer
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:wallet_log_%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{
		Logger: logger.New(log.New(&output, "", 0), logger.Config{
			LogLevel:                  logger.Warn,
			IgnoreRecordNotFoundError: false,
			Colorful:                  false,
		}),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&User{}, &WalletLedgerEntry{}, &AuditOutboxEvent{}))
	require.NoError(t, db.Create(&User{
		Id: 11, Username: "wallet-log-user", AffCode: "wallet-log-aff", Quota: 100,
		Status: common.UserStatusEnabled,
	}).Error)
	output.Reset()

	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		_, adjustErr := AdjustWalletWithTx(tx, 11, 5, "wallet-log-operation", "test", WalletAdjustReference{})
		return adjustErr
	}))
	require.NotContains(t, output.String(), "record not found")
}

func TestManualCompleteTopUpWritesLedger(t *testing.T) {
	setupTopupWalletTestDB(t)
	require.NoError(t, DB.Create(&User{Id: 2, Username: "u2", AffCode: "a2", Quota: 0, Status: common.UserStatusEnabled}).Error)
	require.NoError(t, DB.Create(&TopUp{
		UserId: 2, Amount: 1, Money: 1, TradeNo: "ADMIN-1",
		PaymentMethod: "admin", PaymentProvider: PaymentProviderEpay,
		Status: common.TopUpStatusPending, CreateTime: common.GetTimestamp(),
	}).Error)

	require.NoError(t, ManualCompleteTopUp("ADMIN-1", "127.0.0.1"))
	var user User
	require.NoError(t, DB.First(&user, 2).Error)
	require.Greater(t, user.Quota, int64(0))
	var entry WalletLedgerEntry
	require.NoError(t, DB.Where("operation_id = ?", "topup:ADMIN-1").First(&entry).Error)
	require.Equal(t, "topup", entry.Category)
	require.Equal(t, user.Quota, entry.BalanceAfter)

	// second complete is idempotent and must not double credit
	require.NoError(t, ManualCompleteTopUp("ADMIN-1", "127.0.0.1"))
	var user2 User
	require.NoError(t, DB.First(&user2, 2).Error)
	require.Equal(t, user.Quota, user2.Quota)
	var ledgerCount, logCount int64
	require.NoError(t, DB.Model(&WalletLedgerEntry{}).Where("operation_id = ?", "topup:ADMIN-1").Count(&ledgerCount).Error)
	require.NoError(t, LOG_DB.Model(&Log{}).Where("user_id = ? AND type = ?", 2, LogTypeTopup).Count(&logCount).Error)
	require.Equal(t, int64(1), ledgerCount)
	require.Equal(t, int64(1), logCount)
}

func TestRechargeCreemRejectsQuotaOverflowWithoutCompletingOrder(t *testing.T) {
	setupTopupWalletTestDB(t)
	require.NoError(t, DB.Create(&User{Id: 4, Username: "u4", AffCode: "a4", Quota: 0, Status: common.UserStatusEnabled}).Error)
	require.NoError(t, DB.Create(&TopUp{
		UserId: 4, Amount: int64(common.MaxQuota) + 1, Money: 1, TradeNo: "CREEM-OVERFLOW",
		PaymentMethod: PaymentMethodCreem, PaymentProvider: PaymentProviderCreem,
		Status: common.TopUpStatusPending, CreateTime: common.GetTimestamp(),
	}).Error)

	err := RechargeCreem("CREEM-OVERFLOW", "", "", "127.0.0.1")
	require.Error(t, err)

	var user User
	require.NoError(t, DB.First(&user, 4).Error)
	require.Zero(t, user.Quota)
	var topUp TopUp
	require.NoError(t, DB.Where("trade_no = ?", "CREEM-OVERFLOW").First(&topUp).Error)
	require.Equal(t, common.TopUpStatusPending, topUp.Status)
}

func TestRechargeCreemDoesNotBindPaymentEmailToUser(t *testing.T) {
	setupTopupWalletTestDB(t)
	require.NoError(t, DB.Create(&User{
		Id: 6, Username: "u6", AffCode: "a6", Quota: 0, Status: common.UserStatusEnabled,
	}).Error)
	require.NoError(t, DB.Create(&TopUp{
		UserId: 6, Amount: 100, Money: 1, TradeNo: "CREEM-PAYER-EMAIL",
		PaymentMethod: PaymentMethodCreem, PaymentProvider: PaymentProviderCreem,
		Status: common.TopUpStatusPending, CreateTime: common.GetTimestamp(),
	}).Error)

	require.NoError(t, RechargeCreem(
		"CREEM-PAYER-EMAIL", "payer@example.com", "Payment Customer", "127.0.0.1",
	))

	var user User
	require.NoError(t, DB.First(&user, 6).Error)
	require.Empty(t, user.Email)
	require.Equal(t, int64(100), user.Quota)

	var topUp TopUp
	require.NoError(t, DB.Where("trade_no = ?", "CREEM-PAYER-EMAIL").First(&topUp).Error)
	require.Equal(t, "payer@example.com", topUp.PayerEmail)
	require.Equal(t, "Payment Customer", topUp.PayerName)
}

func TestDecreaseUserQuotaRejectsUnderflow(t *testing.T) {
	setupTopupWalletTestDB(t)
	require.NoError(t, DB.Create(&User{Id: 3, Username: "u3", AffCode: "a3", Quota: 5, Status: common.UserStatusEnabled}).Error)
	err := DecreaseUserQuota(3, 10, true)
	require.Error(t, err)
	var user User
	require.NoError(t, DB.First(&user, 3).Error)
	require.Equal(t, int64(5), user.Quota)
}

func TestAdjustWalletWithTxRejectsIntegerOverflow(t *testing.T) {
	setupTopupWalletTestDB(t)
	maxStoredQuota := int64(^uint64(0) >> 1)
	require.NoError(t, DB.Create(&User{Id: 5, Username: "u5", AffCode: "a5", Quota: maxStoredQuota, Status: common.UserStatusEnabled}).Error)

	err := DB.Transaction(func(tx *gorm.DB) error {
		_, adjustErr := AdjustWalletWithTx(tx, 5, 1, "quota-overflow", "test", WalletAdjustReference{})
		return adjustErr
	})
	require.ErrorIs(t, err, ErrWalletInvalidAdjustment)

	var user User
	require.NoError(t, DB.First(&user, 5).Error)
	require.Equal(t, maxStoredQuota, user.Quota)
}

func TestAdjustWalletWithTxCreditsBalanceAboveBillingLimit(t *testing.T) {
	setupTopupWalletTestDB(t)
	require.NoError(t, DB.Create(&User{Id: 9, Username: "accumulated-wallet", AffCode: "a9", Quota: common.MaxQuota, Status: common.UserStatusEnabled}).Error)

	err := DB.Transaction(func(tx *gorm.DB) error {
		_, adjustErr := AdjustWalletWithTx(tx, 9, 1, "accumulated-wallet-credit", "topup", WalletAdjustReference{
			Type: "topup", ID: "accumulated-credit", Context: "stripe",
		})
		return adjustErr
	})
	require.NoError(t, err)

	var user User
	require.NoError(t, DB.First(&user, 9).Error)
	require.Equal(t, int64(common.MaxQuota)+1, user.Quota)
}

func TestAdjustWalletWithTxDebitsLegacyBalanceAboveBillingLimit(t *testing.T) {
	setupTopupWalletTestDB(t)
	legacyBalance := int64(common.MaxQuota) + 5000
	require.NoError(t, DB.Create(&User{Id: 8, Username: "legacy-wallet", AffCode: "a8", Quota: legacyBalance, Status: common.UserStatusEnabled}).Error)

	err := DB.Transaction(func(tx *gorm.DB) error {
		_, adjustErr := AdjustWalletWithTx(tx, 8, -500, "legacy-wallet-debit", "relay", WalletAdjustReference{
			Type: "relay_request", ID: "legacy-request", Context: "pre_consume",
		})
		return adjustErr
	})
	require.NoError(t, err)

	var user User
	require.NoError(t, DB.First(&user, 8).Error)
	require.Equal(t, legacyBalance-500, user.Quota)
}

func TestCreditTopUpWithTxRepairsLegacyNegativeBalance(t *testing.T) {
	setupTopupWalletTestDB(t)
	require.NoError(t, DB.Create(&User{
		Id: 10, Username: "legacy-debt-wallet", AffCode: "a10", Quota: -100,
		Status: common.UserStatusEnabled,
	}).Error)

	err := DB.Transaction(func(tx *gorm.DB) error {
		_, creditErr := CreditTopUpWithTx(tx, 10, 40, "LEGACY-DEBT-TOPUP", "stripe")
		return creditErr
	})
	require.NoError(t, err)

	var user User
	require.NoError(t, DB.First(&user, 10).Error)
	require.Equal(t, int64(-60), user.Quota)
	var entry WalletLedgerEntry
	require.NoError(t, DB.Where("operation_id = ?", "topup:LEGACY-DEBT-TOPUP").First(&entry).Error)
	require.Equal(t, int64(-60), entry.BalanceAfter)
}

func TestAdjustWalletAuditPayloadExplainsBalanceChange(t *testing.T) {
	setupTopupWalletTestDB(t)
	require.NoError(t, DB.Create(&User{Id: 13, Username: "wallet-audit", AffCode: "a13", Quota: 100, Status: common.UserStatusEnabled}).Error)

	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		_, err := AdjustWalletWithTx(tx, 13, 25, "wallet-audit-detail", "topup", WalletAdjustReference{
			Type: "topup", ID: "trade-13", Context: "manual",
		})
		return err
	}))

	var event AuditOutboxEvent
	require.NoError(t, DB.First(&event, "event_id = ?", "wallet:wallet-audit-detail").Error)
	var payload map[string]any
	require.NoError(t, common.UnmarshalJsonStr(event.Payload, &payload))
	require.Equal(t, float64(100), payload["balance_before"])
	require.Equal(t, float64(125), payload["balance_after"])
	require.Equal(t, float64(25), payload["delta"])
	require.NotEmpty(t, payload["balance_before_display"])
	require.NotEmpty(t, payload["balance_after_display"])
	require.NotEmpty(t, payload["delta_display"])
	require.Equal(t, "topup", payload["category"])
	require.Equal(t, "topup", payload["reference_type"])
	require.Equal(t, "trade-13", payload["reference_id"])
}

func TestAdjustWalletWithTxRejectsConflictingAuditMetadata(t *testing.T) {
	setupTopupWalletTestDB(t)
	require.NoError(t, DB.Create(&User{Id: 6, Username: "u6", AffCode: "a6", Quota: 100, Status: common.UserStatusEnabled}).Error)

	payload, err := common.Marshal(map[string]any{
		"target_user_id": 6,
		"operation_id":   "audit-metadata",
		"category":       "test",
		"delta":          5,
		"balance_after":  105,
		"reference_type": "case",
		"reference_id":   "6",
	})
	require.NoError(t, err)
	require.NoError(t, DB.Create(&AuditOutboxEvent{
		EventID:   "wallet:audit-metadata",
		EventType: "tampered.event",
		Result:    "success",
		ActorID:   6,
		Target:    "user:6",
		Payload:   string(payload),
		Status:    AuditOutboxStatusPending,
	}).Error)

	err = DB.Transaction(func(tx *gorm.DB) error {
		_, adjustErr := AdjustWalletWithTx(tx, 6, 5, "audit-metadata", "test", WalletAdjustReference{Type: "case", ID: "6"})
		return adjustErr
	})
	require.Error(t, err)

	var user User
	require.NoError(t, DB.First(&user, 6).Error)
	require.Equal(t, int64(100), user.Quota)
}

func TestOverrideWalletQuotaWithTxReturnsPreviousBalance(t *testing.T) {
	setupTopupWalletTestDB(t)
	require.NoError(t, DB.Create(&User{Id: 7, Username: "u7", AffCode: "a7", Quota: 100, Status: common.UserStatusEnabled}).Error)

	var previous int64
	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		var overrideErr error
		previous, overrideErr = OverrideWalletQuotaWithTx(tx, 7, 250, "admin:quota:override")
		return overrideErr
	}))
	require.Equal(t, int64(100), previous)

	var user User
	require.NoError(t, DB.First(&user, 7).Error)
	require.Equal(t, int64(250), user.Quota)
	var entry WalletLedgerEntry
	require.NoError(t, DB.Where("operation_id = ?", "admin:quota:override").First(&entry).Error)
	require.Equal(t, int64(150), entry.Delta)
	require.Equal(t, "admin_adjust", entry.Category)
}
