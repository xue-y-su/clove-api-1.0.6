package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/compliance_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func withComplianceOptions(t *testing.T, values map[string]string) {
	t.Helper()

	common.OptionMapRWMutex.Lock()
	previous := common.OptionMap
	common.OptionMap = map[string]string{
		compliance_setting.StatementEnabledKey:           "false",
		compliance_setting.StatementVersionKey:           "",
		compliance_setting.StatementContentKey:           "",
		compliance_setting.TokenAPIEnforcementEnabledKey: "false",
	}
	for key, value := range values {
		common.OptionMap[key] = value
	}
	common.OptionMapRWMutex.Unlock()

	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = previous
		common.OptionMapRWMutex.Unlock()
		ClearComplianceConfirmationCache()
	})
}

func openComplianceMiddlewareTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	gin.SetMode(gin.TestMode)
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	common.RedisEnabled = false

	previousDB := model.DB
	previousLogDB := model.LOG_DB

	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	model.LOG_DB = db
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Token{}, &model.ComplianceStatementSnapshot{}, &model.ComplianceConfirmation{}))

	t.Cleanup(func() {
		model.DB = previousDB
		model.LOG_DB = previousLogDB
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func newComplianceMiddlewareContext(path string, userID int) (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, path, nil)
	ctx.Set("id", userID)
	return ctx, recorder
}

func TestDashboardComplianceBypassesWhenStatementDisabled(t *testing.T) {
	withComplianceOptions(t, map[string]string{})
	ctx, recorder := newComplianceMiddlewareContext("/api/token/", 7)

	ok := EnsureDashboardComplianceConfirmed(ctx)

	require.True(t, ok)
	require.False(t, ctx.IsAborted())
	require.Equal(t, http.StatusOK, recorder.Code)
}

func TestDashboardComplianceRejectsUnconfirmedUserWhenEnabled(t *testing.T) {
	openComplianceMiddlewareTestDB(t)
	withComplianceOptions(t, map[string]string{
		compliance_setting.StatementEnabledKey: "true",
		compliance_setting.StatementVersionKey: "2026-07",
		compliance_setting.StatementContentKey: "声明正文",
	})
	ctx, recorder := newComplianceMiddlewareContext("/api/token/", 7)

	ok := EnsureDashboardComplianceConfirmed(ctx)

	require.False(t, ok)
	require.True(t, ctx.IsAborted())
	require.Equal(t, http.StatusForbidden, recorder.Code)
	require.Contains(t, recorder.Body.String(), "compliance_confirmation_required")
}

func TestDashboardComplianceAllowsConfirmedUserWhenEnabled(t *testing.T) {
	openComplianceMiddlewareTestDB(t)
	withComplianceOptions(t, map[string]string{
		compliance_setting.StatementEnabledKey: "true",
		compliance_setting.StatementVersionKey: "2026-07",
		compliance_setting.StatementContentKey: "声明正文",
	})
	hash := compliance_setting.ContentHash("声明正文")
	require.NoError(t, model.RecordComplianceConfirmation(7, "alice", "2026-07", hash, "203.0.113.10", "ua", 100))
	ctx, _ := newComplianceMiddlewareContext("/api/token/", 7)

	ok := EnsureDashboardComplianceConfirmed(ctx)

	require.True(t, ok)
	require.False(t, ctx.IsAborted())
}

func TestDashboardComplianceAllowsRootWithoutConfirmation(t *testing.T) {
	openComplianceMiddlewareTestDB(t)
	withComplianceOptions(t, map[string]string{
		compliance_setting.StatementEnabledKey: "true",
		compliance_setting.StatementVersionKey: "2026-07",
		compliance_setting.StatementContentKey: "声明正文",
	})
	ctx, recorder := newComplianceMiddlewareContext("/api/log/", 1)
	ctx.Set("role", common.RoleRootUser)

	ok := EnsureDashboardComplianceConfirmed(ctx)

	require.True(t, ok)
	require.False(t, ctx.IsAborted())
	require.Equal(t, http.StatusOK, recorder.Code)
}

func TestAuthenticatedUserComplianceRejectsUnconfirmedNonApiPath(t *testing.T) {
	openComplianceMiddlewareTestDB(t)
	withComplianceOptions(t, map[string]string{
		compliance_setting.StatementEnabledKey: "true",
		compliance_setting.StatementVersionKey: "2026-07",
		compliance_setting.StatementContentKey: "声明正文",
	})
	ctx, recorder := newComplianceMiddlewareContext("/v1/videos/task-id/content", 7)

	ok := EnsureAuthenticatedUserComplianceConfirmed(ctx)

	require.False(t, ok)
	require.True(t, ctx.IsAborted())
	require.Equal(t, http.StatusForbidden, recorder.Code)
	require.Contains(t, recorder.Body.String(), "compliance_confirmation_required")
}

func TestTokenComplianceRequiresSeparateEnforcementSwitch(t *testing.T) {
	openComplianceMiddlewareTestDB(t)
	withComplianceOptions(t, map[string]string{
		compliance_setting.StatementEnabledKey:           "true",
		compliance_setting.StatementVersionKey:           "2026-07",
		compliance_setting.StatementContentKey:           "声明正文",
		compliance_setting.TokenAPIEnforcementEnabledKey: "false",
	})
	ctx, _ := newComplianceMiddlewareContext("/v1/chat/completions", 7)

	require.True(t, EnsureTokenComplianceConfirmed(ctx))

	common.OptionMapRWMutex.Lock()
	common.OptionMap[compliance_setting.TokenAPIEnforcementEnabledKey] = "true"
	common.OptionMapRWMutex.Unlock()
	ctx, recorder := newComplianceMiddlewareContext("/v1/chat/completions", 7)

	require.False(t, EnsureTokenComplianceConfirmed(ctx))
	require.True(t, ctx.IsAborted())
	require.Equal(t, http.StatusForbidden, recorder.Code)
	require.Contains(t, recorder.Body.String(), "compliance_confirmation_required")
}

func TestTokenAuthReadOnlyEnforcesComplianceWhenEnabled(t *testing.T) {
	openComplianceMiddlewareTestDB(t)
	require.NoError(t, model.DB.Create(&model.User{
		Id:       7,
		Username: "alice",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
	}).Error)
	require.NoError(t, model.DB.Create(&model.Token{
		Id:          10,
		UserId:      7,
		Key:         "readonly",
		Status:      common.TokenStatusEnabled,
		ExpiredTime: -1,
	}).Error)
	withComplianceOptions(t, map[string]string{
		compliance_setting.StatementEnabledKey:           "true",
		compliance_setting.StatementVersionKey:           "2026-07",
		compliance_setting.StatementContentKey:           "声明正文",
		compliance_setting.TokenAPIEnforcementEnabledKey: "true",
	})
	ctx, recorder := newComplianceMiddlewareContext("/api/log/token", 0)
	ctx.Request.Header.Set("Authorization", "Bearer sk-readonly")

	TokenAuthReadOnly()(ctx)

	require.True(t, ctx.IsAborted())
	require.Equal(t, http.StatusForbidden, recorder.Code)
	require.Contains(t, recorder.Body.String(), "compliance_confirmation_required")
}
