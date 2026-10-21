package model

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/netip"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/ip_security_setting"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ipAuditProxyArtifactPatterns = []string{
	"0.%", "10.%", "127.%", "169.254.%", "192.168.%",
	"172.16.%", "172.17.%", "172.18.%", "172.19.%",
	"172.20.%", "172.21.%", "172.22.%", "172.23.%",
	"172.24.%", "172.25.%", "172.26.%", "172.27.%",
	"172.28.%", "172.29.%", "172.30.%", "172.31.%",
	"fc%", "fd%", "fe8%", "fe9%", "fea%", "feb%",
}

const (
	ipAuditProjectionAlias       = "ip_projection"
	ipAuditProjectionUserAlias   = "ip_projection_users"
	ipAuditProjectionDefaultSize = 50
	ipAuditProjectionMaxSize     = 100

	IPAuditViewRisk               = "risk"
	IPAuditViewSharedAPI          = "shared_api"
	IPAuditViewSharedRegistration = "shared_registration"
	IPAuditViewAuthentication     = "authentication"
	IPAuditViewAll                = "all"
)

type ipAuditProjectionScope struct {
	ctx       context.Context
	actorID   int
	actorRole int
	start     int64
	end       int64
	lifetime  bool
	exactOnly bool
}

type ipAuditProjectionRow struct {
	IP                      string `gorm:"column:ip"`
	RequestCount            int64  `gorm:"column:request_count"`
	RegistrationCount       int64  `gorm:"column:registration_count"`
	APIRequestCount         int64  `gorm:"column:api_request_count"`
	ErrorCount              int64  `gorm:"column:error_count"`
	UnauthorizedCount       int64  `gorm:"column:unauthorized_count"`
	LoginSuccessCount       int64  `gorm:"column:login_success_count"`
	LoginFailureCount       int64  `gorm:"column:login_failure_count"`
	LogoutCount             int64  `gorm:"column:logout_count"`
	AnomalyCount            int64  `gorm:"column:anomaly_count"`
	UserCount               int64  `gorm:"column:user_count"`
	RegistrationUserCount   int64  `gorm:"column:registration_user_count"`
	APIUserCount            int64  `gorm:"column:api_user_count"`
	AuthenticationUserCount int64  `gorm:"column:authentication_user_count"`
	Quota                   int64  `gorm:"column:quota"`
	FirstSeenAt             int64  `gorm:"column:first_seen_at"`
	LastSeenAt              int64  `gorm:"column:last_seen_at"`
}

type ipAuditProjectionTimelineRow struct {
	ID        int64  `gorm:"column:id"`
	CreatedAt int64  `gorm:"column:created_at"`
	UserID    int    `gorm:"column:user_id"`
	Username  string `gorm:"column:username"`
	Kind      string `gorm:"column:kind"`
	RequestID string `gorm:"column:request_id"`
}

type UserIPAuditProjectionSummary struct {
	RegistrationIP    string
	RecentIP          string
	APIRequestCount   int64
	ErrorCount        int64
	UnauthorizedCount int64
	LoginSuccessCount int64
	LoginFailureCount int64
	LogoutCount       int64
	AnomalyCount      int64
	LastActivityAt    int64
}

type UserIPAuditRelatedUser struct {
	UserID              int
	Username            string
	SharedIPCount       int64
	RegistrationCount   int64
	APICallCount        int64
	AuthenticationCount int64
	AnomalyCount        int64
	LastSeenAt          int64
}

type UserIPAuditProjection struct {
	Summary         UserIPAuditProjectionSummary
	RelatedUsers    []UserIPAuditRelatedUser
	RelatedTotal    int64
	RelatedPage     int
	RelatedPageSize int
	RelatedHasMore  bool
}

type userIPAuditProjectionSummaryRow struct {
	APIRequestCount   int64 `gorm:"column:api_request_count"`
	ErrorCount        int64 `gorm:"column:error_count"`
	UnauthorizedCount int64 `gorm:"column:unauthorized_count"`
	LoginSuccessCount int64 `gorm:"column:login_success_count"`
	LoginFailureCount int64 `gorm:"column:login_failure_count"`
	LogoutCount       int64 `gorm:"column:logout_count"`
	AnomalyCount      int64 `gorm:"column:anomaly_count"`
}

type userIPAuditProjectionActivityRow struct {
	IP         string `gorm:"column:ip"`
	LastSeenAt int64  `gorm:"column:last_seen_at"`
}

// QueryIPAuditProjection returns one SQL-paged view over the materialized
// primary-database projection. Lifetime requests use rollups plus pending
// events; bounded requests combine complete UTC-day rollups with pending
// events and exact events for partial days.
func QueryIPAuditProjection(ctx context.Context, actorID, actorRole int, start, end int64, page, pageSize int) ([]IPAuditItem, int64, error) {
	return QueryIPAuditProjectionForView(ctx, actorID, actorRole, start, end, page, pageSize, IPAuditViewAll)
}

