package service

import (
	"context"
	"errors"
	"net/http"
	"net/netip"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/ip_security_setting"
	"gorm.io/gorm"
)

var (
	ErrIPAuditForbidden          = errors.New("ip_audit_forbidden")
	ErrIPAuditInvalidIP          = errors.New("ip_audit_invalid_ip")
	ErrIPAuditInvalidRange       = errors.New("ip_audit_invalid_range")
	ErrIPAuditInvalidView        = errors.New("ip_audit_invalid_view")
	ErrIPAuditProjectionNotReady = errors.New("ip_audit_projection_not_ready")
	nonPublicInternetIPPrefixes  = []netip.Prefix{
		netip.MustParsePrefix("0.0.0.0/8"),
		netip.MustParsePrefix("100.64.0.0/10"),
		netip.MustParsePrefix("192.0.0.0/24"),
		netip.MustParsePrefix("192.0.2.0/24"),
		netip.MustParsePrefix("192.88.99.0/24"),
		netip.MustParsePrefix("198.18.0.0/15"),
		netip.MustParsePrefix("198.51.100.0/24"),
		netip.MustParsePrefix("203.0.113.0/24"),
		netip.MustParsePrefix("240.0.0.0/4"),
		netip.MustParsePrefix("100::/64"),
		netip.MustParsePrefix("2001::/23"),
		netip.MustParsePrefix("2001:db8::/32"),
		netip.MustParsePrefix("2002::/16"),
		netip.MustParsePrefix("3fff::/20"),
	}
)

const (
	IPAuditRange7Days  = "7d"
	IPAuditRange30Days = "30d"
	IPAuditRange90Days = "90d"
	IPAuditRangeAll    = "all"
	IPAuditRangeCustom = "custom"

	IPAuditViewRisk               = model.IPAuditViewRisk
	IPAuditViewSharedAPI          = model.IPAuditViewSharedAPI
	IPAuditViewSharedRegistration = model.IPAuditViewSharedRegistration
	IPAuditViewAuthentication     = model.IPAuditViewAuthentication
	IPAuditViewAll                = model.IPAuditViewAll

	TrustedProxiesNotConfigured = "trusted_proxies_not_configured"
	TrustedProxiesUnsafe        = "trusted_proxies_unsafe"

	ipAuditReadAttempts       = 3
	ipAuditReadRetryBaseDelay = 25 * time.Millisecond
)

type IPAuditQuery struct {
	Range              string
	View               string
	StartTimestamp     int64
	EndTimestamp       int64
	Page               int
	PageSize           int
	RelatedPage        int
	RelatedPageSize    int
	DetailUserPage     int
	DetailUserPageSize int
	TimelinePage       int
	TimelinePageSize   int
}

type IPAuditPage struct {
	Enabled        bool                `json:"enabled"`
	Items          []model.IPAuditItem `json:"items"`
	Total          int                 `json:"total"`
	Page           int                 `json:"page"`
	PageSize       int                 `json:"page_size"`
	Range          string              `json:"range"`
	View           string              `json:"view"`
	StartTimestamp int64               `json:"start_timestamp"`
	EndTimestamp   int64               `json:"end_timestamp"`
	IsRoot         bool                `json:"is_root"`
	CanViewDetails bool                `json:"can_view_details"`
	ProxyWarning   string              `json:"proxy_warning,omitempty"`
	Sync           IPAuditSyncStatus   `json:"sync"`
}

type IPAuditDetail struct {
	Enabled bool `json:"enabled"`
	model.IPAuditProjectionDetail
	Range          string            `json:"range"`
	StartTimestamp int64             `json:"start_timestamp"`
	EndTimestamp   int64             `json:"end_timestamp"`
	Sync           IPAuditSyncStatus `json:"sync"`
}

