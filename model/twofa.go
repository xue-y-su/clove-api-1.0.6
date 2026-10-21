package model

import (
	"errors"
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm"
)

// TwoFA 用户2FA设置表
type TwoFA struct {
	Id             int            `json:"id" gorm:"primaryKey"`
	UserId         int            `json:"user_id" gorm:"unique;not null;index"`
	Secret         string         `json:"-" gorm:"type:varchar(255);not null"` // TOTP密钥，不返回给前端
	IsEnabled      bool           `json:"is_enabled"`
	FailedAttempts int            `json:"failed_attempts" gorm:"default:0"`
	LockedUntil    *time.Time     `json:"locked_until,omitempty"`
	LastUsedAt     *time.Time     `json:"last_used_at,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `json:"-" gorm:"index"`
}

// TwoFABackupCode 备用码使用记录表
type TwoFABackupCode struct {
	Id        int            `json:"id" gorm:"primaryKey"`
	UserId    int            `json:"user_id" gorm:"not null;index"`
	CodeHash  string         `json:"-" gorm:"type:varchar(255);not null"` // 备用码哈希
	IsUsed    bool           `json:"is_used"`
	UsedAt    *time.Time     `json:"used_at,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

// GetTwoFAByUserId 根据用户ID获取2FA设置
func GetTwoFAByUserId(userId int) (*TwoFA, error) {
	if userId == 0 {
		return nil, errors.New("用户ID不能为空")
	}

	var twoFA TwoFA
	result := DB.Where("user_id = ?", userId).Limit(1).Find(&twoFA)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, nil // 返回nil表示未设置2FA
	}

	return &twoFA, nil
}

// IsTwoFAEnabled 检查用户是否启用了2FA
func IsTwoFAEnabled(userId int) bool {
	twoFA, err := GetTwoFAByUserId(userId)
	if err != nil || twoFA == nil {
		return false
	}
	return twoFA.IsEnabled
}

// CreateTwoFA 创建2FA设置
func (t *TwoFA) Create() error {
	// 检查用户是否已存在2FA设置
	existing, err := GetTwoFAByUserId(t.UserId)
	if err != nil {
		return err
	}
	if existing != nil {
		return errors.New("用户已存在2FA设置")
	}

	// 验证用户存在
	var user User
	if err := DB.First(&user, t.UserId).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("用户不存在")
		}
		return err
	}

	return DB.Create(t).Error
}

// Update 更新2FA设置
func (t *TwoFA) Update() error {
	if t.Id == 0 {
		return errors.New("2FA记录ID不能为空")
	}
	return DB.Save(t).Error
}

// Delete 删除2FA设置
func (t *TwoFA) Delete() error {
	if t.Id == 0 {
		return errors.New("2FA记录ID不能为空")
	}

	// 使用事务确保原子性
	return DB.Transaction(func(tx *gorm.DB) error {
		// 同时删除相关的备用码记录（硬删除）
		if err := tx.Unscoped().Where("user_id = ?", t.UserId).Delete(&TwoFABackupCode{}).Error; err != nil {
			return err
		}

		// 硬删除2FA记录
		return tx.Unscoped().Delete(t).Error
	})
}

// ResetFailedAttempts 重置失败尝试次数
func (t *TwoFA) ResetFailedAttempts() error {
	if t == nil || t.Id == 0 {
		return errors.New("2FA记录ID不能为空")
	}
	var current TwoFA
	err := DB.Transaction(func(tx *gorm.DB) error {
		var err error
		current, err = lockTwoFAByID(tx, t.Id)
		if err != nil {
			return err
		}
		return updateTwoFASuccessWithTx(tx, &current, current.LastUsedAt)
	})
	if err == nil {
		*t = current
	}
	return err
}