func QueryIPAuditProjectionForView(
	ctx context.Context,
	actorID, actorRole int,
	start, end int64,
	page, pageSize int,
	view string,
) ([]IPAuditItem, int64, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, 0, err
	}
	if actorRole < common.RoleAdminUser {
		return nil, 0, errors.New("permission denied for IP audit")
	}
	view = strings.ToLower(strings.TrimSpace(view))
	switch view {
	case IPAuditViewRisk, IPAuditViewSharedAPI, IPAuditViewSharedRegistration, IPAuditViewAuthentication, IPAuditViewAll:
	default:
		return nil, 0, errors.New("invalid IP audit view")
	}
	page, pageSize = normalizeIPAuditProjectionPage(page, pageSize)
	scope := ipAuditProjectionScope{
		ctx: ctx, actorID: actorID, actorRole: actorRole,
		start: start, end: end, lifetime: start == 0 && end == 0,
	}
	groupColumn := scope.groupColumn()
	settings := ip_security_setting.GetSettings()
	viewQuery := func() *gorm.DB {
		query, selectArgs := scope.aggregateQuery()
		aggregated := query.
			Select(selectArgs.statement, selectArgs.args...).
			Group(groupColumn)
		tx := DB.WithContext(ctx).Table("(?) AS ip_audit_view", aggregated)
		switch view {
		case IPAuditViewRisk:
			proxyArtifactPredicate, proxyArtifactArgs := ipAuditProxyArtifactPredicate("ip")
			return tx.
				Where("registration_user_count >= ? OR api_user_count >= ? OR unauthorized_count >= ?", settings.SharedRegistrationUserThreshold, settings.SharedAPIUserThreshold, settings.UnauthorizedThreshold).
				Where("NOT "+proxyArtifactPredicate, proxyArtifactArgs...)
		case IPAuditViewSharedAPI:
			return tx.Where("api_user_count >= ?", settings.SharedAPIUserThreshold)
		case IPAuditViewSharedRegistration:
			return tx.Where("registration_user_count >= ?", settings.SharedRegistrationUserThreshold)
		case IPAuditViewAuthentication:
			return tx.Where("authentication_user_count > ? OR login_failure_count > ?", 1, 0)
		default:
			return tx
		}
	}

	var total int64
	if err := viewQuery().Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []IPAuditItem{}, 0, nil
	}
	offset, pastEnd, err := ipAuditProjectionPageOffset(page, pageSize, total)
	if err != nil {
		return nil, 0, err
	}
	if pastEnd {
		return []IPAuditItem{}, total, nil
	}

	var rows []ipAuditProjectionRow
	rowsQuery := viewQuery()
	switch view {
	case IPAuditViewSharedAPI:
		rowsQuery = rowsQuery.Order("api_user_count DESC").Order("api_request_count DESC")
	case IPAuditViewSharedRegistration:
		rowsQuery = rowsQuery.Order("registration_user_count DESC").Order("registration_count DESC")
	case IPAuditViewAuthentication:
		rowsQuery = rowsQuery.Order("login_failure_count DESC").Order("authentication_user_count DESC")
	default:
		rowsQuery = rowsQuery.
			Order(clause.Expr{SQL: "CASE WHEN registration_user_count >= ? OR api_user_count >= ? OR unauthorized_count >= ? THEN 1 ELSE 0 END DESC", Vars: []interface{}{settings.SharedRegistrationUserThreshold, settings.SharedAPIUserThreshold, settings.UnauthorizedThreshold}}).
			Order("unauthorized_count DESC").
			Order("registration_user_count DESC").
			Order("api_user_count DESC").
			Order("authentication_user_count DESC").
			Order("request_count DESC")
	}
	if err := rowsQuery.
		Order("last_seen_at DESC").
		Order("ip ASC").
		Limit(pageSize).
		Offset(int(offset)).
		Scan(&rows).Error; err != nil {
		return nil, 0, err
	}

	items := make([]IPAuditItem, 0, len(rows))
	for _, row := range rows {
		item, err := ipAuditProjectionItem(row)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, total, nil
}

