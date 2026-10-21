package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/registration_setting"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	ErrInvitationRequired  = errors.New("invitation_required")
	ErrInvitationInvalid   = errors.New("invitation_invalid")
	ErrInvitationExhausted = errors.New("invitation_exhausted")
)

const (
	InvitationValidationAvailable   = "invitation_available"
	InvitationValidationUnavailable = "invitation_unavailable"
)

type InvitationCodeValidation struct {
	Valid bool   `json:"valid"`
	Code  string `json:"code"`
}

type RegistrationRequest struct {
	User                   *model.User
	InvitationCode         string
	Source                 string
	RegistrationIP         string
	ActorID                int
	RequestID              string
	AllowMissingInvitation bool
	AfterInsert            func(tx *gorm.DB, user *model.User) error
}

type RegistrationResult struct {
	UserID       int
	InviteUseID  int64
	InviterID    int
	InviteCodeID int64
}

func ValidateInvitationCode(ctx context.Context, code string, allowMissing bool) error {
	code = strings.TrimSpace(code)
	if code == "" {
		if registration_setting.GetSettings().AffRequired && !allowMissing {
			return ErrInvitationRequired
		}
		return nil
	}
	if len(code) > 64 {
		return ErrInvitationInvalid
	}
	var invite model.InviteCode
	var err error
	for _, candidate := range invitationCodeCandidates(code) {
		err = model.DB.WithContext(ctx).Where("code = ?", candidate).First(&invite).Error
		if err == nil {
			break
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrInvitationInvalid
	}
	if invite.Status == model.InviteCodeStatusExhausted || invite.UsedCount >= invite.MaxUses {
		return ErrInvitationExhausted
	}
	if invite.Status != model.InviteCodeStatusActive || invite.ExpiresAt <= common.GetTimestamp() {
		return ErrInvitationInvalid
	}
	return nil
}

func ValidateInvitationCodePublic(ctx context.Context, code string) (InvitationCodeValidation, error) {
	result := InvitationCodeValidation{Code: InvitationValidationUnavailable}
	if strings.TrimSpace(code) == "" {
		return result, nil
	}
	err := ValidateInvitationCode(ctx, code, false)
	if err == nil {
		result.Valid = true
		result.Code = InvitationValidationAvailable
		return result, nil
	}
	if errors.Is(err, ErrInvitationRequired) || errors.Is(err, ErrInvitationInvalid) || errors.Is(err, ErrInvitationExhausted) {
		return result, nil
	}
	return result, err
}

func RegisterWithInvitation(ctx context.Context, req RegistrationRequest) (RegistrationResult, error) {
	if req.User == nil {
		return RegistrationResult{}, ErrInvitationInvalid
	}
	code := strings.TrimSpace(req.InvitationCode)
	if code == "" && registration_setting.GetSettings().AffRequired && !req.AllowMissingInvitation {
		return RegistrationResult{}, ErrInvitationRequired
	}
	if len(code) > 64 || len(strings.TrimSpace(req.Source)) > 64 {
		return RegistrationResult{}, ErrInvitationInvalid
	}
	baseUser := *req.User
	var result RegistrationResult
	var registeredUser model.User
	var lastErr error
	for attempt := 0; attempt < 5; attempt++ {
		registeredUser = baseUser
		result = RegistrationResult{}
		lastErr = registerWithInvitationOnce(ctx, &registeredUser, code, req, &result)
		if !isSQLiteBusyError(lastErr) {
			break
		}
		select {
		case <-ctx.Done():
			return RegistrationResult{}, ctx.Err()
		case <-time.After(time.Duration(attempt+1) * 10 * time.Millisecond):
		}
	}
	if lastErr != nil {
		return RegistrationResult{}, lastErr
	}
	*req.User = registeredUser
	req.User.FinishInsert(0)
	return result, nil
}

func registerWithInvitationOnce(ctx context.Context, user *model.User, code string, req RegistrationRequest, result *RegistrationResult) error {
	return model.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		user.RegistrationIP = model.NormalizeAuditIP(req.RegistrationIP)
		if code == "" {
			user.InviterId = 0
			if err := user.InsertWithTx(tx, 0); err != nil {
				return err
			}
			if req.AfterInsert != nil {
				if err := req.AfterInsert(tx, user); err != nil {
					return err
				}
			}
			if err := EnqueueRegistrationAudit(tx, user, req); err != nil {
				return err
			}
			result.UserID = user.Id
			return nil
		}

		var invite model.InviteCode
		var err error
		for _, candidate := range invitationCodeCandidates(code) {
			invite, err = model.LockInviteCodeByCode(tx, candidate)
			if err == nil {
				break
			}
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrInvitationInvalid
		}
		now := common.GetTimestamp()
		if invite.Status == model.InviteCodeStatusExhausted || invite.UsedCount >= invite.MaxUses {
			return ErrInvitationExhausted
		}
		if invite.Status != model.InviteCodeStatusActive || invite.ExpiresAt <= now {
			return ErrInvitationInvalid
		}

		user.InviterId = invite.OwnerUserID
		user.InviterSource = model.InviterSourceInviteCode
		if err := user.InsertWithTx(tx, invite.OwnerUserID); err != nil {
			return err
		}
		if req.AfterInsert != nil {
			if err := req.AfterInsert(tx, user); err != nil {
				return err
			}
		}
		inviterReward := 0
		inviteeReward := 0
		incentiveSetting := *operation_setting.GetInvitationIncentiveSetting()
		if operation_setting.IsPaymentComplianceConfirmed() && incentiveSetting.Enabled {
			if incentiveSetting.InviterRegistrationRewardEnabled {
				inviterReward = common.QuotaForInviter
			}
			if incentiveSetting.InviteeRegistrationGiftEnabled {
				inviteeReward = common.QuotaForInvitee
			}
		}
		inviterStatus := model.InviteRewardStatusAvailable
		if inviterReward <= 0 {
			inviterStatus = model.InviteRewardStatusSkipped
		}
		inviteeStatus := model.InviteRewardStatusAvailable
		if inviteeReward <= 0 {
			inviteeStatus = model.InviteRewardStatusSkipped
		}
		use := model.InviteCodeUse{
			InviteCodeID: invite.ID, InviterUserID: invite.OwnerUserID, RegisteredUserID: user.Id,
			RegisteredAt: now, Source: strings.TrimSpace(req.Source), RegistrationIP: model.NormalizeAuditIP(req.RegistrationIP),
			InviterReward: inviterReward, InviteeReward: inviteeReward,
			InviterRewardStatus: inviterStatus, InviteeRewardStatus: inviteeStatus,
			AuditEventID: "invite-use:" + uuid.NewString(),
		}
		if err := tx.Create(&use).Error; err != nil {
			return err
		}
		if err := createInvitationRewardPayoutsWithTx(tx, use); err != nil {
			return err
		}

		newUsedCount := invite.UsedCount + 1
		updates := map[string]any{"used_count": newUsedCount}
		if newUsedCount >= invite.MaxUses {
			updates["status"] = model.InviteCodeStatusExhausted
			updates["active_owner_slot"] = nil
		}
		updateResult := tx.Model(&model.InviteCode{}).
			Where("id = ? AND status = ? AND used_count = ? AND expires_at > ?", invite.ID, model.InviteCodeStatusActive, invite.UsedCount, now).
			Updates(updates)
		if updateResult.Error != nil {
			return updateResult.Error
		}
		if updateResult.RowsAffected != 1 {
			return ErrInvitationExhausted
		}
		if err := EnqueueRegistrationAudit(tx, user, req); err != nil {
			return err
		}
		if err := EnqueueWithTx(tx, AuditEvent{
			EventID: fmt.Sprintf("invite:use:%d", use.ID), Type: "invitation.code_used", Result: "success",
			ActorID: user.Id, Target: fmt.Sprintf("invite-code:%d", invite.ID), IP: use.RegistrationIP,
			Params: map[string]any{
				"invite_code_id": invite.ID, "invite_use_id": use.ID, "inviter_id": invite.OwnerUserID,
				"registered_user_id": user.Id, "source": use.Source,
			},
		}); err != nil {
			return err
		}
		result.UserID = user.Id
		result.InviteUseID = use.ID
		result.InviterID = invite.OwnerUserID
		result.InviteCodeID = invite.ID
		return nil
	})
}

