package service

import (
	"context"
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

var (
	ErrWalletRedemptionInvalid   = errors.New("wallet redemption request is invalid")
	ErrWalletRedemptionAmbiguous = errors.New("wallet redemption code is ambiguous")
)

type WalletRedemptionResult struct {
	Quota    int64
	CodeType string
}

type RedemptionClaimResult struct {
	RewardType     string `json:"reward_type"`
	Source         string `json:"source"`
	Quota          int64  `json:"quota"`
	SubscriptionID int64  `json:"subscription_id"`
}

func RedeemCode(ctx context.Context, userID int, code string) (*RedemptionClaimResult, error) {
	return redeemCodeWithExpectation(ctx, userID, code, "")
}

func redeemCodeWithExpectation(ctx context.Context, userID int, code, expectedRewardType string) (*RedemptionClaimResult, error) {
	if model.DB == nil || userID <= 0 {
		return nil, ErrWalletRedemptionInvalid
	}
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, ErrWalletRedemptionInvalid
	}

	var userCode model.UserRedemptionCode
	userCodeErr := model.DB.WithContext(ctx).Where("code = ?", code).First(&userCode).Error
	if userCodeErr != nil && !errors.Is(userCodeErr, gorm.ErrRecordNotFound) {
		return nil, userCodeErr
	}

	var legacyCode model.Redemption
	legacyCodeErr := model.DB.WithContext(ctx).Where(&model.Redemption{Key: code}).First(&legacyCode).Error
	if legacyCodeErr != nil && !errors.Is(legacyCodeErr, gorm.ErrRecordNotFound) {
		return nil, legacyCodeErr
	}
	if userCodeErr == nil && legacyCodeErr == nil {
		return nil, ErrWalletRedemptionAmbiguous
	}

	if userCodeErr == nil {
		if expectedRewardType != "" && userCode.RewardType != expectedRewardType {
			return nil, &UserRedemptionRewardTypeMismatchError{
				Expected: expectedRewardType,
				Actual:   userCode.RewardType,
			}
		}
		claim, err := ClaimUserRedemptionCodeWithInput(ctx, ClaimUserRedemptionInput{
			ClaimerUserID:      userID,
			Code:               code,
			ExpectedRewardType: expectedRewardType,
		})
		if err != nil {
			return nil, err
		}
		return &RedemptionClaimResult{
			RewardType:     claim.RewardType,
			Source:         "user",
			Quota:          claim.Quota,
			SubscriptionID: claim.UserSubscriptionID,
		}, nil
	}
	if legacyCodeErr == nil {
		if expectedRewardType != "" && expectedRewardType != model.UserRedemptionRewardBalance {
			return nil, &UserRedemptionRewardTypeMismatchError{
				Expected: expectedRewardType,
				Actual:   model.UserRedemptionRewardBalance,
			}
		}
		quota, err := model.Redeem(code, userID)
		if err != nil {
			return nil, err
		}
		return &RedemptionClaimResult{
			RewardType: model.UserRedemptionRewardBalance,
			Source:     "administrator",
			Quota:      int64(quota),
		}, nil
	}
	return nil, ErrWalletRedemptionInvalid
}

func RedeemWalletCode(ctx context.Context, userID int, code string) (*WalletRedemptionResult, error) {
	result, err := redeemCodeWithExpectation(ctx, userID, code, model.UserRedemptionRewardBalance)
	if err != nil {
		return nil, err
	}
	codeType := "user"
	if result.Source == "administrator" {
		codeType = "legacy"
	}
	return &WalletRedemptionResult{Quota: result.Quota, CodeType: codeType}, nil
}
