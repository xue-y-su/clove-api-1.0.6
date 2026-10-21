package model

import (
	"errors"
	"strconv"

	"gorm.io/gorm"
)

const (
	InviteApplicationStatusPending   = "pending"
	InviteApplicationStatusWithdrawn = "withdrawn"
	InviteApplicationStatusApproved  = "approved"
	InviteApplicationStatusRejected  = "rejected"

	InviteCodeKindSingle = "single"
	InviteCodeKindMulti  = "multi"

	InviteCodeStatusActive    = "active"
	InviteCodeStatusExhausted = "exhausted"
	InviteCodeStatusExpired   = "expired"
	InviteCodeStatusRevoked   = "revoked"

	InviteRewardStatusPending       = "pending"
	InviteRewardStatusPendingReview = "pending_review"
	InviteRewardStatusApproved      = "approved"
	InviteRewardStatusRejected      = "rejected"
	InviteRewardStatusWithdrawn     = "withdrawn"
	InviteRewardStatusCompleted     = "completed"
	InviteRewardStatusSkipped       = "skipped"
	InviteRewardStatusFailed        = "failed"
	InviteRewardStatusAvailable     = "available"
	InviteRewardStatusPendingPayout = "pending_payout"
	InviteRewardStatusPaid          = "paid"
	InviteRewardStatusFrozen        = "frozen"

	InviteRewardPayoutRequestStatusPending   = "pending"
	InviteRewardPayoutRequestStatusPaid      = "paid"
	InviteRewardPayoutRequestStatusRejected  = "rejected"
	InviteRewardPayoutRequestStatusFrozen    = "frozen"
	InviteRewardPayoutRequestStatusCancelled = "cancelled"

	InviteRewardPayoutActionFreeze   = "freeze"
	InviteRewardPayoutActionUnfreeze = "unfreeze"

	InviteRewardRecipientInviter = "inviter"
	InviteRewardRecipientInvitee = "invitee"

	InviteRewardTriggerRegistrationInviter    = "registration_inviter"
	InviteRewardTriggerCashTopUp              = "cash_topup"
	InviteRewardTriggerCashSubscription       = "cash_subscription"
	InviteRewardTriggerAdminBalanceRedemption = "admin_balance_redemption"
	InviteRewardTriggerAdminSubscriptionClaim = "admin_subscription_redemption"

	InviteRewardReversalNone    = "none"
	InviteRewardReversalPartial = "partial"
	InviteRewardReversalFull    = "full"
)

type InviteCodeApplication struct {
	ID              int64  `json:"id" gorm:"primaryKey;autoIncrement"`
	UserID          int    `json:"user_id" gorm:"not null;index"`
	Reason          string `json:"reason" gorm:"type:varchar(500);not null"`
	PendingUserSlot *int   `json:"-" gorm:"uniqueIndex;default:null"`
	Status          string `json:"status" gorm:"type:varchar(32);not null;index"`
	ReviewerID      int    `json:"reviewer_id" gorm:"not null;default:0;index"`
	ReviewReason    string `json:"review_reason" gorm:"type:varchar(500);not null;default:''"`
	ReviewedAt      int64  `json:"reviewed_at" gorm:"bigint;not null;default:0"`
	CreatedAt       int64  `json:"created_at" gorm:"bigint;autoCreateTime;index"`
	UpdatedAt       int64  `json:"updated_at" gorm:"bigint;autoUpdateTime"`
}

type InviteCode struct {
	ID              int64  `json:"id" gorm:"primaryKey;autoIncrement"`
	OwnerUserID     int    `json:"owner_user_id" gorm:"not null;index"`
	Code            string `json:"code" gorm:"type:varchar(64);not null;uniqueIndex"`
	ActiveOwnerSlot *int   `json:"-" gorm:"uniqueIndex;default:null"`
	Kind            string `json:"kind" gorm:"type:varchar(16);not null;index"`
	MaxUses         int    `json:"max_uses" gorm:"not null"`
	UsedCount       int    `json:"used_count" gorm:"not null;default:0"`
	Status          string `json:"status" gorm:"type:varchar(32);not null;index"`
	ExpiresAt       int64  `json:"expires_at" gorm:"bigint;not null;index"`
	IssuedBy        int    `json:"issued_by" gorm:"not null;index"`
	ApplicationID   *int64 `json:"application_id,omitempty" gorm:"index;default:null"`
	CreatedAt       int64  `json:"created_at" gorm:"bigint;autoCreateTime;index"`
	UpdatedAt       int64  `json:"updated_at" gorm:"bigint;autoUpdateTime"`
	RevokedAt       *int64 `json:"revoked_at,omitempty" gorm:"bigint;default:null"`
}

