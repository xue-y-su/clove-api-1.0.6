package model

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrInvalidAnnouncementAcknowledgement = errors.New("invalid announcement acknowledgement")

type AnnouncementAcknowledgement struct {
	ID             int64  `json:"id" gorm:"primaryKey;autoIncrement"`
	UserID         int    `json:"user_id" gorm:"not null;uniqueIndex:uniq_announcement_ack,priority:1"`
	Signature      string `json:"signature" gorm:"type:varchar(64);not null;uniqueIndex:uniq_announcement_ack,priority:2"`
	AcknowledgedAt int64  `json:"acknowledged_at" gorm:"bigint;not null;index"`
}

func RecordAnnouncementAcknowledgement(userID int, signature string, acknowledgedAt int64) (bool, error) {
	signature = strings.TrimSpace(signature)
	if userID <= 0 || len(signature) != 64 {
		return false, ErrInvalidAnnouncementAcknowledgement
	}
	if acknowledgedAt == 0 {
		acknowledgedAt = common.GetTimestamp()
	}

	acknowledgement := &AnnouncementAcknowledgement{
		UserID:         userID,
		Signature:      signature,
		AcknowledgedAt: acknowledgedAt,
	}
	if common.UsingMainDatabase(common.DatabaseTypeMySQL) {
		result := DB.Clauses(clause.Insert{Modifier: "IGNORE"}).Create(acknowledgement)
		return result.RowsAffected > 0, result.Error
	}
	result := DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "user_id"},
			{Name: "signature"},
		},
		DoNothing: true,
	}).Create(acknowledgement)
	return result.RowsAffected > 0, result.Error
}

func HasAnnouncementAcknowledgement(userID int, signature string) (bool, error) {
	if userID <= 0 {
		return false, nil
	}
	var acknowledgement AnnouncementAcknowledgement
	err := DB.Where("user_id = ? AND signature = ?", userID, strings.TrimSpace(signature)).First(&acknowledgement).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	return err == nil, err
}
