package controller

import (
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/announcement_setting"
	"github.com/QuantumNous/new-api/setting/console_setting"
	"github.com/QuantumNous/new-api/setting/ip_security_setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/registration_setting"
	"github.com/QuantumNous/new-api/setting/user_governance_setting"
	"github.com/QuantumNous/new-api/setting/user_redemption_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupFeatureSettingControllerTest(t *testing.T) {
	t.Helper()
	db := openTokenControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(
		&model.Option{}, &model.User{}, &model.Log{},
		&model.IPSecurityEvent{}, &model.IPAuditRollup{}, &model.IPSecurityProjectionState{},
	))

	previousTokenSettings := *operation_setting.GetTokenSetting()
	previousInvitationSettings := *operation_setting.GetInvitationIncentiveSetting()
	previousConsoleSettings := *console_setting.GetConsoleSetting()
	common.OptionMapRWMutex.Lock()
	previousOptions := common.OptionMap
	common.OptionMap = map[string]string{
		registration_setting.AffRequiredKey:                   "false",
		registration_setting.InviteLimitUnlimitedKey:          "true",
		registration_setting.InviteLimitCountKey:              "0",
		registration_setting.ApplicationsEnabledKey:           "true",
		registration_setting.ReapplyCooldownSecondsKey:        "86400",
		announcement_setting.PopupEnabledKey:                  "true",
		announcement_setting.ImageMaxBytesKey:                 "5242880",
		ip_security_setting.EnabledKey:                        "true",
		ip_security_setting.RetentionDaysKey:                  "90",
		ip_security_setting.ProjectionIntervalSecondsKey:      "60",
		ip_security_setting.ProjectionBatchSizeKey:            "500",
		user_governance_setting.AvatarUploadEnabledKey:        "true",
		user_governance_setting.AvatarUploadEndpointKey:       "https://img.scdn.io/api/v1.php",
		user_governance_setting.AvatarAllowedDomainsKey:       "img.scdn.io,img.cdn1.vip",
		user_governance_setting.DisplayNameMaxLengthKey:       "40",
		user_governance_setting.DisplayNameCooldownSecondsKey: "604800",
		user_redemption_setting.EnabledKey:                    "true",
		user_redemption_setting.SingleEnabledKey:              "true",
		user_redemption_setting.RandomEnabledKey:              "true",
		user_redemption_setting.MaxActiveCodesPerUserKey:      "0",
		user_redemption_setting.MaxClaimsPerCodeKey:           "100",
		user_redemption_setting.MaxExpiryHoursKey:             "720",
	}
	common.OptionMapRWMutex.Unlock()
	operation_setting.GetTokenSetting().AllowCustomKeys = true
	console_setting.GetConsoleSetting().AnnouncementsEnabled = true

	t.Cleanup(func() {
		*operation_setting.GetTokenSetting() = previousTokenSettings
		*operation_setting.GetInvitationIncentiveSetting() = previousInvitationSettings
		*console_setting.GetConsoleSetting() = previousConsoleSettings
		common.OptionMapRWMutex.Lock()
		common.OptionMap = previousOptions
		common.OptionMapRWMutex.Unlock()
	})
}

func validSecuritySettingsRequest() map[string]any {
	return map[string]any{
		"api_key": map[string]any{
			"allow_custom_keys": false,
		},
		"user_governance": map[string]any{
			"avatar_upload_enabled":         true,
			"avatar_upload_endpoint":        "https://upload.example.test/image",
			"avatar_allowed_domains":        []string{"img.example.test"},
			"display_name_max_length":       36,
			"display_name_cooldown_seconds": 3600,
		},
		"ip_security": map[string]any{
			"enabled":                            true,
			"retention_days":                     30,
			"projection_interval_seconds":        15,
			"projection_batch_size":              250,
			"shared_registration_user_threshold": 3,
			"shared_api_user_threshold":          5,
			"unauthorized_threshold":             10,
		},
		"user_redemption": map[string]any{
			"enabled":                   false,
			"single_enabled":            true,
			"random_enabled":            false,
			"subscription_enabled":      false,
			"max_active_codes_per_user": 3,
			"max_claims_per_code":       12,
			"max_expiry_hours":          48,
		},
	}
}

