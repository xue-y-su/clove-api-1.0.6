package service

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/user_redemption_setting"
	"gorm.io/gorm"
)

const (
	userRedemptionMinClaims       = 2
	userRedemptionMaxClaims       = 100
	userRedemptionMinTTL          = time.Hour
	userRedemptionTTLGrace        = 5 * time.Second
	userRedemptionMaxTTL          = 30 * 24 * time.Hour
	userRedemptionCodeLen         = 24
	userRedemptionCategoryReserve = "user_redemption_reserve"
	userRedemptionCategoryClaim   = "user_redemption_claim"
	userRedemptionCategoryRefund  = "user_redemption_refund"
)

func userRedemptionStandardUnit() (int64, error) {
	unit, err := common.WalletQuotaFromAmountString("1")
	if err != nil || unit <= 0 {
		return 0, ErrUserRedemptionInvalid
	}
	return unit, nil
}

var (
	ErrUserRedemptionInvalid        = errors.New("user redemption request is invalid")
	ErrUserRedemptionNotFound       = errors.New("user redemption code not found")
	ErrUserRedemptionForbidden      = errors.New("user redemption action is not allowed")
	ErrUserRedemptionInactive       = errors.New("user redemption code is not active")
	ErrUserRedemptionCodeDisabled   = errors.New("user redemption code was disabled by an administrator")
	ErrUserRedemptionExpired        = errors.New("user redemption code has expired")
	ErrUserRedemptionAlreadyClaimed = errors.New("user already claimed this redemption code")
	ErrUserRedemptionExhausted      = errors.New("user redemption code has no remaining claims")
	ErrUserRedemptionInsufficient   = errors.New("wallet balance is insufficient for redemption reserve")
	ErrUserRedemptionConflict       = errors.New("user redemption state conflicts with the requested action")
	ErrUserRedemptionDisabled       = errors.New("user redemption creation is disabled")
	ErrUserRedemptionModeDisabled   = errors.New("user redemption mode is disabled")
	ErrUserRedemptionActiveLimit    = errors.New("user redemption active code limit reached")
	ErrUserRedemptionClaimsLimit    = errors.New("user redemption claims limit exceeded")
	ErrUserRedemptionExpiryLimit    = errors.New("user redemption expiry limit exceeded")
)

type UserRedemptionInsufficientBalanceError struct {
	Requested int64
	Available int64
}

func (e *UserRedemptionInsufficientBalanceError) Error() string {
	return ErrUserRedemptionInsufficient.Error()
}

func (e *UserRedemptionInsufficientBalanceError) Unwrap() error {
	return ErrUserRedemptionInsufficient
}

var (
	invalidateUserRedemptionCache     = model.InvalidateUserCache
	sqliteUserRedemptionCreationMutex sync.Mutex
)

type CreateUserRedemptionInput struct {
	CreatorUserID int
	Kind          string
	TotalQuota    int64
	MaxClaims     int
	ExpiresAt     int64
}

type ClaimUserRedemptionInput struct {
	ClaimerUserID      int
	Code               string
	ExpectedRewardType string
}

type UserRedemptionRewardTypeMismatchError struct {
	Expected string
	Actual   string
}

func (e *UserRedemptionRewardTypeMismatchError) Error() string {
	return fmt.Sprintf("user redemption reward type mismatch: expected %s, got %s", e.Expected, e.Actual)
}

func (e *UserRedemptionRewardTypeMismatchError) Unwrap() error {
	return ErrUserRedemptionConflict
}

type UserRedemptionListQuery struct {
	OwnerOnly       bool
	CreatorUserID   int
	CreatorUsername string
	Status          string
	RewardType      string
	Kind            string
	Deleted         string
	CreatedFrom     int64
	CreatedTo       int64
	ExpiresFrom     int64
	ExpiresTo       int64
	Page            int
	PageSize        int
}

type UserRedemptionListResult struct {
	Items    []UserRedemptionCodeView `json:"items"`
	Total    int64                    `json:"total"`
	Page     int                      `json:"page"`
	PageSize int                      `json:"page_size"`
}

type UserRedemptionDetail struct {
	Code   UserRedemptionCodeView    `json:"code"`
	Claims []UserRedemptionClaimView `json:"claims"`
}

type UserRedemptionCodeView struct {
	ID              int64   `json:"id"`
	Code            string  `json:"code"`
	CreatorUserID   int     `json:"creator_user_id"`
	CreatorUsername string  `json:"creator_username,omitempty"`
	RewardType      string  `json:"reward_type"`
	Kind            string  `json:"kind"`
	PlanID          int     `json:"plan_id,omitempty"`
	PlanTitle       string  `json:"plan_title,omitempty"`
	PlanSubtitle    string  `json:"plan_subtitle,omitempty"`
	Currency        string  `json:"currency,omitempty"`
	PriceAmount     float64 `json:"price_amount,omitempty"`
	UnitPaidQuota   int64   `json:"unit_paid_quota,omitempty"`
	TotalQuota      int64   `json:"total_quota"`
	TotalAmount     string  `json:"total_amount"`
	ClaimedQuota    int64   `json:"claimed_quota"`
	ClaimedAmount   string  `json:"claimed_amount"`
	RefundedQuota   int64   `json:"refunded_quota"`
	RefundedAmount  string  `json:"refunded_amount"`
	ForfeitedQuota  int64   `json:"forfeited_quota"`
	ForfeitedAmount string  `json:"forfeited_amount"`
	RemainingQuota  int64   `json:"remaining_quota"`
	RemainingAmount string  `json:"remaining_amount"`
	MaxClaims       int     `json:"max_claims"`
	ClaimCount      int     `json:"claim_count"`
	Status          string  `json:"status"`
	ExpiresAt       int64   `json:"expires_at"`
	DisabledAt      int64   `json:"disabled_at"`
	DisabledBy      int     `json:"disabled_by,omitempty"`
	DisabledReason  string  `json:"disabled_reason,omitempty"`
	TerminalAt      int64   `json:"terminal_at"`
	TerminalActorID int     `json:"terminal_actor_id,omitempty"`
	TerminalReason  string  `json:"terminal_reason,omitempty"`
	Deleted         bool    `json:"deleted"`
	DeletedAt       int64   `json:"deleted_at"`
	DeletedBy       int     `json:"deleted_by,omitempty"`
	DeleteReason    string  `json:"delete_reason,omitempty"`
	CreatedAt       int64   `json:"created_at"`
	UpdatedAt       int64   `json:"updated_at"`
}

