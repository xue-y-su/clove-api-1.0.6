package model

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestCheckinExpirationLifecycleMySQL(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("TEST_MYSQL_DSN"))
	if dsn == "" {
		t.Skip("set TEST_MYSQL_DSN to run the MySQL check-in expiration integration test")
	}

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&User{}, &Checkin{}, &WalletLedgerEntry{}, &AuditOutboxEvent{},
	))

	previousDB := DB
	previousLogDB := LOG_DB
	previousRedisEnabled := common.RedisEnabled
	previousMainType := common.MainDatabaseType()
	previousLogType := common.LogDatabaseType()
	DB = db
	LOG_DB = db
	common.RedisEnabled = false
	common.SetDatabaseTypes(common.DatabaseTypeMySQL, common.DatabaseTypeMySQL)

	userIDs := make([]int, 0, 2)
	t.Cleanup(func() {
		if len(userIDs) > 0 {
			_ = DB.Where("actor_id IN ?", userIDs).Delete(&AuditOutboxEvent{}).Error
			_ = DB.Where("user_id IN ?", userIDs).Delete(&WalletLedgerEntry{}).Error
			_ = DB.Where("user_id IN ?", userIDs).Delete(&Checkin{}).Error
			_ = DB.Unscoped().Where("id IN ?", userIDs).Delete(&User{}).Error
		}
		closeTestDatabase(DB)
		DB = previousDB
		LOG_DB = previousLogDB
		common.RedisEnabled = previousRedisEnabled
		common.SetDatabaseTypes(previousMainType, previousLogType)
	})

	suffix := fmt.Sprintf("ci%x", time.Now().UnixNano())
	primary := User{
		Username: suffix,
		Password: "integration-test-hash",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
		AffCode:  suffix,
		Quota:    200,
	}
	require.NoError(t, DB.Create(&primary).Error)
	userIDs = append(userIDs, primary.Id)

	reward := Checkin{
		UserId:         primary.Id,
		CheckinDate:    "2099-01-01",
		QuotaAwarded:   100,
		QuotaRemaining: 100,
		ExpiresAt:      time.Now().Add(time.Hour).Unix(),
		CreatedAt:      time.Now().Unix(),
	}
	require.NoError(t, DB.Create(&reward).Error)

	adjust := func(delta int64, operationID, category string) {
		t.Helper()
		require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
			_, adjustErr := AdjustWalletWithTx(tx, primary.Id, delta, operationID, category, WalletAdjustReference{
				Type: "mysql_checkin_test",
				ID:   suffix,
			})
			return adjustErr
		}))
	}

	adjust(-30, suffix+":debit", "user_redemption_reserve")
	require.NoError(t, DB.First(&reward, reward.Id).Error)
	require.EqualValues(t, 70, reward.QuotaRemaining)

	adjust(25, suffix+":credit", "topup")
	require.NoError(t, DB.First(&reward, reward.Id).Error)
	require.EqualValues(t, 70, reward.QuotaRemaining, "credits must not restore consumed reward")

	require.NoError(t, DB.Model(&Checkin{}).Where("id = ?", reward.Id).
		UpdateColumn("expires_at", time.Now().Add(-time.Minute).Unix()).Error)
	settled, err := SettleDueCheckinRewards(10)
	require.NoError(t, err)
	require.Equal(t, 1, settled)
	settled, err = SettleDueCheckinRewards(10)
	require.NoError(t, err)
	require.Zero(t, settled, "settlement must be idempotent")

	var updatedPrimary User
	require.NoError(t, DB.First(&updatedPrimary, primary.Id).Error)
	require.EqualValues(t, 125, updatedPrimary.Quota)
	require.NoError(t, DB.First(&reward, reward.Id).Error)
	require.Zero(t, reward.QuotaRemaining)
	require.EqualValues(t, 70, reward.ReclaimedQuota)
	require.NotZero(t, reward.SettledAt)

	var expiryLedger WalletLedgerEntry
	require.NoError(t, DB.Where("operation_id = ?", fmt.Sprintf("checkin-expiry:%d", reward.Id)).First(&expiryLedger).Error)
	require.Equal(t, "checkin_expiry", expiryLedger.Category)
	require.EqualValues(t, -70, expiryLedger.Delta)

	lowBalance := User{
		Username: suffix + "b",
		Password: "integration-test-hash",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
		AffCode:  suffix + "b",
		Quota:    20,
	}
	require.NoError(t, DB.Create(&lowBalance).Error)
	userIDs = append(userIDs, lowBalance.Id)
	lowBalanceReward := Checkin{
		UserId:         lowBalance.Id,
		CheckinDate:    "2099-01-02",
		QuotaAwarded:   60,
		QuotaRemaining: 60,
		ExpiresAt:      time.Now().Add(-time.Minute).Unix(),
		CreatedAt:      time.Now().Unix(),
	}
	require.NoError(t, DB.Create(&lowBalanceReward).Error)
	settled, err = SettleDueCheckinRewards(10)
	require.NoError(t, err)
	require.Equal(t, 1, settled)
	require.NoError(t, DB.First(&lowBalance, lowBalance.Id).Error)
	require.Zero(t, lowBalance.Quota)
	require.NoError(t, DB.First(&lowBalanceReward, lowBalanceReward.Id).Error)
	require.EqualValues(t, 20, lowBalanceReward.ReclaimedQuota)
}
