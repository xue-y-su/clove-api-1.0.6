package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	clickhouseDriver "gorm.io/driver/clickhouse"
	"gorm.io/gorm"
)

func TestNormalizeIPAuditQueryPresetsAndPagination(t *testing.T) {
	now := time.Date(2026, 7, 15, 1, 30, 0, 0, time.FixedZone("UTC+8", 8*60*60))
	todayUTC := now.UTC().Truncate(24 * time.Hour)
	endOfTodayUTC := todayUTC.Add(24*time.Hour - time.Second)
	tests := []struct {
		name         string
		query        IPAuditQuery
		wantRange    string
		wantView     string
		wantStart    int64
		wantEnd      int64
		wantPage     int
		wantPageSize int
	}{
		{
			name: "default thirty days", query: IPAuditQuery{}, wantRange: IPAuditRange30Days,
			wantView:  IPAuditViewAll,
			wantStart: todayUTC.AddDate(0, 0, -29).Unix(), wantEnd: endOfTodayUTC.Unix(),
			wantPage: 1, wantPageSize: common.ItemsPerPage,
		},
		{
			name: "seven days", query: IPAuditQuery{Range: IPAuditRange7Days, View: IPAuditViewRisk, Page: 2, PageSize: 25},
			wantView:  IPAuditViewRisk,
			wantRange: IPAuditRange7Days, wantStart: todayUTC.AddDate(0, 0, -6).Unix(), wantEnd: endOfTodayUTC.Unix(),
			wantPage: 2, wantPageSize: 25,
		},
		{
			name: "ninety days caps page size", query: IPAuditQuery{Range: IPAuditRange90Days, PageSize: 1000},
			wantView:  IPAuditViewAll,
			wantRange: IPAuditRange90Days, wantStart: todayUTC.AddDate(0, 0, -89).Unix(), wantEnd: endOfTodayUTC.Unix(),
			wantPage: 1, wantPageSize: 100,
		},
		{
			name: "all history", query: IPAuditQuery{Range: IPAuditRangeAll}, wantRange: IPAuditRangeAll,
			wantView: IPAuditViewAll,
			wantPage: 1, wantPageSize: common.ItemsPerPage,
		},
		{
			name: "timestamps imply custom", query: IPAuditQuery{StartTimestamp: 100, EndTimestamp: 200},
			wantView:  IPAuditViewAll,
			wantRange: IPAuditRangeCustom, wantStart: 100, wantEnd: 200,
			wantPage: 1, wantPageSize: common.ItemsPerPage,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := normalizeIPAuditQuery(test.query, now)
			require.NoError(t, err)
			require.Equal(t, test.wantRange, got.Range)
			require.Equal(t, test.wantView, got.View)
			require.Equal(t, test.wantStart, got.StartTimestamp)
			require.Equal(t, test.wantEnd, got.EndTimestamp)
			require.Equal(t, test.wantPage, got.Page)
			require.Equal(t, test.wantPageSize, got.PageSize)
		})
	}
}

func TestNormalizeIPAuditQueryRejectsInvalidRanges(t *testing.T) {
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	for _, query := range []IPAuditQuery{
		{Range: "unknown"},
		{Range: IPAuditRangeCustom, StartTimestamp: 200, EndTimestamp: 100},
		{Range: IPAuditRangeCustom, StartTimestamp: 0, EndTimestamp: 100},
	} {
		_, err := normalizeIPAuditQuery(query, now)
		require.ErrorIs(t, err, ErrIPAuditInvalidRange)
	}
	_, err := normalizeIPAuditQuery(IPAuditQuery{Range: IPAuditRangeAll, View: "unknown"}, now)
	require.ErrorIs(t, err, ErrIPAuditInvalidView)
}

func TestRetryIPAuditReadRecoversFromTransientFailures(t *testing.T) {
	attempts := 0
	err := retryIPAuditRead(context.Background(), func() error {
		attempts++
		if attempts < ipAuditReadAttempts {
			return errors.New("temporary audit read failure")
		}
		return nil
	})

	require.NoError(t, err)
	require.Equal(t, ipAuditReadAttempts, attempts)
}

func TestRetryIPAuditReadPreservesCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	attempts := 0
	err := retryIPAuditRead(ctx, func() error {
		attempts++
		return errors.New("must not run")
	})

	require.ErrorIs(t, err, context.Canceled)
	require.Zero(t, attempts)
}

func TestRetryUserSecurityProjectionDiscardsFailedAttemptState(t *testing.T) {
	attempts := 0
	result, err := retryUserSecurityProjection(context.Background(), UserSecurityAudit{
		Enabled:      true,
		TargetUserID: 7,
		RelatedUsers: []UserSecurityRelatedUser{{UserID: 99}},
		Timeline:     []UserSecurityTimelineItem{{RequestID: "stale"}},
	}, func(attempt *UserSecurityAudit) error {
		attempts++
		attempt.Summary.APIRequestCount++
		attempt.RelatedUsers = append(attempt.RelatedUsers, UserSecurityRelatedUser{UserID: attempts})
		attempt.Timeline = append(attempt.Timeline, UserSecurityTimelineItem{RequestID: "attempt"})
		if attempts == 1 {
			return errors.New("temporary projection failure")
		}
		return nil
	})

	require.NoError(t, err)
	require.Equal(t, 2, attempts)
	require.Equal(t, 1, result.Summary.APIRequestCount)
	require.Equal(t, []UserSecurityRelatedUser{{UserID: 2}}, result.RelatedUsers)
	require.Equal(t, []UserSecurityTimelineItem{{RequestID: "attempt"}}, result.Timeline)
	require.Equal(t, 7, result.TargetUserID)
}

func TestTrustedProxyAuditWarning(t *testing.T) {
	require.Equal(t, TrustedProxiesNotConfigured, trustedProxyAuditWarning(""))
	require.Equal(t, TrustedProxiesUnsafe, trustedProxyAuditWarning("0.0.0.0/0"))
	require.Equal(t, TrustedProxiesUnsafe, trustedProxyAuditWarning("::/0"))
	require.Equal(t, TrustedProxiesUnsafe, trustedProxyAuditWarning("*"))
	require.Empty(t, trustedProxyAuditWarning("127.0.0.1,10.0.0.0/8"))
}

func TestCanQueryExternalIPProfileOnlyAllowsRootToQueryPublicAddresses(t *testing.T) {
	tests := []struct {
		name           string
		ip             string
		canViewFullIP  bool
		shouldAllowURL bool
	}{
		{name: "root public IPv4", ip: "8.8.8.8", canViewFullIP: true, shouldAllowURL: true},
		{name: "root public IPv6", ip: "2001:4860:4860::8888", canViewFullIP: true, shouldAllowURL: true},
		{name: "non-root public IPv4", ip: "8.8.8.8", canViewFullIP: false, shouldAllowURL: false},
		{name: "private IPv4", ip: "10.0.0.8", canViewFullIP: true, shouldAllowURL: false},
		{name: "loopback IPv4", ip: "127.0.0.1", canViewFullIP: true, shouldAllowURL: false},
		{name: "link local IPv4", ip: "169.254.1.1", canViewFullIP: true, shouldAllowURL: false},
		{name: "carrier grade NAT IPv4", ip: "100.64.0.8", canViewFullIP: true, shouldAllowURL: false},
		{name: "documentation IPv4", ip: "203.0.113.8", canViewFullIP: true, shouldAllowURL: false},
		{name: "private IPv6", ip: "fd00::8", canViewFullIP: true, shouldAllowURL: false},
		{name: "loopback IPv6", ip: "::1", canViewFullIP: true, shouldAllowURL: false},
		{name: "link local IPv6", ip: "fe80::8", canViewFullIP: true, shouldAllowURL: false},
		{name: "documentation IPv6", ip: "2001:db8::8", canViewFullIP: true, shouldAllowURL: false},
		{name: "invalid address", ip: "not-an-ip", canViewFullIP: true, shouldAllowURL: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.shouldAllowURL, canQueryExternalIPProfile(test.ip, test.canViewFullIP))
		})
	}
}

