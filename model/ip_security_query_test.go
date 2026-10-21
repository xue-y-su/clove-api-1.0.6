package model

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/ip_security_setting"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type ipSecurityQueryCaptureLogger struct {
	logger.Interface
	statements []string
}

func (capture *ipSecurityQueryCaptureLogger) Trace(
	_ context.Context,
	_ time.Time,
	statement func() (string, int64),
	_ error,
) {
	sql, _ := statement()
	if sql = strings.TrimSpace(sql); sql != "" {
		capture.statements = append(capture.statements, strings.ToLower(strings.Join(strings.Fields(sql), " ")))
	}
}

func openIPSecurityQueryTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	previousDB := DB
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	DB = db
	require.NoError(t, db.AutoMigrate(&User{}, &IPSecurityEvent{}, &IPAuditRollup{}))

	t.Cleanup(func() {
		DB = previousDB
		sqlDB, sqlErr := db.DB()
		if sqlErr == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func openIPSecurityQueryExternalDB(t *testing.T, dialect string) *gorm.DB {
	t.Helper()

	var (
		db  *gorm.DB
		err error
	)
	switch dialect {
	case "mysql":
		dsn := strings.TrimSpace(os.Getenv("TEST_MYSQL_DSN"))
		if dsn == "" {
			t.Skip("set TEST_MYSQL_DSN to run MySQL IP audit query integration tests")
		}
		db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
	case "postgres":
		dsn := strings.TrimSpace(os.Getenv("TEST_POSTGRES_DSN"))
		if dsn == "" {
			t.Skip("set TEST_POSTGRES_DSN to run PostgreSQL IP audit query integration tests")
		}
		db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	default:
		t.Fatalf("unsupported database dialect %q", dialect)
	}
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&User{}, &IPSecurityEvent{}, &IPAuditRollup{}))

	previousDB := DB
	previousMainType := common.MainDatabaseType()
	previousLogType := common.LogDatabaseType()
	DB = db
	if dialect == "mysql" {
		common.SetDatabaseTypes(common.DatabaseTypeMySQL, common.DatabaseTypeMySQL)
	} else {
		common.SetDatabaseTypes(common.DatabaseTypePostgreSQL, common.DatabaseTypePostgreSQL)
	}
	t.Cleanup(func() {
		DB = previousDB
		common.SetDatabaseTypes(previousMainType, previousLogType)
		sqlDB, sqlErr := db.DB()
		if sqlErr == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func seedIPSecurityQueryUser(t *testing.T, db *gorm.DB, id int, username string, role int) {
	t.Helper()
	require.NoError(t, db.Create(&User{
		Id: id, Username: username, Password: "password", AffCode: fmt.Sprintf("query-aff-%d", id),
		Role: role, Status: common.UserStatusEnabled,
	}).Error)
}

func seedIPAuditQueryRollup(t *testing.T, db *gorm.DB, bucket int64, userID int, username, ip, maskedIP string, requestCount, lastSeenAt int) {
	t.Helper()
	require.NoError(t, db.Exec(`INSERT INTO ip_audit_rollups
		(bucket, user_id, username, ip, masked_ip, request_count, api_request_count, first_seen_at, last_seen_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		bucket, userID, username, ip, maskedIP, requestCount, requestCount, lastSeenAt, lastSeenAt,
	).Error)
}

func seedIPAuditQueryEvent(t *testing.T, db *gorm.DB, eventID string, userID int, username, ip, maskedIP, kind string, quota int, createdAt int64, requestID string) {
	t.Helper()
	httpStatus := 0
	if kind == IPSecurityKindAPIError {
		httpStatus = http.StatusUnauthorized
	}
	require.NoError(t, db.Exec(`INSERT INTO ip_security_events
		(event_id, user_id, username, ip, masked_ip, kind, quota, http_status, created_at, request_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		eventID, userID, username, ip, maskedIP, kind, quota, httpStatus, createdAt, requestID,
	).Error)
}

func TestQueryIPAuditProjectionIgnoresHistoricalNonUnauthorizedErrorCounts(t *testing.T) {
	db := openIPSecurityQueryTestDB(t)
	seedIPSecurityQueryUser(t, db, 10, "alice", common.RoleCommonUser)
	require.NoError(t, db.Create(&IPAuditRollup{
		Bucket: 0, UserID: 10, Username: "alice", IP: "203.0.113.10", MaskedIP: "203.0.113.*",
		RequestCount: 3, ErrorCount: 3, UnauthorizedCount: 1, AnomalyCount: 3,
		FirstSeenAt: 100, LastSeenAt: 300,
	}).Error)

	items, total, err := QueryIPAuditProjection(context.Background(), 1, common.RoleRootUser, 0, 0, 1, 20)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, items, 1)
	require.Equal(t, 1, items[0].RequestCount)
	require.Equal(t, 1, items[0].ErrorCount)
	require.Equal(t, 1, items[0].UnauthorizedCount)
	require.Equal(t, 1, items[0].AnomalyCount)

	userProjection, err := QueryUserIPAuditProjection(context.Background(), 1, common.RoleRootUser, 10, 0, 0, 1, 20)
	require.NoError(t, err)
	require.EqualValues(t, 1, userProjection.Summary.ErrorCount)
	require.EqualValues(t, 1, userProjection.Summary.UnauthorizedCount)
	require.EqualValues(t, 1, userProjection.Summary.AnomalyCount)
}

func TestIPSecurityTimelineQueryAvoidsTemporaryOrderSort(t *testing.T) {
	db := openIPSecurityQueryTestDB(t)
	type queryPlanRow struct {
		Detail string `gorm:"column:detail"`
	}
	var plan []queryPlanRow
	require.NoError(t, db.Raw(`EXPLAIN QUERY PLAN
		SELECT id, created_at, user_id, username, kind, request_id
		FROM ip_security_events
		WHERE ip <> ? AND ip = ?
		ORDER BY created_at DESC, request_id DESC, id DESC
		LIMIT 20`, "", "203.0.113.10").Scan(&plan).Error)

	details := make([]string, 0, len(plan))
	for _, row := range plan {
		details = append(details, row.Detail)
	}
	require.NotContains(t, strings.Join(details, "\n"), "USE TEMP B-TREE")
}

func TestQueryIPAuditProjectionPaginatesAndSortsLifetimeRollups(t *testing.T) {
	db := openIPSecurityQueryTestDB(t)
	seedIPSecurityQueryUser(t, db, 10, "alice", common.RoleCommonUser)
	seedIPSecurityQueryUser(t, db, 11, "bob", common.RoleCommonUser)

	seedIPAuditQueryRollup(t, db, 0, 10, "alice", "203.0.113.10", "203.0.113.*", 7, 100)
	seedIPAuditQueryRollup(t, db, 0, 11, "bob", "203.0.113.11", "203.0.113.*", 7, 200)
	seedIPAuditQueryRollup(t, db, 0, 10, "alice", "203.0.113.12", "203.0.113.*", 2, 300)
	require.NoError(t, db.Model(&IPAuditRollup{}).Where("user_id = ?", 10).Update("quota", 70).Error)
	require.NoError(t, db.Model(&IPAuditRollup{}).Where("user_id = ? AND ip = ?", 11, "203.0.113.11").Update("quota", 80).Error)

	seedIPAuditQueryEvent(t, db, "event-a", 11, "bob", "203.0.113.11", "203.0.113.*", IPSecurityKindAPIRequest, 40, 200, "request-a")
	seedIPAuditQueryEvent(t, db, "event-b", 11, "bob", "203.0.113.11", "203.0.113.*", IPSecurityKindAPIRequest, 40, 200, "request-b")
	seedIPAuditQueryEvent(t, db, "event-c", 11, "bob", "203.0.113.11", "203.0.113.*", IPSecurityKindAPIError, 0, 200, "request-b")

	items, total, err := QueryIPAuditProjection(context.Background(), 1, common.RoleRootUser, 0, 0, 1, 1)
	require.NoError(t, err)
	require.EqualValues(t, 3, total)
	require.Len(t, items, 1)
	require.Equal(t, "203.0.113.11", items[0].IP)
	require.Equal(t, 7, items[0].RequestCount)
	require.Equal(t, 1, items[0].UserCount)
	require.Empty(t, items[0].Users)
	require.Empty(t, items[0].Timeline)

	detail, err := QueryIPAuditProjectionDetail(context.Background(), 1, common.RoleRootUser, 0, 0, items[0].IP)
	require.NoError(t, err)
	require.Equal(t, []IPAuditUser{{UserId: 11, Username: "bob"}}, detail.Users)
	require.Len(t, detail.Timeline, 3)
	require.Equal(t, "request-b", detail.Timeline[0].RequestId)
	require.Equal(t, IPSecurityKindAPIError, detail.Timeline[0].Kind)
	require.Equal(t, IPSecurityKindAPIRequest, detail.Timeline[1].Kind)
	require.Equal(t, "request-a", detail.Timeline[2].RequestId)

	secondPage, secondTotal, err := QueryIPAuditProjection(context.Background(), 1, common.RoleRootUser, 0, 0, 2, 1)
	require.NoError(t, err)
	require.Equal(t, total, secondTotal)
	require.Len(t, secondPage, 1)
	require.Equal(t, "203.0.113.10", secondPage[0].IP)
}

func TestQueryIPAuditProjectionExcludesGenericActivityFromSecurityFacts(t *testing.T) {
	db := openIPSecurityQueryTestDB(t)
	seedIPSecurityQueryUser(t, db, 10, "api-user", common.RoleCommonUser)
	seedIPSecurityQueryUser(t, db, 11, "management-user", common.RoleCommonUser)

	ip := "203.0.113.90"
	seedIPAuditQueryRollup(t, db, 0, 10, "api-user", ip, "203.0.113.*", 2, 200)
	require.NoError(t, db.Exec(`INSERT INTO ip_audit_rollups
		(bucket, user_id, username, ip, masked_ip, request_count, first_seen_at, last_seen_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		0, 11, "management-user", ip, "203.0.113.*", 50, 100, 300,
	).Error)
	seedIPAuditQueryEvent(t, db, "api-event", 10, "api-user", ip, "203.0.113.*", IPSecurityKindAPIRequest, 0, 200, "api-request")
	seedIPAuditQueryEvent(t, db, "management-event", 11, "management-user", ip, "203.0.113.*", "activity_3", 0, 300, "management-request")

	items, total, err := QueryIPAuditProjection(context.Background(), 1, common.RoleRootUser, 0, 0, 1, 20)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, items, 1)
	require.Equal(t, 2, items[0].RequestCount)
	require.Equal(t, 1, items[0].UserCount)

	detail, err := QueryIPAuditProjectionDetail(context.Background(), 1, common.RoleRootUser, 0, 0, ip)
	require.NoError(t, err)
	require.EqualValues(t, 1, detail.UserTotal)
	require.Equal(t, []IPAuditUser{{UserId: 10, Username: "api-user"}}, detail.Users)
	require.EqualValues(t, 1, detail.TimelineTotal)
	require.Len(t, detail.Timeline, 1)
	require.Equal(t, IPSecurityKindAPIRequest, detail.Timeline[0].Kind)
}

func TestQueryUserIPAuditProjectionUsesRollupsAndPaginatesRelatedUsers(t *testing.T) {
	db := openIPSecurityQueryTestDB(t)
	seedIPSecurityQueryUser(t, db, 1, "super-root", common.RoleRootUser)
	seedIPSecurityQueryUser(t, db, 10, "target", common.RoleCommonUser)
	seedIPSecurityQueryUser(t, db, 11, "registration-peer", common.RoleCommonUser)
	seedIPSecurityQueryUser(t, db, 12, "api-peer", common.RoleCommonUser)
	seedIPSecurityQueryUser(t, db, 13, "multi-ip-peer", common.RoleCommonUser)
	seedIPSecurityQueryUser(t, db, 14, "raw-only-peer", common.RoleCommonUser)

	seedIPAuditQueryRollup(t, db, 0, 10, "target", "203.0.113.10", "203.0.113.*", 3, 300)
	require.NoError(t, db.Model(&IPAuditRollup{}).
		Where("bucket = ? AND user_id = ? AND ip = ?", 0, 10, "203.0.113.10").
		Updates(map[string]interface{}{
			"registration_count":  1,
			"login_success_count": 2,
			"first_seen_at":       100,
		}).Error)
	seedIPAuditQueryRollup(t, db, 0, 10, "target", "198.51.100.20", "198.51.100.*", 2, 400)
	require.NoError(t, db.Model(&IPAuditRollup{}).
		Where("bucket = ? AND user_id = ? AND ip = ?", 0, 10, "198.51.100.20").
		Updates(map[string]interface{}{"error_count": 1, "unauthorized_count": 1, "anomaly_count": 1}).Error)

	seedIPAuditQueryRollup(t, db, 0, 11, "registration-peer", "203.0.113.10", "203.0.113.*", 0, 150)
	require.NoError(t, db.Model(&IPAuditRollup{}).
		Where("bucket = ? AND user_id = ?", 0, 11).
		Updates(map[string]interface{}{"registration_count": 1, "first_seen_at": 150}).Error)
	seedIPAuditQueryRollup(t, db, 0, 12, "api-peer", "203.0.113.10", "203.0.113.*", 8, 250)
	seedIPAuditQueryRollup(t, db, 0, 13, "multi-ip-peer", "203.0.113.10", "203.0.113.*", 1, 350)
	seedIPAuditQueryRollup(t, db, 0, 13, "multi-ip-peer", "198.51.100.20", "198.51.100.*", 1, 500)
	seedIPAuditQueryEvent(t, db, "raw-only", 14, "raw-only-peer", "203.0.113.10", "203.0.113.*", IPSecurityKindAPIRequest, 0, 600, "raw-only")

	firstPage, err := QueryUserIPAuditProjection(
		context.Background(), 1, common.RoleRootUser, 10, 0, 0, 1, 2,
	)
	require.NoError(t, err)
	require.EqualValues(t, 5, firstPage.Summary.APIRequestCount)
	require.EqualValues(t, 1, firstPage.Summary.ErrorCount)
	require.EqualValues(t, 2, firstPage.Summary.LoginSuccessCount)
	require.EqualValues(t, 1, firstPage.Summary.AnomalyCount)
	require.Equal(t, "203.0.113.10", firstPage.Summary.RegistrationIP)
	require.Equal(t, "198.51.100.20", firstPage.Summary.RecentIP)
	require.EqualValues(t, 400, firstPage.Summary.LastActivityAt)
	require.EqualValues(t, 3, firstPage.RelatedTotal)
	require.Equal(t, 1, firstPage.RelatedPage)
	require.Equal(t, 2, firstPage.RelatedPageSize)
	require.True(t, firstPage.RelatedHasMore)
	require.Equal(t, []UserIPAuditRelatedUser{
		{
			UserID: 11, Username: "registration-peer", SharedIPCount: 1,
			RegistrationCount: 1, LastSeenAt: 150,
		},
		{
			UserID: 13, Username: "multi-ip-peer", SharedIPCount: 2,
			APICallCount: 2, LastSeenAt: 500,
		},
	}, firstPage.RelatedUsers)

	secondPage, err := QueryUserIPAuditProjection(
		context.Background(), 1, common.RoleRootUser, 10, 0, 0, 2, 2,
	)
	require.NoError(t, err)
	require.EqualValues(t, 3, secondPage.RelatedTotal)
	require.False(t, secondPage.RelatedHasMore)
	require.Equal(t, []UserIPAuditRelatedUser{{
		UserID: 12, Username: "api-peer", SharedIPCount: 1,
		APICallCount: 8, LastSeenAt: 250,
	}}, secondPage.RelatedUsers)
}

func TestQueryUserIPAuditProjectionExcludesProxyArtifactsFromRelatedAccounts(t *testing.T) {
	db := openIPSecurityQueryTestDB(t)
	seedIPSecurityQueryUser(t, db, 1, "super-root", common.RoleRootUser)
	seedIPSecurityQueryUser(t, db, 10, "target", common.RoleCommonUser)
	seedIPSecurityQueryUser(t, db, 11, "public-peer", common.RoleCommonUser)
	seedIPSecurityQueryUser(t, db, 12, "proxy-only-peer", common.RoleCommonUser)

	seedIPAuditQueryRollup(t, db, 0, 10, "target", "203.0.113.10", "203.0.113.*", 1, 100)
	seedIPAuditQueryRollup(t, db, 0, 10, "target", "172.17.0.1", "172.17.0.*", 1, 200)
	seedIPAuditQueryRollup(t, db, 0, 11, "public-peer", "203.0.113.10", "203.0.113.*", 1, 110)
	seedIPAuditQueryRollup(t, db, 0, 12, "proxy-only-peer", "172.17.0.1", "172.17.0.*", 1, 210)

	result, err := QueryUserIPAuditProjection(
		context.Background(), 1, common.RoleRootUser, 10, 0, 0, 1, 20,
	)
	require.NoError(t, err)
	require.Equal(t, "172.17.0.1", result.Summary.RecentIP)
	require.EqualValues(t, 1, result.RelatedTotal)
	require.Equal(t, []UserIPAuditRelatedUser{{
		UserID: 11, Username: "public-peer", SharedIPCount: 1,
		APICallCount: 1, LastSeenAt: 110,
	}}, result.RelatedUsers)
}

func TestQueryUserIPAuditProjectionFiltersRelatedRowsBeforeGrouping(t *testing.T) {
	db := openIPSecurityQueryTestDB(t)
	seedIPSecurityQueryUser(t, db, 1, "super-root", common.RoleRootUser)
	seedIPSecurityQueryUser(t, db, 10, "target", common.RoleCommonUser)
	seedIPSecurityQueryUser(t, db, 11, "related", common.RoleCommonUser)
	seedIPSecurityQueryUser(t, db, 12, "unrelated", common.RoleCommonUser)
	seedIPAuditQueryRollup(t, db, 0, 10, "target", "203.0.113.10", "203.0.113.*", 1, 100)
	seedIPAuditQueryRollup(t, db, 0, 11, "related", "203.0.113.10", "203.0.113.*", 1, 110)
	seedIPAuditQueryRollup(t, db, 0, 12, "unrelated", "198.51.100.20", "198.51.100.*", 1, 120)

	capture := &ipSecurityQueryCaptureLogger{Interface: logger.Discard}
	DB = db.Session(&gorm.Session{Logger: capture})

	_, err := QueryUserIPAuditProjection(
		context.Background(), 1, common.RoleRootUser, 10, 0, 0, 1, 20,
	)
	require.NoError(t, err)

	relatedQueries := 0
	for _, statement := range capture.statements {
		targetJoin := strings.Index(statement, " as target_ips on ")
		if targetJoin < 0 {
			continue
		}
		relatedQueries++
		firstGroup := strings.Index(statement, " group by ")
		require.NotEqual(t, -1, firstGroup)
		require.Less(t, targetJoin, firstGroup, statement)
	}
	require.Equal(t, 2, relatedQueries)
}

func TestQueryUserIPAuditProjectionSummarizesWithoutPerIPSubqueries(t *testing.T) {
	db := openIPSecurityQueryTestDB(t)
	seedIPSecurityQueryUser(t, db, 1, "super-root", common.RoleRootUser)
	seedIPSecurityQueryUser(t, db, 10, "target", common.RoleCommonUser)
	seedIPSecurityQueryUser(t, db, 11, "related", common.RoleCommonUser)
	seedIPAuditQueryRollup(t, db, 0, 10, "target", "203.0.113.10", "203.0.113.*", 1, 100)
	seedIPAuditQueryRollup(t, db, 0, 11, "related", "203.0.113.10", "203.0.113.*", 1, 110)

	capture := &ipSecurityQueryCaptureLogger{Interface: logger.Discard}
	DB = db.Session(&gorm.Session{Logger: capture})

	_, err := QueryUserIPAuditProjection(
		context.Background(), 1, common.RoleRootUser, 10, 0, 0, 1, 20,
	)
	require.NoError(t, err)

	require.NotEmpty(t, capture.statements)
	for _, statement := range capture.statements {
		require.NotContains(t, statement, " as user_ip_projection", statement)
	}
}

func TestQueryIPAuditProjectionForViewSeparatesSharedRegistrationAPIAndAuthenticationUsers(t *testing.T) {
	db := openIPSecurityQueryTestDB(t)
	setIPSecurityRiskThresholdsForQueryTest(t, 3, 5, 10)
	for userID := 10; userID <= 19; userID++ {
		seedIPSecurityQueryUser(t, db, userID, fmt.Sprintf("user-%d", userID), common.RoleCommonUser)
	}

	for userID := 10; userID <= 14; userID++ {
		seedIPAuditQueryRollup(t, db, 0, userID, fmt.Sprintf("user-%d", userID), "203.0.113.30", "203.0.113.*", 5, 300+userID)
	}
	for userID := 15; userID <= 17; userID++ {
		seedIPAuditQueryRollup(t, db, 0, userID, fmt.Sprintf("user-%d", userID), "198.51.100.40", "198.51.100.*", 0, 400+userID)
		require.NoError(t, db.Model(&IPAuditRollup{}).
			Where("bucket = ? AND user_id = ?", 0, userID).
			Update("registration_count", 1).Error)
	}
	for userID := 18; userID <= 19; userID++ {
		seedIPAuditQueryRollup(t, db, 0, userID, fmt.Sprintf("user-%d", userID), "192.0.2.50", "192.0.2.*", 0, 500+userID)
		require.NoError(t, db.Model(&IPAuditRollup{}).
			Where("bucket = ? AND user_id = ?", 0, userID).
			Updates(map[string]interface{}{
				"login_failure_count": 1,
				"anomaly_count":       1,
			}).Error)
	}

	registrationItems, registrationTotal, err := QueryIPAuditProjectionForView(
		context.Background(), 1, common.RoleRootUser, 0, 0, 1, 20, IPAuditViewSharedRegistration,
	)
	require.NoError(t, err)
	require.EqualValues(t, 1, registrationTotal)
	require.Len(t, registrationItems, 1)
	require.Equal(t, "198.51.100.40", registrationItems[0].IP)
	require.Equal(t, 3, registrationItems[0].RegistrationUserCount)
	require.Zero(t, registrationItems[0].APIUserCount)

	apiItems, apiTotal, err := QueryIPAuditProjectionForView(
		context.Background(), 1, common.RoleRootUser, 0, 0, 1, 20, IPAuditViewSharedAPI,
	)
	require.NoError(t, err)
	require.EqualValues(t, 1, apiTotal)
	require.Len(t, apiItems, 1)
	require.Equal(t, "203.0.113.30", apiItems[0].IP)
	require.Equal(t, 5, apiItems[0].APIUserCount)
	require.Zero(t, apiItems[0].RegistrationUserCount)

	authenticationItems, authenticationTotal, err := QueryIPAuditProjectionForView(
		context.Background(), 1, common.RoleRootUser, 0, 0, 1, 20, IPAuditViewAuthentication,
	)
	require.NoError(t, err)
	require.EqualValues(t, 1, authenticationTotal)
	require.Len(t, authenticationItems, 1)
	require.Equal(t, "192.0.2.50", authenticationItems[0].IP)
	require.Equal(t, 2, authenticationItems[0].AuthenticationUserCount)
	require.Equal(t, 2, authenticationItems[0].LoginFailureCount)
}

func TestIPAuditProjectionItemReturnsStableRiskReasons(t *testing.T) {
	setIPSecurityRiskThresholdsForQueryTest(t, 3, 5, 10)
	item, err := ipAuditProjectionItem(ipAuditProjectionRow{
		IP:                      "203.0.113.30",
		RegistrationUserCount:   3,
		APIUserCount:            5,
		UnauthorizedCount:       10,
		AuthenticationUserCount: 2,
		AnomalyCount:            1,
	})
	require.NoError(t, err)
	require.Equal(t, []string{
		"shared_registration_ip",
		"shared_api_ip",
		"unauthorized_burst",
	}, item.RiskReasons)
	require.Equal(t, 3, item.RiskLevel)

	proxyItem, err := ipAuditProjectionItem(ipAuditProjectionRow{IP: "127.0.0.1"})
	require.NoError(t, err)
	require.Empty(t, proxyItem.RiskReasons)
	require.True(t, proxyItem.ProxyArtifact)
}

func TestQueryIPAuditProjectionRanksConfiguredSecurityRisks(t *testing.T) {
	db := openIPSecurityQueryTestDB(t)
	setIPSecurityRiskThresholdsForQueryTest(t, 3, 5, 10)

	const (
		registrationIP = "203.0.113.141"
		apiIP          = "203.0.113.142"
		unauthorizedIP = "203.0.113.143"
		fiveXXOnlyIP   = "203.0.113.144"
	)

	for userID := 100; userID < 103; userID++ {
		username := fmt.Sprintf("registration-%d", userID)
		seedIPSecurityQueryUser(t, db, userID, username, common.RoleCommonUser)
		seedIPAuditQueryRollup(t, db, 0, userID, username, registrationIP, "203.0.113.*", 0, 100)
		require.NoError(t, db.Model(&IPAuditRollup{}).
			Where("bucket = ? AND user_id = ? AND ip = ?", 0, userID, registrationIP).
			Update("registration_count", 1).Error)
	}
	for userID := 200; userID < 205; userID++ {
		username := fmt.Sprintf("api-%d", userID)
		seedIPSecurityQueryUser(t, db, userID, username, common.RoleCommonUser)
		seedIPAuditQueryRollup(t, db, 0, userID, username, apiIP, "203.0.113.*", 1, 200)
	}
	seedIPSecurityQueryUser(t, db, 300, "unauthorized", common.RoleCommonUser)
	seedIPSecurityQueryUser(t, db, 301, "upstream-five-xx", common.RoleCommonUser)
	for index := 0; index < 10; index++ {
		require.NoError(t, db.Create(&IPSecurityEvent{
			EventID:          fmt.Sprintf("unauthorized-%d", index),
			UserID:           300,
			Username:         "unauthorized",
			IP:               unauthorizedIP,
			MaskedIP:         "203.0.113.*",
			Kind:             IPSecurityKindAPIError,
			HTTPStatus:       http.StatusUnauthorized,
			CreatedAt:        int64(300 + index),
			RequestID:        fmt.Sprintf("unauthorized-request-%d", index),
			ProjectionStatus: IPSecurityEventProjectionPending,
		}).Error)
	}
	require.NoError(t, db.Create(&IPSecurityEvent{
		EventID:          "upstream-five-xx",
		UserID:           301,
		Username:         "upstream-five-xx",
		IP:               fiveXXOnlyIP,
		MaskedIP:         "203.0.113.*",
		Kind:             IPSecurityKindAPIError,
		HTTPStatus:       http.StatusServiceUnavailable,
		CreatedAt:        400,
		RequestID:        "upstream-five-xx",
		ProjectionStatus: IPSecurityEventProjectionPending,
	}).Error)

	riskItems, riskTotal, err := QueryIPAuditProjectionForView(
		context.Background(), 1, common.RoleRootUser, 0, 0, 1, 20, IPAuditViewRisk,
	)
	require.NoError(t, err)
	require.EqualValues(t, 3, riskTotal)
	require.Len(t, riskItems, 3)
	riskReasonsByIP := make(map[string][]string, len(riskItems))
	for _, item := range riskItems {
		riskReasonsByIP[item.IP] = item.RiskReasons
	}
	require.Equal(t, []string{"shared_registration_ip"}, riskReasonsByIP[registrationIP])
	require.Equal(t, []string{"shared_api_ip"}, riskReasonsByIP[apiIP])
	require.Equal(t, []string{"unauthorized_burst"}, riskReasonsByIP[unauthorizedIP])

	allItems, allTotal, err := QueryIPAuditProjection(
		context.Background(), 1, common.RoleRootUser, 0, 0, 1, 20,
	)
	require.NoError(t, err)
	require.EqualValues(t, 3, allTotal)
	for _, item := range allItems {
		require.NotEqual(t, fiveXXOnlyIP, item.IP)
	}

	summary, err := QueryUserIPAuditProjection(
		context.Background(), 1, common.RoleRootUser, 300, 0, 0, 1, 20,
	)
	require.NoError(t, err)
	payload, err := common.Marshal(summary.Summary)
	require.NoError(t, err)
	var summaryJSON map[string]interface{}
	require.NoError(t, common.Unmarshal(payload, &summaryJSON))
	require.Equal(t, float64(10), summaryJSON["UnauthorizedCount"])
}

func setIPSecurityRiskThresholdsForQueryTest(t *testing.T, registration, api, unauthorized int) {
	t.Helper()
	keys := []string{
		ip_security_setting.SharedRegistrationUserThresholdKey,
		ip_security_setting.SharedAPIUserThresholdKey,
		ip_security_setting.UnauthorizedThresholdKey,
	}
	previous := make(map[string]string, len(keys))
	common.OptionMapRWMutex.Lock()
	optionMapWasNil := common.OptionMap == nil
	if optionMapWasNil {
		common.OptionMap = make(map[string]string)
	}
	for _, key := range keys {
		previous[key] = common.OptionMap[key]
	}
	common.OptionMap[ip_security_setting.SharedRegistrationUserThresholdKey] = fmt.Sprintf("%d", registration)
	common.OptionMap[ip_security_setting.SharedAPIUserThresholdKey] = fmt.Sprintf("%d", api)
	common.OptionMap[ip_security_setting.UnauthorizedThresholdKey] = fmt.Sprintf("%d", unauthorized)
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		defer common.OptionMapRWMutex.Unlock()
		if optionMapWasNil {
			common.OptionMap = nil
			return
		}
		for _, key := range keys {
			common.OptionMap[key] = previous[key]
		}
	})
}

func TestQueryIPAuditProjectionDetailPagePaginatesUsersAndTimelineIndependently(t *testing.T) {
	db := openIPSecurityQueryTestDB(t)
	const ip = "203.0.113.88"
	for index := 0; index < 25; index++ {
		userID := 100 + index
		username := fmt.Sprintf("user-%02d", index)
		seedIPSecurityQueryUser(t, db, userID, username, common.RoleCommonUser)
		seedIPAuditQueryRollup(t, db, 0, userID, username, ip, "203.0.113.*", 1, 100+index)
		seedIPAuditQueryEvent(
			t,
			db,
			fmt.Sprintf("detail-%02d", index),
			userID,
			username,
			ip,
			"203.0.113.*",
			IPSecurityKindAPIRequest,
			0,
			int64(100+index),
			fmt.Sprintf("request-%02d", index),
		)
	}

	detail, err := QueryIPAuditProjectionDetailPage(
		context.Background(), 1, common.RoleRootUser, 0, 0, ip, 2, 10, 3, 10,
	)
	require.NoError(t, err)
	require.Equal(t, 25, detail.UserTotal)
	require.Equal(t, 2, detail.UserPage)
	require.Equal(t, 10, detail.UserPageSize)
	require.True(t, detail.UserHasMore)
	require.Len(t, detail.Users, 10)
	require.Equal(t, "user-10", detail.Users[0].Username)
	require.Equal(t, "user-19", detail.Users[9].Username)
	require.Equal(t, 25, detail.TimelineTotal)
	require.Equal(t, 3, detail.TimelinePage)
	require.Equal(t, 10, detail.TimelinePageSize)
	require.False(t, detail.TimelineHasMore)
	require.Len(t, detail.Timeline, 5)
	require.Equal(t, "request-04", detail.Timeline[0].RequestId)
	require.Equal(t, "request-00", detail.Timeline[4].RequestId)
}

func TestQueryIPAuditProjectionAppliesCurrentRoleHierarchy(t *testing.T) {
	db := openIPSecurityQueryTestDB(t)
	seedIPSecurityQueryUser(t, db, 1, "super-root", common.RoleRootUser)
	seedIPSecurityQueryUser(t, db, 2, "root", common.RoleRootUser)
	seedIPSecurityQueryUser(t, db, 20, "common", common.RoleCommonUser)
	seedIPSecurityQueryUser(t, db, 21, "admin", common.RoleAdminUser)
	seedIPSecurityQueryUser(t, db, 22, "peer-root", common.RoleRootUser)
	seedIPSecurityQueryUser(t, db, 23, "deleted", common.RoleCommonUser)
	require.NoError(t, db.Delete(&User{}, 23).Error)

	seedIPAuditQueryRollup(t, db, 0, 20, "common", "198.51.100.20", "198.51.100.*", 5, 500)
	seedIPAuditQueryRollup(t, db, 0, 21, "admin", "192.0.2.21", "192.0.2.*", 4, 400)
	seedIPAuditQueryRollup(t, db, 0, 22, "peer-root", "203.0.113.22", "203.0.113.*", 3, 300)
	seedIPAuditQueryRollup(t, db, 0, 23, "deleted", "100.64.0.23", "100.64.0.*", 2, 200)
	seedIPAuditQueryRollup(t, db, 0, 99, "orphan", "10.0.0.99", "10.0.0.*", 1, 100)

	adminItems, adminTotal, err := QueryIPAuditProjection(context.Background(), 21, common.RoleAdminUser, 0, 0, 1, 100)
	require.NoError(t, err)
	require.EqualValues(t, 1, adminTotal)
	require.Equal(t, []string{"198.51.100.*"}, ipAuditQueryItemIPs(adminItems))
	require.Empty(t, adminItems[0].Users)
	require.Empty(t, adminItems[0].Timeline)

	rootItems, rootTotal, err := QueryIPAuditProjection(context.Background(), 2, common.RoleRootUser, 0, 0, 1, 100)
	require.NoError(t, err)
	require.EqualValues(t, 2, rootTotal)
	require.Equal(t, []string{"198.51.100.20", "192.0.2.21"}, ipAuditQueryItemIPs(rootItems))

	superRootItems, superRootTotal, err := QueryIPAuditProjection(context.Background(), 1, common.RoleRootUser, 0, 0, 1, 100)
	require.NoError(t, err)
	require.EqualValues(t, 5, superRootTotal)
	require.Len(t, superRootItems, 5)

	require.NoError(t, db.Model(&User{}).Where("id = ?", 20).Update("role", common.RoleAdminUser).Error)
	_, adminTotal, err = QueryIPAuditProjection(context.Background(), 21, common.RoleAdminUser, 0, 0, 1, 100)
	require.NoError(t, err)
	require.Zero(t, adminTotal)
	_, rootTotal, err = QueryIPAuditProjection(context.Background(), 2, common.RoleRootUser, 0, 0, 1, 100)
	require.NoError(t, err)
	require.EqualValues(t, 2, rootTotal)

	require.NoError(t, db.Model(&User{}).Where("id = ?", 20).Update("role", common.RoleRootUser).Error)
	rootItems, rootTotal, err = QueryIPAuditProjection(context.Background(), 2, common.RoleRootUser, 0, 0, 1, 100)
	require.NoError(t, err)
	require.EqualValues(t, 1, rootTotal)
	require.Equal(t, []string{"192.0.2.21"}, ipAuditQueryItemIPs(rootItems))
}

func TestQueryIPAuditProjectionUsesEventsForTimeRange(t *testing.T) {
	db := openIPSecurityQueryTestDB(t)
	seedIPSecurityQueryUser(t, db, 30, "range-user", common.RoleCommonUser)
	seedIPAuditQueryRollup(t, db, 0, 30, "range-user", "203.0.113.30", "203.0.113.*", 99, 999)

	seedIPAuditQueryEvent(t, db, "range-90", 30, "range-user", "203.0.113.30", "203.0.113.*", IPSecurityKindAPIRequest, 11, 90, "request-90")
	seedIPAuditQueryEvent(t, db, "range-100", 30, "range-user", "203.0.113.30", "203.0.113.*", IPSecurityKindRegistration, 0, 100, "request-100")
	seedIPAuditQueryEvent(t, db, "range-150", 30, "range-user", "203.0.113.30", "203.0.113.*", IPSecurityKindAPIError, 3, 150, "request-150")
	seedIPAuditQueryEvent(t, db, "range-200", 30, "range-user", "203.0.113.30", "203.0.113.*", IPSecurityKindLoginFailure, 5, 200, "request-200")
	seedIPAuditQueryEvent(t, db, "range-210", 30, "range-user", "203.0.113.30", "203.0.113.*", IPSecurityKindLogout, 7, 210, "request-210")

	items, total, err := QueryIPAuditProjection(context.Background(), 1, common.RoleRootUser, 100, 200, 1, 10)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, items, 1)
	item := items[0]
	require.Equal(t, 2, item.RequestCount)
	require.Equal(t, 1, item.RegistrationCount)
	require.Equal(t, 1, item.ErrorCount)
	require.Equal(t, 1, item.LoginFailureCount)
	require.Equal(t, 2, item.AnomalyCount)
	require.Equal(t, 8, item.Quota)
	require.EqualValues(t, 100, item.FirstSeenAt)
	require.EqualValues(t, 200, item.LastSeenAt)
	require.Empty(t, item.Timeline)
	detail, err := QueryIPAuditProjectionDetail(context.Background(), 1, common.RoleRootUser, 100, 200, item.IP)
	require.NoError(t, err)
	require.Len(t, detail.Timeline, 3)
	require.Equal(t, IPSecurityKindLoginFailure, detail.Timeline[0].Kind)
	require.Equal(t, IPSecurityKindAPIError, detail.Timeline[1].Kind)
	require.Equal(t, IPSecurityKindRegistration, detail.Timeline[2].Kind)
}

func TestQueryIPAuditProjectionCombinesDailyRollupsWithExactPartialDays(t *testing.T) {
	db := openIPSecurityQueryTestDB(t)
	seedIPSecurityQueryUser(t, db, 31, "range-primary", common.RoleCommonUser)
	seedIPSecurityQueryUser(t, db, 32, "range-secondary", common.RoleCommonUser)

	const day = int64(86400)
	start := day + 100
	end := 4*day + 200
	ip := "203.0.113.31"
	maskedIP := "203.0.113.*"

	seedIPAuditQueryRollup(t, db, 2*day, 31, "range-primary", ip, maskedIP, 3, int(2*day+300))
	seedIPAuditQueryRollup(t, db, 3*day, 32, "range-secondary", ip, maskedIP, 4, int(3*day+400))
	require.NoError(t, db.Model(&IPAuditRollup{}).Where("bucket IN ?", []int64{2 * day, 3 * day}).Updates(map[string]interface{}{
		"quota": 10,
	}).Error)

	seedIPAuditQueryEvent(t, db, "before-range", 31, "range-primary", ip, maskedIP, IPSecurityKindAPIRequest, 100, start-1, "before-range")
	seedIPAuditQueryEvent(t, db, "left-api", 31, "range-primary", ip, maskedIP, IPSecurityKindAPIRequest, 1, start, "left-api")
	seedIPAuditQueryEvent(t, db, "left-login-failure", 31, "range-primary", ip, maskedIP, IPSecurityKindLoginFailure, 2, 2*day-1, "left-login-failure")
	seedIPAuditQueryEvent(t, db, "covered-by-rollup", 31, "range-primary", ip, maskedIP, IPSecurityKindAPIRequest, 200, 2*day+500, "covered-by-rollup")
	seedIPAuditQueryEvent(t, db, "right-error", 31, "range-primary", ip, maskedIP, IPSecurityKindAPIError, 3, 4*day+50, "right-error")
	seedIPAuditQueryEvent(t, db, "after-range", 31, "range-primary", ip, maskedIP, IPSecurityKindAPIRequest, 100, end+1, "after-range")

	items, total, err := QueryIPAuditProjection(context.Background(), 1, common.RoleRootUser, start, end, 1, 10)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, items, 1)
	item := items[0]
	require.Equal(t, 10, item.RequestCount)
	require.Equal(t, 8, item.APIRequestCount)
	require.Equal(t, 1, item.ErrorCount)
	require.Equal(t, 1, item.LoginFailureCount)
	require.Equal(t, 2, item.AnomalyCount)
	require.Equal(t, 26, item.Quota)
	require.Equal(t, 2, item.UserCount)
	require.EqualValues(t, start, item.FirstSeenAt)
	require.EqualValues(t, 4*day+50, item.LastSeenAt)
}

func TestQueryIPAuditProjectionIncludesPendingEventsWithRollups(t *testing.T) {
	db := openIPSecurityQueryTestDB(t)
	seedIPSecurityQueryUser(t, db, 34, "pending-user", common.RoleCommonUser)

	const day = int64(86400)
	dayStart := 5 * day
	ip := "203.0.113.34"
	maskedIP := "203.0.113.*"
	seedIPAuditQueryRollup(t, db, 0, 34, "pending-user", ip, maskedIP, 2, int(dayStart+100))
	seedIPAuditQueryRollup(t, db, dayStart, 34, "pending-user", ip, maskedIP, 2, int(dayStart+100))
	require.NoError(t, db.Model(&IPAuditRollup{}).Where("user_id = ?", 34).Update("quota", 10).Error)

	seedIPAuditQueryEvent(t, db, "pending-error", 34, "pending-user", ip, maskedIP, IPSecurityKindAPIError, 3, dayStart+200, "pending-request")
	require.NoError(t, db.Model(&IPSecurityEvent{}).
		Where("event_id = ?", "pending-error").
		Update("projection_status", IPSecurityEventProjectionPending).Error)

	queries := []struct {
		name       string
		start, end int64
	}{
		{name: "lifetime"},
		{name: "complete day", start: dayStart, end: dayStart + day - 1},
	}
	for _, query := range queries {
		t.Run(query.name, func(t *testing.T) {
			items, total, err := QueryIPAuditProjection(context.Background(), 1, common.RoleRootUser, query.start, query.end, 1, 10)
			require.NoError(t, err)
			require.EqualValues(t, 1, total)
			require.Len(t, items, 1)
			require.Equal(t, 3, items[0].RequestCount)
			require.Equal(t, 2, items[0].APIRequestCount)
			require.Equal(t, 1, items[0].ErrorCount)
			require.Equal(t, 1, items[0].AnomalyCount)
			require.Equal(t, 13, items[0].Quota)
			require.EqualValues(t, dayStart+100, items[0].FirstSeenAt)
			require.EqualValues(t, dayStart+200, items[0].LastSeenAt)
		})
	}
}

func TestQueryIPAuditProjectionIncludesPendingEventsAcrossSQLDialects(t *testing.T) {
	for _, dialect := range []string{"mysql", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			db := openIPSecurityQueryExternalDB(t, dialect)
			userID := 9_400_034
			if dialect == "postgres" {
				userID = 9_500_034
			}
			username := dialect + "-pending-user"
			eventID := dialect + "-pending-query-event"
			ip := "2001:db8:ffff::34"
			const day = int64(86400)
			dayStart := 5 * day

			require.NoError(t, db.Where("user_id = ?", userID).Delete(&IPAuditRollup{}).Error)
			require.NoError(t, db.Where("user_id = ?", userID).Delete(&IPSecurityEvent{}).Error)
			require.NoError(t, db.Unscoped().Where("id = ?", userID).Delete(&User{}).Error)
			t.Cleanup(func() {
				_ = db.Where("user_id = ?", userID).Delete(&IPAuditRollup{}).Error
				_ = db.Where("user_id = ?", userID).Delete(&IPSecurityEvent{}).Error
				_ = db.Unscoped().Where("id = ?", userID).Delete(&User{}).Error
			})

			seedIPSecurityQueryUser(t, db, userID, username, common.RoleCommonUser)
			seedIPAuditQueryRollup(t, db, 0, userID, username, ip, "2001:db8:ffff::*", 2, int(dayStart+100))
			seedIPAuditQueryRollup(t, db, dayStart, userID, username, ip, "2001:db8:ffff::*", 2, int(dayStart+100))
			require.NoError(t, db.Model(&IPAuditRollup{}).Where("user_id = ?", userID).Update("quota", 10).Error)
			seedIPAuditQueryEvent(t, db, eventID, userID, username, ip, "2001:db8:ffff::*", IPSecurityKindAPIError, 3, dayStart+200, dialect+"-pending-request")
			require.NoError(t, db.Model(&IPSecurityEvent{}).
				Where("event_id = ?", eventID).
				Update("projection_status", IPSecurityEventProjectionPending).Error)

			for _, query := range []struct {
				name       string
				start, end int64
			}{
				{name: "lifetime"},
				{name: "complete day", start: dayStart, end: dayStart + day - 1},
			} {
				t.Run(query.name, func(t *testing.T) {
					items, total, queryErr := QueryIPAuditProjection(
						context.Background(), 1, common.RoleRootUser, query.start, query.end, 1, 10,
					)
					require.NoError(t, queryErr)
					require.EqualValues(t, 1, total)
					require.Len(t, items, 1)
					require.Equal(t, 3, items[0].RequestCount)
					require.Equal(t, 2, items[0].APIRequestCount)
					require.Equal(t, 1, items[0].ErrorCount)
					require.Equal(t, 1, items[0].UnauthorizedCount)
					require.Equal(t, 1, items[0].AnomalyCount)
					require.Equal(t, 13, items[0].Quota)

					userProjection, userProjectionErr := QueryUserIPAuditProjection(
						context.Background(), 1, common.RoleRootUser, userID, query.start, query.end, 1, 20,
					)
					require.NoError(t, userProjectionErr)
					require.EqualValues(t, 2, userProjection.Summary.APIRequestCount)
					require.EqualValues(t, 1, userProjection.Summary.ErrorCount)
					require.EqualValues(t, 1, userProjection.Summary.UnauthorizedCount)
					require.EqualValues(t, 1, userProjection.Summary.AnomalyCount)
					require.Equal(t, ip, userProjection.Summary.RecentIP)
					require.EqualValues(t, dayStart+200, userProjection.Summary.LastActivityAt)
					require.Zero(t, userProjection.RelatedTotal)
				})
			}
		})
	}
}

func TestQueryIPAuditProjectionDefersRootDetailsUntilRequested(t *testing.T) {
	db := openIPSecurityQueryTestDB(t)
	seedIPSecurityQueryUser(t, db, 33, "deferred-user", common.RoleCommonUser)
	seedIPAuditQueryRollup(t, db, 0, 33, "deferred-user", "203.0.113.33", "203.0.113.*", 1, 100)
	seedIPAuditQueryEvent(t, db, "deferred-event", 33, "deferred-user", "203.0.113.33", "203.0.113.*", IPSecurityKindAPIRequest, 1, 100, "deferred-request")

	items, total, err := QueryIPAuditProjection(context.Background(), 1, common.RoleRootUser, 0, 0, 1, 10)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, items, 1)
	require.Empty(t, items[0].Users)
	require.Empty(t, items[0].Timeline)
}

func TestQueryIPAuditProjectionHandlesExtremePageAndCanceledContext(t *testing.T) {
	db := openIPSecurityQueryTestDB(t)
	seedIPSecurityQueryUser(t, db, 40, "page-user", common.RoleCommonUser)
	seedIPAuditQueryRollup(t, db, 0, 40, "page-user", "203.0.113.40", "203.0.113.*", 1, 100)

	items, total, err := QueryIPAuditProjection(context.Background(), 1, common.RoleRootUser, 0, 0, math.MaxInt, 100)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Empty(t, items)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err = QueryIPAuditProjection(ctx, 1, common.RoleRootUser, 0, 0, 1, 10)
	require.ErrorIs(t, err, context.Canceled)
}

func TestQueryIPAuditProjectionLimitsRootTimelineToTwenty(t *testing.T) {
	db := openIPSecurityQueryTestDB(t)
	seedIPSecurityQueryUser(t, db, 50, "timeline-user", common.RoleCommonUser)
	seedIPAuditQueryRollup(t, db, 0, 50, "timeline-user", "203.0.113.50", "203.0.113.*", 25, 125)
	for index := 0; index < 25; index++ {
		seedIPAuditQueryEvent(t, db, fmt.Sprintf("timeline-%d", index), 50, "timeline-user", "203.0.113.50", "203.0.113.*", IPSecurityKindAPIRequest, 0, int64(100+index), fmt.Sprintf("request-%02d", index))
	}

	items, _, err := QueryIPAuditProjection(context.Background(), 1, common.RoleRootUser, 0, 0, 1, 10)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Empty(t, items[0].Timeline)
	detail, err := QueryIPAuditProjectionDetail(context.Background(), 1, common.RoleRootUser, 0, 0, items[0].IP)
	require.NoError(t, err)
	require.Len(t, detail.Timeline, 20)
	require.EqualValues(t, 124, detail.Timeline[0].CreatedAt)
	require.EqualValues(t, 105, detail.Timeline[19].CreatedAt)
}

func TestQueryIPAuditProjectionBoundsRootUsersButKeepsCompleteCount(t *testing.T) {
	db := openIPSecurityQueryTestDB(t)
	seedIPSecurityQueryUser(t, db, 1, "root", common.RoleRootUser)
	for index := 0; index < 25; index++ {
		userID := 100 + index
		username := fmt.Sprintf("shared-%02d", index)
		seedIPSecurityQueryUser(t, db, userID, username, common.RoleCommonUser)
		seedIPAuditQueryRollup(t, db, 0, userID, username, "203.0.113.200", "203.0.113.*", 1, 100+index)
	}

	items, total, err := QueryIPAuditProjection(context.Background(), 1, common.RoleRootUser, 0, 0, 1, 10)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, items, 1)
	require.Equal(t, 25, items[0].UserCount)
	require.Empty(t, items[0].Users)
	detail, err := QueryIPAuditProjectionDetail(context.Background(), 1, common.RoleRootUser, 0, 0, items[0].IP)
	require.NoError(t, err)
	require.Len(t, detail.Users, 20)
	require.Equal(t, "shared-00", detail.Users[0].Username)
	require.Equal(t, "shared-19", detail.Users[19].Username)
}

func TestQueryIPAuditProjectionSeparatesProxyArtifactsFromAccountRisk(t *testing.T) {
	db := openIPSecurityQueryTestDB(t)
	setIPSecurityRiskThresholdsForQueryTest(t, 3, 5, 10)
	for userID := 70; userID <= 74; userID++ {
		seedIPSecurityQueryUser(t, db, userID, fmt.Sprintf("user-%d", userID), common.RoleCommonUser)
	}

	for _, user := range []struct {
		id       int
		username string
	}{
		{id: 70, username: "user-70"},
		{id: 71, username: "user-71"},
		{id: 72, username: "user-72"},
		{id: 73, username: "user-73"},
		{id: 74, username: "user-74"},
	} {
		seedIPAuditQueryRollup(t, db, 0, user.id, user.username, "203.0.113.70", "203.0.113.*", 1, 100)
		seedIPAuditQueryRollup(t, db, 0, user.id, user.username, "172.17.0.1", "172.17.*.*", 1, 100)
		seedIPAuditQueryRollup(t, db, 0, user.id, user.username, "fd00::1", "fd00::*", 1, 100)
	}
	seedIPAuditQueryRollup(t, db, 0, 0, "", "172.17.0.1", "172.17.*.*", 1, 100)

	riskItems, riskTotal, err := QueryIPAuditProjectionForView(
		context.Background(), 1, common.RoleRootUser, 0, 0, 1, 20, IPAuditViewRisk,
	)
	require.NoError(t, err)
	require.EqualValues(t, 1, riskTotal)
	require.Equal(t, []string{"203.0.113.70"}, ipAuditQueryItemIPs(riskItems))

	allItems, allTotal, err := QueryIPAuditProjectionForView(
		context.Background(), 1, common.RoleRootUser, 0, 0, 1, 20, IPAuditViewAll,
	)
	require.NoError(t, err)
	require.EqualValues(t, 3, allTotal)
	require.Len(t, allItems, 3)
	for _, item := range allItems {
		payload, marshalErr := common.Marshal(item)
		require.NoError(t, marshalErr)
		var contract map[string]interface{}
		require.NoError(t, common.Unmarshal(payload, &contract))
		require.Equal(t, item.IP != "203.0.113.70", contract["proxy_artifact"])
		if item.IP == "172.17.0.1" {
			require.Equal(t, 5, item.UserCount)
			require.Equal(t, 5, item.APIUserCount)
		}
	}

	detail, err := QueryIPAuditProjectionDetail(
		context.Background(), 1, common.RoleRootUser, 0, 0, "172.17.0.1",
	)
	require.NoError(t, err)
	require.Equal(t, 5, detail.UserTotal)
	require.Equal(t, []IPAuditUser{
		{UserId: 70, Username: "user-70"},
		{UserId: 71, Username: "user-71"},
		{UserId: 72, Username: "user-72"},
		{UserId: 73, Username: "user-73"},
		{UserId: 74, Username: "user-74"},
	}, detail.Users)
}

func TestQueryIPAuditProjectionDetailAllowsAdminActors(t *testing.T) {
	db := openIPSecurityQueryTestDB(t)
	seedIPSecurityQueryUser(t, db, 60, "detail-user", common.RoleCommonUser)
	seedIPAuditQueryRollup(t, db, 0, 60, "detail-user", "203.0.113.60", "203.0.113.*", 1, 100)

	detail, err := QueryIPAuditProjectionDetail(context.Background(), 61, common.RoleAdminUser, 0, 0, "203.0.113.*")
	require.NoError(t, err)
	require.Equal(t, "203.0.113.*", detail.IP)
	require.Equal(t, 1, detail.UserTotal)
	require.Equal(t, []IPAuditUser{{UserId: 60, Username: "detail-user"}}, detail.Users)
}

func ipAuditQueryItemIPs(items []IPAuditItem) []string {
	result := make([]string, len(items))
	for index := range items {
		result[index] = items[index].IP
	}
	return result
}
