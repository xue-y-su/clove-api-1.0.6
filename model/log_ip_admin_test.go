package model

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func openLogIPTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	gin.SetMode(gin.TestMode)
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	common.RedisEnabled = false
	common.LogConsumeEnabled = true
	previousDB := DB
	previousLogDB := LOG_DB

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)

	DB = db
	LOG_DB = db
	require.NoError(t, db.AutoMigrate(&User{}, &Log{}, &IPSecurityEvent{}, &IPAuditRollup{}, &IPSecurityProjectionState{}))

	t.Cleanup(func() {
		DB = previousDB
		LOG_DB = previousLogDB
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func seedLogIPTestUser(t *testing.T, db *gorm.DB, id int, username string, recordIP bool) {
	t.Helper()

	user := &User{
		Id:       id,
		Username: username,
		Password: "password",
		AffCode:  fmt.Sprintf("aff-%d", id),
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
	}
	user.SetSetting(dto.UserSetting{RecordIpLog: recordIP})
	require.NoError(t, db.Create(user).Error)
}

func newLogIPTestContext(method string, target string, remoteAddr string) *gin.Context {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(method, target, nil)
	ctx.Request.RemoteAddr = remoteAddr
	return ctx
}

func TestRecordConsumeLogAlwaysStoresClientIPWhenUserSettingDisablesIPLog(t *testing.T) {
	db := openLogIPTestDB(t)
	seedLogIPTestUser(t, db, 10, "ip-disabled-user", false)

	ctx := newLogIPTestContext(http.MethodPost, "/v1/chat/completions", "203.0.113.50:12345")
	ctx.Set("username", "ip-disabled-user")

	RecordConsumeLog(ctx, 10, RecordConsumeLogParams{
		ModelName:      "gpt-test",
		TokenName:      "token-a",
		Quota:          42,
		Content:        "consume",
		UseTimeSeconds: 1,
	})

	var log Log
	require.NoError(t, db.First(&log, "user_id = ? AND type = ?", 10, LogTypeConsume).Error)
	require.Equal(t, "203.0.113.50", log.Ip)
}

func TestRecordConsumeLogPersistsAfterRequestContextIsCanceled(t *testing.T) {
	db := openLogIPTestDB(t)
	seedLogIPTestUser(t, db, 22, "completed-request-user", true)

	requestContext, cancel := context.WithCancel(context.Background())
	ctx := newLogIPTestContext(http.MethodPost, "/v1/images/generations", "203.0.113.54:12345")
	ctx.Request = ctx.Request.WithContext(requestContext)
	ctx.Set("username", "completed-request-user")
	ctx.Set(common.RequestIdKey, "completed-request-id")
	cancel()

	RecordConsumeLog(ctx, 22, RecordConsumeLogParams{
		ChannelId:      51,
		ModelName:      "gpt-image-2",
		TokenName:      "playground-Select",
		Quota:          250000,
		Content:        "image generation",
		UseTimeSeconds: 170,
		Group:          "Select",
	})

	var log Log
	require.NoError(t, db.First(&log, "user_id = ? AND type = ?", 22, LogTypeConsume).Error)
	require.Equal(t, "completed-request-id", log.RequestId)
	require.Equal(t, "203.0.113.54", log.Ip)
	require.Equal(t, "gpt-image-2", log.ModelName)
	require.Equal(t, 250000, log.Quota)

	var event IPSecurityEvent
	require.NoError(t, db.First(&event, "user_id = ?", 22).Error)
	require.Equal(t, "completed-request-id", event.RequestID)
}

func TestRecordConsumeLogProjectsMinimalSecurityEventWhenFullLogsDisabled(t *testing.T) {
	db := openLogIPTestDB(t)
	seedLogIPTestUser(t, db, 20, "security-only-user", false)
	common.LogConsumeEnabled = false
	t.Cleanup(func() { common.LogConsumeEnabled = true })
	ctx := newLogIPTestContext(http.MethodPost, "/v1/chat/completions", "203.0.113.52:12345")
	ctx.Set("username", "security-only-user")
	ctx.Set(common.RequestIdKey, "security-request-id")

	RecordConsumeLog(ctx, 20, RecordConsumeLogParams{
		ModelName: "must-not-persist", TokenName: "must-not-persist", TokenId: 99,
		Quota: 42, Content: "must-not-persist", Other: map[string]interface{}{"secret": "must-not-persist"},
	})

	var logCount int64
	require.NoError(t, db.Model(&Log{}).Where("user_id = ?", 20).Count(&logCount).Error)
	require.Zero(t, logCount)
	var event IPSecurityEvent
	require.NoError(t, db.First(&event, "user_id = ?", 20).Error)
	require.Equal(t, IPSecurityKindAPIRequest, event.Kind)
	require.Equal(t, "203.0.113.52", event.IP)
	require.Equal(t, "security-request-id", event.RequestID)
	require.Equal(t, "security-only-user", event.Username)
	require.Zero(t, event.Quota)
	var lifetime IPAuditRollup
	require.NoError(t, db.First(&lifetime, "bucket = ? AND user_id = ?", 0, 20).Error)
	require.Equal(t, 1, lifetime.APIRequestCount)
	require.Equal(t, 1, lifetime.RequestCount)
}

func TestRecordConsumeLogProjectsOneSecurityEventPerRequestWhenFullLogsEnabled(t *testing.T) {
	db := openLogIPTestDB(t)
	seedLogIPTestUser(t, db, 21, "full-log-user", true)
	ctx := newLogIPTestContext(http.MethodPost, "/v1/chat/completions", "203.0.113.53:12345")
	ctx.Set("username", "full-log-user")
	ctx.Set(common.RequestIdKey, "full-log-request-id")
	params := RecordConsumeLogParams{ModelName: "gpt-test", TokenName: "work", TokenId: 5, Quota: 17, Content: "consume"}

	RecordConsumeLog(ctx, 21, params)
	RecordConsumeLog(ctx, 21, params)

	var logCount int64
	require.NoError(t, db.Model(&Log{}).Where("user_id = ? AND type = ?", 21, LogTypeConsume).Count(&logCount).Error)
	require.EqualValues(t, 2, logCount)
	var eventCount int64
	require.NoError(t, db.Model(&IPSecurityEvent{}).Where("user_id = ?", 21).Count(&eventCount).Error)
	require.EqualValues(t, 1, eventCount)
	var lifetime IPAuditRollup
	require.NoError(t, db.First(&lifetime, "bucket = ? AND user_id = ?", 0, 21).Error)
	require.Equal(t, 1, lifetime.APIRequestCount)
	require.Equal(t, 17, lifetime.Quota)
}

func TestRecordErrorLogAlwaysStoresClientIPWhenUserSettingDisablesIPLog(t *testing.T) {
	db := openLogIPTestDB(t)
	seedLogIPTestUser(t, db, 11, "error-ip-disabled-user", false)

	ctx := newLogIPTestContext(http.MethodPost, "/v1/chat/completions", "203.0.113.51:12345")
	ctx.Set("username", "error-ip-disabled-user")

	RecordErrorLog(ctx, 11, 0, "gpt-test", "token-a", "upstream error", 0, 2, false, "default", nil)

	var log Log
	require.NoError(t, db.First(&log, "user_id = ? AND type = ?", 11, LogTypeError).Error)
	require.Equal(t, "203.0.113.51", log.Ip)
}

func TestLogWritersPreserveIPv4MappedIPv6AddressFamily(t *testing.T) {
	db := openLogIPTestDB(t)
	seedLogIPTestUser(t, db, 12, "mapped-writer-user", true)
	ctx := newLogIPTestContext(http.MethodPost, "/v1/chat/completions", "127.0.0.1:12345")
	ctx.Request.Header.Set("X-Forwarded-For", "::ffff:203.0.113.57")
	ctx.Set("username", "mapped-writer-user")

	RecordConsumeLog(ctx, 12, RecordConsumeLogParams{ModelName: "gpt-test", Content: "consume"})
	RecordErrorLog(ctx, 12, 0, "gpt-test", "token-a", "upstream error", 0, 1, false, "default", nil)
	RecordLoginLog(12, "mapped-writer-user", "login", "::ffff:203.0.113.57", "user.login_success", nil, nil)

	var logs []Log
	require.NoError(t, db.Where("user_id = ?", 12).Order("id asc").Find(&logs).Error)
	require.Len(t, logs, 3)
	for _, log := range logs {
		require.Equal(t, "::ffff:203.0.113.57", log.Ip)
	}
}

func TestQueryUserLogsStripsIPFromNonAdminResults(t *testing.T) {
	db := openLogIPTestDB(t)
	seedLogIPTestUser(t, db, 12, "owner", true)
	require.NoError(t, db.Create(&Log{
		UserId:    12,
		Username:  "owner",
		CreatedAt: 100,
		Type:      LogTypeConsume,
		Content:   "consume",
		Ip:        "203.0.113.52",
	}).Error)

	logs, total, err := GetUserLogs(12, LogTypeConsume, 0, 0, "", "", 0, 10, "", "", "")
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, logs, 1)
	require.Empty(t, logs[0].Ip)
}

func TestTokenLogQueryStripsIPFromNonAdminResults(t *testing.T) {
	db := openLogIPTestDB(t)
	seedLogIPTestUser(t, db, 16, "token-owner", true)
	require.NoError(t, db.Create(&Log{
		UserId:    16,
		Username:  "token-owner",
		CreatedAt: 100,
		Type:      LogTypeConsume,
		Content:   "consume",
		TokenId:   88,
		Ip:        "203.0.113.56",
	}).Error)

	logs, err := GetLogByTokenId(88)
	require.NoError(t, err)
	require.Len(t, logs, 1)
	require.Empty(t, logs[0].Ip)
}

func TestRecordLogStoresOptionalCallerIPAndUserQueriesStripIt(t *testing.T) {
	db := openLogIPTestDB(t)
	seedLogIPTestUser(t, db, 15, "system-owner", true)

	RecordLog(15, LogTypeSystem, "system event", "203.0.113.55")

	var stored Log
	require.NoError(t, db.First(&stored, "user_id = ? AND type = ?", 15, LogTypeSystem).Error)
	require.Equal(t, "203.0.113.55", stored.Ip)

	logs, total, err := GetUserLogs(15, LogTypeSystem, 0, 0, "", "", 0, 10, "", "", "")
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, logs, 1)
	require.Empty(t, logs[0].Ip)
}

