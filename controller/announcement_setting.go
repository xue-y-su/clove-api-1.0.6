package controller

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/announcement_setting"
	"github.com/gin-gonic/gin"
)

func GetAnnouncementSettings(c *gin.Context) {
	common.ApiSuccess(c, announcement_setting.GetSettings())
}

func UpdateAnnouncementSettings(c *gin.Context) {
	var request announcement_setting.Settings
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiErrorMsg(c, "无效的公告设置")
		return
	}
	if err := announcement_setting.ValidateSettings(request); err != nil {
		common.ApiError(c, err)
		return
	}

	previous := announcement_setting.GetSettings()
	if err := model.UpdateOptionsBulk(announcement_setting.SettingsToOptions(request)); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "announcement.settings_update", map[string]interface{}{
		"enabled_from":         previous.Enabled,
		"enabled_to":           request.Enabled,
		"image_max_bytes_from": previous.ImageMaxBytes,
		"image_max_bytes_to":   request.ImageMaxBytes,
	})
	common.ApiSuccess(c, request)
}
