package service

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/url"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/registration_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"gorm.io/gorm"
)

const (
	invitationCodeAlphabet     = "23456789ABCDEFGHJKMNPQRSTVWXYZ"
	invitationCodeLength       = 10
	legacyInvitationCodeLength = 8
	invitationCodeMaxAttempts  = 8

	inviteApplyUnavailableApplicationsDisabled = "applications_disabled"
	inviteApplyUnavailableApplicationPending   = "application_pending"
	inviteApplyUnavailableReapplyCooldown      = "reapply_cooldown"
	inviteApplyUnavailableActiveCode           = "active_code"
)

var invitationCodeReader io.Reader = rand.Reader

var (
	ErrInviteApplicationPending   = errors.New("invite_application_pending")
	ErrInviteReapplyLater         = errors.New("invite_reapply_later")
	ErrInviteApplicationsDisabled = errors.New("invite_applications_disabled")
	ErrInviteActiveCode           = errors.New("invite_active_code")
	ErrInviteTargetForbidden      = errors.New("invite_target_forbidden")
	ErrInviteApplicationInvalid   = errors.New("invite_application_invalid")
	ErrInviteIssueInvalid         = errors.New("invite_issue_invalid")
	ErrInviteCodeInvalid          = errors.New("invite_code_invalid")
)

type InviteReapplyError struct {
	RetryAt int64
}

func (e *InviteReapplyError) Error() string {
	return ErrInviteReapplyLater.Error()
}

func (e *InviteReapplyError) Unwrap() error {
	return ErrInviteReapplyLater
}

func (e *InviteReapplyError) RetryTimestamp() int64 {
	return e.RetryAt
}

type invitationApplicationPolicy struct {
	enabled         bool
	cooldownSeconds int64
}

func currentInvitationApplicationPolicy() invitationApplicationPolicy {
	settings := registration_setting.GetSettings()
	return invitationApplicationPolicy{
		enabled:         settings.InvitationApplicationsEnabled,
		cooldownSeconds: settings.InvitationReapplyCooldownSeconds,
	}
}

func (policy invitationApplicationPolicy) reapplyAt(reviewedAt, now int64) int64 {
	if reviewedAt <= 0 || policy.cooldownSeconds <= 0 {
		return 0
	}
	reapplyAt := reviewedAt + policy.cooldownSeconds
	if reapplyAt <= now {
		return 0
	}
	return reapplyAt
}

type InviteCodeIssueSpec struct {
	Kind      string
	MaxUses   int
	ExpiresAt int64
}

type InviteCodeReviewRequest struct {
	ReviewerID    int
	ApplicationID int64
	Approve       bool
	Issue         InviteCodeIssueSpec
	ReviewReason  string
}

type InviteCodeDirectIssueRequest struct {
	IssuerID     int
	TargetUserID int
	Reason       string
	Issue        InviteCodeIssueSpec
}
type InvitationUserState struct {
	Application            *InvitationUserApplication `json:"application,omitempty"`
	Code                   *InvitationUserCode        `json:"code,omitempty"`
	Uses                   []InvitationUserUse        `json:"uses"`
	CanApply               bool                       `json:"can_apply"`
	ReapplyAt              int64                      `json:"reapply_at,omitempty"`
	ApplyUnavailableReason string                     `json:"apply_unavailable_reason,omitempty"`
}

type InvitationUserApplication struct {
	ID           int64  `json:"id"`
	Reason       string `json:"reason"`
	Status       string `json:"status"`
	ReviewReason string `json:"review_reason"`
	ReviewedAt   int64  `json:"reviewed_at"`
	CreatedAt    int64  `json:"created_at"`
	UpdatedAt    int64  `json:"updated_at"`
}

type InvitationUserCode struct {
	ID             int64  `json:"id"`
	Code           string `json:"code"`
	Kind           string `json:"kind"`
	MaxUses        int    `json:"max_uses"`
	UsedCount      int    `json:"used_count"`
	Status         string `json:"status"`
	ExpiresAt      int64  `json:"expires_at"`
	CreatedAt      int64  `json:"created_at"`
	UpdatedAt      int64  `json:"updated_at"`
	InvitationLink string `json:"invitation_link,omitempty"`
}