func TestNormalizeLogIPMapsLoopbackAndIPv4MappedAddresses(t *testing.T) {
	require.Equal(t, "127.0.0.1", NormalizeLogIP("::1"))
	require.Equal(t, "127.0.0.1", NormalizeLogIP("[::1]:12345"))
	require.Equal(t, "127.0.0.1", NormalizeLogIP("::ffff:127.0.0.1"))
	require.Equal(t, "203.0.113.57", NormalizeLogIP("::ffff:203.0.113.57"))
}

func TestRecordOperationAuditLogPreservesIPv6LoopbackIP(t *testing.T) {
	db := openLogIPTestDB(t)
	seedLogIPTestUser(t, db, 17, "audit-owner", true)

	RecordOperationAuditLog(17, "audit", "::1", "user.test", nil, nil, nil)

	var log Log
	require.NoError(t, db.First(&log, "user_id = ? AND type = ?", 17, LogTypeManage).Error)
	require.Equal(t, "::1", log.Ip)
}

func TestBillingLogsIncludeStructuredBusinessEvent(t *testing.T) {
	db := openLogIPTestDB(t)
	seedLogIPTestUser(t, db, 19, "billing-owner", true)

	RecordTopupLog(19, "manual topup", "203.0.113.60", "manual", "admin")

	ctx := newLogIPTestContext(http.MethodPost, "/v1/chat/completions", "203.0.113.61:12345")
	ctx.Set("username", "billing-owner")
	RecordConsumeLog(ctx, 19, RecordConsumeLogParams{
		ModelName: "gpt-test",
		TokenName: "work-token",
		Quota:     42,
		Content:   "consume",
		TokenId:   9,
		Other:     map[string]interface{}{"request_path": "/v1/chat/completions"},
	})

	RecordTaskBillingLog(RecordTaskBillingLogParams{
		UserId:    19,
		LogType:   LogTypeRefund,
		Content:   "refund",
		ModelName: "image-model",
		Quota:     7,
		Other:     map[string]interface{}{"task_id": "task-1"},
	})

	var logs []Log
	require.NoError(t, db.Order("type asc").Find(&logs, "user_id = ?", 19).Error)
	require.Len(t, logs, 3)

	actions := map[int]string{}
	for _, log := range logs {
		var other map[string]interface{}
		require.NoError(t, common.Unmarshal([]byte(log.Other), &other))
		op, ok := other["op"].(map[string]interface{})
		require.True(t, ok, "log type %d should include op metadata", log.Type)
		actions[log.Type], _ = op["action"].(string)
	}

	require.Equal(t, "billing.topup", actions[LogTypeTopup])
	require.Equal(t, "billing.consume", actions[LogTypeConsume])
	require.Equal(t, "billing.refund", actions[LogTypeRefund])
}

