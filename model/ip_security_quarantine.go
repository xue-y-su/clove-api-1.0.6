package model

import (
	"context"
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

type IPSecurityProjectionMetrics struct {
	Status           string `json:"status"`
	PendingCount     int64  `json:"pending_count"`
	QuarantineCount  int64  `json:"quarantine_count"`
	LastQuarantineAt int64  `json:"last_quarantine_at"`
}

type IPSecurityProjectionQuarantine struct {
	ID             int64  `json:"id" gorm:"primaryKey;autoIncrement"`
	SourceKey      string `json:"source_key" gorm:"type:varchar(255);not null;uniqueIndex"`
	SourceType     string `json:"source_type" gorm:"type:varchar(32);not null;index"`
	SourceIdentity string `json:"source_identity" gorm:"type:varchar(191);not null"`
	Error          string `json:"error" gorm:"type:varchar(2000);not null"`
	AttemptCount   int    `json:"attempt_count" gorm:"not null;default:1"`
	FirstSeenAt    int64  `json:"first_seen_at" gorm:"bigint;not null;index"`
	LastSeenAt     int64  `json:"last_seen_at" gorm:"bigint;not null;index"`
}

func GetIPSecurityProjectionMetrics(ctx context.Context) (IPSecurityProjectionMetrics, error) {
	if DB == nil {
		return IPSecurityProjectionMetrics{}, errors.New("IP security projection database is not initialized")
	}
	metrics := IPSecurityProjectionMetrics{Status: IPSecurityProjectionStatusPending}
	var state IPSecurityProjectionState
	result := DB.WithContext(ctx).Where("id = ?", 1).Limit(1).Find(&state)
	if result.Error != nil {
		return IPSecurityProjectionMetrics{}, result.Error
	}
	if result.RowsAffected == 1 {
		metrics.Status = state.Status
	}
	if err := DB.WithContext(ctx).Model(&IPSecurityEvent{}).
		Where("projection_status = ?", IPSecurityEventProjectionPending).
		Count(&metrics.PendingCount).Error; err != nil {
		return IPSecurityProjectionMetrics{}, err
	}
	var quarantine struct {
		Count      int64 `gorm:"column:count"`
		LastSeenAt int64 `gorm:"column:last_seen_at"`
	}
	if err := DB.WithContext(ctx).Model(&IPSecurityProjectionQuarantine{}).
		Select("COUNT(*) AS count, COALESCE(MAX(last_seen_at), 0) AS last_seen_at").
		Scan(&quarantine).Error; err != nil {
		return IPSecurityProjectionMetrics{}, err
	}
	metrics.QuarantineCount = quarantine.Count
	metrics.LastQuarantineAt = quarantine.LastSeenAt
	return metrics, nil
}

func isQuarantinableIPSecurityProjectionError(err error) bool {
	return errors.Is(err, ErrInvalidIPSecurityEvent) || errors.Is(err, ErrIPSecurityEventConflict)
}

func recordIPSecurityProjectionQuarantineWithTx(
	tx *gorm.DB,
	sourceKey string,
	sourceType string,
	sourceIdentity string,
	sourceErr error,
) error {
	if tx == nil || sourceErr == nil || sourceKey == "" || len(sourceKey) > 255 || sourceType == "" || len(sourceType) > 32 || sourceIdentity == "" || len(sourceIdentity) > 191 {
		return errors.New("invalid IP security projection quarantine record")
	}
	message := strings.TrimSpace(sourceErr.Error())
	if len(message) > 2000 {
		message = message[:2000]
	}
	now := common.GetTimestamp()
	record := IPSecurityProjectionQuarantine{
		SourceKey: sourceKey, SourceType: sourceType, SourceIdentity: sourceIdentity,
		Error: message, AttemptCount: 1, FirstSeenAt: now, LastSeenAt: now,
	}
	result := tx.Model(&IPSecurityProjectionQuarantine{}).
		Where("source_key = ?", sourceKey).
		Updates(map[string]interface{}{
			"error":         message,
			"attempt_count": gorm.Expr("attempt_count + ?", 1),
			"last_seen_at":  now,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected > 0 {
		return nil
	}
	return tx.Create(&record).Error
}
