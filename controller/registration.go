package controller

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/registration_setting"
	"github.com/gin-gonic/gin"
)

func GetRegistrationSettings(c *gin.Context) {
	common.ApiSuccess(c, registration_setting.GetSettings())
}

func UpdateRegistrationSettings(c *gin.Context) {
	var request registration_setting.Settings
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiErrorMsg(c, "无效的邀请注册设置")
		return
	}
	request = registration_setting.NormalizeSettings(request)
	if err := registration_setting.ValidateSettings(request); err != nil {
		common.ApiError(c, err)
		return
	}

	oldSettings := registration_setting.GetSettings()
	if err := model.UpdateOptionsBulk(registration_setting.SettingsToOptions(request)); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "registration.settings_update", map[string]interface{}{
		"aff_required_from":                        oldSettings.AffRequired,
		"aff_required_to":                          request.AffRequired,
		"invitation_applications_enabled_from":     oldSettings.InvitationApplicationsEnabled,
		"invitation_applications_enabled_to":       request.InvitationApplicationsEnabled,
		"invitation_reapply_cooldown_seconds_from": oldSettings.InvitationReapplyCooldownSeconds,
		"invitation_reapply_cooldown_seconds_to":   request.InvitationReapplyCooldownSeconds,
		"invitation_poster_title_from":             oldSettings.InvitationPosterTitle,
		"invitation_poster_title_to":               request.InvitationPosterTitle,
		"invitation_poster_subtitle_from":          oldSettings.InvitationPosterSubtitle,
		"invitation_poster_subtitle_to":            request.InvitationPosterSubtitle,
	})
	common.ApiSuccess(c, request)
}
