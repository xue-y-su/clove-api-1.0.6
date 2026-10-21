package controller

import (
	"net/http"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/compliance_setting"
	"github.com/QuantumNous/new-api/setting/registration_setting"
	"github.com/QuantumNous/new-api/setting/source_attribution_setting"
	"github.com/stretchr/testify/require"
)

func seedComplianceControllerOptions(t *testing.T) {
	t.Helper()

	common.OptionMapRWMutex.Lock()
	previous := common.OptionMap
	common.OptionMap = map[string]string{
		compliance_setting.StatementEnabledKey:           "false",
		compliance_setting.StatementVersionKey:           "",
		compliance_setting.StatementContentKey:           "",
		compliance_setting.StatementItemsKey:             "[]",
		compliance_setting.StatementDisplayModeKey:       compliance_setting.DisplayModeSingle,
		compliance_setting.TokenAPIEnforcementEnabledKey: "false",
		compliance_setting.ShowVersionToUserKey:          "false",
		compliance_setting.ShowContentHashToUserKey:      "false",
	}
	common.OptionMapRWMutex.Unlock()

	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = previous
		common.OptionMapRWMutex.Unlock()
	})
}

func setupComplianceControllerTest(t *testing.T) {
	t.Helper()

	db := openTokenControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Option{}, &model.User{}, &model.Log{}, &model.ComplianceStatementSnapshot{}, &model.ComplianceConfirmation{}))
	seedComplianceControllerOptions(t)
}

func TestUpdateOptionRejectsComplianceNamespace(t *testing.T) {
	setupComplianceControllerTest(t)
	ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/option/", map[string]any{
		"key":   compliance_setting.StatementEnabledKey,
		"value": true,
	}, 1)

	UpdateOption(ctx)

	response := decodeAPIResponse(t, recorder)
	require.False(t, response.Success)
	require.Contains(t, response.Message, "合规声明专用接口")
}

func TestGetOptionsDoesNotExposeComplianceNamespace(t *testing.T) {
	setupComplianceControllerTest(t)
	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/option/", nil, 1)

	GetOptions(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	require.NotContains(t, string(response.Data), "compliance.")
}

func TestUpdateComplianceSettingsValidatesEnabledStatement(t *testing.T) {
	setupComplianceControllerTest(t)
	ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/compliance/settings", map[string]any{
		"statement_enabled": true,
		"statement_version": "",
		"statement_content": "",
	}, 1)
	ctx.Set("username", "root")
	ctx.Set("role", common.RoleRootUser)

	UpdateComplianceSettings(ctx)

	response := decodeAPIResponse(t, recorder)
	require.False(t, response.Success)
	require.Contains(t, response.Message, "版本")
}

func TestUpdateComplianceSettingsPersistsAndAuditsWithoutStatementBody(t *testing.T) {
	setupComplianceControllerTest(t)
	require.NoError(t, model.DB.Create(&model.User{
		Id:       1,
		Username: "root",
		Password: "password",
		Role:     common.RoleRootUser,
		Status:   common.UserStatusEnabled,
	}).Error)
	ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/compliance/settings", map[string]any{
		"statement_enabled":             true,
		"statement_version":             "2026-07",
		"statement_content":             "这是不能进入审计日志的声明正文",
		"token_api_enforcement_enabled": true,
	}, 1)
	ctx.Set("username", "root")
	ctx.Set("role", common.RoleRootUser)
	ctx.Request.RemoteAddr = "203.0.113.20:12345"

	UpdateComplianceSettings(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	settings := compliance_setting.GetSettings()
	require.True(t, settings.StatementEnabled)
	require.True(t, settings.TokenAPIEnforcementEnabled)
	require.Equal(t, "2026-07", settings.StatementVersion)

	var auditLog model.Log
	require.NoError(t, model.LOG_DB.First(&auditLog, "type = ?", model.LogTypeManage).Error)
	require.Contains(t, auditLog.Other, "compliance.settings_update")
	require.Contains(t, auditLog.Other, "content_hash_to")
	require.NotContains(t, auditLog.Other, "这是不能进入审计日志的声明正文")
}