// QueryUserIPAuditProjection returns rollup-backed user summary and related
// identities. Raw events are consulted only when they are pending projection
// or when an exact partial-day range requires them.
func QueryUserIPAuditProjection(
	ctx context.Context,
	actorID, actorRole, targetUserID int,
	start, end int64,
	relatedPage, relatedPageSize int,
) (UserIPAuditProjection, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return UserIPAuditProjection{}, err
	}
	if actorRole < common.RoleAdminUser || targetUserID <= 0 {
		return UserIPAuditProjection{}, errors.New("permission denied for user IP audit")
	}
	relatedPage, relatedPageSize = normalizeIPAuditProjectionPage(relatedPage, relatedPageSize)
	result := UserIPAuditProjection{
		RelatedUsers:    make([]UserIPAuditRelatedUser, 0),
		RelatedPage:     relatedPage,
		RelatedPageSize: relatedPageSize,
	}
	scope := ipAuditProjectionScope{
		ctx: ctx, actorID: actorID, actorRole: actorRole,
		start: start, end: end, lifetime: start == 0 && end == 0,
	}

	var summary userIPAuditProjectionSummaryRow
	usesAggregateCounters := scope.usesAggregateCounters()
	summarySelect := fmt.Sprintf(`
		COALESCE(SUM(%[1]s.api_request_count), 0) AS api_request_count,
		COALESCE(SUM(%[1]s.error_count), 0) AS error_count,
		COALESCE(SUM(%[1]s.unauthorized_count), 0) AS unauthorized_count,
		COALESCE(SUM(%[1]s.login_success_count), 0) AS login_success_count,
		COALESCE(SUM(%[1]s.login_failure_count), 0) AS login_failure_count,
		COALESCE(SUM(%[1]s.logout_count), 0) AS logout_count,
		COALESCE(SUM(%[1]s.anomaly_count), 0) AS anomaly_count`, ipAuditProjectionAlias)
	summaryArgs := make([]interface{}, 0)
	if !usesAggregateCounters {
		summarySelect = fmt.Sprintf(`
			COALESCE(SUM(CASE WHEN %[1]s.kind = ? THEN 1 ELSE 0 END), 0) AS api_request_count,
			COALESCE(SUM(CASE WHEN %[1]s.kind = ? THEN 1 ELSE 0 END), 0) AS error_count,
			COALESCE(SUM(CASE WHEN %[1]s.kind = ? AND %[1]s.http_status = ? THEN 1 ELSE 0 END), 0) AS unauthorized_count,
			COALESCE(SUM(CASE WHEN %[1]s.kind = ? THEN 1 ELSE 0 END), 0) AS login_success_count,
			COALESCE(SUM(CASE WHEN %[1]s.kind = ? THEN 1 ELSE 0 END), 0) AS login_failure_count,
			COALESCE(SUM(CASE WHEN %[1]s.kind = ? THEN 1 ELSE 0 END), 0) AS logout_count,
			COALESCE(SUM(CASE WHEN (%[1]s.kind = ? AND %[1]s.http_status = ?) OR %[1]s.kind = ? THEN 1 ELSE 0 END), 0) AS anomaly_count`, ipAuditProjectionAlias)
		summaryArgs = []interface{}{
			IPSecurityKindAPIRequest,
			IPSecurityKindAPIError,
			IPSecurityKindAPIError, 401,
			IPSecurityKindLoginSuccess,
			IPSecurityKindLoginFailure,
			IPSecurityKindLogout,
			IPSecurityKindAPIError, 401, IPSecurityKindLoginFailure,
		}
	}
	if err := scope.query().
		Where(ipAuditProjectionAlias+".user_id = ?", targetUserID).
		Select(summarySelect, summaryArgs...).
		Scan(&summary).Error; err != nil {
		return UserIPAuditProjection{}, err
	}
	result.Summary.APIRequestCount = summary.APIRequestCount
	result.Summary.ErrorCount = summary.ErrorCount
	result.Summary.UnauthorizedCount = summary.UnauthorizedCount
	result.Summary.LoginSuccessCount = summary.LoginSuccessCount
	result.Summary.LoginFailureCount = summary.LoginFailureCount
	result.Summary.LogoutCount = summary.LogoutCount
	result.Summary.AnomalyCount = summary.AnomalyCount

	groupColumn := scope.groupColumn()
	registrationTimeColumn := ipAuditProjectionAlias + ".first_seen_at"
	recentTimeColumn := ipAuditProjectionAlias + ".last_seen_at"
	registrationBase := scope.query().Where(ipAuditProjectionAlias+".user_id = ?", targetUserID)
	recentBase := scope.query().Where(ipAuditProjectionAlias+".user_id = ?", targetUserID)
	if usesAggregateCounters {
		registrationBase = registrationBase.Where(ipAuditProjectionAlias+".registration_count > ?", 0)
		recentBase = recentBase.Where(ipAuditProjectionAlias+".request_count > ?", 0)
	} else {
		registrationTimeColumn = ipAuditProjectionAlias + ".created_at"
		recentTimeColumn = ipAuditProjectionAlias + ".created_at"
		registrationBase = registrationBase.Where(ipAuditProjectionAlias+".kind = ?", IPSecurityKindRegistration)
		recentBase = recentBase.Where(ipAuditProjectionAlias+".kind <> ?", IPSecurityKindRegistration)
	}

	var registration userIPAuditProjectionActivityRow
	registrationQuery := registrationBase.
		Select(groupColumn + " AS ip, " + registrationTimeColumn + " AS last_seen_at").
		Order(registrationTimeColumn + " DESC").Order(groupColumn + " ASC").Limit(1).
		Scan(&registration)
	if registrationQuery.Error != nil {
		return UserIPAuditProjection{}, registrationQuery.Error
	}
	if registrationQuery.RowsAffected > 0 {
		result.Summary.RegistrationIP = registration.IP
	}

	var recent userIPAuditProjectionActivityRow
	recentQuery := recentBase.
		Select(groupColumn + " AS ip, " + recentTimeColumn + " AS last_seen_at").
		Order(recentTimeColumn + " DESC").Order(groupColumn + " ASC").Limit(1).
		Scan(&recent)
	if recentQuery.Error != nil {
		return UserIPAuditProjection{}, recentQuery.Error
	}
	if recentQuery.RowsAffected > 0 {
		result.Summary.RecentIP = recent.IP
		result.Summary.LastActivityAt = recent.LastSeenAt
	} else if registrationQuery.RowsAffected > 0 {
		result.Summary.RecentIP = registration.IP
		result.Summary.LastActivityAt = registration.LastSeenAt
	}

	proxyArtifactPredicate, proxyArtifactArgs := ipAuditProxyArtifactPredicate(ipAuditProjectionAlias + ".ip")
	targetIPs := scope.query().
		Select("DISTINCT "+ipAuditProjectionAlias+".ip").
		Where(ipAuditProjectionAlias+".user_id = ?", targetUserID).
		Where("NOT "+proxyArtifactPredicate, proxyArtifactArgs...)
	relatedByIP := scope.relatedProjectionByIPQuery(targetIPs, targetUserID)
	relatedBase := func() *gorm.DB {
		return DB.WithContext(ctx).
			Table("(?) AS related_ip_projection", relatedByIP)
	}
	relatedGroups := relatedBase().
		Select("related_ip_projection.user_id").
		Group("related_ip_projection.user_id")
	if err := DB.WithContext(ctx).
		Table("(?) AS user_ip_related_groups", relatedGroups).
		Count(&result.RelatedTotal).Error; err != nil {
		return UserIPAuditProjection{}, err
	}
	if result.RelatedTotal == 0 {
		return result, ctx.Err()
	}
	if actorRole < common.RoleRootUser {
		return result, ctx.Err()
	}
	offset, pastEnd, err := ipAuditProjectionPageOffset(relatedPage, relatedPageSize, result.RelatedTotal)
	if err != nil {
		return UserIPAuditProjection{}, err
	}
	if pastEnd {
		return result, ctx.Err()
	}
	if err := relatedBase().
		Joins("LEFT JOIN users AS current_related_users ON current_related_users.id = related_ip_projection.user_id").
		Select(`
			related_ip_projection.user_id AS user_id,
			COALESCE(MAX(NULLIF(current_related_users.username, '')), MAX(related_ip_projection.username)) AS username,
			COUNT(DISTINCT related_ip_projection.ip) AS shared_ip_count,
			COALESCE(SUM(related_ip_projection.registration_count), 0) AS registration_count,
			COALESCE(SUM(related_ip_projection.api_call_count), 0) AS api_call_count,
			COALESCE(SUM(related_ip_projection.authentication_count), 0) AS authentication_count,
			COALESCE(SUM(related_ip_projection.anomaly_count), 0) AS anomaly_count,
			MAX(related_ip_projection.last_seen_at) AS last_seen_at`).
		Group("related_ip_projection.user_id").
		Order("registration_count DESC").
		Order("anomaly_count DESC").
		Order("shared_ip_count DESC").
		Order("last_seen_at DESC").
		Order("username ASC").
		Order("user_id ASC").
		Limit(relatedPageSize).Offset(int(offset)).
		Scan(&result.RelatedUsers).Error; err != nil {
		return UserIPAuditProjection{}, err
	}
	result.RelatedHasMore = offset+int64(len(result.RelatedUsers)) < result.RelatedTotal
	return result, ctx.Err()
}