// IncrementFailedAttempts 增加失败尝试次数
func (t *TwoFA) IncrementFailedAttempts() error {
	if t == nil || t.Id == 0 {
		return errors.New("2FA记录ID不能为空")
	}
	var current TwoFA
	err := DB.Transaction(func(tx *gorm.DB) error {
		var err error
		current, err = lockTwoFAByID(tx, t.Id)
		if err != nil {
			return err
		}
		if current.IsLocked() {
			return twoFALockedError(&current)
		}
		return updateTwoFAFailureWithTx(tx, &current, time.Now())
	})
	if current.Id != 0 {
		*t = current
	}
	return err
}

// IsLocked 检查账户是否被锁定
func (t *TwoFA) IsLocked() bool {
	if t.LockedUntil == nil {
		return false
	}
	return time.Now().Before(*t.LockedUntil)
}

// CreateBackupCodes 创建备用码
func CreateBackupCodes(userId int, codes []string) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		// 先删除现有的备用码
		if err := tx.Where("user_id = ?", userId).Delete(&TwoFABackupCode{}).Error; err != nil {
			return err
		}

		// 创建新的备用码记录
		for _, code := range codes {
			hashedCode, err := common.HashBackupCode(code)
			if err != nil {
				return err
			}

			backupCode := TwoFABackupCode{
				UserId:   userId,
				CodeHash: hashedCode,
				IsUsed:   false,
			}

			if err := tx.Create(&backupCode).Error; err != nil {
				return err
			}
		}

		return nil
	})
}

// ValidateBackupCode 验证并使用备用码
func ValidateBackupCode(userId int, code string) (bool, error) {
	var valid bool
	err := DB.Transaction(func(tx *gorm.DB) error {
		var err error
		valid, err = validateBackupCodeWithTx(tx, userId, code)
		return err
	})
	return valid, err
}

func validateBackupCodeWithTx(tx *gorm.DB, userId int, code string) (bool, error) {
	if !common.ValidateBackupCode(code) {
		return false, errors.New("验证码或备用码不正确")
	}

	normalizedCode := common.NormalizeBackupCode(code)

	// 查找未使用的备用码
	var backupCodes []TwoFABackupCode
	if err := tx.Where("user_id = ? AND is_used = ?", userId, false).Find(&backupCodes).Error; err != nil {
		return false, err
	}

	// 验证备用码
	for _, bc := range backupCodes {
		if common.ValidatePasswordAndHash(normalizedCode, bc.CodeHash) {
			// 标记为已使用
			now := time.Now()
			result := tx.Model(&TwoFABackupCode{}).
				Where("id = ? AND is_used = ?", bc.Id, false).
				Updates(map[string]any{"is_used": true, "used_at": &now})
			if result.Error != nil {
				return false, result.Error
			}
			if result.RowsAffected != 1 {
				return false, nil
			}

			return true, nil
		}
	}

	return false, nil
}

// GetUnusedBackupCodeCount 获取未使用的备用码数量
func GetUnusedBackupCodeCount(userId int) (int, error) {
	var count int64
	err := DB.Model(&TwoFABackupCode{}).Where("user_id = ? AND is_used = false", userId).Count(&count).Error
	return int(count), err
}

// DisableTwoFA 禁用用户的2FA
func DisableTwoFA(userId int) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		var twoFA TwoFA
		if err := lockForUpdate(tx).Where("user_id = ?", userId).First(&twoFA).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrTwoFANotEnabled
			}
			return err
		}
		if err := tx.Unscoped().Where("user_id = ?", userId).Delete(&TwoFABackupCode{}).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Delete(&twoFA).Error; err != nil {
			return err
		}
		return BumpSecurityVersionWithTx(tx, userId)
	})
}

// EnableTwoFA 启用2FA
func (t *TwoFA) Enable() error {
	t.IsEnabled = true
	t.FailedAttempts = 0
	t.LockedUntil = nil
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(t).Error; err != nil {
			return err
		}
		return BumpSecurityVersionWithTx(tx, t.UserId)
	})
}

