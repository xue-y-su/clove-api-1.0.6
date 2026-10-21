package model

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// PaymentRefund is the durable idempotency and accounting record for a
// provider refund or chargeback notification.
type PaymentRefund struct {
	ID                 int64  `json:"id" gorm:"primaryKey;autoIncrement"`
	Provider           string `json:"provider" gorm:"type:varchar(32);not null;index:idx_payment_refund_trade,priority:1;uniqueIndex:uniq_payment_refund_source,priority:1"`
	SourceType         string `json:"source_type" gorm:"type:varchar(32);not null;uniqueIndex:uniq_payment_refund_source,priority:2"`
	SourceID           string `json:"source_id" gorm:"type:varchar(191);not null;uniqueIndex:uniq_payment_refund_source,priority:3"`
	TradeNo            string `json:"trade_no" gorm:"type:varchar(255);not null;index:idx_payment_refund_trade,priority:2"`
	UserID             int    `json:"user_id" gorm:"not null;index"`
	ReportedAmount     string `json:"reported_amount" gorm:"type:varchar(64);not null"`
	AppliedAmount      string `json:"applied_amount" gorm:"type:varchar(64);not null"`
	RefundedCurrency   string `json:"refunded_currency" gorm:"type:varchar(16);not null"`
	ReversedQuota      int64  `json:"reversed_quota" gorm:"type:bigint;not null;default:0"`
	UserSubscriptionID int64  `json:"user_subscription_id" gorm:"type:bigint;not null;default:0;index"`
	CreatedAt          int64  `json:"created_at" gorm:"bigint;autoCreateTime"`
}

var (
	ErrPaymentRefundInvalid  = errors.New("payment refund is invalid")
	ErrPaymentRefundConflict = errors.New("payment refund conflicts with an existing source")
)

func normalizePaymentCommissionReversalInput(input PaymentCommissionReversalInput) (PaymentCommissionReversalInput, error) {
	input.TradeNo = strings.TrimSpace(input.TradeNo)
	input.ExpectedProvider = strings.TrimSpace(input.ExpectedProvider)
	input.RefundSourceType = strings.TrimSpace(input.RefundSourceType)
	input.RefundSourceID = strings.TrimSpace(input.RefundSourceID)
	input.RefundedAmount = strings.TrimSpace(input.RefundedAmount)
	input.RefundedCurrency = strings.ToUpper(strings.TrimSpace(input.RefundedCurrency))
	if input.TradeNo == "" || input.ExpectedProvider == "" || input.RefundSourceType == "" ||
		input.RefundSourceID == "" || input.RefundedAmount == "" || len(input.TradeNo) > 255 ||
		len(input.ExpectedProvider) > 32 || len(input.RefundSourceType) > 32 || len(input.RefundSourceID) > 191 ||
		len(input.RefundedCurrency) > 16 {
		return PaymentCommissionReversalInput{}, ErrPaymentRefundInvalid
	}
	amount, err := decimal.NewFromString(input.RefundedAmount)
	if err != nil || !amount.IsPositive() {
		return PaymentCommissionReversalInput{}, ErrPaymentRefundInvalid
	}
	return input, nil
}

