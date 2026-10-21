package controller

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/announcement_setting"
	"github.com/QuantumNous/new-api/setting/console_setting"
	"github.com/QuantumNous/new-api/setting/model_catalog_setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/QuantumNous/new-api/setting/user_governance_setting"

	"github.com/gin-gonic/gin"
)

var completionRatioMetaOptionKeys = []string{
	"ModelPrice",
	"ModelRatio",
	"CompletionRatio",
	"CacheRatio",
	"CreateCacheRatio",
	"ImageRatio",
	"AudioRatio",
	"AudioCompletionRatio",
}

func isPaymentComplianceOptionKey(key string) bool {
	return strings.HasPrefix(key, "payment_setting.compliance_")
}

func isProtectedDedicatedOptionKey(key string) bool {
	return strings.HasPrefix(key, "compliance.") ||
		strings.HasPrefix(key, "registration.") ||
		strings.HasPrefix(key, "source_attribution.") ||
		strings.HasPrefix(key, operation_setting.InvitationIncentiveSettingConfigName+".") ||
		strings.HasPrefix(key, "announcement_setting.") ||
		key == announcement_setting.AnnouncementsEnabledKey ||
		strings.HasPrefix(key, "ip_security.") ||
		key == operation_setting.AllowCustomKeysOptionKey ||
		key == user_governance_setting.AvatarUploadEnabledKey ||
		key == user_governance_setting.AvatarUploadEndpointKey ||
		key == user_governance_setting.AvatarAllowedDomainsKey ||
		key == user_governance_setting.DisplayNameMaxLengthKey ||
		key == user_governance_setting.DisplayNameCooldownSecondsKey ||
		key == model_catalog_setting.OptionKey
}

func dedicatedOptionMessage(key string) string {
	switch {
	case strings.HasPrefix(key, "compliance."):
		return "请使用合规声明专用接口修改该配置"
	case strings.HasPrefix(key, "registration."):
		return "请使用注册设置专用接口修改该配置"
	case strings.HasPrefix(key, "source_attribution."):
		return "请使用来源归属专用接口修改该配置"
	case strings.HasPrefix(key, operation_setting.InvitationIncentiveSettingConfigName+"."):
		return "请使用邀请激励专用接口修改该配置"
	case strings.HasPrefix(key, "announcement_setting.") || key == announcement_setting.AnnouncementsEnabledKey:
		return "请使用公告设置专用接口修改该配置"
	case strings.HasPrefix(key, "ip_security."), key == operation_setting.AllowCustomKeysOptionKey,
		key == user_governance_setting.AvatarUploadEnabledKey,
		key == user_governance_setting.AvatarUploadEndpointKey,
		key == user_governance_setting.AvatarAllowedDomainsKey,
		key == user_governance_setting.DisplayNameMaxLengthKey,
		key == user_governance_setting.DisplayNameCooldownSecondsKey:
		return "请使用安全设置专用接口修改该配置"
	case key == model_catalog_setting.OptionKey:
		return "请使用模型广场分组可见性专用接口修改该配置"
	default:
		return "请使用专用接口修改该配置"
	}
}

func isPositiveOptionValue(value string) bool {
	intValue, err := strconv.Atoi(strings.TrimSpace(value))
	if err == nil {
		return intValue > 0
	}
	floatValue, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	return err == nil && floatValue > 0
}

func collectModelNamesFromOptionValue(raw string, modelNames map[string]struct{}) {
	if strings.TrimSpace(raw) == "" {
		return
	}

	var parsed map[string]any
	if err := common.UnmarshalJsonStr(raw, &parsed); err != nil {
		return
	}

	for modelName := range parsed {
		modelNames[modelName] = struct{}{}
	}
}

func buildCompletionRatioMetaValue(optionValues map[string]string) string {
	modelNames := make(map[string]struct{})
	for _, key := range completionRatioMetaOptionKeys {
		collectModelNamesFromOptionValue(optionValues[key], modelNames)
	}

	meta := make(map[string]ratio_setting.CompletionRatioInfo, len(modelNames))
	for modelName := range modelNames {
		meta[modelName] = ratio_setting.GetCompletionRatioInfo(modelName)
	}

	jsonBytes, err := common.Marshal(meta)
	if err != nil {
		return "{}"
	}
	return string(jsonBytes)
}

func GetOptions(c *gin.Context) {
	var options []*model.Option
	optionValues := make(map[string]string)
	common.OptionMapRWMutex.Lock()
	playgroundTavilyAPIKeyConfigured := strings.TrimSpace(common.OptionMap[service.PlaygroundSearchOptionAPIKey]) != ""
	for k, v := range common.OptionMap {
		value := common.Interface2String(v)
		isSensitiveKey := strings.HasSuffix(k, "Token") ||
			strings.HasSuffix(k, "Secret") ||
			strings.HasSuffix(k, "Key") ||
			strings.HasSuffix(k, "secret") ||
			strings.HasSuffix(k, "api_key")
		if isSensitiveKey || isProtectedDedicatedOptionKey(k) {
			continue
		}
		options = append(options, &model.Option{
			Key:   k,
			Value: value,
		})
		for _, optionKey := range completionRatioMetaOptionKeys {
			if optionKey == k {
				optionValues[k] = value
				break
			}
		}
	}
	common.OptionMapRWMutex.Unlock()
	options = append(options, &model.Option{
		Key:   "CompletionRatioMeta",
		Value: buildCompletionRatioMetaValue(optionValues),
	})
	options = append(options, &model.Option{
		Key:   "PlaygroundTavilyAPIKeyConfigured",
		Value: strconv.FormatBool(playgroundTavilyAPIKeyConfigured),
	})
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    options,
	})
}

