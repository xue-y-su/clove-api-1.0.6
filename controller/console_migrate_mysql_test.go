package controller

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestMigrateConsoleSettingDeletesLegacyOptionsOnMySQL(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("TEST_MYSQL_DSN"))
	if dsn == "" {
		t.Skip("set TEST_MYSQL_DSN to run MySQL console migration compatibility test")
	}

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Option{}))
	require.NoError(t, db.Unscoped().Where(&model.Option{Key: "Announcements"}).Delete(&model.Option{}).Error)

	previousDB := model.DB
	previousOptionMap := common.OptionMap
	model.DB = db
	common.OptionMap = map[string]string{}
	t.Cleanup(func() {
		model.DB = previousDB
		common.OptionMap = previousOptionMap
		_ = db.Unscoped().Where(&model.Option{Key: "Announcements"}).Delete(&model.Option{}).Error
		_ = sqlDB.Close()
	})

	require.NoError(t, db.Create(&model.Option{Key: "Announcements", Value: "legacy announcement"}).Error)
	gin.SetMode(gin.TestMode)
	writer := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(writer)
	MigrateConsoleSetting(context)

	require.Equal(t, http.StatusOK, writer.Code)
	var legacyOptionCount int64
	require.NoError(t, db.Model(&model.Option{}).Where(&model.Option{Key: "Announcements"}).Count(&legacyOptionCount).Error)
	require.Zero(t, legacyOptionCount)
}
