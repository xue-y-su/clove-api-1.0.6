package model

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"gorm.io/gorm"
)

// Checkin 签到记录
type Checkin struct {
	Id             int    `json:"id" gorm:"primaryKey;autoIncrement"`
	UserId         int    `json:"user_id" gorm:"not null;uniqueIndex:idx_user_checkin_date"`
	CheckinDate    string `json:"checkin_date" gorm:"type:varchar(10);not null;uniqueIndex:idx_user_checkin_date"` // 格式: YYYY-MM-DD
	QuotaAwarded   int    `json:"quota_awarded" gorm:"not null"`
	QuotaRemaining int64  `json:"quota_remaining" gorm:"not null;default:0"`
	ExpiresAt      int64  `json:"expires_at" gorm:"not null;default:0;index:idx_checkin_expiry,priority:1"`
	ReclaimedQuota int64  `json:"reclaimed_quota" gorm:"not null;default:0"`
	SettledAt      int64  `json:"settled_at" gorm:"not null;default:0;index:idx_checkin_expiry,priority:2"`
	CreatedAt      int64  `json:"created_at" gorm:"bigint"`
}

// CheckinRecord 用于API返回的签到记录（不包含敏感字段）
type CheckinRecord struct {
	CheckinDate    string `json:"checkin_date"`
	QuotaAwarded   int    `json:"quota_awarded"`
	QuotaRemaining int64  `json:"quota_remaining"`
	ExpiresAt      int64  `json:"expires_at"`
	ReclaimedQuota int64  `json:"reclaimed_quota"`
}

func (Checkin) TableName() string {
	return "checkins"
}

// GetUserCheckinRecords 获取用户在指定日期范围内的签到记录
func GetUserCheckinRecords(userId int, startDate, endDate string) ([]Checkin, error) {
	var records []Checkin
	err := DB.Where("user_id = ? AND checkin_date >= ? AND checkin_date <= ?",
		userId, startDate, endDate).
		Order("checkin_date DESC").
		Find(&records).Error
	return records, err
}

// HasCheckedInToday 检查用户今天是否已签到
func HasCheckedInToday(userId int) (bool, error) {
	today := time.Now().Format("2006-01-02")
	var count int64
	err := DB.Model(&Checkin{}).
		Where("user_id = ? AND checkin_date = ?", userId, today).
		Count(&count).Error
	return count > 0, err
}

// UserCheckin 执行用户签到，并在同一事务中写入奖励和钱包流水。
func UserCheckin(userId int) (*Checkin, error) {
	setting := operation_setting.GetCheckinSetting()
	if !setting.Enabled {
		return nil, errors.New("签到功能未启用")
	}

	// 检查今天是否已签到
	hasChecked, err := HasCheckedInToday(userId)
	if err != nil {
		return nil, err
	}
	if hasChecked {
		return nil, errors.New("今日已签到")
	}

	// 计算随机额度奖励
	quotaAwarded, err := randomCheckinQuota(setting.MinQuota, setting.MaxQuota)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	checkin := newCheckinReward(
		userId,
		now.Format("2006-01-02"),
		quotaAwarded,
		now,
		setting.ExpireUnusedQuota,
	)

	return userCheckinWithTransaction(checkin, userId, quotaAwarded)
}

func randomCheckinQuota(minQuota int, maxQuota int) (int, error) {
	if minQuota < 0 || maxQuota < minQuota || maxQuota > common.MaxQuota {
		return 0, errors.New("签到奖励额度配置无效")
	}
	if minQuota == maxQuota {
		return minQuota, nil
	}

	span := int64(maxQuota) - int64(minQuota) + 1
	offset, err := rand.Int(rand.Reader, big.NewInt(span))
	if err != nil {
		return 0, fmt.Errorf("生成签到奖励失败: %w", err)
	}
	return minQuota + int(offset.Int64()), nil
}