func QueryIPAuditProjectionDetail(ctx context.Context, actorID, actorRole int, start, end int64, ip string) (IPAuditProjectionDetail, error) {
	return QueryIPAuditProjectionDetailPage(ctx, actorID, actorRole, start, end, ip, 1, 20, 1, 20)
}

func QueryIPAuditProjectionDetailPage(
	ctx context.Context,
	actorID, actorRole int,
	start, end int64,
	ip string,
	userPage, userPageSize, timelinePage, timelinePageSize int,
) (IPAuditProjectionDetail, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return IPAuditProjectionDetail{}, err
	}
	if actorRole < common.RoleAdminUser {
		return IPAuditProjectionDetail{}, errors.New("permission denied for IP audit details")
	}
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return IPAuditProjectionDetail{}, errors.New("IP audit detail requires an IP")
	}
	userPage, userPageSize = normalizeIPAuditProjectionPage(userPage, userPageSize)
	timelinePage, timelinePageSize = normalizeIPAuditProjectionPage(timelinePage, timelinePageSize)

	scope := ipAuditProjectionScope{
		ctx: ctx, actorID: actorID, actorRole: actorRole,
		start: start, end: end, lifetime: start == 0 && end == 0,
	}
	detail := IPAuditProjectionDetail{
		IP: ip, Users: make([]IPAuditUser, 0), Timeline: make([]IPAuditTimelineItem, 0),
		UserPage: userPage, UserPageSize: userPageSize,
		TimelinePage: timelinePage, TimelinePageSize: timelinePageSize,
	}
	userGroups := scope.query().
		Select(ipAuditProjectionAlias+".user_id").
		Where(scope.groupColumn()+" = ?", ip).
		Where(ipAuditProjectionAlias+".user_id > ?", 0).
		Group(ipAuditProjectionAlias + ".user_id")
	var userTotal int64
	if err := DB.WithContext(ctx).Table("(?) AS ip_audit_detail_users", userGroups).Count(&userTotal).Error; err != nil {
		return IPAuditProjectionDetail{}, err
	}
	convertedUserTotal, err := checkedIPAuditProjectionInt("detail_user_total", userTotal)
	if err != nil {
		return IPAuditProjectionDetail{}, err
	}
	detail.UserTotal = convertedUserTotal
	userOffset, usersPastEnd, err := ipAuditProjectionPageOffset(userPage, userPageSize, userTotal)
	if err != nil {
		return IPAuditProjectionDetail{}, err
	}
	if !usersPastEnd {
		if err := scope.query().
			Select(ipAuditProjectionAlias+".user_id, MAX("+ipAuditProjectionAlias+".username) AS username").
			Where(scope.groupColumn()+" = ?", ip).
			Where(ipAuditProjectionAlias+".user_id > ?", 0).
			Group(ipAuditProjectionAlias + ".user_id").
			Order("username ASC").
			Order("user_id ASC").
			Limit(userPageSize).Offset(int(userOffset)).
			Scan(&detail.Users).Error; err != nil {
			return IPAuditProjectionDetail{}, err
		}
		detail.UserHasMore = userOffset+int64(len(detail.Users)) < userTotal
	}

	timelineScope := scope
	timelineScope.lifetime = false
	timelineScope.exactOnly = true
	timelineBase := timelineScope.query().Where(timelineScope.groupColumn()+" = ?", ip)
	var timelineTotal int64
	if err := timelineBase.Count(&timelineTotal).Error; err != nil {
		return IPAuditProjectionDetail{}, err
	}
	convertedTimelineTotal, err := checkedIPAuditProjectionInt("detail_timeline_total", timelineTotal)
	if err != nil {
		return IPAuditProjectionDetail{}, err
	}
	detail.TimelineTotal = convertedTimelineTotal
	timelineOffset, timelinePastEnd, err := ipAuditProjectionPageOffset(timelinePage, timelinePageSize, timelineTotal)
	if err != nil {
		return IPAuditProjectionDetail{}, err
	}
	if timelinePastEnd {
		return detail, ctx.Err()
	}
	var timelineRows []ipAuditProjectionTimelineRow
	if err := timelineScope.query().
		Select(ipAuditProjectionAlias+".id, "+ipAuditProjectionAlias+".created_at, "+
			ipAuditProjectionAlias+".user_id, "+ipAuditProjectionAlias+".username, "+
			ipAuditProjectionAlias+".kind, "+ipAuditProjectionAlias+".request_id").
		Where(timelineScope.groupColumn()+" = ?", ip).
		Order(ipAuditProjectionAlias + ".created_at DESC").
		Order(ipAuditProjectionAlias + ".request_id DESC").
		Order(ipAuditProjectionAlias + ".id DESC").
		Limit(timelinePageSize).Offset(int(timelineOffset)).
		Scan(&timelineRows).Error; err != nil {
		return IPAuditProjectionDetail{}, err
	}
	for _, row := range timelineRows {
		detail.Timeline = append(detail.Timeline, IPAuditTimelineItem{
			CreatedAt: row.CreatedAt,
			UserId:    row.UserID,
			Username:  row.Username,
			Kind:      row.Kind,
			RequestId: row.RequestID,
		})
	}
	detail.TimelineHasMore = timelineOffset+int64(len(detail.Timeline)) < timelineTotal
	return detail, ctx.Err()
}