type InvitationUserUse struct {
	ID                  int64  `json:"id"`
	RegisteredUserID    int    `json:"registered_user_id"`
	RegisteredAt        int64  `json:"registered_at"`
	Source              string `json:"source"`
	InviterReward       int    `json:"inviter_reward"`
	InviterRewardStatus string `json:"inviter_reward_status"`
	RewardsCompletedAt  int64  `json:"rewards_completed_at"`
}

func GetInvitationUserState(ctx context.Context, userID int) (InvitationUserState, error) {
	state := InvitationUserState{Uses: make([]InvitationUserUse, 0)}
	if userID <= 0 {
		return state, ErrInviteApplicationInvalid
	}
	policy := currentInvitationApplicationPolicy()

	err := model.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := model.LockInvitationUser(tx, userID); err != nil {
			return ErrInviteApplicationInvalid
		}
		now := common.GetTimestamp()
		if _, err := currentActiveInviteCode(tx, userID, now); err != nil {
			return err
		}

		var application InvitationUserApplication
		err := tx.Model(&model.InviteCodeApplication{}).
			Select("id, reason, status, review_reason, reviewed_at, created_at, updated_at").
			Where("user_id = ?", userID).Order("id desc").First(&application).Error
		if err == nil {
			state.Application = &application
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		var code InvitationUserCode
		err = tx.Model(&model.InviteCode{}).
			Select("id, code, kind, max_uses, used_count, status, expires_at, created_at, updated_at").
			Where("owner_user_id = ?", userID).Order("created_at desc, id desc").First(&code).Error
		if err == nil {
			code.InvitationLink = buildInvitationLink(code.Code)
			state.Code = &code
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		if err := tx.Model(&model.InviteCodeUse{}).
			Select("id, registered_user_id, registered_at, source, inviter_reward, inviter_reward_status, rewards_completed_at").
			Where("inviter_user_id = ?", userID).
			Order("registered_at desc, id desc").Find(&state.Uses).Error; err != nil {
			return err
		}

		state.CanApply = true
		switch {
		case state.Code != nil && state.Code.Status == model.InviteCodeStatusActive:
			state.CanApply = false
			state.ApplyUnavailableReason = inviteApplyUnavailableActiveCode
		case state.Application != nil && state.Application.Status == model.InviteApplicationStatusPending:
			state.CanApply = false
			state.ApplyUnavailableReason = inviteApplyUnavailableApplicationPending
		case state.Application != nil && state.Application.Status == model.InviteApplicationStatusRejected && policy.reapplyAt(state.Application.ReviewedAt, now) > 0:
			state.CanApply = false
			state.ReapplyAt = policy.reapplyAt(state.Application.ReviewedAt, now)
			state.ApplyUnavailableReason = inviteApplyUnavailableReapplyCooldown
		case !policy.enabled:
			state.CanApply = false
			state.ApplyUnavailableReason = inviteApplyUnavailableApplicationsDisabled
		}
		return nil
	})
	return state, err
}

func ApplyForInviteCode(ctx context.Context, userID int, reason string) (*model.InviteCodeApplication, error) {
	reason = strings.TrimSpace(reason)
	if userID <= 0 || len(reason) < 5 || len(reason) > 500 {
		return nil, ErrInviteApplicationInvalid
	}
	policy := currentInvitationApplicationPolicy()
	if !policy.enabled {
		return nil, ErrInviteApplicationsDisabled
	}
	var application model.InviteCodeApplication
	err := model.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := model.LockInvitationUser(tx, userID); err != nil {
			return err
		}
		active, err := currentActiveInviteCode(tx, userID, common.GetTimestamp())
		if err != nil {
			return err
		}
		if active != nil {
			return ErrInviteActiveCode
		}
		var latestApplication model.InviteCodeApplication
		err = tx.Where("user_id = ?", userID).
			Order("id desc").First(&latestApplication).Error
		if err == nil && latestApplication.Status == model.InviteApplicationStatusRejected {
			reapplyAt := policy.reapplyAt(latestApplication.ReviewedAt, common.GetTimestamp())
			if reapplyAt > 0 {
				return &InviteReapplyError{RetryAt: reapplyAt}
			}
		}
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		application = model.InviteCodeApplication{
			UserID: userID, Reason: reason, PendingUserSlot: &userID, Status: model.InviteApplicationStatusPending,
		}
		if err := tx.Create(&application).Error; err != nil {
			if isInvitationUniqueConflict(err) {
				return ErrInviteApplicationPending
			}
			return err
		}
		return EnqueueWithTx(tx, AuditEvent{
			EventID: fmt.Sprintf("invite:application:%d:created", application.ID), Type: "invitation.application_created",
			Result: "success", ActorID: userID, Target: fmt.Sprintf("invite-application:%d", application.ID),
			Params: map[string]any{"application_id": application.ID, "target_user_id": userID, "reason": reason},
		})
	})
	if err != nil {
		return nil, err
	}
	return &application, nil
}