func TestSecurityLogReadScopeUsesClickHouseDeduplicationAndStableOrder(t *testing.T) {
	db, err := gorm.Open(clickhouseDriver.New(clickhouseDriver.Config{
		DSN:                       "clickhouse://localhost:9000/cloveapi_test",
		SkipInitializeWithVersion: true,
	}), &gorm.Config{DryRun: true, DisableAutomaticPing: true})
	require.NoError(t, err)
	previousLogDB := model.LOG_DB
	previousLogType := common.LogDatabaseType()
	model.LOG_DB = db
	common.SetLogDatabaseType(common.DatabaseTypeClickHouse)
	t.Cleanup(func() {
		model.LOG_DB = previousLogDB
		common.SetLogDatabaseType(previousLogType)
	})

	statement := securityLogReadDB(context.Background()).
		Where("logs.user_id = ?", 7).
		Order(securityLogOrder("logs.")).
		Find(&[]model.Log{}).
		Statement
	query := strings.ToLower(statement.SQL.String())

	require.Contains(t, query, "row_number() over")
	require.Contains(t, query, "audit_event_rank = 1")
	require.Contains(t, query, "order by logs.created_at desc, logs.request_id desc")

	var aggregate []securityLogCountRow
	aggregateStatement := securityLogReadDB(context.Background()).
		Select("type, content, COUNT(*) AS count").
		Where("logs.user_id = ?", 7).
		Group("type, content").
		Find(&aggregate).
		Statement
	aggregateSQL := strings.ToLower(aggregateStatement.SQL.String())
	require.Contains(t, aggregateSQL, "row_number() over")
	require.Contains(t, aggregateSQL, "group by type, content")

	var ips []struct {
		IP string `gorm:"column:ip"`
	}
	distinctStatement := securityLogReadDB(context.Background()).
		Select("DISTINCT logs.ip").
		Where("logs.user_id = ?", 7).
		Find(&ips).
		Statement
	distinctSQL := strings.ToLower(distinctStatement.SQL.String())
	require.Contains(t, distinctSQL, "select distinct logs.ip")
	require.Contains(t, distinctSQL, "audit_event_rank = 1")

	var count int64
	countStatement := securityLogReadDB(context.Background()).
		Where("logs.user_id = ?", 7).
		Count(&count).
		Statement
	countSQL := strings.ToLower(countStatement.SQL.String())
	require.Contains(t, countSQL, "select count(*)")
	require.Contains(t, countSQL, "audit_event_rank = 1")

	pageStatement := securityLogReadDB(context.Background()).
		Where("logs.user_id = ?", 7).
		Order(securityLogOrder("logs.")).
		Offset(20).
		Limit(10).
		Find(&[]model.Log{}).
		Statement
	pageSQL := strings.ToLower(pageStatement.SQL.String())
	require.Contains(t, pageSQL, "order by logs.created_at desc, logs.request_id desc")
	require.Contains(t, pageSQL, "limit ? offset ?")
	require.GreaterOrEqual(t, len(pageStatement.Vars), 2)
	require.Equal(t, 10, pageStatement.Vars[len(pageStatement.Vars)-2])
	require.Equal(t, 20, pageStatement.Vars[len(pageStatement.Vars)-1])
}

func TestUserSecurityTimelineUsesRequestIDOrderOnRelationalDatabases(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.User{}, &model.Log{}, &model.IPSecurityEvent{}, &model.IPAuditRollup{}, &model.IPSecurityProjectionState{},
	))
	previousMainDB := model.DB
	previousLogDB := model.LOG_DB
	previousMainType := common.MainDatabaseType()
	previousLogType := common.LogDatabaseType()
	model.DB = db
	model.LOG_DB = db
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	t.Cleanup(func() {
		model.DB = previousMainDB
		model.LOG_DB = previousLogDB
		common.SetDatabaseTypes(previousMainType, previousLogType)
	})
	require.NoError(t, db.Create(&[]model.User{
		{Id: 1, Username: "root", AffCode: "root-aff", Role: common.RoleRootUser, Status: common.UserStatusEnabled},
		{Id: 7, Username: "target", AffCode: "target-aff", Role: common.RoleCommonUser, Status: common.UserStatusEnabled},
	}).Error)
	base := int64(1_789_000_100)
	require.NoError(t, db.Create(&[]model.Log{
		{UserId: 7, Username: "target", CreatedAt: base, Type: model.LogTypeConsume, Ip: "203.0.113.77", RequestId: "z"},
		{UserId: 7, Username: "target", CreatedAt: base, Type: model.LogTypeConsume, Ip: "203.0.113.77", RequestId: "a"},
	}).Error)
	require.NoError(t, model.BackfillIPSecurityProjection(context.Background(), 10))

	result, err := GetUserSecurityAudit(context.Background(), 1, 7, IPAuditQuery{
		Range: IPAuditRangeCustom, StartTimestamp: base, EndTimestamp: base, Page: 1, PageSize: 10,
	})

	require.NoError(t, err)
	require.Len(t, result.Timeline, 2)
	require.Equal(t, []string{"z", "a"}, []string{result.Timeline[0].RequestID, result.Timeline[1].RequestID})
}

