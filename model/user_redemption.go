package model

import (
	"errors"

	"gorm.io/gorm"
)

const (
	UserRedemptionKindSingle = "single"
	UserRedemptionKindRandom = "random"
	UserRedemptionKindFixed  = "fixed"

	UserRedemptionRewardBalance      = "balance"
	UserRedemptionRewardSubscription = "subscription"

	UserRedemptionStatusActive         = "active"
	UserRedemptionStatusDisabled       = "disabled"
	UserRedemptionStatusExhausted      = "exhausted"
	UserRedemptionStatusExpiredPending = "expired_pending"
	UserRedemptionStatusRefundPending  = "refund_pending"
	UserRedemptionStatusRefunded       = "refunded"
	UserRedemptionStatusForfeited      = "forfeited"
	UserRedemptionStatusDeleted        = "deleted"

	// Legacy statuses are only retained for data migration from earlier releases.
	UserRedemptionStatusCompleted = "completed"
	UserRedemptionStatusCancelled = "cancelled"
	UserRedemptionStatusExpired   = "expired"
)

// UserRedemptionCode is a wallet-funded user-created redemption code.
// It is independent from the administrator Redemption model.
type UserRedemptionCode struct {
	ID              int64  `json:"id" gorm:"primaryKey;autoIncrement"`
	Code            string `json:"code" gorm:"type:varchar(64);not null;uniqueIndex"`
	CreatorUserID   int    `json:"creator_user_id" gorm:"not null;index"`
	IssuedBy        int    `json:"issued_by" gorm:"not null;default:0;index"`
	OfficialIssuer  bool   `json:"official_issuer" gorm:"not null;default:false;index"`
	RewardType      string `json:"reward_type" gorm:"type:varchar(16);not null;default:'balance';index"`
	Kind            string `json:"kind" gorm:"type:varchar(16);not null;index"`
	TotalQuota      int64  `json:"total_quota" gorm:"type:bigint;not null"`
	ClaimedQuota    int64  `json:"claimed_quota" gorm:"type:bigint;not null;default:0"`
	RefundedQuota   int64  `json:"refunded_quota" gorm:"type:bigint;not null;default:0"`
	ForfeitedQuota  int64  `json:"forfeited_quota" gorm:"type:bigint;not null;default:0"`
	MaxClaims       int    `json:"max_claims" gorm:"not null"`
	ClaimCount      int    `json:"claim_count" gorm:"not null;default:0"`
	Status          string `json:"status" gorm:"type:varchar(32);not null;index"`
	ExpiresAt       int64  `json:"expires_at" gorm:"bigint;not null;index"`
	DisabledAt      int64  `json:"disabled_at" gorm:"bigint;not null;default:0"`
	DisabledBy      int    `json:"disabled_by" gorm:"not null;default:0"`
	DisabledReason  string `json:"disabled_reason" gorm:"type:varchar(500);not null;default:''"`
	TerminalAt      int64  `json:"terminal_at" gorm:"bigint;not null;default:0"`
	TerminalActorID int    `json:"terminal_actor_id" gorm:"not null;default:0"`
	TerminalReason  string `json:"terminal_reason" gorm:"type:varchar(500);not null;default:''"`
	ReserveOpID     string `json:"reserve_operation_id" gorm:"column:reserve_operation_id;type:varchar(191);not null;uniqueIndex"`
	RefundOpID      string `json:"refund_operation_id" gorm:"column:refund_operation_id;type:varchar(191);not null;default:'';index"`
	ForfeitOpID     string `json:"forfeit_operation_id" gorm:"column:forfeit_operation_id;type:varchar(191);not null;default:'';index"`
	DeletedAt       int64  `json:"deleted_at" gorm:"bigint;not null;default:0;index"`
	DeletedBy       int    `json:"deleted_by" gorm:"not null;default:0"`
	DeleteReason    string `json:"delete_reason" gorm:"type:varchar(500);not null;default:''"`
	CreatedAt       int64  `json:"created_at" gorm:"bigint;autoCreateTime;index"`
	UpdatedAt       int64  `json:"updated_at" gorm:"bigint;autoUpdateTime"`
}

// UserRedemptionClaim records one claim against a user redemption code.
type UserRedemptionClaim struct {
	ID                 int64  `json:"id" gorm:"primaryKey;autoIncrement"`
	CodeID             int64  `json:"code_id" gorm:"not null;uniqueIndex:uniq_user_redemption_claim,priority:1;index"`
	ClaimerUserID      int    `json:"claimer_user_id" gorm:"not null;uniqueIndex:uniq_user_redemption_claim,priority:2;index"`
	RewardType         string `json:"reward_type" gorm:"type:varchar(16);not null;default:'balance';index"`
	Quota              int64  `json:"quota" gorm:"type:bigint;not null"`
	UserSubscriptionID int64  `json:"user_subscription_id" gorm:"not null;default:0;index"`
	CreditOpID         string `json:"credit_operation_id" gorm:"type:varchar(191);not null;uniqueIndex"`
	ClaimedAt          int64  `json:"claimed_at" gorm:"bigint;not null;index"`
	CreatedAt          int64  `json:"created_at" gorm:"bigint;autoCreateTime"`
}

