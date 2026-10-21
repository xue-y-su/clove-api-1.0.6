package controller

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/ip_security_setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/user_governance_setting"
	"github.com/QuantumNous/new-api/setting/user_redemption_setting"
	"github.com/gin-gonic/gin"
)

type apiKeySecuritySettings struct {
	AllowCustomKeys bool `json:"allow_custom_keys"`
}

type securitySettings struct {
	APIKey         apiKeySecuritySettings           `json:"api_key"`
	UserGovernance user_governance_setting.Settings `json:"user_governance"`
	IPSecurity     ip_security_setting.Settings     `json:"ip_security"`
	UserRedemption user_redemption_setting.Settings `json:"user_redemption"`
}

type securitySettingsRequest struct {
	APIKey         apiKeySecuritySettings            `json:"api_key"`
	UserGovernance user_governance_setting.Settings  `json:"user_governance"`
	IPSecurity     ip_security_setting.Settings      `json:"ip_security"`
	UserRedemption *user_redemption_setting.Settings `json:"user_redemption"`
}

func GetSecuritySettings(c *gin.Context) {
	common.ApiSuccess(c, currentSecuritySettings())
}

func UpdateSecuritySettings(c *gin.Context) {
	var request securitySettingsRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiErrorMsg(c, "无效的安全设置")
		return
	}
	previous := currentSecuritySettings()
	request.UserGovernance = user_governance_setting.NormalizeSettings(request.UserGovernance)
	userRedemption := previous.UserRedemption
	if request.UserRedemption != nil {
		userRedemption = *request.UserRedemption
	}
	if err := user_redemption_setting.ValidateSettings(userRedemption); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := user_governance_setting.ValidateSettings(request.UserGovernance); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := ip_security_setting.ValidateSettings(request.IPSecurity); err != nil {
		common.ApiError(c, err)
		return
	}

	values := user_governance_setting.SettingsToOptions(request.UserGovernance)
	for key, value := range ip_security_setting.SettingsToOptions(request.IPSecurity) {
		values[key] = value
	}
	for key, value := range user_redemption_setting.SettingsToOptions(userRedemption) {
		values[key] = value
	}
	values[operation_setting.AllowCustomKeysOptionKey] = common.Interface2String(request.APIKey.AllowCustomKeys)
	if err := model.UpdateOptionsBulk(values); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "security.settings_update", map[string]interface{}{
		"allow_custom_keys_from":             previous.APIKey.AllowCustomKeys,
		"allow_custom_keys_to":               request.APIKey.AllowCustomKeys,
		"avatar_upload_enabled_from":         previous.UserGovernance.AvatarUploadEnabled,
		"avatar_upload_enabled_to":           request.UserGovernance.AvatarUploadEnabled,
		"display_name_max_length_from":       previous.UserGovernance.DisplayNameMaxLength,
		"display_name_max_length_to":         request.UserGovernance.DisplayNameMaxLength,
		"display_name_cooldown_seconds_from": previous.UserGovernance.DisplayNameCooldownSeconds,
		"display_name_cooldown_seconds_to":   request.UserGovernance.DisplayNameCooldownSeconds,
		"ip_security_enabled_from":           previous.IPSecurity.Enabled,
		"ip_security_enabled_to":             request.IPSecurity.Enabled,
		"ip_security_retention_days_from":    previous.IPSecurity.RetentionDays,
		"ip_security_retention_days_to":      request.IPSecurity.RetentionDays,
		"ip_security_interval_seconds_from":  previous.IPSecurity.ProjectionIntervalSeconds,
		"ip_security_interval_seconds_to":    request.IPSecurity.ProjectionIntervalSeconds,
		"ip_security_batch_size_from":        previous.IPSecurity.ProjectionBatchSize,
		"ip_security_batch_size_to":          request.IPSecurity.ProjectionBatchSize,
		"user_redemption_enabled_from":       previous.UserRedemption.Enabled,
		"user_redemption_enabled_to":         userRedemption.Enabled,
		"user_redemption_single_from":        previous.UserRedemption.SingleEnabled,
		"user_redemption_single_to":          userRedemption.SingleEnabled,
		"user_redemption_random_from":        previous.UserRedemption.RandomEnabled,
		"user_redemption_random_to":          userRedemption.RandomEnabled,
		"user_redemption_active_limit_from":  previous.UserRedemption.MaxActiveCodesPerUser,
		"user_redemption_active_limit_to":    userRedemption.MaxActiveCodesPerUser,
		"user_redemption_claim_limit_from":   previous.UserRedemption.MaxClaimsPerCode,
		"user_redemption_claim_limit_to":     userRedemption.MaxClaimsPerCode,
		"user_redemption_expiry_from":        previous.UserRedemption.MaxExpiryHours,
		"user_redemption_expiry_to":          userRedemption.MaxExpiryHours,
	})
	common.ApiSuccess(c, securitySettings{
		APIKey:         request.APIKey,
		UserGovernance: request.UserGovernance,
		IPSecurity:     request.IPSecurity,
		UserRedemption: userRedemption,
	})
}

func currentSecuritySettings() securitySettings {
	return securitySettings{
		APIKey: apiKeySecuritySettings{
			AllowCustomKeys: operation_setting.GetTokenSetting().AllowCustomKeys,
		},
		UserGovernance: user_governance_setting.GetSettings(),
		IPSecurity:     ip_security_setting.GetSettings(),
		UserRedemption: user_redemption_setting.GetSettings(),
	}
}