func invitationCodeCandidates(code string) []string {
	trimmed := strings.TrimSpace(code)
	candidates := []string{trimmed}
	if len(trimmed) == invitationCodeLength || len(trimmed) == legacyInvitationCodeLength {
		normalized := strings.ToUpper(trimmed)
		if normalized != trimmed {
			candidates = append(candidates, normalized)
		}
	}
	return candidates
}

func EnqueueRegistrationAudit(tx *gorm.DB, user *model.User, req RegistrationRequest) error {
	eventID := fmt.Sprintf("user:register:%d", user.Id)
	if err := EnqueueWithTx(tx, AuditEvent{
		EventID:   eventID,
		Type:      "user.register",
		Result:    "success",
		ActorID:   req.ActorID,
		Target:    fmt.Sprintf("user:%d", user.Id),
		IP:        user.RegistrationIP,
		RequestID: strings.TrimSpace(req.RequestID),
		Params: map[string]any{
			"target_user_id": user.Id,
			"source":         strings.TrimSpace(req.Source),
		},
	}); err != nil {
		return err
	}
	if user.RegistrationIP == "" {
		return nil
	}
	return model.RecordIPSecurityEventWithTx(tx, &model.IPSecurityEvent{
		EventID: eventID, UserID: user.Id, Username: user.Username, CreatedAt: user.CreatedAt,
		IP: user.RegistrationIP, Kind: model.IPSecurityKindRegistration, RequestID: strings.TrimSpace(req.RequestID),
	})
}

func ProcessInvitationUseRewards(ctx context.Context, inviteUseID int64) error {
	if inviteUseID <= 0 {
		return ErrInvitationInvalid
	}
	return model.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		use, err := model.LockInviteCodeUse(tx, inviteUseID)
		if err != nil {
			return err
		}
		return createInvitationRewardPayoutsWithTx(tx, use)
	})
}

func isSQLiteBusyError(err error) bool {
	if err == nil || !common.UsingMainDatabase(common.DatabaseTypeSQLite) {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "database is locked") || strings.Contains(message, "database table is locked") ||
		strings.Contains(message, "sqlite_busy") || strings.Contains(message, "busy (5)") || strings.Contains(message, "deadlocked (6)")
}
