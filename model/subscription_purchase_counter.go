package model

import (
	"errors"
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrSubscriptionPurchaseLimit          = errors.New("已达到该套餐购买上限")
	ErrSubscriptionPurchaseCounterCorrupt = errors.New("subscription purchase counter is corrupt")
	ErrSubscriptionPurchaseConflict       = errors.New("subscription purchase reservation conflict")
)

// SubscriptionPurchaseCounter is the durable lifetime acquisition count for a
// user and plan. Rows are monotonic and intentionally outlive subscriptions.
type SubscriptionPurchaseCounter struct {
	UserID    int   `json:"user_id" gorm:"primaryKey;autoIncrement:false"`
	PlanID    int   `json:"plan_id" gorm:"primaryKey;autoIncrement:false"`
	UsedCount int64 `json:"used_count" gorm:"not null;default:0"`
	CreatedAt int64 `json:"created_at" gorm:"bigint;autoCreateTime"`
	UpdatedAt int64 `json:"updated_at" gorm:"bigint;autoUpdateTime"`
}

func (SubscriptionPurchaseCounter) TableName() string {
	return "subscription_purchase_counters"
}

func StricterPositiveSubscriptionLimit(snapshot, current int) int {
	if snapshot <= 0 {
		if current > 0 {
			return current
		}
		return 0
	}
	if current <= 0 || snapshot < current {
		return snapshot
	}
	return current
}

func reserveSubscriptionPurchaseSlotTx(tx *gorm.DB, userID, planID, limit int) error {
	if tx == nil || userID <= 0 || planID <= 0 {
		return ErrSubscriptionPurchaseConflict
	}
	now := common.GetTimestamp()
	seed := SubscriptionPurchaseCounter{
		UserID:    userID,
		PlanID:    planID,
		UsedCount: 0,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&seed).Error; err != nil {
		return err
	}

	query := tx.Model(&SubscriptionPurchaseCounter{}).
		Where("user_id = ? AND plan_id = ? AND used_count >= 0", userID, planID)
	if limit > 0 {
		query = query.Where("used_count < ?", limit)
	}
	result := query.Updates(map[string]any{
		"used_count": gorm.Expr("used_count + ?", 1),
		"updated_at": now,
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 1 {
		return nil
	}

	var counter SubscriptionPurchaseCounter
	if err := tx.Where("user_id = ? AND plan_id = ?", userID, planID).First(&counter).Error; err != nil {
		return fmt.Errorf("%w: %v", ErrSubscriptionPurchaseConflict, err)
	}
	if counter.UsedCount < 0 {
		return ErrSubscriptionPurchaseCounterCorrupt
	}
	if limit > 0 && counter.UsedCount >= int64(limit) {
		return ErrSubscriptionPurchaseLimit
	}
	return ErrSubscriptionPurchaseConflict
}

func CountSubscriptionPurchasesByPlan(userID, planID int) (int64, error) {
	if DB == nil || userID <= 0 || planID <= 0 {
		return 0, errors.New("invalid userId or planId")
	}
	return countSubscriptionPurchasesByPlanTx(DB, userID, planID)
}

func countSubscriptionPurchasesByPlanTx(tx *gorm.DB, userID, planID int) (int64, error) {
	var counter SubscriptionPurchaseCounter
	err := tx.Where("user_id = ? AND plan_id = ?", userID, planID).First(&counter).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	if counter.UsedCount < 0 {
		return 0, ErrSubscriptionPurchaseCounterCorrupt
	}
	return counter.UsedCount, nil
}

// BackfillSubscriptionPurchaseCounters raises counters to the strongest
// surviving evidence. It never lowers a row and is safe to rerun.
func BackfillSubscriptionPurchaseCounters(db *gorm.DB) (int64, error) {
	if db == nil {
		return 0, errors.New("subscription purchase counter database is not initialized")
	}
	type evidenceRow struct {
		UserID int
		PlanID int
		Count  int64
	}
	type counterKey struct {
		userID int
		planID int
	}
	currentEvidence := make(map[counterKey]int64)
	recordedEvidence := make(map[counterKey]int64)

	var rows []evidenceRow
	if err := db.Model(&UserSubscription{}).
		Select("user_id, plan_id, COUNT(*) AS count").
		Group("user_id, plan_id").
		Scan(&rows).Error; err != nil {
		return 0, err
	}
	for _, row := range rows {
		if row.UserID > 0 && row.PlanID > 0 && row.Count > 0 {
			currentEvidence[counterKey{userID: row.UserID, planID: row.PlanID}] = row.Count
		}
	}

	rows = nil
	if err := db.Model(&SubscriptionOrder{}).
		Select("user_id, plan_id, COUNT(*) AS count").
		Where("status = ?", common.TopUpStatusSuccess).
		Group("user_id, plan_id").
		Scan(&rows).Error; err != nil {
		return 0, err
	}
	for _, row := range rows {
		if row.UserID > 0 && row.PlanID > 0 && row.Count > 0 {
			key := counterKey{userID: row.UserID, planID: row.PlanID}
			recordedEvidence[key] += row.Count
		}
	}

	rows = nil
	if err := db.Table("user_redemption_claims AS claims").
		Select("claims.claimer_user_id AS user_id, entitlements.plan_id AS plan_id, COUNT(*) AS count").
		Joins("JOIN user_redemption_subscription_entitlements AS entitlements ON entitlements.code_id = claims.code_id").
		Where("claims.reward_type = ?", UserRedemptionRewardSubscription).
		Group("claims.claimer_user_id, entitlements.plan_id").
		Scan(&rows).Error; err != nil {
		return 0, err
	}
	for _, row := range rows {
		if row.UserID > 0 && row.PlanID > 0 && row.Count > 0 {
			key := counterKey{userID: row.UserID, planID: row.PlanID}
			recordedEvidence[key] += row.Count
		}
	}

	baselines := make(map[counterKey]int64, len(currentEvidence)+len(recordedEvidence))
	for key, count := range currentEvidence {
		baselines[key] = count
	}
	for key, count := range recordedEvidence {
		if count > baselines[key] {
			baselines[key] = count
		}
	}

	var updated int64
	err := db.Transaction(func(tx *gorm.DB) error {
		now := common.GetTimestamp()
		for key, baseline := range baselines {
			seed := SubscriptionPurchaseCounter{
				UserID: key.userID, PlanID: key.planID, UsedCount: 0,
				CreatedAt: now, UpdatedAt: now,
			}
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&seed).Error; err != nil {
				return err
			}
			result := tx.Model(&SubscriptionPurchaseCounter{}).
				Where("user_id = ? AND plan_id = ? AND used_count < ?", key.userID, key.planID, baseline).
				Updates(map[string]any{"used_count": baseline, "updated_at": now})
			if result.Error != nil {
				return result.Error
			}
			updated += result.RowsAffected
		}
		return nil
	})
	return updated, err
}
