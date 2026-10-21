package service

import (
	"bytes"
	"encoding/json"
	"log"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
	gormlogger "gorm.io/gorm/logger"
)

func TestCalculateTokenBalanceFiniteSubscriptionConstraint(t *testing.T) {
	result := calculateTokenBalance(tokenBalanceCalculationInput{
		Preference:      "subscription_only",
		TokenRemaining:  100,
		TokenUsed:       50,
		WalletRemaining: 1000,
		Subscriptions: []tokenBalanceSubscriptionInput{
			{Remaining: 60, Used: 40, Total: 100, AllowWalletOverflow: false},
		},
	})

	require.EqualValues(t, 150, result.TokenTotal)
	require.Equal(t, QuotaAmount{Value: 60}, result.CurrentAvailable)
	require.Equal(t, QuotaAmount{Value: 60}, result.TokenCallableQuota)
	require.EqualValues(t, 40, result.Restricted)
	require.Equal(t, TokenBalanceRestrictionSubscription, result.RestrictionReason)
	require.Equal(t, QuotaAmount{Value: 60}, result.SingleRequestLimit)
	require.Equal(t, []string{BillingSourceSubscription}, result.ParticipatingSources)
	require.Nil(t, result.Wallet)
	require.NotNil(t, result.Subscription)
}

func TestResolveTokenBalanceStatusExpiresAtCurrentSecond(t *testing.T) {
	user := &model.UserBase{Status: common.UserStatusEnabled}
	token := &model.Token{
		Status:         common.TokenStatusEnabled,
		ExpiredTime:    100,
		UnlimitedQuota: true,
	}

	require.Equal(t, TokenBalanceStatusExpired, resolveTokenBalanceStatus(token, user, 100))
}

func TestCalculateTokenBalanceDoesNotCombineSubscriptionsForSingleRequest(t *testing.T) {
	result := calculateTokenBalance(tokenBalanceCalculationInput{
		Preference:      "subscription_only",
		TokenUnlimited:  true,
		WalletRemaining: 10,
		Subscriptions: []tokenBalanceSubscriptionInput{
			{Remaining: 20, Total: 20},
			{Remaining: 30, Total: 30},
		},
	})

	require.Equal(t, QuotaAmount{Value: 50}, result.CurrentAvailable)
	require.Equal(t, QuotaAmount{Value: 50}, result.TokenCallableQuota)
	require.Equal(t, QuotaAmount{Value: 30}, result.SingleRequestLimit)
}

func TestCalculateTokenBalanceSubscriptionFirstHonorsStrictFallback(t *testing.T) {
	result := calculateTokenBalance(tokenBalanceCalculationInput{
		Preference:      "subscription_first",
		TokenUnlimited:  true,
		WalletRemaining: 100,
		Subscriptions: []tokenBalanceSubscriptionInput{
			{Remaining: 30, Total: 30, AllowWalletOverflow: false},
		},
	})

	require.Equal(t, QuotaAmount{Value: 30}, result.CurrentAvailable)
	require.Equal(t, QuotaAmount{Value: 30}, result.TokenCallableQuota)
	require.Equal(t, []string{BillingSourceSubscription}, result.ParticipatingSources)
	require.Nil(t, result.Wallet)
}

func TestCalculateTokenBalanceSubscriptionFirstIncludesAllowedWalletFallback(t *testing.T) {
	result := calculateTokenBalance(tokenBalanceCalculationInput{
		Preference:      "subscription_first",
		TokenUnlimited:  true,
		WalletRemaining: 100,
		Subscriptions: []tokenBalanceSubscriptionInput{
			{Remaining: 30, Total: 30, AllowWalletOverflow: true},
		},
	})

	require.Equal(t, QuotaAmount{Value: 130}, result.CurrentAvailable)
	require.Equal(t, QuotaAmount{Value: 130}, result.TokenCallableQuota)
	require.Equal(t, QuotaAmount{Value: 100}, result.SingleRequestLimit)
	require.Equal(t, []string{BillingSourceSubscription, BillingSourceWallet}, result.ParticipatingSources)
	require.NotNil(t, result.Wallet)
}

