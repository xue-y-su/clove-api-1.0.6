package model

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/registration_setting"
	"gorm.io/gorm"
)

const (
	InviteLimitModeInherit   = "inherit"
	InviteLimitModeLimited   = "limited"
	InviteLimitModeUnlimited = "unlimited"
	InviteLimitModeExempt    = "exempt"
)

type InviteLimitStatus struct {
	Unlimited bool   `json:"unlimited"`
	Exempt    bool   `json:"exempt"`
	Limit     int    `json:"limit"`
	Used      int    `json:"used"`
	Remaining int    `json:"remaining"`
	Mode      string `json:"mode"`
}

func NormalizeInviteLimitMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case InviteLimitModeLimited:
		return InviteLimitModeLimited
	case InviteLimitModeUnlimited:
		return InviteLimitModeUnlimited
	default:
		return InviteLimitModeInherit
	}
}

func NormalizeInviteLimitCount(count int) int {
	if count < 0 {
		return 0
	}
	return count
}

func CountSuccessfulInvites(inviterId int) (int, error) {
	return CountSuccessfulInvitesWithTx(DB, inviterId)
}

func CountSuccessfulInvitesWithTx(tx *gorm.DB, inviterId int) (int, error) {
	if inviterId == 0 {
		return 0, nil
	}
	if tx == nil {
		tx = DB
	}
	var count int64
	query := tx.Unscoped().
		Model(&User{}).
		Where("inviter_id = ?", inviterId).
		Where("(inviter_source = ? OR inviter_source = '' OR inviter_source IS NULL)", InviterSourceInviteCode)
	if tx.Migrator().HasTable(&SourceAttributionConfirmation{}) {
		query = query.Where("NOT EXISTS (SELECT 1 FROM source_attribution_confirmations WHERE source_attribution_confirmations.user_id = users.id)")
	}
	err := query.Count(&count).Error
	return int(count), err
}

func GetInviteLimitStatus(inviter User, settings registration_setting.Settings) (InviteLimitStatus, error) {
	return GetInviteLimitStatusWithTx(DB, inviter, settings)
}

func GetInviteLimitStatusWithTx(tx *gorm.DB, inviter User, settings registration_setting.Settings) (InviteLimitStatus, error) {
	used, err := CountSuccessfulInvitesWithTx(tx, inviter.Id)
	if err != nil {
		return InviteLimitStatus{}, err
	}

	status := InviteLimitStatus{
		Used: used,
		Mode: NormalizeInviteLimitMode(inviter.InviteLimitMode),
	}

	switch status.Mode {
	case InviteLimitModeUnlimited:
		status.Unlimited = true
		return status, nil
	case InviteLimitModeLimited:
		status.Limit = NormalizeInviteLimitCount(inviter.InviteLimitCount)
	default:
		if inviter.Role >= common.RoleAdminUser {
			status.Unlimited = true
			status.Exempt = true
			status.Mode = InviteLimitModeExempt
			return status, nil
		}
		if settings.InviteLimitUnlimited {
			status.Unlimited = true
			return status, nil
		}
		status.Limit = NormalizeInviteLimitCount(settings.InviteLimitCount)
	}

	status.Remaining = status.Limit - status.Used
	if status.Remaining < 0 {
		status.Remaining = 0
	}
	return status, nil
}

func GetInviteLimitStatusByUserId(inviterId int, settings registration_setting.Settings) (InviteLimitStatus, error) {
	if inviterId == 0 {
		return InviteLimitStatus{Unlimited: true, Mode: InviteLimitModeUnlimited}, nil
	}
	user, err := GetUserById(inviterId, false)
	if err != nil {
		return InviteLimitStatus{}, err
	}
	return GetInviteLimitStatus(*user, settings)
}

func InviteLimitReached(status InviteLimitStatus) bool {
	return !status.Unlimited && !status.Exempt && status.Used >= status.Limit
}

func GetRegistrationInviterByAffCodeWithTx(tx *gorm.DB, affCode string) (User, error) {
	var user User
	affCode = strings.TrimSpace(affCode)
	if affCode == "" {
		return user, errors.New("affCode 为空！")
	}
	if tx == nil {
		tx = DB
	}
	err := lockForUpdate(tx).
		Select("id", "role", "invite_limit_mode", "invite_limit_count").
		First(&user, "aff_code = ?", affCode).Error
	return user, err
}