type IPAuditSyncStatus struct {
	Status         string `json:"status"`
	Ready          bool   `json:"ready"`
	ProcessedLogs  int64  `json:"processed_logs"`
	ProcessedUsers int64  `json:"processed_users"`
	StartedAt      int64  `json:"started_at"`
	UpdatedAt      int64  `json:"updated_at"`
}

type UserSecurityAuditSummary struct {
	RegistrationIP    string `json:"registration_ip"`
	RecentIP          string `json:"recent_ip"`
	APIRequestCount   int    `json:"api_request_count"`
	ErrorCount        int    `json:"error_count"`
	UnauthorizedCount int    `json:"unauthorized_count"`
	LoginSuccessCount int    `json:"login_success_count"`
	LoginFailureCount int    `json:"login_failure_count"`
	LogoutCount       int    `json:"logout_count"`
	AnomalyCount      int    `json:"anomaly_count"`
	RelatedUserCount  int    `json:"related_user_count"`
	LastActivityAt    int64  `json:"last_activity_at"`
}

type UserSecurityRelatedUser struct {
	UserID              int    `json:"user_id"`
	Username            string `json:"username"`
	SharedIPCount       int    `json:"shared_ip_count"`
	RegistrationCount   int    `json:"registration_count"`
	APICallCount        int    `json:"api_call_count"`
	AuthenticationCount int    `json:"authentication_count"`
	AnomalyCount        int    `json:"anomaly_count"`
	LastSeenAt          int64  `json:"last_seen_at"`
}

type UserSecurityTimelineItem struct {
	CreatedAt int64  `json:"created_at"`
	IP        string `json:"ip"`
	Kind      string `json:"kind"`
	RequestID string `json:"request_id,omitempty"`
}

type UserSecurityAudit struct {
	Enabled         bool                       `json:"enabled"`
	TargetUserID    int                        `json:"target_user_id"`
	Username        string                     `json:"username"`
	Projection      string                     `json:"projection"`
	IsRoot          bool                       `json:"is_root"`
	ProxyWarning    string                     `json:"proxy_warning,omitempty"`
	Summary         UserSecurityAuditSummary   `json:"summary"`
	RelatedUsers    []UserSecurityRelatedUser  `json:"related_users"`
	RelatedTotal    int                        `json:"related_total"`
	RelatedPage     int                        `json:"related_page"`
	RelatedPageSize int                        `json:"related_page_size"`
	RelatedHasMore  bool                       `json:"related_has_more"`
	Timeline        []UserSecurityTimelineItem `json:"timeline"`
	Total           int                        `json:"total"`
	Page            int                        `json:"page"`
	PageSize        int                        `json:"page_size"`
	Range           string                     `json:"range"`
	StartTimestamp  int64                      `json:"start_timestamp"`
	EndTimestamp    int64                      `json:"end_timestamp"`
	Sync            IPAuditSyncStatus          `json:"sync"`
}

type securityLogCountRow struct {
	Type    int    `gorm:"column:type"`
	Content string `gorm:"column:content"`
	Other   string `gorm:"column:other"`
	Count   int64  `gorm:"column:count"`
}

type securityRelatedLogRow struct {
	UserID   int    `gorm:"column:user_id"`
	Username string `gorm:"column:username"`
	IP       string `gorm:"column:ip"`
}

