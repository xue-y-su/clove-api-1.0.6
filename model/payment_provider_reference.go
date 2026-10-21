package model

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrPaymentProviderReferenceConflict = errors.New("payment provider reference conflicts with an existing order")

type PaymentProviderReference struct {
	ID            int64  `json:"id"`
	Provider      string `json:"provider" gorm:"type:varchar(50);not null;uniqueIndex:uk_payment_provider_reference,priority:1"`
	ReferenceType string `json:"reference_type" gorm:"type:varchar(32);not null;uniqueIndex:uk_payment_provider_reference,priority:2"`
	ExternalID    string `json:"external_id" gorm:"type:varchar(191);not null;uniqueIndex:uk_payment_provider_reference,priority:3"`
	TradeNo       string `json:"trade_no" gorm:"type:varchar(255);not null;index"`
	CreatedAt     int64  `json:"created_at" gorm:"autoCreateTime"`
}

type PaymentProviderReferenceInput struct {
	Provider      string
	ReferenceType string
	ExternalID    string
	TradeNo       string
}

func RecordPaymentProviderReference(input PaymentProviderReferenceInput) error {
	if DB == nil {
		return ErrCashPaymentInvalid
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		return RecordPaymentProviderReferenceWithTx(tx, input)
	})
}

func RecordPaymentProviderReferenceWithTx(tx *gorm.DB, input PaymentProviderReferenceInput) error {
	input.Provider = strings.TrimSpace(input.Provider)
	input.ReferenceType = strings.TrimSpace(input.ReferenceType)
	input.ExternalID = strings.TrimSpace(input.ExternalID)
	input.TradeNo = strings.TrimSpace(input.TradeNo)
	if tx == nil || input.Provider == "" || len(input.Provider) > 50 || input.ReferenceType == "" || len(input.ReferenceType) > 32 ||
		input.ExternalID == "" || len(input.ExternalID) > 191 || input.TradeNo == "" || len(input.TradeNo) > 255 {
		return ErrCashPaymentInvalid
	}
	reference := PaymentProviderReference{
		Provider: input.Provider, ReferenceType: input.ReferenceType, ExternalID: input.ExternalID,
		TradeNo: input.TradeNo, CreatedAt: common.GetTimestamp(),
	}
	if err := tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "provider"}, {Name: "reference_type"}, {Name: "external_id"}},
		DoNothing: true,
	}).Create(&reference).Error; err != nil {
		return err
	}
	var stored PaymentProviderReference
	if err := tx.Where("provider = ? AND reference_type = ? AND external_id = ?", input.Provider, input.ReferenceType, input.ExternalID).First(&stored).Error; err != nil {
		return err
	}
	if stored.TradeNo != input.TradeNo {
		return ErrPaymentProviderReferenceConflict
	}
	return nil
}

func FindPaymentTradeNoByProviderReference(provider, referenceType, externalID string) (string, bool, error) {
	provider = strings.TrimSpace(provider)
	referenceType = strings.TrimSpace(referenceType)
	externalID = strings.TrimSpace(externalID)
	if DB == nil || provider == "" || referenceType == "" || externalID == "" {
		return "", false, ErrCashPaymentInvalid
	}
	var reference PaymentProviderReference
	err := DB.Where("provider = ? AND reference_type = ? AND external_id = ?", provider, referenceType, externalID).First(&reference).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return reference.TradeNo, true, nil
}