func TestGlobalIPAuditUsesReadyPrimaryProjectionWithoutLogDatabaseRead(t *testing.T) {
	mainDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, mainDB.AutoMigrate(
		&model.User{}, &model.IPSecurityEvent{}, &model.IPAuditRollup{}, &model.IPSecurityProjectionState{},
	))
	logDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	previousMainDB := model.DB
	previousLogDB := model.LOG_DB
	previousMainType := common.MainDatabaseType()
	previousLogType := common.LogDatabaseType()
	model.DB = mainDB
	model.LOG_DB = logDB
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	t.Cleanup(func() {
		model.DB = previousMainDB
		model.LOG_DB = previousLogDB
		common.SetDatabaseTypes(previousMainType, previousLogType)
	})
	require.NoError(t, mainDB.Create(&[]model.User{
		{Id: 1, Username: "root", AffCode: "projection-root", Role: common.RoleRootUser, Status: common.UserStatusEnabled},
		{Id: 7, Username: "target", AffCode: "projection-target", Role: common.RoleCommonUser, Status: common.UserStatusEnabled},
	}).Error)
	require.NoError(t, mainDB.Transaction(func(tx *gorm.DB) error {
		return model.RecordIPSecurityEventWithTx(tx, &model.IPSecurityEvent{
			EventID: "projection-event", UserID: 7, Username: "target", CreatedAt: 1_789_000_200,
			IP: "8.8.8.8", Kind: model.IPSecurityKindAPIRequest, RequestID: "projection-request",
		})
	}))
	require.NoError(t, mainDB.Create(&model.IPSecurityProjectionState{
		ID: 1, Status: model.IPSecurityProjectionStatusReady,
	}).Error)

	page, err := GetGlobalIPAudit(context.Background(), 1, IPAuditQuery{Range: IPAuditRangeAll, Page: 1, PageSize: 1})

	require.NoError(t, err)
	require.Equal(t, 1, page.Total)
	require.Len(t, page.Items, 1)
	require.Equal(t, "8.8.8.8", page.Items[0].IP)
	payload, err := common.Marshal(page.Items[0])
	require.NoError(t, err)
	var item map[string]interface{}
	require.NoError(t, common.Unmarshal(payload, &item))
	require.Equal(t, true, item["can_query_external_profile"])
}

