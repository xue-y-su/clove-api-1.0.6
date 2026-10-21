package controller

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/gin-gonic/gin"
)

func GetAllLogs(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	logType, _ := strconv.Atoi(c.Query("type"))
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	username := c.Query("username")
	tokenName := c.Query("token_name")
	modelName := c.Query("model_name")
	channel, _ := strconv.Atoi(c.Query("channel"))
	group := c.Query("group")
	requestId := c.Query("request_id")
	upstreamRequestId := c.Query("upstream_request_id")
	ipFilter, err := scopedSensitiveIPFilter(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	logs, total, err := model.GetAllLogsWithIP(logType, startTimestamp, endTimestamp, modelName, username, tokenName, ipFilter, pageInfo.GetStartIdx(), pageInfo.GetPageSize(), channel, group, requestId, upstreamRequestId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	normalizeVisibleLogVersions(logs)
	enrichLegacyLogBillingModes(logs)
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(logs)
	if err := projectSensitiveIPList(c, pageInfo, "logs", "log.full_ip_read", len(logs), func() {
		maskNonRootLogIPs(logs)
	}); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, pageInfo)
	return
}

func maskNonRootLogIPs(logs []*model.Log) {
	for _, log := range logs {
		if log == nil {
			continue
		}
		log.Ip = model.MaskAuditIP(log.Ip)

		other, err := common.StrToMap(log.Other)
		if err != nil {
			continue
		}
		adminInfo, ok := other["admin_info"].(map[string]interface{})
		if !ok {
			continue
		}
		callerIP, ok := adminInfo["caller_ip"].(string)
		if !ok {
			continue
		}
		adminInfo["caller_ip"] = model.MaskAuditIP(callerIP)
		log.Other = common.MapToJsonStr(other)
	}
}

func GetUserLogs(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	userId := c.GetInt("id")
	logType, _ := strconv.Atoi(c.Query("type"))
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	tokenName := c.Query("token_name")
	modelName := c.Query("model_name")
	group := c.Query("group")
	requestId := c.Query("request_id")
	upstreamRequestId := c.Query("upstream_request_id")
	logs, total, err := model.GetUserLogs(userId, logType, startTimestamp, endTimestamp, modelName, tokenName, pageInfo.GetStartIdx(), pageInfo.GetPageSize(), group, requestId, upstreamRequestId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	enrichLegacyLogBillingModes(logs)
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(logs)
	common.ApiSuccess(c, pageInfo)
	return
}

// Deprecated: SearchAllLogs 已废弃，前端未使用该接口。
func SearchAllLogs(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": false,
		"message": "该接口已废弃",
	})
}

// Deprecated: SearchUserLogs 已废弃，前端未使用该接口。
func SearchUserLogs(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": false,
		"message": "该接口已废弃",
	})
}

func GetLogByKey(c *gin.Context) {
	tokenId := c.GetInt("token_id")
	if tokenId == 0 {
		c.JSON(200, gin.H{
			"success": false,
			"message": "无效的令牌",
		})
		return
	}
	logs, err := model.GetLogByTokenId(tokenId)
	if err != nil {
		c.JSON(200, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	normalizeVisibleLogVersions(logs)
	c.JSON(200, gin.H{
		"success": true,
		"message": "",
		"data":    logs,
	})
}

func normalizeVisibleLogVersions(logs []*model.Log) {
	const legacyDisplayVersion = "v1.0.0-rc.21.clove-enhanced"
	for _, log := range logs {
		if log == nil || strings.TrimSpace(log.Other) == "" {
			continue
		}
		other, err := common.StrToMap(log.Other)
		if err != nil {
			continue
		}
		adminInfo, ok := other["admin_info"].(map[string]interface{})
		if !ok || adminInfo["version"] != legacyDisplayVersion {
			continue
		}
		adminInfo["version"] = common.DisplayVersion
		log.Other = common.MapToJsonStr(other)
	}
}

func enrichLegacyLogBillingModes(logs []*model.Log) {
	for _, log := range logs {
		if log == nil || log.Type != model.LogTypeConsume || strings.TrimSpace(log.Other) == "" {
			continue
		}
		other, err := common.StrToMap(log.Other)
		if err != nil {
			continue
		}
		if _, exists := other["billing_mode"]; exists {
			continue
		}
		if _, usesPrice := ratio_setting.GetModelPrice(log.ModelName, false); usesPrice {
			other["billing_mode"] = "per_call"
		} else {
			other["billing_mode"] = "per_token"
		}
		log.Other = common.MapToJsonStr(other)
	}
}

func GetLogsStat(c *gin.Context) {
	logType, _ := strconv.Atoi(c.Query("type"))
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	tokenName := c.Query("token_name")
	username := c.Query("username")
	modelName := c.Query("model_name")
	channel, _ := strconv.Atoi(c.Query("channel"))
	group := c.Query("group")
	ipFilter, err := scopedSensitiveIPFilter(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	stat, err := model.SumUsedQuotaWithIP(logType, startTimestamp, endTimestamp, modelName, username, tokenName, ipFilter, channel, group)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if c.GetInt("role") >= common.RoleRootUser && strings.TrimSpace(c.Query("ip")) != "" {
		if err := enqueueCriticalAudit(c, c.GetInt("id"), "log-stat", "log.full_ip_stat_read", "success", map[string]interface{}{
			"full_ip_access": true,
		}); err != nil {
			common.ApiError(c, err)
			return
		}
	}
	//tokenNum := model.SumUsedToken(logType, startTimestamp, endTimestamp, modelName, username, "")
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"quota": stat.Quota,
			"rpm":   stat.Rpm,
			"tpm":   stat.Tpm,
		},
	})
	return
}

func GetIPAudit(c *gin.Context) {
	query, err := parseIPAuditQuery(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	page, err := service.GetGlobalIPAudit(c.Request.Context(), c.GetInt("id"), query)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, page)
}

func GetIPAuditDetail(c *gin.Context) {
	query, err := parseIPAuditQuery(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	detail, err := service.GetGlobalIPAuditDetail(c.Request.Context(), c.GetInt("id"), c.Query("ip"), query)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, detail)
}

func GetLogsSelfStat(c *gin.Context) {
	username := c.GetString("username")
	logType, _ := strconv.Atoi(c.Query("type"))
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	tokenName := c.Query("token_name")
	modelName := c.Query("model_name")
	channel, _ := strconv.Atoi(c.Query("channel"))
	group := c.Query("group")
	quotaNum, err := model.SumUsedQuota(logType, startTimestamp, endTimestamp, modelName, username, tokenName, channel, group)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	//tokenNum := model.SumUsedToken(logType, startTimestamp, endTimestamp, modelName, username, tokenName)
	c.JSON(200, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"quota": quotaNum.Quota,
			"rpm":   quotaNum.Rpm,
			"tpm":   quotaNum.Tpm,
			//"token": tokenNum,
		},
	})
	return
}

// DeleteHistoryLogs is the legacy synchronous log cleanup endpoint (DELETE /api/log/).
// It deletes directly instead of going through the async system task. It is kept only
// for the classic frontend; the default frontend uses POST /api/system-task/log-cleanup.
// TODO: remove this handler (and its route) once the classic frontend is removed.
func DeleteHistoryLogs(c *gin.Context) {
	targetTimestamp, _ := strconv.ParseInt(c.Query("target_timestamp"), 10, 64)
	if targetTimestamp == 0 {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "target timestamp is required",
		})
		return
	}
	count, err := model.DeleteOldLog(c.Request.Context(), targetTimestamp, 100)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    count,
	})
	return
}
