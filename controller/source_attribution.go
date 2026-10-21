package controller

import (
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/source_attribution_setting"
	"github.com/gin-gonic/gin"
)

type sourceAttributionStatusResponse struct {
	Enabled   bool                                      `json:"enabled"`
	Confirmed bool                                      `json:"confirmed"`
	Bypassed  bool                                      `json:"bypassed,omitempty"`
	Options   []source_attribution_setting.PublicOption `json:"options"`
}

type sourceAttributionConfirmRequest struct {
	OptionId          string `json:"option_id"`
	SourceDescription string `json:"source_description"`
}

func GetSourceAttributionStatus(c *gin.Context) {
	settings := source_attribution_setting.GetSettings()
	confirmed := true
	options := make([]source_attribution_setting.PublicOption, 0)
	if settings.Enabled {
		if err := source_attribution_setting.ValidateSettings(settings); err != nil {
			common.ApiError(c, err)
			return
		}
		if c.GetInt("role") == common.RoleRootUser {
			confirmed = true
		} else {
			var err error
			confirmed, err = model.HasSourceAttribution(c.GetInt("id"))
			if err != nil {
				common.ApiError(c, err)
				return
			}
			if !confirmed {
				confirmed = model.IsRootUser(c.GetInt("id"))
			}
		}
		options = source_attribution_setting.PublicOptions(settings.Options)
	}
	common.ApiSuccess(c, sourceAttributionStatusResponse{
		Enabled:   settings.Enabled,
		Confirmed: confirmed,
		Bypassed:  settings.Enabled && confirmed && c.GetInt("role") == common.RoleRootUser,
		Options:   options,
	})
}

func ConfirmSourceAttribution(c *gin.Context) {
	settings := source_attribution_setting.GetSettings()
	if !settings.Enabled {
		common.ApiSuccess(c, gin.H{"confirmed": true})
		return
	}
	if err := source_attribution_setting.ValidateSettings(settings); err != nil {
		common.ApiError(c, err)
		return
	}

	var request sourceAttributionConfirmRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiError(c, errors.New("无效的来源归属确认请求"))
		return
	}
	option, ok := source_attribution_setting.FindOption(settings, request.OptionId)
	if !ok {
		common.ApiError(c, errors.New("来源归属选项不存在"))
		return
	}
	description := strings.TrimSpace(request.SourceDescription)
	if option.RequiresDescription && description == "" {
		common.ApiError(c, errors.New("来源说明不能为空"))
		return
	}
	if utf8.RuneCountInString(description) > source_attribution_setting.MaxSourceDescriptionLength {
		common.ApiError(c, errors.New("来源说明过长"))
		return
	}

	userId := c.GetInt("id")
	result, err := model.ConfirmSourceAttribution(userId, c.GetString("username"), model.SourceAttributionSelection{
		OptionId:          option.Id,
		OptionName:        option.Name,
		TargetUserID:      option.TargetUserID,
		TargetAffCode:     option.TargetAffCode,
		SourceDescription: description,
		Ip:                c.ClientIP(),
		UserAgent:         c.Request.UserAgent(),
		ConfirmedAt:       common.GetTimestamp(),
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if result.Confirmation != nil {
		params := map[string]interface{}{
			"option_id":                   option.Id,
			"option_name":                 option.Name,
			"source_description_provided": description != "",
		}
		auditInfo := map[string]interface{}{
			"target_user_id":     result.Confirmation.TargetUserID,
			"target_username":    result.Confirmation.TargetUsername,
			"target_aff_code":    result.Confirmation.TargetAffCode,
			"source_description": description,
		}
		model.RecordOperationAuditLog(userId, auditContentEN("source_attribution.confirm", params), c.ClientIP(), "source_attribution.confirm", params, nil, auditInfo)
		markAuditLogged(c)
	}
	common.ApiSuccess(c, gin.H{
		"confirmed":         true,
		"already_confirmed": result.AlreadyConfirmed,
	})
}

func GetSourceAttributionConfirmations(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	ipFilter, err := scopedSensitiveIPFilter(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	filter := model.SourceAttributionConfirmationFilter{
		UserId:         parsePositiveIntQuery(c, "user_id"),
		Username:       c.Query("username"),
		OptionId:       c.Query("option_id"),
		TargetUserID:   parsePositiveIntQuery(c, "target_user_id"),
		TargetAffCode:  c.Query("target_aff_code"),
		Ip:             ipFilter,
		StartTimestamp: int64(parsePositiveIntQuery(c, "start_timestamp")),
		EndTimestamp:   int64(parsePositiveIntQuery(c, "end_timestamp")),
	}
	items, total, err := model.GetSourceAttributionConfirmations(filter, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	if err := projectSensitiveIPList(c, pageInfo, "source-attribution-confirmations", "source_attribution.full_ip_read", len(items), func() {
		for _, item := range items {
			item.Ip = model.MaskAuditIP(item.Ip)
		}
	}); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, pageInfo)
}

func GetSourceAttributionSettings(c *gin.Context) {
	common.ApiSuccess(c, source_attribution_setting.GetSettings())
}

func UpdateSourceAttributionSettings(c *gin.Context) {
	var request source_attribution_setting.Settings
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiErrorMsg(c, "无效的来源归属设置")
		return
	}
	request = source_attribution_setting.NormalizeSettings(request)
	if err := source_attribution_setting.ValidateSettings(request); err != nil {
		common.ApiError(c, err)
		return
	}
	if request.Enabled {
		for index := range request.Options {
			if request.Options[index].TargetUserID <= 0 {
				legacyTargetID, err := model.GetUserIdByAffCode(request.Options[index].TargetAffCode)
				if err != nil {
					common.ApiError(c, errors.New("来源归属目标用户不存在"))
					return
				}
				request.Options[index].TargetUserID = legacyTargetID
			}
			if _, err := model.GetUserById(request.Options[index].TargetUserID, false); err != nil {
				common.ApiError(c, errors.New("来源归属目标用户不存在"))
				return
			}
			request.Options[index].TargetAffCode = ""
		}
	}

	oldSettings := source_attribution_setting.GetSettings()
	optionValues, err := source_attribution_setting.SettingsToOptions(request)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.UpdateOptionsBulk(optionValues); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "source_attribution.settings_update", map[string]interface{}{
		"enabled_from":                       oldSettings.Enabled,
		"enabled_to":                         request.Enabled,
		"token_api_enforcement_enabled_from": oldSettings.TokenAPIEnforcementEnabled,
		"token_api_enforcement_enabled_to":   request.TokenAPIEnforcementEnabled,
		"options_from":                       len(oldSettings.Options),
		"options_to":                         len(request.Options),
	})
	common.ApiSuccess(c, request)
}
