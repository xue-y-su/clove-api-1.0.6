package model

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"gorm.io/gorm"
)

type Redemption struct {
	Id           int            `json:"id"`
	UserId       int            `json:"user_id"`
	Key          string         `json:"key" gorm:"type:char(32);uniqueIndex"`
	Status       int            `json:"status" gorm:"default:1"`
	Name         string         `json:"name" gorm:"index"`
	Quota        int            `json:"quota" gorm:"default:100"`
	CreatedTime  int64          `json:"created_time" gorm:"bigint"`
	RedeemedTime int64          `json:"redeemed_time" gorm:"bigint"`
	Count        int            `json:"count" gorm:"-:all"` // only for api request
	UsedUserId   int            `json:"used_user_id"`
	DeletedAt    gorm.DeletedAt `gorm:"index"`
	ExpiredTime  int64          `json:"expired_time" gorm:"bigint"` // 过期时间，0 表示不过期
}

func GetAllRedemptions(startIdx int, num int) (redemptions []*Redemption, total int64, err error) {
	// 开始事务
	tx := DB.Begin()
	if tx.Error != nil {
		return nil, 0, tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 获取总数
	err = tx.Model(&Redemption{}).Count(&total).Error
	if err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	// 获取分页数据
	err = tx.Order("id desc").Limit(num).Offset(startIdx).Find(&redemptions).Error
	if err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	// 提交事务
	if err = tx.Commit().Error; err != nil {
		return nil, 0, err
	}

	return redemptions, total, nil
}

func SearchRedemptions(keyword string, status string, startIdx int, num int) (redemptions []*Redemption, total int64, err error) {
	tx := DB.Begin()
	if tx.Error != nil {
		return nil, 0, tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	query := tx.Model(&Redemption{})

	if keyword != "" {
		if id, err := strconv.Atoi(keyword); err == nil {
			query = query.Where("id = ? OR name LIKE ?", id, keyword+"%")
		} else {
			query = query.Where("name LIKE ?", keyword+"%")
		}
	}

	if status != "" {
		now := common.GetTimestamp()
		switch status {
		case "expired":
			query = query.Where(
				"status = ? AND expired_time != 0 AND expired_time < ?",
				common.RedemptionCodeStatusEnabled,
				now,
			)
		case strconv.Itoa(common.RedemptionCodeStatusEnabled):
			query = query.Where(
				"status = ? AND (expired_time = 0 OR expired_time >= ?)",
				common.RedemptionCodeStatusEnabled,
				now,
			)
		case strconv.Itoa(common.RedemptionCodeStatusDisabled):
			query = query.Where("status = ?", common.RedemptionCodeStatusDisabled)
		case strconv.Itoa(common.RedemptionCodeStatusUsed):
			query = query.Where("status = ?", common.RedemptionCodeStatusUsed)
		}
	}

	// Get total count
	err = query.Count(&total).Error
	if err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	// Get paginated data
	err = query.Order("id desc").Limit(num).Offset(startIdx).Find(&redemptions).Error
	if err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	if err = tx.Commit().Error; err != nil {
		return nil, 0, err
	}

	return redemptions, total, nil
}

func GetRedemptionById(id int) (*Redemption, error) {
	if id == 0 {
		return nil, errors.New("id 为空！")
	}
	redemption := Redemption{Id: id}
	var err error = nil
	err = DB.First(&redemption, "id = ?", id).Error
	return &redemption, err
}

func Redeem(key string, userId int) (quota int, err error) {
	if key == "" {
		return 0, ErrRedemptionInvalid
	}
	if userId == 0 {
		return 0, ErrRedemptionInvalid
	}
	redemption := &Redemption{}

	keyCol := "`key`"
	if common.UsingMainDatabase(common.DatabaseTypePostgreSQL) {
		keyCol = `"key"`
	}
	common.RandomSleep()
	err = DB.Transaction(func(tx *gorm.DB) error {
		err := lockForUpdate(tx).Where(keyCol+" = ?", key).First(redemption).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrRedemptionInvalid
			}
			return err
		}
		if redemption.Status != common.RedemptionCodeStatusEnabled {
			return ErrRedemptionUsed
		}
		if redemption.Quota <= 0 || redemption.Quota > common.MaxQuota {
			return ErrRedemptionInvalid
		}
		if redemption.ExpiredTime != 0 && redemption.ExpiredTime < common.GetTimestamp() {
			return ErrRedemptionExpired
		}
		// Compare-and-swap on status: only the transaction that flips
		// enabled -> used may credit quota, so a concurrent redeem of the
		// same code loses here even without a row lock (e.g. on SQLite).
		result := tx.Model(&Redemption{}).
			Where("id = ? AND status = ?", redemption.Id, common.RedemptionCodeStatusEnabled).
			Updates(map[string]interface{}{
				"redeemed_time": common.GetTimestamp(),
				"status":        common.RedemptionCodeStatusUsed,
				"used_user_id":  userId,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrRedemptionUsed
		}
		if _, err := AdjustWalletWithTx(tx, userId, int64(redemption.Quota),
			fmt.Sprintf("redemption:%d", redemption.Id),
			"admin_redemption",
			WalletAdjustReference{Type: "redemption", ID: fmt.Sprintf("%d", redemption.Id), Context: "redeem"},
		); err != nil {
			return err
		}
		setting := operation_setting.GetInvitationIncentiveSetting()
		if setting.Enabled && setting.AdminBalanceRedemptionCommissionEnabled {
			rateBasisPoints, _, err := operation_setting.ParseInvitationCommissionRate(setting.AdminBalanceRedemptionCommissionRate)
			if err != nil {
				return err
			}
			if _, _, err := CreateInvitationCommissionWithTx(tx, InvitationCommissionInput{
				InviteeUserID: userId, TriggerType: InviteRewardTriggerAdminBalanceRedemption,
				TriggerID: strconv.Itoa(redemption.Id), EligibleQuota: int64(redemption.Quota),
				RateBasisPoints: rateBasisPoints,
			}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		common.SysError("redemption failed: " + err.Error())
		if errors.Is(err, ErrRedemptionInvalid) || errors.Is(err, ErrRedemptionUsed) || errors.Is(err, ErrRedemptionExpired) {
			return 0, err
		}
		return 0, ErrRedeemFailed
	}
	_ = InvalidateUserCache(userId)
	RecordLog(userId, LogTypeTopup, fmt.Sprintf("通过兑换码充值 %s，兑换码ID %d", logger.LogQuota(redemption.Quota), redemption.Id))
	return redemption.Quota, nil
}

func IsRedemptionCodeAvailable(tx *gorm.DB, code string) (bool, error) {
	var count int64
	if err := tx.Unscoped().Model(&Redemption{}).Where(commonKeyCol+" = ?", code).Count(&count).Error; err != nil {
		return false, err
	}
	if count > 0 {
		return false, nil
	}
	if err := tx.Model(&UserRedemptionCode{}).Where("code = ?", code).Count(&count).Error; err != nil {
		return false, err
	}
	return count == 0, nil
}

func (redemption *Redemption) Insert() error {
	if redemption == nil {
		return ErrRedemptionInvalid
	}
	redemption.Name = strings.TrimSpace(redemption.Name)
	if redemption.Status == 0 {
		redemption.Status = common.RedemptionCodeStatusEnabled
	}
	if !validRedemptionDefinition(redemption) ||
		(redemption.Status != common.RedemptionCodeStatusEnabled && redemption.Status != common.RedemptionCodeStatusDisabled) {
		return ErrRedemptionInvalid
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		available, err := IsRedemptionCodeAvailable(tx, redemption.Key)
		if err != nil {
			return err
		}
		if !available {
			return ErrRedemptionCodeConflict
		}
		return tx.Create(redemption).Error
	})
}

func (redemption *Redemption) SelectUpdate() error {
	if redemption == nil {
		return ErrRedemptionInvalid
	}
	return redemption.UpdateStatus(redemption.Status)
}

// Update changes editable metadata without ever writing the redemption status.
func (redemption *Redemption) Update() error {
	if redemption == nil || redemption.Id <= 0 {
		return ErrRedemptionInvalid
	}
	redemption.Name = strings.TrimSpace(redemption.Name)
	if !validRedemptionDefinition(redemption) {
		return ErrRedemptionInvalid
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		var current Redemption
		if err := lockForUpdate(tx).First(&current, redemption.Id).Error; err != nil {
			return err
		}
		if current.Status == common.RedemptionCodeStatusUsed && current.Quota != redemption.Quota {
			return ErrRedemptionUsed
		}
		if current.Name == redemption.Name && current.Quota == redemption.Quota && current.ExpiredTime == redemption.ExpiredTime {
			return nil
		}
		result := tx.Model(&Redemption{}).
			Where("id = ? AND (status <> ? OR quota = ?)", redemption.Id, common.RedemptionCodeStatusUsed, redemption.Quota).
			Updates(map[string]any{
				"name": redemption.Name, "quota": redemption.Quota, "expired_time": redemption.ExpiredTime,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrRedemptionUsed
		}
		return nil
	})
}

func (redemption *Redemption) UpdateStatus(status int) error {
	if redemption == nil || redemption.Id <= 0 ||
		(status != common.RedemptionCodeStatusEnabled && status != common.RedemptionCodeStatusDisabled) {
		return ErrRedemptionInvalid
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		var current Redemption
		if err := lockForUpdate(tx).First(&current, redemption.Id).Error; err != nil {
			return err
		}
		if current.Status == common.RedemptionCodeStatusUsed {
			return ErrRedemptionUsed
		}
		if current.Status == status {
			redemption.Status = status
			return nil
		}
		result := tx.Model(&Redemption{}).
			Where("id = ? AND status <> ?", redemption.Id, common.RedemptionCodeStatusUsed).
			Update("status", status)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrRedemptionUsed
		}
		redemption.Status = status
		return nil
	})
}

func validRedemptionDefinition(redemption *Redemption) bool {
	if redemption == nil || redemption.Quota <= 0 || redemption.Quota > common.MaxQuota {
		return false
	}
	nameLength := utf8.RuneCountInString(strings.TrimSpace(redemption.Name))
	if nameLength == 0 || nameLength > 20 {
		return false
	}
	return redemption.ExpiredTime == 0 || redemption.ExpiredTime >= common.GetTimestamp()
}

func (redemption *Redemption) Delete() error {
	var err error
	err = DB.Delete(redemption).Error
	return err
}

func DeleteRedemptionById(id int) (err error) {
	if id == 0 {
		return errors.New("id 为空！")
	}
	redemption := Redemption{Id: id}
	err = DB.Where(redemption).First(&redemption).Error
	if err != nil {
		return err
	}
	return redemption.Delete()
}

func DeleteInvalidRedemptions() (int64, error) {
	now := common.GetTimestamp()
	result := DB.Where("status IN ? OR (status = ? AND expired_time != 0 AND expired_time < ?)", []int{common.RedemptionCodeStatusUsed, common.RedemptionCodeStatusDisabled}, common.RedemptionCodeStatusEnabled, now).Delete(&Redemption{})
	return result.RowsAffected, result.Error
}