type UserRedemptionClaimView struct {
	ID                 int64  `json:"id"`
	RewardType         string `json:"reward_type"`
	Quota              int64  `json:"quota"`
	UserSubscriptionID int64  `json:"user_subscription_id,omitempty"`
	Amount             string `json:"amount"`
	ClaimedAt          int64  `json:"claimed_at"`
	ClaimantDisplay    string `json:"claimant_display,omitempty"`
	ClaimantUserID     int    `json:"claimant_user_id,omitempty"`
	ClaimantUsername   string `json:"claimant_username,omitempty"`
}

func UserRedemptionRemainingQuota(row model.UserRedemptionCode) int64 {
	if row.TotalQuota <= 0 || row.ClaimedQuota < 0 || row.RefundedQuota < 0 || row.ForfeitedQuota < 0 {
		return 0
	}
	remaining := row.TotalQuota
	for _, consumed := range []int64{row.ClaimedQuota, row.RefundedQuota, row.ForfeitedQuota} {
		if consumed >= remaining {
			return 0
		}
		remaining -= consumed
	}
	return remaining
}

func hasMinimumUserRedemptionQuota(total, minimum int64, claims int) bool {
	if total <= 0 || minimum <= 0 || claims <= 0 {
		return false
	}
	return int64(claims) <= total/minimum
}

func isUserRedemptionExpired(expiresAt, now int64) bool {
	return expiresAt > 0 && expiresAt <= now
}

func validateUserRedemptionExpiry(expiresAt int64, settings user_redemption_setting.Settings) (int64, error) {
	now := common.GetTimestamp()
	if expiresAt <= now {
		return 0, ErrUserRedemptionInvalid
	}
	ttlSeconds := expiresAt - now
	minTTLSeconds := int64((userRedemptionMinTTL - userRedemptionTTLGrace) / time.Second)
	hardMaxTTLSeconds := int64(userRedemptionMaxTTL / time.Second)
	configuredMaxTTLSeconds := settings.MaxExpiryHours * int64(time.Hour/time.Second)
	if ttlSeconds < minTTLSeconds || ttlSeconds > hardMaxTTLSeconds {
		return 0, ErrUserRedemptionInvalid
	}
	if ttlSeconds > configuredMaxTTLSeconds {
		return 0, ErrUserRedemptionExpiryLimit
	}
	return now, nil
}

func userRedemptionCodeView(row model.UserRedemptionCode, entitlement *model.UserRedemptionSubscriptionEntitlement, actor UserIdentity, creatorUsername string) UserRedemptionCodeView {
	remainingQuota := UserRedemptionRemainingQuota(row)
	view := UserRedemptionCodeView{
		ID:              row.ID,
		Code:            row.Code,
		CreatorUserID:   row.CreatorUserID,
		RewardType:      normalizeUserRedemptionRewardType(row.RewardType),
		Kind:            row.Kind,
		TotalQuota:      row.TotalQuota,
		TotalAmount:     common.WalletAmountFromQuota(row.TotalQuota),
		ClaimedQuota:    row.ClaimedQuota,
		ClaimedAmount:   common.WalletAmountFromQuota(row.ClaimedQuota),
		RefundedQuota:   row.RefundedQuota,
		RefundedAmount:  common.WalletAmountFromQuota(row.RefundedQuota),
		ForfeitedQuota:  row.ForfeitedQuota,
		ForfeitedAmount: common.WalletAmountFromQuota(row.ForfeitedQuota),
		RemainingQuota:  remainingQuota,
		RemainingAmount: common.WalletAmountFromQuota(remainingQuota),
		MaxClaims:       row.MaxClaims,
		ClaimCount:      row.ClaimCount,
		Status:          row.Status,
		ExpiresAt:       row.ExpiresAt,
		DisabledAt:      row.DisabledAt,
		DisabledReason:  row.DisabledReason,
		TerminalAt:      row.TerminalAt,
		TerminalReason:  row.TerminalReason,
		Deleted:         row.DeletedAt != 0,
		DeletedAt:       row.DeletedAt,
		DeleteReason:    row.DeleteReason,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
	}
	if entitlement != nil {
		view.PlanID = entitlement.PlanID
		view.PlanTitle = entitlement.PlanTitle
		view.PlanSubtitle = entitlement.PlanSubtitle
		view.Currency = entitlement.Currency
		view.PriceAmount = entitlement.PriceAmount
		view.UnitPaidQuota = entitlement.UnitPaidQuota
	}
	if actor.Role >= common.RoleAdminUser {
		view.CreatorUsername = creatorUsername
		view.DisabledBy = row.DisabledBy
		view.TerminalActorID = row.TerminalActorID
		view.DeletedBy = row.DeletedBy
	}
	return view
}