func TestUserSettingsForceRecordIPLogOnRead(t *testing.T) {
	user := &User{}
	user.SetSetting(dto.UserSetting{RecordIpLog: false})

	require.True(t, user.GetSetting().RecordIpLog)
	require.True(t, user.ToBaseUser().GetSetting().RecordIpLog)
}

func TestAdminLogQueryAndStatsFilterByIP(t *testing.T) {
	db := openLogIPTestDB(t)
	seedLogIPTestUser(t, db, 13, "first", true)
	seedLogIPTestUser(t, db, 14, "second", true)
	now := common.GetTimestamp()
	require.NoError(t, db.Create(&[]Log{
		{UserId: 13, Username: "first", CreatedAt: now, Type: LogTypeConsume, Content: "first", Quota: 7, Ip: "203.0.113.53"},
		{UserId: 14, Username: "second", CreatedAt: now + 1, Type: LogTypeConsume, Content: "second", Quota: 11, Ip: "203.0.113.54"},
	}).Error)

	logs, total, err := GetAllLogsWithIP(LogTypeConsume, 0, 0, "", "", "", "203.0.113.54", 0, 10, 0, "", "", "")
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, logs, 1)
	require.Equal(t, "second", logs[0].Username)
	require.Equal(t, "203.0.113.54", logs[0].Ip)

	stat, err := SumUsedQuotaWithIP(LogTypeConsume, 0, 0, "", "", "", "203.0.113.54", 0, "")
	require.NoError(t, err)
	require.Equal(t, 11, stat.Quota)
	require.Equal(t, 1, stat.Rpm)
}