func GetGlobalIPAudit(ctx context.Context, actorID int, query IPAuditQuery) (IPAuditPage, error) {
	actor, err := model.GetUserById(actorID, false)
	if err != nil {
		return IPAuditPage{}, err
	}
	if actor.Role < common.RoleAdminUser {
		return IPAuditPage{}, ErrIPAuditForbidden
	}

	query, err = normalizeIPAuditQuery(query, time.Now())
	if err != nil {
		return IPAuditPage{}, err
	}
	settings := ip_security_setting.GetSettings()
	page := IPAuditPage{
		Enabled: settings.Enabled, Items: []model.IPAuditItem{}, Page: query.Page, PageSize: query.PageSize,
		Range: query.Range, View: query.View, StartTimestamp: query.StartTimestamp, EndTimestamp: query.EndTimestamp,
		IsRoot: actor.Role >= common.RoleRootUser, CanViewDetails: actor.Role >= common.RoleAdminUser,
		ProxyWarning: trustedProxyAuditWarning(os.Getenv("TRUSTED_PROXIES")),
		Sync:         IPAuditSyncStatus{Status: "disabled", Ready: true},
	}
	if !settings.Enabled {
		return page, nil
	}
	page.Sync, err = getIPAuditSyncStatus(ctx)
	if err != nil {
		return IPAuditPage{}, err
	}
	var items []model.IPAuditItem
	var total int64
	err = retryIPAuditRead(ctx, func() error {
		var queryErr error
		items, total, queryErr = model.QueryIPAuditProjectionForView(
			ctx,
			actor.Id,
			actor.Role,
			query.StartTimestamp,
			query.EndTimestamp,
			query.Page,
			query.PageSize,
			query.View,
		)
		return queryErr
	})
	if err != nil {
		return IPAuditPage{}, err
	}
	maxInt := int64(^uint(0) >> 1)
	if total > maxInt {
		return IPAuditPage{}, errors.New("IP audit total exceeds platform integer range")
	}
	page.Enabled = true
	page.Items = items
	for index := range page.Items {
		page.Items[index].CanQueryExternalProfile = canQueryExternalIPProfile(page.Items[index].IP, page.IsRoot)
	}
	page.Total = int(total)
	return page, nil
}

func canQueryExternalIPProfile(ip string, canViewFullIP bool) bool {
	if !canViewFullIP {
		return false
	}
	address, err := netip.ParseAddr(strings.TrimSpace(ip))
	if err != nil {
		return false
	}
	address = address.Unmap()
	if !address.IsGlobalUnicast() || address.IsPrivate() || address.IsLoopback() || address.IsLinkLocalUnicast() {
		return false
	}
	for _, prefix := range nonPublicInternetIPPrefixes {
		if prefix.Contains(address) {
			return false
		}
	}
	return true
}

func GetGlobalIPAuditDetail(ctx context.Context, actorID int, ip string, query IPAuditQuery) (IPAuditDetail, error) {
	actor, err := model.GetUserById(actorID, false)
	if err != nil {
		return IPAuditDetail{}, err
	}
	if actor.Role < common.RoleAdminUser {
		return IPAuditDetail{}, ErrIPAuditForbidden
	}
	ip = strings.TrimSpace(ip)
	if ip == "" || len(ip) > 64 {
		return IPAuditDetail{}, ErrIPAuditInvalidIP
	}
	query, err = normalizeIPAuditQuery(query, time.Now())
	if err != nil {
		return IPAuditDetail{}, err
	}
	settings := ip_security_setting.GetSettings()
	detail := IPAuditDetail{
		Enabled: settings.Enabled,
		IPAuditProjectionDetail: model.IPAuditProjectionDetail{
			IP: ip, Users: []model.IPAuditUser{}, Timeline: []model.IPAuditTimelineItem{},
		},
		Range: query.Range, StartTimestamp: query.StartTimestamp, EndTimestamp: query.EndTimestamp,
		Sync: IPAuditSyncStatus{Status: "disabled", Ready: true},
	}
	if !settings.Enabled {
		return detail, nil
	}
	detail.Sync, err = getIPAuditSyncStatus(ctx)
	if err != nil {
		return IPAuditDetail{}, err
	}
	var projection model.IPAuditProjectionDetail
	err = retryIPAuditRead(ctx, func() error {
		var queryErr error
		projection, queryErr = model.QueryIPAuditProjectionDetailPage(
			ctx,
			actor.Id,
			actor.Role,
			query.StartTimestamp,
			query.EndTimestamp,
			ip,
			query.DetailUserPage,
			query.DetailUserPageSize,
			query.TimelinePage,
			query.TimelinePageSize,
		)
		return queryErr
	})
	if err != nil {
		return IPAuditDetail{}, err
	}
	detail.Enabled = true
	detail.IPAuditProjectionDetail = projection
	return detail, nil
}