// ValidateTOTPAndUpdateUsage 验证TOTP并更新使用记录
func (t *TwoFA) ValidateTOTPAndUpdateUsage(code string) (bool, error) {
	if t == nil || t.Id == 0 {
		return false, errors.New("2FA记录ID不能为空")
	}
	valid := false
	var current TwoFA
	err := DB.Transaction(func(tx *gorm.DB) error {
		var err error
		current, err = lockTwoFAByID(tx, t.Id)
		if err != nil {
			return err
		}
		if current.IsLocked() {
			return twoFALockedError(&current)
		}
		now := time.Now()
		if !common.ValidateTOTPCode(current.Secret, code) {
			return updateTwoFAFailureWithTx(tx, &current, now)
		}
		valid = true
		return updateTwoFASuccessWithTx(tx, &current, &now)
	})
	if current.Id != 0 {
		*t = current
	}
	return valid, err
}

// ValidateBackupCodeAndUpdateUsage 验证备用码并更新使用记录
func (t *TwoFA) ValidateBackupCodeAndUpdateUsage(code string) (bool, error) {
	if t == nil || t.Id == 0 {
		return false, errors.New("2FA记录ID不能为空")
	}
	valid := false
	var current TwoFA
	err := DB.Transaction(func(tx *gorm.DB) error {
		var err error
		current, err = lockTwoFAByID(tx, t.Id)
		if err != nil {
			return err
		}
		if current.IsLocked() {
			return twoFALockedError(&current)
		}
		valid, err = validateBackupCodeWithTx(tx, current.UserId, code)
		if err != nil {
			return err
		}
		now := time.Now()
		if !valid {
			return updateTwoFAFailureWithTx(tx, &current, now)
		}
		return updateTwoFASuccessWithTx(tx, &current, &now)
	})
	if current.Id != 0 {
		*t = current
	}
	return valid, err
}

func lockTwoFAByID(tx *gorm.DB, id int) (TwoFA, error) {
	var current TwoFA
	if tx == nil || id == 0 {
		return current, errors.New("2FA记录ID不能为空")
	}
	err := lockForUpdate(tx).Where("id = ?", id).First(&current).Error
	return current, err
}

func updateTwoFAFailureWithTx(tx *gorm.DB, current *TwoFA, now time.Time) error {
	current.FailedAttempts++
	if current.FailedAttempts >= common.MaxFailAttempts {
		lockUntil := now.Add(time.Duration(common.LockoutDuration) * time.Second)
		current.LockedUntil = &lockUntil
	}
	return tx.Model(&TwoFA{}).Where("id = ?", current.Id).Updates(map[string]any{
		"failed_attempts": current.FailedAttempts,
		"locked_until":    current.LockedUntil,
	}).Error
}

func updateTwoFASuccessWithTx(tx *gorm.DB, current *TwoFA, lastUsedAt *time.Time) error {
	current.FailedAttempts = 0
	current.LockedUntil = nil
	current.LastUsedAt = lastUsedAt
	return tx.Model(&TwoFA{}).Where("id = ?", current.Id).Updates(map[string]any{
		"failed_attempts": 0,
		"locked_until":    nil,
		"last_used_at":    lastUsedAt,
	}).Error
}

func twoFALockedError(current *TwoFA) error {
	return fmt.Errorf("账户已被锁定，请在%v后重试", current.LockedUntil.Format("2006-01-02 15:04:05"))
}

// GetTwoFAStats 获取2FA统计信息（管理员使用）
func GetTwoFAStats() (map[string]interface{}, error) {
	var totalUsers, enabledUsers int64

	// 总用户数
	if err := DB.Model(&User{}).Count(&totalUsers).Error; err != nil {
		return nil, err
	}

	// 启用2FA的用户数
	if err := DB.Model(&TwoFA{}).Where("is_enabled = true").Count(&enabledUsers).Error; err != nil {
		return nil, err
	}

	enabledRate := float64(0)
	if totalUsers > 0 {
		enabledRate = float64(enabledUsers) / float64(totalUsers) * 100
	}

	return map[string]interface{}{
		"total_users":   totalUsers,
		"enabled_users": enabledUsers,
		"enabled_rate":  fmt.Sprintf("%.1f%%", enabledRate),
	}, nil
}
