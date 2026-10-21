package model

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestSetupStoresLongVersionMySQL(t *testing.T) {
	dsn := os.Getenv("TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set TEST_MYSQL_DSN to run mysql setup compatibility test")
	}

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	defer sqlDB.Close()

	require.NoError(t, db.Migrator().DropTable(&Setup{}))
	require.NoError(t, db.AutoMigrate(&Setup{}))

	version := "v1.0.0-rc.21.clove-enhanced"
	require.NoError(t, db.Create(&Setup{Version: version, InitializedAt: 1}).Error)

	var setup Setup
	require.NoError(t, db.First(&setup).Error)
	require.Equal(t, version, setup.Version)
}
