package model

import (
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	relayFundingReservationActive   = "active"
	relayFundingReservationSettled  = "settled"
	relayFundingReservationRefunded = "refunded"
)

// RelayFundingReservation stores the cumulative subscription and wallet
// allocation for one relay request. Both balances are changed in the same DB
// transaction, so a partial subscription reservation never leaves an orphaned
// wallet debit (or vice versa).
type RelayFundingReservation struct {
	RequestID                string `json:"request_id" gorm:"primaryKey;type:varchar(64)"`
	UserID                   int    `json:"user_id" gorm:"not null;index"`
	UserSubscriptionID       int    `json:"user_subscription_id" gorm:"not null;index"`
	SubscriptionQuota        int64  `json:"subscription_quota" gorm:"type:bigint;not null;default:0"`
	SubscriptionAppliedQuota int64  `json:"subscription_applied_quota" gorm:"type:bigint;not null;default:0"`
	WalletQuota              int64  `json:"wallet_quota" gorm:"type:bigint;not null;default:0"`
	Revision                 int64  `json:"revision" gorm:"type:bigint;not null;default:0"`
	Status                   string `json:"status" gorm:"type:varchar(16);not null;index"`
	CreatedAt                int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt                int64  `json:"updated_at" gorm:"bigint;index"`
}

func (r *RelayFundingReservation) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	r.CreatedAt = now
	r.UpdatedAt = now
	return nil
}

func (r *RelayFundingReservation) BeforeUpdate(tx *gorm.DB) error {
	r.UpdatedAt = common.GetTimestamp()
	return nil
}

type RelayFundingReservationResult struct {
	UserSubscriptionID int
	SubscriptionQuota  int64
	WalletQuota        int64
	AmountTotal        int64
	AmountUsedBefore   int64
	AmountUsedAfter    int64
	PlanID             int
	PlanTitle          string
}

func SetRelayFundingReservationTarget(
	requestID string,
	userID int,
	target int64,
	eligibility SubscriptionEligibility,
	permitWalletOverflow bool,
) (*RelayFundingReservationResult, error) {
	return setRelayFundingReservationTarget(nil, requestID, userID, target, eligibility, permitWalletOverflow, relayFundingReservationActive)
}

// SetRelayFundingReservationTargetWithTx applies a cumulative funding target
// inside the caller's transaction. Cache invalidation remains the caller's
// responsibility after the transaction commits.
func SetRelayFundingReservationTargetWithTx(
	tx *gorm.DB,
	requestID string,
	userID int,
	target int64,
	eligibility SubscriptionEligibility,
	permitWalletOverflow bool,
) (*RelayFundingReservationResult, error) {
	if tx == nil {
		return nil, errors.New("tx is nil")
	}
	return setRelayFundingReservationTarget(tx, requestID, userID, target, eligibility, permitWalletOverflow, relayFundingReservationActive)
}

func SettleRelayFundingReservation(
	requestID string,
	userID int,
	target int64,
	eligibility SubscriptionEligibility,
	permitWalletOverflow bool,
) (*RelayFundingReservationResult, error) {
	return setRelayFundingReservationTarget(nil, requestID, userID, target, eligibility, permitWalletOverflow, relayFundingReservationSettled)
}

func SettleRelayFundingReservationWithTx(
	tx *gorm.DB,
	requestID string,
	userID int,
	target int64,
	eligibility SubscriptionEligibility,
	permitWalletOverflow bool,
) (*RelayFundingReservationResult, error) {
	if tx == nil {
		return nil, errors.New("tx is nil")
	}
	return setRelayFundingReservationTarget(tx, requestID, userID, target, eligibility, permitWalletOverflow, relayFundingReservationSettled)
}

func RefundRelayFundingReservation(requestID string) error {
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		return errors.New("requestId is empty")
	}
	_, err := setRelayFundingReservationTarget(nil, requestID, 0, 0, nil, true, relayFundingReservationRefunded)
	return err
}

// RefundRelayFundingReservationWithTx releases a reservation and persists its
// refunded state in the same caller-owned transaction.
func RefundRelayFundingReservationWithTx(tx *gorm.DB, requestID string) error {
	if tx == nil {
		return errors.New("tx is nil")
	}
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		return errors.New("requestId is empty")
	}
	_, err := setRelayFundingReservationTarget(tx, requestID, 0, 0, nil, true, relayFundingReservationRefunded)
	return err
}

func CleanupRelayFundingReservations(olderThanSeconds int64) (int64, error) {
	if olderThanSeconds <= 0 {
		olderThanSeconds = 7 * 24 * 3600
	}
	cutoff := GetDBTimestamp() - olderThanSeconds
	res := DB.Where("updated_at < ? AND status IN ?", cutoff, []string{
		relayFundingReservationSettled,
		relayFundingReservationRefunded,
	}).Delete(&RelayFundingReservation{})
	return res.RowsAffected, res.Error
}

