package model

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func applyExplicitLogTextFilter(tx *gorm.DB, column string, value string) (*gorm.DB, error) {
	if value == "" {
		return tx, nil
	}
	if strings.Contains(value, "%") {
		condition, pattern, err := buildLogLikeCondition(column, value)
		if err != nil {
			return nil, err
		}
		return tx.Where(condition, pattern), nil
	}
	return tx.Where(column+" = ?", value), nil
}

func buildLogLikeCondition(column string, value string) (string, string, error) {
	if common.UsingLogDatabase(common.DatabaseTypeClickHouse) {
		pattern, err := sanitizeClickHouseLikePattern(value)
		if err != nil {
			return "", "", err
		}
		return column + " LIKE ?", pattern, nil
	}

	pattern, err := sanitizeLikePattern(value)
	if err != nil {
		return "", "", err
	}
	return column + " LIKE ? ESCAPE '!'", pattern, nil
}

func sanitizeClickHouseLikePattern(input string) (string, error) {
	input = strings.ReplaceAll(input, `\`, `\\`)
	input = strings.ReplaceAll(input, `_`, `\_`)

	if err := validateLikePattern(input); err != nil {
		return "", err
	}
	return input, nil
}

type Log struct {
	Id                 int     `json:"id" gorm:"index:idx_created_at_id,priority:2;index:idx_logs_created_at_id,priority:2;index:idx_user_id_id,priority:2"`
	EventId            *string `json:"event_id,omitempty" gorm:"type:varchar(191);uniqueIndex;default:null"`
	UserId             int     `json:"user_id" gorm:"index;index:idx_user_id_id,priority:1"`
	CreatedAt          int64   `json:"created_at" gorm:"bigint;index:idx_created_at_id,priority:1;index:idx_logs_created_at_id,priority:1;index:idx_created_at_type;index:idx_logs_stats_covering,priority:2"`
	Type               int     `json:"type" gorm:"index:idx_created_at_type;index:idx_logs_stats_covering,priority:1"`
	Content            string  `json:"content"`
	Username           string  `json:"username" gorm:"index;index:index_username_model_name,priority:2;default:''"`
	TokenName          string  `json:"token_name" gorm:"index;default:''"`
	ModelName          string  `json:"model_name" gorm:"index;index:index_username_model_name,priority:1;default:''"`
	Quota              int     `json:"quota" gorm:"default:0;index:idx_logs_stats_covering,priority:3"`
	PromptTokens       int     `json:"prompt_tokens" gorm:"default:0;index:idx_logs_stats_covering,priority:4"`
	CompletionTokens   int     `json:"completion_tokens" gorm:"default:0;index:idx_logs_stats_covering,priority:5"`
	UseTime            int     `json:"use_time" gorm:"default:0"`
	IsStream           bool    `json:"is_stream"`
	ChannelId          int     `json:"channel" gorm:"index"`
	ChannelName        string  `json:"channel_name" gorm:"->"`
	TokenId            int     `json:"token_id" gorm:"default:0;index"`
	Group              string  `json:"group" gorm:"index"`
	Ip                 string  `json:"ip" gorm:"index;default:''"`
	RequestId          string  `json:"request_id,omitempty" gorm:"type:varchar(64);index:idx_logs_request_id;default:''"`
	UpstreamRequestId  string  `json:"upstream_request_id,omitempty" gorm:"type:varchar(128);index:idx_logs_upstream_request_id;default:''"`
	Other              string  `json:"other"`
	AuditEventIdentity string  `json:"-" gorm:"column:audit_event_identity;->;-:migration"`
}

func (log *Log) IPSecurityHTTPStatus() int {
	if log == nil || log.Type != LogTypeError || strings.TrimSpace(log.Other) == "" {
		return 0
	}
	var metadata struct {
		StatusCode int `json:"status_code"`
	}
	if err := common.UnmarshalJsonStr(log.Other, &metadata); err != nil {
		return 0
	}
	if metadata.StatusCode < 100 || metadata.StatusCode > 599 {
		return 0
	}
	return metadata.StatusCode
}

type IPAuditUser struct {
	UserId   int    `json:"user_id" gorm:"column:user_id"`
	Username string `json:"username" gorm:"column:username"`
}

type IPAuditTimelineItem struct {
	Id        int    `json:"-" gorm:"column:id"`
	CreatedAt int64  `json:"created_at" gorm:"column:created_at"`
	UserId    int    `json:"user_id" gorm:"column:user_id"`
	Username  string `json:"username" gorm:"column:username"`
	Type      int    `json:"type" gorm:"column:type"`
	Content   string `json:"-" gorm:"column:content"`
	Kind      string `json:"kind" gorm:"-"`
	RequestId string `json:"request_id,omitempty" gorm:"column:request_id"`
}

type IPAuditItem struct {
	IP                      string                `json:"ip"`
	ProxyArtifact           bool                  `json:"proxy_artifact"`
	CanQueryExternalProfile bool                  `json:"can_query_external_profile"`
	RiskReasons             []string              `json:"risk_reasons"`
	RiskLevel               int                   `json:"risk_level"`
	RequestCount            int                   `json:"request_count"`
	RegistrationCount       int                   `json:"registration_count"`
	APIRequestCount         int                   `json:"api_request_count"`
	ErrorCount              int                   `json:"error_count"`
	UnauthorizedCount       int                   `json:"unauthorized_count"`
	LoginSuccessCount       int                   `json:"login_success_count"`
	LoginFailureCount       int                   `json:"login_failure_count"`
	LogoutCount             int                   `json:"logout_count"`
	AnomalyCount            int                   `json:"anomaly_count"`
	UserCount               int                   `json:"user_count"`
	RegistrationUserCount   int                   `json:"registration_user_count"`
	APIUserCount            int                   `json:"api_user_count"`
	AuthenticationUserCount int                   `json:"authentication_user_count"`
	Quota                   int                   `json:"quota"`
	FirstSeenAt             int64                 `json:"first_seen_at"`
	LastSeenAt              int64                 `json:"last_seen_at"`
	Users                   []IPAuditUser         `json:"users"`
	Timeline                []IPAuditTimelineItem `json:"timeline"`
}

type IPAuditProjectionDetail struct {
	IP               string                `json:"ip"`
	Users            []IPAuditUser         `json:"users"`
	UserTotal        int                   `json:"user_total"`
	UserPage         int                   `json:"user_page"`
	UserPageSize     int                   `json:"user_page_size"`
	UserHasMore      bool                  `json:"user_has_more"`
	Timeline         []IPAuditTimelineItem `json:"timeline"`
	TimelineTotal    int                   `json:"timeline_total"`
	TimelinePage     int                   `json:"timeline_page"`
	TimelinePageSize int                   `json:"timeline_page_size"`
	TimelineHasMore  bool                  `json:"timeline_has_more"`
}

type ipAuditAggregateRow struct {
	IP                string `gorm:"column:ip"`
	RequestCount      int64  `gorm:"column:request_count"`
	APIRequestCount   int64  `gorm:"column:api_request_count"`
	ErrorCount        int64  `gorm:"column:error_count"`
	LoginSuccessCount int64  `gorm:"column:login_success_count"`
	LoginFailureCount int64  `gorm:"column:login_failure_count"`
	LogoutCount       int64  `gorm:"column:logout_count"`
	UserCount         int64  `gorm:"column:user_count"`
	Quota             int64  `gorm:"column:quota"`
	FirstSeenAt       int64  `gorm:"column:first_seen_at"`
	LastSeenAt        int64  `gorm:"column:last_seen_at"`
}

type ipAuditRegistrationRow struct {
	UserID    int    `gorm:"column:user_id"`
	Username  string `gorm:"column:username"`
	IP        string `gorm:"column:ip"`
	CreatedAt int64  `gorm:"column:created_at"`
}

func NormalizeLogIP(ip string) string {
	trimmed := strings.TrimSpace(ip)
	if trimmed == "" {
		return ""
	}
	parsed := net.ParseIP(trimmed)
	if parsed == nil {
		if host, _, err := net.SplitHostPort(trimmed); err == nil {
			parsed = net.ParseIP(host)
			if parsed == nil {
				return strings.TrimSpace(host)
			}
		} else {
			return trimmed
		}
	}
	if parsed.IsLoopback() {
		if parsed.To4() != nil || parsed.Equal(net.IPv6loopback) {
			return "127.0.0.1"
		}
	}
	if ipv4 := parsed.To4(); ipv4 != nil {
		return ipv4.String()
	}
	return parsed.String()
}

func GetIPAuditItems(role int, startTimestamp int64, endTimestamp int64, limit int) ([]IPAuditItem, error) {
	items, _, err := GetIPAuditPage(role, startTimestamp, endTimestamp, 1, limit)
	return items, err
}

func GetIPAuditLogUserIDs(ctx context.Context) ([]int, error) {
	var userIDs []int
	err := logReadDB().WithContext(ctx).Where("user_id > ?", 0).Distinct("user_id").Pluck("user_id", &userIDs).Error
	return userIDs, err
}

func GetIPAuditPage(role int, startTimestamp int64, endTimestamp int64, page int, pageSize int) ([]IPAuditItem, int, error) {
	return GetScopedIPAuditPage(role, startTimestamp, endTimestamp, page, pageSize, nil)
}

func GetScopedIPAuditPage(role int, startTimestamp int64, endTimestamp int64, page int, pageSize int, excludedUserIDs []int) ([]IPAuditItem, int, error) {
	if role < common.RoleAdminUser {
		return nil, 0, errors.New("无权查看 IP 审计")
	}
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 50
	}
	if pageSize > 100 {
		pageSize = 100
	}

	tx := applyIPAuditUserExclusions(logReadDB(), excludedUserIDs).Select(
		"ip, COUNT(*) AS request_count, "+
			"COALESCE(SUM(CASE WHEN type = ? THEN 1 ELSE 0 END), 0) AS api_request_count, "+
			"COALESCE(SUM(CASE WHEN type = ? THEN 1 ELSE 0 END), 0) AS error_count, "+
			"COALESCE(SUM(CASE WHEN type = ? AND content NOT IN (?, ?, ?) THEN 1 ELSE 0 END), 0) AS login_success_count, "+
			"COALESCE(SUM(CASE WHEN type = ? AND content = ? THEN 1 ELSE 0 END), 0) AS login_failure_count, "+
			"COALESCE(SUM(CASE WHEN type = ? AND content = ? THEN 1 ELSE 0 END), 0) AS logout_count, "+
			"COUNT(DISTINCT user_id) AS user_count, COALESCE(SUM(quota), 0) AS quota, "+
			"MIN(created_at) AS first_seen_at, MAX(created_at) AS last_seen_at",
		LogTypeConsume,
		LogTypeError,
		LogTypeLogin, "user.login_failed", "user.register", "user.logout",
		LogTypeLogin, "user.login_failed",
		LogTypeLogin, "user.logout",
	).Where("ip <> ? AND content <> ?", "", "user.register").Group("ip")
	if startTimestamp != 0 {
		tx = tx.Where("created_at >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		tx = tx.Where("created_at <= ?", endTimestamp)
	}

	var rows []ipAuditAggregateRow
	if err := tx.Scan(&rows).Error; err != nil {
		return nil, 0, err
	}
	registrations, err := getIPAuditRegistrationRows(startTimestamp, endTimestamp, excludedUserIDs)
	if err != nil {
		return nil, 0, err
	}

	items, variants := buildIPAuditItems(rows)
	items = mergeIPAuditRegistrationRows(items, registrations)
	if role < common.RoleRootUser {
		items = mergeMaskedIPAuditItems(items)
		if err := populateMaskedIPAuditUserCounts(items, registrations, startTimestamp, endTimestamp, excludedUserIDs); err != nil {
			return nil, 0, err
		}
	}
	sortIPAuditItems(items)
	total := len(items)
	start := (page - 1) * pageSize
	if start >= total {
		return []IPAuditItem{}, total, nil
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	items = items[start:end]
	if role >= common.RoleRootUser {
		if err := populateRootIPAuditDetails(items, variants, registrations, startTimestamp, endTimestamp, excludedUserIDs); err != nil {
			return nil, 0, err
		}
	}
	return items, total, nil
}

func getIPAuditRegistrationRows(startTimestamp int64, endTimestamp int64, excludedUserIDs []int) ([]ipAuditRegistrationRow, error) {
	tx := DB.Model(&User{}).
		Select("id AS user_id, username, registration_ip AS ip, created_at").
		Where("registration_ip <> ?", "")
	for start := 0; start < len(excludedUserIDs); start += 500 {
		end := min(start+500, len(excludedUserIDs))
		tx = tx.Where("id NOT IN ?", excludedUserIDs[start:end])
	}
	tx = applyIPAuditTimeRange(tx, startTimestamp, endTimestamp)
	var rows []ipAuditRegistrationRow
	err := tx.Scan(&rows).Error
	return rows, err
}

func buildIPAuditItems(rows []ipAuditAggregateRow) ([]IPAuditItem, map[string][]string) {
	itemsByIP := make(map[string]*IPAuditItem)
	variantSets := make(map[string]map[string]bool)
	for _, row := range rows {
		normalized := NormalizeAuditIP(row.IP)
		if normalized == "" {
			continue
		}
		item := itemsByIP[normalized]
		if item == nil {
			item = &IPAuditItem{IP: normalized, FirstSeenAt: row.FirstSeenAt, LastSeenAt: row.LastSeenAt}
			itemsByIP[normalized] = item
		}
		item.RequestCount += int(row.RequestCount)
		item.APIRequestCount += int(row.APIRequestCount)
		item.ErrorCount += int(row.ErrorCount)
		item.LoginSuccessCount += int(row.LoginSuccessCount)
		item.LoginFailureCount += int(row.LoginFailureCount)
		item.LogoutCount += int(row.LogoutCount)
		item.AnomalyCount += int(row.ErrorCount + row.LoginFailureCount)
		item.UserCount += int(row.UserCount)
		item.Quota = common.QuotaFromFloat(float64(item.Quota) + float64(row.Quota))
		if item.FirstSeenAt == 0 || row.FirstSeenAt < item.FirstSeenAt {
			item.FirstSeenAt = row.FirstSeenAt
		}
		if row.LastSeenAt > item.LastSeenAt {
			item.LastSeenAt = row.LastSeenAt
		}
		if variantSets[normalized] == nil {
			variantSets[normalized] = map[string]bool{}
		}
		variantSets[normalized][row.IP] = true
	}

	items := make([]IPAuditItem, 0, len(itemsByIP))
	for _, item := range itemsByIP {
		items = append(items, *item)
	}
	variants := make(map[string][]string, len(variantSets))
	for ip, set := range variantSets {
		for variant := range set {
			variants[ip] = append(variants[ip], variant)
		}
		sort.Strings(variants[ip])
	}
	return items, variants
}

func mergeIPAuditRegistrationRows(items []IPAuditItem, rows []ipAuditRegistrationRow) []IPAuditItem {
	indexByIP := make(map[string]int, len(items))
	for i := range items {
		indexByIP[items[i].IP] = i
	}
	for _, row := range rows {
		normalized := NormalizeAuditIP(row.IP)
		if normalized == "" {
			continue
		}
		index, exists := indexByIP[normalized]
		if !exists {
			items = append(items, IPAuditItem{IP: normalized})
			index = len(items) - 1
			indexByIP[normalized] = index
		}
		item := &items[index]
		item.RegistrationCount++
		item.UserCount++
		if item.FirstSeenAt == 0 || row.CreatedAt < item.FirstSeenAt {
			item.FirstSeenAt = row.CreatedAt
		}
		if row.CreatedAt > item.LastSeenAt {
			item.LastSeenAt = row.CreatedAt
		}
	}
	return items
}

func mergeMaskedIPAuditItems(items []IPAuditItem) []IPAuditItem {
	merged := make(map[string]*IPAuditItem)
	for _, item := range items {
		maskedIP := MaskAuditIP(item.IP)
		current := merged[maskedIP]
		if current == nil {
			item.IP = maskedIP
			item.Users = nil
			item.Timeline = nil
			merged[maskedIP] = &item
			continue
		}
		current.RequestCount += item.RequestCount
		current.RegistrationCount += item.RegistrationCount
		current.APIRequestCount += item.APIRequestCount
		current.ErrorCount += item.ErrorCount
		current.LoginSuccessCount += item.LoginSuccessCount
		current.LoginFailureCount += item.LoginFailureCount
		current.LogoutCount += item.LogoutCount
		current.AnomalyCount += item.AnomalyCount
		current.UserCount += item.UserCount
		current.Quota += item.Quota
		if current.FirstSeenAt == 0 || item.FirstSeenAt < current.FirstSeenAt {
			current.FirstSeenAt = item.FirstSeenAt
		}
		if item.LastSeenAt > current.LastSeenAt {
			current.LastSeenAt = item.LastSeenAt
		}
	}

	result := make([]IPAuditItem, 0, len(merged))
	for _, item := range merged {
		result = append(result, *item)
	}
	return result
}

func populateMaskedIPAuditUserCounts(items []IPAuditItem, registrations []ipAuditRegistrationRow, startTimestamp int64, endTimestamp int64, excludedUserIDs []int) error {
	var rows []struct {
		IP     string `gorm:"column:ip"`
		UserID int    `gorm:"column:user_id"`
	}
	tx := applyIPAuditTimeRange(
		applyIPAuditUserExclusions(logReadDB(), excludedUserIDs).
			Select("DISTINCT ip, user_id").Where("ip <> ? AND user_id > ? AND content <> ?", "", 0, "user.register"),
		startTimestamp,
		endTimestamp,
	)
	if err := tx.Scan(&rows).Error; err != nil {
		return err
	}
	usersByMask := make(map[string]map[int]struct{})
	for _, row := range rows {
		masked := MaskAuditIP(row.IP)
		if masked == "" {
			continue
		}
		if usersByMask[masked] == nil {
			usersByMask[masked] = make(map[int]struct{})
		}
		usersByMask[masked][row.UserID] = struct{}{}
	}
	for _, row := range registrations {
		masked := MaskAuditIP(row.IP)
		if masked == "" || row.UserID <= 0 {
			continue
		}
		if usersByMask[masked] == nil {
			usersByMask[masked] = make(map[int]struct{})
		}
		usersByMask[masked][row.UserID] = struct{}{}
	}
	for i := range items {
		items[i].UserCount = len(usersByMask[items[i].IP])
	}
	return nil
}

func populateRootIPAuditDetails(items []IPAuditItem, variants map[string][]string, registrations []ipAuditRegistrationRow, startTimestamp int64, endTimestamp int64, excludedUserIDs []int) error {
	for i := range items {
		variantList := variants[items[i].IP]
		if len(variantList) == 0 {
			variantList = []string{items[i].IP}
		}
		userQuery := applyIPAuditTimeRange(
			applyIPAuditUserExclusions(logReadDB(), excludedUserIDs).
				Select("DISTINCT user_id, username").Where("user_id > ?", 0),
			startTimestamp,
			endTimestamp,
		)
		userQuery = applyIPAuditVariantFilter(userQuery, variantList)
		var logUsers []IPAuditUser
		if err := userQuery.Order("username asc, user_id asc").Scan(&logUsers).Error; err != nil {
			return err
		}
		users := make(map[int]IPAuditUser, len(logUsers))
		for _, user := range logUsers {
			users[user.UserId] = user
		}
		for _, registration := range registrations {
			if NormalizeAuditIP(registration.IP) == items[i].IP && registration.UserID > 0 {
				users[registration.UserID] = IPAuditUser{UserId: registration.UserID, Username: registration.Username}
			}
		}
		items[i].Users = make([]IPAuditUser, 0, len(users))
		for _, user := range users {
			items[i].Users = append(items[i].Users, user)
		}
		sort.Slice(items[i].Users, func(left, right int) bool {
			if items[i].Users[left].Username != items[i].Users[right].Username {
				return items[i].Users[left].Username < items[i].Users[right].Username
			}
			return items[i].Users[left].UserId < items[i].Users[right].UserId
		})
		items[i].UserCount = len(items[i].Users)

		timelineQuery := applyIPAuditTimeRange(
			applyIPAuditUserExclusions(logReadDB(), excludedUserIDs).
				Select("id, created_at, user_id, username, type, content, request_id").
				Where("content <> ?", "user.register"),
			startTimestamp,
			endTimestamp,
		)
		timelineQuery = applyIPAuditVariantFilter(timelineQuery, variantList)
		if err := timelineQuery.Order(LogReadOrder("logs.")).Limit(20).Scan(&items[i].Timeline).Error; err != nil {
			return err
		}
		for j := range items[i].Timeline {
			items[i].Timeline[j].Kind = ipAuditTimelineKind(items[i].Timeline[j].Type, items[i].Timeline[j].Content)
			items[i].Timeline[j].Content = ""
		}
		for _, registration := range registrations {
			if NormalizeAuditIP(registration.IP) != items[i].IP {
				continue
			}
			items[i].Timeline = append(items[i].Timeline, IPAuditTimelineItem{
				CreatedAt: registration.CreatedAt,
				UserId:    registration.UserID,
				Username:  registration.Username,
				Kind:      "registration",
			})
		}
		sort.SliceStable(items[i].Timeline, func(left, right int) bool {
			if items[i].Timeline[left].CreatedAt != items[i].Timeline[right].CreatedAt {
				return items[i].Timeline[left].CreatedAt > items[i].Timeline[right].CreatedAt
			}
			if items[i].Timeline[left].RequestId != items[i].Timeline[right].RequestId {
				return items[i].Timeline[left].RequestId > items[i].Timeline[right].RequestId
			}
			if items[i].Timeline[left].Id != items[i].Timeline[right].Id {
				return items[i].Timeline[left].Id > items[i].Timeline[right].Id
			}
			if items[i].Timeline[left].Kind != items[i].Timeline[right].Kind {
				return items[i].Timeline[left].Kind < items[i].Timeline[right].Kind
			}
			return items[i].Timeline[left].UserId < items[i].Timeline[right].UserId
		})
		if len(items[i].Timeline) > 20 {
			items[i].Timeline = items[i].Timeline[:20]
		}
	}
	return nil
}

func ipAuditTimelineKind(logType int, content string) string {
	switch logType {
	case LogTypeConsume:
		return "api_request"
	case LogTypeError:
		return "api_error"
	case LogTypeLogin:
		if content == "user.login_failed" {
			return "login_failure"
		}
		if content == "user.logout" {
			return "logout"
		}
		if content == "user.register" {
			return "registration"
		}
		return "login_success"
	default:
		return "activity_" + strconv.Itoa(logType)
	}
}

func applyIPAuditTimeRange(tx *gorm.DB, startTimestamp int64, endTimestamp int64) *gorm.DB {
	if startTimestamp != 0 {
		tx = tx.Where("created_at >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		tx = tx.Where("created_at <= ?", endTimestamp)
	}
	return tx
}

func applyIPAuditUserExclusions(tx *gorm.DB, excludedUserIDs []int) *gorm.DB {
	for start := 0; start < len(excludedUserIDs); start += 500 {
		end := min(start+500, len(excludedUserIDs))
		tx = tx.Where("user_id NOT IN ?", excludedUserIDs[start:end])
	}
	return tx
}

func applyIPAuditVariantFilter(tx *gorm.DB, variants []string) *gorm.DB {
	if len(variants) == 1 {
		return tx.Where("ip = ?", variants[0])
	}
	return tx.Where("ip IN ?", variants)
}

func sortIPAuditItems(items []IPAuditItem) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].RequestCount != items[j].RequestCount {
			return items[i].RequestCount > items[j].RequestCount
		}
		if items[i].LastSeenAt != items[j].LastSeenAt {
			return items[i].LastSeenAt > items[j].LastSeenAt
		}
		return items[i].IP < items[j].IP
	})
}

func logIPFilterValues(ip string) []string {
	normalized := NormalizeLogIP(ip)
	if normalized == "" {
		return nil
	}
	values := []string{normalized}
	seen := map[string]bool{normalized: true}
	if normalized == "127.0.0.1" {
		for _, value := range []string{"::1", "::ffff:127.0.0.1"} {
			if !seen[value] {
				values = append(values, value)
				seen[value] = true
			}
		}
	}
	trimmed := strings.TrimSpace(ip)
	if trimmed != "" && !seen[trimmed] {
		values = append(values, trimmed)
	}
	return values
}

func applyLogIPFilter(tx *gorm.DB, column string, ip string) (*gorm.DB, error) {
	trimmed := strings.TrimSpace(ip)
	if trimmed == "" {
		return tx, nil
	}
	if strings.Contains(trimmed, "%") {
		return applyExplicitLogTextFilter(tx, column, trimmed)
	}
	values := logIPFilterValues(trimmed)
	if len(values) == 0 {
		return tx, nil
	}
	if len(values) == 1 {
		return tx.Where(column+" = ?", values[0]), nil
	}
	return tx.Where(column+" IN ?", values), nil
}

func normalizeAdminLogIPs(logs []*Log) {
	for i := range logs {
		logs[i].Ip = NormalizeAuditIP(logs[i].Ip)
	}
}

func FormatInvitationCommissionLogContent(triggerType string, amount, rateBasisPoints int64) string {
	label := "邀请返现"
	switch triggerType {
	case InviteRewardTriggerCashTopUp:
		label = "邀请充值返现"
	case InviteRewardTriggerCashSubscription:
		label = "邀请订阅返现"
	case InviteRewardTriggerAdminBalanceRedemption:
		label = "邀请兑换返现"
	case InviteRewardTriggerAdminSubscriptionClaim:
		label = "邀请订阅兑换返现"
	}
	rate := strconv.FormatInt(rateBasisPoints/100, 10)
	if fractional := rateBasisPoints % 100; fractional != 0 {
		rate += "." + strings.TrimRight(fmt.Sprintf("%02d", fractional), "0")
	}
	return label + " +" + logger.LogQuota64(amount) + "，比例 " + rate + "%"
}

func FormatInvitationCommissionReversalLogContent(amount int64) string {
	return "邀请返现撤销 -" + logger.LogQuota64(amount)
}

// don't use iota, avoid change log type value
const (
	LogTypeUnknown = 0
	LogTypeTopup   = 1
	LogTypeConsume = 2
	LogTypeManage  = 3
	LogTypeSystem  = 4
	LogTypeError   = 5
	LogTypeRefund  = 6
	LogTypeLogin   = 7
)

func ensureLogRequestId(log *Log) {
	if log != nil && log.RequestId == "" {
		log.RequestId = common.NewRequestId()
	}
}

func createLog(log *Log) error {
	return createLogWithContext(context.Background(), log)
}

func createLogWithContext(ctx context.Context, log *Log) error {
	ensureLogRequestId(log)
	if err := LOG_DB.WithContext(ctx).Create(log).Error; err != nil {
		return err
	}
	return projectIPSecurityLog(ctx, log)
}

func projectIPSecurityLog(ctx context.Context, log *Log) error {
	if !IPSecurityCollectionEnabled() {
		return nil
	}
	event, project := IPSecurityEventFromLog(log)
	if !project {
		return nil
	}
	return DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return RecordIPSecurityEventWithTx(tx, event)
	})
}

func clickHouseLogOrder(prefix string) string {
	return prefix + "created_at desc, " + prefix + "request_id desc"
}

func clickHouseAuditEventIdentitySQL() string {
	return `if(event_id = '', concat('legacy:', toString(cityHash64(user_id, created_at, type, content,
		username, token_name, model_name, quota, prompt_tokens, completion_tokens, use_time, is_stream,
		channel_id, token_id, ` + "`group`" + `, ip, request_id, upstream_request_id, other, id))), event_id)`
}

func clickHouseDeduplicatedLogsTableSQL() string {
	return clickHouseDeduplicatedLogsTableSQLWithFilter("")
}

func clickHouseDeduplicatedLogsTableSQLWithFilter(sourceFilter string) string {
	identity := clickHouseAuditEventIdentitySQL()
	source := "FROM logs"
	if sourceFilter != "" {
		source += "\n\tWHERE " + sourceFilter
	}
	return `(SELECT * FROM (
	SELECT logs.*, ` + identity + ` AS audit_event_identity, row_number() OVER (
		PARTITION BY ` + identity + `
		ORDER BY created_at DESC, request_id DESC
	) AS audit_event_rank
	` + source + `
) WHERE audit_event_rank = 1)`
}

func clickHouseScopedLogReadDB(sourceFilter string, args ...any) *gorm.DB {
	return LOG_DB.Table(clickHouseDeduplicatedLogsTableSQLWithFilter(sourceFilter)+" AS logs", args...)
}

func logReadDB() *gorm.DB {
	if common.UsingLogDatabase(common.DatabaseTypeClickHouse) {
		return LOG_DB.Table(clickHouseDeduplicatedLogsTableSQL() + " AS logs")
	}
	return LOG_DB.Model(&Log{})
}

// LogReadDB returns the canonical read scope, including ClickHouse event de-duplication.
func LogReadDB() *gorm.DB {
	return logReadDB()
}

// FindCanonicalLogByEventID reads the persisted audit log selected by the active database's de-duplication rules.
func FindCanonicalLogByEventID(ctx context.Context, eventID string) (Log, bool, error) {
	var log Log
	query := LogReadDB()
	condition := "event_id = ?"
	if common.UsingLogDatabase(common.DatabaseTypeClickHouse) {
		query = clickHouseScopedLogReadDB("event_id = ?", eventID)
		condition = "logs.event_id = ?"
	}
	result := query.WithContext(ctx).Where(condition, eventID).Limit(1).Find(&log)
	if result.Error != nil {
		return Log{}, false, result.Error
	}
	return log, result.RowsAffected == 1, nil
}

// LogReadOrder returns a deterministic newest-first order for the active log database.
func LogReadOrder(prefix string) string {
	if common.UsingLogDatabase(common.DatabaseTypeClickHouse) {
		return clickHouseLogOrder(prefix)
	}
	return prefix + "created_at desc, " + prefix + "request_id desc, " + prefix + "id desc"
}

func assignDisplayLogIds(logs []*Log, startIdx int) {
	for i := range logs {
		logs[i].Id = startIdx + i + 1
	}
}

func formatUserLogs(logs []*Log, startIdx int) {
	for i := range logs {
		logs[i].ChannelName = ""
		logs[i].Ip = ""
		var otherMap map[string]interface{}
		otherMap, _ = common.StrToMap(logs[i].Other)
		if otherMap != nil {
			redactUserVisibleOperationParams(otherMap)
			// Remove admin-only debug fields.
			delete(otherMap, "admin_info")
			// Remove operation-audit details (operator/route info), admin-only.
			delete(otherMap, "audit_info")
			// delete(otherMap, "reject_reason")
		}
		logs[i].Other = common.MapToJsonStr(otherMap)
	}
	assignDisplayLogIds(logs, startIdx)
}

func redactUserVisibleOperationParams(other map[string]interface{}) {
	op, ok := other["op"].(map[string]interface{})
	if !ok {
		return
	}
	params, ok := op["params"].(map[string]interface{})
	if !ok {
		return
	}
	action, _ := op["action"].(string)
	switch {
	case action == "source_attribution.confirm":
		delete(params, "target_inviter_id")
		delete(params, "target_username")
		delete(params, "target_aff_code")
		delete(params, "source_description")
	case strings.HasPrefix(action, "user.redemption."):
		delete(params, "target_user_id")
		delete(params, "claimant_user_id")
		delete(params, "requested_by_user_id")
	}
}

func GetLogByTokenId(tokenId int) (logs []*Log, err error) {
	order := "id desc"
	if common.UsingLogDatabase(common.DatabaseTypeClickHouse) {
		order = clickHouseLogOrder("")
	}
	err = logReadDB().Where("token_id = ?", tokenId).Order(order).Limit(common.MaxRecentItems).Find(&logs).Error
	formatUserLogs(logs, 0)
	return logs, err
}

func RecordLog(userId int, logType int, content string, callerIp ...string) {
	if logType == LogTypeConsume && !common.LogConsumeEnabled {
		return
	}
	username, _ := GetUsernameById(userId, false)
	ip := ""
	if len(callerIp) > 0 {
		ip = NormalizeAuditIP(callerIp[0])
	}
	log := &Log{
		UserId:    userId,
		Username:  username,
		CreatedAt: common.GetTimestamp(),
		Type:      logType,
		Content:   content,
		Ip:        ip,
	}
	err := createLog(log)
	if err != nil {
		common.SysLog("failed to record log: " + err.Error())
	}
}

// RecordLogWithAdminInfo 记录操作日志，并将管理员相关信息存入 Other.admin_info，
func RecordLogWithAdminInfo(userId int, logType int, content string, adminInfo map[string]interface{}) {
	if logType == LogTypeConsume && !common.LogConsumeEnabled {
		return
	}
	username, _ := GetUsernameById(userId, false)
	log := &Log{
		UserId:    userId,
		Username:  username,
		CreatedAt: common.GetTimestamp(),
		Type:      logType,
		Content:   content,
	}
	if len(adminInfo) > 0 {
		other := map[string]interface{}{
			"admin_info": adminInfo,
		}
		log.Other = common.MapToJsonStr(other)
	}
	if err := createLog(log); err != nil {
		common.SysLog("failed to record log: " + err.Error())
	}
}

// buildOpField 构建语言无关的操作描述（写入 Other.op）。
// 前端依据 action(稳定操作标识) + params(结构化参数) 在渲染期用 i18n 本地化展示，
// 因此不在数据库中存储自然语言句子。
func buildOpField(action string, params map[string]interface{}) map[string]interface{} {
	op := map[string]interface{}{
		"action": action,
	}
	if len(params) > 0 {
		op["params"] = params
	}
	return op
}

func attachBusinessOp(other map[string]interface{}, action string, params map[string]interface{}) map[string]interface{} {
	if other == nil {
		other = map[string]interface{}{}
	}
	if _, exists := other["op"]; !exists {
		other["op"] = buildOpField(action, params)
	}
	return other
}

// RecordLoginLog 记录用户登录成功的审计日志（type=LogTypeLogin）。
// username 由调用方传入（登录流程已持有用户对象），避免额外的数据库查询。
// content 为英文兜底文本（用于导出/经典前端）；action+params 供前端本地化渲染。
// extra 可携带 login_method、user_agent 等附加信息（普通用户可见）。
func RecordLoginLog(userId int, username string, content string, ip string, action string, params map[string]interface{}, extra map[string]interface{}) {
	other := map[string]interface{}{}
	for k, v := range extra {
		other[k] = v
	}
	other["op"] = buildOpField(action, params)
	log := &Log{
		UserId:    userId,
		Username:  username,
		CreatedAt: common.GetTimestamp(),
		Type:      LogTypeLogin,
		Content:   content,
		Ip:        NormalizeAuditIP(ip),
		Other:     common.MapToJsonStr(other),
	}
	if err := createLog(log); err != nil {
		common.SysLog("failed to record login log: " + err.Error())
	}
}

// RecordOperationAuditLog 记录管理/高危操作审计日志（type=LogTypeManage）。
// logUserId 为日志归属者，管理审计日志应归属实际操作者；目标资源/用户放入
// action params。username 内部按 logUserId 查询。content 为英文兜底文本（导出/经典前端用）。
// action+params 写入 Other.op，供前端本地化渲染（普通用户可见，不含敏感信息）。
// adminInfo 存放操作者身份（写入 Other.admin_info，普通用户查询时剥离）；
// auditInfo 存放路由/方法/结果等中间件兜底信息（写入 Other.audit_info，普通用户查询时剥离）。
func RecordOperationAuditLog(logUserId int, content string, ip string, action string, params map[string]interface{}, adminInfo map[string]interface{}, auditInfo map[string]interface{}) {
	username, _ := GetUsernameById(logUserId, false)
	other := map[string]interface{}{
		"op": buildOpField(action, params),
	}
	if len(adminInfo) > 0 {
		other["admin_info"] = adminInfo
	}
	if len(auditInfo) > 0 {
		other["audit_info"] = auditInfo
	}
	log := &Log{
		UserId:    logUserId,
		Username:  username,
		CreatedAt: common.GetTimestamp(),
		Type:      LogTypeManage,
		Content:   content,
		Ip:        NormalizeAuditIP(ip),
		Other:     common.MapToJsonStr(other),
	}
	if err := createLog(log); err != nil {
		common.SysLog("failed to record operation audit log: " + err.Error())
	}
}

func RecordTopupLog(userId int, content string, callerIp string, paymentMethod string, callbackPaymentMethod string) {
	username, _ := GetUsernameById(userId, false)
	normalizedCallerIP := NormalizeAuditIP(callerIp)
	adminInfo := map[string]interface{}{
		"server_ip":               common.GetIp(),
		"node_name":               common.NodeName,
		"caller_ip":               normalizedCallerIP,
		"payment_method":          paymentMethod,
		"callback_payment_method": callbackPaymentMethod,
		"version":                 common.DisplayVersion,
	}
	other := map[string]interface{}{
		"admin_info": adminInfo,
	}
	other = attachBusinessOp(other, "billing.topup", map[string]interface{}{
		"payment_method":          paymentMethod,
		"callback_payment_method": callbackPaymentMethod,
	})
	log := &Log{
		UserId:    userId,
		Username:  username,
		CreatedAt: common.GetTimestamp(),
		Type:      LogTypeTopup,
		Content:   content,
		Ip:        normalizedCallerIP,
		Other:     common.MapToJsonStr(other),
	}
	err := createLog(log)
	if err != nil {
		common.SysLog("failed to record topup log: " + err.Error())
	}
}

func RecordErrorLog(c *gin.Context, userId int, channelId int, modelName string, tokenName string, content string, tokenId int, useTimeSeconds int,
	isStream bool, group string, other map[string]interface{}) {
	logger.LogInfo(c, fmt.Sprintf("record error log: userId=%d, channelId=%d, modelName=%s, tokenName=%s, content=%s", userId, channelId, modelName, tokenName, common.LocalLogPreview(content)))
	username := c.GetString("username")
	requestId := c.GetString(common.RequestIdKey)
	upstreamRequestId := c.GetString(common.UpstreamRequestIdKey)
	otherStr := common.MapToJsonStr(other)
	// 判断是否需要记录 IP
	log := &Log{
		UserId:            userId,
		Username:          username,
		CreatedAt:         common.GetTimestamp(),
		Type:              LogTypeError,
		Content:           content,
		PromptTokens:      0,
		CompletionTokens:  0,
		TokenName:         tokenName,
		ModelName:         modelName,
		Quota:             0,
		ChannelId:         channelId,
		TokenId:           tokenId,
		UseTime:           useTimeSeconds,
		IsStream:          isStream,
		Group:             group,
		Ip:                NormalizeAuditIP(c.ClientIP()),
		RequestId:         requestId,
		UpstreamRequestId: upstreamRequestId,
		Other:             otherStr,
	}
	err := createLog(log)
	if err != nil {
		logger.LogError(c, "failed to record log: "+err.Error())
	}
}

type RecordConsumeLogParams struct {
	ChannelId        int                    `json:"channel_id"`
	PromptTokens     int                    `json:"prompt_tokens"`
	CompletionTokens int                    `json:"completion_tokens"`
	ModelName        string                 `json:"model_name"`
	TokenName        string                 `json:"token_name"`
	Quota            int                    `json:"quota"`
	Content          string                 `json:"content"`
	TokenId          int                    `json:"token_id"`
	UseTimeSeconds   int                    `json:"use_time_seconds"`
	IsStream         bool                   `json:"is_stream"`
	Group            string                 `json:"group"`
	Other            map[string]interface{} `json:"other"`
}

func RecordConsumeLog(c *gin.Context, userId int, params RecordConsumeLogParams) error {
	username := c.GetString("username")
	requestId := c.GetString(common.RequestIdKey)
	if requestId == "" {
		requestId = common.NewRequestId()
		c.Set(common.RequestIdKey, requestId)
	}
	upstreamRequestId := c.GetString(common.UpstreamRequestIdKey)
	createdAt := common.GetTimestamp()
	persistContext, cancel := context.WithTimeout(context.WithoutCancel(c.Request.Context()), 5*time.Second)
	defer cancel()
	if !common.LogConsumeEnabled {
		log := &Log{
			UserId: userId, Username: username, CreatedAt: createdAt, Type: LogTypeConsume,
			Ip: NormalizeAuditIP(c.ClientIP()), RequestId: requestId,
		}
		if err := projectIPSecurityLog(persistContext, log); err != nil {
			logger.LogError(c, "failed to record IP security event: "+err.Error())
			return err
		}
		return nil
	}
	logger.LogInfo(c, fmt.Sprintf("record consume log: userId=%d, params=%s", userId, common.GetJsonString(params)))
	params.Other = attachBusinessOp(params.Other, "billing.consume", map[string]interface{}{
		"model":      params.ModelName,
		"token_id":   params.TokenId,
		"channel_id": params.ChannelId,
		"quota":      params.Quota,
	})
	otherStr := common.MapToJsonStr(params.Other)
	// 判断是否需要记录 IP
	log := &Log{
		UserId:            userId,
		Username:          username,
		CreatedAt:         createdAt,
		Type:              LogTypeConsume,
		Content:           params.Content,
		PromptTokens:      params.PromptTokens,
		CompletionTokens:  params.CompletionTokens,
		TokenName:         params.TokenName,
		ModelName:         params.ModelName,
		Quota:             params.Quota,
		ChannelId:         params.ChannelId,
		TokenId:           params.TokenId,
		UseTime:           params.UseTimeSeconds,
		IsStream:          params.IsStream,
		Group:             params.Group,
		Ip:                NormalizeAuditIP(c.ClientIP()),
		RequestId:         requestId,
		UpstreamRequestId: upstreamRequestId,
		Other:             otherStr,
	}
	err := createLogWithContext(persistContext, log)
	if err != nil {
		logger.LogError(c, "failed to record log: "+err.Error())
		return err
	}
	if common.DataExportEnabled {
		LogQuotaData(QuotaDataLogParams{
			UserID:    userId,
			Username:  username,
			ModelName: params.ModelName,
			Quota:     params.Quota,
			CreatedAt: createdAt,
			TokenUsed: params.PromptTokens + params.CompletionTokens,
			UseGroup:  params.Group,
			TokenID:   params.TokenId,
			ChannelID: params.ChannelId,
			NodeName:  common.NodeName,
		})
	}
	return nil
}

type RecordTaskBillingLogParams struct {
	UserId    int
	LogType   int
	Content   string
	ChannelId int
	ModelName string
	Quota     int
	TokenId   int
	Group     string
	Ip        string
	Other     map[string]interface{}
	NodeName  string // 任务发起节点；为空时回退当前节点
}

func RecordTaskBillingLog(params RecordTaskBillingLogParams) {
	if params.LogType == LogTypeConsume && !common.LogConsumeEnabled {
		return
	}
	username, _ := GetUsernameById(params.UserId, false)
	tokenName := ""
	if params.TokenId > 0 {
		if token, err := GetTokenById(params.TokenId); err == nil {
			tokenName = token.Name
		}
	}
	createdAt := common.GetTimestamp()
	action := "billing.consume"
	if params.LogType == LogTypeRefund {
		action = "billing.refund"
	}
	params.Other = attachBusinessOp(params.Other, action, map[string]interface{}{
		"model":      params.ModelName,
		"token_id":   params.TokenId,
		"channel_id": params.ChannelId,
		"quota":      params.Quota,
	})
	log := &Log{
		UserId:    params.UserId,
		Username:  username,
		CreatedAt: createdAt,
		Type:      params.LogType,
		Content:   params.Content,
		TokenName: tokenName,
		ModelName: params.ModelName,
		Quota:     params.Quota,
		ChannelId: params.ChannelId,
		TokenId:   params.TokenId,
		Group:     params.Group,
		Ip:        NormalizeAuditIP(params.Ip),
		Other:     common.MapToJsonStr(params.Other),
	}
	err := createLog(log)
	if err != nil {
		common.SysLog("failed to record task billing log: " + err.Error())
	}
	if params.LogType == LogTypeConsume && common.DataExportEnabled {
		nodeName := params.NodeName
		if nodeName == "" {
			nodeName = common.NodeName
		}
		LogQuotaData(QuotaDataLogParams{
			UserID:    params.UserId,
			Username:  username,
			ModelName: params.ModelName,
			Quota:     params.Quota,
			CreatedAt: createdAt,
			UseGroup:  params.Group,
			TokenID:   params.TokenId,
			ChannelID: params.ChannelId,
			NodeName:  nodeName,
		})
	}
}

// RecordTaskBillingLogWithTx persists one terminal task billing log inside a
// caller-owned transaction. EventId is the deterministic terminal operation
// identity, making retries and process restarts idempotent without relying on
// timestamps. The regular RecordTaskBillingLog path remains unchanged for
// synchronous billing callers.
func RecordTaskBillingLogWithTx(tx *gorm.DB, params RecordTaskBillingLogParams, operationID string) error {
	if tx == nil || strings.TrimSpace(operationID) == "" {
		return errors.New("invalid task billing log transaction")
	}
	if params.LogType == LogTypeConsume && !common.LogConsumeEnabled {
		return nil
	}
	username := ""
	_ = tx.Model(&User{}).Where("id = ?", params.UserId).Select("username").Scan(&username).Error
	tokenName := ""
	if params.TokenId > 0 {
		var token Token
		if err := tx.Unscoped().Select("name").Where("id = ?", params.TokenId).First(&token).Error; err == nil {
			tokenName = token.Name
		}
	}
	action := "billing.consume"
	if params.LogType == LogTypeRefund {
		action = "billing.refund"
	}
	params.Other = attachBusinessOp(params.Other, action, map[string]interface{}{
		"model":        params.ModelName,
		"token_id":     params.TokenId,
		"channel_id":   params.ChannelId,
		"quota":        params.Quota,
		"operation_id": operationID,
	})
	eventID := operationID
	log := &Log{
		EventId:   &eventID,
		UserId:    params.UserId,
		Username:  username,
		CreatedAt: common.GetTimestamp(),
		Type:      params.LogType,
		Content:   params.Content,
		TokenName: tokenName,
		ModelName: params.ModelName,
		Quota:     params.Quota,
		ChannelId: params.ChannelId,
		TokenId:   params.TokenId,
		Group:     params.Group,
		Ip:        NormalizeAuditIP(params.Ip),
		Other:     common.MapToJsonStr(params.Other),
	}
	result := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "event_id"}}, DoNothing: true}).Create(log)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		var existing Log
		if err := tx.Where("event_id = ?", operationID).First(&existing).Error; err != nil {
			return err
		}
		if existing.UserId != params.UserId || existing.Type != params.LogType || existing.Quota != params.Quota {
			return fmt.Errorf("task billing log operation conflict: %s", operationID)
		}
	}
	return nil
}

func GetAllLogs(logType int, startTimestamp int64, endTimestamp int64, modelName string, username string, tokenName string, startIdx int, num int, channel int, group string, requestId string, upstreamRequestId string) (logs []*Log, total int64, err error) {
	return GetAllLogsWithIP(logType, startTimestamp, endTimestamp, modelName, username, tokenName, "", startIdx, num, channel, group, requestId, upstreamRequestId)
}

func GetAllLogsWithIP(logType int, startTimestamp int64, endTimestamp int64, modelName string, username string, tokenName string, ip string, startIdx int, num int, channel int, group string, requestId string, upstreamRequestId string) (logs []*Log, total int64, err error) {
	var tx *gorm.DB
	if logType == LogTypeUnknown {
		tx = logReadDB()
	} else {
		tx = logReadDB().Where("logs.type = ?", logType)
	}

	if tx, err = applyExplicitLogTextFilter(tx, "logs.model_name", modelName); err != nil {
		return nil, 0, err
	}
	if tx, err = applyExplicitLogTextFilter(tx, "logs.username", username); err != nil {
		return nil, 0, err
	}
	if tokenName != "" {
		tx = tx.Where("logs.token_name = ?", tokenName)
	}
	if tx, err = applyLogIPFilter(tx, "logs.ip", ip); err != nil {
		return nil, 0, err
	}
	if requestId != "" {
		tx = tx.Where("logs.request_id = ?", requestId)
	}
	if upstreamRequestId != "" {
		tx = tx.Where("logs.upstream_request_id = ?", upstreamRequestId)
	}
	if startTimestamp != 0 {
		tx = tx.Where("logs.created_at >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		tx = tx.Where("logs.created_at <= ?", endTimestamp)
	}
	if channel != 0 {
		tx = tx.Where("logs.channel_id = ?", channel)
	}
	if group != "" {
		tx = tx.Where("logs."+logGroupCol+" = ?", group)
	}
	err = tx.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}
	order := "logs.created_at desc, logs.id desc"
	if common.UsingLogDatabase(common.DatabaseTypeClickHouse) {
		order = clickHouseLogOrder("logs.")
	}
	err = tx.Order(order).Limit(num).Offset(startIdx).Find(&logs).Error
	if err != nil {
		return nil, 0, err
	}
	if common.UsingLogDatabase(common.DatabaseTypeClickHouse) {
		assignDisplayLogIds(logs, startIdx)
	}
	normalizeAdminLogIPs(logs)

	channelIds := types.NewSet[int]()
	for _, log := range logs {
		if log.ChannelId != 0 {
			channelIds.Add(log.ChannelId)
		}
	}

	if channelIds.Len() > 0 {
		var channels []struct {
			Id   int    `gorm:"column:id"`
			Name string `gorm:"column:name"`
		}
		if common.MemoryCacheEnabled {
			// Cache get channel
			for _, channelId := range channelIds.Items() {
				if cacheChannel, err := CacheGetChannel(channelId); err == nil {
					channels = append(channels, struct {
						Id   int    `gorm:"column:id"`
						Name string `gorm:"column:name"`
					}{
						Id:   channelId,
						Name: cacheChannel.Name,
					})
				}
			}
		} else {
			// Bulk query channels from DB
			if err = DB.Table("channels").Select("id, name").Where("id IN ?", channelIds.Items()).Find(&channels).Error; err != nil {
				return logs, total, err
			}
		}
		channelMap := make(map[int]string, len(channels))
		for _, channel := range channels {
			channelMap[channel.Id] = channel.Name
		}
		for i := range logs {
			logs[i].ChannelName = channelMap[logs[i].ChannelId]
		}
	}

	return logs, total, err
}

const logSearchCountLimit = 10000

func GetUserLogs(userId int, logType int, startTimestamp int64, endTimestamp int64, modelName string, tokenName string, startIdx int, num int, group string, requestId string, upstreamRequestId string) (logs []*Log, total int64, err error) {
	var tx *gorm.DB
	if logType == LogTypeUnknown {
		tx = logReadDB().Where("logs.user_id = ?", userId)
	} else {
		tx = logReadDB().Where("logs.user_id = ? and logs.type = ?", userId, logType)
	}

	if tx, err = applyExplicitLogTextFilter(tx, "logs.model_name", modelName); err != nil {
		return nil, 0, err
	}
	if tokenName != "" {
		tx = tx.Where("logs.token_name = ?", tokenName)
	}
	if requestId != "" {
		tx = tx.Where("logs.request_id = ?", requestId)
	}
	if upstreamRequestId != "" {
		tx = tx.Where("logs.upstream_request_id = ?", upstreamRequestId)
	}
	if startTimestamp != 0 {
		tx = tx.Where("logs.created_at >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		tx = tx.Where("logs.created_at <= ?", endTimestamp)
	}
	if group != "" {
		tx = tx.Where("logs."+logGroupCol+" = ?", group)
	}
	err = tx.Limit(logSearchCountLimit).Count(&total).Error
	if err != nil {
		common.SysError("failed to count user logs: " + err.Error())
		return nil, 0, errors.New("查询日志失败")
	}
	order := "logs.id desc"
	if common.UsingLogDatabase(common.DatabaseTypeClickHouse) {
		order = clickHouseLogOrder("logs.")
	}
	err = tx.Order(order).Limit(num).Offset(startIdx).Find(&logs).Error
	if err != nil {
		common.SysError("failed to search user logs: " + err.Error())
		return nil, 0, errors.New("查询日志失败")
	}

	formatUserLogs(logs, startIdx)
	return logs, total, err
}

type Stat struct {
	Quota int `json:"quota"`
	Rpm   int `json:"rpm"`
	Tpm   int `json:"tpm"`
}

func SumUsedQuota(logType int, startTimestamp int64, endTimestamp int64, modelName string, username string, tokenName string, channel int, group string) (stat Stat, err error) {
	return SumUsedQuotaWithIP(logType, startTimestamp, endTimestamp, modelName, username, tokenName, "", channel, group)
}

func SumUsedQuotaWithIP(logType int, startTimestamp int64, endTimestamp int64, modelName string, username string, tokenName string, ip string, channel int, group string) (stat Stat, err error) {
	tx := logReadDB().Select("COALESCE(sum(quota), 0) quota")

	// 为rpm和tpm创建单独的查询
	rpmTpmQuery := logReadDB().Select("count(*) rpm, COALESCE(sum(prompt_tokens), 0) + COALESCE(sum(completion_tokens), 0) tpm")

	if tx, err = applyExplicitLogTextFilter(tx, "username", username); err != nil {
		return stat, err
	}
	if rpmTpmQuery, err = applyExplicitLogTextFilter(rpmTpmQuery, "username", username); err != nil {
		return stat, err
	}
	if tokenName != "" {
		tx = tx.Where("token_name = ?", tokenName)
		rpmTpmQuery = rpmTpmQuery.Where("token_name = ?", tokenName)
	}
	if tx, err = applyLogIPFilter(tx, "ip", ip); err != nil {
		return stat, err
	}
	if rpmTpmQuery, err = applyLogIPFilter(rpmTpmQuery, "ip", ip); err != nil {
		return stat, err
	}
	if startTimestamp != 0 {
		tx = tx.Where("created_at >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		tx = tx.Where("created_at <= ?", endTimestamp)
	}
	if tx, err = applyExplicitLogTextFilter(tx, "model_name", modelName); err != nil {
		return stat, err
	}
	if rpmTpmQuery, err = applyExplicitLogTextFilter(rpmTpmQuery, "model_name", modelName); err != nil {
		return stat, err
	}
	if channel != 0 {
		tx = tx.Where("channel_id = ?", channel)
		rpmTpmQuery = rpmTpmQuery.Where("channel_id = ?", channel)
	}
	if group != "" {
		tx = tx.Where(logGroupCol+" = ?", group)
		rpmTpmQuery = rpmTpmQuery.Where(logGroupCol+" = ?", group)
	}

	tx = tx.Where("type = ?", LogTypeConsume)
	rpmTpmQuery = rpmTpmQuery.Where("type = ?", LogTypeConsume)

	// 只统计最近60秒的rpm和tpm
	rpmTpmQuery = rpmTpmQuery.Where("created_at >= ?", time.Now().Add(-60*time.Second).Unix())

	// 执行查询
	if err := tx.Scan(&stat).Error; err != nil {
		common.SysError("failed to query log stat: " + err.Error())
		return stat, errors.New("查询统计数据失败")
	}
	var rateStat struct {
		Rpm int
		Tpm int
	}
	if err := rpmTpmQuery.Scan(&rateStat).Error; err != nil {
		common.SysError("failed to query rpm/tpm stat: " + err.Error())
		return stat, errors.New("查询统计数据失败")
	}
	stat.Rpm = rateStat.Rpm
	stat.Tpm = rateStat.Tpm

	return stat, nil
}

func SumUsedToken(logType int, startTimestamp int64, endTimestamp int64, modelName string, username string, tokenName string) (token int) {
	tx := logReadDB().Select("COALESCE(sum(prompt_tokens), 0) + COALESCE(sum(completion_tokens), 0)")
	if username != "" {
		tx = tx.Where("username = ?", username)
	}
	if tokenName != "" {
		tx = tx.Where("token_name = ?", tokenName)
	}
	if startTimestamp != 0 {
		tx = tx.Where("created_at >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		tx = tx.Where("created_at <= ?", endTimestamp)
	}
	if modelName != "" {
		tx = tx.Where("model_name = ?", modelName)
	}
	tx.Where("type = ?", LogTypeConsume).Scan(&token)
	return token
}

func CountOldLog(ctx context.Context, targetTimestamp int64) (int64, error) {
	logTotal, err := countOldLogRows(ctx, targetTimestamp)
	if err != nil {
		return 0, err
	}
	projectionTotal, err := countOldIPSecurityProjection(ctx, targetTimestamp)
	if err != nil {
		return 0, err
	}
	return logTotal + projectionTotal, nil
}

func countOldLogRows(ctx context.Context, targetTimestamp int64) (int64, error) {
	var total int64
	if err := LOG_DB.WithContext(ctx).Model(&Log{}).Where("created_at < ?", targetTimestamp).Count(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}

func DeleteOldLogBatch(ctx context.Context, targetTimestamp int64, limit int) (int64, error) {
	if limit <= 0 {
		limit = 100
	}
	if nil != ctx.Err() {
		return 0, ctx.Err()
	}

	if common.UsingLogDatabase(common.DatabaseTypeClickHouse) {
		// ClickHouse DELETE is a heavy mutation that rewrites data parts, so
		// per-batch mutations would be pathologically slow. Remove all matching
		// rows in a single synchronous mutation regardless of limit; the reported
		// count lets the caller's progress loop complete in one pass.
		total, err := countOldLogRows(ctx, targetTimestamp)
		if err != nil {
			return 0, err
		}
		if total == 0 {
			return deleteOldIPSecurityProjectionBatch(ctx, targetTimestamp, limit)
		}
		if err := LOG_DB.WithContext(ctx).Exec(
			"ALTER TABLE logs DELETE WHERE created_at < ? SETTINGS mutations_sync = 1",
			targetTimestamp,
		).Error; err != nil {
			return 0, err
		}
		return total, nil
	}

	var ids []int
	if err := LOG_DB.WithContext(ctx).Model(&Log{}).Where("created_at < ?", targetTimestamp).
		Order("id asc").Limit(limit).Pluck("id", &ids).Error; err != nil {
		return 0, err
	}
	if len(ids) == 0 {
		return deleteOldIPSecurityProjectionBatch(ctx, targetTimestamp, limit)
	}
	result := LOG_DB.WithContext(ctx).Where("id IN ?", ids).Delete(&Log{})
	if result.Error != nil {
		return 0, result.Error
	}
	return result.RowsAffected, nil
}

func DeleteOldLog(ctx context.Context, targetTimestamp int64, limit int) (int64, error) {
	if limit <= 0 {
		limit = 100
	}

	var total int64 = 0

	for {
		if nil != ctx.Err() {
			return total, ctx.Err()
		}

		rowsAffected, err := DeleteOldLogBatch(ctx, targetTimestamp, limit)
		if nil != err {
			return total, err
		}

		total += rowsAffected

		if rowsAffected == 0 {
			break
		}
	}

	return total, nil
}
