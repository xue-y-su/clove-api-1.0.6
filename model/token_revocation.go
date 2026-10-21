package model

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

var ErrRevokedTokenFingerprintInvalid = errors.New("revoked token fingerprint is invalid")

type RevokedTokenFingerprint struct {
	ID          int64  `gorm:"primaryKey;autoIncrement"`
	Fingerprint string `gorm:"type:char(64);not null;uniqueIndex"`
	Reason      string `gorm:"type:varchar(64);not null"`
	CreatedAt   int64  `gorm:"bigint;not null"`
}

func RecordRevokedTokenFingerprint(tx *gorm.DB, fingerprint string, reason string) error {
	if tx == nil {
		return ErrDatabase
	}
	fingerprint = strings.TrimSpace(fingerprint)
	reason = strings.TrimSpace(reason)
	if len(fingerprint) != 64 || reason == "" || len(reason) > 64 {
		return ErrRevokedTokenFingerprintInvalid
	}
	return tx.Create(&RevokedTokenFingerprint{
		Fingerprint: fingerprint,
		Reason:      reason,
		CreatedAt:   common.GetTimestamp(),
	}).Error
}

func IsTokenFingerprintRevoked(tx *gorm.DB, fingerprint string) (bool, error) {
	if tx == nil {
		return false, ErrDatabase
	}
	fingerprint = strings.TrimSpace(fingerprint)
	if len(fingerprint) != 64 {
		return false, ErrRevokedTokenFingerprintInvalid
	}
	var record RevokedTokenFingerprint
	err := tx.Where("fingerprint = ?", fingerprint).First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}
