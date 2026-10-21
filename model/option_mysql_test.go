package model

import (
	"os"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/model_catalog_setting"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestLoadOptionsFromDatabaseRepairsLegacySMTPValueOnMySQL(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("TEST_MYSQL_DSN"))
	if dsn == "" {
		t.Skip("set TEST_MYSQL_DSN to run MySQL option query compatibility test")
	}

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Option{}))
	require.NoError(t, db.Unscoped().Where(&Option{Key: "SMTPServer"}).Delete(&Option{}).Error)

	previousDB := DB
	previousOptionMap := common.OptionMap
	previousKeyCol := commonKeyCol
	DB = db
	common.OptionMap = map[string]string{}
	commonKeyCol = `"key"`
	t.Cleanup(func() {
		DB = previousDB
		common.OptionMap = previousOptionMap
		commonKeyCol = previousKeyCol
		_ = db.Unscoped().Where(&Option{Key: "SMTPServer"}).Delete(&Option{}).Error
		_ = sqlDB.Close()
	})

	require.NoError(t, db.Create(&Option{Key: "SMTPServer", Value: "<nil>"}).Error)
	loadOptionsFromDatabase()

	var reloaded Option
	require.NoError(t, db.First(&reloaded, "`key` = ?", "SMTPServer").Error)
	require.Empty(t, reloaded.Value)
}

func TestCatalogVisibilityOptionLookupQuotesReservedKeyOnMySQL(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("TEST_MYSQL_DSN"))
	if dsn == "" {
		t.Skip("set TEST_MYSQL_DSN to run MySQL catalog visibility compatibility test")
	}

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Option{}))
	const optionKey = model_catalog_setting.OptionKey
	require.NoError(t, db.Unscoped().Where("`key` = ?", optionKey).Delete(&Option{}).Error)

	previousDB := DB
	previousKeyCol := commonKeyCol
	DB = db
	commonKeyCol = "`key`"
	t.Cleanup(func() {
		DB = previousDB
		commonKeyCol = previousKeyCol
		_ = db.Unscoped().Where("`key` = ?", optionKey).Delete(&Option{}).Error
		_ = sqlDB.Close()
	})

	require.NoError(t, db.Create(&Option{Key: optionKey, Value: `{"version":1,"mode":"all","groups":[]}`}).Error)
	raw, exists, err := loadPersistedCatalogVisibility()
	require.NoError(t, err)
	require.True(t, exists)
	require.JSONEq(t, `{"version":1,"mode":"all","groups":[]}`, raw)
}
