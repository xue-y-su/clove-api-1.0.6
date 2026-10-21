package controller

import (
	"errors"
	"net/netip"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

var errSensitiveIPFilterInvalid = errors.New("sensitive_ip_filter_invalid")

func scopedSensitiveIPFilter(c *gin.Context) (string, error) {
	raw := strings.TrimSpace(c.Query("ip"))
	if raw == "" || c.GetInt("role") >= common.RoleRootUser {
		return raw, nil
	}
	if strings.ContainsAny(raw, "%*") {
		return "", errSensitiveIPFilterInvalid
	}
	normalized := model.NormalizeAuditIP(raw)
	if _, err := netip.ParseAddr(normalized); err != nil {
		return "", errSensitiveIPFilterInvalid
	}
	return strings.ReplaceAll(model.MaskAuditIP(normalized), "*", "%"), nil
}

func projectSensitiveIPList(c *gin.Context, pageInfo *common.PageInfo, target string, eventType string, itemCount int, mask func()) error {
	if c.GetInt("role") < common.RoleRootUser {
		mask()
		return nil
	}
	return enqueueCriticalAudit(c, c.GetInt("id"), target, eventType, "success", map[string]interface{}{
		"page":           pageInfo.GetPage(),
		"page_size":      pageInfo.GetPageSize(),
		"total":          pageInfo.Total,
		"item_count":     itemCount,
		"full_ip_access": true,
	})
}