func TestConfirmComplianceStatementIsIdempotentAndStoresRequestMetadata(t *testing.T) {
	setupComplianceControllerTest(t)
	require.NoError(t, model.DB.Create(&model.User{
		Id:       7,
		Username: "alice",
		Password: "password",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
	}).Error)
	require.NoError(t, model.UpdateOptionsBulk(map[string]string{
		compliance_setting.StatementEnabledKey: "true",
		compliance_setting.StatementVersionKey: "2026-07",
		compliance_setting.StatementContentKey: "声明正文",
	}))

	firstCtx, firstRecorder := newAuthenticatedContext(t, http.MethodPost, "/api/compliance/confirm", nil, 7)
	firstCtx.Set("username", "alice")
	firstCtx.Request.RemoteAddr = "203.0.113.30:12345"
	firstCtx.Request.Header.Set("User-Agent", "first-agent")
	ConfirmComplianceStatement(firstCtx)
	require.True(t, decodeAPIResponse(t, firstRecorder).Success)

	secondCtx, secondRecorder := newAuthenticatedContext(t, http.MethodPost, "/api/compliance/confirm", nil, 7)
	secondCtx.Set("username", "alice")
	secondCtx.Request.RemoteAddr = "203.0.113.31:12345"
	secondCtx.Request.Header.Set("User-Agent", "second-agent")
	ConfirmComplianceStatement(secondCtx)
	require.True(t, decodeAPIResponse(t, secondRecorder).Success)

	var confirmations []model.ComplianceConfirmation
	require.NoError(t, model.DB.Find(&confirmations).Error)
	require.Len(t, confirmations, 1)
	require.Equal(t, "203.0.113.31", confirmations[0].Ip)
	require.Equal(t, "second-agent", confirmations[0].UserAgent)

	var userAudit model.Log
	require.NoError(t, model.LOG_DB.First(&userAudit, "user_id = ? AND type = ?", 7, model.LogTypeManage).Error)
	require.True(t, strings.Contains(userAudit.Other, "compliance.confirm"))
}

