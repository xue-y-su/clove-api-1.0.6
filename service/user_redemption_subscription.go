package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/user_redemption_setting"
	"gorm.io/gorm"
)

var (
	ErrUserRedemptionPlanDisabled           = errors.New("subscription plan is disabled")
	ErrUserRedemptionBalancePaymentDisabled = errors.New("subscription plan does not allow balance payment")
	ErrUserRedemptionPlanPurchaseLimit      = errors.New("subscription plan purchase limit reached")
)

type CreateSubscriptionRedemptionInput struct {
	CreatorUserID int
	PlanID        int
	Units         int
	ExpiresAt     int64
}

func subscriptionPlanFromEntitlement(entitlement model.UserRedemptionSubscriptionEntitlement) *model.SubscriptionPlan {
	return &model.SubscriptionPlan{
		Id:                      entitlement.PlanID,
		Title:                   entitlement.PlanTitle,
		Subtitle:                entitlement.PlanSubtitle,
		PriceAmount:             entitlement.PriceAmount,
		Currency:                entitlement.Currency,
		DurationUnit:            entitlement.DurationUnit,
		DurationValue:           entitlement.DurationValue,
		CustomSeconds:           entitlement.CustomSeconds,
		Enabled:                 true,
		AllowBalancePay:         common.GetPointer(true),
		AllowWalletOverflow:     common.GetPointer(entitlement.AllowWalletOverflow),
		MaxPurchasePerUser:      entitlement.MaxPurchasePerUser,
		UpgradeGroup:            entitlement.UpgradeGroup,
		DowngradeGroup:          entitlement.DowngradeGroup,
		TotalAmount:             entitlement.TotalAmount,
		QuotaResetPeriod:        entitlement.QuotaResetPeriod,
		QuotaResetCustomSeconds: entitlement.QuotaResetCustomSeconds,
	}
}

func multiplySubscriptionRedemptionQuota(unitQuota int64, units int) (int64, error) {
	if unitQuota < 0 || units <= 0 {
		return 0, ErrUserRedemptionInvalid
	}
	if unitQuota > 0 && int64(units) > math.MaxInt64/unitQuota {
		return 0, ErrUserRedemptionInvalid
	}
	return unitQuota * int64(units), nil
}

func subscriptionRedemptionRefundableQuotaWithTx(tx *gorm.DB, row model.UserRedemptionCode) (int64, error) {
	if tx == nil || row.ID <= 0 || row.MaxClaims <= 0 {
		return 0, ErrUserRedemptionInvalid
	}
	var entitlement model.UserRedemptionSubscriptionEntitlement
	if err := tx.Where("code_id = ?", row.ID).First(&entitlement).Error; err != nil {
		return 0, err
	}
	if entitlement.UnitPaidQuota < 0 {
		return 0, ErrUserRedemptionConflict
	}
	var claimCount int64
	if err := tx.Model(&model.UserRedemptionClaim{}).Where("code_id = ?", row.ID).Count(&claimCount).Error; err != nil {
		return 0, err
	}
	if claimCount < 0 || claimCount > int64(row.MaxClaims) {
		return 0, ErrUserRedemptionConflict
	}
	total, err := multiplySubscriptionRedemptionQuota(entitlement.UnitPaidQuota, row.MaxClaims)
	if err != nil {
		return 0, err
	}
	claimed, err := multiplySubscriptionRedemptionQuota(entitlement.UnitPaidQuota, int(claimCount))
	if err != nil && claimCount != 0 {
		return 0, err
	}
	remaining := total
	for _, consumed := range []int64{claimed, row.RefundedQuota, row.ForfeitedQuota} {
		if consumed < 0 {
			return 0, ErrUserRedemptionConflict
		}
		if consumed >= remaining {
			return 0, nil
		}
		remaining -= consumed
	}
	return remaining, nil
}