func TestUserSecurityAuditUsesReadyPrimaryProjectionWithoutLogDatabaseRead(t *testing.T) {
	mainDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, mainDB.AutoMigrate(
		&model.User{}, &model.IPSecurityEvent{}, &model.IPAuditRollup{}, &model.IPSecurityProjectionState{},
	))
	logDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	previousMainDB := model.DB
	previousLogDB := model.LOG_DB
	previousMainType := common.MainDatabaseType()
	previousLogType := common.LogDatabaseType()
	model.DB = mainDB
	model.LOG_DB = logDB
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	t.Cleanup(func() {
		model.DB = previousMainDB
		model.LOG_DB = previousLogDB
		common.SetDatabaseTypes(previousMainType, previousLogType)
	})
	require.NoError(t, mainDB.Create(&[]model.User{
		{Id: 1, Username: "root", AffCode: "detail-root", Role: common.RoleRootUser, Status: common.UserStatusEnabled},
		{Id: 7, Username: "target", AffCode: "detail-target", Role: common.RoleCommonUser, Status: common.UserStatusEnabled},
	}).Error)
	require.NoError(t, mainDB.Transaction(func(tx *gorm.DB) error {
		return model.RecordIPSecurityEventWithTx(tx, &model.IPSecurityEvent{
			EventID: "detail-event", UserID: 7, Username: "target", CreatedAt: 1_789_000_300,
			IP: "203.0.113.80", Kind: model.IPSecurityKindLoginSuccess, RequestID: "detail-request",
		})
	}))
	require.NoError(t, mainDB.Create(&model.IPSecurityProjectionState{
		ID: 1, Status: model.IPSecurityProjectionStatusReady,
	}).Error)

	detail, err := GetUserSecurityAudit(context.Background(), 1, 7, IPAuditQuery{Range: IPAuditRangeAll, Page: 1, PageSize: 10})

	require.NoError(t, err)
	require.Equal(t, 1, detail.Summary.LoginSuccessCount)
	require.Equal(t, "203.0.113.80", detail.Summary.RecentIP)
	require.Equal(t, 1, detail.Total)
	require.Len(t, detail.Timeline, 1)
}

func TestIPAuditQueriesReturnAvailableProjectionWhileBackfillRuns(t *testing.T) {
	mainDB, _ := setupAuditOutboxServiceTest(t)
	setIPSecurityEnabledForServiceTest(t, true)
	require.NoError(t, mainDB.Create(&[]model.User{
		{Id: 1, Username: "root", AffCode: "running-root", Role: common.RoleRootUser, Status: common.UserStatusEnabled},
		{Id: 7, Username: "target", AffCode: "running-target", Role: common.RoleCommonUser, Status: common.UserStatusEnabled},
	}).Error)
	require.NoError(t, mainDB.Transaction(func(tx *gorm.DB) error {
		return model.RecordIPSecurityEventWithTx(tx, &model.IPSecurityEvent{
			EventID: "running-event", UserID: 7, Username: "target", CreatedAt: 1_789_000_400,
			IP: "8.8.8.8", Kind: model.IPSecurityKindAPIRequest, RequestID: "running-request",
		})
	}))
	require.NoError(t, mainDB.Create(&model.IPSecurityProjectionState{
		ID: 1, Status: model.IPSecurityProjectionStatusRunning, ProcessedLogs: 1250, ProcessedUsers: 25,
		StartedAt: 1_789_000_000,
	}).Error)

	page, err := GetGlobalIPAudit(context.Background(), 1, IPAuditQuery{
		Range: IPAuditRangeAll, View: IPAuditViewAll, Page: 1, PageSize: 20,
	})
	require.NoError(t, err)
	require.Len(t, page.Items, 1)
	require.False(t, page.Sync.Ready)
	require.Equal(t, model.IPSecurityProjectionStatusRunning, page.Sync.Status)
	require.EqualValues(t, 1250, page.Sync.ProcessedLogs)

	detail, err := GetGlobalIPAuditDetail(context.Background(), 1, "8.8.8.8", IPAuditQuery{Range: IPAuditRangeAll})
	require.NoError(t, err)
	require.Len(t, detail.Timeline, 1)
	require.False(t, detail.Sync.Ready)

	userAudit, err := GetUserSecurityAudit(context.Background(), 1, 7, IPAuditQuery{Range: IPAuditRangeAll})
	require.NoError(t, err)
	require.Equal(t, 1, userAudit.Summary.APIRequestCount)
	require.False(t, userAudit.Sync.Ready)
}