func TestAdminLogQueryNormalizesLegacyLoopbackIPDisplayAndFilter(t *testing.T) {
	db := openLogIPTestDB(t)
	seedLogIPTestUser(t, db, 18, "legacy-loopback", true)
	now := common.GetTimestamp()
	require.NoError(t, db.Create(&Log{
		UserId:    18,
		Username:  "legacy-loopback",
		CreatedAt: now,
		Type:      LogTypeConsume,
		Content:   "consume",
		Quota:     13,
		Ip:        "::1",
	}).Error)

	logs, total, err := GetAllLogsWithIP(LogTypeConsume, 0, 0, "", "", "", "127.0.0.1", 0, 10, 0, "", "", "")
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, logs, 1)
	require.Equal(t, "::1", logs[0].Ip)

	stat, err := SumUsedQuotaWithIP(LogTypeConsume, 0, 0, "", "", "", "127.0.0.1", 0, "")
	require.NoError(t, err)
	require.Equal(t, 13, stat.Quota)
	require.Equal(t, 1, stat.Rpm)
}

func TestIPAuditSummaryMasksIPForAdminAndFiltersDateRange(t *testing.T) {
	db := openLogIPTestDB(t)
	seedLogIPTestUser(t, db, 21, "alice", true)
	seedLogIPTestUser(t, db, 22, "bob", true)
	base := time.Date(2026, 7, 14, 10, 0, 0, 0, time.UTC).Unix()
	require.NoError(t, db.Create(&[]Log{
		{UserId: 21, Username: "alice", CreatedAt: base - 86400, Type: LogTypeConsume, Content: "before", Quota: 1, Ip: "203.0.113.9"},
		{UserId: 21, Username: "alice", CreatedAt: base, Type: LogTypeConsume, Content: "first", Quota: 7, Ip: "203.0.113.9"},
		{UserId: 22, Username: "bob", CreatedAt: base + 60, Type: LogTypeError, Content: "second", Quota: 0, Ip: "203.0.113.9"},
		{UserId: 22, Username: "bob", CreatedAt: base + 120, Type: LogTypeConsume, Content: "other", Quota: 11, Ip: "198.51.100.88"},
	}).Error)

	items, err := GetIPAuditItems(common.RoleAdminUser, base, base+120, 10)
	require.NoError(t, err)
	require.Len(t, items, 2)
	require.Equal(t, "203.0.113.*", items[0].IP)
	require.Empty(t, items[0].Users)
	require.Empty(t, items[0].Timeline)
	require.Equal(t, 2, items[0].RequestCount)
	require.Equal(t, 7, items[0].Quota)
	require.Equal(t, int64(base), items[0].FirstSeenAt)
	require.Equal(t, int64(base+60), items[0].LastSeenAt)
}