func maskUserRedemptionUsername(username string) string {
	runes := []rune(strings.TrimSpace(username))
	switch len(runes) {
	case 0, 1:
		return "*"
	case 2:
		return string(runes[0]) + "*"
	default:
		return string(runes[0]) + "***" + string(runes[len(runes)-1])
	}
}

func loadUserRedemptionUsernames(ctx context.Context, userIDs []int) (map[int]string, error) {
	if len(userIDs) == 0 {
		return map[int]string{}, nil
	}
	type userRow struct {
		ID       int
		Username string
	}
	rows := make([]userRow, 0, len(userIDs))
	if err := model.DB.WithContext(ctx).Model(&model.User{}).
		Select("id", "username").
		Where("id IN ?", userIDs).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	usernames := make(map[int]string, len(rows))
	for _, row := range rows {
		usernames[row.ID] = row.Username
	}
	return usernames, nil
}

func CreateUserRedemptionCode(ctx context.Context, input CreateUserRedemptionInput) (*model.UserRedemptionCode, error) {
	if model.DB == nil || input.CreatorUserID <= 0 {
		return nil, ErrUserRedemptionInvalid
	}
	settings := user_redemption_setting.GetSettings()
	if !settings.Enabled {
		return nil, ErrUserRedemptionDisabled
	}
	kind := strings.TrimSpace(strings.ToLower(input.Kind))
	if kind != model.UserRedemptionKindSingle && kind != model.UserRedemptionKindRandom {
		return nil, ErrUserRedemptionInvalid
	}
	if (kind == model.UserRedemptionKindSingle && !settings.SingleEnabled) ||
		(kind == model.UserRedemptionKindRandom && !settings.RandomEnabled) {
		return nil, ErrUserRedemptionModeDisabled
	}
	minUnit, err := userRedemptionStandardUnit()
	if err != nil {
		return nil, err
	}
	if input.TotalQuota < minUnit {
		return nil, ErrUserRedemptionInvalid
	}
	maxClaims := input.MaxClaims
	if kind == model.UserRedemptionKindSingle {
		maxClaims = 1
	} else if maxClaims < userRedemptionMinClaims || maxClaims > userRedemptionMaxClaims {
		return nil, ErrUserRedemptionInvalid
	}
	if kind == model.UserRedemptionKindRandom && maxClaims > settings.MaxClaimsPerCode {
		return nil, ErrUserRedemptionClaimsLimit
	}
	if kind == model.UserRedemptionKindRandom && !hasMinimumUserRedemptionQuota(input.TotalQuota, minUnit, maxClaims) {
		return nil, ErrUserRedemptionInvalid
	}
	now, err := validateUserRedemptionExpiry(input.ExpiresAt, settings)
	if err != nil {
		return nil, err
	}

	var created *model.UserRedemptionCode
	if common.UsingMainDatabase(common.DatabaseTypeSQLite) {
		// SQLite deferred transactions cannot upgrade two concurrent read-then-write
		// creations without one failing SQLITE_BUSY/LOCKED.
		sqliteUserRedemptionCreationMutex.Lock()
		defer sqliteUserRedemptionCreationMutex.Unlock()
	}
	err = model.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if settings.MaxActiveCodesPerUser > 0 {
			if err := model.LockUserRedemptionCreator(tx, input.CreatorUserID); err != nil {
				return err
			}
			var openCodeCount int64
			if err := tx.Model(&model.UserRedemptionCode{}).
				Where("creator_user_id = ? AND deleted_at = ? AND status IN ? AND expires_at > ?", input.CreatorUserID, 0, []string{model.UserRedemptionStatusActive, model.UserRedemptionStatusDisabled}, now).
				Count(&openCodeCount).Error; err != nil {
				return err
			}
			if openCodeCount >= int64(settings.MaxActiveCodesPerUser) {
				return ErrUserRedemptionActiveLimit
			}
		}
		codeValue, err := generateUserRedemptionCode(tx)
		if err != nil {
			return err
		}
		row := model.UserRedemptionCode{
			Code:          codeValue,
			CreatorUserID: input.CreatorUserID,
			IssuedBy:      input.CreatorUserID,
			Kind:          kind,
			TotalQuota:    input.TotalQuota,
			ClaimedQuota:  0,
			RefundedQuota: 0,
			MaxClaims:     maxClaims,
			ClaimCount:    0,
			Status:        model.UserRedemptionStatusActive,
			ExpiresAt:     input.ExpiresAt,
			// Temporary placeholder; rewritten after ID assignment.
			ReserveOpID: fmt.Sprintf("user-redemption:pending:%d:%s", input.CreatorUserID, codeValue),
		}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		reserveOp := fmt.Sprintf("user-redemption:%d:reserve", row.ID)
		if _, err := AdjustWithTx(tx, input.CreatorUserID, -input.TotalQuota, reserveOp, userRedemptionCategoryReserve, WalletReference{
			Type:    "user_redemption_code",
			ID:      fmt.Sprintf("%d", row.ID),
			Context: "reserve",
		}); err != nil {
			if errors.Is(err, ErrWalletInsufficientBalance) {
				available := int64(0)
				var balanceError *model.WalletInsufficientBalanceError
				if errors.As(err, &balanceError) {
					available = balanceError.Balance
				}
				return &UserRedemptionInsufficientBalanceError{
					Requested: input.TotalQuota,
					Available: available,
				}
			}
			return err
		}
		if err := tx.Model(&row).Update("reserve_operation_id", reserveOp).Error; err != nil {
			return err
		}
		row.ReserveOpID = reserveOp
		if err := enqueueUserRedemptionBusinessAuditWithTx(tx, fmt.Sprintf("user-redemption:%d:create", row.ID), "user.redemption.create", input.CreatorUserID, input.CreatorUserID, row, input.TotalQuota, 0); err != nil {
			return err
		}
		created = &row
		return nil
	})
	if err != nil {
		return nil, err
	}
	_ = invalidateUserRedemptionCache(input.CreatorUserID)
	return created, nil
}