func (scope ipAuditProjectionScope) relatedProjectionByIPQuery(targetIPs *gorm.DB, targetUserID int) *gorm.DB {
	query := scope.query().
		Joins("JOIN (?) AS target_ips ON target_ips.ip = "+ipAuditProjectionAlias+".ip", targetIPs).
		Where(ipAuditProjectionAlias+".user_id > ?", 0).
		Where(ipAuditProjectionAlias+".user_id <> ?", targetUserID)
	if scope.usesAggregateCounters() {
		return query.
			Select(fmt.Sprintf(`
				%s.user_id AS user_id,
				MAX(%s.username) AS username,
				%s.ip AS ip,
				COALESCE(SUM(%s.registration_count), 0) AS registration_count,
				COALESCE(SUM(%s.api_request_count), 0) + COALESCE(SUM(%s.error_count), 0) AS api_call_count,
				COALESCE(SUM(%s.login_success_count), 0) + COALESCE(SUM(%s.login_failure_count), 0) + COALESCE(SUM(%s.logout_count), 0) AS authentication_count,
				COALESCE(SUM(%s.anomaly_count), 0) AS anomaly_count,
				MAX(%s.last_seen_at) AS last_seen_at`,
				ipAuditProjectionAlias,
				ipAuditProjectionAlias,
				ipAuditProjectionAlias,
				ipAuditProjectionAlias,
				ipAuditProjectionAlias, ipAuditProjectionAlias,
				ipAuditProjectionAlias, ipAuditProjectionAlias, ipAuditProjectionAlias,
				ipAuditProjectionAlias,
				ipAuditProjectionAlias,
			)).
			Group(ipAuditProjectionAlias + ".user_id").
			Group(ipAuditProjectionAlias + ".ip")
	}

	return query.
		Select(fmt.Sprintf(`
			%s.user_id AS user_id,
			MAX(%s.username) AS username,
			%s.ip AS ip,
			COALESCE(SUM(CASE WHEN %s.kind = ? THEN 1 ELSE 0 END), 0) AS registration_count,
			COALESCE(SUM(CASE WHEN %s.kind IN (?, ?) THEN 1 ELSE 0 END), 0) AS api_call_count,
			COALESCE(SUM(CASE WHEN %s.kind IN (?, ?, ?) THEN 1 ELSE 0 END), 0) AS authentication_count,
			COALESCE(SUM(CASE WHEN %s.kind IN (?, ?) THEN 1 ELSE 0 END), 0) AS anomaly_count,
			MAX(%s.created_at) AS last_seen_at`,
			ipAuditProjectionAlias,
			ipAuditProjectionAlias,
			ipAuditProjectionAlias,
			ipAuditProjectionAlias,
			ipAuditProjectionAlias,
			ipAuditProjectionAlias,
			ipAuditProjectionAlias,
			ipAuditProjectionAlias,
		),
			IPSecurityKindRegistration,
			IPSecurityKindAPIRequest, IPSecurityKindAPIError,
			IPSecurityKindLoginSuccess, IPSecurityKindLoginFailure, IPSecurityKindLogout,
			IPSecurityKindAPIError, IPSecurityKindLoginFailure,
		).
		Group(ipAuditProjectionAlias + ".user_id").
		Group(ipAuditProjectionAlias + ".ip")
}

type ipAuditProjectionSelect struct {
	statement string
	args      []interface{}
}

func (scope ipAuditProjectionScope) query() *gorm.DB {
	var tx *gorm.DB
	if scope.lifetime {
		tx = DB.WithContext(scope.ctx).Table(
			"(?) AS "+ipAuditProjectionAlias,
			scope.lifetimeQuery(),
		)
	} else {
		dailyStart, dailyEnd, hasCompleteDays := scope.dailyRollupBounds()
		if hasCompleteDays && !scope.exactOnly {
			tx = DB.WithContext(scope.ctx).Table(
				"(?) AS "+ipAuditProjectionAlias,
				scope.mixedRangeQuery(dailyStart, dailyEnd),
			)
		} else {
			tx = DB.WithContext(scope.ctx).Table("ip_security_events AS " + ipAuditProjectionAlias)
			if scope.start != 0 {
				tx = tx.Where(ipAuditProjectionAlias+".created_at >= ?", scope.start)
			}
			if scope.end != 0 {
				tx = tx.Where(ipAuditProjectionAlias+".created_at <= ?", scope.end)
			}
		}
	}
	if !scope.superRoot() {
		tx = tx.Joins(
			"JOIN users AS "+ipAuditProjectionUserAlias+
				" ON "+ipAuditProjectionUserAlias+".id = "+ipAuditProjectionAlias+".user_id"+
				" AND "+ipAuditProjectionUserAlias+".deleted_at IS NULL",
		).Where(ipAuditProjectionUserAlias+".role < ?", scope.actorRole)
	}
	if scope.usesAggregateCounters() {
		tx = tx.Where("("+
			ipAuditProjectionAlias+".registration_count > ? OR "+
			ipAuditProjectionAlias+".api_request_count > ? OR "+
			ipAuditProjectionAlias+".error_count > ? OR "+
			ipAuditProjectionAlias+".login_success_count > ? OR "+
			ipAuditProjectionAlias+".login_failure_count > ? OR "+
			ipAuditProjectionAlias+".logout_count > ?)",
			0, 0, 0, 0, 0, 0,
		)
	} else {
		tx = tx.Where(ipAuditProjectionAlias+".kind IN ?", []string{
			IPSecurityKindRegistration,
			IPSecurityKindAPIRequest,
			IPSecurityKindAPIError,
			IPSecurityKindLoginSuccess,
			IPSecurityKindLoginFailure,
			IPSecurityKindLogout,
		}).Where("("+ipAuditProjectionAlias+".kind <> ? OR "+ipAuditProjectionAlias+".http_status = ?)", IPSecurityKindAPIError, http.StatusUnauthorized)
	}
	return tx.Where(scope.groupColumn()+" <> ?", "")
}