func TestIPAuditSummaryShowsFullIPUsersAndTimelineForRoot(t *testing.T) {
	db := openLogIPTestDB(t)
	seedLogIPTestUser(t, db, 31, "root-alice", true)
	seedLogIPTestUser(t, db, 32, "root-bob", true)
	base := time.Date(2026, 7, 14, 11, 0, 0, 0, time.UTC).Unix()
	require.NoError(t, db.Create(&[]Log{
		{UserId: 31, Username: "root-alice", CreatedAt: base, Type: LogTypeConsume, Content: "first", Quota: 3, Ip: "2001:db8::1"},
		{UserId: 32, Username: "root-bob", CreatedAt: base + 60, Type: LogTypeLogin, Content: "login", Quota: 0, Ip: "2001:db8::1"},
	}).Error)

	items, err := GetIPAuditItems(common.RoleRootUser, base, base+60, 10)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, "2001:db8::1", items[0].IP)
	require.ElementsMatch(t, []IPAuditUser{{UserId: 31, Username: "root-alice"}, {UserId: 32, Username: "root-bob"}}, items[0].Users)
	require.Len(t, items[0].Timeline, 2)
	require.Equal(t, int64(base+60), items[0].Timeline[0].CreatedAt)
	require.Equal(t, "root-bob", items[0].Timeline[0].Username)
	require.Equal(t, LogTypeLogin, items[0].Timeline[0].Type)
}