func TestGetComplianceConfirmationsReturnsPaginatedRecords(t *testing.T) {
	setupComplianceControllerTest(t)
	require.NoError(t, model.RecordComplianceConfirmation(7, "alice", "2026-07", "hash-a", "203.0.113.30", "ua", 100))
	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/compliance/confirmations?p=1&page_size=10&username=alice", nil, 1)
	ctx.Set("role", common.RoleAdminUser)

	GetComplianceConfirmations(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	require.Contains(t, string(response.Data), "alice")
	require.Contains(t, string(response.Data), "203.0.113.*")
	require.NotContains(t, string(response.Data), "203.0.113.30")
}

func TestGetComplianceStatementIncludesCurrentConfirmationStatus(t *testing.T) {
	setupComplianceControllerTest(t)
	require.NoError(t, model.UpdateOptionsBulk(map[string]string{
		compliance_setting.StatementEnabledKey:      "true",
		compliance_setting.StatementVersionKey:      "2026-07",
		compliance_setting.ShowContentHashToUserKey: "true",
		compliance_setting.StatementContentKey:      "声明正文",
	}))
	hash := compliance_setting.ContentHash("声明正文")
	require.NoError(t, model.RecordComplianceConfirmation(7, "alice", "2026-07", hash, "203.0.113.30", "ua", 100))
	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/compliance/statement", nil, 7)

	GetComplianceStatement(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	require.Contains(t, string(response.Data), `"confirmed":true`)
	require.Contains(t, string(response.Data), hash)
}

func TestGetComplianceStatementHidesVersionAndHashByDefault(t *testing.T) {
	setupComplianceControllerTest(t)
	require.NoError(t, model.UpdateOptionsBulk(map[string]string{
		compliance_setting.StatementEnabledKey:     "true",
		compliance_setting.StatementVersionKey:     "2026-07",
		compliance_setting.StatementContentKey:     "legacy fallback",
		compliance_setting.StatementItemsKey:       `[{"item_key":"terms","title":"Terms","source_type":"manual","version":"2026-07","content":"terms body","sort_order":1,"confirmation_mode":"checkbox"}]`,
		compliance_setting.StatementDisplayModeKey: "multiple",
	}))

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/compliance/statement", nil, 7)

	GetComplianceStatement(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	body := string(response.Data)
	require.Contains(t, body, `"content":"legacy fallback"`)
	require.Contains(t, body, `"content":"terms body"`)
	require.Contains(t, body, `"version":""`)
	require.Contains(t, body, `"content_hash":""`)
	require.NotContains(t, body, `"version":"2026-07"`)
	require.NotContains(t, body, compliance_setting.ContentHash("legacy fallback"))
	require.NotContains(t, body, compliance_setting.ContentHash("terms body"))
}

func TestGetComplianceStatementShowsVersionAndHashWhenEnabled(t *testing.T) {
	setupComplianceControllerTest(t)
	require.NoError(t, model.UpdateOptionsBulk(map[string]string{
		compliance_setting.StatementEnabledKey:      "true",
		compliance_setting.StatementVersionKey:      "2026-07",
		compliance_setting.StatementContentKey:      "statement body",
		compliance_setting.ShowVersionToUserKey:     "true",
		compliance_setting.ShowContentHashToUserKey: "true",
	}))

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/compliance/statement", nil, 7)

	GetComplianceStatement(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	body := string(response.Data)
	require.Contains(t, body, `"version":"2026-07"`)
	require.Contains(t, body, compliance_setting.ContentHash("statement body"))
}

func TestHiddenComplianceStatementStillConfirmsWithRealHash(t *testing.T) {
	setupComplianceControllerTest(t)
	require.NoError(t, model.DB.Create(&model.User{
		Id:       7,
		Username: "alice",
		Password: "password",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
	}).Error)
	require.NoError(t, model.UpdateOptionsBulk(map[string]string{
		compliance_setting.StatementEnabledKey: "true",
		compliance_setting.StatementVersionKey: "2026-07",
		compliance_setting.StatementContentKey: "statement body",
	}))

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/compliance/confirm", nil, 7)
	ctx.Set("username", "alice")

	ConfirmComplianceStatement(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	require.NotContains(t, string(response.Data), "2026-07")
	require.NotContains(t, string(response.Data), compliance_setting.ContentHash("statement body"))
	var confirmation model.ComplianceConfirmation
	require.NoError(t, model.DB.First(&confirmation, "user_id = ?", 7).Error)
	require.Equal(t, "2026-07", confirmation.Version)
	require.Equal(t, compliance_setting.ContentHash("statement body"), confirmation.ContentHash)
}

func TestGetComplianceSettingsAlwaysReturnsRealVersionAndHash(t *testing.T) {
	setupComplianceControllerTest(t)
	require.NoError(t, model.UpdateOptionsBulk(map[string]string{
		compliance_setting.StatementEnabledKey:      "true",
		compliance_setting.StatementVersionKey:      "2026-07",
		compliance_setting.StatementContentKey:      "statement body",
		compliance_setting.ShowVersionToUserKey:     "false",
		compliance_setting.ShowContentHashToUserKey: "false",
	}))
	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/compliance/settings", nil, 1)
	ctx.Set("role", common.RoleRootUser)

	GetComplianceSettings(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	body := string(response.Data)
	require.Contains(t, body, `"statement_version":"2026-07"`)
	require.Contains(t, body, compliance_setting.ContentHash("statement body"))
	require.Contains(t, body, `"show_version_to_user":false`)
	require.Contains(t, body, `"show_content_hash_to_user":false`)
}

func TestUpdateComplianceSettingsPersistsUserDisplaySwitches(t *testing.T) {
	setupComplianceControllerTest(t)
	ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/compliance/settings", map[string]any{
		"statement_enabled":             false,
		"statement_version":             "",
		"statement_content":             "",
		"statement_items":               []any{},
		"statement_display_mode":        "single",
		"token_api_enforcement_enabled": false,
		"show_version_to_user":          true,
		"show_content_hash_to_user":     true,
	}, 1)
	ctx.Set("username", "root")
	ctx.Set("role", common.RoleRootUser)

	UpdateComplianceSettings(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	settings := compliance_setting.GetSettings()
	require.True(t, settings.ShowVersionToUser)
	require.True(t, settings.ShowContentHashToUser)
	require.Contains(t, string(response.Data), `"show_version_to_user":true`)
	require.Contains(t, string(response.Data), `"show_content_hash_to_user":true`)
}

func TestUpdateOptionRejectsAffAndSourceAttributionNamespaces(t *testing.T) {
	setupComplianceControllerTest(t)
	for _, key := range []string{
		registration_setting.AffRequiredKey,
		source_attribution_setting.EnabledKey,
	} {
		ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/option/", map[string]any{
			"key":   key,
			"value": true,
		}, 1)

		UpdateOption(ctx)

		response := decodeAPIResponse(t, recorder)
		require.False(t, response.Success)
		require.Contains(t, response.Message, "专用接口")
	}
}

func TestGetOptionsDoesNotExposeAffAndSourceAttributionNamespaces(t *testing.T) {
	setupComplianceControllerTest(t)
	common.OptionMapRWMutex.Lock()
	common.OptionMap[registration_setting.AffRequiredKey] = "true"
	common.OptionMap[source_attribution_setting.EnabledKey] = "true"
	common.OptionMapRWMutex.Unlock()
	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/option/", nil, 1)

	GetOptions(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	require.NotContains(t, string(response.Data), "registration.")
	require.NotContains(t, string(response.Data), "source_attribution.")
}

func TestComplianceSnapshotConfirmationsAreScopedPerSnapshot(t *testing.T) {
	setupComplianceControllerTest(t)
	require.NoError(t, model.DB.AutoMigrate(&model.ComplianceStatementSnapshot{}))
	require.NoError(t, model.UpdateOptionsBulk(map[string]string{
		compliance_setting.StatementEnabledKey:     "true",
		compliance_setting.StatementVersionKey:     "2026-07",
		compliance_setting.StatementContentKey:     "legacy fallback",
		compliance_setting.StatementItemsKey:       `[{"item_key":"terms","title":"Terms","source_type":"manual","version":"2026-07","content":"same text","sort_order":1,"confirmation_mode":"checkbox"},{"item_key":"privacy","title":"Privacy","source_type":"manual","version":"2026-07","content":"same text","sort_order":2,"confirmation_mode":"checkbox"}]`,
		compliance_setting.StatementDisplayModeKey: "multiple",
	}))
	snapshots, err := model.EnsureComplianceStatementSnapshots(compliance_setting.GetSettings())
	require.NoError(t, err)
	require.Len(t, snapshots, 2)

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/compliance/confirm", map[string]any{
		"snapshot_id": snapshots[0].Id,
	}, 7)
	ctx.Set("username", "alice")
	ConfirmComplianceStatement(ctx)
	require.True(t, decodeAPIResponse(t, recorder).Success)

	confirmed, err := model.HasAllComplianceSnapshotConfirmations(7, snapshots)
	require.NoError(t, err)
	require.False(t, confirmed)

	ctx, recorder = newAuthenticatedContext(t, http.MethodPost, "/api/compliance/confirm", map[string]any{
		"snapshot_id": snapshots[1].Id,
	}, 7)
	ctx.Set("username", "alice")
	ConfirmComplianceStatement(ctx)
	require.True(t, decodeAPIResponse(t, recorder).Success)

	confirmed, err = model.HasAllComplianceSnapshotConfirmations(7, snapshots)
	require.NoError(t, err)
	require.True(t, confirmed)
}

func TestComplianceSnapshotUsesCurrentUserAgreementContent(t *testing.T) {
	setupComplianceControllerTest(t)
	require.NoError(t, model.UpdateOption("legal.user_agreement", "terms v1"))
	require.NoError(t, model.UpdateOptionsBulk(map[string]string{
		compliance_setting.StatementEnabledKey:     "true",
		compliance_setting.StatementVersionKey:     "2026-07",
		compliance_setting.StatementContentKey:     "",
		compliance_setting.StatementItemsKey:       `[{"item_key":"terms","title":"Terms","source_type":"user_agreement","version":"2026-07","sort_order":1,"confirmation_mode":"checkbox"}]`,
		compliance_setting.StatementDisplayModeKey: "multiple",
	}))

	snapshots, err := model.EnsureComplianceStatementSnapshots(compliance_setting.GetSettings())

	require.NoError(t, err)
	require.Len(t, snapshots, 1)
	require.Equal(t, "terms v1", snapshots[0].Content)
	require.Equal(t, compliance_setting.ContentHash("terms v1"), snapshots[0].ContentHash)
}

func TestUpdatingReferencedUserAgreementPublishesNewComplianceSnapshot(t *testing.T) {
	setupComplianceControllerTest(t)
	require.NoError(t, model.UpdateOption("legal.user_agreement", "terms v1"))
	require.NoError(t, model.UpdateOptionsBulk(map[string]string{
		compliance_setting.StatementEnabledKey:     "true",
		compliance_setting.StatementVersionKey:     "2026-07",
		compliance_setting.StatementContentKey:     "",
		compliance_setting.StatementItemsKey:       `[{"item_key":"terms","title":"Terms","source_type":"user_agreement","version":"2026-07","sort_order":1,"confirmation_mode":"checkbox"}]`,
		compliance_setting.StatementDisplayModeKey: "multiple",
	}))
	firstSnapshots, err := model.EnsureComplianceStatementSnapshots(compliance_setting.GetSettings())
	require.NoError(t, err)
	require.Len(t, firstSnapshots, 1)
	require.NoError(t, model.RecordComplianceSnapshotConfirmation(7, "alice", firstSnapshots[0], "203.0.113.30", "ua", 100))

	require.NoError(t, model.UpdateOption("legal.user_agreement", "terms v2"))

	var allSnapshots []model.ComplianceStatementSnapshot
	require.NoError(t, model.DB.Order("id asc").Find(&allSnapshots, "item_key = ?", "terms").Error)
	require.Len(t, allSnapshots, 2)
	require.Equal(t, "terms v2", allSnapshots[1].Content)

	currentSnapshots, err := model.EnsureComplianceStatementSnapshots(compliance_setting.GetSettings())
	require.NoError(t, err)
	require.Len(t, currentSnapshots, 1)
	confirmed, err := model.HasAllComplianceSnapshotConfirmations(7, currentSnapshots)
	require.NoError(t, err)
	require.False(t, confirmed)
}

func TestUpdatingReferencedPrivacyPolicyPublishesNewComplianceSnapshot(t *testing.T) {
	setupComplianceControllerTest(t)
	require.NoError(t, model.UpdateOption("legal.privacy_policy", "privacy v1"))
	require.NoError(t, model.UpdateOptionsBulk(map[string]string{
		compliance_setting.StatementEnabledKey:     "true",
		compliance_setting.StatementVersionKey:     "2026-07",
		compliance_setting.StatementContentKey:     "",
		compliance_setting.StatementItemsKey:       `[{"item_key":"privacy","title":"Privacy","source_type":"privacy_policy","version":"2026-07","sort_order":1,"confirmation_mode":"checkbox"}]`,
		compliance_setting.StatementDisplayModeKey: "multiple",
	}))
	firstSnapshots, err := model.EnsureComplianceStatementSnapshots(compliance_setting.GetSettings())
	require.NoError(t, err)
	require.Len(t, firstSnapshots, 1)
	require.NoError(t, model.RecordComplianceSnapshotConfirmation(7, "alice", firstSnapshots[0], "203.0.113.30", "ua", 100))

	require.NoError(t, model.UpdateOption("legal.privacy_policy", "privacy v2"))

	var allSnapshots []model.ComplianceStatementSnapshot
	require.NoError(t, model.DB.Order("id asc").Find(&allSnapshots, "item_key = ?", "privacy").Error)
	require.Len(t, allSnapshots, 2)
	require.Equal(t, "privacy v2", allSnapshots[1].Content)

	currentSnapshots, err := model.EnsureComplianceStatementSnapshots(compliance_setting.GetSettings())
	require.NoError(t, err)
	require.Len(t, currentSnapshots, 1)
	confirmed, err := model.HasAllComplianceSnapshotConfirmations(7, currentSnapshots)
	require.NoError(t, err)
	require.False(t, confirmed)
}
