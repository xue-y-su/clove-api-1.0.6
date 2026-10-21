package controller

import (
	"errors"
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type sensitiveIPListCase struct {
	name      string
	path      string
	fullIP    string
	maskedIP  string
	eventType string
	target    string
	handler   gin.HandlerFunc
}

func setupSensitiveIPProjectionTest(t *testing.T) {
	t.Helper()
	db := openTokenControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(
		&model.Log{},
		&model.ComplianceConfirmation{},
		&model.SourceAttributionConfirmation{},
		&model.AuditOutboxEvent{},
	))
	require.NoError(t, db.Create(&model.Log{
		UserId: 11, Username: "log-user", Type: model.LogTypeConsume,
		CreatedAt: 300, Ip: "203.0.113.41",
	}).Error)
	require.NoError(t, db.Create(&model.ComplianceConfirmation{
		UserId: 12, Username: "compliance-user", Version: "2026-07",
		ContentHash: "hash", ConfirmedAt: 200, Ip: "198.51.100.42",
	}).Error)
	require.NoError(t, db.Create(&model.SourceAttributionConfirmation{
		UserId: 13, Username: "source-user", OptionId: "direct",
		ConfirmedAt: 100, Ip: "192.0.2.43",
	}).Error)
}

func sensitiveIPListCases() []sensitiveIPListCase {
	return []sensitiveIPListCase{
		{
			name: "ordinary logs", path: "/api/log/?p=1&page_size=10",
			fullIP: "203.0.113.41", maskedIP: "203.0.113.*",
			eventType: "log.full_ip_read", target: "logs", handler: GetAllLogs,
		},
		{
			name: "compliance confirmations", path: "/api/compliance/confirmations?p=1&page_size=10",
			fullIP: "198.51.100.42", maskedIP: "198.51.100.*",
			eventType: "compliance.full_ip_read", target: "compliance-confirmations", handler: GetComplianceConfirmations,
		},
		{
			name: "source attribution confirmations", path: "/api/source-attribution/confirmations?p=1&page_size=10",
			fullIP: "192.0.2.43", maskedIP: "192.0.2.*",
			eventType: "source_attribution.full_ip_read", target: "source-attribution-confirmations", handler: GetSourceAttributionConfirmations,
		},
	}
}

func TestAdminSensitiveIPRecordListsReturnMaskedProjection(t *testing.T) {
	setupSensitiveIPProjectionTest(t)

	for _, testCase := range sensitiveIPListCases() {
		t.Run(testCase.name, func(t *testing.T) {
			ctx, recorder := newAuthenticatedContext(t, http.MethodGet, testCase.path, nil, 2)
			ctx.Set("role", common.RoleAdminUser)

			testCase.handler(ctx)

			response := decodeAPIResponse(t, recorder)
			require.True(t, response.Success, response.Message)
			require.Contains(t, string(response.Data), testCase.maskedIP)
			require.NotContains(t, string(response.Data), testCase.fullIP)
		})
	}
}

func TestGetAllLogsScopesTopupCallerIPByRole(t *testing.T) {
	for _, test := range []struct {
		name   string
		role   int
		wantIP string
	}{
		{name: "admin", role: common.RoleAdminUser, wantIP: "203.0.113.*"},
		{name: "root", role: common.RoleRootUser, wantIP: "203.0.113.77"},
	} {
		t.Run(test.name, func(t *testing.T) {
			setupSensitiveIPProjectionTest(t)
			require.NoError(t, model.DB.Create(&model.Log{
				UserId: 17, Username: "topup-user", Type: model.LogTypeTopup,
				CreatedAt: 400, Ip: "203.0.113.77",
				Other: common.MapToJsonStr(map[string]interface{}{
					"admin_info": map[string]interface{}{
						"caller_ip":      "203.0.113.77",
						"payment_method": "manual",
					},
				}),
			}).Error)

			ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/log/?p=1&page_size=10&type=1", nil, 2)
			ctx.Set("role", test.role)
			GetAllLogs(ctx)

			response := decodeAPIResponse(t, recorder)
			require.True(t, response.Success, response.Message)
			var page struct {
				Items []model.Log `json:"items"`
			}
			require.NoError(t, common.Unmarshal(response.Data, &page))
			require.Len(t, page.Items, 1)
			require.Equal(t, test.wantIP, page.Items[0].Ip)

			var other map[string]interface{}
			require.NoError(t, common.Unmarshal([]byte(page.Items[0].Other), &other))
			adminInfo, ok := other["admin_info"].(map[string]interface{})
			require.True(t, ok)
			require.Equal(t, test.wantIP, adminInfo["caller_ip"])
			require.Equal(t, "manual", adminInfo["payment_method"])
		})
	}
}