func WithdrawInviteCodeApplication(ctx context.Context, userID int, applicationID int64) error {
	if userID <= 0 || applicationID <= 0 {
		return ErrInviteApplicationInvalid
	}
	return model.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		application, err := model.LockInviteCodeApplication(tx, applicationID)
		if err != nil {
			return ErrInviteApplicationInvalid
		}
		if application.UserID != userID || application.Status != model.InviteApplicationStatusPending {
			return ErrInviteApplicationInvalid
		}
		result := tx.Model(&model.InviteCodeApplication{}).
			Where("id = ? AND user_id = ? AND status = ?", applicationID, userID, model.InviteApplicationStatusPending).
			Updates(map[string]any{"status": model.InviteApplicationStatusWithdrawn, "pending_user_slot": nil})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrInviteApplicationInvalid
		}
		return EnqueueWithTx(tx, AuditEvent{
			EventID: fmt.Sprintf("invite:application:%d:withdrawn", applicationID), Type: "invitation.application_withdrawn",
			Result: "success", ActorID: userID, Target: fmt.Sprintf("invite-application:%d", applicationID),
			Params: map[string]any{"application_id": applicationID, "target_user_id": userID},
		})
	})
}

func ReviewInviteCodeApplication(ctx context.Context, req InviteCodeReviewRequest) (*model.InviteCode, error) {
	if req.ReviewerID <= 0 || req.ApplicationID <= 0 {
		return nil, ErrInviteApplicationInvalid
	}
	if !req.Approve && strings.TrimSpace(req.ReviewReason) == "" {
		return nil, ErrInviteApplicationInvalid
	}
	if req.Approve {
		if err := validateInviteIssueSpec(req.Issue, common.GetTimestamp()); err != nil {
			return nil, err
		}
	}
	var issued *model.InviteCode
	err := model.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		reviewer, err := model.LockInvitationUser(tx, req.ReviewerID)
		if err != nil {
			return ErrInviteTargetForbidden
		}
		application, err := model.LockInviteCodeApplication(tx, req.ApplicationID)
		if err != nil || application.Status != model.InviteApplicationStatusPending {
			return ErrInviteApplicationInvalid
		}
		target, err := model.LockInvitationUser(tx, application.UserID)
		if err != nil {
			return ErrInviteTargetForbidden
		}
		if !canIssueInviteCode(reviewer, target) {
			return ErrInviteTargetForbidden
		}

		now := common.GetTimestamp()
		status := model.InviteApplicationStatusRejected
		if req.Approve {
			active, err := currentActiveInviteCode(tx, target.Id, now)
			if err != nil {
				return err
			}
			if active != nil {
				return ErrInviteActiveCode
			}
			applicationID := application.ID
			invite := &model.InviteCode{
				OwnerUserID: target.Id, ActiveOwnerSlot: &target.Id, Kind: req.Issue.Kind,
				MaxUses: req.Issue.MaxUses, Status: model.InviteCodeStatusActive, ExpiresAt: req.Issue.ExpiresAt,
				IssuedBy: reviewer.Id, ApplicationID: &applicationID,
			}
			if err := insertInviteCodeWithRetry(tx, invite); err != nil {
				if isInvitationUniqueConflict(err) {
					return ErrInviteActiveCode
				}
				return err
			}
			issued = invite
			status = model.InviteApplicationStatusApproved
		}

		result := tx.Model(&model.InviteCodeApplication{}).
			Where("id = ? AND status = ?", application.ID, model.InviteApplicationStatusPending).
			Updates(map[string]any{
				"status": status, "pending_user_slot": nil, "reviewer_id": reviewer.Id,
				"review_reason": strings.TrimSpace(req.ReviewReason), "reviewed_at": now,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrInviteApplicationInvalid
		}
		params := map[string]any{
			"application_id": application.ID, "target_user_id": target.Id, "reviewer_id": reviewer.Id,
			"review_reason": strings.TrimSpace(req.ReviewReason), "status": status,
		}
		if issued != nil {
			params["invite_code_id"] = issued.ID
			params["kind"] = issued.Kind
			params["max_uses"] = issued.MaxUses
			params["expires_at"] = issued.ExpiresAt
		}
		return EnqueueWithTx(tx, AuditEvent{
			EventID: fmt.Sprintf("invite:application:%d:reviewed", application.ID), Type: "invitation.application_reviewed",
			Result: "success", ActorID: reviewer.Id, Target: fmt.Sprintf("user:%d", target.Id), Params: params,
		})
	})
	if err != nil {
		return nil, err
	}
	return issued, nil
}