func TestIPAuditAdminDeduplicatesUsersAcrossMaskedAddressesAndCountsEventKinds(t *testing.T) {
	db := openLogIPTestDB(t)
	seedLogIPTestUser(t, db, 41, "same-user", true)
	seedLogIPTestUser(t, db, 42, "other-user", true)
	base := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC).Unix()
	require.NoError(t, db.Create(&[]Log{
		{UserId: 41, Username: "same-user", CreatedAt: base, Type: LogTypeConsume, Content: "request body must not leak", Ip: "203.0.113.10"},
		{UserId: 41, Username: "same-user", CreatedAt: base + 10, Type: LogTypeError, Content: "upstream secret must not leak", Ip: "203.0.113.20"},
		{UserId: 42, Username: "other-user", CreatedAt: base + 20, Type: LogTypeLogin, Content: "user.login_failed", Ip: "203.0.113.30"},
		{UserId: 42, Username: "other-user", CreatedAt: base + 30, Type: LogTypeLogin, Content: "user.login_succeeded", Ip: "203.0.113.30"},
	}).Error)

	items, err := GetIPAuditItems(common.RoleAdminUser, base, base+30, 10)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, "203.0.113.*", items[0].IP)
	require.Equal(t, 2, items[0].UserCount)
	require.Equal(t, 1, items[0].APIRequestCount)
	require.Equal(t, 1, items[0].ErrorCount)
	require.Equal(t, 1, items[0].LoginFailureCount)
	require.Equal(t, 1, items[0].LoginSuccessCount)
	require.Equal(t, 2, items[0].AnomalyCount)
}

func TestIPAuditRootTimelineExposesStableKindWithoutRawContent(t *testing.T) {
	db := openLogIPTestDB(t)
	seedLogIPTestUser(t, db, 51, "timeline-user", true)
	base := time.Date(2026, 7, 14, 13, 0, 0, 0, time.UTC).Unix()
	require.NoError(t, db.Create(&Log{
		UserId: 51, Username: "timeline-user", CreatedAt: base, Type: LogTypeError,
		Content: "authorization=Bearer complete-secret", Ip: "2001:db8::51",
	}).Error)

	items, err := GetIPAuditItems(common.RoleRootUser, base, base, 10)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Len(t, items[0].Timeline, 1)
	require.Equal(t, "api_error", items[0].Timeline[0].Kind)
	payload, err := json.Marshal(items[0])
	require.NoError(t, err)
	require.NotContains(t, string(payload), "complete-secret")
}

func TestIPAuditKeepsIPv4MappedIPv6SeparateFromIPv4(t *testing.T) {
	db := openLogIPTestDB(t)
	seedLogIPTestUser(t, db, 61, "ipv4-user", true)
	seedLogIPTestUser(t, db, 62, "mapped-user", true)
	base := time.Date(2026, 7, 14, 14, 0, 0, 0, time.UTC).Unix()
	require.NoError(t, db.Create(&[]Log{
		{UserId: 61, Username: "ipv4-user", CreatedAt: base, Type: LogTypeConsume, Ip: "203.0.113.57"},
		{UserId: 62, Username: "mapped-user", CreatedAt: base + 1, Type: LogTypeConsume, Ip: "::ffff:203.0.113.57"},
	}).Error)

	rootItems, err := GetIPAuditItems(common.RoleRootUser, base, base+1, 10)
	require.NoError(t, err)
	require.Len(t, rootItems, 2)
	require.ElementsMatch(t, []string{"203.0.113.57", "::ffff:203.0.113.57"}, []string{rootItems[0].IP, rootItems[1].IP})

	adminItems, err := GetIPAuditItems(common.RoleAdminUser, base, base+1, 10)
	require.NoError(t, err)
	require.Len(t, adminItems, 2)
	require.ElementsMatch(t, []string{"203.0.113.*", "::ffff:203.0.113.*"}, []string{adminItems[0].IP, adminItems[1].IP})
}

