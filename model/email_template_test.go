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

func setupEmailTemplateModelTestDB(t *testing.T) {
	t.Helper()
	prevDB := DB
	prevLogDB := LOG_DB
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:email_tpl_model_%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	DB = db
	LOG_DB = db
	t.Cleanup(func() {
		DB = prevDB
		LOG_DB = prevLogDB
		_ = sqlDB.Close()
	})
	require.NoError(t, db.AutoMigrate(&EmailTemplate{}))
}

func TestEmailTemplateModelDefaultAndList(t *testing.T) {
	setupEmailTemplateModelTestDB(t)
	require.NoError(t, DB.Create(&EmailTemplate{
		Name:      "Builtin",
		Kind:      EmailTemplateKindBuiltin,
		Subject:   "s",
		HTMLBody:  "<p>{{code}}</p>",
		IsDefault: true,
		Enabled:   true,
		BuiltinKey: "default_verification",
	}).Error)
	require.NoError(t, DB.Create(&EmailTemplate{
		Name:     "Custom",
		Kind:     EmailTemplateKindCustom,
		Subject:  "c",
		HTMLBody: "<p>hi</p>",
		Enabled:  true,
	}).Error)

	def, err := GetDefaultEmailTemplate()
	require.NoError(t, err)
	require.Equal(t, "Builtin", def.Name)

	items, err := ListEmailTemplates()
	require.NoError(t, err)
	require.Len(t, items, 2)

	byID, err := GetEmailTemplateByID(def.ID)
	require.NoError(t, err)
	require.Equal(t, def.Name, byID.Name)
}
