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

func setupPrefillGroupModelTestDB(t *testing.T) {
	t.Helper()
	previousDB := DB
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:prefill_group_%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{
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
	require.NoError(t, db.AutoMigrate(&PrefillGroup{}))
}

func TestPrefillGroupUpdatePreservesCreatedTime(t *testing.T) {
	setupPrefillGroupModelTestDB(t)

	group := PrefillGroup{
		Name:        "Original group",
		Type:        "model",
		Items:       JSONValue(`["model-a"]`),
		Description: "Original description",
	}
	require.NoError(t, group.Insert())
	require.NotZero(t, group.CreatedTime)

	update := PrefillGroup{
		Id:          group.Id,
		Name:        "Updated group",
		Type:        "tag",
		Items:       JSONValue(`["tag-a"]`),
		Description: "Updated description",
	}
	require.NoError(t, update.Update())

	var persisted PrefillGroup
	require.NoError(t, DB.First(&persisted, group.Id).Error)
	require.Equal(t, group.CreatedTime, persisted.CreatedTime)
	require.NotZero(t, persisted.UpdatedTime)
	require.Equal(t, "Updated group", persisted.Name)
	require.Equal(t, "tag", persisted.Type)
	require.JSONEq(t, `["tag-a"]`, string(persisted.Items))
	require.Equal(t, "Updated description", persisted.Description)
}
