package middleware

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/source_attribution_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

func EnsureDashboardSourceAttributionConfirmed(c *gin.Context) bool {
	if shouldSkipDashboardSourceAttribution(c) {
		return true
	}
	return EnsureAuthenticatedUserSourceAttributionConfirmed(c)
}

func EnsureAuthenticatedUserSourceAttributionConfirmed(c *gin.Context) bool {
	settings := source_attribution_setting.GetSettings()
	if !settings.Enabled {
		return true
	}
	if source_attribution_setting.ValidateSettings(settings) != nil {
		return true
	}
	confirmed, err := isSourceAttributionSatisfied(c)
	if err != nil {
		common.SysLog(fmt.Sprintf("检查来源归属失败：%v", err))
		confirmed = false
	}
	if confirmed {
		return true
	}
	c.JSON(http.StatusForbidden, gin.H{
		"success": false,
		"message": "请先确认来源归属",
		"code":    string(types.ErrorCodeSourceAttributionRequired),
	})
	c.Abort()
	return false
}

func EnsureTokenSourceAttributionConfirmed(c *gin.Context) bool {
	settings := source_attribution_setting.GetSettings()
	if !settings.Enabled || !settings.TokenAPIEnforcementEnabled {
		return true
	}
	if source_attribution_setting.ValidateSettings(settings) != nil {
		return true
	}
	confirmed, err := isSourceAttributionSatisfied(c)
	if err != nil {
		common.SysLog(fmt.Sprintf("检查 Token 来源归属失败：%v", err))
		confirmed = false
	}
	if confirmed {
		return true
	}
	abortWithOpenAiMessage(c, http.StatusForbidden, "请先确认来源归属", types.ErrorCodeSourceAttributionRequired)
	return false
}

func isSourceAttributionSatisfied(c *gin.Context) (bool, error) {
	if c.GetInt("role") == common.RoleRootUser {
		return true, nil
	}
	userId := c.GetInt("id")
	confirmed, err := model.HasSourceAttribution(userId)
	if confirmed || err != nil {
		return confirmed, err
	}
	return model.IsRootUser(userId), nil
}

func shouldSkipDashboardSourceAttribution(c *gin.Context) bool {
	if c == nil || c.Request == nil || c.Request.URL == nil {
		return true
	}
	path := c.Request.URL.Path
	if !strings.HasPrefix(path, "/api/") {
		return true
	}
	if strings.HasPrefix(path, "/api/compliance/") ||
		strings.HasPrefix(path, "/api/source-attribution/") ||
		strings.HasPrefix(path, "/api/registration/") {
		return true
	}
	switch {
	case path == "/api/user/self" && c.Request.Method == http.MethodGet:
		return true
	case path == "/api/user/logout":
		return true
	case strings.HasPrefix(path, "/api/user/2fa/"):
		return true
	case strings.HasPrefix(path, "/api/oauth/"):
		return true
	default:
		return false
	}
}