func nextLocalMidnight(now time.Time) int64 {
	year, month, day := now.Date()
	return time.Date(year, month, day+1, 0, 0, 0, 0, now.Location()).Unix()
}

func newCheckinReward(userID int, checkinDate string, quotaAwarded int, now time.Time, expireUnusedQuota bool) *Checkin {
	checkin := &Checkin{
		UserId:       userID,
		CheckinDate:  checkinDate,
		QuotaAwarded: quotaAwarded,
		CreatedAt:    now.Unix(),
	}
	if expireUnusedQuota {
		checkin.QuotaRemaining = int64(quotaAwarded)
		checkin.ExpiresAt = nextLocalMidnight(now)
	}
	return checkin
}

func consumeActiveCheckinRewardWithTx(tx *gorm.DB, userID int, delta, now int64) error {
	if delta >= 0 {
		return nil
	}

	var checkin Checkin
	result := lockForUpdate(tx).
		Where("user_id = ? AND expires_at > ? AND settled_at = 0 AND quota_remaining > 0", userID, now).
		Order("expires_at ASC, id ASC").
		Limit(1).
		Find(&checkin)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return nil
	}

	remaining := checkin.QuotaRemaining
	if delta > -remaining {
		remaining += delta
	} else {
		remaining = 0
	}
	updates := map[string]any{"quota_remaining": remaining}
	if remaining == 0 {
		updates["settled_at"] = now
	}
	return tx.Model(&Checkin{}).Where("id = ?", checkin.Id).Updates(updates).Error
}

// SettleDueCheckinRewards settles a bounded batch. Candidate discovery does not
// lock rows; each item is rechecked transactionally after locking its user.
func SettleDueCheckinRewards(limit int) (int, error) {
	if limit <= 0 {
		return 0, nil
	}
	now := time.Now().Unix()
	var candidates []struct {
		ID     int
		UserID int
	}
	if err := DB.Model(&Checkin{}).
		Select("id", "user_id").
		Where("expires_at > 0 AND expires_at <= ? AND settled_at = 0", now).
		Order("id ASC").
		Limit(limit).
		Find(&candidates).Error; err != nil {
		return 0, err
	}

	settledCount := 0
	for _, candidate := range candidates {
		settled := false
		err := DB.Transaction(func(tx *gorm.DB) error {
			var user struct{ ID int }
			if err := lockForUpdate(tx).Model(&User{}).Select("id").First(&user, candidate.UserID).Error; err != nil {
				return err
			}
			var err error
			settled, err = settleCheckinRewardWithLockedUser(tx, candidate.UserID, candidate.ID, now)
			return err
		})
		if err != nil {
			return settledCount, err
		}
		if settled {
			settledCount++
			_ = InvalidateUserCache(candidate.UserID)
		}
	}
	return settledCount, nil
}

func settleDueCheckinRewardsForUserWithTx(tx *gorm.DB, userID int, now int64) (bool, error) {
	var checkinIDs []int
	if err := tx.Model(&Checkin{}).
		Where("user_id = ? AND expires_at > 0 AND expires_at <= ? AND settled_at = 0", userID, now).
		Order("id ASC").
		Pluck("id", &checkinIDs).Error; err != nil {
		return false, err
	}

	settledAny := false
	for _, checkinID := range checkinIDs {
		settled, err := settleCheckinRewardWithLockedUser(tx, userID, checkinID, now)
		if err != nil {
			return false, err
		}
		settledAny = settledAny || settled
	}
	return settledAny, nil
}