func TestRootSensitiveIPRecordListsAuditFullProjectionBeforeResponse(t *testing.T) {
	setupSensitiveIPProjectionTest(t)

	for _, testCase := range sensitiveIPListCases() {
		t.Run(testCase.name, func(t *testing.T) {
			ctx, recorder := newAuthenticatedContext(t, http.MethodGet, testCase.path, nil, 1)
			ctx.Set("role", common.RoleRootUser)
			ctx.Request.RemoteAddr = "198.18.0.10:4050"

			testCase.handler(ctx)

			response := decodeAPIResponse(t, recorder)
			require.True(t, response.Success, response.Message)
			require.Contains(t, string(response.Data), testCase.fullIP)

			var event model.AuditOutboxEvent
			require.NoError(t, model.DB.First(&event, "event_type = ?", testCase.eventType).Error)
			require.Equal(t, 1, event.ActorID)
			require.Equal(t, testCase.target, event.Target)
			require.Equal(t, "198.18.0.10", event.IP)
			require.Contains(t, event.Payload, `"full_ip_access":true`)
			require.Contains(t, event.Payload, `"item_count":1`)
			require.Contains(t, event.Payload, `"page":1`)
			require.Contains(t, event.Payload, `"page_size":10`)
			require.Contains(t, event.Payload, `"total":1`)
		})
	}
}

func TestRootSensitiveIPRecordListsReturnNoDataWhenAuditEnqueueFails(t *testing.T) {
	for _, testCase := range sensitiveIPListCases() {
		t.Run(testCase.name, func(t *testing.T) {
			setupSensitiveIPProjectionTest(t)
			failure := errors.New("outbox unavailable")
			require.NoError(t, model.DB.Callback().Create().Before("gorm:create").Register("sensitive_ip_projection_test:fail_outbox", func(tx *gorm.DB) {
				if tx.Statement != nil && tx.Statement.Schema != nil && tx.Statement.Schema.Table == "audit_outbox_events" {
					tx.AddError(failure)
				}
			}))
			t.Cleanup(func() {
				_ = model.DB.Callback().Create().Remove("sensitive_ip_projection_test:fail_outbox")
			})

			ctx, recorder := newAuthenticatedContext(t, http.MethodGet, testCase.path, nil, 1)
			ctx.Set("role", common.RoleRootUser)

			testCase.handler(ctx)

			response := decodeAPIResponse(t, recorder)
			require.False(t, response.Success)
			require.Empty(t, response.Data)
			require.NotContains(t, recorder.Body.String(), testCase.fullIP)
		})
	}
}

