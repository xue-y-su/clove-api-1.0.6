package controller

import (
	"net/http"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/source_attribution_setting"
	"github.com/stretchr/testify/require"
)

func setupSourceAttributionControllerTest(t *testing.T) {
	t.Helper()

	db := openTokenControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Option{}, &model.User{}, &model.Log{}, &model.SourceAttributionConfirmation{}))

	optionsBytes, err := common.Marshal([]source_attribution_setting.Option{
		{
			Id:                  "xiaoc",
			Name:                "Xiao C",
			TargetAffCode:       "abcd",
			RequiresDescription: false,
			SortOrder:           1,
		},
		{
			Id:                  "other",
			Name:                "Other",
			TargetAffCode:       "abcd",
			RequiresDescription: true,
			SortOrder:           2,
		},
	})
	require.NoError(t, err)

	common.OptionMapRWMutex.Lock()
	previousOptionMap := common.OptionMap
	common.OptionMap = map[string]string{
		source_attribution_setting.EnabledKey:                    "true",
		source_attribution_setting.TokenAPIEnforcementEnabledKey: "false",
		source_attribution_setting.OptionsKey:                    string(optionsBytes),
	}
	common.OptionMapRWMutex.Unlock()

	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = previousOptionMap
		common.OptionMapRWMutex.Unlock()
	})
}

func seedSourceAttributionUsers(t *testing.T) {
	t.Helper()
	require.NoError(t, model.DB.Create(&model.User{
		Id:          1,
		Username:    "inviter",
		Password:    "password",
		DisplayName: "Inviter",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		AffCode:     "abcd",
	}).Error)
	require.NoError(t, model.DB.Create(&model.User{
		Id:          2,
		Username:    "alice",
		Password:    "password",
		DisplayName: "Alice",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		AffCode:     "efgh",
	}).Error)
}

func TestGetSourceAttributionStatusHidesTargetAffCode(t *testing.T) {
	setupSourceAttributionControllerTest(t)
	seedSourceAttributionUsers(t)

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/source-attribution/status", nil, 2)
	GetSourceAttributionStatus(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	require.NotContains(t, string(response.Data), "inviter_id")
	responseBody := string(response.Data)
	require.Contains(t, responseBody, `"enabled":true`)
	require.Contains(t, responseBody, `"confirmed":false`)
	require.Contains(t, responseBody, `"id":"xiaoc"`)
	require.NotContains(t, responseBody, "target_aff_code")
	require.NotContains(t, responseBody, "abcd")
}

func TestGetSourceAttributionStatusTreatsRootAsConfirmed(t *testing.T) {
	setupSourceAttributionControllerTest(t)
	seedSourceAttributionUsers(t)
	require.NoError(t, model.DB.Create(&model.User{
		Id:       3,
		Username: "root",
		Role:     common.RoleRootUser,
		Status:   common.UserStatusEnabled,
		AffCode:  "rootaff",
	}).Error)

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/source-attribution/status", nil, 3)
	ctx.Set("role", common.RoleRootUser)
	GetSourceAttributionStatus(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	responseBody := string(response.Data)
	require.Contains(t, responseBody, `"enabled":true`)
	require.Contains(t, responseBody, `"confirmed":true`)
	require.Contains(t, responseBody, `"bypassed":true`)
}

func TestConfirmSourceAttributionUsesConfiguredOptionAndAudits(t *testing.T) {
	setupSourceAttributionControllerTest(t)
	seedSourceAttributionUsers(t)

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/source-attribution/confirm", map[string]any{
		"option_id":          "other",
		"source_description": "offline invite",
	}, 2)
	ctx.Set("username", "alice")
	ctx.Request.RemoteAddr = "203.0.113.40:12345"
	ctx.Request.Header.Set("User-Agent", "source-test-agent")

	ConfirmSourceAttribution(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	require.NotContains(t, string(response.Data), "inviter_id")

	var user model.User
	require.NoError(t, model.DB.First(&user, 2).Error)
	require.Equal(t, 1, user.InviterId)

	var confirmation model.SourceAttributionConfirmation
	require.NoError(t, model.DB.First(&confirmation, "user_id = ?", 2).Error)
	require.Equal(t, "other", confirmation.OptionId)
	require.Equal(t, "abcd", confirmation.TargetAffCode)
	require.Equal(t, "offline invite", confirmation.SourceDescription)
	require.Equal(t, "203.0.113.40", confirmation.Ip)
	require.Equal(t, "source-test-agent", confirmation.UserAgent)

	var auditLog model.Log
	require.NoError(t, model.LOG_DB.First(&auditLog, "user_id = ? AND type = ?", 2, model.LogTypeManage).Error)
	require.Contains(t, auditLog.Other, "source_attribution.confirm")
	require.Contains(t, auditLog.Other, "target_aff_code")
	require.Contains(t, auditLog.Other, "offline invite")
}

func TestConfirmSourceAttributionRequiresOtherDescription(t *testing.T) {
	setupSourceAttributionControllerTest(t)
	seedSourceAttributionUsers(t)

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/source-attribution/confirm", map[string]any{
		"option_id": "other",
	}, 2)
	ctx.Set("username", "alice")

	ConfirmSourceAttribution(ctx)

	response := decodeAPIResponse(t, recorder)
	require.False(t, response.Success)

	var user model.User
	require.NoError(t, model.DB.First(&user, 2).Error)
	require.Equal(t, 0, user.InviterId)
}

func TestConfirmSourceAttributionAcceptsMaximumUnicodeDescription(t *testing.T) {
	setupSourceAttributionControllerTest(t)
	seedSourceAttributionUsers(t)

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/source-attribution/confirm", map[string]any{
		"option_id":          "other",
		"source_description": strings.Repeat("中", source_attribution_setting.MaxSourceDescriptionLength),
	}, 2)
	ctx.Set("username", "alice")

	ConfirmSourceAttribution(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
}