func (scope ipAuditProjectionScope) lifetimeQuery() *gorm.DB {
	eventProjection, eventArgs := ipAuditEventProjection(0, 0, true)
	statement := `
		SELECT user_id, username, ip, masked_ip,
			api_request_count + unauthorized_count + login_success_count + login_failure_count + logout_count AS request_count,
			registration_count, api_request_count, unauthorized_count AS error_count,
			unauthorized_count,
			login_success_count, login_failure_count, logout_count,
			unauthorized_count + login_failure_count AS anomaly_count,
			quota, first_seen_at, last_seen_at
		FROM ip_audit_rollups
		WHERE bucket = ?
	UNION ALL
` + eventProjection
	args := []interface{}{int64(0)}
	args = append(args, eventArgs...)
	return DB.WithContext(scope.ctx).Raw(statement, args...)
}

func (scope ipAuditProjectionScope) dailyRollupBounds() (int64, int64, bool) {
	if scope.start <= 0 || scope.end <= 0 || scope.start > scope.end {
		return 0, 0, false
	}

	const daySeconds int64 = 86400
	dailyStart := utcDayBucket(scope.start)
	if dailyStart != scope.start {
		if dailyStart > math.MaxInt64-daySeconds {
			return 0, 0, false
		}
		dailyStart += daySeconds
	}

	dailyEnd := utcDayBucket(scope.end)
	if scope.end-dailyEnd == daySeconds-1 {
		if dailyEnd > math.MaxInt64-daySeconds {
			return 0, 0, false
		}
		dailyEnd += daySeconds
	}
	return dailyStart, dailyEnd, dailyStart < dailyEnd
}

func (scope ipAuditProjectionScope) mixedRangeQuery(dailyStart, dailyEnd int64) *gorm.DB {
	parts := []string{`
		SELECT user_id, username, ip, masked_ip,
			api_request_count + unauthorized_count + login_success_count + login_failure_count + logout_count AS request_count,
			registration_count, api_request_count, unauthorized_count AS error_count,
			unauthorized_count,
			login_success_count, login_failure_count, logout_count,
			unauthorized_count + login_failure_count AS anomaly_count,
			quota, first_seen_at, last_seen_at
		FROM ip_audit_rollups
		WHERE bucket >= ? AND bucket < ?`}
	args := []interface{}{dailyStart, dailyEnd}
	appendEventRange := func(start, end int64, pendingOnly bool) {
		eventProjection, eventArgs := ipAuditEventProjection(start, end, pendingOnly)
		parts = append(parts, eventProjection)
		args = append(args, eventArgs...)
	}
	appendEventRange(dailyStart, dailyEnd-1, true)
	if scope.start < dailyStart {
		appendEventRange(scope.start, dailyStart-1, false)
	}
	if dailyEnd <= scope.end {
		appendEventRange(dailyEnd, scope.end, false)
	}
	return DB.WithContext(scope.ctx).Raw(strings.Join(parts, "\nUNION ALL\n"), args...)
}

func ipAuditEventProjection(start, end int64, pendingOnly bool) (string, []interface{}) {
	statement := `
		SELECT user_id, username, ip, masked_ip,
			CASE WHEN kind IN (?, ?, ?, ?) OR (kind = ? AND http_status = ?) THEN 1 ELSE 0 END AS request_count,
			CASE WHEN kind = ? THEN 1 ELSE 0 END AS registration_count,
			CASE WHEN kind = ? THEN 1 ELSE 0 END AS api_request_count,
			CASE WHEN kind = ? AND http_status = ? THEN 1 ELSE 0 END AS error_count,
			CASE WHEN kind = ? AND http_status = ? THEN 1 ELSE 0 END AS unauthorized_count,
			CASE WHEN kind = ? THEN 1 ELSE 0 END AS login_success_count,
			CASE WHEN kind = ? THEN 1 ELSE 0 END AS login_failure_count,
			CASE WHEN kind = ? THEN 1 ELSE 0 END AS logout_count,
			CASE WHEN (kind = ? AND http_status = ?) OR kind = ? THEN 1 ELSE 0 END AS anomaly_count,
			quota, created_at AS first_seen_at, created_at AS last_seen_at
		FROM ip_security_events`
	args := []interface{}{
		IPSecurityKindAPIRequest, IPSecurityKindLoginSuccess, IPSecurityKindLoginFailure, IPSecurityKindLogout,
		IPSecurityKindAPIError, http.StatusUnauthorized,
		IPSecurityKindRegistration,
		IPSecurityKindAPIRequest,
		IPSecurityKindAPIError, http.StatusUnauthorized,
		IPSecurityKindAPIError, http.StatusUnauthorized,
		IPSecurityKindLoginSuccess,
		IPSecurityKindLoginFailure,
		IPSecurityKindLogout,
		IPSecurityKindAPIError, http.StatusUnauthorized, IPSecurityKindLoginFailure,
	}
	predicates := make([]string, 0, 3)
	if pendingOnly {
		predicates = append(predicates, "projection_status = ?")
		args = append(args, IPSecurityEventProjectionPending)
	}
	if start != 0 {
		predicates = append(predicates, "created_at >= ?")
		args = append(args, start)
	}
	if end != 0 {
		predicates = append(predicates, "created_at <= ?")
		args = append(args, end)
	}
	return statement + "\n\t\tWHERE " + strings.Join(predicates, " AND "), args
}

func (scope ipAuditProjectionScope) superRoot() bool {
	return scope.actorID == 1 && scope.actorRole >= common.RoleRootUser
}