func TestCalculateTokenBalanceWalletFirstFallsBackToSingleSubscription(t *testing.T) {
	result := calculateTokenBalance(tokenBalanceCalculationInput{
		Preference:      "wallet_first",
		TokenUnlimited:  true,
		WalletRemaining: 10,
		Subscriptions: []tokenBalanceSubscriptionInput{
			{Remaining: 20, Total: 20},
			{Remaining: 30, Total: 30},
		},
	})

	require.Equal(t, QuotaAmount{Value: 60}, result.CurrentAvailable)
	require.Equal(t, QuotaAmount{Value: 60}, result.TokenCallableQuota)
	require.Equal(t, QuotaAmount{Value: 30}, result.SingleRequestLimit)
	require.Equal(t, []string{BillingSourceWallet, BillingSourceSubscription}, result.ParticipatingSources)
}

func TestCalculateTokenBalanceWalletOnlyOmitsSubscription(t *testing.T) {
	result := calculateTokenBalance(tokenBalanceCalculationInput{
		Preference:      "wallet_only",
		TokenRemaining:  100,
		WalletRemaining: 80,
		Subscriptions: []tokenBalanceSubscriptionInput{
			{Remaining: 1000, Total: 1000},
		},
	})

	require.Equal(t, QuotaAmount{Value: 80}, result.CurrentAvailable)
	require.Equal(t, QuotaAmount{Value: 80}, result.TokenCallableQuota)
	require.EqualValues(t, 20, result.Restricted)
	require.Equal(t, TokenBalanceRestrictionWallet, result.RestrictionReason)
	require.NotNil(t, result.Wallet)
	require.Nil(t, result.Subscription)
}

func TestCalculateTokenBalanceWalletDoesNotExposeUnknownHistoricalTotals(t *testing.T) {
	result := calculateTokenBalance(tokenBalanceCalculationInput{
		Preference:      "wallet_only",
		TokenUnlimited:  true,
		WalletRemaining: 80,
	})

	serialized, err := json.Marshal(result)
	require.NoError(t, err)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(serialized, &payload))
	wallet, ok := payload["wallet"].(map[string]any)
	require.True(t, ok)
	require.Contains(t, wallet, "available")
	require.NotContains(t, wallet, "used")
	require.NotContains(t, wallet, "total")
}

func TestCalculateTokenBalancePreservesUnlimitedWithoutMagicNumber(t *testing.T) {
	result := calculateTokenBalance(tokenBalanceCalculationInput{
		Preference:     "subscription_only",
		TokenUnlimited: true,
		Subscriptions: []tokenBalanceSubscriptionInput{
			{Unlimited: true, Used: 500},
		},
	})

	require.True(t, result.CurrentAvailable.Unlimited)
	require.Zero(t, result.CurrentAvailable.Value)
	require.True(t, result.TokenCallableQuota.Unlimited)
	require.Zero(t, result.TokenCallableQuota.Value)
	require.True(t, result.SingleRequestLimit.Unlimited)
	require.Zero(t, result.Restricted)
}

func TestCalculateTokenBalanceFiniteKeyCapsUnlimitedFunding(t *testing.T) {
	result := calculateTokenBalance(tokenBalanceCalculationInput{
		Preference:     "subscription_only",
		TokenRemaining: 75,
		TokenUsed:      25,
		Subscriptions: []tokenBalanceSubscriptionInput{
			{Unlimited: true},
		},
	})

	require.Equal(t, QuotaAmount{Value: 75}, result.CurrentAvailable)
	require.Equal(t, QuotaAmount{Value: 75}, result.TokenCallableQuota)
	require.Equal(t, QuotaAmount{Value: 75}, result.SingleRequestLimit)
	require.Zero(t, result.Restricted)
}

