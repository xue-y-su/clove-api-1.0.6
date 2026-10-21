package service

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestListWalletLedgerMySQLStrictGrouping(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("TEST_MYSQL_DSN"))
	if dsn == "" {
		t.Skip("set TEST_MYSQL_DSN to run the MySQL wallet ledger integration test")
	}

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.Exec("SET SESSION sql_mode = CONCAT_WS(',', @@sql_mode, 'ONLY_FULL_GROUP_BY')").Error)

	previousDB := model.DB
	previousLogDB := model.LOG_DB
	previousMainType := common.MainDatabaseType()
	previousLogType := common.LogDatabaseType()
	model.DB = db
	model.LOG_DB = db
	common.SetDatabaseTypes(common.DatabaseTypeMySQL, common.DatabaseTypeMySQL)

	suffix := fmt.Sprintf("wallet-strict-%x", time.Now().UnixNano())
	user := model.User{
		Username: suffix,
		Password: "integration-test-hash",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
		AffCode:  suffix,
		Quota:    100,
	}
	require.NoError(t, model.DB.Create(&user).Error)
	t.Cleanup(func() {
		_ = model.DB.Where("user_id = ?", user.Id).Delete(&model.WalletLedgerEntry{}).Error
		_ = model.DB.Unscoped().Where("id = ?", user.Id).Delete(&model.User{}).Error
		if sqlDB, sqlErr := db.DB(); sqlErr == nil {
			_ = sqlDB.Close()
		}
		model.DB = previousDB
		model.LOG_DB = previousLogDB
		common.SetDatabaseTypes(previousMainType, previousLogType)
	})

	require.NoError(t, model.DB.Create(&[]model.WalletLedgerEntry{
		{UserID: user.Id, OperationID: suffix + ":reserve", Category: "reserve", Delta: -30, BalanceAfter: 70, ReferenceType: "relay_request", ReferenceID: suffix, CreatedAt: 100},
		{UserID: user.Id, OperationID: suffix + ":settle", Category: "settlement", Delta: 10, BalanceAfter: 80, ReferenceType: "relay_request", ReferenceID: suffix, CreatedAt: 200},
	}).Error)

	page, err := ListWalletLedgerWithEntries(context.Background(), user.Id, 1, 10, true)
	require.NoError(t, err)
	require.EqualValues(t, 1, page.Total)
	require.Len(t, page.Items, 1)
	require.EqualValues(t, -20, page.Items[0].Delta)
	require.EqualValues(t, 80, page.Items[0].BalanceAfter)
	require.Equal(t, "settlement", page.Items[0].Category)
	require.EqualValues(t, 2, page.Items[0].EntryCount)
	require.Len(t, page.Items[0].Entries, 2)
}
