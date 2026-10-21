package service

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func openUserSecurityProjectionTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	previousDB := model.DB
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.IPSecurityEvent{}))
	model.DB = db

	t.Cleanup(func() {
		model.DB = previousDB
		sqlDB, sqlErr := db.DB()
		if sqlErr == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func seedUserSecurityProjectionUser(t *testing.T, db *gorm.DB, id int, username string, role int) *model.User {
	t.Helper()
	user := &model.User{
		Id: id, Username: username, Password: "password", AffCode: fmt.Sprintf("security-projection-%d", id),
		Role: role, Status: common.UserStatusEnabled,
	}
	require.NoError(t, db.Create(user).Error)
	return user
}

func seedUserSecurityProjectionEvent(
	t *testing.T,
	db *gorm.DB,
	eventID string,
	userID int,
	username string,
	createdAt int64,
	ip string,
	kind string,
	requestID string,
) {
	t.Helper()
	httpStatus := 0
	if kind == model.IPSecurityKindAPIError {
		httpStatus = http.StatusUnauthorized
	}
	require.NoError(t, db.Create(&model.IPSecurityEvent{
		EventID: eventID, UserID: userID, Username: username, CreatedAt: createdAt,
		IP: ip, MaskedIP: model.MaskAuditIP(ip), Kind: kind, HTTPStatus: httpStatus, RequestID: requestID,
	}).Error)
}

func TestQueryUserSecurityProjectionPaginatesMoreThanTenThousandEvents(t *testing.T) {
	db := openUserSecurityProjectionTestDB(t)
	actor := seedUserSecurityProjectionUser(t, db, 2, "root", common.RoleRootUser)
	target := seedUserSecurityProjectionUser(t, db, 20, "target", common.RoleCommonUser)

	events := make([]model.IPSecurityEvent, 10_001)
	for index := range events {
		createdAt := int64(index + 1)
		events[index] = model.IPSecurityEvent{
			EventID: fmt.Sprintf("large-%05d", index), UserID: target.Id, Username: target.Username,
			CreatedAt: createdAt, IP: "203.0.113.20", MaskedIP: "203.0.113.*",
			Kind: model.IPSecurityKindAPIRequest, RequestID: fmt.Sprintf("request-%05d", index),
		}
	}
	require.NoError(t, db.CreateInBatches(&events, 200).Error)

	result := UserSecurityAudit{}
	err := queryUserSecurityProjection(context.Background(), actor, target, IPAuditQuery{
		StartTimestamp: 1, EndTimestamp: 10_001, Page: 1001, PageSize: 10,
	}, &result)
	require.NoError(t, err)
	require.Equal(t, 10_001, result.Summary.APIRequestCount)
	require.EqualValues(t, 10_001, result.Summary.LastActivityAt)
	require.Equal(t, "203.0.113.20", result.Summary.RecentIP)
	require.Equal(t, 10_001, result.Total)
	require.Len(t, result.Timeline, 1)
	require.EqualValues(t, 1, result.Timeline[0].CreatedAt)
	require.Equal(t, "request-00000", result.Timeline[0].RequestID)
}

func TestQueryUserSecurityProjectionScopesSummaryAndRelatedUsersToRangeAndHierarchy(t *testing.T) {
	db := openUserSecurityProjectionTestDB(t)
	actor := seedUserSecurityProjectionUser(t, db, 2, "root", common.RoleRootUser)
	target := seedUserSecurityProjectionUser(t, db, 20, "target", common.RoleCommonUser)
	seedUserSecurityProjectionUser(t, db, 21, "eligible", common.RoleCommonUser)
	seedUserSecurityProjectionUser(t, db, 22, "target-ip-outside", common.RoleCommonUser)
	seedUserSecurityProjectionUser(t, db, 23, "related-ip-outside", common.RoleCommonUser)
	seedUserSecurityProjectionUser(t, db, 24, "peer-root", common.RoleRootUser)
	deleted := seedUserSecurityProjectionUser(t, db, 25, "deleted", common.RoleCommonUser)
	require.NoError(t, db.Delete(deleted).Error)

	seedUserSecurityProjectionEvent(t, db, "target-old-registration", target.Id, target.Username, 50, "198.51.100.20", model.IPSecurityKindRegistration, "")
	seedUserSecurityProjectionEvent(t, db, "target-registration", target.Id, target.Username, 100, "192.0.2.20", model.IPSecurityKindRegistration, "registration-request")
	seedUserSecurityProjectionEvent(t, db, "target-api", target.Id, target.Username, 110, "203.0.113.20", model.IPSecurityKindAPIRequest, "api-request")
	seedUserSecurityProjectionEvent(t, db, "target-error", target.Id, target.Username, 120, "203.0.113.20", model.IPSecurityKindAPIError, "error-request")
	seedUserSecurityProjectionEvent(t, db, "target-login", target.Id, target.Username, 130, "203.0.113.20", model.IPSecurityKindLoginSuccess, "login-request")
	seedUserSecurityProjectionEvent(t, db, "target-login-failure", target.Id, target.Username, 140, "203.0.113.20", model.IPSecurityKindLoginFailure, "login-failure-request")
	seedUserSecurityProjectionEvent(t, db, "target-logout", target.Id, target.Username, 150, "203.0.113.20", model.IPSecurityKindLogout, "logout-request")

	seedUserSecurityProjectionEvent(t, db, "eligible", 21, "stale-eligible-name", 125, "203.0.113.20", model.IPSecurityKindLoginSuccess, "eligible-request")
	seedUserSecurityProjectionEvent(t, db, "target-ip-outside", 22, "target-ip-outside", 125, "198.51.100.20", model.IPSecurityKindLoginSuccess, "outside-target-request")
	seedUserSecurityProjectionEvent(t, db, "related-ip-outside", 23, "related-ip-outside", 250, "203.0.113.20", model.IPSecurityKindLoginSuccess, "outside-related-request")
	seedUserSecurityProjectionEvent(t, db, "peer-root", 24, "peer-root", 125, "203.0.113.20", model.IPSecurityKindLoginSuccess, "peer-root-request")
	seedUserSecurityProjectionEvent(t, db, "deleted", 25, "deleted", 125, "203.0.113.20", model.IPSecurityKindLoginSuccess, "deleted-request")
	seedUserSecurityProjectionEvent(t, db, "orphan", 99, "orphan", 125, "203.0.113.20", model.IPSecurityKindLoginSuccess, "orphan-request")

	result := UserSecurityAudit{}
	err := queryUserSecurityProjection(context.Background(), actor, target, IPAuditQuery{
		StartTimestamp: 100, EndTimestamp: 200, Page: 1, PageSize: 10,
	}, &result)
	require.NoError(t, err)
	require.Equal(t, "192.0.2.20", result.Summary.RegistrationIP)
	require.Equal(t, "203.0.113.20", result.Summary.RecentIP)
	require.EqualValues(t, 150, result.Summary.LastActivityAt)
	require.Equal(t, 1, result.Summary.APIRequestCount)
	require.Equal(t, 1, result.Summary.ErrorCount)
	require.Equal(t, 1, result.Summary.LoginSuccessCount)
	require.Equal(t, 1, result.Summary.LoginFailureCount)
	require.Equal(t, 1, result.Summary.LogoutCount)
	require.Equal(t, 2, result.Summary.AnomalyCount)
	require.Equal(t, 1, result.Summary.RelatedUserCount)
	require.Equal(t, []UserSecurityRelatedUser{{
		UserID: 21, Username: "eligible", SharedIPCount: 1,
		AuthenticationCount: 1, LastSeenAt: 125,
	}}, result.RelatedUsers)
	require.Equal(t, 6, result.Total)
	require.Len(t, result.Timeline, 6)
	require.Equal(t, []string{
		model.IPSecurityKindLogout,
		model.IPSecurityKindLoginFailure,
		model.IPSecurityKindLoginSuccess,
		model.IPSecurityKindAPIError,
		model.IPSecurityKindAPIRequest,
		model.IPSecurityKindRegistration,
	}, []string{
		result.Timeline[0].Kind,
		result.Timeline[1].Kind,
		result.Timeline[2].Kind,
		result.Timeline[3].Kind,
		result.Timeline[4].Kind,
		result.Timeline[5].Kind,
	})
}

func TestQueryUserSecurityProjectionReturnsOnlyMaskedSummaryForAdminAndRechecksRoles(t *testing.T) {
	db := openUserSecurityProjectionTestDB(t)
	actor := seedUserSecurityProjectionUser(t, db, 10, "admin", common.RoleAdminUser)
	target := seedUserSecurityProjectionUser(t, db, 20, "target", common.RoleCommonUser)
	related := seedUserSecurityProjectionUser(t, db, 21, "related", common.RoleCommonUser)

	seedUserSecurityProjectionEvent(t, db, "admin-registration", target.Id, target.Username, 100, "2001:db8::1", model.IPSecurityKindRegistration, "registration")
	seedUserSecurityProjectionEvent(t, db, "admin-api", target.Id, target.Username, 110, "203.0.113.55", model.IPSecurityKindAPIRequest, "api")
	seedUserSecurityProjectionEvent(t, db, "admin-related", related.Id, related.Username, 120, "203.0.113.55", model.IPSecurityKindLoginSuccess, "related")

	query := IPAuditQuery{StartTimestamp: 100, EndTimestamp: 200, Page: 1, PageSize: 20}
	result := UserSecurityAudit{}
	require.NoError(t, queryUserSecurityProjection(context.Background(), actor, target, query, &result))
	require.False(t, result.IsRoot)
	require.Equal(t, "masked", result.Projection)
	require.Equal(t, "2001:db8:*", result.Summary.RegistrationIP)
	require.Equal(t, "203.0.113.*", result.Summary.RecentIP)
	require.Equal(t, 1, result.Summary.RelatedUserCount)
	require.Empty(t, result.RelatedUsers)
	require.Empty(t, result.Timeline)
	require.Zero(t, result.Total)

	require.NoError(t, db.Model(&model.User{}).Where("id = ?", related.Id).
		Update("role", common.RoleAdminUser).Error)
	result = UserSecurityAudit{}
	require.NoError(t, queryUserSecurityProjection(context.Background(), actor, target, query, &result))
	require.Zero(t, result.Summary.RelatedUserCount)
}

func TestQueryUserSecurityProjectionAllowsIDOneToSeeAllRelatedEventIdentities(t *testing.T) {
	db := openUserSecurityProjectionTestDB(t)
	actor := seedUserSecurityProjectionUser(t, db, 1, "super-root", common.RoleRootUser)
	target := seedUserSecurityProjectionUser(t, db, 20, "target", common.RoleCommonUser)
	peerRoot := seedUserSecurityProjectionUser(t, db, 30, "peer-root", common.RoleRootUser)
	deleted := seedUserSecurityProjectionUser(t, db, 31, "deleted", common.RoleCommonUser)
	require.NoError(t, db.Delete(deleted).Error)

	seedUserSecurityProjectionEvent(t, db, "super-target", target.Id, target.Username, 100, "203.0.113.20", model.IPSecurityKindAPIRequest, "target")
	seedUserSecurityProjectionEvent(t, db, "super-peer", peerRoot.Id, peerRoot.Username, 110, "203.0.113.20", model.IPSecurityKindLoginSuccess, "peer")
	seedUserSecurityProjectionEvent(t, db, "super-deleted", deleted.Id, deleted.Username, 120, "203.0.113.20", model.IPSecurityKindLoginSuccess, "deleted")
	seedUserSecurityProjectionEvent(t, db, "super-orphan", 99, "orphan", 130, "203.0.113.20", model.IPSecurityKindLoginSuccess, "orphan")

	result := UserSecurityAudit{}
	require.NoError(t, queryUserSecurityProjection(context.Background(), actor, target, IPAuditQuery{
		StartTimestamp: 100, EndTimestamp: 200, Page: 1, PageSize: 20,
	}, &result))
	require.Equal(t, 3, result.Summary.RelatedUserCount)
	require.Equal(t, []UserSecurityRelatedUser{
		{UserID: 99, Username: "orphan", SharedIPCount: 1, AuthenticationCount: 1, LastSeenAt: 130},
		{UserID: deleted.Id, Username: "deleted", SharedIPCount: 1, AuthenticationCount: 1, LastSeenAt: 120},
		{UserID: peerRoot.Id, Username: "peer-root", SharedIPCount: 1, AuthenticationCount: 1, LastSeenAt: 110},
	}, result.RelatedUsers)
}

func TestQueryUserSecurityProjectionPaginatesRelatedUsersAndKeepsCompleteCount(t *testing.T) {
	db := openUserSecurityProjectionTestDB(t)
	actor := seedUserSecurityProjectionUser(t, db, 1, "root", common.RoleRootUser)
	target := seedUserSecurityProjectionUser(t, db, 20, "target", common.RoleCommonUser)
	seedUserSecurityProjectionEvent(t, db, "bounded-target", target.Id, target.Username, 100, "203.0.113.20", model.IPSecurityKindAPIRequest, "target")
	for index := 0; index < 55; index++ {
		userID := 100 + index
		username := fmt.Sprintf("related-%02d", index)
		seedUserSecurityProjectionUser(t, db, userID, username, common.RoleCommonUser)
		seedUserSecurityProjectionEvent(t, db, fmt.Sprintf("bounded-related-%02d", index), userID, username, int64(101+index), "203.0.113.20", model.IPSecurityKindLoginSuccess, fmt.Sprintf("related-%02d", index))
	}

	result := UserSecurityAudit{}
	require.NoError(t, queryUserSecurityProjection(context.Background(), actor, target, IPAuditQuery{
		StartTimestamp: 100, EndTimestamp: 200, Page: 1, PageSize: 20,
		RelatedPage: 1, RelatedPageSize: 20,
	}, &result))
	require.Equal(t, 55, result.Summary.RelatedUserCount)
	require.Equal(t, 55, result.RelatedTotal)
	require.Equal(t, 1, result.RelatedPage)
	require.Equal(t, 20, result.RelatedPageSize)
	require.True(t, result.RelatedHasMore)
	require.Len(t, result.RelatedUsers, 20)
	require.Equal(t, "related-54", result.RelatedUsers[0].Username)
	require.Equal(t, 1, result.RelatedUsers[0].SharedIPCount)
	require.Equal(t, 1, result.RelatedUsers[0].AuthenticationCount)
	require.Equal(t, "related-35", result.RelatedUsers[19].Username)

	result = UserSecurityAudit{}
	require.NoError(t, queryUserSecurityProjection(context.Background(), actor, target, IPAuditQuery{
		StartTimestamp: 100, EndTimestamp: 200, Page: 1, PageSize: 20,
		RelatedPage: 3, RelatedPageSize: 20,
	}, &result))
	require.Equal(t, 55, result.RelatedTotal)
	require.Equal(t, 3, result.RelatedPage)
	require.Equal(t, 20, result.RelatedPageSize)
	require.False(t, result.RelatedHasMore)
	require.Len(t, result.RelatedUsers, 15)
	require.Equal(t, "related-14", result.RelatedUsers[0].Username)
	require.Equal(t, "related-00", result.RelatedUsers[14].Username)
}

func TestQueryUserSecurityProjectionFailsClosedWhenIDOneIsNotRoot(t *testing.T) {
	db := openUserSecurityProjectionTestDB(t)
	actor := seedUserSecurityProjectionUser(t, db, 1, "demoted-id-one", common.RoleAdminUser)
	target := seedUserSecurityProjectionUser(t, db, 20, "target", common.RoleCommonUser)
	eligible := seedUserSecurityProjectionUser(t, db, 21, "eligible", common.RoleCommonUser)
	peerRoot := seedUserSecurityProjectionUser(t, db, 30, "peer-root", common.RoleRootUser)

	seedUserSecurityProjectionEvent(t, db, "demoted-target", target.Id, target.Username, 100, "203.0.113.20", model.IPSecurityKindAPIRequest, "target")
	seedUserSecurityProjectionEvent(t, db, "demoted-eligible", eligible.Id, eligible.Username, 110, "203.0.113.20", model.IPSecurityKindLoginSuccess, "eligible")
	seedUserSecurityProjectionEvent(t, db, "demoted-peer", peerRoot.Id, peerRoot.Username, 120, "203.0.113.20", model.IPSecurityKindLoginSuccess, "peer")
	seedUserSecurityProjectionEvent(t, db, "demoted-orphan", 99, "orphan", 130, "203.0.113.20", model.IPSecurityKindLoginSuccess, "orphan")

	result := UserSecurityAudit{}
	require.NoError(t, queryUserSecurityProjection(context.Background(), actor, target, IPAuditQuery{
		StartTimestamp: 100, EndTimestamp: 200, Page: 1, PageSize: 20,
	}, &result))
	require.Equal(t, 1, result.Summary.RelatedUserCount)
	require.Empty(t, result.RelatedUsers)
}

func TestQueryUserSecurityProjectionOrdersEqualTimestampsByRequestIDThenID(t *testing.T) {
	db := openUserSecurityProjectionTestDB(t)
	actor := seedUserSecurityProjectionUser(t, db, 2, "root", common.RoleRootUser)
	target := seedUserSecurityProjectionUser(t, db, 20, "target", common.RoleCommonUser)

	seedUserSecurityProjectionEvent(t, db, "order-z-first", target.Id, target.Username, 100, "203.0.113.20", model.IPSecurityKindAPIRequest, "z")
	seedUserSecurityProjectionEvent(t, db, "order-a", target.Id, target.Username, 100, "203.0.113.20", model.IPSecurityKindLogout, "a")
	seedUserSecurityProjectionEvent(t, db, "order-z-second", target.Id, target.Username, 100, "203.0.113.20", model.IPSecurityKindAPIError, "z")

	result := UserSecurityAudit{}
	require.NoError(t, queryUserSecurityProjection(context.Background(), actor, target, IPAuditQuery{
		StartTimestamp: 100, EndTimestamp: 100, Page: 1, PageSize: 20,
	}, &result))
	require.Equal(t, []string{
		model.IPSecurityKindAPIError,
		model.IPSecurityKindAPIRequest,
		model.IPSecurityKindLogout,
	}, []string{result.Timeline[0].Kind, result.Timeline[1].Kind, result.Timeline[2].Kind})
}

func TestQueryUserSecurityProjectionPropagatesCanceledContext(t *testing.T) {
	db := openUserSecurityProjectionTestDB(t)
	actor := seedUserSecurityProjectionUser(t, db, 2, "root", common.RoleRootUser)
	target := seedUserSecurityProjectionUser(t, db, 20, "target", common.RoleCommonUser)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := queryUserSecurityProjection(ctx, actor, target, IPAuditQuery{Page: 1, PageSize: 20}, &UserSecurityAudit{})
	require.ErrorIs(t, err, context.Canceled)
}

func TestQueryUserSecurityProjectionHandlesMaxIntPageWithoutOverflow(t *testing.T) {
	db := openUserSecurityProjectionTestDB(t)
	actor := seedUserSecurityProjectionUser(t, db, 2, "root", common.RoleRootUser)
	target := seedUserSecurityProjectionUser(t, db, 20, "target", common.RoleCommonUser)
	seedUserSecurityProjectionEvent(t, db, "max-page", target.Id, target.Username, 100, "203.0.113.20", model.IPSecurityKindAPIRequest, "request")

	result := UserSecurityAudit{}
	require.NoError(t, queryUserSecurityProjection(context.Background(), actor, target, IPAuditQuery{
		StartTimestamp: 100, EndTimestamp: 100, Page: math.MaxInt, PageSize: 100,
	}, &result))
	require.Equal(t, 1, result.Total)
	require.Empty(t, result.Timeline)
}

func TestQueryUserSecurityProjectionFallsBackToRegistrationForRecentActivity(t *testing.T) {
	db := openUserSecurityProjectionTestDB(t)
	actor := seedUserSecurityProjectionUser(t, db, 2, "root", common.RoleRootUser)
	target := seedUserSecurityProjectionUser(t, db, 20, "target", common.RoleCommonUser)
	seedUserSecurityProjectionEvent(t, db, "registration-only", target.Id, target.Username, 100, "203.0.113.20", model.IPSecurityKindRegistration, "registration")

	result := UserSecurityAudit{}
	require.NoError(t, queryUserSecurityProjection(context.Background(), actor, target, IPAuditQuery{
		StartTimestamp: 100, EndTimestamp: 200, Page: 1, PageSize: 20,
	}, &result))
	require.Equal(t, "203.0.113.20", result.Summary.RegistrationIP)
	require.Equal(t, "203.0.113.20", result.Summary.RecentIP)
	require.EqualValues(t, 100, result.Summary.LastActivityAt)

	result = UserSecurityAudit{}
	require.NoError(t, queryUserSecurityProjection(context.Background(), actor, target, IPAuditQuery{
		StartTimestamp: 101, EndTimestamp: 200, Page: 1, PageSize: 20,
	}, &result))
	require.Empty(t, result.Summary.RegistrationIP)
	require.Empty(t, result.Summary.RecentIP)
	require.Zero(t, result.Summary.LastActivityAt)
}

func TestCheckedUserSecurityProjectionIntHonorsPlatformRange(t *testing.T) {
	value, err := checkedUserSecurityProjectionInt("count", math.MaxInt64)
	if strconv.IntSize == 64 {
		require.NoError(t, err)
		require.EqualValues(t, math.MaxInt64, value)
		return
	}
	require.ErrorContains(t, err, "count exceeds platform integer range")
}

func TestApplyUserSecurityProjectionSummaryExposesUnauthorizedCount(t *testing.T) {
	result := UserSecurityAudit{}
	err := applyUserSecurityProjectionSummary(model.UserIPAuditProjection{
		Summary: model.UserIPAuditProjectionSummary{UnauthorizedCount: 10},
	}, &result)
	require.NoError(t, err)
	payload, err := common.Marshal(result.Summary)
	require.NoError(t, err)
	var summary map[string]interface{}
	require.NoError(t, common.Unmarshal(payload, &summary))
	require.Equal(t, float64(10), summary["unauthorized_count"])
}