func CreateSubscriptionRedemptionCode(ctx context.Context, input CreateSubscriptionRedemptionInput) (*model.UserRedemptionCode, error) {
	if model.DB == nil || input.CreatorUserID <= 0 || input.PlanID <= 0 || input.Units <= 0 {
		return nil, ErrUserRedemptionInvalid
	}
	settings := user_redemption_setting.GetSettings()
	if !settings.Enabled {
		return nil, ErrUserRedemptionDisabled
	}
	if !settings.SubscriptionEnabled {
		return nil, ErrUserRedemptionModeDisabled
	}
	if input.Units > settings.MaxClaimsPerCode {
		return nil, ErrUserRedemptionClaimsLimit
	}
	now, err := validateUserRedemptionExpiry(input.ExpiresAt, settings)
	if err != nil {
		return nil, err
	}

	if common.UsingMainDatabase(common.DatabaseTypeSQLite) {
		sqliteUserRedemptionCreationMutex.Lock()
		defer sqliteUserRedemptionCreationMutex.Unlock()
	}

	var created *model.UserRedemptionCode
	err = model.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		plan, err := model.LockSubscriptionPlanByID(tx, input.PlanID)
		if err != nil {
			return err
		}
		if !plan.Enabled {
			return ErrUserRedemptionPlanDisabled
		}
		if plan.PriceAmount < 0 {
			return ErrUserRedemptionInvalid
		}
		if plan.AllowBalancePay != nil && !*plan.AllowBalancePay {
			return ErrUserRedemptionBalancePaymentDisabled
		}

		unitQuotaValue, err := model.CalculateSubscriptionBalanceQuota(plan.PriceAmount)
		if err != nil {
			return err
		}
		unitQuota := int64(unitQuotaValue)
		totalQuota, err := multiplySubscriptionRedemptionQuota(unitQuota, input.Units)
		if err != nil {
			return err
		}

		if err := model.LockUserRedemptionCreator(tx, input.CreatorUserID); err != nil {
			return err
		}
		var creatorRole int
		if err := tx.Model(&model.User{}).Where("id = ?", input.CreatorUserID).Pluck("role", &creatorRole).Error; err != nil {
			return err
		}
		if settings.MaxActiveCodesPerUser > 0 {
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
			Code:           codeValue,
			CreatorUserID:  input.CreatorUserID,
			IssuedBy:       input.CreatorUserID,
			OfficialIssuer: creatorRole >= common.RoleAdminUser,
			RewardType:     model.UserRedemptionRewardSubscription,
			Kind:           model.UserRedemptionKindFixed,
			TotalQuota:     totalQuota,
			MaxClaims:      input.Units,
			Status:         model.UserRedemptionStatusActive,
			ExpiresAt:      input.ExpiresAt,
			ReserveOpID:    fmt.Sprintf("user-redemption:pending:%d:%s", input.CreatorUserID, codeValue),
		}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}

		entitlement := model.UserRedemptionSubscriptionEntitlement{
			CodeID:                  row.ID,
			PlanID:                  plan.Id,
			PlanTitle:               plan.Title,
			PlanSubtitle:            plan.Subtitle,
			Currency:                plan.Currency,
			PriceAmount:             plan.PriceAmount,
			UnitPaidQuota:           unitQuota,
			DurationUnit:            plan.DurationUnit,
			DurationValue:           plan.DurationValue,
			CustomSeconds:           plan.CustomSeconds,
			TotalAmount:             plan.TotalAmount,
			QuotaResetPeriod:        plan.QuotaResetPeriod,
			QuotaResetCustomSeconds: plan.QuotaResetCustomSeconds,
			AllowWalletOverflow:     plan.AllowWalletOverflow == nil || *plan.AllowWalletOverflow,
			UpgradeGroup:            plan.UpgradeGroup,
			DowngradeGroup:          plan.DowngradeGroup,
			MaxPurchasePerUser:      plan.MaxPurchasePerUser,
		}
		if err := tx.Create(&entitlement).Error; err != nil {
			return err
		}

		reserveOp := fmt.Sprintf("user-redemption:%d:reserve", row.ID)
		if totalQuota > 0 {
			if _, err := AdjustWithTx(tx, input.CreatorUserID, -totalQuota, reserveOp, userRedemptionCategoryReserve, WalletReference{
				Type:    "user_redemption_code",
				ID:      fmt.Sprintf("%d", row.ID),
				Context: "subscription_reserve",
			}); err != nil {
				if errors.Is(err, ErrWalletInsufficientBalance) {
					available := int64(0)
					var balanceError *model.WalletInsufficientBalanceError
					if errors.As(err, &balanceError) {
						available = balanceError.Balance
					}
					return &UserRedemptionInsufficientBalanceError{Requested: totalQuota, Available: available}
				}
				return err
			}
		}
		if err := tx.Model(&row).Update("reserve_operation_id", reserveOp).Error; err != nil {
			return err
		}
		row.ReserveOpID = reserveOp
		if err := enqueueUserRedemptionBusinessAuditWithTx(tx, fmt.Sprintf("user-redemption:%d:create", row.ID), "user.redemption.subscription.create", input.CreatorUserID, input.CreatorUserID, row, totalQuota, 0); err != nil {
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

func SealSubscriptionRedemptionsForPlanTx(tx *gorm.DB, planID int, actorID int, now int64) error {
	if tx == nil || planID <= 0 || now <= 0 {
		return ErrUserRedemptionInvalid
	}
	codeIDs := tx.Model(&model.UserRedemptionSubscriptionEntitlement{}).
		Select("code_id").
		Where("plan_id = ?", planID)
	var rows []model.UserRedemptionCode
	if err := tx.Where("id IN (?) AND reward_type = ? AND deleted_at = ? AND status IN ?", codeIDs, model.UserRedemptionRewardSubscription, 0, []string{
		model.UserRedemptionStatusActive,
		model.UserRedemptionStatusDisabled,
	}).Order("id ASC").Find(&rows).Error; err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(rows))
	for i := range rows {
		ids = append(ids, rows[i].ID)
	}
	result := tx.Model(&model.UserRedemptionCode{}).
		Where("id IN ? AND reward_type = ? AND deleted_at = ? AND status IN ?", ids, model.UserRedemptionRewardSubscription, 0, []string{
			model.UserRedemptionStatusActive,
			model.UserRedemptionStatusDisabled,
		}).
		Updates(map[string]any{
			"status":            model.UserRedemptionStatusRefundPending,
			"terminal_at":       now,
			"terminal_actor_id": actorID,
			"terminal_reason":   "plan_disabled",
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != int64(len(rows)) {
		return ErrUserRedemptionConflict
	}
	for i := range rows {
		rows[i].Status = model.UserRedemptionStatusRefundPending
		rows[i].TerminalAt = now
		rows[i].TerminalActorID = actorID
		rows[i].TerminalReason = "plan_disabled"
		if err := enqueueUserRedemptionBusinessAuditWithTx(tx, fmt.Sprintf("user-redemption:%d:plan-disabled", rows[i].ID), "user.redemption.subscription.plan_disabled", actorID, rows[i].CreatorUserID, rows[i], 0, 0, map[string]any{
			"plan_id": planID,
		}); err != nil {
			return err
		}
	}
	return nil
}

func SetSubscriptionPlanEnabledWithTx(tx *gorm.DB, planID int, enabled bool, actorID int, now int64) error {
	if tx == nil || planID <= 0 || now <= 0 {
		return ErrUserRedemptionInvalid
	}
	plan, err := model.LockSubscriptionPlanByID(tx, planID)
	if err != nil {
		return err
	}
	if plan.Enabled != enabled {
		if err := tx.Model(&model.SubscriptionPlan{}).Where("id = ?", planID).Updates(map[string]any{
			"enabled":    enabled,
			"updated_at": now,
		}).Error; err != nil {
			return err
		}
	}
	if !enabled {
		return SealSubscriptionRedemptionsForPlanTx(tx, planID, actorID, now)
	}
	return nil
}

func SetSubscriptionPlanEnabled(ctx context.Context, planID int, enabled bool, actorID int) error {
	if model.DB == nil || planID <= 0 {
		return ErrUserRedemptionInvalid
	}
	err := model.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return SetSubscriptionPlanEnabledWithTx(tx, planID, enabled, actorID, common.GetTimestamp())
	})
	if err != nil {
		return err
	}
	model.InvalidateSubscriptionPlanCache(planID)
	return nil
}

