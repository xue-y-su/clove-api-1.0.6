package controller

import (
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

func GetUserSecurityAudit(c *gin.Context) {
	targetID, err := strconv.Atoi(c.Param("id"))
	if err != nil || targetID <= 0 {
		common.ApiError(c, service.ErrIPAuditForbidden)
		return
	}
	query, err := parseIPAuditQuery(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	result, err := service.GetUserSecurityAudit(c.Request.Context(), c.GetInt("id"), targetID, query)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

func parseIPAuditQuery(c *gin.Context) (service.IPAuditQuery, error) {
	startTimestamp, err := parseOptionalIPAuditTimestamp(c, "start_timestamp")
	if err != nil {
		return service.IPAuditQuery{}, err
	}
	endTimestamp, err := parseOptionalIPAuditTimestamp(c, "end_timestamp")
	if err != nil {
		return service.IPAuditQuery{}, err
	}
	pageInfo := common.GetPageQuery(c)
	relatedPage, _ := strconv.Atoi(strings.TrimSpace(c.Query("related_page")))
	relatedPageSize, _ := strconv.Atoi(strings.TrimSpace(c.Query("related_page_size")))
	detailUserPage, _ := strconv.Atoi(strings.TrimSpace(c.Query("user_page")))
	detailUserPageSize, _ := strconv.Atoi(strings.TrimSpace(c.Query("user_page_size")))
	timelinePage, _ := strconv.Atoi(strings.TrimSpace(c.Query("timeline_page")))
	timelinePageSize, _ := strconv.Atoi(strings.TrimSpace(c.Query("timeline_page_size")))
	return service.IPAuditQuery{
		Range: c.Query("range"), View: c.Query("view"),
		StartTimestamp: startTimestamp, EndTimestamp: endTimestamp,
		Page: pageInfo.GetPage(), PageSize: pageInfo.GetPageSize(),
		RelatedPage: relatedPage, RelatedPageSize: relatedPageSize,
		DetailUserPage: detailUserPage, DetailUserPageSize: detailUserPageSize,
		TimelinePage: timelinePage, TimelinePageSize: timelinePageSize,
	}, nil
}

func parseOptionalIPAuditTimestamp(c *gin.Context, key string) (int64, error) {
	raw, exists := c.GetQuery(key)
	if !exists {
		return 0, nil
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, service.ErrIPAuditInvalidRange
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value < 0 {
		return 0, service.ErrIPAuditInvalidRange
	}
	return value, nil
}