func excludedIPAuditUserIDs(ctx context.Context, actor *model.User) ([]int, error) {
	if actor.Id == 1 && actor.Role == common.RoleRootUser {
		return nil, nil
	}
	var privilegedUserIDs []int
	if err := model.DB.WithContext(ctx).Unscoped().Model(&model.User{}).
		Where("role >= ?", actor.Role).
		Pluck("id", &privilegedUserIDs).Error; err != nil {
		return nil, err
	}
	logUserIDs, err := model.GetIPAuditLogUserIDs(ctx)
	if err != nil {
		return nil, err
	}
	existing := make(map[int]struct{}, len(logUserIDs))
	for start := 0; start < len(logUserIDs); start += 500 {
		end := min(start+500, len(logUserIDs))
		var batch []int
		if err := model.DB.WithContext(ctx).Unscoped().Model(&model.User{}).
			Where("id IN ?", logUserIDs[start:end]).Pluck("id", &batch).Error; err != nil {
			return nil, err
		}
		for _, userID := range batch {
			existing[userID] = struct{}{}
		}
	}
	excluded := make(map[int]struct{}, len(privilegedUserIDs)+1)
	excluded[0] = struct{}{}
	for _, userID := range privilegedUserIDs {
		excluded[userID] = struct{}{}
	}
	for _, userID := range logUserIDs {
		if _, ok := existing[userID]; !ok {
			excluded[userID] = struct{}{}
		}
	}
	userIDs := make([]int, 0, len(excluded))
	for userID := range excluded {
		userIDs = append(userIDs, userID)
	}
	sort.Ints(userIDs)
	return userIDs, nil
}

func GetUserSecurityAudit(ctx context.Context, actorID int, targetID int, query IPAuditQuery) (UserSecurityAudit, error) {
	actor, err := model.GetUserById(actorID, false)
	if err != nil {
		return UserSecurityAudit{}, err
	}
	target, err := model.GetUserById(targetID, false)
	if err != nil {
		return UserSecurityAudit{}, err
	}
	if !CanManageUser(
		UserIdentity{ID: actor.Id, Role: actor.Role},
		UserIdentity{ID: target.Id, Role: target.Role},
	) {
		return UserSecurityAudit{}, ErrIPAuditForbidden
	}
	query, err = normalizeIPAuditQuery(query, time.Now())
	if err != nil {
		return UserSecurityAudit{}, err
	}
	settings := ip_security_setting.GetSettings()
	isRoot := actor.Role >= common.RoleRootUser
	result := UserSecurityAudit{
		Enabled: settings.Enabled, TargetUserID: target.Id, Username: target.Username,
		Projection: "masked", Page: query.Page, PageSize: query.PageSize,
		Range: query.Range, StartTimestamp: query.StartTimestamp, EndTimestamp: query.EndTimestamp,
		IsRoot: isRoot, ProxyWarning: trustedProxyAuditWarning(os.Getenv("TRUSTED_PROXIES")),
		RelatedUsers: []UserSecurityRelatedUser{}, Timeline: []UserSecurityTimelineItem{},
		Sync: IPAuditSyncStatus{Status: "disabled", Ready: true},
	}
	if isRoot {
		result.Projection = "full"
	}
	if !settings.Enabled {
		return result, nil
	}
	result.Sync, err = getIPAuditSyncStatus(ctx)
	if err != nil {
		return UserSecurityAudit{}, err
	}

	result.Enabled = true
	result, err = retryUserSecurityProjection(ctx, result, func(attempt *UserSecurityAudit) error {
		return queryUserSecurityProjection(ctx, actor, target, query, attempt)
	})
	if err != nil {
		return UserSecurityAudit{}, err
	}
	return result, nil
}