// UserRedemptionSubscriptionEntitlement snapshots the plan terms purchased for a code.
type UserRedemptionSubscriptionEntitlement struct {
	ID                      int64   `json:"id" gorm:"primaryKey;autoIncrement"`
	CodeID                  int64   `json:"code_id" gorm:"not null;uniqueIndex"`
	PlanID                  int     `json:"plan_id" gorm:"not null;index"`
	PlanTitle               string  `json:"plan_title" gorm:"type:varchar(128);not null"`
	PlanSubtitle            string  `json:"plan_subtitle" gorm:"type:varchar(255);not null;default:''"`
	Currency                string  `json:"currency" gorm:"type:varchar(8);not null"`
	PriceAmount             float64 `json:"price_amount" gorm:"not null"`
	UnitPaidQuota           int64   `json:"unit_paid_quota" gorm:"type:bigint;not null"`
	DurationUnit            string  `json:"duration_unit" gorm:"type:varchar(16);not null"`
	DurationValue           int     `json:"duration_value" gorm:"not null"`
	CustomSeconds           int64   `json:"custom_seconds" gorm:"type:bigint;not null"`
	TotalAmount             int64   `json:"total_amount" gorm:"type:bigint;not null"`
	QuotaResetPeriod        string  `json:"quota_reset_period" gorm:"type:varchar(16);not null"`
	QuotaResetCustomSeconds int64   `json:"quota_reset_custom_seconds" gorm:"type:bigint;not null"`
	AllowWalletOverflow     bool    `json:"allow_wallet_overflow" gorm:"not null"`
	UpgradeGroup            string  `json:"upgrade_group" gorm:"type:varchar(64);not null"`
	DowngradeGroup          string  `json:"downgrade_group" gorm:"type:varchar(64);not null"`
	MaxPurchasePerUser      int     `json:"max_purchase_per_user" gorm:"not null"`
	CreatedAt               int64   `json:"created_at" gorm:"bigint;autoCreateTime"`
	UpdatedAt               int64   `json:"updated_at" gorm:"bigint;autoUpdateTime"`
}

func (UserRedemptionCode) TableName() string  { return "user_redemption_codes" }
func (UserRedemptionClaim) TableName() string { return "user_redemption_claims" }
func (UserRedemptionSubscriptionEntitlement) TableName() string {
	return "user_redemption_subscription_entitlements"
}

func LockUserRedemptionCodeByCode(tx *gorm.DB, code string) (UserRedemptionCode, error) {
	var row UserRedemptionCode
	err := lockForUpdate(tx).Where("code = ?", code).First(&row).Error
	return row, err
}

func LockUserRedemptionCodeByID(tx *gorm.DB, id int64) (UserRedemptionCode, error) {
	var row UserRedemptionCode
	err := lockForUpdate(tx).First(&row, id).Error
	return row, err
}

func LockUserRedemptionCreator(tx *gorm.DB, userID int) error {
	var user User
	return lockForUpdate(tx).Select("id").First(&user, userID).Error
}

func LockUserRedemptionCreatorUnscoped(tx *gorm.DB, userID int) error {
	var user User
	return lockForUpdate(tx.Unscoped()).Select("id").First(&user, userID).Error
}

func GetUserRedemptionCodeByID(id int64) (*UserRedemptionCode, error) {
	var row UserRedemptionCode
	if err := DB.First(&row, id).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

func GetUserRedemptionCodeByCode(code string) (*UserRedemptionCode, error) {
	var row UserRedemptionCode
	if err := DB.Where("code = ?", code).First(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

func ListUserRedemptionClaims(codeID int64) ([]UserRedemptionClaim, error) {
	var claims []UserRedemptionClaim
	err := DB.Where("code_id = ?", codeID).Order("id asc").Find(&claims).Error
	return claims, err
}

func MigrateUserRedemptionStatuses(db *gorm.DB) error {
	if db == nil {
		return errors.New("user redemption database is not initialized")
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&UserRedemptionCode{}).Where("issued_by = ?", 0).
			Update("issued_by", gorm.Expr("creator_user_id")).Error; err != nil {
			return err
		}
		if err := tx.Model(&UserRedemptionCode{}).Where("reward_type = ? OR reward_type IS NULL", "").
			Update("reward_type", UserRedemptionRewardBalance).Error; err != nil {
			return err
		}
		if err := tx.Model(&UserRedemptionClaim{}).Where("reward_type = ? OR reward_type IS NULL", "").
			Update("reward_type", UserRedemptionRewardBalance).Error; err != nil {
			return err
		}
		var rows []UserRedemptionCode
		if err := tx.Where(
			"status IN ? OR refunded_quota > ? OR forfeited_quota > ?",
			[]string{UserRedemptionStatusCompleted, UserRedemptionStatusCancelled, UserRedemptionStatusExpired},
			0,
			0,
		).Find(&rows).Error; err != nil {
			return err
		}
		for _, row := range rows {
			status := row.Status
			switch {
			case row.RefundedQuota > 0:
				status = UserRedemptionStatusRefunded
			case row.ForfeitedQuota > 0:
				status = UserRedemptionStatusForfeited
			case row.Status == UserRedemptionStatusCompleted:
				status = UserRedemptionStatusExhausted
			case row.Status == UserRedemptionStatusCancelled || row.Status == UserRedemptionStatusExpired:
				status = UserRedemptionStatusExpiredPending
			}
			if status == row.Status {
				continue
			}
			if err := tx.Model(&UserRedemptionCode{}).Where("id = ?", row.ID).Update("status", status).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