// ProcessPaymentRefund applies principal or subscription entitlement reversal
// and invitation commission reversal in one idempotent database transaction.
func ProcessPaymentRefund(input PaymentCommissionReversalInput) (bool, error) {
	var err error
	input, err = normalizePaymentCommissionReversalInput(input)
	if err != nil || DB == nil {
		return false, ErrPaymentRefundInvalid
	}
	created := false
	var userID int
	var commissionRecipientUserID int
	var cacheGroup string
	err = DB.Transaction(func(tx *gorm.DB) error {
		var topUp TopUp
		if err := LockTopUpByTradeNo(tx, input.TradeNo, &topUp); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrTopUpNotFound
			}
			return err
		}
		if topUp.PaymentProvider != input.ExpectedProvider {
			return ErrPaymentMethodMismatch
		}
		if topUp.Status != common.TopUpStatusSuccess {
			return ErrTopUpStatusInvalid
		}
		currency := strings.ToUpper(strings.TrimSpace(topUp.PaidCurrency))
		if currency == "" {
			currency = input.RefundedCurrency
		}
		if input.RefundedCurrency == "" {
			input.RefundedCurrency = currency
		}
		if currency == "" || currency != input.RefundedCurrency {
			return ErrPaymentRefundInvalid
		}
		paidAmount, err := decimal.NewFromString(strings.TrimSpace(topUp.PaidAmount))
		if err != nil || !paidAmount.IsPositive() {
			return ErrPaymentRefundInvalid
		}
		reportedAmount, err := decimal.NewFromString(input.RefundedAmount)
		if err != nil || !reportedAmount.IsPositive() {
			return ErrPaymentRefundInvalid
		}

		var existing PaymentRefund
		result := lockForUpdate(tx).Where("provider = ? AND source_type = ? AND source_id = ?", input.ExpectedProvider, input.RefundSourceType, input.RefundSourceID).First(&existing)
		if result.Error == nil {
			if existing.TradeNo != input.TradeNo || existing.ReportedAmount != reportedAmount.String() || existing.RefundedCurrency != currency {
				return ErrPaymentRefundConflict
			}
			return nil
		}
		if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return result.Error
		}

		var previous []PaymentRefund
		if err := lockForUpdate(tx).Where("trade_no = ?", input.TradeNo).Order("id asc").Find(&previous).Error; err != nil {
			return err
		}
		refundedTotal := decimal.Zero
		var previouslyReversedQuota int64
		for i := range previous {
			if strings.ToUpper(previous[i].RefundedCurrency) != currency {
				return ErrPaymentRefundConflict
			}
			amount, err := decimal.NewFromString(previous[i].AppliedAmount)
			if err != nil || amount.IsNegative() {
				return ErrPaymentRefundConflict
			}
			refundedTotal = refundedTotal.Add(amount)
			if previous[i].ReversedQuota < 0 || (previouslyReversedQuota > 0 && previous[i].ReversedQuota > mathMaxInt64-previouslyReversedQuota) {
				return ErrPaymentRefundConflict
			}
			previouslyReversedQuota += previous[i].ReversedQuota
		}
		remaining := paidAmount.Sub(refundedTotal)
		if remaining.IsNegative() {
			remaining = decimal.Zero
		}
		appliedAmount := reportedAmount
		if appliedAmount.GreaterThan(remaining) {
			appliedAmount = remaining
		}
		if appliedAmount.IsNegative() {
			appliedAmount = decimal.Zero
		}

		var order SubscriptionOrder
		orderResult := lockForUpdate(tx).Where("trade_no = ?", input.TradeNo).First(&order)
		isSubscription := orderResult.Error == nil
		quotaDelta := int64(0)
		if orderResult.Error != nil && !errors.Is(orderResult.Error, gorm.ErrRecordNotFound) {
			return orderResult.Error
		}
		if isSubscription {
			if order.PaymentProvider != input.ExpectedProvider || order.Status != common.TopUpStatusSuccess || order.UserId != topUp.UserId {
				return ErrPaymentRefundInvalid
			}
			userID = order.UserId
			if order.UserSubscriptionID <= 0 {
				var candidates []UserSubscription
				if err := tx.Where("user_id = ? AND plan_id = ? AND source = ? AND created_at BETWEEN ? AND ?", order.UserId, order.PlanId, "order", order.CompleteTime-5, order.CompleteTime+5).Find(&candidates).Error; err != nil {
					return err
				}
				if len(candidates) != 1 {
					return ErrPaymentRefundInvalid
				}
				order.UserSubscriptionID = int64(candidates[0].Id)
			}
			if _, activeErr := lockUserRelayReservationThenSubscriptionForUpdateTx(tx, order.UserId, int(order.UserSubscriptionID)); activeErr != nil {
				return activeErr
			}
			var sub UserSubscription
			if err := lockForUpdate(tx).First(&sub, order.UserSubscriptionID).Error; err != nil {
				return err
			}
			if appliedAmount.IsPositive() && sub.Status == "active" {
				now := common.GetTimestamp()
				if err := tx.Model(&sub).Updates(map[string]any{"status": "cancelled", "end_time": now, "updated_at": now}).Error; err != nil {
					return err
				}
				cacheGroup, err = downgradeUserGroupForSubscriptionTx(tx, &sub, now)
				if err != nil {
					return err
				}
			}
		} else {
			userID = topUp.UserId
			originalQuota, err := originalTopUpQuotaTx(tx, &topUp)
			if err != nil {
				return err
			}
			cumulative := refundedTotal.Add(appliedAmount)
			targetQuotaBig := decimal.NewFromInt(originalQuota).
				Mul(cumulative).Div(paidAmount).Truncate(0).BigInt()
			if !targetQuotaBig.IsInt64() || targetQuotaBig.Sign() < 0 {
				return ErrPaymentRefundInvalid
			}
			targetQuota := targetQuotaBig.Int64()
			quotaDelta = targetQuota - previouslyReversedQuota
			if quotaDelta < 0 {
				return ErrPaymentRefundConflict
			}
			if quotaDelta > 0 {
				operationID := "payment-refund:" + common.Sha1([]byte(input.ExpectedProvider+"\x00"+input.RefundSourceType+"\x00"+input.RefundSourceID))
				if _, err := AdjustWalletAllowDebtWithTx(tx, userID, -quotaDelta, operationID, "payment_refund", WalletAdjustReference{Type: "payment_refund", ID: input.TradeNo, Context: input.RefundSourceType}); err != nil {
					return err
				}
			}
		}

		if appliedAmount.IsPositive() {
			commissionInput := input
			commissionInput.RefundedAmount = appliedAmount.String()
			_, recipientUserID, err := reversePaymentInvitationCommissionTx(tx, &topUp, commissionInput)
			if err != nil {
				return err
			}
			commissionRecipientUserID = recipientUserID
		}
		record := PaymentRefund{
			Provider: input.ExpectedProvider, SourceType: input.RefundSourceType, SourceID: input.RefundSourceID,
			TradeNo: input.TradeNo, UserID: userID, ReportedAmount: reportedAmount.String(),
			AppliedAmount: appliedAmount.String(), RefundedCurrency: currency, ReversedQuota: quotaDelta,
		}
		if isSubscription {
			record.UserSubscriptionID = order.UserSubscriptionID
		}
		createdResult := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "provider"}, {Name: "source_type"}, {Name: "source_id"}}, DoNothing: true}).Create(&record)
		if createdResult.Error != nil {
			return createdResult.Error
		}
		if createdResult.RowsAffected != 1 {
			return nil
		}
		created = true
		return nil
	})
	if err != nil {
		return false, err
	}
	if created && userID > 0 {
		_ = InvalidateUserCache(userID)
		if cacheGroup != "" {
			_ = UpdateUserGroupCache(userID, cacheGroup)
		}
	}
	if created && commissionRecipientUserID > 0 && commissionRecipientUserID != userID {
		_ = InvalidateUserCache(commissionRecipientUserID)
	}
	return created, nil
}

const mathMaxInt64 = int64(^uint64(0) >> 1)

func originalTopUpQuotaTx(tx *gorm.DB, topUp *TopUp) (int64, error) {
	if tx == nil || topUp == nil {
		return 0, ErrPaymentRefundInvalid
	}
	var entry WalletLedgerEntry
	result := lockForUpdate(tx).Where("operation_id = ?", "topup:"+topUp.TradeNo).First(&entry)
	if result.Error == nil {
		if entry.UserID != topUp.UserId || entry.Delta <= 0 || entry.Category != "topup" {
			return 0, ErrPaymentRefundConflict
		}
		return entry.Delta, nil
	}
	if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return 0, result.Error
	}
	quota, err := cashTopUpCreditedQuota(topUp)
	if err != nil {
		return 0, err
	}
	return int64(quota), nil
}
