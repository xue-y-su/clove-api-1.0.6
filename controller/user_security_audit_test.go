package controller

import (
	"errors"
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type userSecurityAuditData struct {
	TargetUserID int    `json:"target_user_id"`
	Username     string `json:"username"`
	Projection   string `json:"projection"`
	IsRoot       bool   `json:"is_root"`
	ProxyWarning string `json:"proxy_warning"`
	Summary      struct {
		RegistrationIP    string `json:"registration_ip"`
		RecentIP          string `json:"recent_ip"`
		APIRequestCount   int    `json:"api_request_count"`
		ErrorCount        int    `json:"error_count"`
		LoginSuccessCount int    `json:"login_success_count"`
		LoginFailureCount int    `json:"login_failure_count"`
		LogoutCount       int    `json:"logout_count"`
		AnomalyCount      int    `json:"anomaly_count"`
		RelatedUserCount  int    `json:"related_user_count"`
		LastActivityAt    int64  `json:"last_activity_at"`
	} `json:"summary"`
	RelatedUsers []struct {
		UserID   int    `json:"user_id"`
		Username string `json:"username"`
	} `json:"related_users"`
	Timeline []struct {
		IP   string `json:"ip"`
		Kind string `json:"kind"`
	} `json:"timeline"`
	Total    int `json:"total"`
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
}

func TestParseIPAuditQueryParsesIndependentRelatedPagination(t *testing.T) {
	ctx, _ := newAuthenticatedContext(
		t,
		http.MethodGet,
		"/api/user/10/security-audit?range=all&view=shared_api&p=2&page_size=5&related_page=3&related_page_size=25&user_page=4&user_page_size=30&timeline_page=5&timeline_page_size=40",
		nil,
		1,
	)

	query, err := parseIPAuditQuery(ctx)
	require.NoError(t, err)
	require.Equal(t, "shared_api", query.View)
	require.Equal(t, 2, query.Page)
	require.Equal(t, 5, query.PageSize)
	require.Equal(t, 3, query.RelatedPage)
	require.Equal(t, 25, query.RelatedPageSize)
	require.Equal(t, 4, query.DetailUserPage)
	require.Equal(t, 30, query.DetailUserPageSize)
	require.Equal(t, 5, query.TimelinePage)
	require.Equal(t, 40, query.TimelinePageSize)
}

func TestRootUserSecurityAuditReturnsFullProjectionWithoutAuditingRead(t *testing.T) {
	setupUserRootBindingControllerTest(t)
	t.Setenv("TRUSTED_PROXIES", "127.0.0.1")
	seedUserRootBindingUser(t, 1, "primary-root", common.RoleRootUser)
	seedUserRootBindingUser(t, 10, "target-user", common.RoleCommonUser)
	seedUserRootBindingUser(t, 11, "related-user", common.RoleCommonUser)
	now := common.GetTimestamp()
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", 10).Updates(map[string]any{
		"registration_ip": "203.0.113.10", "created_at": now - 120,
	}).Error)
	require.NoError(t, model.LOG_DB.Create(&[]model.Log{
		{UserId: 10, Username: "target-user", Type: model.LogTypeConsume, CreatedAt: now - 90, Ip: "203.0.113.10"},
		{UserId: 10, Username: "target-user", Type: model.LogTypeError, CreatedAt: now - 60, Ip: "198.51.100.20", Other: `{"status_code":401}`},
		{UserId: 10, Username: "target-user", Type: model.LogTypeLogin, CreatedAt: now - 30, Ip: "198.51.100.20"},
		{UserId: 11, Username: "related-user", Type: model.LogTypeConsume, CreatedAt: now - 20, Ip: "203.0.113.10"},
	}).Error)
	backfillIPSecurityProjectionForControllerTest(t)

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/user/10/security-audit?range=all&p=1&page_size=2", nil, 1)
	ctx.Set("username", "primary-root")
	ctx.Set("role", common.RoleRootUser)
	ctx.Params = gin.Params{{Key: "id", Value: "10"}}
	ctx.Request.RemoteAddr = "192.0.2.90:4050"

	GetUserSecurityAudit(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	var data userSecurityAuditData
	require.NoError(t, common.Unmarshal(response.Data, &data))
	require.Equal(t, 10, data.TargetUserID)
	require.Equal(t, "target-user", data.Username)
	require.Equal(t, "full", data.Projection)
	require.True(t, data.IsRoot)
	require.Equal(t, "203.0.113.10", data.Summary.RegistrationIP)
	require.Equal(t, "198.51.100.20", data.Summary.RecentIP)
	require.Equal(t, 1, data.Summary.APIRequestCount)
	require.Equal(t, 1, data.Summary.ErrorCount)
	require.Equal(t, 1, data.Summary.LoginSuccessCount)
	require.Equal(t, 1, data.Summary.RelatedUserCount)
	require.Len(t, data.RelatedUsers, 1)
	require.Equal(t, "related-user", data.RelatedUsers[0].Username)
	require.Equal(t, 4, data.Total)
	require.Len(t, data.Timeline, 2)
	require.Equal(t, 1, data.Page)
	require.Equal(t, 2, data.PageSize)
	require.Empty(t, data.ProxyWarning)

	var count int64
	require.NoError(t, model.DB.Model(&model.AuditOutboxEvent{}).Where("event_type = ?", "user.security_audit_read").Count(&count).Error)
	require.Zero(t, count)
}

