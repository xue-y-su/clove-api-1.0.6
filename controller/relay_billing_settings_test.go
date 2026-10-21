package controller

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/ip_security_setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func setupRelayBillingSettingsControllerTest(t *testing.T) {
	t.Helper()
	db := openTokenControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(
		&model.Option{}, &model.User{}, &model.Log{},
		&model.IPSecurityEvent{}, &model.IPAuditRollup{}, &model.IPSecurityProjectionState{},
	))

	previousOptions := common.OptionMap
	previousAlpha := *operation_setting.GetAlphaSearchSetting()
	previousDiagnostic := *operation_setting.GetUpstreamDiagnosticSetting()
	previousToolPricing := operation_setting.CurrentToolPriceSetting()
	common.OptionMapRWMutex.Lock()
	common.OptionMap = map[string]string{ip_security_setting.EnabledKey: "false"}
	common.OptionMapRWMutex.Unlock()

	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = previousOptions
		common.OptionMapRWMutex.Unlock()
		*operation_setting.GetAlphaSearchSetting() = previousAlpha
		operation_setting.RebuildAlphaSearchSnapshot()
		*operation_setting.GetUpstreamDiagnosticSetting() = previousDiagnostic
		operation_setting.RebuildUpstreamDiagnosticSnapshot()
		encoded, err := json.Marshal(previousToolPricing)
		if err == nil {
			_ = operation_setting.ApplyToolPriceSetting(string(encoded))
		}
	})
}

func validRelayBillingControllerSettings() dto.RelayBillingSettings {
	return dto.RelayBillingSettings{
		AlphaSearch: dto.AlphaSearchDocument{Enabled: true},
		ToolPricing: operation_setting.ToolPriceSetting{
			Enabled:                 true,
			Prices:                  map[string]float64{"web_search": 3.5},
			MultiplierMode:          operation_setting.ToolMultiplierGroupOnly,
			ImagePricingMode:        operation_setting.ImagePricingMatrix,
			ImagePriceMatrix:        map[string]map[string]float64{"low": {"1024x1024": 1.2}},
			ImageModelModeOverrides: map[string]string{"gpt-image-": operation_setting.ImagePricingFlat},
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

func newRawJSONContext(t *testing.T, method, target, body string, userID int) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(method, target, bytes.NewBufferString(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Set("id", userID)
	return ctx, recorder
}

func TestGetRelayBillingSettingsReturnsCurrentSnapshot(t *testing.T) {
	setupRelayBillingSettingsControllerTest(t)
	settings := validRelayBillingControllerSettings()
	_, err := service.UpdateRelayBillingSettings(settings)
	require.NoError(t, err)

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/option/relay-billing-settings", nil, 7)
	GetRelayBillingSettings(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	var got dto.RelayBillingSettings
	require.NoError(t, common.Unmarshal(response.Data, &got))
	require.Equal(t, settings, got)
}

func TestUpdateRelayBillingSettingsPersistsAndAuditsOnce(t *testing.T) {
	setupRelayBillingSettingsControllerTest(t)
	require.NoError(t, model.DB.Create(&model.User{
		Id: 42, Username: "relay-admin", Password: "password", Role: common.RoleRootUser,
		Status: common.UserStatusEnabled,
	}).Error)
	settings := validRelayBillingControllerSettings()
	ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/option/relay-billing-settings", settings, 42)
	ctx.Set("username", "relay-admin")
	ctx.Set("role", common.RoleRootUser)

	UpdateRelayBillingSettings(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	var returned dto.RelayBillingSettings
	require.NoError(t, common.Unmarshal(response.Data, &returned))
	require.Equal(t, settings, returned)

	var options int64
	require.NoError(t, model.DB.Model(&model.Option{}).Count(&options).Error)
	require.Equal(t, int64(12), options)
	var audits []model.Log
	require.NoError(t, model.LOG_DB.Where("type = ?", model.LogTypeManage).Find(&audits).Error)
	require.Len(t, audits, 1)
	require.Equal(t, "Updated relay billing settings", audits[0].Content)
	require.Contains(t, audits[0].Other, `"action":"option.update.relay_billing"`)
	require.Contains(t, audits[0].Other, `"admin_username":"relay-admin"`)
}

func TestUpdateRelayBillingSettingsRejectsMalformedJSONWithoutWriteOrAudit(t *testing.T) {
	setupRelayBillingSettingsControllerTest(t)
	ctx, recorder := newRawJSONContext(t, http.MethodPut, "/api/option/relay-billing-settings", "{", 7)

	UpdateRelayBillingSettings(ctx)

	response := decodeAPIResponse(t, recorder)
	require.False(t, response.Success)
	var options int64
	require.NoError(t, model.DB.Model(&model.Option{}).Count(&options).Error)
	require.Zero(t, options)
	var audits int64
	require.NoError(t, model.LOG_DB.Model(&model.Log{}).Where("type = ?", model.LogTypeManage).Count(&audits).Error)
	require.Zero(t, audits)
}

func TestRelayBillingAuditTemplateRemainsStable(t *testing.T) {
	require.Equal(t, "Updated relay billing settings", auditContentEN("option.update.relay_billing", nil))
	require.NotContains(t, strings.ToLower(auditContentEN("option.update.relay_billing", nil)), "undefined")
}
