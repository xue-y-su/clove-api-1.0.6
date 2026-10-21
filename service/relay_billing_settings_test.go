package service

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func validRelayBillingSettings() dto.RelayBillingSettings {
	return dto.RelayBillingSettings{
		AlphaSearch: dto.AlphaSearchDocument{Enabled: true},
		ToolPricing: operation_setting.ToolPriceSetting{
			Enabled:          true,
			Prices:           map[string]float64{"web_search": 3.5},
			MultiplierMode:   operation_setting.ToolMultiplierGroupOnly,
			ImagePricingMode: operation_setting.ImagePricingMatrix,
			ImagePriceMatrix: map[string]map[string]float64{"low": {"1024x1024": 1.2}},
			ImageModelModeOverrides: map[string]string{
				"gpt-image-": operation_setting.ImagePricingFlat,
			},
		},
		UpstreamDiagnostic: operation_setting.UpstreamDiagnosticSetting{
			Enabled:             true,
			CaptureMode:         operation_setting.UpstreamDiagnosticCaptureAllErrors,
			PreviewChars:        1024,
			SyncToUsageLog:      true,
			DedupeWindowSeconds: 120,
		},
	}
}

func setupRelayBillingSettingsTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	previousDB := model.DB
	common.OptionMapRWMutex.Lock()
	previousOptionMap := common.OptionMap
	common.OptionMap = make(map[string]string)
	common.OptionMapRWMutex.Unlock()
	db, err := gorm.Open(sqlite.Open("file:relay_billing_settings?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	require.NoError(t, db.AutoMigrate(&model.Option{}))
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = previousOptionMap
		common.OptionMapRWMutex.Unlock()
		model.DB = previousDB
		sqlDB, dbErr := db.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func TestUpdateRelayBillingSettingsRejectsInvalidDocumentAtomically(t *testing.T) {
	db := setupRelayBillingSettingsTestDB(t)
	beforeTool := operation_setting.CurrentToolPricingSnapshot()
	beforeAlpha := operation_setting.CurrentAlphaSearchSnapshot()
	beforeDiagnostic := operation_setting.CurrentUpstreamDiagnosticSnapshot()

	tests := []struct {
		name string
		edit func(*dto.RelayBillingSettings)
	}{
		{name: "negative price", edit: func(settings *dto.RelayBillingSettings) { settings.ToolPricing.Prices["web_search"] = -1 }},
		{name: "invalid capture mode", edit: func(settings *dto.RelayBillingSettings) { settings.UpstreamDiagnostic.CaptureMode = "invalid" }},
		{name: "preview below minimum", edit: func(settings *dto.RelayBillingSettings) { settings.UpstreamDiagnostic.PreviewChars = 255 }},
		{name: "dedupe above maximum", edit: func(settings *dto.RelayBillingSettings) { settings.UpstreamDiagnostic.DedupeWindowSeconds = 3601 }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			settings := validRelayBillingSettings()
			test.edit(&settings)
			_, err := UpdateRelayBillingSettings(settings)
			require.Error(t, err)
			var count int64
			require.NoError(t, db.Model(&model.Option{}).Count(&count).Error)
			require.Zero(t, count)
			require.Same(t, beforeTool, operation_setting.CurrentToolPricingSnapshot())
			require.Equal(t, beforeAlpha, operation_setting.CurrentAlphaSearchSnapshot())
			require.Equal(t, beforeDiagnostic, operation_setting.CurrentUpstreamDiagnosticSnapshot())
		})
	}
}

func TestUpdateRelayBillingSettingsWritesOneCompleteDocument(t *testing.T) {
	db := setupRelayBillingSettingsTestDB(t)
	previous := CurrentRelayBillingSettings()
	t.Cleanup(func() {
		_, err := UpdateRelayBillingSettings(previous)
		require.NoError(t, err)
	})
	settings := validRelayBillingSettings()
	result, err := UpdateRelayBillingSettings(settings)
	require.NoError(t, err)
	require.Equal(t, settings, result)

	var count int64
	require.NoError(t, db.Model(&model.Option{}).Count(&count).Error)
	require.Equal(t, int64(12), count)
	current := CurrentRelayBillingSettings()
	require.Equal(t, settings.AlphaSearch, current.AlphaSearch)
	require.Equal(t, settings.ToolPricing, current.ToolPricing)
	require.Equal(t, settings.UpstreamDiagnostic, current.UpstreamDiagnostic)
}