func (scope ipAuditProjectionScope) groupColumn() string {
	if scope.actorRole >= common.RoleRootUser {
		return ipAuditProjectionAlias + ".ip"
	}
	return ipAuditProjectionAlias + ".masked_ip"
}

func (scope ipAuditProjectionScope) aggregateQuery() (*gorm.DB, ipAuditProjectionSelect) {
	groupColumn := scope.groupColumn()
	if scope.usesAggregateCounters() {
		return scope.query(), ipAuditProjectionSelect{statement: fmt.Sprintf(`
			%s AS ip,
			COALESCE(SUM(%[2]s.api_request_count + %[2]s.error_count + %[2]s.login_success_count + %[2]s.login_failure_count + %[2]s.logout_count), 0) AS request_count,
			COALESCE(SUM(%[2]s.registration_count), 0) AS registration_count,
			COALESCE(SUM(%[2]s.api_request_count), 0) AS api_request_count,
			COALESCE(SUM(%[2]s.error_count), 0) AS error_count,
			COALESCE(SUM(%[2]s.unauthorized_count), 0) AS unauthorized_count,
			COALESCE(SUM(%[2]s.login_success_count), 0) AS login_success_count,
			COALESCE(SUM(%[2]s.login_failure_count), 0) AS login_failure_count,
			COALESCE(SUM(%[2]s.logout_count), 0) AS logout_count,
			COALESCE(SUM(%[2]s.anomaly_count), 0) AS anomaly_count,
			COUNT(DISTINCT CASE WHEN %[2]s.user_id > 0 THEN %[2]s.user_id ELSE NULL END) AS user_count,
			COUNT(DISTINCT CASE WHEN %[2]s.user_id > 0 AND %[2]s.registration_count > 0 THEN %[2]s.user_id ELSE NULL END) AS registration_user_count,
			COUNT(DISTINCT CASE WHEN %[2]s.user_id > 0 AND (%[2]s.api_request_count > 0 OR %[2]s.error_count > 0) THEN %[2]s.user_id ELSE NULL END) AS api_user_count,
			COUNT(DISTINCT CASE WHEN %[2]s.user_id > 0 AND (%[2]s.login_success_count > 0 OR %[2]s.login_failure_count > 0 OR %[2]s.logout_count > 0) THEN %[2]s.user_id ELSE NULL END) AS authentication_user_count,
			COALESCE(SUM(%[2]s.quota), 0) AS quota,
			MIN(%[2]s.first_seen_at) AS first_seen_at,
			MAX(%[2]s.last_seen_at) AS last_seen_at`,
			groupColumn, ipAuditProjectionAlias,
		)}
	}

	return scope.query(), ipAuditProjectionSelect{
		statement: fmt.Sprintf(`
			%s AS ip,
			COALESCE(SUM(CASE WHEN %[2]s.kind <> ? THEN 1 ELSE 0 END), 0) AS request_count,
			COALESCE(SUM(CASE WHEN %[2]s.kind = ? THEN 1 ELSE 0 END), 0) AS registration_count,
			COALESCE(SUM(CASE WHEN %[2]s.kind = ? THEN 1 ELSE 0 END), 0) AS api_request_count,
			COALESCE(SUM(CASE WHEN %[2]s.kind = ? THEN 1 ELSE 0 END), 0) AS error_count,
			COALESCE(SUM(CASE WHEN %[2]s.kind = ? AND %[2]s.http_status = ? THEN 1 ELSE 0 END), 0) AS unauthorized_count,
			COALESCE(SUM(CASE WHEN %[2]s.kind = ? THEN 1 ELSE 0 END), 0) AS login_success_count,
			COALESCE(SUM(CASE WHEN %[2]s.kind = ? THEN 1 ELSE 0 END), 0) AS login_failure_count,
			COALESCE(SUM(CASE WHEN %[2]s.kind = ? THEN 1 ELSE 0 END), 0) AS logout_count,
			COALESCE(SUM(CASE WHEN (%[2]s.kind = ? AND %[2]s.http_status = ?) OR %[2]s.kind = ? THEN 1 ELSE 0 END), 0) AS anomaly_count,
			COUNT(DISTINCT CASE WHEN %[2]s.user_id > 0 THEN %[2]s.user_id ELSE NULL END) AS user_count,
			COUNT(DISTINCT CASE WHEN %[2]s.user_id > 0 AND %[2]s.kind = ? THEN %[2]s.user_id ELSE NULL END) AS registration_user_count,
			COUNT(DISTINCT CASE WHEN %[2]s.user_id > 0 AND %[2]s.kind IN (?, ?) THEN %[2]s.user_id ELSE NULL END) AS api_user_count,
			COUNT(DISTINCT CASE WHEN %[2]s.user_id > 0 AND %[2]s.kind IN (?, ?, ?) THEN %[2]s.user_id ELSE NULL END) AS authentication_user_count,
			COALESCE(SUM(%[2]s.quota), 0) AS quota,
			MIN(%[2]s.created_at) AS first_seen_at,
			MAX(%[2]s.created_at) AS last_seen_at`,
			groupColumn, ipAuditProjectionAlias,
		),
		args: []interface{}{
			IPSecurityKindRegistration,
			IPSecurityKindRegistration,
			IPSecurityKindAPIRequest,
			IPSecurityKindAPIError,
			IPSecurityKindAPIError, 401,
			IPSecurityKindLoginSuccess,
			IPSecurityKindLoginFailure,
			IPSecurityKindLogout,
			IPSecurityKindAPIError, 401, IPSecurityKindLoginFailure,
			IPSecurityKindRegistration,
			IPSecurityKindAPIRequest, IPSecurityKindAPIError,
			IPSecurityKindLoginSuccess, IPSecurityKindLoginFailure, IPSecurityKindLogout,
		},
	}
}

func ipAuditProxyArtifactPredicate(column string) (string, []interface{}) {
	clauses := []string{column + " = ?"}
	args := []interface{}{"::1"}
	for _, pattern := range ipAuditProxyArtifactPatterns {
		clauses = append(clauses, "LOWER("+column+") LIKE ?")
		args = append(args, pattern)
	}
	return "(" + strings.Join(clauses, " OR ") + ")", args
}