func TestUpdateSecuritySettingsRejectsInvalidBatchWithoutPartialWrite(t *testing.T) {
	setupFeatureSettingControllerTest(t)
	request := validSecuritySettingsRequest()
	request["ip_security"].(map[string]any)["projection_batch_size"] = 5001
	ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/security/settings", request, 1)
	ctx.Set("username", "root")
	ctx.Set("role", common.RoleRootUser)

	UpdateSecuritySettings(ctx)

	response := decodeAPIResponse(t, recorder)
	require.False(t, response.Success)
	assert.True(t, operation_setting.GetTokenSetting().AllowCustomKeys)
	assert.Equal(t, 40, user_governance_setting.GetSettings().DisplayNameMaxLength)
	assert.Equal(t, 500, ip_security_setting.GetSettings().ProjectionBatchSize)
	var count int64
	require.NoError(t, model.DB.Model(&model.Option{}).Count(&count).Error)
	assert.Zero(t, count)
}

func TestUpdateSecuritySettingsPersistsWholeValidatedBatch(t *testing.T) {
	setupFeatureSettingControllerTest(t)
	ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/security/settings", validSecuritySettingsRequest(), 1)
	ctx.Set("username", "root")
	ctx.Set("role", common.RoleRootUser)

	UpdateSecuritySettings(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	assert.False(t, operation_setting.GetTokenSetting().AllowCustomKeys)
	assert.Equal(t, 36, user_governance_setting.GetSettings().DisplayNameMaxLength)
	assert.Equal(t, 250, ip_security_setting.GetSettings().ProjectionBatchSize)
	assert.Equal(t, 3, ip_security_setting.GetSettings().SharedRegistrationUserThreshold)
	assert.Equal(t, 5, ip_security_setting.GetSettings().SharedAPIUserThreshold)
	assert.Equal(t, 10, ip_security_setting.GetSettings().UnauthorizedThreshold)

	var count int64
	require.NoError(t, model.DB.Model(&model.Option{}).Where("key IN ?", []string{
		operation_setting.AllowCustomKeysOptionKey,
		user_governance_setting.DisplayNameMaxLengthKey,
		ip_security_setting.ProjectionBatchSizeKey,
		ip_security_setting.UnauthorizedThresholdKey,
	}).Count(&count).Error)
	assert.Equal(t, int64(4), count)
}

func TestUpdateSecuritySettingsPersistsUserRedemptionPolicy(t *testing.T) {
	setupFeatureSettingControllerTest(t)
	request := validSecuritySettingsRequest()
	ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/security/settings", request, 1)
	ctx.Set("username", "root")
	ctx.Set("role", common.RoleRootUser)

	UpdateSecuritySettings(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	settings := user_redemption_setting.GetSettings()
	assert.False(t, settings.Enabled)
	assert.True(t, settings.SingleEnabled)
	assert.False(t, settings.RandomEnabled)
	assert.False(t, settings.SubscriptionEnabled)
	assert.Equal(t, 3, settings.MaxActiveCodesPerUser)
	assert.Equal(t, 12, settings.MaxClaimsPerCode)
	assert.Equal(t, int64(48), settings.MaxExpiryHours)
}

func TestUpdateSecuritySettingsPreservesUserRedemptionPolicyWhenOmitted(t *testing.T) {
	setupFeatureSettingControllerTest(t)
	common.OptionMapRWMutex.Lock()
	common.OptionMap[user_redemption_setting.EnabledKey] = "false"
	common.OptionMap[user_redemption_setting.RandomEnabledKey] = "false"
	common.OptionMap[user_redemption_setting.MaxActiveCodesPerUserKey] = "7"
	common.OptionMapRWMutex.Unlock()
	request := validSecuritySettingsRequest()
	delete(request, "user_redemption")
	ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/security/settings", request, 1)
	ctx.Set("username", "root")
	ctx.Set("role", common.RoleRootUser)

	UpdateSecuritySettings(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	settings := user_redemption_setting.GetSettings()
	assert.False(t, settings.Enabled)
	assert.False(t, settings.RandomEnabled)
	assert.Equal(t, 7, settings.MaxActiveCodesPerUser)
}

func TestUpdateAnnouncementSettingsValidatesBeforePersisting(t *testing.T) {
	setupFeatureSettingControllerTest(t)
	ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/announcement/settings", map[string]any{
		"enabled":         false,
		"popup_enabled":   false,
		"image_max_bytes": 5*announcement_setting.MiB + 1,
	}, 1)
	ctx.Set("username", "root")
	ctx.Set("role", common.RoleRootUser)

	UpdateAnnouncementSettings(ctx)

	response := decodeAPIResponse(t, recorder)
	require.False(t, response.Success)
	assert.True(t, announcement_setting.GetSettings().Enabled)
}

func TestUpdateAnnouncementSettingsPersistsValidatedBatch(t *testing.T) {
	setupFeatureSettingControllerTest(t)
	ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/announcement/settings", map[string]any{
		"enabled":         false,
		"popup_enabled":   false,
		"image_max_bytes": 2 * announcement_setting.MiB,
	}, 1)
	ctx.Set("username", "root")
	ctx.Set("role", common.RoleRootUser)

	UpdateAnnouncementSettings(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	settings := announcement_setting.GetSettings()
	assert.False(t, settings.Enabled)
	assert.Equal(t, int64(2*announcement_setting.MiB), settings.ImageMaxBytes)
	assert.NotContains(t, string(response.Data), "popup_enabled")
	common.OptionMapRWMutex.RLock()
	assert.Equal(t, "true", common.OptionMap[announcement_setting.PopupEnabledKey])
	common.OptionMapRWMutex.RUnlock()
}

