package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/source_attribution_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func withSourceAttributionOptions(t *testing.T, enabled bool, tokenEnforcement bool) {
	t.Helper()

	optionsBytes, err := common.Marshal([]source_attribution_setting.Option{
		{
			Id:            "root",
			Name:          "Root source",
			TargetAffCode: "rootaff",
			SortOrder:     1,
		},
	})
	require.NoError(t, err)

	common.OptionMapRWMutex.Lock()
	previous := common.OptionMap
	common.OptionMap = map[string]string{
		source_attribution_setting.EnabledKey:                    "false",
		source_attribution_setting.TokenAPIEnforcementEnabledKey: "false",
		source_attribution_setting.OptionsKey:                    "[]",
	}
	if enabled {
		common.OptionMap[source_attribution_setting.EnabledKey] = "true"
		common.OptionMap[source_attribution_setting.OptionsKey] = string(optionsBytes)
	}
	if tokenEnforcement {
		common.OptionMap[source_attribution_setting.TokenAPIEnforcementEnabledKey] = "true"
	}
	common.OptionMapRWMutex.Unlock()

	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = previous
		common.OptionMapRWMutex.Unlock()
	})
}

func openSourceAttributionMiddlewareTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	gin.SetMode(gin.TestMode)
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	common.RedisEnabled = false

	previousDB := model.DB

	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.SourceAttributionConfirmation{}))

	t.Cleanup(func() {
		model.DB = previousDB
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func createSourceAttributionRootUser(t *testing.T) {
	t.Helper()

	require.NoError(t, model.DB.Create(&model.User{
		Id:       1,
		Username: "root",
		Role:     common.RoleRootUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
		AffCode:  "rootaff",
	}).Error)
}

func newSourceAttributionContext(path string, userID int, role int) (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, path, nil)
	ctx.Set("id", userID)
	if role != 0 {
		ctx.Set("role", role)
	}
	return ctx, recorder
}

func TestDashboardSourceAttributionAllowsRootWithoutInviter(t *testing.T) {
	openSourceAttributionMiddlewareTestDB(t)
	createSourceAttributionRootUser(t)
	withSourceAttributionOptions(t, true, false)
	ctx, recorder := newSourceAttributionContext("/api/channel/", 1, common.RoleRootUser)

	ok := EnsureDashboardSourceAttributionConfirmed(ctx)

	require.True(t, ok)
	require.False(t, ctx.IsAborted())
	require.Equal(t, http.StatusOK, recorder.Code)
}

func TestAuthenticatedUserSourceAttributionRejectsUnconfirmedNonApiPath(t *testing.T) {
	openSourceAttributionMiddlewareTestDB(t)
	createSourceAttributionRootUser(t)
	require.NoError(t, model.DB.Create(&model.User{
		Id:       7,
		Username: "alice",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
		AffCode:  "aliceaff",
	}).Error)
	withSourceAttributionOptions(t, true, false)
	ctx, recorder := newSourceAttributionContext("/v1/videos/task-id/content", 7, common.RoleCommonUser)

	ok := EnsureAuthenticatedUserSourceAttributionConfirmed(ctx)

	require.False(t, ok)
	require.True(t, ctx.IsAborted())
	require.Equal(t, http.StatusForbidden, recorder.Code)
	require.Contains(t, recorder.Body.String(), "source_attribution_required")
}

func TestTokenSourceAttributionAllowsRootWithoutInviter(t *testing.T) {
	openSourceAttributionMiddlewareTestDB(t)
	createSourceAttributionRootUser(t)
	withSourceAttributionOptions(t, true, true)
	ctx, recorder := newSourceAttributionContext("/v1/chat/completions", 1, 0)

	ok := EnsureTokenSourceAttributionConfirmed(ctx)

	require.True(t, ok)
	require.False(t, ctx.IsAborted())
	require.Equal(t, http.StatusOK, recorder.Code)
}
