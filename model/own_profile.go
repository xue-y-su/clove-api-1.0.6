package model

import (
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

var invalidateOwnProfileCache = InvalidateUserCache

// OwnProfilePatch is deliberately narrower than User. It is the only payload
// accepted by the self profile mutation path for profile and password fields.
type OwnProfilePatch struct {
	DisplayName    string
	HasDisplayName bool
	AvatarURL      string
	HasAvatarURL   bool
	PasswordHash   string
}

func DisplayNameNextChangeAt(user User, now time.Time) int64 {
	if user.DisplayNameUpdatedAt <= 0 {
		return 0
	}
	_, cooldown := displayNamePolicy()
	next := user.DisplayNameUpdatedAt + cooldown
	if next <= now.Unix() {
		return 0
	}
	return next
}

func UpdateOwnProfileWithTx(tx *gorm.DB, userID int, patch OwnProfilePatch, now time.Time) (*User, error) {
	var current User
	if err := lockForUpdate(tx).First(&current, userID).Error; err != nil {
		return nil, err
	}
	updates := map[string]interface{}{}
	if patch.HasDisplayName {
		name := strings.TrimSpace(patch.DisplayName)
		if err := ValidateDisplayName(name); err != nil {
			return nil, err
		}
		_, cooldown := displayNamePolicy()
		if name != current.DisplayName && current.DisplayNameUpdatedAt > 0 && now.Unix() < current.DisplayNameUpdatedAt+cooldown {
			return nil, ErrNicknameCooldown
		}
		updates["display_name"] = name
		if name != current.DisplayName {
			updates["display_name_updated_at"] = now.Unix()
		}
	}
	if patch.HasAvatarURL {
		if err := ValidateAvatarURL(strings.TrimSpace(patch.AvatarURL)); err != nil {
			return nil, err
		}
		updates["avatar_url"] = strings.TrimSpace(patch.AvatarURL)
	}
	if patch.PasswordHash != "" {
		updates["password"] = patch.PasswordHash
		updates["security_version"] = gorm.Expr("security_version + 1")
	}
	if len(updates) > 0 {
		if err := tx.Model(&User{}).Where("id = ?", userID).Updates(updates).Error; err != nil {
			return nil, err
		}
	}
	if err := tx.First(&current, userID).Error; err != nil {
		return nil, err
	}
	return &current, nil
}

func UpdateOwnProfile(userID int, patch OwnProfilePatch, now time.Time) (*User, error) {
	var updated *User
	if err := DB.Transaction(func(tx *gorm.DB) error {
		var err error
		updated, err = UpdateOwnProfileWithTx(tx, userID, patch, now)
		return err
	}); err != nil {
		return nil, err
	}
	if err := invalidateOwnProfileCache(userID); err != nil {
		common.SysLog(fmt.Sprintf("failed to invalidate profile cache for user %d: %v", userID, err))
	}
	return updated, nil
}

func BumpSecurityVersion(userID int) error {
	return BumpSecurityVersionWithTx(DB, userID)
}

func BumpSecurityVersionWithTx(tx *gorm.DB, userID int) error {
	return tx.Model(&User{}).Where("id = ?", userID).UpdateColumn("security_version", gorm.Expr("security_version + 1")).Error
}