func lockActiveRelayFundingReservationsForUserTx(tx *gorm.DB, userID int) error {
	if tx == nil || userID <= 0 {
		return errors.New("invalid relay funding reservation lock args")
	}
	var reservations []struct{ RequestID string }
	return lockForUpdate(tx.Model(&RelayFundingReservation{})).
		Select("request_id").
		Where("user_id = ? AND status = ?", userID, relayFundingReservationActive).
		Order("request_id asc").
		Find(&reservations).Error
}

func resetRelayFundingReservationCycleTx(tx *gorm.DB, userSubscriptionID int) error {
	if tx == nil || userSubscriptionID <= 0 {
		return nil
	}
	return tx.Model(&RelayFundingReservation{}).
		Where("user_subscription_id = ? AND status = ?", userSubscriptionID, relayFundingReservationActive).
		Update("subscription_applied_quota", 0).Error
}

func setRelayFundingReservationTarget(
	outerTx *gorm.DB,
	requestID string,
	userID int,
	target int64,
	eligibility SubscriptionEligibility,
	permitWalletOverflow bool,
	targetStatus string,
) (*RelayFundingReservationResult, error) {
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		return nil, errors.New("requestId is empty")
	}
	if target < 0 {
		return nil, errors.New("target must be >= 0")
	}
	if targetStatus != relayFundingReservationActive &&
		targetStatus != relayFundingReservationSettled &&
		targetStatus != relayFundingReservationRefunded {
		return nil, errors.New("invalid relay funding reservation target status")
	}

	result := &RelayFundingReservationResult{}
	mutatedUserID := userID
	runMutation := func(fn func(tx *gorm.DB) error) error {
		return DB.Transaction(fn)
	}
	if outerTx != nil {
		runMutation = func(fn func(tx *gorm.DB) error) error {
			return fn(outerTx)
		}
	}
	err := runMutation(func(tx *gorm.DB) error {
		reservation := RelayFundingReservation{}
		query := tx.Where("request_id = ?", requestID).Limit(1).Find(&reservation)
		if query.Error != nil {
			return query.Error
		}
		hasReservation := query.RowsAffected > 0
		if hasReservation {
			if userID > 0 && reservation.UserID != userID {
				return errors.New("relay funding reservation belongs to another user")
			}
			userID = reservation.UserID
			mutatedUserID = userID
			if reservation.Status == relayFundingReservationRefunded {
				if targetStatus == relayFundingReservationRefunded && target == 0 {
					return nil
				}
				return errors.New("relay funding reservation already refunded")
			}
			if reservation.Status == relayFundingReservationSettled &&
				(targetStatus != relayFundingReservationSettled || target != reservation.SubscriptionQuota+reservation.WalletQuota) {
				return errors.New("relay funding reservation already settled")
			}
			if reservation.Status != "" && reservation.Status != relayFundingReservationActive &&
				reservation.Status != relayFundingReservationSettled {
				return errors.New("relay funding reservation has invalid status")
			}
		}
		if userID <= 0 {
			return errors.New("invalid userId")
		}

		// Serialize every funding mutation for the user before selecting a
		// subscription or writing the wallet ledger.
		var lockedUser struct {
			ID    int
			Quota int64
		}
		if err := lockForUpdate(tx.Model(&User{})).Select("id", "quota").First(&lockedUser, userID).Error; err != nil {
			return err
		}

		// Re-read after the user lock so concurrent first reservations for the
		// same request observe the committed cumulative target.
		query = lockForUpdate(tx).Where("request_id = ?", requestID).Limit(1).Find(&reservation)
		if query.Error != nil {
			return query.Error
		}
		hasReservation = query.RowsAffected > 0
		if hasReservation && reservation.Status == relayFundingReservationRefunded {
			if targetStatus == relayFundingReservationRefunded && target == 0 {
				return nil
			}
			return errors.New("relay funding reservation already refunded")
		}
		if hasReservation && reservation.Status == relayFundingReservationSettled &&
			(targetStatus != relayFundingReservationSettled || target != reservation.SubscriptionQuota+reservation.WalletQuota) {
			return errors.New("relay funding reservation already settled")
		}
		if hasReservation && reservation.Status != "" && reservation.Status != relayFundingReservationActive &&
			reservation.Status != relayFundingReservationSettled {
			return errors.New("relay funding reservation has invalid status")
		}
		if err := lockActiveRelayFundingReservationsForUserTx(tx, userID); err != nil {
			return err
		}

		if !hasReservation && target == 0 {
			return nil
		}

		var subscription UserSubscription
		var plan *SubscriptionPlan
		if hasReservation && reservation.UserSubscriptionID > 0 {
			if err := lockForUpdate(tx).Where("id = ?", reservation.UserSubscriptionID).First(&subscription).Error; err != nil {
				return err
			}
			var err error
			plan, err = getSubscriptionPlanByIdTx(tx, subscription.PlanId)
			if err != nil {
				return err
			}
		} else {
			var candidates []UserSubscription
			now := getDBTimestampTx(tx)
			if err := lockForUpdate(tx).
				Where("user_id = ? AND status = ? AND end_time > ?", userID, "active", now).
				Order("end_time asc, id asc").
				Find(&candidates).Error; err != nil {
				return fmt.Errorf("query active subscriptions: %w", err)
			}
			bestCapacity := int64(-1)
			eligibleCount := 0
			for i := range candidates {
				candidate := candidates[i]
				if eligibility != nil && !eligibility(strings.TrimSpace(candidate.UpgradeGroup)) {
					continue
				}
				eligibleCount++
				candidatePlan, err := getSubscriptionPlanByIdTx(tx, candidate.PlanId)
				if err != nil {
					return err
				}
				if err := maybeResetUserSubscriptionWithPlanTx(tx, &candidate, candidatePlan, now); err != nil {
					return err
				}
				capacity := target
				if candidate.AmountTotal > 0 {
					capacity = candidate.AmountTotal - candidate.AmountUsed
					if capacity < 0 {
						capacity = 0
					}
				}
				if capacity >= target {
					subscription = candidate
					plan = candidatePlan
					break
				}
				if capacity > bestCapacity {
					bestCapacity = capacity
					subscription = candidate
					plan = candidatePlan
				}
			}
			if eligibleCount == 0 || subscription.Id == 0 {
				return ErrNoActiveSubscription
			}
		}

		currentSubscription := reservation.SubscriptionQuota
		currentAppliedSubscription := reservation.SubscriptionAppliedQuota
		if currentAppliedSubscription < 0 {
			currentAppliedSubscription = 0
		}
		if currentAppliedSubscription > currentSubscription {
			currentAppliedSubscription = currentSubscription
		}
		legacySubscription := currentSubscription - currentAppliedSubscription
		currentWallet := reservation.WalletQuota
		usedBefore := subscription.AmountUsed
		capacityForRequest := target
		if subscription.AmountTotal > 0 {
			usedByOthers := subscription.AmountUsed - currentAppliedSubscription
			if usedByOthers < 0 {
				usedByOthers = 0
			}
			capacityForRequest = legacySubscription + subscription.AmountTotal - usedByOthers
			if capacityForRequest < 0 {
				capacityForRequest = 0
			}
		}
		desiredSubscription := target
		if desiredSubscription > capacityForRequest {
			desiredSubscription = capacityForRequest
		}
		desiredAppliedSubscription := desiredSubscription - legacySubscription
		if desiredAppliedSubscription < 0 {
			desiredAppliedSubscription = 0
		}
		desiredWallet := target - desiredSubscription
		if desiredWallet > 0 && (!permitWalletOverflow || !subscription.AllowWalletOverflow) {
			return fmt.Errorf("%w: need=%d available=%d", ErrSubscriptionQuotaInsufficient, target, capacityForRequest)
		}

		subscriptionDelta := desiredAppliedSubscription - currentAppliedSubscription
		walletDelta := currentWallet - desiredWallet
		if subscriptionDelta != 0 {
			newUsed := subscription.AmountUsed + subscriptionDelta
			if newUsed < 0 || (subscription.AmountTotal > 0 && newUsed > subscription.AmountTotal) {
				return fmt.Errorf("%w: invalid subscription target", ErrSubscriptionQuotaInsufficient)
			}
			subscription.AmountUsed = newUsed
			if err := tx.Save(&subscription).Error; err != nil {
				return err
			}
		}
		if walletDelta != 0 {
			revision := reservation.Revision + 1
			operationID := fmt.Sprintf("relay-funding:%s:%d", requestID, revision)
			contextName := "subscription_funding_reserve"
			if walletDelta > 0 {
				contextName = "subscription_funding_credit"
			}
			if _, err := AdjustWalletWithTx(tx, userID, walletDelta, operationID, "relay", WalletAdjustReference{
				Type:    "relay_request",
				ID:      requestID,
				Context: contextName,
			}); err != nil {
				return err
			}
			reservation.Revision = revision
		}

		reservation.RequestID = requestID
		reservation.UserID = userID
		reservation.UserSubscriptionID = subscription.Id
		reservation.SubscriptionQuota = desiredSubscription
		reservation.SubscriptionAppliedQuota = desiredAppliedSubscription
		reservation.WalletQuota = desiredWallet
		reservation.Status = targetStatus
		if hasReservation {
			if err := tx.Save(&reservation).Error; err != nil {
				return err
			}
		} else if err := tx.Create(&reservation).Error; err != nil {
			return err
		}

		result.UserSubscriptionID = subscription.Id
		result.SubscriptionQuota = desiredSubscription
		result.WalletQuota = desiredWallet
		result.AmountTotal = subscription.AmountTotal
		result.AmountUsedBefore = usedBefore
		result.AmountUsedAfter = subscription.AmountUsed
		result.PlanID = subscription.PlanId
		if plan != nil {
			result.PlanTitle = plan.Title
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if outerTx == nil && mutatedUserID > 0 {
		_ = InvalidateUserCache(mutatedUserID)
	}
	return result, nil
}
