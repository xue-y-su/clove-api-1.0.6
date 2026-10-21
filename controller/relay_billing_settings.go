package controller

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

func GetRelayBillingSettings(c *gin.Context) {
	common.ApiSuccess(c, service.CurrentRelayBillingSettings())
}

func UpdateRelayBillingSettings(c *gin.Context) {
	var request dto.RelayBillingSettings
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiErrorMsg(c, "无效的 Relay Billing 设置")
		return
	}
	settings, err := service.UpdateRelayBillingSettings(request)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "option.update.relay_billing", map[string]interface{}{
		"alpha_search_enabled": settings.AlphaSearch.Enabled,
		"tool_pricing_enabled": settings.ToolPricing.Enabled,
		"diagnostic_enabled":   settings.UpstreamDiagnostic.Enabled,
	})
	common.ApiSuccess(c, settings)
}
