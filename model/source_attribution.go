package model

import (
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/source_attribution_setting"
	"gorm.io/gorm"
)

type SourceAttributionConfirmation struct {
	Id                int    `json:"id"`
	UserId            int    `json:"user_id" gorm:"uniqueIndex:idx_source_attribution_confirmations_user_id"`
	Username          string `json:"username" gorm:"index;default:''"`
	OptionId          string `json:"option_id" gorm:"type:varchar(128);index;default:''"`
	OptionName        string `json:"option_name" gorm:"type:varchar(255);default:''"`
	TargetUserID      int    `json:"target_user_id" gorm:"column:target_inviter_id;index;default:0"`
	TargetUsername    string `json:"target_username" gorm:"index;default:''"`
	TargetAffCode     string `json:"target_aff_code" gorm:"type:varchar(32);index;default:''"`
	SourceDescription string `json:"source_description" gorm:"type:text"`
	Ip                string `json:"ip" gorm:"type:varchar(64);index;default:''"`
	UserAgent         string `json:"user_agent" gorm:"type:text"`
	ConfirmedAt       int64  `json:"confirmed_at" gorm:"bigint;index"`
}

type SourceAttributionSelection struct {
	OptionId          string
	OptionName        string
	TargetUserID      int
	TargetAffCode     string
	SourceDescription string
	Ip                string
	UserAgent         string
	ConfirmedAt       int64
}

type SourceAttributionResult struct {
	AlreadyConfirmed bool
	InviterId        int
	Confirmation     *SourceAttributionConfirmation
}

type SourceAttributionConfirmationFilter struct {
	UserId         int
	Username       string
	OptionId       string
	TargetUserID   int
	TargetAffCode  string
	Ip             string
	StartTimestamp int64
	EndTimestamp   int64
}

func ConfirmSourceAttribution(userId int, username string, selection SourceAttributionSelection) (SourceAttributionResult, error) {
	var result SourceAttributionResult
	if userId <= 0 {
		return result, errors.New("用户 ID 不能为空")
	}
	selection.OptionId = strings.TrimSpace(selection.OptionId)
	selection.OptionName = strings.TrimSpace(selection.OptionName)
	selection.TargetAffCode = strings.TrimSpace(selection.TargetAffCode)
	selection.SourceDescription = strings.TrimSpace(selection.SourceDescription)
	selection.Ip = NormalizeAuditIP(selection.Ip)
	if selection.OptionId == "" {
		return result, errors.New("来源归属选项不能为空")
	}
	if selection.TargetUserID <= 0 && selection.TargetAffCode == "" {
		return result, errors.New("来源归属目标用户 ID 不能为空")
	}
	if utf8.RuneCountInString(selection.SourceDescription) > source_attribution_setting.MaxSourceDescriptionLength {
		return result, errors.New("来源说明过长")
	}
	if selection.ConfirmedAt == 0 {
		selection.ConfirmedAt = common.GetTimestamp()
	}

	err := DB.Transaction(func(tx *gorm.DB) error {
		var user User
		if err := tx.First(&user, userId).Error; err != nil {
			return err
		}
		if user.InviterId != 0 {
			result.AlreadyConfirmed = true
			result.InviterId = user.InviterId
			return nil
		}

		var inviter User
		inviterQuery := tx.Select("id", "username", "aff_code")
		if selection.TargetUserID > 0 {
			inviterQuery = inviterQuery.Where("id = ?", selection.TargetUserID)
		} else {
			inviterQuery = inviterQuery.Where("aff_code = ?", selection.TargetAffCode)
		}
		if err := inviterQuery.First(&inviter).Error; err != nil {
			return err
		}
		if inviter.Id == user.Id {
			return errors.New("来源归属不能绑定当前用户自己")
		}

		update := tx.Model(&User{}).Where("id = ? AND (inviter_id = 0 OR inviter_id IS NULL)", user.Id).Updates(map[string]interface{}{
			"inviter_id":     inviter.Id,
			"inviter_source": InviterSourceSourceAttribution,
		})
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected == 0 {
			result.AlreadyConfirmed = true
			result.InviterId = inviter.Id
			return nil
		}

		confirmation := SourceAttributionConfirmation{
			UserId:            user.Id,
			Username:          strings.TrimSpace(username),
			OptionId:          selection.OptionId,
			OptionName:        selection.OptionName,
			TargetUserID:      inviter.Id,
			TargetUsername:    inviter.Username,
			TargetAffCode:     inviter.AffCode,
			SourceDescription: selection.SourceDescription,
			Ip:                selection.Ip,
			UserAgent:         selection.UserAgent,
			ConfirmedAt:       selection.ConfirmedAt,
		}
		if confirmation.Username == "" {
			confirmation.Username = user.Username
		}
		if err := tx.Create(&confirmation).Error; err != nil {
			return err
		}
		result.InviterId = inviter.Id
		result.Confirmation = &confirmation
		return nil
	})
	if err != nil {
		return result, err
	}
	if result.InviterId != 0 {
		_ = invalidateUserCache(userId)
	}
	return result, nil
}

func HasSourceAttribution(userId int) (bool, error) {
	if userId <= 0 {
		return false, nil
	}
	var user User
	if err := DB.Select("inviter_id").First(&user, userId).Error; err != nil {
		return false, err
	}
	return user.InviterId != 0, nil
}

func GetSourceAttributionConfirmations(filter SourceAttributionConfirmationFilter, startIdx int, num int) ([]*SourceAttributionConfirmation, int64, error) {
	if num <= 0 {
		num = common.ItemsPerPage
	}
	if num > 100 {
		num = 100
	}
	tx := DB.Model(&SourceAttributionConfirmation{})
	if filter.UserId > 0 {
		tx = tx.Where("user_id = ?", filter.UserId)
	}
	var err error
	if tx, err = applyExplicitLogTextFilter(tx, "username", strings.TrimSpace(filter.Username)); err != nil {
		return nil, 0, err
	}
	if tx, err = applyExplicitLogTextFilter(tx, "option_id", strings.TrimSpace(filter.OptionId)); err != nil {
		return nil, 0, err
	}
	if filter.TargetUserID > 0 {
		tx = tx.Where("target_inviter_id = ?", filter.TargetUserID)
	}
	if tx, err = applyExplicitLogTextFilter(tx, "target_aff_code", strings.TrimSpace(filter.TargetAffCode)); err != nil {
		return nil, 0, err
	}
	if tx, err = applyLogIPFilter(tx, "ip", filter.Ip); err != nil {
		return nil, 0, err
	}
	if filter.StartTimestamp != 0 {
		tx = tx.Where("confirmed_at >= ?", filter.StartTimestamp)
	}
	if filter.EndTimestamp != 0 {
		tx = tx.Where("confirmed_at <= ?", filter.EndTimestamp)
	}

	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var confirmations []*SourceAttributionConfirmation
	err = tx.Order("confirmed_at desc, id desc").Offset(startIdx).Limit(num).Find(&confirmations).Error
	for _, confirmation := range confirmations {
		confirmation.Ip = NormalizeAuditIP(confirmation.Ip)
	}
	return confirmations, total, err
}