type OptionUpdateRequest struct {
	Key   string `json:"key"`
	Value any    `json:"value"`
}

func UpdateOption(c *gin.Context) {
	var option OptionUpdateRequest
	err := common.DecodeJson(c.Request.Body, &option)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的参数",
		})
		return
	}
	if option.Value == nil {
		common.ApiErrorMsg(c, "配置值不能为空")
		return
	}
	switch option.Value.(type) {
	case bool:
		option.Value = common.Interface2String(option.Value.(bool))
	case float64:
		option.Value = common.Interface2String(option.Value.(float64))
	case int:
		option.Value = common.Interface2String(option.Value.(int))
	default:
		option.Value = fmt.Sprintf("%v", option.Value)
	}
	switch option.Key {
	case "QuotaForInviter", "QuotaForInvitee":
		if isPositiveOptionValue(option.Value.(string)) && !operation_setting.IsPaymentComplianceConfirmed() {
			common.ApiErrorI18n(c, i18n.MsgPaymentComplianceRequired)
			return
		}
	default:
		if isProtectedDedicatedOptionKey(option.Key) {
			common.ApiErrorMsg(c, dedicatedOptionMessage(option.Key))
			return
		}
		if isPaymentComplianceOptionKey(option.Key) {
			common.ApiErrorMsg(c, "合规确认字段不允许通过通用设置接口修改")
			return
		}
	}
	if validateErr := validateOperationSettingOption(option.Key, option.Value.(string)); validateErr != nil {
		common.ApiErrorMsg(c, validateErr.Error())
		return
	}
	switch option.Key {
	case service.PlaygroundSearchOptionEnabled:
		if option.Value != "true" && option.Value != "false" {
			common.ApiErrorMsg(c, "Playground Tavily enabled value must be true or false")
			return
		}
		if option.Value == "true" {
			common.OptionMapRWMutex.RLock()
			apiKeyConfigured := strings.TrimSpace(common.OptionMap[service.PlaygroundSearchOptionAPIKey]) != ""
			common.OptionMapRWMutex.RUnlock()
			if !apiKeyConfigured {
				common.ApiErrorMsg(c, "Playground Tavily API key must be configured before enabling search")
				return
			}
		}
	case service.PlaygroundSearchOptionAPIKey:
		if strings.TrimSpace(option.Value.(string)) == "" {
			common.OptionMapRWMutex.RLock()
			enabled := strings.EqualFold(strings.TrimSpace(common.OptionMap[service.PlaygroundSearchOptionEnabled]), "true")
			common.OptionMapRWMutex.RUnlock()
			if enabled {
				common.ApiErrorMsg(c, "Playground Tavily API key cannot be cleared while search is enabled")
				return
			}
		}
	case service.PlaygroundSearchOptionDefaultDepth:
		value := strings.ToLower(strings.TrimSpace(option.Value.(string)))
		if value != "basic" && value != "advanced" {
			common.ApiErrorMsg(c, "Playground Tavily search depth must be basic or advanced")
			return
		}
	case service.PlaygroundSearchOptionMaxResults:
		value, parseErr := strconv.Atoi(strings.TrimSpace(option.Value.(string)))
		if parseErr != nil || value < 1 || value > 10 {
			common.ApiErrorMsg(c, "Playground Tavily max results must be between 1 and 10")
			return
		}
	case service.PlaygroundSearchOptionMaxContextLength:
		value, parseErr := strconv.Atoi(strings.TrimSpace(option.Value.(string)))
		if parseErr != nil || value < 500 || value > 20000 {
			common.ApiErrorMsg(c, "Playground Tavily max context length must be between 500 and 20000")
			return
		}
	case service.PlaygroundSearchOptionNativeModels:
		if validateErr := service.ValidatePlaygroundModelListOption(option.Value.(string)); validateErr != nil {
			common.ApiErrorMsg(c, "Playground model policy is invalid: "+validateErr.Error())
			return
		}
	case service.PlaygroundSearchOptionNativeChannels:
		if validateErr := service.ValidatePlaygroundNativeSearchChannelsOption(option.Value.(string)); validateErr != nil {
			common.ApiErrorMsg(c, "Playground native search channels are invalid: "+validateErr.Error())
			return
		}
	case "GitHubOAuthEnabled":
		if option.Value == "true" && common.GitHubClientId == "" {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无法启用 GitHub OAuth，请先填入 GitHub Client Id 以及 GitHub Client Secret！",
			})
			return
		}
	case "discord.enabled":
		if option.Value == "true" && system_setting.GetDiscordSettings().ClientId == "" {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无法启用 Discord OAuth，请先填入 Discord Client Id 以及 Discord Client Secret！",
			})
			return
		}
	case "oidc.enabled":
		if option.Value == "true" && system_setting.GetOIDCSettings().ClientId == "" {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无法启用 OIDC 登录，请先填入 OIDC Client Id 以及 OIDC Client Secret！",
			})
			return
		}
	case "LinuxDOOAuthEnabled":
		if option.Value == "true" && common.LinuxDOClientId == "" {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无法启用 LinuxDO OAuth，请先填入 LinuxDO Client Id 以及 LinuxDO Client Secret！",
			})
			return
		}
	case "EmailDomainRestrictionEnabled":
		if option.Value == "true" && len(common.EmailDomainWhitelist) == 0 {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无法启用邮箱域名限制，请先填入限制的邮箱域名！",
			})
			return
		}
	case "WeChatAuthEnabled":
		if option.Value == "true" && common.WeChatServerAddress == "" {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无法启用微信登录，请先填入微信登录相关配置信息！",
			})
			return
		}
	case "TurnstileCheckEnabled":
		if option.Value == "true" && common.TurnstileSiteKey == "" {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无法启用 Turnstile 校验，请先填入 Turnstile 校验相关配置信息！",
			})

			return
		}
	case "TelegramOAuthEnabled":
		if option.Value == "true" && common.TelegramBotToken == "" {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无法启用 Telegram OAuth，请先填入 Telegram Bot Token！",
			})
			return
		}
	case "theme.frontend":
		if option.Value != "default" && option.Value != "classic" {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无效的主题值，可选值：default（新版前端）、classic（经典前端）",
			})
			return
		}
	case "HomePageMode":
		if option.Value != "default" && option.Value != "snow" {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "无效的首页模式，可选值：default、snow",
			})
			return
		}
	case "GroupRatio":
		err = ratio_setting.CheckGroupRatio(option.Value.(string))
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	case "ImageRatio":
		err = ratio_setting.UpdateImageRatioByJSONString(option.Value.(string))
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "图片倍率设置失败: " + err.Error(),
			})
			return
		}
	case "AudioRatio":
		err = ratio_setting.UpdateAudioRatioByJSONString(option.Value.(string))
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "音频倍率设置失败: " + err.Error(),
			})
			return
		}
	case "AudioCompletionRatio":
		err = ratio_setting.UpdateAudioCompletionRatioByJSONString(option.Value.(string))
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "音频补全倍率设置失败: " + err.Error(),
			})
			return
		}
	case "CreateCacheRatio":
		err = ratio_setting.UpdateCreateCacheRatioByJSONString(option.Value.(string))
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "缓存创建倍率设置失败: " + err.Error(),
			})
			return
		}
	case "ModelRequestRateLimitGroup":
		err = setting.CheckModelRequestRateLimitGroup(option.Value.(string))
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	case "AutomaticDisableStatusCodes":
		_, err = operation_setting.ParseHTTPStatusCodeRanges(option.Value.(string))
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	case "AutomaticRetryStatusCodes":
		_, err = operation_setting.ParseHTTPStatusCodeRanges(option.Value.(string))
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	case "console_setting.api_info":
		err = console_setting.ValidateConsoleSettings(option.Value.(string), "ApiInfo")
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	case "console_setting.announcements":
		err = console_setting.ValidateConsoleSettings(option.Value.(string), "Announcements")
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	case "console_setting.faq":
		err = console_setting.ValidateConsoleSettings(option.Value.(string), "FAQ")
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	case "console_setting.uptime_kuma_groups":
		err = console_setting.ValidateConsoleSettings(option.Value.(string), "UptimeKumaGroups")
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	}
	if option.Key == "console_setting.announcements" {
		err = service.SaveAnnouncements(option.Value.(string), time.Now())
	} else {
		err = model.UpdateOption(option.Key, option.Value.(string))
	}
	if err != nil {
		common.ApiError(c, err)
		return
	}
	// 出于安全考虑只记录被修改的配置项名称，不记录配置值（可能含密钥等敏感信息）。
	recordManageAudit(c, "option.update", map[string]interface{}{
		"key": option.Key,
	})
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

func validateOperationSettingOption(key, value string) error {
	if strings.HasPrefix(key, "tool_price_setting.") {
		return operation_setting.ValidateToolPriceOption(strings.TrimPrefix(key, "tool_price_setting."), value)
	}
	alphaSearchPrefix := operation_setting.AlphaSearchSettingConfigName + "."
	if strings.HasPrefix(key, alphaSearchPrefix) {
		return operation_setting.ValidateAlphaSearchOption(strings.TrimPrefix(key, alphaSearchPrefix), value)
	}
	diagnosticPrefix := operation_setting.UpstreamDiagnosticSettingConfigName + "."
	if strings.HasPrefix(key, diagnosticPrefix) {
		return operation_setting.ValidateUpstreamDiagnosticOption(strings.TrimPrefix(key, diagnosticPrefix), value)
	}
	return nil
}
