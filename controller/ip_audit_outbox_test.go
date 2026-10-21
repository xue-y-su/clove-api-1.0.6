package controller

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type ipAuditPageData struct {
	Items []struct {
		IP string `json:"ip"`
	} `json:"items"`
	Total        int    `json:"total"`
	Page         int    `json:"page"`
	PageSize     int    `json:"page_size"`
	IsRoot       bool   `json:"is_root"`
	ProxyWarning string `json:"proxy_warning"`
}

func TestIPAuditReadDoesNotEnqueueAudit(t *testing.T) {
	setupUserRootBindingControllerTest(t)
	seedUserRootBindingUser(t, 1, "primary-root", common.RoleRootUser)
	require.NoError(t, model.LOG_DB.Create(&model.Log{
		UserId: 8, Username: "ip-user", Type: model.LogTypeLogin, CreatedAt: common.GetTimestamp(), Ip: "203.0.113.10",
	}).Error)
	backfillIPSecurityProjectionForControllerTest(t)
	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/log/ip-audit?limit=25", nil, 1)
	ctx.Set("username", "primary-root")
	ctx.Set("role", common.RoleRootUser)
	ctx.Request.RemoteAddr = "203.0.113.90:4050"

	GetIPAudit(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	var count int64
	require.NoError(t, model.DB.Model(&model.AuditOutboxEvent{}).Where("event_type = ?", "ip_audit_read").Count(&count).Error)
	require.Zero(t, count)
}

func TestIPAuditReadSucceedsWhenAuditOutboxIsUnavailable(t *testing.T) {
	setupUserRootBindingControllerTest(t)
	seedUserRootBindingUser(t, 1, "primary-root", common.RoleRootUser)
	require.NoError(t, model.LOG_DB.Create(&model.Log{
		UserId: 8, Username: "ip-user", Type: model.LogTypeLogin, CreatedAt: common.GetTimestamp(), Ip: "203.0.113.10",
	}).Error)
	backfillIPSecurityProjectionForControllerTest(t)
	failure := errors.New("outbox unavailable")
	require.NoError(t, model.DB.Callback().Create().Before("gorm:create").Register("ip_audit_test:fail_outbox", func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Schema != nil && tx.Statement.Schema.Table == "audit_outbox_events" {
			tx.AddError(failure)
		}
	}))
	t.Cleanup(func() { _ = model.DB.Callback().Create().Remove("ip_audit_test:fail_outbox") })
	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/log/ip-audit?limit=25", nil, 1)
	ctx.Set("username", "primary-root")
	ctx.Set("role", common.RoleRootUser)

	GetIPAudit(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	require.NotEmpty(t, response.Data)
}

func TestIPAuditDetailLoadsOneIPWithoutAuditingRead(t *testing.T) {
	setupUserRootBindingControllerTest(t)
	seedUserRootBindingUser(t, 1, "primary-root", common.RoleRootUser)
	seedUserRootBindingUser(t, 8, "ip-user", common.RoleCommonUser)
	now := common.GetTimestamp()
	require.NoError(t, model.LOG_DB.Create(&[]model.Log{
		{UserId: 8, Username: "ip-user", Type: model.LogTypeLogin, CreatedAt: now - 1, Ip: "203.0.113.10"},
		{UserId: 8, Username: "ip-user", Type: model.LogTypeConsume, CreatedAt: now, Ip: "198.51.100.20", RequestId: "other-ip"},
	}).Error)
	backfillIPSecurityProjectionForControllerTest(t)
	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/log/ip-audit/detail?ip=203.0.113.10", nil, 1)
	ctx.Set("username", "primary-root")
	ctx.Set("role", common.RoleRootUser)
	ctx.Request.RemoteAddr = "203.0.113.90:4050"

	GetIPAuditDetail(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	var detail model.IPAuditProjectionDetail
	require.NoError(t, common.Unmarshal(response.Data, &detail))
	require.Equal(t, "203.0.113.10", detail.IP)
	require.Equal(t, []model.IPAuditUser{{UserId: 8, Username: "ip-user"}}, detail.Users)
	require.Len(t, detail.Timeline, 1)

	var count int64
	require.NoError(t, model.DB.Model(&model.AuditOutboxEvent{}).Where("event_type = ?", "ip_audit_detail_read").Count(&count).Error)
	require.Zero(t, count)
}

func TestIPAuditDetailAllowsAdminActors(t *testing.T) {
	setupUserRootBindingControllerTest(t)
	seedUserRootBindingUser(t, 4, "admin", common.RoleAdminUser)
	seedUserRootBindingUser(t, 8, "ip-user", common.RoleCommonUser)
	now := common.GetTimestamp()
	require.NoError(t, model.LOG_DB.Create(&model.Log{
		UserId: 8, Username: "ip-user", Type: model.LogTypeLogin, CreatedAt: now, Ip: "203.0.113.10",
	}).Error)
	backfillIPSecurityProjectionForControllerTest(t)
	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/log/ip-audit/detail?range=all&ip=203.0.113.*", nil, 4)
	ctx.Set("role", common.RoleAdminUser)

	GetIPAuditDetail(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	var detail model.IPAuditProjectionDetail
	require.NoError(t, common.Unmarshal(response.Data, &detail))
	require.Equal(t, "203.0.113.*", detail.IP)
	require.Equal(t, []model.IPAuditUser{{UserId: 8, Username: "ip-user"}}, detail.Users)
	var count int64
	require.NoError(t, model.DB.Model(&model.AuditOutboxEvent{}).Where("event_type = ?", "ip_audit_detail_read").Count(&count).Error)
	require.Zero(t, count)
}

func TestIPAuditDefaultsToThirtyDaysAndReturnsRealPaginationMetadata(t *testing.T) {
	setupUserRootBindingControllerTest(t)
	t.Setenv("TRUSTED_PROXIES", "")
	seedUserRootBindingUser(t, 1, "primary-root", common.RoleRootUser)
	now := common.GetTimestamp()
	require.NoError(t, model.LOG_DB.Create(&[]model.Log{
		{UserId: 8, Username: "ip-user", Type: model.LogTypeConsume, CreatedAt: now - int64(31*24*time.Hour/time.Second), Ip: "192.0.2.10"},
		{UserId: 8, Username: "ip-user", Type: model.LogTypeConsume, CreatedAt: now - 60, Ip: "203.0.113.10"},
		{UserId: 9, Username: "ip-user-2", Type: model.LogTypeError, CreatedAt: now - 30, Ip: "198.51.100.20", Other: `{"status_code":401}`},
	}).Error)
	backfillIPSecurityProjectionForControllerTest(t)

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/log/ip-audit?p=1&page_size=1", nil, 1)
	ctx.Set("username", "primary-root")
	ctx.Set("role", common.RoleRootUser)
	GetIPAudit(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	var data ipAuditPageData
	require.NoError(t, common.Unmarshal(response.Data, &data))
	require.Equal(t, 2, data.Total)
	require.Equal(t, 1, data.Page)
	require.Equal(t, 1, data.PageSize)
	require.Len(t, data.Items, 1)
	require.True(t, data.IsRoot)
	require.Equal(t, "trusted_proxies_not_configured", data.ProxyWarning)
}

func TestIPAuditRejectsMalformedTimestampInsteadOfFallingBackToDefaultRange(t *testing.T) {
	setupUserRootBindingControllerTest(t)
	seedUserRootBindingUser(t, 1, "primary-root", common.RoleRootUser)
	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/log/ip-audit?start_timestamp=not-a-number", nil, 1)
	ctx.Set("username", "primary-root")
	ctx.Set("role", common.RoleRootUser)

	GetIPAudit(ctx)

	response := decodeAPIResponse(t, recorder)
	require.False(t, response.Success)
	require.Empty(t, response.Data)
	var count int64
	require.NoError(t, model.DB.Model(&model.AuditOutboxEvent{}).Where("event_type = ?", "ip_audit_read").Count(&count).Error)
	require.Zero(t, count)
}

func TestIPAuditScopesGlobalRowsToUsersTheActorCanManage(t *testing.T) {
	setupUserRootBindingControllerTest(t)
	seedUserRootBindingUser(t, 1, "primary-root", common.RoleRootUser)
	seedUserRootBindingUser(t, 2, "secondary-root", common.RoleRootUser)
	seedUserRootBindingUser(t, 3, "peer-root", common.RoleRootUser)
	seedUserRootBindingUser(t, 4, "admin", common.RoleAdminUser)
	seedUserRootBindingUser(t, 10, "ordinary-user", common.RoleCommonUser)
	now := common.GetTimestamp()
	require.NoError(t, model.LOG_DB.Create(&[]model.Log{
		{UserId: 2, Username: "secondary-root", Type: model.LogTypeLogin, CreatedAt: now, Ip: "192.0.2.2"},
		{UserId: 3, Username: "peer-root", Type: model.LogTypeLogin, CreatedAt: now, Ip: "192.0.2.3"},
		{UserId: 4, Username: "admin", Type: model.LogTypeLogin, CreatedAt: now, Ip: "198.51.100.4"},
		{UserId: 10, Username: "ordinary-user", Type: model.LogTypeLogin, CreatedAt: now, Ip: "203.0.113.10"},
	}).Error)
	backfillIPSecurityProjectionForControllerTest(t)

	tests := []struct {
		name      string
		actorID   int
		actorRole int
		wantIPs   []string
	}{
		{
			name: "primary root sees every account", actorID: 1, actorRole: common.RoleRootUser,
			wantIPs: []string{"192.0.2.2", "192.0.2.3", "198.51.100.4", "203.0.113.10"},
		},
		{
			name: "secondary root excludes self and peer roots", actorID: 2, actorRole: common.RoleRootUser,
			wantIPs: []string{"198.51.100.4", "203.0.113.10"},
		},
		{
			name: "admin sees only ordinary users masked", actorID: 4, actorRole: common.RoleAdminUser,
			wantIPs: []string{"203.0.113.*"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/log/ip-audit?range=all", nil, test.actorID)
			ctx.Set("role", test.actorRole)
			GetIPAudit(ctx)

			response := decodeAPIResponse(t, recorder)
			require.True(t, response.Success, response.Message)
			var data ipAuditPageData
			require.NoError(t, common.Unmarshal(response.Data, &data))
			gotIPs := make([]string, 0, len(data.Items))
			for _, item := range data.Items {
				gotIPs = append(gotIPs, item.IP)
			}
			require.ElementsMatch(t, test.wantIPs, gotIPs)
		})
	}
}

func TestIPAuditExcludesHardDeletedPrivilegedUserLogsFromSecondaryRoot(t *testing.T) {
	setupUserRootBindingControllerTest(t)
	seedUserRootBindingUser(t, 2, "secondary-root", common.RoleRootUser)
	seedUserRootBindingUser(t, 3, "deleted-peer-root", common.RoleRootUser)
	seedUserRootBindingUser(t, 10, "target-user", common.RoleCommonUser)
	now := common.GetTimestamp()
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", 10).Update("registration_ip", "203.0.113.10").Error)
	require.NoError(t, model.LOG_DB.Create(&[]model.Log{
		{UserId: 3, Username: "deleted-peer-root", Type: model.LogTypeConsume, CreatedAt: now, Ip: "192.0.2.3"},
		{UserId: 3, Username: "deleted-peer-root", Type: model.LogTypeConsume, CreatedAt: now, Ip: "203.0.113.10"},
		{UserId: 10, Username: "target-user", Type: model.LogTypeConsume, CreatedAt: now, Ip: "203.0.113.10"},
	}).Error)
	require.NoError(t, model.DB.Unscoped().Delete(&model.User{}, "id = ?", 3).Error)
	backfillIPSecurityProjectionForControllerTest(t)

	globalCtx, globalRecorder := newAuthenticatedContext(t, http.MethodGet, "/api/log/ip-audit?range=all", nil, 2)
	globalCtx.Set("role", common.RoleRootUser)
	GetIPAudit(globalCtx)
	globalResponse := decodeAPIResponse(t, globalRecorder)
	require.True(t, globalResponse.Success, globalResponse.Message)
	var page ipAuditPageData
	require.NoError(t, common.Unmarshal(globalResponse.Data, &page))
	require.Len(t, page.Items, 1)
	require.Equal(t, "203.0.113.10", page.Items[0].IP)

	detailCtx, detailRecorder := newAuthenticatedContext(t, http.MethodGet, "/api/user/10/security-audit?range=all", nil, 2)
	detailCtx.Set("role", common.RoleRootUser)
	detailCtx.Params = gin.Params{{Key: "id", Value: "10"}}
	GetUserSecurityAudit(detailCtx)
	detailResponse := decodeAPIResponse(t, detailRecorder)
	require.True(t, detailResponse.Success, detailResponse.Message)
	var detail userSecurityAuditData
	require.NoError(t, common.Unmarshal(detailResponse.Data, &detail))
	require.Zero(t, detail.Summary.RelatedUserCount)
	require.Empty(t, detail.RelatedUsers)
}
