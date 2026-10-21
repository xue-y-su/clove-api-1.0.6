package controller

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
)

func GetInvitationIncentiveSettings(c *gin.Context) {
	common.ApiSuccess(c, *operation_setting.GetInvitationIncentiveSetting())
}

func UpdateInvitationIncentiveSettings(c *gin.Context) {
	var request operation_setting.InvitationIncentiveSetting
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiErrorMsg(c, "无效的邀请激励设置")
		return
	}
	normalized, err := operation_setting.NormalizeInvitationIncentiveSetting(request)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	previous := *operation_setting.GetInvitationIncentiveSetting()
	if err := model.UpdateOptionsBulk(operation_setting.InvitationIncentiveSettingsToOptions(normalized)); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "invitation.incentive_settings_update", map[string]interface{}{
		"enabled_from":  previous.Enabled,
		"enabled_to":    normalized.Enabled,
		"settings_from": previous,
		"settings_to":   normalized,
	})
	common.ApiSuccess(c, normalized)
}