type InviteCodeUse struct {
	ID                  int64  `json:"id" gorm:"primaryKey;autoIncrement"`
	InviteCodeID        int64  `json:"invite_code_id" gorm:"not null;uniqueIndex:uniq_invite_registered,priority:1;index"`
	InviterUserID       int    `json:"inviter_user_id" gorm:"not null;index"`
	RegisteredUserID    int    `json:"registered_user_id" gorm:"not null;uniqueIndex:uniq_invite_registered,priority:2;index"`
	RegisteredAt        int64  `json:"registered_at" gorm:"bigint;not null;index"`
	Source              string `json:"source" gorm:"type:varchar(64);not null;default:'';index"`
	RegistrationIP      string `json:"registration_ip" gorm:"type:varchar(64);not null;default:''"`
	InviterReward       int    `json:"inviter_reward" gorm:"not null;default:0"`
	InviteeReward       int    `json:"invitee_reward" gorm:"not null;default:0"`
	InviterRewardStatus string `json:"inviter_reward_status" gorm:"type:varchar(32);not null;index"`
	InviteeRewardStatus string `json:"invitee_reward_status" gorm:"type:varchar(32);not null;index"`
	RewardAttemptCount  int    `json:"reward_attempt_count" gorm:"not null;default:0"`
	RewardLastError     string `json:"reward_last_error" gorm:"type:text;not null"`
	RewardNextAttemptAt int64  `json:"reward_next_attempt_at" gorm:"bigint;not null;default:0;index"`
	RewardsCompletedAt  int64  `json:"rewards_completed_at" gorm:"bigint;not null;default:0"`
	AuditEventID        string `json:"audit_event_id" gorm:"type:varchar(191);not null;uniqueIndex"`
	CreatedAt           int64  `json:"created_at" gorm:"bigint;autoCreateTime"`
	UpdatedAt           int64  `json:"updated_at" gorm:"bigint;autoUpdateTime"`
}

type InviteRewardPayout struct {
	ID              int64  `json:"id" gorm:"primaryKey;autoIncrement"`
	InviteCodeUseID int64  `json:"invite_code_use_id" gorm:"not null;index"`
	TriggerType     string `json:"trigger_type" gorm:"type:varchar(32);not null;default:'';uniqueIndex:uniq_invite_reward_source,priority:1"`
	TriggerID       string `json:"trigger_id" gorm:"type:varchar(191);not null;default:'';uniqueIndex:uniq_invite_reward_source,priority:2"`
	RecipientKind   string `json:"recipient_kind" gorm:"type:varchar(16);not null;uniqueIndex:uniq_invite_reward_source,priority:3;index"`
	RecipientUserID int    `json:"recipient_user_id" gorm:"not null;index"`
	EligibleQuota   int64  `json:"eligible_quota" gorm:"type:bigint;not null;default:0"`
	RateBasisPoints int64  `json:"rate_basis_points" gorm:"type:bigint;not null;default:0"`
	Amount          int64  `json:"amount" gorm:"type:bigint;not null"`
	ReversedAmount  int64  `json:"reversed_amount" gorm:"type:bigint;not null;default:0"`
	ReversalState   string `json:"reversal_state" gorm:"type:varchar(16);not null;default:'none';index"`
	Status          string `json:"status" gorm:"type:varchar(32);not null;index"`
	ReviewerID      int    `json:"reviewer_id" gorm:"not null;default:0;index"`
	ReviewReason    string `json:"review_reason" gorm:"type:varchar(500);not null;default:''"`
	ReviewedAt      int64  `json:"reviewed_at" gorm:"bigint;not null;default:0"`
	WithdrawnAt     int64  `json:"withdrawn_at" gorm:"bigint;not null;default:0;index"`
	CreatedAt       int64  `json:"created_at" gorm:"bigint;autoCreateTime;index"`
	UpdatedAt       int64  `json:"updated_at" gorm:"bigint;autoUpdateTime"`
}

type InviteRewardReversal struct {
	ID                       int64  `json:"id" gorm:"primaryKey;autoIncrement"`
	RewardID                 int64  `json:"reward_id" gorm:"not null;uniqueIndex:uniq_invite_reward_refund,priority:1;index"`
	RefundSourceType         string `json:"refund_source_type" gorm:"type:varchar(32);not null;uniqueIndex:uniq_invite_reward_refund,priority:2"`
	RefundSourceID           string `json:"refund_source_id" gorm:"type:varchar(191);not null;uniqueIndex:uniq_invite_reward_refund,priority:3"`
	RefundedBase             int64  `json:"refunded_base" gorm:"type:bigint;not null"`
	Amount                   int64  `json:"amount" gorm:"type:bigint;not null"`
	CumulativeReversedAmount int64  `json:"cumulative_reversed_amount" gorm:"type:bigint;not null"`
	RewardStatusBefore       string `json:"reward_status_before" gorm:"type:varchar(32);not null"`
	WalletOperationID        string `json:"wallet_operation_id" gorm:"type:varchar(191);not null;default:'';index"`
	CreatedAt                int64  `json:"created_at" gorm:"bigint;autoCreateTime;index"`
}