func normalizeUserRedemptionRewardType(rewardType string) string {
	rewardType = strings.TrimSpace(strings.ToLower(rewardType))
	if rewardType == "" {
		return model.UserRedemptionRewardBalance
	}
	return rewardType
}

func ClaimUserRedemptionCode(ctx context.Context, claimerUserID int, code string) (*model.UserRedemptionClaim, error) {
	return ClaimUserRedemptionCodeWithInput(ctx, ClaimUserRedemptionInput{
		ClaimerUserID: claimerUserID,
		Code:          code,
	})
}

func ClaimUserRedemptionCodeWithInput(ctx context.Context, input ClaimUserRedemptionInput) (*model.UserRedemptionClaim, error) {
	input.Code = strings.TrimSpace(input.Code)
	input.ExpectedRewardType = strings.TrimSpace(strings.ToLower(input.ExpectedRewardType))
	if model.DB == nil || input.ClaimerUserID <= 0 || input.Code == "" {
		return nil, ErrUserRedemptionInvalid
	}
	var route model.UserRedemptionCode
	if err := model.DB.WithContext(ctx).Select("id", "reward_type").Where("code = ?", input.Code).First(&route).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserRedemptionNotFound
		}
		return nil, err
	}
	actualRewardType := normalizeUserRedemptionRewardType(route.RewardType)
	if input.ExpectedRewardType != "" && input.ExpectedRewardType != model.UserRedemptionRewardBalance && input.ExpectedRewardType != model.UserRedemptionRewardSubscription {
		return nil, ErrUserRedemptionInvalid
	}
	if input.ExpectedRewardType != "" && input.ExpectedRewardType != actualRewardType {
		return nil, &UserRedemptionRewardTypeMismatchError{
			Expected: input.ExpectedRewardType,
			Actual:   actualRewardType,
		}
	}
	if actualRewardType == model.UserRedemptionRewardSubscription {
		var entitlement model.UserRedemptionSubscriptionEntitlement
		if err := model.DB.WithContext(ctx).Select("plan_id").Where("code_id = ?", route.ID).First(&entitlement).Error; err != nil {
			return nil, ErrUserRedemptionConflict
		}
		return claimSubscriptionRedemptionCode(ctx, input, route.ID, entitlement.PlanID)
	}
	return claimBalanceUserRedemptionCode(ctx, input.ClaimerUserID, input.Code)
}

