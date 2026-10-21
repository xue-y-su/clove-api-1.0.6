package model

import (
	"context"
	"database/sql"
	"errors"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/ip_security_setting"
	"gorm.io/gorm"
)

type ipSecurityRollupIdentity struct {
	UserID int
	IP     string
}

func IPSecurityProjectionRetentionCutoff(now int64) (int64, bool) {
	return IPSecurityProjectionRetentionCutoffForDays(now, ip_security_setting.GetSettings().RetentionDays)
}

func IPSecurityProjectionRetentionCutoffForDays(now int64, retentionDays int) (int64, bool) {
	if now <= 0 {
		return 0, false
	}
	if retentionDays < 0 {
		retentionDays = 0
	}
	if retentionDays > 3650 {
		retentionDays = 3650
	}
	var cutoff int64
	enabled := false
	if retentionDays > 0 {
		cutoff = now - int64(retentionDays)*86400
		enabled = true
	}
	if !common.UsingLogDatabase(common.DatabaseTypeClickHouse) {
		return cutoff, enabled
	}
	ttlDays := clickHouseLogTTLDays()
	if ttlDays <= 0 {
		return cutoff, enabled
	}
	ttlCutoff := now - int64(ttlDays)*86400
	if !enabled || ttlCutoff > cutoff {
		cutoff = ttlCutoff
	}
	return cutoff, true
}

func DeleteOldIPSecurityProjectionBatch(ctx context.Context, targetTimestamp int64, limit int) (int64, error) {
	return deleteOldIPSecurityProjectionBatch(ctx, targetTimestamp, limit)
}

func countOldIPSecurityProjection(ctx context.Context, targetTimestamp int64) (int64, error) {
	if DB == nil {
		return 0, errors.New("IP security projection database is not initialized")
	}
	dayCutoff := utcDayBucket(targetTimestamp)
	queries := []*struct {
		model interface{}
		where string
		args  []interface{}
	}{
		{model: &IPSecurityEvent{}, where: "created_at < ? AND projection_status IN ?", args: []interface{}{targetTimestamp, []string{IPSecurityEventProjectionPending, IPSecurityEventProjectionProjected}}},
		{model: &IPSecurityProjectionQuarantine{}, where: "last_seen_at < ?", args: []interface{}{targetTimestamp}},
		{model: &IPAuditRollup{}, where: "bucket > 0 AND bucket < ?", args: []interface{}{dayCutoff}},
		{model: &IPAuditRollup{}, where: "bucket = 0 AND last_seen_at < ?", args: []interface{}{targetTimestamp}},
	}
	var total int64
	for _, query := range queries {
		var count int64
		if err := DB.WithContext(ctx).Model(query.model).Where(query.where, query.args...).Count(&count).Error; err != nil {
			return 0, err
		}
		total += count
	}
	return total, nil
}

