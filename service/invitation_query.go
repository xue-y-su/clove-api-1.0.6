package service

import (
	"context"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

type InvitationAdminListQuery struct {
	ActorID  int
	Page     int
	PageSize int
	Status   string
	Keyword  string
}

type InvitationApplicationListItem struct {
	model.InviteCodeApplication
	ApplicantUsername string `json:"applicant_username" gorm:"column:applicant_username"`
	ApplicantRole     int    `json:"applicant_role" gorm:"column:applicant_role"`
	ReviewerUsername  string `json:"reviewer_username" gorm:"column:reviewer_username"`
}

type InvitationApplicationListResult struct {
	Items    []InvitationApplicationListItem `json:"items"`
	Total    int64                           `json:"total"`
	Page     int                             `json:"page"`
	PageSize int                             `json:"page_size"`
}

type InvitationCodeListItem struct {
	model.InviteCode
	OwnerUsername  string `json:"owner_username" gorm:"column:owner_username"`
	OwnerRole      int    `json:"owner_role" gorm:"column:owner_role"`
	IssuerUsername string `json:"issuer_username" gorm:"column:issuer_username"`
}

type InvitationCodeListResult struct {
	Items    []InvitationCodeListItem `json:"items"`
	Total    int64                    `json:"total"`
	Page     int                      `json:"page"`
	PageSize int                      `json:"page_size"`
}

type InvitationUseListItem struct {
	ID                  int64  `json:"id"`
	InviteCodeID        int64  `json:"invite_code_id"`
	InviterUserID       int    `json:"inviter_user_id"`
	RegisteredUserID    int    `json:"registered_user_id"`
	RegisteredAt        int64  `json:"registered_at"`
	Source              string `json:"source"`
	RegistrationIP      string `json:"registration_ip"`
	InviterReward       int    `json:"inviter_reward"`
	InviteeReward       int    `json:"invitee_reward"`
	InviterRewardStatus string `json:"inviter_reward_status"`
	InviteeRewardStatus string `json:"invitee_reward_status"`
	RewardAttemptCount  int    `json:"reward_attempt_count"`
	RewardNextAttemptAt int64  `json:"reward_next_attempt_at"`
	RewardsCompletedAt  int64  `json:"rewards_completed_at"`
	CreatedAt           int64  `json:"created_at"`
	UpdatedAt           int64  `json:"updated_at"`
	InviteCode          string `json:"invite_code" gorm:"column:invite_code"`
	InviterUsername     string `json:"inviter_username" gorm:"column:inviter_username"`
	InviterRole         int    `json:"inviter_role" gorm:"column:inviter_role"`
	RegisteredUsername  string `json:"registered_username" gorm:"column:registered_username"`
}

type InvitationUseListResult struct {
	Items    []InvitationUseListItem `json:"items"`
	Total    int64                   `json:"total"`
	Page     int                     `json:"page"`
	PageSize int                     `json:"page_size"`
}

func invitationRewardRequestItemAggregates(db *gorm.DB) *gorm.DB {
	return db.Table("invite_reward_payout_request_items AS aggregate_items").
		Joins("JOIN invite_reward_payouts AS aggregate_rewards ON aggregate_rewards.id = aggregate_items.reward_id").
		Select("aggregate_items.request_id, COUNT(*) AS item_count, COALESCE(SUM(aggregate_rewards.reversed_amount), 0) AS reversed_amount, COALESCE(SUM(CASE WHEN aggregate_rewards.amount >= aggregate_rewards.reversed_amount THEN aggregate_rewards.amount - aggregate_rewards.reversed_amount ELSE 0 END), 0) AS net_amount").
		Group("aggregate_items.request_id")
}

func ListInvitationApplications(ctx context.Context, query InvitationAdminListQuery) (InvitationApplicationListResult, error) {
	query.normalize()
	result := InvitationApplicationListResult{
		Items: make([]InvitationApplicationListItem, 0), Page: query.Page, PageSize: query.PageSize,
	}
	actor, err := invitationListActor(ctx, query.ActorID)
	if err != nil {
		return result, err
	}

	db := model.DB.WithContext(ctx).Table("invite_code_applications AS applications").
		Joins("JOIN users AS applicant ON applicant.id = applications.user_id").
		Joins("LEFT JOIN users AS reviewer ON reviewer.id = applications.reviewer_id")
	db = invitationActorScope(db, actor, "applicant.role")
	if query.Status != "" {
		db = db.Where("applications.status = ?", query.Status)
	}
	if query.Keyword != "" {
		pattern := "%" + strings.ToLower(query.Keyword) + "%"
		db = db.Where("LOWER(applicant.username) LIKE ? OR LOWER(applicant.display_name) LIKE ? OR LOWER(applications.reason) LIKE ?", pattern, pattern, pattern)
	}
	if err := db.Count(&result.Total).Error; err != nil {
		return result, err
	}
	offset, pastEnd := invitationListOffset(query.Page, query.PageSize, result.Total)
	if pastEnd {
		return result, nil
	}
	err = db.Select("applications.*, applicant.username AS applicant_username, applicant.role AS applicant_role, COALESCE(reviewer.username, '') AS reviewer_username").
		Order("applications.id DESC").Limit(query.PageSize).Offset(offset).
		Scan(&result.Items).Error
	return result, err
}

func ListInvitationCodes(ctx context.Context, query InvitationAdminListQuery) (InvitationCodeListResult, error) {
	query.normalize()
	result := InvitationCodeListResult{
		Items: make([]InvitationCodeListItem, 0), Page: query.Page, PageSize: query.PageSize,
	}
	actor, err := invitationListActor(ctx, query.ActorID)
	if err != nil {
		return result, err
	}

	db := model.DB.WithContext(ctx).Table("invite_codes AS codes").
		Joins("JOIN users AS owner ON owner.id = codes.owner_user_id").
		Joins("LEFT JOIN users AS issuer ON issuer.id = codes.issued_by")
	db = invitationActorScope(db, actor, "owner.role")
	if query.Status != "" {
		db = db.Where("codes.status = ?", query.Status)
	}
	if query.Keyword != "" {
		pattern := "%" + strings.ToLower(query.Keyword) + "%"
		db = db.Where("LOWER(codes.code) LIKE ? OR LOWER(owner.username) LIKE ? OR LOWER(owner.display_name) LIKE ? OR LOWER(COALESCE(issuer.username, '')) LIKE ?", pattern, pattern, pattern, pattern)
	}
	if err := db.Count(&result.Total).Error; err != nil {
		return result, err
	}
	offset, pastEnd := invitationListOffset(query.Page, query.PageSize, result.Total)
	if pastEnd {
		return result, nil
	}
	err = db.Select("codes.*, owner.username AS owner_username, owner.role AS owner_role, COALESCE(issuer.username, '') AS issuer_username").
		Order("codes.id DESC").Limit(query.PageSize).Offset(offset).
		Scan(&result.Items).Error
	return result, err
}

func ListInvitationUses(ctx context.Context, query InvitationAdminListQuery) (InvitationUseListResult, error) {
	query.normalize()
	result := InvitationUseListResult{
		Items: make([]InvitationUseListItem, 0), Page: query.Page, PageSize: query.PageSize,
	}
	actor, err := invitationListActor(ctx, query.ActorID)
	if err != nil {
		return result, err
	}

	db := model.DB.WithContext(ctx).Table("invite_code_uses AS uses").
		Joins("JOIN invite_codes AS codes ON codes.id = uses.invite_code_id").
		Joins("JOIN users AS inviter ON inviter.id = uses.inviter_user_id").
		Joins("JOIN users AS registered ON registered.id = uses.registered_user_id")
	db = invitationActorScope(db, actor, "inviter.role")
	if query.Status != "" {
		db = db.Where("uses.inviter_reward_status = ? OR uses.invitee_reward_status = ?", query.Status, query.Status)
	}
	if query.Keyword != "" {
		pattern := "%" + strings.ToLower(query.Keyword) + "%"
		db = db.Where("LOWER(codes.code) LIKE ? OR LOWER(inviter.username) LIKE ? OR LOWER(inviter.display_name) LIKE ? OR LOWER(registered.username) LIKE ? OR LOWER(registered.display_name) LIKE ? OR LOWER(uses.source) LIKE ?", pattern, pattern, pattern, pattern, pattern, pattern)
	}
	if err := db.Count(&result.Total).Error; err != nil {
		return result, err
	}
	offset, pastEnd := invitationListOffset(query.Page, query.PageSize, result.Total)
	if pastEnd {
		return result, nil
	}
	err = db.Select("uses.id, uses.invite_code_id, uses.inviter_user_id, uses.registered_user_id, uses.registered_at, uses.source, uses.registration_ip, uses.inviter_reward, uses.invitee_reward, uses.inviter_reward_status, uses.invitee_reward_status, uses.reward_attempt_count, uses.reward_next_attempt_at, uses.rewards_completed_at, uses.created_at, uses.updated_at, codes.code AS invite_code, inviter.username AS inviter_username, inviter.role AS inviter_role, registered.username AS registered_username").
		Order("uses.id DESC").Limit(query.PageSize).Offset(offset).
		Scan(&result.Items).Error
	if err != nil {
		return result, err
	}
	if actor.Id != 1 && actor.Role < common.RoleRootUser {
		for index := range result.Items {
			result.Items[index].RegistrationIP = model.MaskAuditIP(result.Items[index].RegistrationIP)
		}
	}
	return result, nil
}

func (query *InvitationAdminListQuery) normalize() {
	if query.Page < 1 {
		query.Page = 1
	}
	if query.PageSize < 1 {
		query.PageSize = 20
	} else if query.PageSize > 100 {
		query.PageSize = 100
	}
	query.Status = strings.TrimSpace(query.Status)
	query.Keyword = strings.TrimSpace(query.Keyword)
}

func invitationListOffset(page, pageSize int, total int64) (int, bool) {
	pageIndex := int64(page - 1)
	maxInt := int64(^uint(0) >> 1)
	if total <= 0 || pageIndex < 0 || pageIndex > maxInt/int64(pageSize) {
		return 0, true
	}
	offset := pageIndex * int64(pageSize)
	if offset >= total {
		return 0, true
	}
	return int(offset), false
}

func invitationListActor(ctx context.Context, actorID int) (model.User, error) {
	var actor model.User
	if actorID <= 0 || model.DB.WithContext(ctx).First(&actor, actorID).Error != nil ||
		actor.Status != common.UserStatusEnabled || actor.Role < common.RoleAdminUser {
		return model.User{}, ErrInviteTargetForbidden
	}
	return actor, nil
}

func invitationActorScope(db *gorm.DB, actor model.User, roleColumn string) *gorm.DB {
	if actor.Id == 1 {
		return db
	}
	if actor.Role >= common.RoleRootUser {
		return db.Where(roleColumn+" < ?", common.RoleRootUser)
	}
	return db.Where(roleColumn+" < ?", common.RoleAdminUser)
}