func TestUpdateRegistrationSettingsPersistsInvitationPolicy(t *testing.T) {
	setupFeatureSettingControllerTest(t)
	ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/registration/settings", map[string]any{
		"aff_required":                        true,
		"invite_limit_unlimited":              true,
		"invite_limit_count":                  0,
		"invitation_applications_enabled":     false,
		"invitation_reapply_cooldown_seconds": 1800,
	}, 1)
	ctx.Set("username", "root")
	ctx.Set("role", common.RoleRootUser)

	UpdateRegistrationSettings(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	settings := registration_setting.GetSettings()
	assert.False(t, settings.InvitationApplicationsEnabled)
	assert.Equal(t, int64(1800), settings.InvitationReapplyCooldownSeconds)
}

func TestInvitationPosterSettingsAPIPersistsReturnsAndAuditsCopy(t *testing.T) {
	setupFeatureSettingControllerTest(t)
	common.OptionMapRWMutex.Lock()
	common.OptionMap[registration_setting.InvitationPosterTitleKey] = "Old title"
	common.OptionMap[registration_setting.InvitationPosterSubtitleKey] = "Old subtitle"
	common.OptionMapRWMutex.Unlock()

	ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/registration/settings", map[string]any{
		"aff_required":                        true,
		"invite_limit_unlimited":              true,
		"invite_limit_count":                  0,
		"invitation_applications_enabled":     true,
		"invitation_reapply_cooldown_seconds": 3600,
		"invitation_poster_title":             "  Join {站点名称}  ",
		"invitation_poster_subtitle":          "  Scan now  ",
	}, 1)
	ctx.Set("username", "root")
	ctx.Set("role", common.RoleRootUser)

	UpdateRegistrationSettings(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	settings := registration_setting.GetSettings()
	assert.Equal(t, "Join {站点名称}", settings.InvitationPosterTitle)
	assert.Equal(t, "Scan now", settings.InvitationPosterSubtitle)
	require.Contains(t, string(response.Data), `"invitation_poster_title":"Join {站点名称}"`)
	require.Contains(t, string(response.Data), `"invitation_poster_subtitle":"Scan now"`)

	getCtx, getRecorder := newAuthenticatedContext(t, http.MethodGet, "/api/registration/settings", nil, 1)
	GetRegistrationSettings(getCtx)
	getResponse := decodeAPIResponse(t, getRecorder)
	require.True(t, getResponse.Success, getResponse.Message)
	require.Contains(t, string(getResponse.Data), `"invitation_poster_title":"Join {站点名称}"`)

	var auditLog model.Log
	require.NoError(t, model.LOG_DB.First(&auditLog, "type = ?", model.LogTypeManage).Error)
	assert.Contains(t, auditLog.Other, `"invitation_poster_title_from":"Old title"`)
	assert.Contains(t, auditLog.Other, `"invitation_poster_title_to":"Join {站点名称}"`)
	assert.Contains(t, auditLog.Other, `"invitation_poster_subtitle_from":"Old subtitle"`)
	assert.Contains(t, auditLog.Other, `"invitation_poster_subtitle_to":"Scan now"`)
}

