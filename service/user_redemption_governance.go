package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

type UserRedemptionAdminAction string

const (
	UserRedemptionActionDisable   UserRedemptionAdminAction = "disable"
	UserRedemptionActionEnable    UserRedemptionAdminAction = "enable"
	UserRedemptionActionTerminate UserRedemptionAdminAction = "terminate"
	UserRedemptionActionDelete    UserRedemptionAdminAction = "delete"
)

type UserRedemptionSettlement string

const (
	UserRedemptionSettlementRefund  UserRedemptionSettlement = "refund"
	UserRedemptionSettlementForfeit UserRedemptionSettlement = "forfeit"
)

type UserRedemptionAdminCommand struct {
	Actor      UserIdentity
	CodeID     int64
	Action     UserRedemptionAdminAction
	Settlement UserRedemptionSettlement
	Reason     string
}

type UserRedemptionBatchDispositionRequest struct {
	IDs         []int64                  `json:"ids" binding:"required,min=1,max=100"`
	Disposition UserRedemptionSettlement `json:"disposition" binding:"required,oneof=refund forfeit"`
	Reason      string                   `json:"reason" binding:"required,max=500"`
}

type UserRedemptionBatchDispositionResult struct {
	ID      int64  `json:"id"`
	Success bool   `json:"success"`
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

func AdminBatchSettleUserRedemptionCodes(ctx context.Context, actor UserIdentity, request UserRedemptionBatchDispositionRequest) ([]UserRedemptionBatchDispositionResult, error) {
	reason := strings.TrimSpace(request.Reason)
	if model.DB == nil || actor.ID <= 0 || actor.Role < common.RoleAdminUser || len(request.IDs) == 0 || len(request.IDs) > 100 || reason == "" || len(reason) > 500 {
		return nil, ErrUserRedemptionInvalid
	}
	if request.Disposition != UserRedemptionSettlementRefund && request.Disposition != UserRedemptionSettlementForfeit {
		return nil, ErrUserRedemptionInvalid
	}

	results := make([]UserRedemptionBatchDispositionResult, 0, len(request.IDs))
	for _, codeID := range request.IDs {
		result := UserRedemptionBatchDispositionResult{ID: codeID}
		if codeID <= 0 {
			result.Code = "invalid"
			result.Message = "Invalid redemption code ID"
			results = append(results, result)
			continue
		}
		var rewardType string
		if err := model.DB.WithContext(ctx).Model(&model.UserRedemptionCode{}).Where("id = ?", codeID).Pluck("reward_type", &rewardType).Error; err == nil && normalizeUserRedemptionRewardType(rewardType) == model.UserRedemptionRewardSubscription {
			result.Code = "invalid"
			result.Message = "Subscription redemption codes are settled individually"
			results = append(results, result)
			continue
		}

		_, err := AdminMutateUserRedemptionCode(ctx, UserRedemptionAdminCommand{
			Actor:      actor,
			CodeID:     codeID,
			Action:     UserRedemptionActionTerminate,
			Settlement: request.Disposition,
			Reason:     reason,
		})
		if err == nil {
			result.Success = true
			results = append(results, result)
			continue
		}

		switch {
		case errors.Is(err, ErrUserRedemptionNotFound):
			result.Code = "not_found"
			result.Message = "Redemption code was not found"
		case errors.Is(err, ErrUserRedemptionInactive), errors.Is(err, ErrUserRedemptionExpired), errors.Is(err, ErrUserRedemptionExhausted):
			result.Code = "inactive"
			result.Message = "Redemption code is not eligible for settlement"
		case errors.Is(err, ErrUserRedemptionConflict):
			result.Code = "conflict"
			result.Message = "Redemption code has already been settled differently"
		case errors.Is(err, ErrUserRedemptionInvalid), errors.Is(err, ErrUserRedemptionForbidden):
			result.Code = "invalid"
			result.Message = "Settlement request is invalid"
		default:
			result.Code = "failed"
			result.Message = "Unable to settle this redemption code"
		}
		results = append(results, result)
	}
	return results, nil
}

func AdminMutateUserRedemptionCode(ctx context.Context, command UserRedemptionAdminCommand) (*model.UserRedemptionCode, error) {
	reason := strings.TrimSpace(command.Reason)
	if model.DB == nil || command.Actor.ID <= 0 || command.Actor.Role < common.RoleAdminUser || command.CodeID <= 0 || reason == "" || len(reason) > 500 {
		return nil, ErrUserRedemptionInvalid
	}
	var route model.UserRedemptionCode
	if err := model.DB.WithContext(ctx).Select("creator_user_id", "reward_type").First(&route, command.CodeID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserRedemptionNotFound
		}
		return nil, err
	}
	isSubscription := normalizeUserRedemptionRewardType(route.RewardType) == model.UserRedemptionRewardSubscription

	var output *model.UserRedemptionCode
	err := model.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if isSubscription {
			if err := model.LockUserRedemptionCreator(tx, route.CreatorUserID); err != nil {
				return err
			}
		}
		row, err := model.LockUserRedemptionCodeByID(tx, command.CodeID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrUserRedemptionNotFound
			}
			return err
		}
		if isSubscription && (row.CreatorUserID != route.CreatorUserID || normalizeUserRedemptionRewardType(row.RewardType) != model.UserRedemptionRewardSubscription) {
			return ErrUserRedemptionConflict
		}

		now := common.GetTimestamp()
		affectedQuota, err := userRedemptionRefundableQuotaWithTx(tx, row)
		if err != nil {
			return err
		}
		switch command.Action {
		case UserRedemptionActionDisable:
			if row.Status == model.UserRedemptionStatusDisabled {
				output = &row
				return nil
			}
			if row.Status != model.UserRedemptionStatusActive || row.DeletedAt != 0 {
				return ErrUserRedemptionInactive
			}
			row.Status = model.UserRedemptionStatusDisabled
			row.DisabledAt = now
			row.DisabledBy = command.Actor.ID
			row.DisabledReason = reason
		case UserRedemptionActionEnable:
			if row.Status == model.UserRedemptionStatusActive {
				output = &row
				return nil
			}
			if row.Status != model.UserRedemptionStatusDisabled || row.DeletedAt != 0 {
				return ErrUserRedemptionInactive
			}
			if isUserRedemptionExpired(row.ExpiresAt, now) {
				return ErrUserRedemptionExpired
			}
			row.Status = model.UserRedemptionStatusActive
			row.DisabledAt = 0
			row.DisabledBy = 0
			row.DisabledReason = ""
		case UserRedemptionActionTerminate:
			if row.Status == model.UserRedemptionStatusRefunded {
				if command.Settlement == UserRedemptionSettlementRefund {
					output = &row
					return nil
				}
				return ErrUserRedemptionConflict
			}
			if row.Status == model.UserRedemptionStatusForfeited {
				if command.Settlement == UserRedemptionSettlementForfeit {
					output = &row
					return nil
				}
				return ErrUserRedemptionConflict
			}
			if (row.Status != model.UserRedemptionStatusActive && row.Status != model.UserRedemptionStatusDisabled && row.Status != model.UserRedemptionStatusExpiredPending && row.Status != model.UserRedemptionStatusRefundPending) || row.DeletedAt != 0 {
				return ErrUserRedemptionInactive
			}
			if err := settleAdminUserRedemptionWithTx(tx, &row, command.Settlement, command.Actor.ID, reason, now); err != nil {
				return err
			}
		case UserRedemptionActionDelete:
			if row.DeletedAt != 0 {
				output = &row
				return nil
			}
			if row.Status == model.UserRedemptionStatusActive || row.Status == model.UserRedemptionStatusDisabled || row.Status == model.UserRedemptionStatusExpiredPending || row.Status == model.UserRedemptionStatusRefundPending {
				if err := settleAdminUserRedemptionWithTx(tx, &row, command.Settlement, command.Actor.ID, reason, now); err != nil {
					return err
				}
			} else if command.Settlement != "" {
				return ErrUserRedemptionConflict
			}
			row.Status = model.UserRedemptionStatusDeleted
			row.DeletedAt = now
			row.DeletedBy = command.Actor.ID
			row.DeleteReason = reason
		default:
			return ErrUserRedemptionInvalid
		}

		if err := tx.Save(&row).Error; err != nil {
			return err
		}
		if err := enqueueAdminUserRedemptionAuditWithTx(tx, row, command, affectedQuota); err != nil {
			return err
		}
		output = &row
		return nil
	})
	if err != nil {
		return nil, err
	}
	if output != nil {
		_ = invalidateUserRedemptionCache(output.CreatorUserID)
	}
	return output, nil
}