type InviteRewardPayoutRequest struct {
	ID              int64  `json:"id" gorm:"primaryKey;autoIncrement"`
	OperationID     string `json:"operation_id" gorm:"type:varchar(191);not null;uniqueIndex"`
	UserID          int    `json:"user_id" gorm:"not null;index"`
	PendingUserSlot *int   `json:"-" gorm:"uniqueIndex;default:null"`
	Amount          int64  `json:"amount" gorm:"type:bigint;not null"`
	Status          string `json:"status" gorm:"type:varchar(32);not null;index"`
	Reason          string `json:"reason" gorm:"type:varchar(500);not null;default:''"`
	ReviewerID      int    `json:"reviewer_id" gorm:"not null;default:0;index"`
	FreezeRevision  int    `json:"freeze_revision" gorm:"not null;default:0"`
	CreatedAt       int64  `json:"created_at" gorm:"bigint;autoCreateTime;index"`
	ReviewedAt      int64  `json:"reviewed_at" gorm:"bigint;not null;default:0"`
	UpdatedAt       int64  `json:"updated_at" gorm:"bigint;autoUpdateTime"`
}

type InviteRewardPayoutRequestItem struct {
	RequestID int64 `json:"request_id" gorm:"primaryKey;autoIncrement:false"`
	RewardID  int64 `json:"reward_id" gorm:"primaryKey;autoIncrement:false;index"`
}

type InviteRewardPayoutAction struct {
	ID                int64  `json:"id" gorm:"primaryKey;autoIncrement"`
	RequestID         int64  `json:"request_id" gorm:"not null;uniqueIndex:uniq_invite_reward_action_revision,priority:1;index"`
	Action            string `json:"action" gorm:"type:varchar(16);not null;uniqueIndex:uniq_invite_reward_action_revision,priority:2;index"`
	Revision          int    `json:"revision" gorm:"not null;uniqueIndex:uniq_invite_reward_action_revision,priority:3"`
	ActorID           int    `json:"actor_id" gorm:"not null;index"`
	Reason            string `json:"reason" gorm:"type:varchar(500);not null"`
	Amount            int64  `json:"amount" gorm:"type:bigint;not null"`
	WalletOperationID string `json:"wallet_operation_id" gorm:"type:varchar(191);not null;uniqueIndex"`
	CreatedAt         int64  `json:"created_at" gorm:"bigint;autoCreateTime;index"`
}

func LockInvitationUser(tx *gorm.DB, userID int) (User, error) {
	var user User
	err := lockForUpdate(tx).First(&user, userID).Error
	return user, err
}

func LockInviteCodeApplication(tx *gorm.DB, applicationID int64) (InviteCodeApplication, error) {
	var application InviteCodeApplication
	err := lockForUpdate(tx).First(&application, applicationID).Error
	return application, err
}

func LockInviteCodeByCode(tx *gorm.DB, code string) (InviteCode, error) {
	var invite InviteCode
	err := lockForUpdate(tx).Where("code = ?", code).First(&invite).Error
	return invite, err
}

func LockInviteCodeByID(tx *gorm.DB, inviteCodeID int64) (InviteCode, error) {
	var invite InviteCode
	err := lockForUpdate(tx).First(&invite, inviteCodeID).Error
	return invite, err
}

func LockInviteCodeUse(tx *gorm.DB, inviteUseID int64) (InviteCodeUse, error) {
	var use InviteCodeUse
	err := lockForUpdate(tx).First(&use, inviteUseID).Error
	return use, err
}

func LockInviteRewardPayoutRequest(tx *gorm.DB, requestID int64) (InviteRewardPayoutRequest, error) {
	var request InviteRewardPayoutRequest
	err := lockForUpdate(tx).First(&request, requestID).Error
	return request, err
}

func LockPendingInviteRewardPayoutRequest(tx *gorm.DB, userID int) (InviteRewardPayoutRequest, error) {
	var request InviteRewardPayoutRequest
	err := lockForUpdate(tx).
		Where("user_id = ? AND status = ?", userID, InviteRewardPayoutRequestStatusPending).
		First(&request).Error
	return request, err
}

func LockAvailableInviteRewardPayouts(tx *gorm.DB, recipientUserID int) ([]InviteRewardPayout, error) {
	rewards := make([]InviteRewardPayout, 0)
	err := lockForUpdate(tx).
		Where("recipient_user_id = ? AND recipient_kind = ? AND status = ?", recipientUserID, InviteRewardRecipientInviter, InviteRewardStatusAvailable).
		Order("id ASC").Find(&rewards).Error
	return rewards, err
}

