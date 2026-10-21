package service

import (
	"context"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRedeemCodeAutoDetectsAdministratorUserAndSubscriptionCodes(t *testing.T) {
	t.Run("administrator balance", func(t *testing.T) {
		setupUserRedemptionTestDB(t)
		seedWalletUser(t, 4101, 0)
		legacy := model.Redemption{
			Name: "unified-admin", Key: "41010000000000000000000000000001",
			Status: common.RedemptionCodeStatusEnabled, Quota: 1234, CreatedTime: common.GetTimestamp(),
		}
		require.NoError(t, model.DB.Create(&legacy).Error)

		result, err := RedeemCode(context.Background(), 4101, legacy.Key)
		require.NoError(t, err)
		assert.Equal(t, model.UserRedemptionRewardBalance, result.RewardType)
		assert.Equal(t, "administrator", result.Source)
		assert.EqualValues(t, 1234, result.Quota)
		assert.Zero(t, result.SubscriptionID)
	})

	t.Run("user balance", func(t *testing.T) {
		setupUserRedemptionTestDB(t)
		quota := redemptionQuota(1)
		seedWalletUser(t, 4111, quota*2)
		seedWalletUser(t, 4112, 0)
		code, err := CreateUserRedemptionCode(context.Background(), CreateUserRedemptionInput{
			CreatorUserID: 4111, Kind: model.UserRedemptionKindSingle,
			TotalQuota: quota, MaxClaims: 1,
			ExpiresAt: common.GetTimestamp() + int64((2 * time.Hour).Seconds()),
		})
		require.NoError(t, err)

		result, err := RedeemCode(context.Background(), 4112, code.Code)
		require.NoError(t, err)
		assert.Equal(t, model.UserRedemptionRewardBalance, result.RewardType)
		assert.Equal(t, "user", result.Source)
		assert.EqualValues(t, quota, result.Quota)
		assert.Zero(t, result.SubscriptionID)
	})

	t.Run("subscription", func(t *testing.T) {
		setupUserRedemptionTestDB(t)
		plan := seedSubscriptionRedemptionPlan(t, 1, true, true)
		code := createSubscriptionRedemptionForTest(t, 4121, plan, 1)
		seedWalletUser(t, 4122, 0)

		result, err := RedeemCode(context.Background(), 4122, code.Code)
		require.NoError(t, err)
		assert.Equal(t, model.UserRedemptionRewardSubscription, result.RewardType)
		assert.Equal(t, "user", result.Source)
		assert.Equal(t, subscriptionRedemptionUnitQuota(t, plan.PriceAmount), result.Quota)
		assert.Positive(t, result.SubscriptionID)
	})
}

func TestRedeemWalletCodeRejectsSubscriptionBeforeMutation(t *testing.T) {
	setupUserRedemptionTestDB(t)
	plan := seedSubscriptionRedemptionPlan(t, 1, true, true)
	code := createSubscriptionRedemptionForTest(t, 4131, plan, 1)
	seedWalletUser(t, 4132, 0)

	_, err := RedeemWalletCode(context.Background(), 4132, code.Code)
	require.Error(t, err)
	var mismatch *UserRedemptionRewardTypeMismatchError
	require.ErrorAs(t, err, &mismatch)
	assert.Equal(t, model.UserRedemptionRewardBalance, mismatch.Expected)
	assert.Equal(t, model.UserRedemptionRewardSubscription, mismatch.Actual)

	var claims int64
	require.NoError(t, model.DB.Model(&model.UserRedemptionClaim{}).Where("code_id = ?", code.ID).Count(&claims).Error)
	assert.Zero(t, claims)
}

func TestRedeemCodeDoesNotFallThroughAfterStoredUserCodeFailure(t *testing.T) {
	setupUserRedemptionTestDB(t)
	quota := redemptionQuota(1)
	seedWalletUser(t, 4141, quota*2)
	seedWalletUser(t, 4142, 0)
	code, err := CreateUserRedemptionCode(context.Background(), CreateUserRedemptionInput{
		CreatorUserID: 4141, Kind: model.UserRedemptionKindSingle,
		TotalQuota: quota, MaxClaims: 1,
		ExpiresAt: common.GetTimestamp() + int64((2 * time.Hour).Seconds()),
	})
	require.NoError(t, err)
	require.NoError(t, model.DB.Model(code).Updates(map[string]any{
		"status": model.UserRedemptionStatusDisabled, "disabled_at": common.GetTimestamp(),
	}).Error)

	_, err = RedeemCode(context.Background(), 4142, code.Code)
	require.ErrorIs(t, err, ErrUserRedemptionCodeDisabled)
	var claimant model.User
	require.NoError(t, model.DB.First(&claimant, 4142).Error)
	assert.Zero(t, claimant.Quota)
}