func deleteOldIPSecurityProjectionBatch(ctx context.Context, targetTimestamp int64, limit int) (int64, error) {
	if DB == nil {
		return 0, errors.New("IP security projection database is not initialized")
	}
	if limit < 1 {
		limit = 100
	}
	dayCutoff := utcDayBucket(targetTimestamp)
	var deleted int64

	var transactionOptions []*sql.TxOptions
	if common.UsingMainDatabase(common.DatabaseTypeMySQL) {
		transactionOptions = append(transactionOptions, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	}
	if err := DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var oldEvents []IPSecurityEvent
		if err := lockForUpdate(tx).Model(&IPSecurityEvent{}).Select("id", "user_id", "ip").
			Where("created_at < ? AND projection_status = ?", targetTimestamp, IPSecurityEventProjectionProjected).
			Order("id asc").Limit(limit).Find(&oldEvents).Error; err != nil {
			return err
		}
		if len(oldEvents) == 0 {
			return nil
		}

		eventIDs := make([]int64, 0, len(oldEvents))
		identities := make([]ipSecurityRollupIdentity, 0, len(oldEvents))
		seenIdentities := make(map[ipSecurityRollupIdentity]struct{}, len(oldEvents))
		for _, event := range oldEvents {
			eventIDs = append(eventIDs, event.ID)
			identity := ipSecurityRollupIdentity{UserID: event.UserID, IP: event.IP}
			if _, seen := seenIdentities[identity]; !seen {
				seenIdentities[identity] = struct{}{}
				identities = append(identities, identity)
			}
		}

		result := tx.Where("id IN ? AND created_at < ? AND projection_status = ?", eventIDs, targetTimestamp, IPSecurityEventProjectionProjected).Delete(&IPSecurityEvent{})
		if result.Error != nil {
			return result.Error
		}
		deleted += result.RowsAffected
		for _, identity := range identities {
			if err := rebuildRetainedIPAuditLifetime(tx, identity, targetTimestamp); err != nil {
				return err
			}
		}
		return nil
	}, transactionOptions...); err != nil {
		return 0, err
	}
	if deleted >= int64(limit) {
		return deleted, nil
	}

	remaining := limit - int(deleted)
	var pendingEventIDs []int64
	if err := DB.WithContext(ctx).Model(&IPSecurityEvent{}).
		Where("created_at < ? AND projection_status = ?", targetTimestamp, IPSecurityEventProjectionPending).
		Order("id asc").Limit(remaining).Pluck("id", &pendingEventIDs).Error; err != nil {
		return deleted, err
	}
	if len(pendingEventIDs) > 0 {
		result := DB.WithContext(ctx).
			Where("id IN ? AND created_at < ? AND projection_status = ?", pendingEventIDs, targetTimestamp, IPSecurityEventProjectionPending).
			Delete(&IPSecurityEvent{})
		if result.Error != nil {
			return deleted, result.Error
		}
		deleted += result.RowsAffected
	}
	if deleted >= int64(limit) {
		return deleted, nil
	}

	remaining = limit - int(deleted)
	var quarantineIDs []int64
	if err := DB.WithContext(ctx).Model(&IPSecurityProjectionQuarantine{}).
		Where("last_seen_at < ?", targetTimestamp).
		Order("id asc").Limit(remaining).Pluck("id", &quarantineIDs).Error; err != nil {
		return deleted, err
	}
	if len(quarantineIDs) > 0 {
		result := DB.WithContext(ctx).
			Where("id IN ? AND last_seen_at < ?", quarantineIDs, targetTimestamp).
			Delete(&IPSecurityProjectionQuarantine{})
		if result.Error != nil {
			return deleted, result.Error
		}
		deleted += result.RowsAffected
	}
	if deleted >= int64(limit) {
		return deleted, nil
	}

	remaining = limit - int(deleted)
	var dailyIDs []int64
	if err := DB.WithContext(ctx).Model(&IPAuditRollup{}).
		Where("bucket > 0 AND bucket < ?", dayCutoff).
		Order("id asc").Limit(remaining).Pluck("id", &dailyIDs).Error; err != nil {
		return deleted, err
	}
	if len(dailyIDs) > 0 {
		result := DB.WithContext(ctx).Where("id IN ?", dailyIDs).Delete(&IPAuditRollup{})
		if result.Error != nil {
			return deleted, result.Error
		}
		deleted += result.RowsAffected
	}
	if deleted >= int64(limit) {
		return deleted, nil
	}

	remaining = limit - int(deleted)
	var lifetimeIDs []int64
	if err := DB.WithContext(ctx).Model(&IPAuditRollup{}).
		Where("bucket = 0 AND last_seen_at < ?", targetTimestamp).
		Order("id asc").Limit(remaining).Pluck("id", &lifetimeIDs).Error; err != nil {
		return deleted, err
	}
	if len(lifetimeIDs) > 0 {
		result := DB.WithContext(ctx).
			Where("id IN ? AND bucket = 0 AND last_seen_at < ?", lifetimeIDs, targetTimestamp).
			Delete(&IPAuditRollup{})
		if result.Error != nil {
			return deleted, result.Error
		}
		deleted += result.RowsAffected
	}
	return deleted, nil
}