func claimSubscriptionRedemptionCode(ctx context.Context, input ClaimUserRedemptionInput, codeID int64, planID int) (*model.UserRedemptionClaim, error) {
	for attempt := 0; attempt < 4; attempt++ {
		claim, err := claimSubscriptionRedemptionCodeOnce(ctx, input, codeID, planID)
		if err == nil || !isSQLiteBusyError(err) {
			return claim, err
		}
		time.Sleep(time.Duration(attempt+1) * 10 * time.Millisecond)
	}
	return nil, ErrUserRedemptionConflict
}

func claimSubscriptionRedemptionCodeOnce(ctx context.Context, input ClaimUserRedemptionInput, codeID int64, planID int) (*model.UserRedemptionClaim, error) {
	var claim *model.UserRedemptionClaim
	var upgradeGroup string
	var expiredInTxn bool
	err := model.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		plan, err := model.LockSubscriptionPlanByID(tx, planID)
		if err != nil {
			return err
		}
		if err := model.LockUserRedemptionCreator(tx, input.ClaimerUserID); err != nil {
			return err
		}
		row, err := model.LockUserRedemptionCodeByID(tx, codeID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrUserRedemptionNotFound
			}
			return err
		}
		if normalizeUserRedemptionRewardType(row.RewardType) != model.UserRedemptionRewardSubscription {
			return ErrUserRedemptionConflict
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
		if !plan.Enabled {
			return ErrUserRedemptionPlanDisabled
		}
		if isUserRedemptionExpired(row.ExpiresAt, now) {
			row.Status = model.UserRedemptionStatusRefundPending
			row.TerminalAt = now
			row.TerminalReason = "expired"
			if err := tx.Model(&model.UserRedemptionCode{}).Where("id = ?", row.ID).Updates(map[string]any{
				"status":          row.Status,
				"terminal_at":     row.TerminalAt,
				"terminal_reason": row.TerminalReason,
			}).Error; err != nil {
				return err
			}
			if err := enqueueUserRedemptionBusinessAuditWithTx(tx, fmt.Sprintf("user-redemption:%d:refund-pending", row.ID), "user.redemption.subscription.refund_pending", 0, row.CreatorUserID, row, 0, 0); err != nil {
				return err
			}
			expiredInTxn = true
			return nil
		}

		var entitlement model.UserRedemptionSubscriptionEntitlement
		if err := tx.Where("code_id = ?", row.ID).First(&entitlement).Error; err != nil {
			return err
		}
		if entitlement.PlanID != plan.Id || entitlement.UnitPaidQuota < 0 {
			return ErrUserRedemptionConflict
		}
		var existing model.UserRedemptionClaim
		err = tx.Where("code_id = ? AND claimer_user_id = ?", row.ID, input.ClaimerUserID).First(&existing).Error
		if err == nil {
			return ErrUserRedemptionAlreadyClaimed
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if row.ClaimCount >= row.MaxClaims {
			return ErrUserRedemptionExhausted
		}
		snapshotPlan := subscriptionPlanFromEntitlement(entitlement)
		snapshotPlan.MaxPurchasePerUser = model.StricterPositiveSubscriptionLimit(
			entitlement.MaxPurchasePerUser,
			plan.MaxPurchasePerUser,
		)
		subscription, err := model.CreateUserSubscriptionFromPlanTx(tx, input.ClaimerUserID, snapshotPlan, "redemption")
		if err != nil {
			if errors.Is(err, model.ErrSubscriptionPurchaseLimit) || errors.Is(err, model.ErrSubscriptionPurchaseCounterCorrupt) {
				return ErrUserRedemptionPlanPurchaseLimit
			}
			return err
		}
		creditOp := fmt.Sprintf("user-redemption:%d:subscription-claim:%d", row.ID, input.ClaimerUserID)
		claimRow := model.UserRedemptionClaim{
			CodeID:             row.ID,
			ClaimerUserID:      input.ClaimerUserID,
			RewardType:         model.UserRedemptionRewardSubscription,
			Quota:              entitlement.UnitPaidQuota,
			UserSubscriptionID: int64(subscription.Id),
			CreditOpID:         creditOp,
			ClaimedAt:          now,
		}
		if err := tx.Create(&claimRow).Error; err != nil {
			if isUniqueViolation(err) {
				return ErrUserRedemptionAlreadyClaimed
			}
			return err
		}
		if row.OfficialIssuer {
			setting := operation_setting.GetInvitationIncentiveSetting()
			if setting.Enabled && setting.AdminSubscriptionRedemptionCommissionEnabled {
				rateBasisPoints, _, err := operation_setting.ParseInvitationCommissionRate(setting.AdminSubscriptionRedemptionCommissionRate)
				if err != nil {
					return err
				}
				if _, _, err := model.CreateInvitationCommissionWithTx(tx, model.InvitationCommissionInput{
					InviteeUserID:   input.ClaimerUserID,
					TriggerType:     model.InviteRewardTriggerAdminSubscriptionClaim,
					TriggerID:       fmt.Sprintf("%d", claimRow.ID),
					EligibleQuota:   entitlement.UnitPaidQuota,
					RateBasisPoints: rateBasisPoints,
				}); err != nil {
					return err
				}
			}
		}

		row.ClaimCount++
		row.ClaimedQuota += entitlement.UnitPaidQuota
		updates := map[string]any{
			"claim_count":   row.ClaimCount,
			"claimed_quota": row.ClaimedQuota,
		}
		if row.ClaimCount >= row.MaxClaims {
			row.Status = model.UserRedemptionStatusExhausted
			row.TerminalAt = now
			updates["status"] = row.Status
			updates["terminal_at"] = row.TerminalAt
		}
		if err := tx.Model(&model.UserRedemptionCode{}).Where("id = ?", row.ID).Updates(updates).Error; err != nil {
			return err
		}
		if err := enqueueUserRedemptionBusinessAuditWithTx(tx, fmt.Sprintf("user-redemption:%d:subscription-claim:%d:audit", row.ID, input.ClaimerUserID), "user.redemption.subscription.claim", input.ClaimerUserID, input.ClaimerUserID, row, entitlement.UnitPaidQuota, input.ClaimerUserID, map[string]any{
			"plan_id": entitlement.PlanID, "user_subscription_id": claimRow.UserSubscriptionID,
		}); err != nil {
			return err
		}
		claim = &claimRow
		upgradeGroup = snapshotPlan.UpgradeGroup
		return nil
	})
	if err != nil {
		return nil, err
	}
	if expiredInTxn {
		return nil, ErrUserRedemptionExpired
	}
	_ = invalidateUserRedemptionCache(input.ClaimerUserID)
	if upgradeGroup != "" {
		_ = model.UpdateUserGroupCache(input.ClaimerUserID, upgradeGroup)
	}
	return claim, nil
}