func claimBalanceUserRedemptionCode(ctx context.Context, claimerUserID int, code string) (*model.UserRedemptionClaim, error) {
	var claim *model.UserRedemptionClaim
	var expiredInTxn bool
	err := model.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		row, err := model.LockUserRedemptionCodeByCode(tx, code)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrUserRedemptionNotFound
			}
			return err
		}
		now := common.GetTimestamp()
		if row.Status != model.UserRedemptionStatusActive {
			switch row.Status {
			case model.UserRedemptionStatusDisabled:
				return ErrUserRedemptionCodeDisabled
			case model.UserRedemptionStatusExpiredPending:
				return ErrUserRedemptionExpired
			case model.UserRedemptionStatusExhausted:
				return ErrUserRedemptionExhausted
			default:
				return ErrUserRedemptionInactive
			}
		}
		if isUserRedemptionExpired(row.ExpiresAt, now) {
			row.Status = model.UserRedemptionStatusExpiredPending
			row.TerminalAt = now
			if err := tx.Model(&model.UserRedemptionCode{}).Where("id = ?", row.ID).Updates(map[string]any{
				"status":      row.Status,
				"terminal_at": row.TerminalAt,
			}).Error; err != nil {
				return err
			}
			if err := enqueueUserRedemptionBusinessAuditWithTx(tx, fmt.Sprintf("user-redemption:%d:expire-pending", row.ID), "user.redemption.expire_pending", 0, row.CreatorUserID, row, 0, 0); err != nil {
				return err
			}
			expiredInTxn = true
			return nil
		}
		var existing model.UserRedemptionClaim
		err = tx.Where("code_id = ? AND claimer_user_id = ?", row.ID, claimerUserID).First(&existing).Error
		if err == nil {
			return ErrUserRedemptionAlreadyClaimed
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if row.ClaimCount >= row.MaxClaims {
			return ErrUserRedemptionExhausted
		}
		remainingClaims := row.MaxClaims - row.ClaimCount
		remainingQuota := row.TotalQuota - row.ClaimedQuota
		if remainingClaims <= 0 || remainingQuota <= 0 {
			return ErrUserRedemptionExhausted
		}
		minUnit, err := userRedemptionStandardUnit()
		if err != nil {
			return err
		}
		quota, err := randomClaimQuota(remainingQuota, remainingClaims, minUnit)
		if err != nil {
			return err
		}
		if quota <= 0 || quota > remainingQuota {
			return ErrUserRedemptionInvalid
		}
		creditOp := fmt.Sprintf("user-redemption:%d:claim:%d", row.ID, claimerUserID)
		claimRow := model.UserRedemptionClaim{
			CodeID:        row.ID,
			ClaimerUserID: claimerUserID,
			RewardType:    model.UserRedemptionRewardBalance,
			Quota:         quota,
			CreditOpID:    creditOp,
			ClaimedAt:     now,
		}
		if err := tx.Create(&claimRow).Error; err != nil {
			if isUniqueViolation(err) {
				return ErrUserRedemptionAlreadyClaimed
			}
			return err
		}
		if _, err := AdjustWithTx(tx, claimerUserID, quota, creditOp, userRedemptionCategoryClaim, WalletReference{
			Type:    "user_redemption_code",
			ID:      fmt.Sprintf("%d", row.ID),
			Context: "claim",
		}); err != nil {
			return err
		}
		row.ClaimCount++
		row.ClaimedQuota += quota
		updates := map[string]any{
			"claim_count":   row.ClaimCount,
			"claimed_quota": row.ClaimedQuota,
		}
		if row.ClaimCount >= row.MaxClaims || row.ClaimedQuota >= row.TotalQuota {
			row.Status = model.UserRedemptionStatusExhausted
			row.TerminalAt = now
			updates["status"] = row.Status
			updates["terminal_at"] = row.TerminalAt
		}
		if err := tx.Model(&model.UserRedemptionCode{}).Where("id = ?", row.ID).Updates(updates).Error; err != nil {
			return err
		}
		if err := enqueueUserRedemptionBusinessAuditWithTx(tx, fmt.Sprintf("user-redemption:%d:claim:%d:audit", row.ID, claimerUserID), "user.redemption.claim", claimerUserID, claimerUserID, row, quota, claimerUserID); err != nil {
			return err
		}
		claim = &claimRow
		return nil
	})
	if err != nil {
		return nil, err
	}
	if expiredInTxn {
		return nil, ErrUserRedemptionExpired
	}
	_ = invalidateUserRedemptionCache(claimerUserID)
	return claim, nil
}

func CancelUserRedemptionCode(ctx context.Context, actor UserIdentity, codeID int64) (*model.UserRedemptionCode, error) {
	if model.DB == nil || actor.ID <= 0 || codeID <= 0 {
		return nil, ErrUserRedemptionInvalid
	}
	var route model.UserRedemptionCode
	if err := model.DB.WithContext(ctx).Select("creator_user_id", "reward_type").First(&route, codeID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserRedemptionNotFound
		}
		return nil, err
	}
	if route.CreatorUserID != actor.ID {
		return nil, ErrUserRedemptionForbidden
	}
	isSubscription := normalizeUserRedemptionRewardType(route.RewardType) == model.UserRedemptionRewardSubscription

	var out *model.UserRedemptionCode
	err := model.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if isSubscription {
			if err := model.LockUserRedemptionCreator(tx, route.CreatorUserID); err != nil {
				return err
			}
		}
		row, err := model.LockUserRedemptionCodeByID(tx, codeID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrUserRedemptionNotFound
			}
			return err
		}
		// Administrators must use AdminMutateUserRedemptionCode so settlement and
		// reason are always explicit and auditable.
		if row.CreatorUserID != actor.ID {
			return ErrUserRedemptionForbidden
		}
		if isSubscription && (row.CreatorUserID != route.CreatorUserID || normalizeUserRedemptionRewardType(row.RewardType) != model.UserRedemptionRewardSubscription) {
			return ErrUserRedemptionConflict
		}
		now := common.GetTimestamp()
		if row.Status != model.UserRedemptionStatusActive {
			return ErrUserRedemptionInactive
		}
		row.TerminalAt = now
		row.TerminalActorID = actor.ID
		row.TerminalReason = "creator_cancelled"
		if isSubscription {
			refundable, err := userRedemptionRefundableQuotaWithTx(tx, row)
			if err != nil {
				return err
			}
			if err := refundUserRedemptionWithTx(tx, &row, now); err != nil {
				return err
			}
			if err := tx.Model(&model.UserRedemptionCode{}).Where("id = ?", row.ID).Updates(map[string]any{
				"terminal_actor_id": row.TerminalActorID,
				"terminal_reason":   row.TerminalReason,
			}).Error; err != nil {
				return err
			}
			if err := enqueueUserRedemptionBusinessAuditWithTx(tx, fmt.Sprintf("user-redemption:%d:cancel-refund", row.ID), "user.redemption.subscription.refund", actor.ID, row.CreatorUserID, row, refundable, 0); err != nil {
				return err
			}
			out = &row
			return nil
		}

		row.Status = model.UserRedemptionStatusExpiredPending
		if err := tx.Model(&model.UserRedemptionCode{}).Where("id = ?", row.ID).Updates(map[string]any{
			"status":            row.Status,
			"terminal_at":       row.TerminalAt,
			"terminal_actor_id": row.TerminalActorID,
			"terminal_reason":   row.TerminalReason,
		}).Error; err != nil {
			return err
		}
		if err := enqueueUserRedemptionBusinessAuditWithTx(tx, fmt.Sprintf("user-redemption:%d:cancel-pending", row.ID), "user.redemption.cancel_pending", actor.ID, row.CreatorUserID, row, 0, 0); err != nil {
			return err
		}
		out = &row
		return nil
	})
	if err != nil {
		return nil, err
	}
	if out != nil {
		_ = invalidateUserRedemptionCache(out.CreatorUserID)
	}
	return out, nil
}