func TestCalculateTokenBalanceSaturatesAggregateValues(t *testing.T) {
	result := calculateTokenBalance(tokenBalanceCalculationInput{
		Preference:     "subscription_only",
		TokenUnlimited: true,
		Subscriptions: []tokenBalanceSubscriptionInput{
			{Remaining: math.MaxInt64 - 5, Total: math.MaxInt64 - 5},
			{Remaining: 10, Total: 10},
		},
	})

	require.EqualValues(t, math.MaxInt64, result.CurrentAvailable.Value)
	require.EqualValues(t, math.MaxInt64, result.TokenCallableQuota.Value)
}

func TestTokenBalanceSubscriptionInputsFollowOfficialBillingAcrossGroups(t *testing.T) {
	subscriptions := []model.UserSubscription{
		{AmountTotal: 100, AmountUsed: 20, UpgradeGroup: "standard"},
		{AmountTotal: 200, AmountUsed: 50, UpgradeGroup: "pro"},
	}

	inputs := tokenBalanceSubscriptionInputs(subscriptions, time.Now().Unix())

	require.Len(t, inputs, 2)
	require.EqualValues(t, 80, inputs[0].Remaining)
	require.EqualValues(t, 150, inputs[1].Remaining)
}

func TestTokenBalanceSubscriptionInputsReflectDueResetWithoutMutation(t *testing.T) {
	now := time.Now().Unix()
	subscription := model.UserSubscription{
		AmountTotal:   100,
		AmountUsed:    70,
		NextResetTime: now,
	}

	inputs := tokenBalanceSubscriptionInputs([]model.UserSubscription{subscription}, now)

	require.Len(t, inputs, 1)
	require.Zero(t, inputs[0].Used)
	require.EqualValues(t, 100, inputs[0].Remaining)
	require.EqualValues(t, 70, subscription.AmountUsed)
}

func TestBuildTokenBalanceSnapshotReturnsRedactedGroupScopedBalance(t *testing.T) {
	truncate(t)
	if !model.DB.Migrator().HasTable(&model.SubscriptionPlan{}) {
		require.NoError(t, model.DB.AutoMigrate(&model.SubscriptionPlan{}))
	}
	t.Cleanup(func() { model.DB.Exec("DELETE FROM subscription_plans") })

	now := time.Now().Unix()
	user := &model.User{
		Id:        9910,
		Username:  "private-balance-user",
		Email:     "private@example.com",
		Quota:     1000,
		UsedQuota: 900,
		Status:    common.UserStatusEnabled,
		Group:     "pro",
	}
	user.SetSetting(dto.UserSetting{BillingPreference: "subscription_only"})
	require.NoError(t, model.DB.Create(user).Error)
	require.NoError(t, model.DB.Create(&model.Token{
		Id:                 9911,
		UserId:             user.Id,
		Key:                "balancekey9911",
		Status:             common.TokenStatusEnabled,
		ExpiredTime:        now + 86400,
		RemainQuota:        100,
		UsedQuota:          50,
		Group:              "pro",
		ModelLimitsEnabled: true,
		ModelLimits:        "gpt-z,gpt-a,gpt-a",
	}).Error)
	plan := &model.SubscriptionPlan{
		Id:               9912,
		Title:            "Private Pro",
		DurationUnit:     model.SubscriptionDurationMonth,
		DurationValue:    1,
		TotalAmount:      100,
		QuotaResetPeriod: model.SubscriptionResetNever,
	}
	require.NoError(t, model.DB.Create(plan).Error)
	require.NoError(t, model.DB.Create(&model.UserSubscription{
		Id:                  9913,
		UserId:              user.Id,
		PlanId:              plan.Id,
		AmountTotal:         100,
		AmountUsed:          40,
		StartTime:           now - 60,
		EndTime:             now + 86400,
		Status:              "active",
		UpgradeGroup:        "pro",
		AllowWalletOverflow: false,
	}).Error)
	require.NoError(t, model.DB.Create(&model.UserSubscription{
		Id:                  9914,
		UserId:              user.Id,
		PlanId:              plan.Id,
		AmountTotal:         1000,
		AmountUsed:          0,
		StartTime:           now - 60,
		EndTime:             now + 86400,
		Status:              "active",
		UpgradeGroup:        "standard",
		AllowWalletOverflow: true,
	}).Error)

	snapshot, err := BuildTokenBalanceSnapshot("sk-balancekey9911")

	require.NoError(t, err)
	require.Equal(t, TokenBalanceStatusActive, snapshot.Status)
	require.NotContains(t, snapshot.MaskedKey, "balancekey9911")
	require.Equal(t, "fixed", snapshot.Group.Mode)
	require.Equal(t, []string{"pro"}, snapshot.Group.EffectiveGroups)
	require.Equal(t, []string{"gpt-a", "gpt-z"}, snapshot.ModelLimits.Models)
	require.Equal(t, QuotaAmount{Value: 60}, snapshot.Balance.CurrentAvailable)
	require.Equal(t, QuotaAmount{Value: 60}, snapshot.Balance.TokenCallableQuota)
	require.EqualValues(t, 40, snapshot.Balance.Restricted)
	require.Nil(t, snapshot.Balance.Wallet)
	require.NotNil(t, snapshot.Balance.Subscription)

	serialized, err := json.Marshal(snapshot)
	require.NoError(t, err)
	response := string(serialized)
	for _, secret := range []string{"balancekey9911", user.Username, user.Email, "user_id", "subscription_id"} {
		require.False(t, strings.Contains(response, secret), response)
	}
}