// retryIPAuditRead absorbs short-lived database connection/lock failures on
// read-only audit queries while preserving cancellation and validation errors.
func retryIPAuditRead(ctx context.Context, operation func() error) error {
	if ctx == nil {
		ctx = context.Background()
	}
	var err error
	for attempt := 0; attempt < ipAuditReadAttempts; attempt++ {
		if err = ctx.Err(); err != nil {
			return err
		}
		err = operation()
		if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || attempt == ipAuditReadAttempts-1 {
			return err
		}

		delay := ipAuditReadRetryBaseDelay * time.Duration(1<<attempt)
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return ctx.Err()
		case <-timer.C:
		}
	}
	return err
}

// retryUserSecurityProjection keeps each attempt isolated and commits only a
// fully successful projection to the returned audit result.
func retryUserSecurityProjection(
	ctx context.Context,
	base UserSecurityAudit,
	operation func(*UserSecurityAudit) error,
) (UserSecurityAudit, error) {
	var committed UserSecurityAudit
	err := retryIPAuditRead(ctx, func() error {
		attempt := base
		attempt.Summary = UserSecurityAuditSummary{}
		attempt.RelatedUsers = make([]UserSecurityRelatedUser, 0)
		attempt.Timeline = make([]UserSecurityTimelineItem, 0)
		if err := operation(&attempt); err != nil {
			return err
		}
		committed = attempt
		return nil
	})
	return committed, err
}

func getIPAuditSyncStatus(ctx context.Context) (IPAuditSyncStatus, error) {
	state, ready, err := model.GetIPSecurityProjectionProgress(ctx)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return IPAuditSyncStatus{}, ctxErr
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return IPAuditSyncStatus{}, err
		}
		logger.LogWarn(ctx, "IP audit sync status unavailable; serving available projection: "+err.Error())
		return IPAuditSyncStatus{Status: "unknown", Ready: false}, nil
	}
	return IPAuditSyncStatus{
		Status: state.Status, Ready: ready, ProcessedLogs: state.ProcessedLogs,
		ProcessedUsers: state.ProcessedUsers, StartedAt: state.StartedAt, UpdatedAt: state.UpdatedAt,
	}, nil
}

func populateUserSecuritySummary(ctx context.Context, target *model.User, query IPAuditQuery, result *UserSecurityAudit) error {
	if timestampInIPAuditRange(target.CreatedAt, query) {
		result.Summary.RegistrationIP = model.NormalizeAuditIP(target.RegistrationIP)
	}
	var counts []securityLogCountRow
	countQuery := applyIPAuditRange(securityLogReadDB(ctx), query).
		Select("type, content, other, COUNT(*) AS count").
		Where("user_id = ?", target.Id).
		Group("type, content, other")
	if err := countQuery.Scan(&counts).Error; err != nil {
		return err
	}
	for _, row := range counts {
		switch row.Type {
		case model.LogTypeConsume:
			result.Summary.APIRequestCount += int(row.Count)
		case model.LogTypeError:
			log := model.Log{Type: row.Type, Other: row.Other}
			if log.IPSecurityHTTPStatus() == http.StatusUnauthorized {
				result.Summary.ErrorCount += int(row.Count)
				result.Summary.UnauthorizedCount += int(row.Count)
			}
		case model.LogTypeLogin:
			switch row.Content {
			case "user.login_failed":
				result.Summary.LoginFailureCount += int(row.Count)
			case "user.logout":
				result.Summary.LogoutCount += int(row.Count)
			case "user.register":
				// Registration is sourced from users.created_at to avoid double counting outbox delivery.
			default:
				result.Summary.LoginSuccessCount += int(row.Count)
			}
		}
	}
	result.Summary.AnomalyCount = result.Summary.ErrorCount + result.Summary.LoginFailureCount

	var recent model.Log
	recentQuery := applyIPAuditRange(securityLogReadDB(ctx), query).
		Where("user_id = ? AND ip <> ? AND content <> ?", target.Id, "", "user.register").
		Order(securityLogOrder("logs.")).
		Limit(1).
		Find(&recent)
	if recentQuery.Error != nil {
		return recentQuery.Error
	}
	if recentQuery.RowsAffected > 0 {
		result.Summary.RecentIP = model.NormalizeAuditIP(recent.Ip)
		result.Summary.LastActivityAt = recent.CreatedAt
	} else if result.Summary.RegistrationIP != "" {
		result.Summary.RecentIP = result.Summary.RegistrationIP
		result.Summary.LastActivityAt = target.CreatedAt
	}
	return nil
}