func TestUpdateInvitationIncentiveSettingsNormalizesPersistsAndAudits(t *testing.T) {
	setupFeatureSettingControllerTest(t)
	require.NoError(t, model.DB.Create(&model.User{
		Id: 1, Username: "root", Password: "password", Role: common.RoleRootUser,
		Status: common.UserStatusEnabled,
	}).Error)
	ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/invitation/incentive-settings", map[string]any{
		"enabled":                                          false,
		"inviter_registration_reward_enabled":              false,
		"invitee_registration_gift_enabled":                true,
		"cash_commission_enabled":                          true,
		"cash_commission_rate":                             "200",
		"admin_balance_redemption_commission_enabled":      true,
		"admin_balance_redemption_commission_rate":         "5.25",
		"admin_subscription_redemption_commission_enabled": true,
		"admin_subscription_redemption_commission_rate":    "12.5",
	}, 1)
	ctx.Set("username", "root")
	ctx.Set("role", common.RoleRootUser)

	UpdateInvitationIncentiveSettings(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	var settings operation_setting.InvitationIncentiveSetting
	require.NoError(t, common.Unmarshal(response.Data, &settings))
	assert.False(t, settings.Enabled)
	assert.False(t, settings.InviterRegistrationRewardEnabled)
	assert.True(t, settings.InviteeRegistrationGiftEnabled)
	assert.Equal(t, "200.00", settings.CashCommissionRate)
	assert.Equal(t, "5.25", settings.AdminBalanceRedemptionCommissionRate)
	assert.Equal(t, "12.50", settings.AdminSubscriptionRedemptionCommissionRate)
	assert.Equal(t, settings, *operation_setting.GetInvitationIncentiveSetting())

	var count int64
	require.NoError(t, model.DB.Model(&model.Option{}).
		Where("key LIKE ?", operation_setting.InvitationIncentiveSettingConfigName+".%").
		Count(&count).Error)
	assert.Equal(t, int64(9), count)
	var auditLog model.Log
	require.NoError(t, model.LOG_DB.First(&auditLog, "type = ?", model.LogTypeManage).Error)
	assert.Contains(t, auditLog.Other, "invitation.incentive_settings_update")
}

func TestGetInvitationIncentiveSettingsReturnsCurrentTypedPolicy(t *testing.T) {
	setupFeatureSettingControllerTest(t)
	current := operation_setting.GetInvitationIncentiveSetting()
	current.CashCommissionEnabled = true
	current.CashCommissionRate = "200.00"
	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/invitation/incentive-settings", nil, 1)

	GetInvitationIncentiveSettings(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	var settings operation_setting.InvitationIncentiveSetting
	require.NoError(t, common.Unmarshal(response.Data, &settings))
	assert.True(t, settings.CashCommissionEnabled)
	assert.Equal(t, "200.00", settings.CashCommissionRate)
}

func TestUpdateOptionRejectsDedicatedFeatureSettings(t *testing.T) {
	setupFeatureSettingControllerTest(t)
	keys := []string{
		announcement_setting.AnnouncementsEnabledKey,
		announcement_setting.PopupEnabledKey,
		ip_security_setting.EnabledKey,
		user_governance_setting.DisplayNameMaxLengthKey,
		operation_setting.AllowCustomKeysOptionKey,
		operation_setting.InvitationIncentiveSettingConfigName + ".enabled",
	}
	for _, key := range keys {
		ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/option/", map[string]any{
			"key": key, "value": "false",
		}, 1)
		ctx.Set("username", "root")
		ctx.Set("role", common.RoleRootUser)
		UpdateOption(ctx)
		response := decodeAPIResponse(t, recorder)
		require.False(t, response.Success, key)
		require.Contains(t, response.Message, "专用接口", key)
	}
}