func TestIPAuditIncludesRegistrationFactsWithoutLogEvents(t *testing.T) {
	db := openLogIPTestDB(t)
	seedLogIPTestUser(t, db, 71, "registration-only", true)
	base := time.Date(2026, 7, 14, 15, 0, 0, 0, time.UTC).Unix()
	require.NoError(t, db.Model(&User{}).Where("id = ?", 71).Updates(map[string]any{
		"registration_ip": "2001:db8::71",
		"created_at":      base,
	}).Error)

	rootItems, err := GetIPAuditItems(common.RoleRootUser, base-1, base+1, 10)
	require.NoError(t, err)
	require.Len(t, rootItems, 1)
	require.Equal(t, "2001:db8::71", rootItems[0].IP)
	require.Equal(t, 1, rootItems[0].RegistrationCount)
	require.Equal(t, 1, rootItems[0].UserCount)
	require.Equal(t, []IPAuditUser{{UserId: 71, Username: "registration-only"}}, rootItems[0].Users)
	require.Len(t, rootItems[0].Timeline, 1)
	require.Equal(t, "registration", rootItems[0].Timeline[0].Kind)
	require.Equal(t, base, rootItems[0].Timeline[0].CreatedAt)

	adminItems, err := GetIPAuditItems(common.RoleAdminUser, base-1, base+1, 10)
	require.NoError(t, err)
	require.Len(t, adminItems, 1)
	require.Equal(t, "2001:db8:*", adminItems[0].IP)
	require.Equal(t, 1, adminItems[0].RegistrationCount)
	require.Equal(t, 1, adminItems[0].UserCount)
	require.Empty(t, adminItems[0].Users)
	require.Empty(t, adminItems[0].Timeline)
}

func TestIPAuditDoesNotDoubleCountRegistrationAuditAndSeparatesLogout(t *testing.T) {
	db := openLogIPTestDB(t)
	seedLogIPTestUser(t, db, 72, "security-events", true)
	base := time.Date(2026, 7, 14, 15, 30, 0, 0, time.UTC).Unix()
	require.NoError(t, db.Model(&User{}).Where("id = ?", 72).Updates(map[string]any{
		"registration_ip": "203.0.113.72", "created_at": base,
	}).Error)
	require.NoError(t, db.Create(&[]Log{
		{UserId: 72, Username: "security-events", CreatedAt: base, Type: LogTypeLogin, Content: "user.register", Ip: "203.0.113.72"},
		{UserId: 72, Username: "security-events", CreatedAt: base + 1, Type: LogTypeLogin, Content: "user.logout", Ip: "203.0.113.72"},
	}).Error)

	items, err := GetIPAuditItems(common.RoleRootUser, base, base+1, 10)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, 1, items[0].RegistrationCount)
	require.Equal(t, 1, items[0].LogoutCount)
	require.Zero(t, items[0].LoginSuccessCount)
	require.Len(t, items[0].Timeline, 2)
	require.Equal(t, []string{"logout", "registration"}, []string{items[0].Timeline[0].Kind, items[0].Timeline[1].Kind})
}

func TestIPAuditPaginationTotalIsNotTruncatedAtTenThousand(t *testing.T) {
	db := openLogIPTestDB(t)
	base := time.Date(2026, 7, 14, 16, 0, 0, 0, time.UTC).Unix()
	logs := make([]Log, 10_001)
	for index := range logs {
		logs[index] = Log{
			UserId: 1, Username: "large-audit", CreatedAt: base,
			Type: LogTypeConsume, Ip: fmt.Sprintf("2001:db8::%x", index+1),
		}
	}
	require.NoError(t, db.CreateInBatches(&logs, 500).Error)

	items, total, err := GetIPAuditPage(common.RoleRootUser, base, base, 1, 25)
	require.NoError(t, err)
	require.Equal(t, 10_001, total)
	require.Len(t, items, 25)
}

func TestIPAuditPropagatesLogAndRegistrationQueryErrors(t *testing.T) {
	t.Run("log aggregation", func(t *testing.T) {
		db := openLogIPTestDB(t)
		require.NoError(t, db.Migrator().DropTable(&Log{}))

		_, _, err := GetIPAuditPage(common.RoleRootUser, 0, 0, 1, 10)
		require.Error(t, err)
	})

	t.Run("registration facts", func(t *testing.T) {
		db := openLogIPTestDB(t)
		require.NoError(t, db.Migrator().DropTable(&User{}))

		_, _, err := GetIPAuditPage(common.RoleRootUser, 0, 0, 1, 10)
		require.Error(t, err)
	})
}