func populateUserSecurityRelations(ctx context.Context, targetID int, query IPAuditQuery, excludedUserIDs []int, result *UserSecurityAudit) error {
	var targetIPs []string
	if result.Summary.RegistrationIP != "" {
		targetIPs = append(targetIPs, result.Summary.RegistrationIP)
	}
	var logIPs []string
	if err := applyIPAuditRange(securityLogReadDB(ctx), query).
		Distinct("ip").
		Where("user_id = ? AND ip <> ?", targetID, "").
		Pluck("ip", &logIPs).Error; err != nil {
		return err
	}
	targetIPs = append(targetIPs, logIPs...)
	canonicalSet := make(map[string]struct{})
	rawSet := make(map[string]struct{})
	for _, ip := range targetIPs {
		raw := strings.TrimSpace(ip)
		canonical := model.NormalizeAuditIP(raw)
		if raw != "" {
			rawSet[raw] = struct{}{}
		}
		if canonical != "" {
			canonicalSet[canonical] = struct{}{}
			rawSet[canonical] = struct{}{}
		}
	}
	if len(rawSet) == 0 {
		return nil
	}
	rawValues := make([]string, 0, len(rawSet))
	for ip := range rawSet {
		rawValues = append(rawValues, ip)
	}

	related := make(map[int]UserSecurityRelatedUser)
	var logRows []securityRelatedLogRow
	logQuery := applyIPAuditRange(securityLogReadDB(ctx), query).
		Select("DISTINCT user_id, username, ip").
		Where("user_id <> ? AND ip IN ?", targetID, rawValues)
	if len(excludedUserIDs) > 0 {
		logQuery = applyExcludedIPAuditUsers(logQuery, "user_id", excludedUserIDs)
	}
	if err := logQuery.Scan(&logRows).Error; err != nil {
		return err
	}
	for _, row := range logRows {
		if _, ok := canonicalSet[model.NormalizeAuditIP(row.IP)]; ok && row.UserID > 0 {
			related[row.UserID] = UserSecurityRelatedUser{UserID: row.UserID, Username: row.Username}
		}
	}
	var registrationUsers []model.User
	registrationQuery := model.DB.WithContext(ctx).
		Select("id", "username", "registration_ip").
		Where("id <> ? AND registration_ip IN ?", targetID, rawValues)
	registrationQuery = applyIPAuditRange(registrationQuery, query)
	if len(excludedUserIDs) > 0 {
		registrationQuery = applyExcludedIPAuditUsers(registrationQuery, "id", excludedUserIDs)
	}
	if err := registrationQuery.Find(&registrationUsers).Error; err != nil {
		return err
	}
	for _, user := range registrationUsers {
		if _, ok := canonicalSet[model.NormalizeAuditIP(user.RegistrationIP)]; ok {
			related[user.Id] = UserSecurityRelatedUser{UserID: user.Id, Username: user.Username}
		}
	}
	result.Summary.RelatedUserCount = len(related)
	if !result.IsRoot {
		return nil
	}
	for _, user := range related {
		result.RelatedUsers = append(result.RelatedUsers, user)
	}
	sort.Slice(result.RelatedUsers, func(i, j int) bool {
		if result.RelatedUsers[i].Username != result.RelatedUsers[j].Username {
			return result.RelatedUsers[i].Username < result.RelatedUsers[j].Username
		}
		return result.RelatedUsers[i].UserID < result.RelatedUsers[j].UserID
	})
	return nil
}