func DirectIssueInviteCode(ctx context.Context, req InviteCodeDirectIssueRequest) (*model.InviteCode, error) {
	req.Reason = strings.TrimSpace(req.Reason)
	if req.IssuerID <= 0 || req.TargetUserID <= 0 || req.Reason == "" || len(req.Reason) > 500 {
		return nil, ErrInviteIssueInvalid
	}
	if err := validateInviteIssueSpec(req.Issue, common.GetTimestamp()); err != nil {
		return nil, err
	}
	var issued model.InviteCode
	err := model.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		issuer, err := model.LockInvitationUser(tx, req.IssuerID)
		if err != nil {
			return ErrInviteTargetForbidden
		}
		target := issuer
		if req.TargetUserID != req.IssuerID {
			target, err = model.LockInvitationUser(tx, req.TargetUserID)
			if err != nil {
				return ErrInviteTargetForbidden
			}
		}
		if !canIssueInviteCode(issuer, target) {
			return ErrInviteTargetForbidden
		}
		active, err := currentActiveInviteCode(tx, target.Id, common.GetTimestamp())
		if err != nil {
			return err
		}
		if active != nil {
			return ErrInviteActiveCode
		}
		issued = model.InviteCode{
			OwnerUserID: target.Id, ActiveOwnerSlot: &target.Id, Kind: req.Issue.Kind,
			MaxUses: req.Issue.MaxUses, Status: model.InviteCodeStatusActive, ExpiresAt: req.Issue.ExpiresAt,
			IssuedBy: issuer.Id,
		}
		if err := insertInviteCodeWithRetry(tx, &issued); err != nil {
			if isInvitationUniqueConflict(err) {
				return ErrInviteActiveCode
			}
			return err
		}
		return EnqueueWithTx(tx, AuditEvent{
			EventID: fmt.Sprintf("invite:code:%d:issued", issued.ID), Type: "invitation.code_issued",
			Result: "success", ActorID: issuer.Id, Target: fmt.Sprintf("user:%d", target.Id),
			Params: map[string]any{
				"invite_code_id": issued.ID, "target_user_id": target.Id, "issuer_id": issuer.Id,
				"kind": issued.Kind, "max_uses": issued.MaxUses, "expires_at": issued.ExpiresAt, "reason": req.Reason,
			},
		})
	})
	if err != nil {
		return nil, err
	}
	return &issued, nil
}

func RevokeInviteCode(ctx context.Context, actorID int, inviteCodeID int64, reason string) error {
	reason = strings.TrimSpace(reason)
	if actorID <= 0 || inviteCodeID <= 0 || reason == "" || len(reason) > 500 {
		return ErrInviteCodeInvalid
	}
	return model.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		actor, err := model.LockInvitationUser(tx, actorID)
		if err != nil {
			return ErrInviteTargetForbidden
		}
		invite, err := model.LockInviteCodeByID(tx, inviteCodeID)
		if err != nil || invite.Status != model.InviteCodeStatusActive {
			return ErrInviteCodeInvalid
		}
		if invite.OwnerUserID == actor.Id {
			if actor.Status != common.UserStatusEnabled {
				return ErrInviteTargetForbidden
			}
		} else {
			target, lockErr := model.LockInvitationUser(tx, invite.OwnerUserID)
			if lockErr != nil || !canIssueInviteCode(actor, target) {
				return ErrInviteTargetForbidden
			}
		}
		now := common.GetTimestamp()
		result := tx.Model(&model.InviteCode{}).
			Where("id = ? AND status = ?", invite.ID, model.InviteCodeStatusActive).
			Updates(map[string]any{"status": model.InviteCodeStatusRevoked, "active_owner_slot": nil, "revoked_at": now})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrInviteCodeInvalid
		}
		return EnqueueWithTx(tx, AuditEvent{
			EventID: fmt.Sprintf("invite:code:%d:revoked", invite.ID), Type: "invitation.code_revoked",
			Result: "success", ActorID: actor.Id, Target: fmt.Sprintf("invite-code:%d", invite.ID),
			Params: map[string]any{"invite_code_id": invite.ID, "target_user_id": invite.OwnerUserID, "reason": reason},
		})
	})
}