func TestBuildTokenBalanceSnapshotClearsCallableQuotaWhenKeyUnavailable(t *testing.T) {
	truncate(t)

	now := time.Now().Unix()
	user := &model.User{
		Id:       9920,
		Username: "disabled-balance-user",
		Email:    "disabled-balance@example.com",
		Quota:    1000,
		Status:   common.UserStatusEnabled,
		Group:    "default",
	}
	user.SetSetting(dto.UserSetting{BillingPreference: "wallet_only"})
	require.NoError(t, model.DB.Create(user).Error)
	require.NoError(t, model.DB.Create(&model.Token{
		Id:          9921,
		UserId:      user.Id,
		Key:         "disabledbalancekey9921",
		Status:      common.TokenStatusDisabled,
		ExpiredTime: now + 86400,
		RemainQuota: 100,
		UsedQuota:   50,
		Group:       "default",
	}).Error)

	snapshot, err := BuildTokenBalanceSnapshot("sk-disabledbalancekey9921")

	require.NoError(t, err)
	require.Equal(t, TokenBalanceStatusDisabled, snapshot.Status)
	require.Equal(t, QuotaAmount{}, snapshot.Balance.CurrentAvailable)
	require.Equal(t, QuotaAmount{}, snapshot.Balance.TokenCallableQuota)
	require.Equal(t, QuotaAmount{}, snapshot.Balance.SingleRequestLimit)
	require.EqualValues(t, 100, snapshot.Balance.Restricted)
	require.Equal(t, TokenBalanceRestrictionUnavailable, snapshot.Balance.RestrictionReason)
	require.Nil(t, snapshot.Balance.Wallet)
	require.Nil(t, snapshot.Balance.Subscription)
}

func TestBuildTokenBalanceSnapshotReturnsGenericErrorForUnknownKey(t *testing.T) {
	truncate(t)

	snapshot, err := BuildTokenBalanceSnapshot("sk-key-that-does-not-exist")

	require.ErrorIs(t, err, ErrTokenBalanceKeyInvalid)
	require.Nil(t, snapshot)
}

