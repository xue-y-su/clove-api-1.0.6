package model

import (
	"context"
	"errors"
)

func DeleteDeliveredAuditOutboxBatch(ctx context.Context, cutoff int64, limit int) (int64, error) {
	if DB == nil {
		return 0, errors.New("audit outbox database is not initialized")
	}
	if cutoff <= 0 {
		return 0, nil
	}
	if limit < 1 {
		limit = 100
	}
	if limit > 5000 {
		limit = 5000
	}

	var ids []int64
	if err := DB.WithContext(ctx).Model(&AuditOutboxEvent{}).
		Where("status = ? AND delivered_at > 0 AND delivered_at < ?", AuditOutboxStatusDelivered, cutoff).
		Order("delivered_at asc, id asc").Limit(limit).Pluck("id", &ids).Error; err != nil {
		return 0, err
	}
	if len(ids) == 0 {
		return 0, nil
	}
	result := DB.WithContext(ctx).
		Where("id IN ? AND status = ? AND delivered_at > 0 AND delivered_at < ?", ids, AuditOutboxStatusDelivered, cutoff).
		Delete(&AuditOutboxEvent{})
	return result.RowsAffected, result.Error
}