func isIPAuditProxyArtifact(ip string) bool {
	normalized := strings.ToLower(strings.TrimSpace(ip))
	if normalized == "::1" {
		return true
	}
	if address, err := netip.ParseAddr(normalized); err == nil {
		return address.IsPrivate() || address.IsLoopback() || address.IsLinkLocalUnicast() || address.IsUnspecified()
	}
	for _, pattern := range ipAuditProxyArtifactPatterns {
		if strings.HasPrefix(normalized, strings.TrimSuffix(pattern, "%")) {
			return true
		}
	}
	return false
}

func (scope ipAuditProjectionScope) usesAggregateCounters() bool {
	_, _, usesDailyRollups := scope.dailyRollupBounds()
	return scope.lifetime || (usesDailyRollups && !scope.exactOnly)
}

func normalizeIPAuditProjectionPage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = ipAuditProjectionDefaultSize
	}
	if pageSize > ipAuditProjectionMaxSize {
		pageSize = ipAuditProjectionMaxSize
	}
	return page, pageSize
}

func ipAuditProjectionPageOffset(page, pageSize int, total int64) (int64, bool, error) {
	pageIndex := int64(page - 1)
	if pageIndex > math.MaxInt64/int64(pageSize) {
		return 0, true, nil
	}
	offset := pageIndex * int64(pageSize)
	if offset >= total {
		return offset, true, nil
	}
	if offset > int64(^uint(0)>>1) {
		return 0, false, errors.New("IP audit page offset exceeds platform integer range")
	}
	return offset, false, nil
}

func ipAuditProjectionItem(row ipAuditProjectionRow) (IPAuditItem, error) {
	requestCount, err := checkedIPAuditProjectionInt("request_count", row.RequestCount)
	if err != nil {
		return IPAuditItem{}, err
	}
	registrationCount, err := checkedIPAuditProjectionInt("registration_count", row.RegistrationCount)
	if err != nil {
		return IPAuditItem{}, err
	}
	apiRequestCount, err := checkedIPAuditProjectionInt("api_request_count", row.APIRequestCount)
	if err != nil {
		return IPAuditItem{}, err
	}
	errorCount, err := checkedIPAuditProjectionInt("error_count", row.ErrorCount)
	if err != nil {
		return IPAuditItem{}, err
	}
	unauthorizedCount, err := checkedIPAuditProjectionInt("unauthorized_count", row.UnauthorizedCount)
	if err != nil {
		return IPAuditItem{}, err
	}
	loginSuccessCount, err := checkedIPAuditProjectionInt("login_success_count", row.LoginSuccessCount)
	if err != nil {
		return IPAuditItem{}, err
	}
	loginFailureCount, err := checkedIPAuditProjectionInt("login_failure_count", row.LoginFailureCount)
	if err != nil {
		return IPAuditItem{}, err
	}
	logoutCount, err := checkedIPAuditProjectionInt("logout_count", row.LogoutCount)
	if err != nil {
		return IPAuditItem{}, err
	}
	anomalyCount, err := checkedIPAuditProjectionInt("anomaly_count", row.AnomalyCount)
	if err != nil {
		return IPAuditItem{}, err
	}
	userCount, err := checkedIPAuditProjectionInt("user_count", row.UserCount)
	if err != nil {
		return IPAuditItem{}, err
	}
	registrationUserCount, err := checkedIPAuditProjectionInt("registration_user_count", row.RegistrationUserCount)
	if err != nil {
		return IPAuditItem{}, err
	}
	apiUserCount, err := checkedIPAuditProjectionInt("api_user_count", row.APIUserCount)
	if err != nil {
		return IPAuditItem{}, err
	}
	authenticationUserCount, err := checkedIPAuditProjectionInt("authentication_user_count", row.AuthenticationUserCount)
	if err != nil {
		return IPAuditItem{}, err
	}
	quota, err := checkedIPAuditProjectionInt("quota", row.Quota)
	if err != nil {
		return IPAuditItem{}, err
	}
	proxyArtifact := isIPAuditProxyArtifact(row.IP)
	settings := ip_security_setting.GetSettings()
	riskReasons := make([]string, 0, 3)
	if registrationUserCount >= settings.SharedRegistrationUserThreshold {
		riskReasons = append(riskReasons, "shared_registration_ip")
	}
	if apiUserCount >= settings.SharedAPIUserThreshold {
		riskReasons = append(riskReasons, "shared_api_ip")
	}
	if unauthorizedCount >= settings.UnauthorizedThreshold {
		riskReasons = append(riskReasons, "unauthorized_burst")
	}
	return IPAuditItem{
		IP: row.IP, ProxyArtifact: proxyArtifact, RiskReasons: riskReasons, RiskLevel: len(riskReasons),
		RequestCount: requestCount, RegistrationCount: registrationCount,
		APIRequestCount: apiRequestCount, ErrorCount: errorCount, UnauthorizedCount: unauthorizedCount,
		LoginSuccessCount: loginSuccessCount, LoginFailureCount: loginFailureCount,
		LogoutCount: logoutCount, AnomalyCount: anomalyCount, UserCount: userCount,
		RegistrationUserCount: registrationUserCount, APIUserCount: apiUserCount,
		AuthenticationUserCount: authenticationUserCount,
		Quota:                   quota, FirstSeenAt: row.FirstSeenAt, LastSeenAt: row.LastSeenAt,
	}, nil
}

func checkedIPAuditProjectionInt(field string, value int64) (int, error) {
	maxInt := int64(^uint(0) >> 1)
	minInt := -maxInt - 1
	if value < minInt || value > maxInt {
		return 0, fmt.Errorf("IP audit field %s exceeds platform integer range", field)
	}
	return int(value), nil
}
