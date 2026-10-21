package model

import (
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupModelVendorMetaTestDB(t *testing.T) {
	t.Helper()
	previousDB := DB
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:model_vendor_meta_%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	DB = db
	t.Cleanup(func() {
		DB = previousDB
		_ = sqlDB.Close()
	})
	require.NoError(t, db.AutoMigrate(&Vendor{}, &Model{}))
}

func TestVendorInsertAndUpdatePreservePersistedMetadata(t *testing.T) {
	setupModelVendorMetaTestDB(t)

	vendor := Vendor{
		Name:        "Disabled vendor",
		Description: "Original description",
		Status:      0,
	}
	require.NoError(t, vendor.Insert())
	require.NotZero(t, vendor.CreatedTime)
	require.Equal(t, 0, vendor.Status)

	createdTime := vendor.CreatedTime
	update := Vendor{
		Id:          vendor.Id,
		Name:        "Updated vendor",
		Description: "Updated description",
		Status:      1,
	}
	require.NoError(t, update.Update())
	require.Equal(t, createdTime, update.CreatedTime)
	require.NotZero(t, update.UpdatedTime)

	var persisted Vendor
	require.NoError(t, DB.First(&persisted, vendor.Id).Error)
	require.Equal(t, createdTime, persisted.CreatedTime)
	require.Equal(t, "Updated vendor", persisted.Name)
	require.Equal(t, "Updated description", persisted.Description)
	require.Equal(t, 1, persisted.Status)
}

func TestModelInsertAndUpdateReturnPersistedMetadata(t *testing.T) {
	setupModelVendorMetaTestDB(t)

	model := Model{
		ModelName:    "qa-model",
		Description:  "Original description",
		Status:       0,
		SyncOfficial: 0,
	}
	require.NoError(t, model.Insert())
	require.NotZero(t, model.CreatedTime)
	require.Equal(t, 0, model.Status)
	require.Equal(t, 0, model.SyncOfficial)

	createdTime := model.CreatedTime
	update := Model{
		Id:           model.Id,
		ModelName:    "qa-model-updated",
		Description:  "Updated description",
		Status:       1,
		SyncOfficial: 0,
	}
	require.NoError(t, update.Update())
	require.Equal(t, createdTime, update.CreatedTime)
	require.NotZero(t, update.UpdatedTime)
	require.Equal(t, "qa-model-updated", update.ModelName)
	require.Equal(t, "Updated description", update.Description)
	require.Equal(t, 1, update.Status)
	require.Equal(t, 0, update.SyncOfficial)

	var persisted Model
	require.NoError(t, DB.First(&persisted, model.Id).Error)
	require.Equal(t, createdTime, persisted.CreatedTime)
	require.Equal(t, update.ModelName, persisted.ModelName)
	require.Equal(t, update.Description, persisted.Description)
	require.Equal(t, update.Status, persisted.Status)
	require.Equal(t, update.SyncOfficial, persisted.SyncOfficial)
}