func TestBuildTokenBalanceSnapshotDoesNotTruncateHyphenatedKey(t *testing.T) {
	truncate(t)

	user := &model.User{
		Id:       9930,
		Username: "hyphen-balance-user",
		Quota:    1000,
		Status:   common.UserStatusEnabled,
		Group:    "default",
	}
	user.SetSetting(dto.UserSetting{BillingPreference: "wallet_only"})
	require.NoError(t, model.DB.Create(user).Error)
	require.NoError(t, model.DB.Create(&model.Token{
		Id:          9931,
		UserId:      user.Id,
		Key:         "abc",
		Status:      common.TokenStatusEnabled,
		ExpiredTime: time.Now().Unix() + 86400,
		RemainQuota: 100,
		Group:       "default",
	}).Error)

	snapshot, err := BuildTokenBalanceSnapshot("abc-def-ghi")

	require.ErrorIs(t, err, ErrTokenBalanceKeyInvalid)
	require.Nil(t, snapshot)
}

func TestBuildTokenBalanceSnapshotSupportsSkPrefixFallbackWithoutHyphenTruncation(t *testing.T) {
	truncate(t)

	user := &model.User{
		Id:       9940,
		Username: "prefix-balance-user",
		Quota:    1000,
		Status:   common.UserStatusEnabled,
		Group:    "default",
	}
	user.SetSetting(dto.UserSetting{BillingPreference: "wallet_only"})
	require.NoError(t, model.DB.Create(user).Error)
	require.NoError(t, model.DB.Create(&model.Token{
		Id:          9941,
		UserId:      user.Id,
		Key:         "custom_key-123456",
		Status:      common.TokenStatusEnabled,
		ExpiredTime: time.Now().Unix() + 86400,
		RemainQuota: 100,
		Group:       "default",
	}).Error)

	snapshot, err := BuildTokenBalanceSnapshot("sk-custom_key-123456")

	require.NoError(t, err)
	require.NotNil(t, snapshot)
	require.Equal(t, TokenBalanceStatusActive, snapshot.Status)
}

func TestBuildTokenBalanceSnapshotPreservesCustomKeyPrefixInMaskedKey(t *testing.T) {
	truncate(t)

	user := &model.User{
		Id:       9950,
		Username: "custom-prefix-balance-user",
		Quota:    1000,
		Status:   common.UserStatusEnabled,
		Group:    "default",
	}
	user.SetSetting(dto.UserSetting{BillingPreference: "wallet_only"})
	require.NoError(t, model.DB.Create(user).Error)
	require.NoError(t, model.DB.Create(&model.Token{
		Id:          9951,
		UserId:      user.Id,
		Key:         "custom_key-123456",
		KeyFormat:   model.TokenKeyFormatFull,
		Status:      common.TokenStatusEnabled,
		ExpiredTime: time.Now().Unix() + 86400,
		RemainQuota: 100,
		Group:       "default",
	}).Error)

	snapshot, err := BuildTokenBalanceSnapshot("custom_key-123456")

	require.NoError(t, err)
	require.Equal(t, "cust**********3456", snapshot.MaskedKey)
}

func TestBuildTokenBalanceSnapshotDoesNotLogInvalidKey(t *testing.T) {
	truncate(t)

	const keyBody = "privateinvalidbalancekey"
	var output bytes.Buffer
	originalLogger := model.DB.Logger
	model.DB.Logger = gormlogger.New(log.New(&output, "", 0), gormlogger.Config{
		LogLevel:                  gormlogger.Info,
		IgnoreRecordNotFoundError: false,
		Colorful:                  false,
	})
	t.Cleanup(func() { model.DB.Logger = originalLogger })

	snapshot, err := BuildTokenBalanceSnapshot("sk-" + keyBody)

	require.ErrorIs(t, err, ErrTokenBalanceKeyInvalid)
	require.Nil(t, snapshot)
	require.NotContains(t, output.String(), keyBody)
}