func LockInviteRewardPayoutRequestItems(tx *gorm.DB, requestID int64) ([]InviteRewardPayoutRequestItem, error) {
	items := make([]InviteRewardPayoutRequestItem, 0)
	err := lockForUpdate(tx).Where("request_id = ?", requestID).Order("reward_id ASC").Find(&items).Error
	return items, err
}

func LockInviteRewardPayoutsByID(tx *gorm.DB, rewardIDs []int64) ([]InviteRewardPayout, error) {
	rewards := make([]InviteRewardPayout, 0, len(rewardIDs))
	err := lockForUpdate(tx).Where("id IN ?", rewardIDs).Order("id ASC").Find(&rewards).Error
	return rewards, err
}

func MigrateInvitationRewardStatuses(db *gorm.DB) error {
	if db == nil {
		return errors.New("invitation reward database is unavailable")
	}
	return db.Transaction(func(tx *gorm.DB) error {
		availableStatuses := []string{
			InviteRewardStatusPending,
			InviteRewardStatusPendingReview,
			InviteRewardStatusApproved,
			InviteRewardStatusFailed,
		}
		paidStatuses := []string{InviteRewardStatusWithdrawn, InviteRewardStatusCompleted}
		if err := tx.Model(&InviteRewardPayout{}).Where("status IN ?", availableStatuses).
			Update("status", InviteRewardStatusAvailable).Error; err != nil {
			return err
		}
		if err := tx.Model(&InviteRewardPayout{}).Where("status IN ?", paidStatuses).
			Update("status", InviteRewardStatusPaid).Error; err != nil {
			return err
		}
		for _, column := range []string{"inviter_reward_status", "invitee_reward_status"} {
			if err := tx.Model(&InviteCodeUse{}).Where(column+" IN ?", availableStatuses).
				Update(column, InviteRewardStatusAvailable).Error; err != nil {
				return err
			}
			if err := tx.Model(&InviteCodeUse{}).Where(column+" IN ?", paidStatuses).
				Update(column, InviteRewardStatusPaid).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func MigrateInvitationRewardFacts(db *gorm.DB) error {
	if db == nil {
		return errors.New("invitation reward database is unavailable")
	}
	if !db.Migrator().HasTable(&InviteRewardPayout{}) {
		return db.AutoMigrate(&InviteRewardPayout{}, &InviteRewardReversal{})
	}

	for _, field := range []string{
		"TriggerType",
		"TriggerID",
		"EligibleQuota",
		"RateBasisPoints",
		"ReversedAmount",
		"ReversalState",
	} {
		if !db.Migrator().HasColumn(&InviteRewardPayout{}, field) {
			if err := db.Migrator().AddColumn(&InviteRewardPayout{}, field); err != nil {
				return err
			}
		}
	}

	var legacyRewards []InviteRewardPayout
	if err := db.Where("trigger_type = ? OR trigger_type IS NULL OR trigger_id = ? OR trigger_id IS NULL", "", "").
		Order("id ASC").Find(&legacyRewards).Error; err != nil {
		return err
	}
	for _, reward := range legacyRewards {
		if err := db.Model(&InviteRewardPayout{}).Where("id = ?", reward.ID).Updates(map[string]any{
			"trigger_type":      InviteRewardTriggerRegistrationInviter,
			"trigger_id":        strconv.FormatInt(reward.InviteCodeUseID, 10),
			"eligible_quota":    reward.Amount,
			"rate_basis_points": 0,
			"reversed_amount":   0,
			"reversal_state":    InviteRewardReversalNone,
		}).Error; err != nil {
			return err
		}
	}
	if err := db.Model(&InviteRewardPayout{}).
		Where("reversal_state = ? OR reversal_state IS NULL", "").
		Update("reversal_state", InviteRewardReversalNone).Error; err != nil {
		return err
	}

	if db.Migrator().HasIndex(&InviteRewardPayout{}, "uniq_invite_reward_recipient") {
		if err := db.Migrator().DropIndex(&InviteRewardPayout{}, "uniq_invite_reward_recipient"); err != nil {
			return err
		}
	}
	if err := db.AutoMigrate(&InviteRewardPayout{}, &InviteRewardReversal{}); err != nil {
		return err
	}
	if !db.Migrator().HasIndex(&InviteRewardPayout{}, "uniq_invite_reward_source") {
		if err := db.Migrator().CreateIndex(&InviteRewardPayout{}, "uniq_invite_reward_source"); err != nil {
			return err
		}
	}
	return nil
}