func FinalizeExpiredUserRedemptionCodes(ctx context.Context, limit int) (int, error) {
	if model.DB == nil {
		return 0, ErrUserRedemptionInvalid
	}
	if limit <= 0 {
		limit = 100
	}
	now := common.GetTimestamp()
	var ids []int64
	if err := model.DB.WithContext(ctx).
		Model(&model.UserRedemptionCode{}).
		Where("status IN ? AND expires_at > 0 AND expires_at <= ?", []string{model.UserRedemptionStatusActive, model.UserRedemptionStatusDisabled}, now).
		Order("id asc").
		Limit(limit).
		Pluck("id", &ids).Error; err != nil {
		return 0, err
	}
	finalized := 0
	for _, id := range ids {
		var creatorID int
		var didFinalize bool
		err := model.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			row, err := model.LockUserRedemptionCodeByID(tx, id)
			if err != nil {
				return err
			}
			if row.Status != model.UserRedemptionStatusActive && row.Status != model.UserRedemptionStatusDisabled {
				return nil
			}
			if !isUserRedemptionExpired(row.ExpiresAt, common.GetTimestamp()) {
				return nil
			}
			row.Status = model.UserRedemptionStatusExpiredPending
			if normalizeUserRedemptionRewardType(row.RewardType) == model.UserRedemptionRewardSubscription {
				row.Status = model.UserRedemptionStatusRefundPending
			}
			row.TerminalAt = common.GetTimestamp()
			row.TerminalReason = "expired"
			if err := tx.Model(&model.UserRedemptionCode{}).Where("id = ?", row.ID).Updates(map[string]any{
				"status":          row.Status,
				"terminal_at":     row.TerminalAt,
				"terminal_reason": row.TerminalReason,
			}).Error; err != nil {
				return err
			}
			eventType := "user.redemption.expire_pending"
			if row.Status == model.UserRedemptionStatusRefundPending {
				eventType = "user.redemption.subscription.refund_pending"
			}
			if err := enqueueUserRedemptionBusinessAuditWithTx(tx, fmt.Sprintf("user-redemption:%d:expire-pending", row.ID), eventType, 0, row.CreatorUserID, row, 0, 0); err != nil {
				return err
			}
			creatorID = row.CreatorUserID
			didFinalize = true
			return nil
		})
		if err != nil {
			return finalized, err
		}
		if didFinalize {
			if creatorID > 0 {
				_ = invalidateUserRedemptionCache(creatorID)
			}
			finalized++
		}
	}
	return finalized, nil
}

func GetUserRedemptionDetail(ctx context.Context, actor UserIdentity, codeID int64, includeClaims bool) (*UserRedemptionDetail, error) {
	if model.DB == nil || actor.ID <= 0 || codeID <= 0 {
		return nil, ErrUserRedemptionInvalid
	}
	var row model.UserRedemptionCode
	if err := model.DB.WithContext(ctx).First(&row, codeID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserRedemptionNotFound
		}
		return nil, err
	}
	if row.CreatorUserID != actor.ID {
		// Any authenticated admin/root may inspect any user redemption code.
		// Ordinary users may only read their own codes.
		if actor.Role < common.RoleAdminUser {
			return nil, ErrUserRedemptionForbidden
		}
	}
	usernames, err := loadUserRedemptionUsernames(ctx, []int{row.CreatorUserID})
	if err != nil {
		return nil, err
	}
	var entitlement *model.UserRedemptionSubscriptionEntitlement
	if normalizeUserRedemptionRewardType(row.RewardType) == model.UserRedemptionRewardSubscription {
		var snapshot model.UserRedemptionSubscriptionEntitlement
		if err := model.DB.WithContext(ctx).Where("code_id = ?", row.ID).First(&snapshot).Error; err != nil {
			return nil, err
		}
		entitlement = &snapshot
	}
	detail := &UserRedemptionDetail{Code: userRedemptionCodeView(row, entitlement, actor, usernames[row.CreatorUserID])}
	if includeClaims {
		claims, err := model.ListUserRedemptionClaims(codeID)
		if err != nil {
			return nil, err
		}
		claimantIDs := make([]int, 0, len(claims))
		for _, claim := range claims {
			claimantIDs = append(claimantIDs, claim.ClaimerUserID)
		}
		claimantUsernames, err := loadUserRedemptionUsernames(ctx, claimantIDs)
		if err != nil {
			return nil, err
		}
		detail.Claims = make([]UserRedemptionClaimView, 0, len(claims))
		for _, claim := range claims {
			claimView := UserRedemptionClaimView{
				ID:                 claim.ID,
				RewardType:         normalizeUserRedemptionRewardType(claim.RewardType),
				Quota:              claim.Quota,
				UserSubscriptionID: claim.UserSubscriptionID,
				Amount:             common.WalletAmountFromQuota(claim.Quota),
				ClaimedAt:          claim.ClaimedAt,
			}
			username := claimantUsernames[claim.ClaimerUserID]
			if actor.Role >= common.RoleAdminUser {
				claimView.ClaimantUserID = claim.ClaimerUserID
				claimView.ClaimantUsername = username
			} else {
				claimView.ClaimantDisplay = maskUserRedemptionUsername(username)
			}
			detail.Claims = append(detail.Claims, claimView)
		}
	}
	return detail, nil
}