func settleAdminUserRedemptionWithTx(tx *gorm.DB, row *model.UserRedemptionCode, settlement UserRedemptionSettlement, actorID int, reason string, now int64) error {
	switch settlement {
	case UserRedemptionSettlementRefund:
		if err := refundUserRedemptionWithTx(tx, row, now); err != nil {
			return err
		}
	case UserRedemptionSettlementForfeit:
		forfeitable, err := userRedemptionRefundableQuotaWithTx(tx, *row)
		if err != nil {
			return err
		}
		row.ForfeitedQuota += forfeitable
		row.Status = model.UserRedemptionStatusForfeited
		row.TerminalAt = now
		row.ForfeitOpID = fmt.Sprintf("user-redemption:%d:forfeit", row.ID)
	default:
		return ErrUserRedemptionInvalid
	}
	row.TerminalActorID = actorID
	row.TerminalReason = reason
	return nil
}

func enqueueAdminUserRedemptionAuditWithTx(tx *gorm.DB, row model.UserRedemptionCode, command UserRedemptionAdminCommand, affectedQuota int64) error {
	params := map[string]any{
		"id":                   row.ID,
		"action":               string(command.Action),
		"status":               row.Status,
		"target_user_id":       row.CreatorUserID,
		"requested_by_user_id": command.Actor.ID,
		"reason":               strings.TrimSpace(command.Reason),
	}
	if affectedQuota > 0 {
		params["quota"] = affectedQuota
	}
	if command.Settlement != "" {
		params["settlement"] = string(command.Settlement)
	}

	eventTypes := make([]string, 0, 2)
	switch command.Action {
	case UserRedemptionActionDisable:
		eventTypes = append(eventTypes, "user.redemption.disable")
	case UserRedemptionActionEnable:
		eventTypes = append(eventTypes, "user.redemption.enable")
	case UserRedemptionActionTerminate:
		if command.Settlement == UserRedemptionSettlementRefund {
			eventTypes = append(eventTypes, "user.redemption.terminate_refund")
		} else {
			eventTypes = append(eventTypes, "user.redemption.terminate_forfeit")
		}
	case UserRedemptionActionDelete:
		if command.Settlement == UserRedemptionSettlementRefund {
			eventTypes = append(eventTypes, "user.redemption.terminate_refund")
		} else if command.Settlement == UserRedemptionSettlementForfeit {
			eventTypes = append(eventTypes, "user.redemption.terminate_forfeit")
		}
		eventTypes = append(eventTypes, "user.redemption.delete")
	default:
		return ErrUserRedemptionInvalid
	}

	for _, eventType := range eventTypes {
		if err := EnqueueWithTx(tx, AuditEvent{
			Type:      eventType,
			ActorID:   command.Actor.ID,
			LogUserID: row.CreatorUserID,
			Target:    fmt.Sprintf("user_redemption_code:%d", row.ID),
			Params:    params,
		}); err != nil {
			return err
		}
	}
	return nil
}