func TestAdminUserSecurityAuditMasksDataAndRejectsPeerAdmin(t *testing.T) {
	setupUserRootBindingControllerTest(t)
	t.Setenv("TRUSTED_PROXIES", "")
	seedUserRootBindingUser(t, 2, "admin", common.RoleAdminUser)
	seedUserRootBindingUser(t, 10, "target-user", common.RoleCommonUser)
	seedUserRootBindingUser(t, 12, "peer-admin", common.RoleAdminUser)
	now := common.GetTimestamp()
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", 10).Update("registration_ip", "203.0.113.10").Error)
	require.NoError(t, model.LOG_DB.Create(&[]model.Log{
		{UserId: 10, Username: "target-user", Type: model.LogTypeConsume, CreatedAt: now - 30, Ip: "203.0.113.10"},
		{UserId: 10, Username: "target-user", Type: model.LogTypeError, CreatedAt: now - 20, Ip: "203.0.113.10", Other: `{"status_code":401}`},
		{UserId: 10, Username: "target-user", Type: model.LogTypeLogin, Content: "user.login_failed", CreatedAt: now - 10, Ip: "203.0.113.10"},
	}).Error)
	backfillIPSecurityProjectionForControllerTest(t)

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/user/10/security-audit", nil, 2)
	ctx.Set("username", "admin")
	ctx.Set("role", common.RoleAdminUser)
	ctx.Params = gin.Params{{Key: "id", Value: "10"}}
	GetUserSecurityAudit(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	var data userSecurityAuditData
	require.NoError(t, common.Unmarshal(response.Data, &data))
	require.Equal(t, "masked", data.Projection)
	require.False(t, data.IsRoot)
	require.Equal(t, "203.0.113.*", data.Summary.RegistrationIP)
	require.Equal(t, "203.0.113.*", data.Summary.RecentIP)
	require.Equal(t, "trusted_proxies_not_configured", data.ProxyWarning)
	require.Equal(t, 1, data.Summary.APIRequestCount)
	require.Equal(t, 1, data.Summary.ErrorCount)
	require.Equal(t, 1, data.Summary.LoginFailureCount)
	require.Equal(t, 2, data.Summary.AnomalyCount)
	require.Empty(t, data.RelatedUsers)
	require.Empty(t, data.Timeline)

	peerCtx, peerRecorder := newAuthenticatedContext(t, http.MethodGet, "/api/user/12/security-audit", nil, 2)
	peerCtx.Set("username", "admin")
	peerCtx.Set("role", common.RoleAdminUser)
	peerCtx.Params = gin.Params{{Key: "id", Value: "12"}}
	GetUserSecurityAudit(peerCtx)
	require.False(t, decodeAPIResponse(t, peerRecorder).Success)
}

func TestUserSecurityAuditSucceedsWhenAuditOutboxIsUnavailable(t *testing.T) {
	setupUserRootBindingControllerTest(t)
	seedUserRootBindingUser(t, 1, "primary-root", common.RoleRootUser)
	seedUserRootBindingUser(t, 10, "target-user", common.RoleCommonUser)
	require.NoError(t, model.LOG_DB.Create(&model.Log{
		UserId: 10, Username: "target-user", Type: model.LogTypeLogin,
		CreatedAt: common.GetTimestamp(), Ip: "203.0.113.10",
	}).Error)
	backfillIPSecurityProjectionForControllerTest(t)
	failure := errors.New("outbox unavailable")
	require.NoError(t, model.DB.Callback().Create().Before("gorm:create").Register("user_security_audit_test:fail_outbox", func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Schema != nil && tx.Statement.Schema.Table == "audit_outbox_events" {
			tx.AddError(failure)
		}
	}))
	t.Cleanup(func() { _ = model.DB.Callback().Create().Remove("user_security_audit_test:fail_outbox") })
	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/user/10/security-audit", nil, 1)
	ctx.Set("username", "primary-root")
	ctx.Set("role", common.RoleRootUser)
	ctx.Params = gin.Params{{Key: "id", Value: "10"}}

	GetUserSecurityAudit(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	require.NotEmpty(t, response.Data)
}

func TestUserSecurityAuditRejectsUnauthorizedTargetBeforeLogDatabaseQuery(t *testing.T) {
	setupUserRootBindingControllerTest(t)
	seedUserRootBindingUser(t, 2, "admin", common.RoleAdminUser)
	seedUserRootBindingUser(t, 12, "peer-admin", common.RoleAdminUser)
	logDB, err := gorm.Open(sqlite.Open("file:user-security-auth-order?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, logDB.AutoMigrate(&model.Log{}))
	queryCount := 0
	require.NoError(t, logDB.Callback().Query().Before("gorm:query").Register("user_security_audit_test:count_queries", func(*gorm.DB) {
		queryCount++
	}))
	model.LOG_DB = logDB
	t.Cleanup(func() {
		if sqlDB, dbErr := logDB.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})
	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/user/12/security-audit", nil, 2)
	ctx.Set("username", "admin")
	ctx.Set("role", common.RoleAdminUser)
	ctx.Params = gin.Params{{Key: "id", Value: "12"}}

	GetUserSecurityAudit(ctx)

	require.False(t, decodeAPIResponse(t, recorder).Success)
	require.Zero(t, queryCount)
}

func TestUserSecurityAuditExcludesRelatedAccountsOutsideActorScope(t *testing.T) {
	setupUserRootBindingControllerTest(t)
	seedUserRootBindingUser(t, 2, "secondary-root", common.RoleRootUser)
	seedUserRootBindingUser(t, 3, "peer-root", common.RoleRootUser)
	seedUserRootBindingUser(t, 10, "target-user", common.RoleCommonUser)
	seedUserRootBindingUser(t, 11, "related-user", common.RoleCommonUser)
	now := common.GetTimestamp()
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", 10).Update("registration_ip", "203.0.113.10").Error)
	require.NoError(t, model.LOG_DB.Create(&[]model.Log{
		{UserId: 10, Username: "target-user", Type: model.LogTypeConsume, CreatedAt: now, Ip: "203.0.113.10"},
		{UserId: 3, Username: "peer-root", Type: model.LogTypeConsume, CreatedAt: now, Ip: "203.0.113.10"},
		{UserId: 11, Username: "related-user", Type: model.LogTypeConsume, CreatedAt: now, Ip: "203.0.113.10"},
	}).Error)
	backfillIPSecurityProjectionForControllerTest(t)
	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/user/10/security-audit?range=all", nil, 2)
	ctx.Set("username", "secondary-root")
	ctx.Set("role", common.RoleRootUser)
	ctx.Params = gin.Params{{Key: "id", Value: "10"}}

	GetUserSecurityAudit(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	var data userSecurityAuditData
	require.NoError(t, common.Unmarshal(response.Data, &data))
	require.Equal(t, 1, data.Summary.RelatedUserCount)
	require.Equal(t, []struct {
		UserID   int    `json:"user_id"`
		Username string `json:"username"`
	}{{UserID: 11, Username: "related-user"}}, data.RelatedUsers)
}

func TestUserSecurityAuditSeparatesRegistrationAndLogoutEvents(t *testing.T) {
	setupUserRootBindingControllerTest(t)
	seedUserRootBindingUser(t, 1, "primary-root", common.RoleRootUser)
	seedUserRootBindingUser(t, 10, "target-user", common.RoleCommonUser)
	base := common.GetTimestamp() - 60
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", 10).Updates(map[string]any{
		"registration_ip": "203.0.113.10", "created_at": base,
	}).Error)
	require.NoError(t, model.LOG_DB.Create(&[]model.Log{
		{UserId: 10, Username: "target-user", Type: model.LogTypeLogin, Content: "user.register", CreatedAt: base, Ip: "203.0.113.10"},
		{UserId: 10, Username: "target-user", Type: model.LogTypeLogin, Content: "user.logout", CreatedAt: base + 1, Ip: "203.0.113.10"},
	}).Error)
	backfillIPSecurityProjectionForControllerTest(t)
	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/user/10/security-audit?range=all", nil, 1)
	ctx.Set("role", common.RoleRootUser)
	ctx.Params = gin.Params{{Key: "id", Value: "10"}}

	GetUserSecurityAudit(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	var data userSecurityAuditData
	require.NoError(t, common.Unmarshal(response.Data, &data))
	require.Zero(t, data.Summary.LoginSuccessCount)
	require.Equal(t, 1, data.Summary.LogoutCount)
	require.Equal(t, 2, data.Total)
	require.Len(t, data.Timeline, 2)
	require.Equal(t, []string{"logout", "registration"}, []string{data.Timeline[0].Kind, data.Timeline[1].Kind})
}

func TestUserSecurityAuditCustomRangeExcludesHistoricalRegistrationFacts(t *testing.T) {
	setupUserRootBindingControllerTest(t)
	seedUserRootBindingUser(t, 1, "primary-root", common.RoleRootUser)
	seedUserRootBindingUser(t, 10, "target-user", common.RoleCommonUser)
	seedUserRootBindingUser(t, 11, "historical-related", common.RoleCommonUser)
	require.NoError(t, model.DB.Model(&model.User{}).Where("id IN ?", []int{10, 11}).Updates(map[string]any{
		"registration_ip": "203.0.113.10", "created_at": 100,
	}).Error)
	backfillIPSecurityProjectionForControllerTest(t)
	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/user/10/security-audit?range=custom&start_timestamp=200&end_timestamp=300", nil, 1)
	ctx.Set("role", common.RoleRootUser)
	ctx.Params = gin.Params{{Key: "id", Value: "10"}}

	GetUserSecurityAudit(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	var data userSecurityAuditData
	require.NoError(t, common.Unmarshal(response.Data, &data))
	require.Empty(t, data.Summary.RegistrationIP)
	require.Empty(t, data.Summary.RecentIP)
	require.Zero(t, data.Summary.LastActivityAt)
	require.Zero(t, data.Summary.RelatedUserCount)
	require.Empty(t, data.RelatedUsers)
	require.Zero(t, data.Total)
	require.Empty(t, data.Timeline)
}