func applyExcludedIPAuditUsers(tx *gorm.DB, column string, userIDs []int) *gorm.DB {
	for start := 0; start < len(userIDs); start += 500 {
		end := min(start+500, len(userIDs))
		tx = tx.Where(column+" NOT IN ?", userIDs[start:end])
	}
	return tx
}

func populateUserSecurityTimeline(ctx context.Context, target *model.User, query IPAuditQuery, result *UserSecurityAudit) error {
	base := applyIPAuditRange(securityLogReadDB(ctx), query).
		Where("user_id = ? AND content <> ?", target.Id, "user.register")
	var logCount int64
	if err := base.Count(&logCount).Error; err != nil {
		return err
	}
	registrationIncluded := target.RegistrationIP != "" && timestampInIPAuditRange(target.CreatedAt, query)
	result.Total = int(logCount)
	registrationIndex := -1
	if registrationIncluded {
		result.Total++
		var newerLogs int64
		if err := base.Where("created_at >= ?", target.CreatedAt).Count(&newerLogs).Error; err != nil {
			return err
		}
		registrationIndex = int(newerLogs)
	}

	offset := (query.Page - 1) * query.PageSize
	end := offset + query.PageSize
	logOffset := offset
	logLimit := query.PageSize
	includeRegistration := registrationIndex >= offset && registrationIndex < end
	if registrationIndex >= 0 && registrationIndex < offset {
		logOffset--
	}
	if includeRegistration {
		logLimit--
	}
	if logOffset < 0 {
		logOffset = 0
	}

	var logs []model.Log
	if logLimit > 0 {
		if err := base.Order(securityLogOrder("logs.")).Offset(logOffset).Limit(logLimit).Find(&logs).Error; err != nil {
			return err
		}
	}
	for _, log := range logs {
		result.Timeline = append(result.Timeline, UserSecurityTimelineItem{
			CreatedAt: log.CreatedAt, IP: model.NormalizeAuditIP(log.Ip),
			Kind: securityTimelineKind(log), RequestID: log.RequestId,
		})
	}
	if includeRegistration {
		index := registrationIndex - offset
		registration := UserSecurityTimelineItem{
			CreatedAt: target.CreatedAt, IP: model.NormalizeAuditIP(target.RegistrationIP), Kind: "registration",
		}
		result.Timeline = append(result.Timeline, UserSecurityTimelineItem{})
		copy(result.Timeline[index+1:], result.Timeline[index:])
		result.Timeline[index] = registration
	}
	return nil
}

func applyIPAuditRange(tx *gorm.DB, query IPAuditQuery) *gorm.DB {
	if query.StartTimestamp > 0 {
		tx = tx.Where("created_at >= ?", query.StartTimestamp)
	}
	if query.EndTimestamp > 0 {
		tx = tx.Where("created_at <= ?", query.EndTimestamp)
	}
	return tx
}

func securityLogReadDB(ctx context.Context) *gorm.DB {
	return model.LogReadDB().WithContext(ctx)
}

func securityLogOrder(prefix string) string {
	return model.LogReadOrder(prefix)
}

func timestampInIPAuditRange(timestamp int64, query IPAuditQuery) bool {
	if timestamp <= 0 {
		return false
	}
	if query.StartTimestamp > 0 && timestamp < query.StartTimestamp {
		return false
	}
	return query.EndTimestamp <= 0 || timestamp <= query.EndTimestamp
}

