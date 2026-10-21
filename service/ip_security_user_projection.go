package service

import (
	"context"
	"fmt"
	"math"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

func queryUserSecurityProjection(
	ctx context.Context,
	actor *model.User,
	target *model.User,
	query IPAuditQuery,
	result *UserSecurityAudit,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	isRoot := actor.Role >= common.RoleRootUser
	result.IsRoot = isRoot
	result.Projection = "masked"
	if isRoot {
		result.Projection = "full"
	}
	result.Page = query.Page
	result.PageSize = query.PageSize
	result.Range = query.Range
	result.StartTimestamp = query.StartTimestamp
	result.EndTimestamp = query.EndTimestamp
	result.Summary = UserSecurityAuditSummary{}
	result.RelatedUsers = make([]UserSecurityRelatedUser, 0)
	result.Timeline = make([]UserSecurityTimelineItem, 0)

	projection, err := model.QueryUserIPAuditProjection(
		ctx,
		actor.Id,
		actor.Role,
		target.Id,
		query.StartTimestamp,
		query.EndTimestamp,
		query.RelatedPage,
		query.RelatedPageSize,
	)
	if err != nil {
		return err
	}
	if err := applyUserSecurityProjectionSummary(projection, result); err != nil {
		return err
	}
	if err := applyUserSecurityProjectionRelations(projection, isRoot, result); err != nil {
		return err
	}

	if !isRoot {
		result.Total = 0
		return nil
	}

	var total int64
	if err := userSecurityTargetEvents(ctx, target.Id, query).Count(&total).Error; err != nil {
		return err
	}
	result.Total, err = checkedUserSecurityProjectionInt("total", total)
	if err != nil {
		return err
	}

	page := query.Page
	if page < 1 {
		page = 1
	}
	pageSize := query.PageSize
	if pageSize < 1 {
		pageSize = common.ItemsPerPage
	}
	if pageSize > 100 {
		pageSize = 100
	}
	result.Page = page
	result.PageSize = pageSize
	offset, pastEnd, err := userSecurityProjectionPageOffset(page, pageSize, total)
	if err != nil {
		return err
	}
	if pastEnd {
		return nil
	}

	var events []model.IPSecurityEvent
	if err := userSecurityTargetEvents(ctx, target.Id, query).
		Order("created_at DESC, request_id DESC, id DESC").
		Offset(offset).Limit(pageSize).Find(&events).Error; err != nil {
		return err
	}
	for _, event := range events {
		result.Timeline = append(result.Timeline, UserSecurityTimelineItem{
			CreatedAt: event.CreatedAt,
			IP:        event.IP,
			Kind:      event.Kind,
			RequestID: event.RequestID,
		})
	}
	return nil
}

func applyUserSecurityProjectionSummary(projection model.UserIPAuditProjection, result *UserSecurityAudit) error {
	apiRequestCount, err := checkedUserSecurityProjectionInt("api_request_count", projection.Summary.APIRequestCount)
	if err != nil {
		return err
	}
	errorCount, err := checkedUserSecurityProjectionInt("error_count", projection.Summary.ErrorCount)
	if err != nil {
		return err
	}
	unauthorizedCount, err := checkedUserSecurityProjectionInt("unauthorized_count", projection.Summary.UnauthorizedCount)
	if err != nil {
		return err
	}
	loginSuccessCount, err := checkedUserSecurityProjectionInt("login_success_count", projection.Summary.LoginSuccessCount)
	if err != nil {
		return err
	}
	loginFailureCount, err := checkedUserSecurityProjectionInt("login_failure_count", projection.Summary.LoginFailureCount)
	if err != nil {
		return err
	}
	logoutCount, err := checkedUserSecurityProjectionInt("logout_count", projection.Summary.LogoutCount)
	if err != nil {
		return err
	}
	anomalyCount, err := checkedUserSecurityProjectionInt("anomaly_count", projection.Summary.AnomalyCount)
	if err != nil {
		return err
	}
	result.Summary.RegistrationIP = projection.Summary.RegistrationIP
	result.Summary.RecentIP = projection.Summary.RecentIP
	result.Summary.APIRequestCount = apiRequestCount
	result.Summary.ErrorCount = errorCount
	result.Summary.UnauthorizedCount = unauthorizedCount
	result.Summary.LoginSuccessCount = loginSuccessCount
	result.Summary.LoginFailureCount = loginFailureCount
	result.Summary.LogoutCount = logoutCount
	result.Summary.AnomalyCount = anomalyCount
	result.Summary.LastActivityAt = projection.Summary.LastActivityAt
	return nil
}

func applyUserSecurityProjectionRelations(projection model.UserIPAuditProjection, includeIdentities bool, result *UserSecurityAudit) error {
	relatedTotal, err := checkedUserSecurityProjectionInt("related_total", projection.RelatedTotal)
	if err != nil {
		return err
	}
	result.Summary.RelatedUserCount = relatedTotal
	result.RelatedTotal = relatedTotal
	result.RelatedPage = projection.RelatedPage
	result.RelatedPageSize = projection.RelatedPageSize
	result.RelatedHasMore = projection.RelatedHasMore
	if !includeIdentities {
		return nil
	}
	for _, related := range projection.RelatedUsers {
		sharedIPCount, err := checkedUserSecurityProjectionInt("shared_ip_count", related.SharedIPCount)
		if err != nil {
			return err
		}
		registrationCount, err := checkedUserSecurityProjectionInt("registration_count", related.RegistrationCount)
		if err != nil {
			return err
		}
		apiCallCount, err := checkedUserSecurityProjectionInt("api_call_count", related.APICallCount)
		if err != nil {
			return err
		}
		authenticationCount, err := checkedUserSecurityProjectionInt("authentication_count", related.AuthenticationCount)
		if err != nil {
			return err
		}
		anomalyCount, err := checkedUserSecurityProjectionInt("related_anomaly_count", related.AnomalyCount)
		if err != nil {
			return err
		}
		result.RelatedUsers = append(result.RelatedUsers, UserSecurityRelatedUser{
			UserID: related.UserID, Username: related.Username,
			SharedIPCount: sharedIPCount, RegistrationCount: registrationCount,
			APICallCount: apiCallCount, AuthenticationCount: authenticationCount,
			AnomalyCount: anomalyCount, LastSeenAt: related.LastSeenAt,
		})
	}
	return nil
}

func userSecurityProjectionPageOffset(page, pageSize int, total int64) (int, bool, error) {
	pageIndex := int64(page - 1)
	if pageIndex > math.MaxInt64/int64(pageSize) {
		return 0, true, nil
	}
	offset := pageIndex * int64(pageSize)
	if offset >= total {
		return 0, true, nil
	}
	convertedOffset, err := checkedUserSecurityProjectionInt("page_offset", offset)
	return convertedOffset, false, err
}

func checkedUserSecurityProjectionInt(field string, value int64) (int, error) {
	maxInt := int64(^uint(0) >> 1)
	minInt := -maxInt - 1
	if value < minInt || value > maxInt {
		return 0, fmt.Errorf("user security projection field %s exceeds platform integer range", field)
	}
	return int(value), nil
}

func userSecurityTargetEvents(ctx context.Context, targetID int, query IPAuditQuery) *gorm.DB {
	return userSecurityEventRange(model.DB.WithContext(ctx).Model(&model.IPSecurityEvent{}), query).
		Where("user_id = ?", targetID)
}

func userSecurityEventRange(tx *gorm.DB, query IPAuditQuery) *gorm.DB {
	if query.StartTimestamp > 0 {
		tx = tx.Where("created_at >= ?", query.StartTimestamp)
	}
	if query.EndTimestamp > 0 {
		tx = tx.Where("created_at <= ?", query.EndTimestamp)
	}
	return tx
}