func TestAdminSensitiveIPFiltersAreWidenedToMaskedNetwork(t *testing.T) {
	setupSensitiveIPProjectionTest(t)
	require.NoError(t, model.DB.Model(&model.Log{}).Where("ip = ?", "203.0.113.41").Update("quota", 3).Error)
	require.NoError(t, model.DB.Create(&model.Log{
		UserId: 14, Username: "log-user-2", Type: model.LogTypeConsume,
		CreatedAt: 301, Ip: "203.0.113.42", Quota: 7,
	}).Error)
	require.NoError(t, model.DB.Create(&model.ComplianceConfirmation{
		UserId: 15, Username: "compliance-user-2", Version: "2026-07",
		ContentHash: "hash-2", ConfirmedAt: 201, Ip: "198.51.100.43",
	}).Error)
	require.NoError(t, model.DB.Create(&model.SourceAttributionConfirmation{
		UserId: 16, Username: "source-user-2", OptionId: "direct",
		ConfirmedAt: 101, Ip: "192.0.2.44",
	}).Error)
	testCases := []struct {
		name    string
		path    string
		handler gin.HandlerFunc
	}{
		{name: "ordinary logs", path: "/api/log/?p=1&page_size=10&ip=203.0.113.99", handler: GetAllLogs},
		{name: "compliance", path: "/api/compliance/confirmations?p=1&page_size=10&ip=198.51.100.99", handler: GetComplianceConfirmations},
		{name: "source attribution", path: "/api/source-attribution/confirmations?p=1&page_size=10&ip=192.0.2.99", handler: GetSourceAttributionConfirmations},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			ctx, recorder := newAuthenticatedContext(t, http.MethodGet, testCase.path, nil, 2)
			ctx.Set("role", common.RoleAdminUser)
			testCase.handler(ctx)

			response := decodeAPIResponse(t, recorder)
			require.True(t, response.Success, response.Message)
			var page struct {
				Total int `json:"total"`
			}
			require.NoError(t, common.Unmarshal(response.Data, &page))
			require.Equal(t, 2, page.Total)
		})
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/log/stat?type=2&ip=203.0.113.99", nil, 2)
	ctx.Set("role", common.RoleAdminUser)
	GetLogsStat(ctx)
	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	var stat struct {
		Quota int `json:"quota"`
	}
	require.NoError(t, common.Unmarshal(response.Data, &stat))
	require.Equal(t, 10, stat.Quota)
}

func TestAdminSensitiveIPFilterRejectsCallerSuppliedWildcard(t *testing.T) {
	setupSensitiveIPProjectionTest(t)
	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/log/?ip=203.0.113.%25", nil, 2)
	ctx.Set("role", common.RoleAdminUser)

	GetAllLogs(ctx)

	require.False(t, decodeAPIResponse(t, recorder).Success)
}

func TestRootExactIPStatReadIsAuditedBeforeResponse(t *testing.T) {
	setupSensitiveIPProjectionTest(t)
	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/log/stat?type=2&ip=203.0.113.41", nil, 1)
	ctx.Set("role", common.RoleRootUser)
	ctx.Request.RemoteAddr = "198.18.0.11:4050"

	GetLogsStat(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	var event model.AuditOutboxEvent
	require.NoError(t, model.DB.First(&event, "event_type = ?", "log.full_ip_stat_read").Error)
	require.Equal(t, 1, event.ActorID)
	require.Equal(t, "198.18.0.11", event.IP)
	require.Contains(t, event.Payload, `"full_ip_access":true`)
}

func TestRootExactIPStatReadReturnsNoDataWhenAuditEnqueueFails(t *testing.T) {
	setupSensitiveIPProjectionTest(t)
	failure := errors.New("outbox unavailable")
	require.NoError(t, model.DB.Callback().Create().Before("gorm:create").Register("sensitive_ip_projection_test:fail_stat_outbox", func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Schema != nil && tx.Statement.Schema.Table == "audit_outbox_events" {
			tx.AddError(failure)
		}
	}))
	t.Cleanup(func() {
		_ = model.DB.Callback().Create().Remove("sensitive_ip_projection_test:fail_stat_outbox")
	})
	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/log/stat?type=2&ip=203.0.113.41", nil, 1)
	ctx.Set("role", common.RoleRootUser)

	GetLogsStat(ctx)

	response := decodeAPIResponse(t, recorder)
	require.False(t, response.Success)
	require.Empty(t, response.Data)
	require.NotContains(t, recorder.Body.String(), `"quota"`)
}