func securityTimelineKind(log model.Log) string {
	switch log.Type {
	case model.LogTypeConsume:
		return "api_request"
	case model.LogTypeError:
		return "api_error"
	case model.LogTypeLogin:
		if log.Content == "user.login_failed" {
			return "login_failure"
		}
		if log.Content == "user.logout" {
			return "logout"
		}
		if log.Content == "user.register" {
			return "registration"
		}
		return "login_success"
	default:
		return "activity_" + strconv.Itoa(log.Type)
	}
}

func normalizeIPAuditQuery(query IPAuditQuery, now time.Time) (IPAuditQuery, error) {
	if query.Page < 1 {
		query.Page = 1
	}
	if query.PageSize < 1 {
		query.PageSize = common.ItemsPerPage
	}
	if query.PageSize > 100 {
		query.PageSize = 100
	}
	if query.RelatedPage < 1 {
		query.RelatedPage = 1
	}
	if query.RelatedPageSize < 1 {
		query.RelatedPageSize = query.PageSize
	}
	if query.RelatedPageSize > 100 {
		query.RelatedPageSize = 100
	}
	if query.DetailUserPage < 1 {
		query.DetailUserPage = 1
	}
	if query.DetailUserPageSize < 1 {
		query.DetailUserPageSize = 20
	}
	if query.DetailUserPageSize > 100 {
		query.DetailUserPageSize = 100
	}
	if query.TimelinePage < 1 {
		query.TimelinePage = 1
	}
	if query.TimelinePageSize < 1 {
		query.TimelinePageSize = 20
	}
	if query.TimelinePageSize > 100 {
		query.TimelinePageSize = 100
	}
	query.View = strings.ToLower(strings.TrimSpace(query.View))
	if query.View == "" {
		query.View = IPAuditViewAll
	}
	switch query.View {
	case IPAuditViewRisk, IPAuditViewSharedAPI, IPAuditViewSharedRegistration, IPAuditViewAuthentication, IPAuditViewAll:
	default:
		return IPAuditQuery{}, ErrIPAuditInvalidView
	}

	query.Range = strings.ToLower(strings.TrimSpace(query.Range))
	if query.Range == "" && (query.StartTimestamp != 0 || query.EndTimestamp != 0) {
		query.Range = IPAuditRangeCustom
	}
	if query.Range == "" {
		query.Range = IPAuditRange30Days
	}

	todayUTC := now.UTC().Truncate(24 * time.Hour)
	endOfTodayUTC := todayUTC.Add(24*time.Hour - time.Second).Unix()
	switch query.Range {
	case IPAuditRange7Days:
		query.StartTimestamp = todayUTC.AddDate(0, 0, -6).Unix()
		query.EndTimestamp = endOfTodayUTC
	case IPAuditRange30Days:
		query.StartTimestamp = todayUTC.AddDate(0, 0, -29).Unix()
		query.EndTimestamp = endOfTodayUTC
	case IPAuditRange90Days:
		query.StartTimestamp = todayUTC.AddDate(0, 0, -89).Unix()
		query.EndTimestamp = endOfTodayUTC
	case IPAuditRangeAll:
		query.StartTimestamp = 0
		query.EndTimestamp = 0
	case IPAuditRangeCustom:
		if query.StartTimestamp <= 0 || query.EndTimestamp <= 0 || query.StartTimestamp > query.EndTimestamp {
			return IPAuditQuery{}, ErrIPAuditInvalidRange
		}
	default:
		return IPAuditQuery{}, ErrIPAuditInvalidRange
	}
	return query, nil
}

func trustedProxyAuditWarning(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return TrustedProxiesNotConfigured
	}
	for _, value := range strings.Split(raw, ",") {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		prefix, err := netip.ParsePrefix(value)
		if err != nil {
			if value == "*" {
				return TrustedProxiesUnsafe
			}
			continue
		}
		if prefix.Bits() == 0 {
			return TrustedProxiesUnsafe
		}
	}
	return ""
}