func rebuildRetainedIPAuditLifetime(tx *gorm.DB, identity ipSecurityRollupIdentity, targetTimestamp int64) error {
	var lifetime IPAuditRollup
	if err := lockForUpdate(tx).
		Where("bucket = 0 AND user_id = ? AND ip = ?", identity.UserID, identity.IP).
		First(&lifetime).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}

	retainedEvents := func() *gorm.DB {
		return tx.Model(&IPSecurityEvent{}).
			Where("user_id = ? AND ip = ? AND created_at >= ? AND projection_status = ?",
				identity.UserID, identity.IP, targetTimestamp, IPSecurityEventProjectionProjected).
			Where("kind IN ?", []string{
				IPSecurityKindRegistration,
				IPSecurityKindAPIRequest,
				IPSecurityKindAPIError,
				IPSecurityKindLoginSuccess,
				IPSecurityKindLoginFailure,
				IPSecurityKindLogout,
			}).
			Where("(kind <> ? OR http_status = ?)", IPSecurityKindAPIError, http.StatusUnauthorized)
	}
	var latest IPSecurityEvent
	if err := retainedEvents().Select("username", "masked_ip").
		Order("created_at desc").Order("id desc").First(&latest).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}

	var retained IPAuditRollup
	if err := retainedEvents().Select(`
		COALESCE(SUM(CASE WHEN kind <> ? THEN 1 ELSE 0 END), 0) AS request_count,
		COALESCE(SUM(CASE WHEN kind = ? THEN 1 ELSE 0 END), 0) AS registration_count,
		COALESCE(SUM(CASE WHEN kind = ? THEN 1 ELSE 0 END), 0) AS api_request_count,
		COALESCE(SUM(CASE WHEN kind = ? AND http_status = ? THEN 1 ELSE 0 END), 0) AS error_count,
		COALESCE(SUM(CASE WHEN kind = ? AND http_status = ? THEN 1 ELSE 0 END), 0) AS unauthorized_count,
		COALESCE(SUM(CASE WHEN kind = ? THEN 1 ELSE 0 END), 0) AS login_success_count,
		COALESCE(SUM(CASE WHEN kind = ? THEN 1 ELSE 0 END), 0) AS login_failure_count,
		COALESCE(SUM(CASE WHEN kind = ? THEN 1 ELSE 0 END), 0) AS logout_count,
		COALESCE(SUM(CASE WHEN (kind = ? AND http_status = ?) OR kind = ? THEN 1 ELSE 0 END), 0) AS anomaly_count,
		COALESCE(SUM(quota), 0) AS quota,
		COALESCE(MIN(created_at), 0) AS first_seen_at,
		COALESCE(MAX(created_at), 0) AS last_seen_at`,
		IPSecurityKindRegistration,
		IPSecurityKindRegistration,
		IPSecurityKindAPIRequest,
		IPSecurityKindAPIError, http.StatusUnauthorized,
		IPSecurityKindAPIError, http.StatusUnauthorized,
		IPSecurityKindLoginSuccess,
		IPSecurityKindLoginFailure,
		IPSecurityKindLogout,
		IPSecurityKindAPIError, http.StatusUnauthorized,
		IPSecurityKindLoginFailure,
	).Scan(&retained).Error; err != nil {
		return err
	}

	return tx.Model(&IPAuditRollup{}).Where("id = ?", lifetime.ID).Updates(map[string]interface{}{
		"username":            latest.Username,
		"masked_ip":           latest.MaskedIP,
		"request_count":       retained.RequestCount,
		"registration_count":  retained.RegistrationCount,
		"api_request_count":   retained.APIRequestCount,
		"error_count":         retained.ErrorCount,
		"unauthorized_count":  retained.UnauthorizedCount,
		"login_success_count": retained.LoginSuccessCount,
		"login_failure_count": retained.LoginFailureCount,
		"logout_count":        retained.LogoutCount,
		"anomaly_count":       retained.AnomalyCount,
		"quota":               retained.Quota,
		"first_seen_at":       retained.FirstSeenAt,
		"last_seen_at":        retained.LastSeenAt,
	}).Error
}