func TestIPAuditQueriesReturnProjectionWhenSyncStatusIsTemporarilyUnavailable(t *testing.T) {
	mainDB, _ := setupAuditOutboxServiceTest(t)
	setIPSecurityEnabledForServiceTest(t, true)
	require.NoError(t, mainDB.Create(&[]model.User{
		{Id: 1, Username: "status-root", AffCode: "status-root-aff", Role: common.RoleRootUser, Status: common.UserStatusEnabled},
		{Id: 7, Username: "status-target", AffCode: "status-target-aff", Role: common.RoleCommonUser, Status: common.UserStatusEnabled},
	}).Error)
	require.NoError(t, mainDB.Transaction(func(tx *gorm.DB) error {
		return model.RecordIPSecurityEventWithTx(tx, &model.IPSecurityEvent{
			EventID: "status-event", UserID: 7, Username: "status-target", CreatedAt: 1_789_000_500,
			IP: "8.8.4.4", Kind: model.IPSecurityKindAPIRequest, RequestID: "status-request",
		})
	}))
	callbackName := "ip_audit_test:fail_projection_status_query"
	require.NoError(t, mainDB.Callback().Query().Before("gorm:query").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Schema != nil && tx.Statement.Schema.Table == "ip_security_projection_states" {
			tx.AddError(errors.New("temporary projection status failure"))
		}
	}))
	t.Cleanup(func() { _ = mainDB.Callback().Query().Remove(callbackName) })

	page, err := GetGlobalIPAudit(context.Background(), 1, IPAuditQuery{
		Range: IPAuditRangeAll, View: IPAuditViewAll, Page: 1, PageSize: 20,
	})
	require.NoError(t, err)
	require.Len(t, page.Items, 1)
	require.Equal(t, "unknown", page.Sync.Status)
	require.False(t, page.Sync.Ready)

	userAudit, err := GetUserSecurityAudit(context.Background(), 1, 7, IPAuditQuery{
		Range: IPAuditRangeAll, Page: 1, PageSize: 20,
	})
	require.NoError(t, err)
	require.Equal(t, 1, userAudit.Summary.APIRequestCount)
	require.Equal(t, "unknown", userAudit.Sync.Status)
	require.False(t, userAudit.Sync.Ready)
}

func TestIPAuditReturnsSuccessfulDisabledStateWithoutProjection(t *testing.T) {
	mainDB, _ := setupAuditOutboxServiceTest(t)
	seedAuditOutboxWalletUser(t, mainDB, 1)
	require.NoError(t, mainDB.Model(&model.User{}).Where("id = ?", 1).Updates(map[string]any{
		"role": common.RoleRootUser,
	}).Error)
	setIPSecurityEnabledForServiceTest(t, false)

	page, err := GetGlobalIPAudit(context.Background(), 1, IPAuditQuery{Range: IPAuditRangeAll, Page: 2, PageSize: 20})
	require.NoError(t, err)
	require.False(t, page.Enabled)
	require.Empty(t, page.Items)
	require.Zero(t, page.Total)
	require.Equal(t, 2, page.Page)
	require.Equal(t, 20, page.PageSize)
}

func TestUserSecurityAuditReturnsSuccessfulDisabledStateWithoutProjection(t *testing.T) {
	mainDB, _ := setupAuditOutboxServiceTest(t)
	require.NoError(t, mainDB.Create(&[]model.User{
		{Id: 1, Username: "disabled-root", AffCode: "disabled-root-aff", Role: common.RoleRootUser, Status: common.UserStatusEnabled},
		{Id: 7, Username: "disabled-target", AffCode: "disabled-target-aff", Role: common.RoleCommonUser, Status: common.UserStatusEnabled},
	}).Error)
	setIPSecurityEnabledForServiceTest(t, false)

	audit, err := GetUserSecurityAudit(context.Background(), 1, 7, IPAuditQuery{Range: IPAuditRange30Days})
	require.NoError(t, err)
	require.False(t, audit.Enabled)
	require.Equal(t, 7, audit.TargetUserID)
	require.Empty(t, audit.Timeline)
	require.Empty(t, audit.RelatedUsers)
	require.Zero(t, audit.Total)
}

func setIPSecurityEnabledForServiceTest(t *testing.T, enabled bool) {
	t.Helper()
	previousOptionMap := common.OptionMap
	common.OptionMapRWMutex.Lock()
	common.OptionMap = map[string]string{
		"ip_security.enabled":                     common.Interface2String(enabled),
		"ip_security.retention_days":              "90",
		"ip_security.projection_interval_seconds": "60",
		"ip_security.projection_batch_size":       "500",
	}
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = previousOptionMap
		common.OptionMapRWMutex.Unlock()
	})
}