func ListUserRedemptionCodes(ctx context.Context, actor UserIdentity, q UserRedemptionListQuery) (*UserRedemptionListResult, error) {
	if model.DB == nil || actor.ID <= 0 {
		return nil, ErrUserRedemptionInvalid
	}
	if q.Page < 1 {
		q.Page = 1
	}
	if q.PageSize < 1 {
		q.PageSize = common.ItemsPerPage
	}
	if q.PageSize > 100 {
		q.PageSize = 100
	}
	// Authorization lives here (not only on the router). Owner-scoped queries
	// remain owner-scoped even when the actor is an administrator.
	ownerScope := q.OwnerOnly || actor.Role < common.RoleAdminUser
	if ownerScope {
		q.CreatorUserID = actor.ID
	}
	query := model.DB.WithContext(ctx).Model(&model.UserRedemptionCode{})
	if q.CreatorUserID > 0 {
		query = query.Where("creator_user_id = ?", q.CreatorUserID)
	}
	if actor.Role >= common.RoleAdminUser && !ownerScope {
		if creatorUsername := strings.TrimSpace(q.CreatorUsername); creatorUsername != "" {
			var creatorIDs []int
			if err := model.DB.WithContext(ctx).Model(&model.User{}).
				Where("username LIKE ?", "%"+creatorUsername+"%").
				Pluck("id", &creatorIDs).Error; err != nil {
				return nil, err
			}
			if len(creatorIDs) == 0 {
				query = query.Where("1 = 0")
			} else {
				query = query.Where("creator_user_id IN ?", creatorIDs)
			}
		}
	}
	deletedFilter := strings.ToLower(strings.TrimSpace(q.Deleted))
	if ownerScope {
		switch deletedFilter {
		case "", "all":
		case "false":
			query = query.Where("deleted_at = ?", 0)
		case "true":
			query = query.Where("deleted_at <> ?", 0)
		default:
			return nil, ErrUserRedemptionInvalid
		}
	} else {
		switch deletedFilter {
		case "", "false":
			query = query.Where("deleted_at = ?", 0)
		case "true":
			query = query.Where("deleted_at <> ?", 0)
		case "all":
		default:
			return nil, ErrUserRedemptionInvalid
		}
	}
	if status := strings.TrimSpace(q.Status); status != "" {
		query = query.Where("status = ?", status)
	}
	if rewardType := strings.TrimSpace(strings.ToLower(q.RewardType)); rewardType != "" {
		if rewardType != model.UserRedemptionRewardBalance && rewardType != model.UserRedemptionRewardSubscription {
			return nil, ErrUserRedemptionInvalid
		}
		query = query.Where("reward_type = ?", rewardType)
	}
	if kind := strings.TrimSpace(q.Kind); kind != "" {
		query = query.Where("kind = ?", kind)
	}
	if q.CreatedFrom > 0 {
		query = query.Where("created_at >= ?", q.CreatedFrom)
	}
	if q.CreatedTo > 0 {
		query = query.Where("created_at <= ?", q.CreatedTo)
	}
	if q.ExpiresFrom > 0 {
		query = query.Where("expires_at >= ?", q.ExpiresFrom)
	}
	if q.ExpiresTo > 0 {
		query = query.Where("expires_at <= ?", q.ExpiresTo)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	result := &UserRedemptionListResult{
		Items:    make([]UserRedemptionCodeView, 0),
		Total:    total,
		Page:     q.Page,
		PageSize: q.PageSize,
	}
	pageStart := int64(q.Page - 1)
	pageCount := (total + int64(q.PageSize) - 1) / int64(q.PageSize)
	maxInt := int64(^uint(0) >> 1)
	if pageStart >= pageCount || pageStart > maxInt/int64(q.PageSize) {
		return result, nil
	}
	var items []model.UserRedemptionCode
	offset := int(pageStart * int64(q.PageSize))
	if err := query.Order("id desc").Offset(offset).Limit(q.PageSize).Find(&items).Error; err != nil {
		return nil, err
	}
	creatorIDs := make([]int, 0, len(items))
	for _, item := range items {
		creatorIDs = append(creatorIDs, item.CreatorUserID)
	}
	creatorUsernames, err := loadUserRedemptionUsernames(ctx, creatorIDs)
	if err != nil {
		return nil, err
	}
	codeIDs := make([]int64, 0, len(items))
	for _, item := range items {
		if normalizeUserRedemptionRewardType(item.RewardType) == model.UserRedemptionRewardSubscription {
			codeIDs = append(codeIDs, item.ID)
		}
	}
	entitlements := make(map[int64]model.UserRedemptionSubscriptionEntitlement, len(codeIDs))
	if len(codeIDs) > 0 {
		var snapshots []model.UserRedemptionSubscriptionEntitlement
		if err := model.DB.WithContext(ctx).Where("code_id IN ?", codeIDs).Find(&snapshots).Error; err != nil {
			return nil, err
		}
		for _, snapshot := range snapshots {
			entitlements[snapshot.CodeID] = snapshot
		}
	}
	views := make([]UserRedemptionCodeView, 0, len(items))
	for _, item := range items {
		var entitlement *model.UserRedemptionSubscriptionEntitlement
		if snapshot, ok := entitlements[item.ID]; ok {
			snapshotCopy := snapshot
			entitlement = &snapshotCopy
		}
		views = append(views, userRedemptionCodeView(item, entitlement, actor, creatorUsernames[item.CreatorUserID]))
	}
	result.Items = views
	return result, nil
}

func randomClaimQuota(remaining int64, remainingClaims int, minimum int64) (int64, error) {
	if remainingClaims <= 0 || remaining <= 0 || minimum <= 0 {
		return 0, ErrUserRedemptionInvalid
	}
	if !hasMinimumUserRedemptionQuota(remaining, minimum, remainingClaims) {
		return 0, ErrUserRedemptionInvalid
	}
	if remainingClaims == 1 {
		return remaining, nil
	}
	// Keep enough for every remaining claimant including this one.
	spendable := remaining - minimum*int64(remainingClaims-1)
	if spendable < minimum {
		return 0, ErrUserRedemptionInvalid
	}
	averageBase := remaining / int64(remainingClaims)
	const maxInt64 = int64(^uint64(0) >> 1)
	averageBound := maxInt64
	if averageBase <= maxInt64/2 {
		averageBound = averageBase * 2
	}
	upper := spendable
	if averageBound > minimum && averageBound < upper {
		upper = averageBound
	}
	if upper < minimum {
		upper = minimum
	}
	return secureInt64Between(minimum, upper)
}

func secureInt64Between(minV, maxV int64) (int64, error) {
	if maxV < minV {
		return 0, ErrUserRedemptionInvalid
	}
	if maxV == minV {
		return minV, nil
	}
	span := new(big.Int).Sub(big.NewInt(maxV), big.NewInt(minV))
	span.Add(span, big.NewInt(1))
	n, err := rand.Int(rand.Reader, span)
	if err != nil {
		return 0, err
	}
	n.Add(n, big.NewInt(minV))
	return n.Int64(), nil
}

func generateUserRedemptionCode(tx *gorm.DB) (string, error) {
	for i := 0; i < 12; i++ {
		code := strings.ToUpper(common.GetRandomString(userRedemptionCodeLen))
		available, err := model.IsRedemptionCodeAvailable(tx, code)
		if err != nil {
			return "", err
		}
		if available {
			return code, nil
		}
	}
	return "", fmt.Errorf("failed to allocate unique user redemption code")
}

func refundUserRedemptionWithTx(tx *gorm.DB, row *model.UserRedemptionCode, now int64) error {
	refundable, err := userRedemptionRefundableQuotaWithTx(tx, *row)
	if err != nil {
		return err
	}
	refundOp := fmt.Sprintf("user-redemption:%d:refund", row.ID)
	row.Status = model.UserRedemptionStatusRefunded
	if refundable > 0 {
		if _, err := adjustUnscopedUserWithTx(tx, row.CreatorUserID, refundable, refundOp, userRedemptionCategoryRefund, WalletReference{
			Type:    "user_redemption_code",
			ID:      fmt.Sprintf("%d", row.ID),
			Context: row.Status,
		}); err != nil {
			return err
		}
		row.RefundedQuota += refundable
	}
	row.TerminalAt = now
	row.RefundOpID = refundOp
	return tx.Model(&model.UserRedemptionCode{}).Where("id = ?", row.ID).Updates(map[string]any{
		"status":              row.Status,
		"terminal_at":         row.TerminalAt,
		"refunded_quota":      row.RefundedQuota,
		"refund_operation_id": row.RefundOpID,
	}).Error
}

func userRedemptionRefundableQuotaWithTx(tx *gorm.DB, row model.UserRedemptionCode) (int64, error) {
	if normalizeUserRedemptionRewardType(row.RewardType) == model.UserRedemptionRewardSubscription {
		return subscriptionRedemptionRefundableQuotaWithTx(tx, row)
	}
	return UserRedemptionRemainingQuota(row), nil
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique") || strings.Contains(msg, "duplicate")
}

func enqueueUserRedemptionBusinessAuditWithTx(tx *gorm.DB, eventID, eventType string, actorID, logUserID int, row model.UserRedemptionCode, quota int64, claimantUserID int, extra ...map[string]any) error {
	params := map[string]any{
		"id":             row.ID,
		"action":         eventType,
		"status":         row.Status,
		"target_user_id": row.CreatorUserID,
		"kind":           row.Kind,
	}
	if quota > 0 {
		params["quota"] = quota
	}
	if claimantUserID > 0 {
		params["claimant_user_id"] = claimantUserID
	}
	for _, values := range extra {
		for key, value := range values {
			params[key] = value
		}
	}
	requestIP := ""
	requestID := ""
	if tx != nil && tx.Statement != nil && tx.Statement.Context != nil {
		if value, ok := tx.Statement.Context.Value(constant.ContextKeyClientIP).(string); ok {
			requestIP = value
		}
		if value, ok := tx.Statement.Context.Value(constant.ContextKey(common.RequestIdKey)).(string); ok {
			requestID = value
		}
	}
	return EnqueueWithTx(tx, AuditEvent{
		EventID:   eventID,
		Type:      eventType,
		ActorID:   actorID,
		LogUserID: logUserID,
		Target:    fmt.Sprintf("user_redemption_code:%d", row.ID),
		IP:        requestIP,
		RequestID: requestID,
		Params:    params,
	})
}