func settleCheckinRewardWithLockedUser(tx *gorm.DB, userID, checkinID int, now int64) (bool, error) {
	var checkin Checkin
	result := lockForUpdate(tx).
		Where("id = ? AND user_id = ? AND expires_at > 0 AND expires_at <= ? AND settled_at = 0", checkinID, userID, now).
		Limit(1).
		Find(&checkin)
	if result.Error != nil {
		return false, result.Error
	}
	if result.RowsAffected == 0 {
		return false, nil
	}

	var user struct{ Quota int64 }
	if err := tx.Model(&User{}).Select("quota").First(&user, userID).Error; err != nil {
		return false, err
	}
	reclaim := checkin.QuotaRemaining
	if reclaim < 0 {
		reclaim = 0
	}
	if user.Quota <= 0 {
		reclaim = 0
	} else if reclaim > user.Quota {
		reclaim = user.Quota
	}
	if reclaim > 0 {
		if _, err := adjustWalletWithTx(
			tx,
			userID,
			-reclaim,
			fmt.Sprintf("checkin-expiry:%d", checkin.Id),
			"checkin_expiry",
			WalletAdjustReference{Type: "checkin", ID: checkin.CheckinDate, Context: "unused_daily_reward"},
			walletAdjustOptions{skipCheckinLifecycle: true},
		); err != nil {
			return false, err
		}
	}

	updates := map[string]any{
		"quota_remaining": 0,
		"reclaimed_quota": reclaim,
		"settled_at":      now,
	}
	if err := tx.Model(&Checkin{}).Where("id = ?", checkin.Id).Updates(updates).Error; err != nil {
		return false, err
	}
	return true, nil
}

// userCheckinWithTransaction 使用事务执行签到。
func userCheckinWithTransaction(checkin *Checkin, userId int, quotaAwarded int) (*Checkin, error) {
	err := DB.Transaction(func(tx *gorm.DB) error {
		// 步骤1: 创建签到记录
		// 数据库有唯一约束 (user_id, checkin_date)，可以防止并发重复签到
		if err := tx.Create(checkin).Error; err != nil {
			return errors.New("签到失败，请稍后重试")
		}

		// 步骤2: 账本入账（operation_id 按用户+日期幂等）
		if _, err := AdjustWalletWithTx(tx, userId, int64(quotaAwarded),
			fmt.Sprintf("checkin:%d:%s", userId, checkin.CheckinDate),
			"checkin",
			WalletAdjustReference{Type: "checkin", ID: checkin.CheckinDate, Context: "daily"},
		); err != nil {
			return errors.New("签到失败：更新额度出错")
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	_ = InvalidateUserCache(userId)
	return checkin, nil
}

// GetUserCheckinStats 获取用户签到统计信息
func GetUserCheckinStats(userId int, month string) (map[string]interface{}, error) {
	// 获取指定月份的所有签到记录
	startDate := month + "-01"
	endDate := month + "-31"

	records, err := GetUserCheckinRecords(userId, startDate, endDate)
	if err != nil {
		return nil, err
	}

	// 转换为不包含敏感字段的记录
	checkinRecords := make([]CheckinRecord, len(records))
	for i, r := range records {
		checkinRecords[i] = CheckinRecord{
			CheckinDate:    r.CheckinDate,
			QuotaAwarded:   r.QuotaAwarded,
			QuotaRemaining: r.QuotaRemaining,
			ExpiresAt:      r.ExpiresAt,
			ReclaimedQuota: r.ReclaimedQuota,
		}
	}

	// 检查今天是否已签到
	hasCheckedToday, _ := HasCheckedInToday(userId)

	// 获取用户所有时间的签到统计
	var totalCheckins int64
	var totalQuota int64
	DB.Model(&Checkin{}).Where("user_id = ?", userId).Count(&totalCheckins)
	DB.Model(&Checkin{}).Where("user_id = ?", userId).Select("COALESCE(SUM(quota_awarded), 0)").Scan(&totalQuota)

	return map[string]interface{}{
		"total_quota":      totalQuota,      // 所有时间累计获得的额度
		"total_checkins":   totalCheckins,   // 所有时间累计签到次数
		"checkin_count":    len(records),    // 本月签到次数
		"checked_in_today": hasCheckedToday, // 今天是否已签到
		"records":          checkinRecords,  // 本月签到记录详情（不含id和user_id）
	}, nil
}