func currentActiveInviteCode(tx *gorm.DB, ownerUserID int, now int64) (*model.InviteCode, error) {
	var invite model.InviteCode
	err := tx.Where("owner_user_id = ? AND status = ?", ownerUserID, model.InviteCodeStatusActive).
		Order("id desc").First(&invite).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if invite.ExpiresAt > 0 && invite.ExpiresAt <= now {
		if err := tx.Model(&model.InviteCode{}).Where("id = ? AND status = ?", invite.ID, model.InviteCodeStatusActive).
			Updates(map[string]any{"status": model.InviteCodeStatusExpired, "active_owner_slot": nil}).Error; err != nil {
			return nil, err
		}
		return nil, nil
	}
	if invite.UsedCount >= invite.MaxUses {
		if err := tx.Model(&model.InviteCode{}).Where("id = ? AND status = ?", invite.ID, model.InviteCodeStatusActive).
			Updates(map[string]any{"status": model.InviteCodeStatusExhausted, "active_owner_slot": nil}).Error; err != nil {
			return nil, err
		}
		return nil, nil
	}
	return &invite, nil
}

func validateInviteIssueSpec(spec InviteCodeIssueSpec, now int64) error {
	if spec.ExpiresAt <= now || spec.ExpiresAt > now+365*24*60*60 {
		return ErrInviteIssueInvalid
	}
	switch spec.Kind {
	case model.InviteCodeKindSingle:
		if spec.MaxUses != 1 {
			return ErrInviteIssueInvalid
		}
	case model.InviteCodeKindMulti:
		if spec.MaxUses < 2 || spec.MaxUses > 100 {
			return ErrInviteIssueInvalid
		}
	default:
		return ErrInviteIssueInvalid
	}
	return nil
}

func canIssueInviteCode(actor model.User, target model.User) bool {
	if actor.Status != common.UserStatusEnabled || actor.Role < common.RoleAdminUser {
		return false
	}
	if actor.Id == 1 {
		return true
	}
	if actor.Id == target.Id {
		return false
	}
	if actor.Role >= common.RoleRootUser {
		return target.Role < common.RoleRootUser
	}
	return target.Role < common.RoleAdminUser
}

func generateInvitationCode() (string, error) {
	var code strings.Builder
	code.Grow(invitationCodeLength)
	limit := big.NewInt(int64(len(invitationCodeAlphabet)))
	for index := 0; index < invitationCodeLength; index++ {
		position, err := rand.Int(invitationCodeReader, limit)
		if err != nil {
			return "", err
		}
		code.WriteByte(invitationCodeAlphabet[position.Int64()])
	}
	return code.String(), nil
}

func buildInvitationLink(code string) string {
	base, err := url.Parse(strings.TrimSpace(system_setting.ServerAddress))
	if err != nil || base.Scheme == "" || base.Host == "" {
		return ""
	}
	query := base.Query()
	query.Set("aff", code)
	base.RawQuery = query.Encode()
	return base.String()
}

func insertInviteCodeWithRetry(tx *gorm.DB, invite *model.InviteCode) error {
	var lastErr error
	for attempt := 0; attempt < invitationCodeMaxAttempts; attempt++ {
		code, err := generateInvitationCode()
		if err != nil {
			return err
		}
		invite.ID = 0
		invite.Code = code
		savepoint := fmt.Sprintf("invite_code_%d", attempt)
		if err := tx.SavePoint(savepoint).Error; err != nil {
			return err
		}
		if err := tx.Create(invite).Error; err == nil {
			return nil
		} else if !isInvitationUniqueConflict(err) {
			return err
		} else {
			lastErr = err
			if rollbackErr := tx.RollbackTo(savepoint).Error; rollbackErr != nil {
				return rollbackErr
			}
		}
	}
	return lastErr
}

func isInvitationUniqueConflict(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "unique") || strings.Contains(message, "duplicate") || strings.Contains(message, "1062")
}
