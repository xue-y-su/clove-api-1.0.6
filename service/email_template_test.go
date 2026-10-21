package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupEmailTemplateTestDB(t *testing.T) {
	t.Helper()
	prevDB := model.DB
	prevLogDB := model.LOG_DB
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:email_tpl_%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	model.DB = db
	model.LOG_DB = db
	t.Cleanup(func() {
		model.DB = prevDB
		model.LOG_DB = prevLogDB
		_ = sqlDB.Close()
	})
	require.NoError(t, db.AutoMigrate(&model.EmailTemplate{}))
}

func TestEmailTemplateBuiltinAndSetDefault(t *testing.T) {
	setupEmailTemplateTestDB(t)
	common.SystemName = "CloveAPI"
	items, err := ListEmailTemplates(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, items)
	require.Equal(t, model.EmailTemplateKindBuiltin, items[0].Kind)

	custom, err := CreateEmailTemplate(context.Background(), EmailTemplateInput{
		Name:     "Custom",
		Subject:  "Hi {{site_name}}",
		HTMLBody: "<p>code {{code}}</p>",
	})
	require.NoError(t, err)
	require.NoError(t, SetDefaultEmailTemplate(context.Background(), custom.ID))
	def, err := model.GetDefaultEmailTemplate()
	require.NoError(t, err)
	require.Equal(t, custom.ID, def.ID)
}

func TestEmailTemplateBuiltinUsesChineseDefaults(t *testing.T) {
	setupEmailTemplateTestDB(t)
	previousSystemName := common.SystemName
	previousLegacyTemplate := common.EmailVerificationTemplate
	common.SystemName = "CloveAPI"
	common.EmailVerificationTemplate = ""
	t.Cleanup(func() {
		common.SystemName = previousSystemName
		common.EmailVerificationTemplate = previousLegacyTemplate
	})

	require.NoError(t, EnsureBuiltinEmailTemplates(context.Background()))

	var builtin model.EmailTemplate
	require.NoError(t, model.DB.Where("builtin_key = ?", builtinVerificationKey).First(&builtin).Error)
	require.Equal(t, "默认邮箱验证模板", builtin.Name)
	require.Equal(t, "{{site_name}} 邮箱验证", builtin.Subject)
	require.Contains(t, builtin.HTMLBody, "您的邮箱验证码是")
	require.Contains(t, builtin.HTMLBody, "有效期为 {{expiration_minutes}} 分钟")

	subject, body, err := RenderActiveVerificationEmail("654321")
	require.NoError(t, err)
	require.Equal(t, "CloveAPI 邮箱验证", subject)
	require.Contains(t, body, "654321")
}

func TestEmailTemplateMigrationUsesChineseMetadata(t *testing.T) {
	setupEmailTemplateTestDB(t)
	previousLegacyTemplate := common.EmailVerificationTemplate
	common.EmailVerificationTemplate = "<p>验证码：{{code}}</p>"
	t.Cleanup(func() {
		common.EmailVerificationTemplate = previousLegacyTemplate
	})

	require.NoError(t, EnsureBuiltinEmailTemplates(context.Background()))

	var migrated model.EmailTemplate
	require.NoError(t, model.DB.Where("migrated_from = ?", "EmailVerificationTemplate").First(&migrated).Error)
	require.Equal(t, "从旧设置迁移的邮箱验证模板", migrated.Name)
	require.Equal(t, "{{site_name}} 邮箱验证", migrated.Subject)
}

func TestEmailTemplateEnsureNormalizesExistingBuiltinMetadata(t *testing.T) {
	setupEmailTemplateTestDB(t)
	previousLegacyTemplate := common.EmailVerificationTemplate
	common.EmailVerificationTemplate = ""
	t.Cleanup(func() {
		common.EmailVerificationTemplate = previousLegacyTemplate
	})

	require.NoError(t, model.DB.Create(&model.EmailTemplate{
		Name:       "Default verification",
		Kind:       model.EmailTemplateKindBuiltin,
		Subject:    "{{site_name}} email verification",
		HTMLBody:   "<p>Old English body {{code}}</p>",
		IsDefault:  true,
		Enabled:    true,
		BuiltinKey: builtinVerificationKey,
	}).Error)

	require.NoError(t, EnsureBuiltinEmailTemplates(context.Background()))

	var builtin model.EmailTemplate
	require.NoError(t, model.DB.Where("builtin_key = ?", builtinVerificationKey).First(&builtin).Error)
	require.Equal(t, "默认邮箱验证模板", builtin.Name)
	require.Equal(t, "{{site_name}} 邮箱验证", builtin.Subject)
	require.Contains(t, builtin.HTMLBody, "您的邮箱验证码是")
	require.True(t, builtin.IsDefault)
	require.True(t, builtin.Enabled)
}

func TestEmailTemplateRejectsScript(t *testing.T) {
	setupEmailTemplateTestDB(t)
	_, err := CreateEmailTemplate(context.Background(), EmailTemplateInput{
		Name:     "bad",
		Subject:  "s",
		HTMLBody: `<script>alert(1)</script>`,
	})
	require.ErrorIs(t, err, ErrEmailTemplateInvalid)
}

func TestRenderActiveVerificationEmail(t *testing.T) {
	setupEmailTemplateTestDB(t)
	common.SystemName = "CloveAPI"
	common.VerificationValidMinutes = 10
	require.NoError(t, EnsureBuiltinEmailTemplates(context.Background()))
	subject, body, err := RenderActiveVerificationEmail("654321")
	require.NoError(t, err)
	require.Contains(t, subject, "CloveAPI")
	require.Contains(t, body, "654321")
	require.NotContains(t, body, "<script")
}
