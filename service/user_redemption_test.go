package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/user_redemption_setting"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupUserRedemptionTestDB(t *testing.T) {
	t.Helper()
	prevDB := model.DB
	prevLogDB := model.LOG_DB
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:user_redemption_%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	model.DB = db
	model.LOG_DB = db
	t.Cleanup(func() {
		model.DB = prevDB
		model.LOG_DB = prevLogDB
		_ = sqlDB.Close()
	})
	require.NoError(t, db.AutoMigrate(
		&model.User{},
		&model.Checkin{},
		&model.Redemption{},
		&model.Log{},
		&model.WalletLedgerEntry{},
		&model.SubscriptionPlan{},
		&model.UserSubscription{},
		&model.SubscriptionPurchaseCounter{},
		&model.UserRedemptionCode{},
		&model.UserRedemptionClaim{},
		&model.UserRedemptionSubscriptionEntitlement{},
		&model.InviteCode{},
		&model.InviteCodeUse{},
		&model.InviteRewardPayout{},
		&model.AuditOutboxEvent{},
	))
}

func setInvitationCommissionTestSettings(t *testing.T) {
	t.Helper()
	previous := *operation_setting.GetInvitationIncentiveSetting()
	setting := operation_setting.DefaultInvitationIncentiveSetting()
	setting.AdminSubscriptionRedemptionCommissionEnabled = true
	setting.AdminSubscriptionRedemptionCommissionRate = "10.00"
	setting.AdminBalanceRedemptionCommissionEnabled = true
	setting.AdminBalanceRedemptionCommissionRate = "10.00"
	*operation_setting.GetInvitationIncentiveSetting() = setting
	t.Cleanup(func() {
		*operation_setting.GetInvitationIncentiveSetting() = previous
	})
}

func seedInvitedRedemptionUsers(t *testing.T, inviterID int, inviteeIDs ...int) {
	t.Helper()
	inviter := model.User{
		Id: inviterID, Username: fmt.Sprintf("inviter-%d", inviterID),
		AffCode: fmt.Sprintf("inviter-aff-%d", inviterID), Role: common.RoleCommonUser,
		Status: common.UserStatusEnabled,
	}
	require.NoError(t, model.DB.Create(&inviter).Error)
	code := model.InviteCode{
		OwnerUserID: inviterID, Code: fmt.Sprintf("INVITE-%d", inviterID),
		Kind: model.InviteCodeKindMulti, MaxUses: len(inviteeIDs), UsedCount: len(inviteeIDs),
		Status: model.InviteCodeStatusActive, IssuedBy: inviterID,
	}
	require.NoError(t, model.DB.Create(&code).Error)
	for _, inviteeID := range inviteeIDs {
		invitee := model.User{
			Id: inviteeID, Username: fmt.Sprintf("invitee-%d", inviteeID),
			AffCode: fmt.Sprintf("invitee-aff-%d", inviteeID), Role: common.RoleCommonUser,
			Status: common.UserStatusEnabled, InviterId: inviterID,
			InviterSource: model.InviterSourceInviteCode,
		}
		require.NoError(t, model.DB.Create(&invitee).Error)
		require.NoError(t, model.DB.Create(&model.InviteCodeUse{
			InviteCodeID: code.ID, InviterUserID: inviterID, RegisteredUserID: inviteeID,
			Source: "password", InviterRewardStatus: model.InviteRewardStatusSkipped,
			InviteeRewardStatus: model.InviteRewardStatusSkipped,
			AuditEventID:        fmt.Sprintf("invite-use-%d", inviteeID),
		}).Error)
	}
}

func seedWalletUser(t *testing.T, id int, quota int64) {
	t.Helper()
	require.NoError(t, model.DB.Create(&model.User{Id: id, Username: fmt.Sprintf("u%d", id), AffCode: fmt.Sprintf("aff%d", id), Quota: quota, Status: common.UserStatusEnabled, Role: common.RoleCommonUser}).Error)
}

func redemptionQuota(amount float64) int64 {
	return int64(amount * common.QuotaPerUnit)
}

func setUserRedemptionTestSettings(t *testing.T, values map[string]string) {
	t.Helper()
	common.OptionMapRWMutex.Lock()
	previous := common.OptionMap
	common.OptionMap = values
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = previous
		common.OptionMapRWMutex.Unlock()
	})
}

func seedSubscriptionRedemptionPlan(t *testing.T, price float64, enabled, allowBalancePay bool) *model.SubscriptionPlan {
	t.Helper()
	plan := &model.SubscriptionPlan{
		Title:               fmt.Sprintf("subscription-plan-%.2f", price),
		Subtitle:            "subscription redemption test",
		PriceAmount:         price,
		Currency:            "USD",
		DurationUnit:        model.SubscriptionDurationMonth,
		DurationValue:       1,
		Enabled:             enabled,
		AllowBalancePay:     common.GetPointer(allowBalancePay),
		AllowWalletOverflow: common.GetPointer(true),
		TotalAmount:         1_000,
		QuotaResetPeriod:    model.SubscriptionResetMonthly,
		MaxPurchasePerUser:  2,
		UpgradeGroup:        "vip",
		DowngradeGroup:      "default",
	}
	require.NoError(t, model.DB.Create(plan).Error)
	if !enabled {
		require.NoError(t, model.DB.Model(plan).Update("enabled", false).Error)
		plan.Enabled = false
	}
	return plan
}

func subscriptionRedemptionUnitQuota(t *testing.T, price float64) int64 {
	t.Helper()
	require.Greater(t, common.QuotaPerUnit, 0.0)
	return int64(math.Ceil(price * common.QuotaPerUnit))
}

func TestCreateSubscriptionRedemptionReservesExactUnits(t *testing.T) {
	setupUserRedemptionTestDB(t)
	setUserRedemptionTestSettings(t, map[string]string{
		user_redemption_setting.EnabledKey:             "true",
		user_redemption_setting.SubscriptionEnabledKey: "true",
	})
	plan := seedSubscriptionRedemptionPlan(t, 1.25, true, true)
	unitQuota := subscriptionRedemptionUnitQuota(t, plan.PriceAmount)
	startingQuota := unitQuota*4 + 17
	seedWalletUser(t, 801, startingQuota)

	code, err := CreateSubscriptionRedemptionCode(context.Background(), CreateSubscriptionRedemptionInput{
		CreatorUserID: 801,
		PlanID:        plan.Id,
		Units:         3,
		ExpiresAt:     common.GetTimestamp() + int64((2 * time.Hour).Seconds()),
	})

	require.NoError(t, err)
	require.Equal(t, model.UserRedemptionRewardSubscription, code.RewardType)
	require.Equal(t, model.UserRedemptionKindFixed, code.Kind)
	require.Equal(t, 3, code.MaxClaims)
	require.Equal(t, unitQuota*3, code.TotalQuota)
	var creator model.User
	require.NoError(t, model.DB.First(&creator, 801).Error)
	require.Equal(t, startingQuota-code.TotalQuota, creator.Quota)
	var entitlement model.UserRedemptionSubscriptionEntitlement
	require.NoError(t, model.DB.Where("code_id = ?", code.ID).First(&entitlement).Error)
	require.Equal(t, plan.Id, entitlement.PlanID)
	require.Equal(t, unitQuota, entitlement.UnitPaidQuota)
	require.Equal(t, plan.Title, entitlement.PlanTitle)
	require.Equal(t, plan.QuotaResetPeriod, entitlement.QuotaResetPeriod)
	var subscriptionCount int64
	require.NoError(t, model.DB.Model(&model.UserSubscription{}).Where("user_id = ?", 801).Count(&subscriptionCount).Error)
	require.Zero(t, subscriptionCount)
}

func TestOfficialSubscriptionCommissionUsesIssuerSnapshotAndCreatesPerClaimRewards(t *testing.T) {
	setupUserRedemptionTestDB(t)
	setInvitationCommissionTestSettings(t)
	setUserRedemptionTestSettings(t, map[string]string{
		user_redemption_setting.EnabledKey:             "true",
		user_redemption_setting.SubscriptionEnabledKey: "true",
	})
	plan := seedSubscriptionRedemptionPlan(t, 1, true, true)
	unitQuota := subscriptionRedemptionUnitQuota(t, plan.PriceAmount)
	model.InvalidateSubscriptionPlanCache(plan.Id)
	admin := model.User{
		Id: 850, Username: "official-issuer", AffCode: "official-issuer-aff",
		Role: common.RoleAdminUser, Status: common.UserStatusEnabled, Quota: unitQuota*2 + 1,
	}
	require.NoError(t, model.DB.Create(&admin).Error)
	seedInvitedRedemptionUsers(t, 851, 852, 853)

	code, err := CreateSubscriptionRedemptionCode(context.Background(), CreateSubscriptionRedemptionInput{
		CreatorUserID: admin.Id, PlanID: plan.Id, Units: 2,
		ExpiresAt: common.GetTimestamp() + int64((2 * time.Hour).Seconds()),
	})
	require.NoError(t, err)
	require.True(t, code.OfficialIssuer)
	require.Equal(t, admin.Id, code.IssuedBy)
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", admin.Id).Update("role", common.RoleCommonUser).Error)

	first, err := claimSubscriptionRedemptionForTest(852, code.Code)
	require.NoError(t, err)
	second, err := claimSubscriptionRedemptionForTest(853, code.Code)
	require.NoError(t, err)
	var rewards []model.InviteRewardPayout
	require.NoError(t, model.DB.Where("trigger_type = ?", model.InviteRewardTriggerAdminSubscriptionClaim).Order("id asc").Find(&rewards).Error)
	require.Len(t, rewards, 2)
	require.Equal(t, []string{fmt.Sprintf("%d", first.ID), fmt.Sprintf("%d", second.ID)}, []string{rewards[0].TriggerID, rewards[1].TriggerID})
	for _, reward := range rewards {
		require.Equal(t, unitQuota, reward.EligibleQuota)
		require.Equal(t, unitQuota/10, reward.Amount)
	}
}

func TestOfficialSubscriptionCommissionDoesNotFollowLaterRoleUpgrade(t *testing.T) {
	setupUserRedemptionTestDB(t)
	setInvitationCommissionTestSettings(t)
	setUserRedemptionTestSettings(t, map[string]string{
		user_redemption_setting.EnabledKey:             "true",
		user_redemption_setting.SubscriptionEnabledKey: "true",
	})
	plan := seedSubscriptionRedemptionPlan(t, 1, true, true)
	unitQuota := subscriptionRedemptionUnitQuota(t, plan.PriceAmount)
	creator := model.User{
		Id: 860, Username: "user-issuer", AffCode: "user-issuer-aff",
		Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Quota: unitQuota + 1,
	}
	require.NoError(t, model.DB.Create(&creator).Error)
	seedInvitedRedemptionUsers(t, 861, 862)
	code, err := CreateSubscriptionRedemptionCode(context.Background(), CreateSubscriptionRedemptionInput{
		CreatorUserID: creator.Id, PlanID: plan.Id, Units: 1,
		ExpiresAt: common.GetTimestamp() + int64((2 * time.Hour).Seconds()),
	})
	require.NoError(t, err)
	require.False(t, code.OfficialIssuer)
	require.Equal(t, creator.Id, code.IssuedBy)
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", creator.Id).Update("role", common.RoleAdminUser).Error)

	_, err = claimSubscriptionRedemptionForTest(862, code.Code)
	require.NoError(t, err)
	var rewardCount int64
	require.NoError(t, model.DB.Model(&model.InviteRewardPayout{}).Count(&rewardCount).Error)
	require.Zero(t, rewardCount)
}

func TestInvitationCommissionSourceMatrixUserBalanceCodeDoesNotReward(t *testing.T) {
	setupUserRedemptionTestDB(t)
	setInvitationCommissionTestSettings(t)
	creatorQuota := redemptionQuota(2)
	seedWalletUser(t, 870, creatorQuota)
	seedInvitedRedemptionUsers(t, 871, 872)
	code, err := CreateUserRedemptionCode(context.Background(), CreateUserRedemptionInput{
		CreatorUserID: 870, Kind: model.UserRedemptionKindSingle, TotalQuota: redemptionQuota(1),
		ExpiresAt: common.GetTimestamp() + int64((2 * time.Hour).Seconds()),
	})
	require.NoError(t, err)
	_, err = ClaimUserRedemptionCode(context.Background(), 872, code.Code)
	require.NoError(t, err)
	var rewardCount int64
	require.NoError(t, model.DB.Model(&model.InviteRewardPayout{}).Count(&rewardCount).Error)
	require.Zero(t, rewardCount)
}

func TestCreateSubscriptionRedemptionRejectsDisabledFeature(t *testing.T) {
	setupUserRedemptionTestDB(t)
	setUserRedemptionTestSettings(t, map[string]string{
		user_redemption_setting.EnabledKey:             "true",
		user_redemption_setting.SubscriptionEnabledKey: "false",
	})
	plan := seedSubscriptionRedemptionPlan(t, 1, true, true)
	seedWalletUser(t, 802, subscriptionRedemptionUnitQuota(t, plan.PriceAmount))

	_, err := CreateSubscriptionRedemptionCode(context.Background(), CreateSubscriptionRedemptionInput{
		CreatorUserID: 802,
		PlanID:        plan.Id,
		Units:         1,
		ExpiresAt:     common.GetTimestamp() + int64((2 * time.Hour).Seconds()),
	})

	require.ErrorIs(t, err, ErrUserRedemptionModeDisabled)
}

func TestCreateSubscriptionRedemptionRejectsDisabledPlan(t *testing.T) {
	setupUserRedemptionTestDB(t)
	plan := seedSubscriptionRedemptionPlan(t, 1, false, true)
	seedWalletUser(t, 803, subscriptionRedemptionUnitQuota(t, plan.PriceAmount))

	_, err := CreateSubscriptionRedemptionCode(context.Background(), CreateSubscriptionRedemptionInput{
		CreatorUserID: 803,
		PlanID:        plan.Id,
		Units:         1,
		ExpiresAt:     common.GetTimestamp() + int64((2 * time.Hour).Seconds()),
	})

	require.ErrorIs(t, err, ErrUserRedemptionPlanDisabled)
}

func TestCreateSubscriptionRedemptionRejectsPlanWithoutBalancePayment(t *testing.T) {
	setupUserRedemptionTestDB(t)
	plan := seedSubscriptionRedemptionPlan(t, 1, true, false)
	seedWalletUser(t, 804, subscriptionRedemptionUnitQuota(t, plan.PriceAmount))

	_, err := CreateSubscriptionRedemptionCode(context.Background(), CreateSubscriptionRedemptionInput{
		CreatorUserID: 804,
		PlanID:        plan.Id,
		Units:         1,
		ExpiresAt:     common.GetTimestamp() + int64((2 * time.Hour).Seconds()),
	})

	require.ErrorIs(t, err, ErrUserRedemptionBalancePaymentDisabled)
}

func TestCreateSubscriptionRedemptionAllowsFreePlanWithoutWalletEntry(t *testing.T) {
	setupUserRedemptionTestDB(t)
	plan := seedSubscriptionRedemptionPlan(t, 0, true, true)
	seedWalletUser(t, 805, 0)

	code, err := CreateSubscriptionRedemptionCode(context.Background(), CreateSubscriptionRedemptionInput{
		CreatorUserID: 805,
		PlanID:        plan.Id,
		Units:         2,
		ExpiresAt:     common.GetTimestamp() + int64((2 * time.Hour).Seconds()),
	})

	require.NoError(t, err)
	require.Zero(t, code.TotalQuota)
	var entitlement model.UserRedemptionSubscriptionEntitlement
	require.NoError(t, model.DB.Where("code_id = ?", code.ID).First(&entitlement).Error)
	require.Zero(t, entitlement.UnitPaidQuota)
	var ledgerCount int64
	require.NoError(t, model.DB.Model(&model.WalletLedgerEntry{}).Where("user_id = ?", 805).Count(&ledgerCount).Error)
	require.Zero(t, ledgerCount)
}

func TestCreateSubscriptionRedemptionReportsInsufficientWallet(t *testing.T) {
	setupUserRedemptionTestDB(t)
	plan := seedSubscriptionRedemptionPlan(t, 1.5, true, true)
	unitQuota := subscriptionRedemptionUnitQuota(t, plan.PriceAmount)
	seedWalletUser(t, 806, unitQuota*2-1)

	_, err := CreateSubscriptionRedemptionCode(context.Background(), CreateSubscriptionRedemptionInput{
		CreatorUserID: 806,
		PlanID:        plan.Id,
		Units:         2,
		ExpiresAt:     common.GetTimestamp() + int64((2 * time.Hour).Seconds()),
	})

	var balanceErr *UserRedemptionInsufficientBalanceError
	require.ErrorAs(t, err, &balanceErr)
	require.Equal(t, unitQuota*2, balanceErr.Requested)
	require.Equal(t, unitQuota*2-1, balanceErr.Available)
}

func TestCreateSubscriptionRedemptionEnforcesConfiguredUnitLimit(t *testing.T) {
	setupUserRedemptionTestDB(t)
	setUserRedemptionTestSettings(t, map[string]string{
		user_redemption_setting.EnabledKey:             "true",
		user_redemption_setting.SubscriptionEnabledKey: "true",
		user_redemption_setting.MaxClaimsPerCodeKey:    "2",
	})
	plan := seedSubscriptionRedemptionPlan(t, 1, true, true)
	seedWalletUser(t, 807, subscriptionRedemptionUnitQuota(t, plan.PriceAmount)*3)

	_, err := CreateSubscriptionRedemptionCode(context.Background(), CreateSubscriptionRedemptionInput{
		CreatorUserID: 807,
		PlanID:        plan.Id,
		Units:         3,
		ExpiresAt:     common.GetTimestamp() + int64((2 * time.Hour).Seconds()),
	})

	require.ErrorIs(t, err, ErrUserRedemptionClaimsLimit)
}

func TestSubscriptionRedemptionCheckedQuotaMultiplicationRejectsOverflow(t *testing.T) {
	_, err := multiplySubscriptionRedemptionQuota(math.MaxInt64, 2)
	require.ErrorIs(t, err, ErrUserRedemptionInvalid)
}

func createSubscriptionRedemptionForTest(t *testing.T, creatorID int, plan *model.SubscriptionPlan, units int) *model.UserRedemptionCode {
	t.Helper()
	unitQuota := subscriptionRedemptionUnitQuota(t, plan.PriceAmount)
	seedWalletUser(t, creatorID, unitQuota*int64(units)+1)
	code, err := CreateSubscriptionRedemptionCode(context.Background(), CreateSubscriptionRedemptionInput{
		CreatorUserID: creatorID,
		PlanID:        plan.Id,
		Units:         units,
		ExpiresAt:     common.GetTimestamp() + int64((2 * time.Hour).Seconds()),
	})
	require.NoError(t, err)
	return code
}

func claimSubscriptionRedemptionForTest(userID int, code string) (*model.UserRedemptionClaim, error) {
	return ClaimUserRedemptionCodeWithInput(context.Background(), ClaimUserRedemptionInput{
		ClaimerUserID:      userID,
		Code:               code,
		ExpectedRewardType: model.UserRedemptionRewardSubscription,
	})
}

func TestSubscriptionClaimCreatesSubscriptionFromEntitlementSnapshot(t *testing.T) {
	setupUserRedemptionTestDB(t)
	plan := seedSubscriptionRedemptionPlan(t, 1.25, true, true)
	code := createSubscriptionRedemptionForTest(t, 811, plan, 2)
	seedWalletUser(t, 812, 0)
	require.NoError(t, model.DB.Model(plan).Updates(map[string]any{
		"duration_value":        6,
		"total_amount":          9_999,
		"quota_reset_period":    model.SubscriptionResetDaily,
		"allow_wallet_overflow": false,
		"upgrade_group":         "changed",
		"downgrade_group":       "changed-back",
		"max_purchase_per_user": 9,
	}).Error)

	claim, err := claimSubscriptionRedemptionForTest(812, code.Code)

	require.NoError(t, err)
	require.Equal(t, model.UserRedemptionRewardSubscription, claim.RewardType)
	require.Equal(t, subscriptionRedemptionUnitQuota(t, 1.25), claim.Quota)
	require.NotZero(t, claim.UserSubscriptionID)
	var subscription model.UserSubscription
	require.NoError(t, model.DB.First(&subscription, claim.UserSubscriptionID).Error)
	require.Equal(t, 812, subscription.UserId)
	require.Equal(t, plan.Id, subscription.PlanId)
	require.Equal(t, "redemption", subscription.Source)
	require.Equal(t, int64(1_000), subscription.AmountTotal)
	require.Equal(t, model.SubscriptionResetMonthly, subscription.QuotaResetPeriod)
	require.True(t, subscription.AllowWalletOverflow)
	require.Equal(t, "vip", subscription.UpgradeGroup)
	require.Equal(t, "default", subscription.DowngradeGroup)
	var claimant model.User
	require.NoError(t, model.DB.First(&claimant, 812).Error)
	require.Equal(t, "vip", claimant.Group)
}

func TestSubscriptionClaimReportsExpectedRewardTypeMismatch(t *testing.T) {
	setupUserRedemptionTestDB(t)
	plan := seedSubscriptionRedemptionPlan(t, 1, true, true)
	code := createSubscriptionRedemptionForTest(t, 813, plan, 1)
	seedWalletUser(t, 814, 0)

	_, err := ClaimUserRedemptionCodeWithInput(context.Background(), ClaimUserRedemptionInput{
		ClaimerUserID:      814,
		Code:               code.Code,
		ExpectedRewardType: model.UserRedemptionRewardBalance,
	})

	var mismatch *UserRedemptionRewardTypeMismatchError
	require.ErrorAs(t, err, &mismatch)
	require.Equal(t, model.UserRedemptionRewardBalance, mismatch.Expected)
	require.Equal(t, model.UserRedemptionRewardSubscription, mismatch.Actual)
	var claimCount int64
	require.NoError(t, model.DB.Model(&model.UserRedemptionClaim{}).Where("code_id = ?", code.ID).Count(&claimCount).Error)
	require.Zero(t, claimCount)
}

func TestSubscriptionClaimDispatchesWithoutExpectedRewardType(t *testing.T) {
	setupUserRedemptionTestDB(t)
	plan := seedSubscriptionRedemptionPlan(t, 1, true, true)
	code := createSubscriptionRedemptionForTest(t, 827, plan, 1)
	seedWalletUser(t, 828, 0)

	claim, err := ClaimUserRedemptionCode(context.Background(), 828, code.Code)

	require.NoError(t, err)
	require.Equal(t, model.UserRedemptionRewardSubscription, claim.RewardType)
	require.NotZero(t, claim.UserSubscriptionID)
}

func TestSubscriptionClaimRejectsRepeatClaimOnActiveCode(t *testing.T) {
	setupUserRedemptionTestDB(t)
	plan := seedSubscriptionRedemptionPlan(t, 1, true, true)
	code := createSubscriptionRedemptionForTest(t, 815, plan, 2)
	seedWalletUser(t, 816, 0)

	_, err := claimSubscriptionRedemptionForTest(816, code.Code)
	require.NoError(t, err)
	_, err = claimSubscriptionRedemptionForTest(816, code.Code)

	require.ErrorIs(t, err, ErrUserRedemptionAlreadyClaimed)
}

func TestSubscriptionClaimRejectsExhaustedCode(t *testing.T) {
	setupUserRedemptionTestDB(t)
	plan := seedSubscriptionRedemptionPlan(t, 1, true, true)
	code := createSubscriptionRedemptionForTest(t, 817, plan, 1)
	seedWalletUser(t, 818, 0)
	seedWalletUser(t, 819, 0)

	_, err := claimSubscriptionRedemptionForTest(818, code.Code)
	require.NoError(t, err)
	_, err = claimSubscriptionRedemptionForTest(819, code.Code)

	require.ErrorIs(t, err, ErrUserRedemptionExhausted)
}

func TestSubscriptionClaimSupportsFreePlan(t *testing.T) {
	setupUserRedemptionTestDB(t)
	plan := seedSubscriptionRedemptionPlan(t, 0, true, true)
	code := createSubscriptionRedemptionForTest(t, 820, plan, 1)
	seedWalletUser(t, 821, 0)

	claim, err := claimSubscriptionRedemptionForTest(821, code.Code)

	require.NoError(t, err)
	require.Zero(t, claim.Quota)
	require.NotZero(t, claim.UserSubscriptionID)
	var stored model.UserRedemptionCode
	require.NoError(t, model.DB.First(&stored, code.ID).Error)
	require.Equal(t, 1, stored.ClaimCount)
	require.Equal(t, model.UserRedemptionStatusExhausted, stored.Status)
}

func TestConcurrentSubscriptionClaimsCannotExceedPlanLimit(t *testing.T) {
	setupUserRedemptionTestDB(t)
	plan := seedSubscriptionRedemptionPlan(t, 1, true, true)
	require.NoError(t, model.DB.Model(plan).Update("max_purchase_per_user", 1).Error)
	plan.MaxPurchasePerUser = 1
	unitQuota := subscriptionRedemptionUnitQuota(t, plan.PriceAmount)
	seedWalletUser(t, 822, unitQuota*2+1)
	create := func() *model.UserRedemptionCode {
		code, err := CreateSubscriptionRedemptionCode(context.Background(), CreateSubscriptionRedemptionInput{
			CreatorUserID: 822,
			PlanID:        plan.Id,
			Units:         1,
			ExpiresAt:     common.GetTimestamp() + int64((2 * time.Hour).Seconds()),
		})
		require.NoError(t, err)
		return code
	}
	codeA := create()
	codeB := create()
	seedWalletUser(t, 823, 0)

	start := make(chan struct{})
	results := make(chan error, 2)
	for _, code := range []string{codeA.Code, codeB.Code} {
		go func(code string) {
			<-start
			_, claimErr := claimSubscriptionRedemptionForTest(823, code)
			results <- claimErr
		}(code)
	}
	close(start)
	successes := 0
	limitErrors := 0
	for range 2 {
		err := <-results
		switch {
		case err == nil:
			successes++
		case errors.Is(err, ErrUserRedemptionPlanPurchaseLimit):
			limitErrors++
		default:
			require.NoError(t, err)
		}
	}

	require.Equal(t, 1, successes)
	require.Equal(t, 1, limitErrors)
	var subscriptionCount int64
	require.NoError(t, model.DB.Model(&model.UserSubscription{}).Where("user_id = ? AND plan_id = ?", 823, plan.Id).Count(&subscriptionCount).Error)
	require.EqualValues(t, 1, subscriptionCount)
}

func TestSubscriptionClaimUsesCurrentLimitWhenSnapshotWasUnlimited(t *testing.T) {
	setupUserRedemptionTestDB(t)
	plan := seedSubscriptionRedemptionPlan(t, 1, true, true)
	require.NoError(t, model.DB.Model(plan).Update("max_purchase_per_user", 0).Error)
	plan.MaxPurchasePerUser = 0
	code := createSubscriptionRedemptionForTest(t, 1831, plan, 1)
	seedWalletUser(t, 1832, 0)

	historical, err := model.CreateUserSubscriptionFromPlanTx(model.DB, 1832, plan, "admin")
	require.NoError(t, err)
	require.NoError(t, model.DB.Delete(historical).Error)
	require.NoError(t, model.DB.Model(plan).Update("max_purchase_per_user", 1).Error)

	_, err = claimSubscriptionRedemptionForTest(1832, code.Code)
	require.ErrorIs(t, err, ErrUserRedemptionPlanPurchaseLimit)
	var claimCount int64
	require.NoError(t, model.DB.Model(&model.UserRedemptionClaim{}).Where("code_id = ?", code.ID).Count(&claimCount).Error)
	require.Zero(t, claimCount)
}

func TestSubscriptionClaimKeepsSnapshotLimitWhenCurrentPlanBecomesUnlimited(t *testing.T) {
	setupUserRedemptionTestDB(t)
	plan := seedSubscriptionRedemptionPlan(t, 1, true, true)
	require.NoError(t, model.DB.Model(plan).Update("max_purchase_per_user", 1).Error)
	plan.MaxPurchasePerUser = 1
	code := createSubscriptionRedemptionForTest(t, 1841, plan, 1)
	seedWalletUser(t, 1842, 0)

	historical, err := model.CreateUserSubscriptionFromPlanTx(model.DB, 1842, plan, "admin")
	require.NoError(t, err)
	require.NoError(t, model.DB.Delete(historical).Error)
	require.NoError(t, model.DB.Model(plan).Update("max_purchase_per_user", 0).Error)

	_, err = claimSubscriptionRedemptionForTest(1842, code.Code)
	require.ErrorIs(t, err, ErrUserRedemptionPlanPurchaseLimit)
	var claimCount int64
	require.NoError(t, model.DB.Model(&model.UserRedemptionClaim{}).Where("code_id = ?", code.ID).Count(&claimCount).Error)
	require.Zero(t, claimCount)
}

func TestConcurrentSubscriptionClaimsCreateIndependentSubscriptions(t *testing.T) {
	setupUserRedemptionTestDB(t)
	plan := seedSubscriptionRedemptionPlan(t, 1, true, true)
	code := createSubscriptionRedemptionForTest(t, 824, plan, 2)
	seedWalletUser(t, 825, 0)
	seedWalletUser(t, 826, 0)

	start := make(chan struct{})
	results := make(chan error, 2)
	for _, userID := range []int{825, 826} {
		go func(userID int) {
			<-start
			_, claimErr := claimSubscriptionRedemptionForTest(userID, code.Code)
			results <- claimErr
		}(userID)
	}
	close(start)
	for range 2 {
		require.NoError(t, <-results)
	}

	var subscriptionCount int64
	require.NoError(t, model.DB.Model(&model.UserSubscription{}).Where("plan_id = ? AND user_id IN ?", plan.Id, []int{825, 826}).Count(&subscriptionCount).Error)
	require.EqualValues(t, 2, subscriptionCount)
	var stored model.UserRedemptionCode
	require.NoError(t, model.DB.First(&stored, code.ID).Error)
	require.Equal(t, 2, stored.ClaimCount)
	require.Equal(t, model.UserRedemptionStatusExhausted, stored.Status)
}

func createPartiallyClaimedSubscriptionRedemptionForTest(t *testing.T, creatorID int, plan *model.SubscriptionPlan, units int, claimantIDs ...int) (*model.UserRedemptionCode, int64) {
	t.Helper()
	code := createSubscriptionRedemptionForTest(t, creatorID, plan, units)
	for _, claimantID := range claimantIDs {
		seedWalletUser(t, claimantID, 0)
		_, err := claimSubscriptionRedemptionForTest(claimantID, code.Code)
		require.NoError(t, err)
	}
	return code, subscriptionRedemptionUnitQuota(t, plan.PriceAmount)
}

func TestDisablePlanSealsSubscriptionCodesBeforeRefundWorker(t *testing.T) {
	setupUserRedemptionTestDB(t)
	plan := seedSubscriptionRedemptionPlan(t, 1, true, true)
	code, unitQuota := createPartiallyClaimedSubscriptionRedemptionForTest(t, 831, plan, 5, 832, 833)

	require.NoError(t, SetSubscriptionPlanEnabled(context.Background(), plan.Id, false, 9001))
	var sealed model.UserRedemptionCode
	require.NoError(t, model.DB.First(&sealed, code.ID).Error)
	require.Equal(t, model.UserRedemptionStatusRefundPending, sealed.Status)
	require.Equal(t, "plan_disabled", sealed.TerminalReason)
	require.Equal(t, 9001, sealed.TerminalActorID)
	seedWalletUser(t, 834, 0)
	_, err := claimSubscriptionRedemptionForTest(834, code.Code)
	require.ErrorIs(t, err, ErrUserRedemptionInactive)

	count, err := FinalizePendingSubscriptionRedemptionRefunds(context.Background(), 100)
	require.NoError(t, err)
	require.Equal(t, 1, count)
	var refunded model.UserRedemptionCode
	require.NoError(t, model.DB.First(&refunded, code.ID).Error)
	require.Equal(t, model.UserRedemptionStatusRefunded, refunded.Status)
	require.Equal(t, unitQuota*3, refunded.RefundedQuota)
	var creator model.User
	require.NoError(t, model.DB.First(&creator, 831).Error)
	require.Equal(t, unitQuota*3+1, creator.Quota)

	count, err = FinalizePendingSubscriptionRedemptionRefunds(context.Background(), 100)
	require.NoError(t, err)
	require.Zero(t, count)
	require.NoError(t, SetSubscriptionPlanEnabled(context.Background(), plan.Id, true, 9001))
	require.NoError(t, model.DB.First(&refunded, code.ID).Error)
	require.Equal(t, model.UserRedemptionStatusRefunded, refunded.Status)
}

func TestCancelSubscriptionRedemptionRefundsUnusedUnitsImmediately(t *testing.T) {
	setupUserRedemptionTestDB(t)
	plan := seedSubscriptionRedemptionPlan(t, 2, true, true)
	code, unitQuota := createPartiallyClaimedSubscriptionRedemptionForTest(t, 835, plan, 3, 836)

	cancelled, err := CancelUserRedemptionCode(context.Background(), UserIdentity{ID: 835, Role: common.RoleCommonUser}, code.ID)
	require.NoError(t, err)
	require.Equal(t, model.UserRedemptionStatusRefunded, cancelled.Status)
	require.Equal(t, unitQuota*2, cancelled.RefundedQuota)
	require.Equal(t, "creator_cancelled", cancelled.TerminalReason)
	require.Equal(t, 835, cancelled.TerminalActorID)

	var stored model.UserRedemptionCode
	require.NoError(t, model.DB.First(&stored, code.ID).Error)
	require.Equal(t, model.UserRedemptionStatusRefunded, stored.Status)
	require.Equal(t, unitQuota*2, stored.RefundedQuota)

	var creator model.User
	require.NoError(t, model.DB.First(&creator, 835).Error)
	require.Equal(t, unitQuota*2+1, creator.Quota)

	count, err := FinalizePendingSubscriptionRedemptionRefunds(context.Background(), 10)
	require.NoError(t, err)
	require.Zero(t, count)

	var auditEvents []model.AuditOutboxEvent
	require.NoError(t, model.DB.Where("event_type = ?", "user.redemption.subscription.refund").Find(&auditEvents).Error)
	require.Len(t, auditEvents, 1)
	require.Equal(t, 835, auditEvents[0].ActorID)
}

func TestUserRedemptionAuditIncludesRequestMetadata(t *testing.T) {
	setupUserRedemptionTestDB(t)
	seedWalletUser(t, 839, redemptionQuota(2))
	ctx := context.WithValue(context.Background(), constant.ContextKeyClientIP, "203.0.113.39")
	ctx = context.WithValue(ctx, constant.ContextKey(common.RequestIdKey), "redemption-request-839")

	code, err := CreateUserRedemptionCode(ctx, CreateUserRedemptionInput{
		CreatorUserID: 839,
		Kind:          model.UserRedemptionKindSingle,
		TotalQuota:    redemptionQuota(1),
		ExpiresAt:     common.GetTimestamp() + int64((2 * time.Hour).Seconds()),
	})
	require.NoError(t, err)

	var event model.AuditOutboxEvent
	require.NoError(t, model.DB.Where("event_id = ?", fmt.Sprintf("user-redemption:%d:create", code.ID)).First(&event).Error)
	require.Equal(t, "203.0.113.39", event.IP)
	require.Equal(t, "redemption-request-839", event.RequestID)
}

func TestExpiredSubscriptionRedemptionQueuesUnusedUnitsForRefund(t *testing.T) {
	setupUserRedemptionTestDB(t)
	plan := seedSubscriptionRedemptionPlan(t, 1, true, true)
	code, unitQuota := createPartiallyClaimedSubscriptionRedemptionForTest(t, 837, plan, 2, 838)
	require.NoError(t, model.DB.Model(&model.UserRedemptionCode{}).Where("id = ?", code.ID).Update("expires_at", common.GetTimestamp()-1).Error)

	count, err := FinalizeExpiredUserRedemptionCodes(context.Background(), 10)
	require.NoError(t, err)
	require.Equal(t, 1, count)
	var pending model.UserRedemptionCode
	require.NoError(t, model.DB.First(&pending, code.ID).Error)
	require.Equal(t, model.UserRedemptionStatusRefundPending, pending.Status)
	count, err = FinalizePendingSubscriptionRedemptionRefunds(context.Background(), 10)
	require.NoError(t, err)
	require.Equal(t, 1, count)
	var creator model.User
	require.NoError(t, model.DB.First(&creator, 837).Error)
	require.Equal(t, unitQuota+1, creator.Quota)
}

func TestAdminCanForfeitUnusedSubscriptionRedemptionUnits(t *testing.T) {
	setupUserRedemptionTestDB(t)
	plan := seedSubscriptionRedemptionPlan(t, 1, true, true)
	code, unitQuota := createPartiallyClaimedSubscriptionRedemptionForTest(t, 839, plan, 3, 840)
	seedWalletUser(t, 841, 0)
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", 841).Updates(map[string]any{"role": common.RoleAdminUser}).Error)

	forfeited, err := AdminMutateUserRedemptionCode(context.Background(), UserRedemptionAdminCommand{
		Actor:      UserIdentity{ID: 841, Role: common.RoleAdminUser},
		CodeID:     code.ID,
		Action:     UserRedemptionActionTerminate,
		Settlement: UserRedemptionSettlementForfeit,
		Reason:     "administrator review",
	})

	require.NoError(t, err)
	require.Equal(t, model.UserRedemptionStatusForfeited, forfeited.Status)
	require.Equal(t, unitQuota*2, forfeited.ForfeitedQuota)
	var creator model.User
	require.NoError(t, model.DB.First(&creator, 839).Error)
	require.Equal(t, int64(1), creator.Quota)
}

func TestAdminBatchSettlementExcludesSubscriptionRedemptions(t *testing.T) {
	setupUserRedemptionTestDB(t)
	plan := seedSubscriptionRedemptionPlan(t, 1, true, true)
	code := createSubscriptionRedemptionForTest(t, 842, plan, 1)
	seedWalletUser(t, 843, 0)
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", 843).Update("role", common.RoleAdminUser).Error)

	results, err := AdminBatchSettleUserRedemptionCodes(context.Background(), UserIdentity{ID: 843, Role: common.RoleAdminUser}, UserRedemptionBatchDispositionRequest{
		IDs:         []int64{code.ID},
		Disposition: UserRedemptionSettlementRefund,
		Reason:      "balance-code batch only",
	})

	require.NoError(t, err)
	require.Len(t, results, 1)
	require.False(t, results[0].Success)
	require.Equal(t, "invalid", results[0].Code)
	var stored model.UserRedemptionCode
	require.NoError(t, model.DB.First(&stored, code.ID).Error)
	require.Equal(t, model.UserRedemptionStatusActive, stored.Status)
}

func TestCreateUserRedemptionRejectsWhenCreationIsDisabled(t *testing.T) {
	setupUserRedemptionTestDB(t)
	setUserRedemptionTestSettings(t, map[string]string{
		user_redemption_setting.EnabledKey: "false",
	})
	unit := redemptionQuota(1)
	seedWalletUser(t, 701, unit*2)

	_, err := CreateUserRedemptionCode(context.Background(), CreateUserRedemptionInput{
		CreatorUserID: 701,
		Kind:          model.UserRedemptionKindSingle,
		TotalQuota:    unit,
		MaxClaims:     1,
		ExpiresAt:     common.GetTimestamp() + int64((2 * time.Hour).Seconds()),
	})

	require.ErrorIs(t, err, ErrUserRedemptionDisabled)
}

func TestCreateUserRedemptionSupportsWalletAmountsAboveSingleRequestLimit(t *testing.T) {
	setupUserRedemptionTestDB(t)
	setUserRedemptionTestSettings(t, map[string]string{
		user_redemption_setting.EnabledKey:       "true",
		user_redemption_setting.SingleEnabledKey: "true",
	})
	largeQuota := int64(5000 * common.QuotaPerUnit)
	remainder := redemptionQuota(1)
	require.Greater(t, largeQuota, int64(common.MaxQuota))
	seedWalletUser(t, 707, largeQuota+remainder)

	code, err := CreateUserRedemptionCode(context.Background(), CreateUserRedemptionInput{
		CreatorUserID: 707,
		Kind:          model.UserRedemptionKindSingle,
		TotalQuota:    largeQuota,
		MaxClaims:     1,
		ExpiresAt:     common.GetTimestamp() + int64((2 * time.Hour).Seconds()),
	})

	require.NoError(t, err)
	require.Equal(t, largeQuota, code.TotalQuota)
	var creator model.User
	require.NoError(t, model.DB.First(&creator, 707).Error)
	require.Equal(t, remainder, creator.Quota)
	var ledger model.WalletLedgerEntry
	require.NoError(t, model.DB.Where("operation_id = ?", code.ReserveOpID).First(&ledger).Error)
	require.Equal(t, -largeQuota, ledger.Delta)
	require.Equal(t, remainder, ledger.BalanceAfter)
}

func TestCreateUserRedemptionRejectsDisabledMode(t *testing.T) {
	setupUserRedemptionTestDB(t)
	setUserRedemptionTestSettings(t, map[string]string{
		user_redemption_setting.EnabledKey:       "true",
		user_redemption_setting.SingleEnabledKey: "true",
		user_redemption_setting.RandomEnabledKey: "false",
	})
	unit := redemptionQuota(2)
	seedWalletUser(t, 702, unit*2)

	_, err := CreateUserRedemptionCode(context.Background(), CreateUserRedemptionInput{
		CreatorUserID: 702,
		Kind:          model.UserRedemptionKindRandom,
		TotalQuota:    unit,
		MaxClaims:     2,
		ExpiresAt:     common.GetTimestamp() + int64((2 * time.Hour).Seconds()),
	})

	require.ErrorIs(t, err, ErrUserRedemptionModeDisabled)
}

func TestCreateUserRedemptionEnforcesActiveCodeLimit(t *testing.T) {
	setupUserRedemptionTestDB(t)
	setUserRedemptionTestSettings(t, map[string]string{
		user_redemption_setting.EnabledKey:               "true",
		user_redemption_setting.SingleEnabledKey:         "true",
		user_redemption_setting.RandomEnabledKey:         "true",
		user_redemption_setting.MaxActiveCodesPerUserKey: "1",
	})
	unit := redemptionQuota(1)
	seedWalletUser(t, 703, unit*3)
	input := CreateUserRedemptionInput{
		CreatorUserID: 703,
		Kind:          model.UserRedemptionKindSingle,
		TotalQuota:    unit,
		MaxClaims:     1,
		ExpiresAt:     common.GetTimestamp() + int64((2 * time.Hour).Seconds()),
	}

	_, err := CreateUserRedemptionCode(context.Background(), input)
	require.NoError(t, err)
	_, err = CreateUserRedemptionCode(context.Background(), input)
	require.ErrorIs(t, err, ErrUserRedemptionActiveLimit)
}

func TestCreateUserRedemptionIgnoresExpiredOpenCodeForActiveLimit(t *testing.T) {
	setupUserRedemptionTestDB(t)
	setUserRedemptionTestSettings(t, map[string]string{
		user_redemption_setting.EnabledKey:               "true",
		user_redemption_setting.SingleEnabledKey:         "true",
		user_redemption_setting.RandomEnabledKey:         "true",
		user_redemption_setting.MaxActiveCodesPerUserKey: "1",
	})
	unit := redemptionQuota(1)
	seedWalletUser(t, 705, unit*3)
	require.NoError(t, model.DB.Create(&model.UserRedemptionCode{
		Code:          "expired-open-code",
		CreatorUserID: 705,
		Kind:          model.UserRedemptionKindSingle,
		TotalQuota:    unit,
		MaxClaims:     1,
		Status:        model.UserRedemptionStatusActive,
		ExpiresAt:     common.GetTimestamp() - 1,
		ReserveOpID:   "user-redemption:expired-open-code:reserve",
	}).Error)

	_, err := CreateUserRedemptionCode(context.Background(), CreateUserRedemptionInput{
		CreatorUserID: 705,
		Kind:          model.UserRedemptionKindSingle,
		TotalQuota:    unit,
		MaxClaims:     1,
		ExpiresAt:     common.GetTimestamp() + int64((2 * time.Hour).Seconds()),
	})

	require.NoError(t, err)
}

func TestCreateUserRedemptionEnforcesActiveLimitConcurrentlyOnSQLite(t *testing.T) {
	setupUserRedemptionTestDB(t)
	previousDatabaseType := common.MainDatabaseType()
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	t.Cleanup(func() { common.SetMainDatabaseType(previousDatabaseType) })
	sqlDB, err := model.DB.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(4)
	setUserRedemptionTestSettings(t, map[string]string{
		user_redemption_setting.EnabledKey:               "true",
		user_redemption_setting.SingleEnabledKey:         "true",
		user_redemption_setting.RandomEnabledKey:         "true",
		user_redemption_setting.MaxActiveCodesPerUserKey: "1",
	})
	unit := redemptionQuota(1)
	seedWalletUser(t, 706, unit*3)
	input := CreateUserRedemptionInput{
		CreatorUserID: 706,
		Kind:          model.UserRedemptionKindSingle,
		TotalQuota:    unit,
		MaxClaims:     1,
		ExpiresAt:     common.GetTimestamp() + int64((2 * time.Hour).Seconds()),
	}
	start := make(chan struct{})
	results := make(chan error, 2)
	for range 2 {
		go func() {
			<-start
			_, createErr := CreateUserRedemptionCode(context.Background(), input)
			results <- createErr
		}()
	}
	close(start)

	successes := 0
	limitErrors := 0
	for range 2 {
		createErr := <-results
		switch {
		case createErr == nil:
			successes++
		case errors.Is(createErr, ErrUserRedemptionActiveLimit):
			limitErrors++
		default:
			require.NoError(t, createErr)
		}
	}
	require.Equal(t, 1, successes)
	require.Equal(t, 1, limitErrors)
}

func TestCreateUserRedemptionEnforcesConfiguredClaimsAndExpiryLimits(t *testing.T) {
	setupUserRedemptionTestDB(t)
	setUserRedemptionTestSettings(t, map[string]string{
		user_redemption_setting.EnabledKey:          "true",
		user_redemption_setting.RandomEnabledKey:    "true",
		user_redemption_setting.MaxClaimsPerCodeKey: "3",
		user_redemption_setting.MaxExpiryHoursKey:   "2",
	})
	unit := redemptionQuota(4)
	seedWalletUser(t, 704, unit*2)

	_, err := CreateUserRedemptionCode(context.Background(), CreateUserRedemptionInput{
		CreatorUserID: 704,
		Kind:          model.UserRedemptionKindRandom,
		TotalQuota:    unit,
		MaxClaims:     4,
		ExpiresAt:     common.GetTimestamp() + int64((2 * time.Hour).Seconds()),
	})
	require.ErrorIs(t, err, ErrUserRedemptionClaimsLimit)

	_, err = CreateUserRedemptionCode(context.Background(), CreateUserRedemptionInput{
		CreatorUserID: 704,
		Kind:          model.UserRedemptionKindRandom,
		TotalQuota:    unit,
		MaxClaims:     2,
		ExpiresAt:     common.GetTimestamp() + int64((3 * time.Hour).Seconds()),
	})
	require.ErrorIs(t, err, ErrUserRedemptionExpiryLimit)
}

func TestUserRedemptionRemainingQuotaAccountsForForfeiture(t *testing.T) {
	row := model.UserRedemptionCode{
		TotalQuota:     100,
		ClaimedQuota:   25,
		RefundedQuota:  15,
		ForfeitedQuota: 10,
	}

	require.Equal(t, int64(50), UserRedemptionRemainingQuota(row))
}

func TestUserRedemptionRemainingQuotaRejectsMalformedCounters(t *testing.T) {
	require.Zero(t, UserRedemptionRemainingQuota(model.UserRedemptionCode{
		TotalQuota:   10,
		ClaimedQuota: -1,
	}))
	require.Zero(t, UserRedemptionRemainingQuota(model.UserRedemptionCode{
		TotalQuota:   10,
		ClaimedQuota: 11,
	}))
	require.Zero(t, UserRedemptionRemainingQuota(model.UserRedemptionCode{
		TotalQuota: -1,
	}))
}

func TestUserRedemptionMinimumQuotaCheckAvoidsIntegerOverflow(t *testing.T) {
	maxInt64 := int64(^uint64(0) >> 1)
	require.False(t, hasMinimumUserRedemptionQuota(maxInt64, maxInt64/2+1, 3))
	require.True(t, hasMinimumUserRedemptionQuota(maxInt64, maxInt64/2, 2))
}

func TestCreateUserRedemptionRejectsOverflowingExpiryDuration(t *testing.T) {
	setupUserRedemptionTestDB(t)
	seedWalletUser(t, 6, redemptionQuota(2))

	// Multiplying this many seconds by time.Second wraps to one hour in int64.
	overflowingExpiry := common.GetTimestamp() + (int64(1) << 55) + int64(time.Hour/time.Second)
	_, err := CreateUserRedemptionCode(context.Background(), CreateUserRedemptionInput{
		CreatorUserID: 6,
		Kind:          model.UserRedemptionKindSingle,
		TotalQuota:    redemptionQuota(1),
		ExpiresAt:     overflowingExpiry,
	})
	require.ErrorIs(t, err, ErrUserRedemptionInvalid)
}

func TestRedeemWalletCodeAcceptsUserCreatedCodeThroughWalletEntry(t *testing.T) {
	setupUserRedemptionTestDB(t)
	standardQuota := redemptionQuota(1)
	seedWalletUser(t, 1, standardQuota*2)
	seedWalletUser(t, 2, 0)
	expires := common.GetTimestamp() + int64((2 * time.Hour).Seconds())
	code, err := CreateUserRedemptionCode(context.Background(), CreateUserRedemptionInput{
		CreatorUserID: 1,
		Kind:          model.UserRedemptionKindSingle,
		TotalQuota:    standardQuota,
		MaxClaims:     1,
		ExpiresAt:     expires,
	})
	require.NoError(t, err)

	result, err := RedeemWalletCode(context.Background(), 2, code.Code)
	require.NoError(t, err)
	require.Equal(t, standardQuota, result.Quota)
	require.Equal(t, "user", result.CodeType)

	var claimer model.User
	require.NoError(t, model.DB.First(&claimer, 2).Error)
	require.Equal(t, standardQuota, claimer.Quota)
}

func TestRedeemWalletCodeKeepsLegacyAdminCodeCompatibility(t *testing.T) {
	setupUserRedemptionTestDB(t)
	seedWalletUser(t, 3, 0)
	legacyQuota := redemptionQuota(2)
	legacyCode := model.Redemption{
		Name:        "legacy-wallet-code",
		Key:         "20000000000000000000000000000003",
		Status:      common.RedemptionCodeStatusEnabled,
		Quota:       int(legacyQuota),
		CreatedTime: common.GetTimestamp(),
	}
	require.NoError(t, model.DB.Create(&legacyCode).Error)

	result, err := RedeemWalletCode(context.Background(), 3, legacyCode.Key)
	require.NoError(t, err)
	require.Equal(t, legacyQuota, result.Quota)
	require.Equal(t, "legacy", result.CodeType)

	var user model.User
	require.NoError(t, model.DB.First(&user, 3).Error)
	require.Equal(t, legacyQuota, user.Quota)
}

func TestLegacyRedemptionInsertRejectsUserCodeConflict(t *testing.T) {
	setupUserRedemptionTestDB(t)
	code := "SHAREDREDEMPTIONCODE1234"
	require.NoError(t, model.DB.Create(&model.UserRedemptionCode{
		Code:          code,
		CreatorUserID: 1,
		Kind:          model.UserRedemptionKindSingle,
		TotalQuota:    redemptionQuota(1),
		MaxClaims:     1,
		Status:        model.UserRedemptionStatusActive,
		ExpiresAt:     common.GetTimestamp() + 3600,
		ReserveOpID:   "user-redemption:conflict:reserve",
	}).Error)

	err := (&model.Redemption{
		Name:        "legacy-conflict",
		Key:         code,
		Status:      common.RedemptionCodeStatusEnabled,
		Quota:       int(redemptionQuota(1)),
		CreatedTime: common.GetTimestamp(),
	}).Insert()

	require.Error(t, err)
}

func TestRedeemWalletCodeRejectsAmbiguousCrossSystemCode(t *testing.T) {
	setupUserRedemptionTestDB(t)
	seedWalletUser(t, 4, 0)
	code := "AMBIGUOUSREDEMPTION1234"
	require.NoError(t, model.DB.Create(&model.UserRedemptionCode{
		Code:          code,
		CreatorUserID: 1,
		Kind:          model.UserRedemptionKindSingle,
		TotalQuota:    redemptionQuota(1),
		MaxClaims:     1,
		Status:        model.UserRedemptionStatusActive,
		ExpiresAt:     common.GetTimestamp() + 3600,
		ReserveOpID:   "user-redemption:ambiguous:reserve",
	}).Error)
	require.NoError(t, model.DB.Create(&model.Redemption{
		Name:        "legacy-ambiguous",
		Key:         code,
		Status:      common.RedemptionCodeStatusEnabled,
		Quota:       int(redemptionQuota(2)),
		CreatedTime: common.GetTimestamp(),
	}).Error)

	_, err := RedeemWalletCode(context.Background(), 4, code)

	require.ErrorIs(t, err, ErrWalletRedemptionAmbiguous)
	var user model.User
	require.NoError(t, model.DB.First(&user, 4).Error)
	require.Zero(t, user.Quota)
}

func TestUserRedemptionSingleCreateClaimCancel(t *testing.T) {
	setupUserRedemptionTestDB(t)
	seedWalletUser(t, 1, redemptionQuota(10))
	seedWalletUser(t, 2, 0)
	expires := common.GetTimestamp() + int64((2 * time.Hour).Seconds())
	code, err := CreateUserRedemptionCode(context.Background(), CreateUserRedemptionInput{
		CreatorUserID: 1,
		Kind:          model.UserRedemptionKindSingle,
		TotalQuota:    redemptionQuota(2),
		MaxClaims:     1,
		ExpiresAt:     expires,
	})
	require.NoError(t, err)
	require.Equal(t, model.UserRedemptionStatusActive, code.Status)

	var creator model.User
	require.NoError(t, model.DB.First(&creator, 1).Error)
	require.Equal(t, redemptionQuota(8), creator.Quota)

	claim, err := ClaimUserRedemptionCode(context.Background(), 2, code.Code)
	require.NoError(t, err)
	require.Equal(t, redemptionQuota(2), claim.Quota)

	require.NoError(t, model.DB.First(&creator, 1).Error)
	var claimer model.User
	require.NoError(t, model.DB.First(&claimer, 2).Error)
	require.Equal(t, redemptionQuota(8), creator.Quota)
	require.Equal(t, redemptionQuota(2), claimer.Quota)

	var stored model.UserRedemptionCode
	require.NoError(t, model.DB.First(&stored, code.ID).Error)
	require.Equal(t, model.UserRedemptionStatusExhausted, stored.Status)
}

func TestUserRedemptionRejectsRawQuotaBelowStandardUnit(t *testing.T) {
	setupUserRedemptionTestDB(t)
	seedWalletUser(t, 99, int64(common.QuotaFromFloat(common.QuotaPerUnit)+1))
	expires := common.GetTimestamp() + int64((2 * time.Hour).Seconds())

	_, err := CreateUserRedemptionCode(context.Background(), CreateUserRedemptionInput{
		CreatorUserID: 99,
		Kind:          model.UserRedemptionKindSingle,
		TotalQuota:    1,
		ExpiresAt:     expires,
	})

	require.ErrorIs(t, err, ErrUserRedemptionInvalid)
}

func TestUserRedemptionSelfClaimConservesReservedQuota(t *testing.T) {
	setupUserRedemptionTestDB(t)
	seedWalletUser(t, 3, redemptionQuota(5))
	expires := common.GetTimestamp() + int64((2 * time.Hour).Seconds())
	code, err := CreateUserRedemptionCode(context.Background(), CreateUserRedemptionInput{
		CreatorUserID: 3,
		Kind:          model.UserRedemptionKindSingle,
		TotalQuota:    redemptionQuota(1),
		ExpiresAt:     expires,
	})
	require.NoError(t, err)
	claim, err := ClaimUserRedemptionCode(context.Background(), 3, code.Code)
	require.NoError(t, err)
	require.Equal(t, redemptionQuota(1), claim.Quota)

	var user model.User
	require.NoError(t, model.DB.First(&user, 3).Error)
	require.Equal(t, redemptionQuota(5), user.Quota)

	var stored model.UserRedemptionCode
	require.NoError(t, model.DB.First(&stored, code.ID).Error)
	require.Equal(t, model.UserRedemptionStatusExhausted, stored.Status)
	require.Equal(t, 1, stored.ClaimCount)
	require.Equal(t, redemptionQuota(1), stored.ClaimedQuota)
	require.Equal(t, redemptionQuota(5), user.Quota+UserRedemptionRemainingQuota(stored))

	var claimCount int64
	require.NoError(t, model.DB.Model(&model.UserRedemptionClaim{}).Where("code_id = ?", code.ID).Count(&claimCount).Error)
	require.Equal(t, int64(1), claimCount)
}

func TestClaimUserRedemptionReturnsSpecificStoredStateErrors(t *testing.T) {
	states := []struct {
		name   string
		status string
		want   error
	}{
		{name: "disabled by administrator", status: model.UserRedemptionStatusDisabled, want: ErrUserRedemptionCodeDisabled},
		{name: "expired pending settlement", status: model.UserRedemptionStatusExpiredPending, want: ErrUserRedemptionExpired},
		{name: "fully claimed", status: model.UserRedemptionStatusExhausted, want: ErrUserRedemptionExhausted},
	}

	for index, testCase := range states {
		t.Run(testCase.name, func(t *testing.T) {
			setupUserRedemptionTestDB(t)
			creatorID := 150 + index*2
			claimerID := creatorID + 1
			seedWalletUser(t, creatorID, redemptionQuota(2))
			seedWalletUser(t, claimerID, 0)
			code, err := CreateUserRedemptionCode(context.Background(), CreateUserRedemptionInput{
				CreatorUserID: creatorID,
				Kind:          model.UserRedemptionKindSingle,
				TotalQuota:    redemptionQuota(1),
				ExpiresAt:     common.GetTimestamp() + int64((2 * time.Hour).Seconds()),
			})
			require.NoError(t, err)
			require.NoError(t, model.DB.Model(&model.UserRedemptionCode{}).
				Where("id = ?", code.ID).
				Update("status", testCase.status).Error)

			_, err = ClaimUserRedemptionCode(context.Background(), claimerID, code.Code)

			require.ErrorIs(t, err, testCase.want)
		})
	}
}

func TestClaimUserRedemptionReturnsNotFoundForUnknownCode(t *testing.T) {
	setupUserRedemptionTestDB(t)
	seedWalletUser(t, 160, 0)

	_, err := ClaimUserRedemptionCode(context.Background(), 160, "UNKNOWN-USER-REDEMPTION")

	require.ErrorIs(t, err, ErrUserRedemptionNotFound)
}

func TestUserRedemptionCancelMarksUnclaimedCodePending(t *testing.T) {
	setupUserRedemptionTestDB(t)
	seedWalletUser(t, 4, redemptionQuota(3))
	expires := common.GetTimestamp() + int64((2 * time.Hour).Seconds())
	code, err := CreateUserRedemptionCode(context.Background(), CreateUserRedemptionInput{
		CreatorUserID: 4,
		Kind:          model.UserRedemptionKindSingle,
		TotalQuota:    redemptionQuota(1),
		ExpiresAt:     expires,
	})
	require.NoError(t, err)
	cancelled, err := CancelUserRedemptionCode(context.Background(), UserIdentity{ID: 4, Role: common.RoleCommonUser}, code.ID)
	require.NoError(t, err)
	require.Equal(t, model.UserRedemptionStatusExpiredPending, cancelled.Status)
	require.Zero(t, cancelled.RefundedQuota)
	var user model.User
	require.NoError(t, model.DB.First(&user, 4).Error)
	require.Equal(t, redemptionQuota(2), user.Quota)
}

func TestRandomClaimQuotaLastGetsRemainder(t *testing.T) {
	q, err := randomClaimQuota(30, 1, 10)
	require.NoError(t, err)
	require.Equal(t, int64(30), q)
	q, err = randomClaimQuota(30, 3, 10)
	require.NoError(t, err)
	require.GreaterOrEqual(t, q, int64(10))
	require.LessOrEqual(t, q, int64(20))
}

func TestUserRedemptionInsufficientBalance(t *testing.T) {
	setupUserRedemptionTestDB(t)
	seedWalletUser(t, 9, 5000)
	expires := common.GetTimestamp() + int64((2 * time.Hour).Seconds())
	_, err := CreateUserRedemptionCode(context.Background(), CreateUserRedemptionInput{
		CreatorUserID: 9,
		Kind:          model.UserRedemptionKindSingle,
		TotalQuota:    redemptionQuota(1),
		ExpiresAt:     expires,
	})
	require.ErrorIs(t, err, ErrUserRedemptionInsufficient)
}

func TestUserRedemptionDuplicateClaimRejected(t *testing.T) {
	setupUserRedemptionTestDB(t)
	seedWalletUser(t, 10, redemptionQuota(5))
	seedWalletUser(t, 11, 0)
	expires := common.GetTimestamp() + int64((2 * time.Hour).Seconds())
	code, err := CreateUserRedemptionCode(context.Background(), CreateUserRedemptionInput{
		CreatorUserID: 10,
		Kind:          model.UserRedemptionKindRandom,
		TotalQuota:    redemptionQuota(3),
		MaxClaims:     3,
		ExpiresAt:     expires,
	})
	require.NoError(t, err)
	_, err = ClaimUserRedemptionCode(context.Background(), 11, code.Code)
	require.NoError(t, err)
	_, err = ClaimUserRedemptionCode(context.Background(), 11, code.Code)
	require.ErrorIs(t, err, ErrUserRedemptionAlreadyClaimed)
}

func TestFinalizeExpiredUserRedemptionCodes(t *testing.T) {
	setupUserRedemptionTestDB(t)
	seedWalletUser(t, 12, redemptionQuota(4))
	// create with valid TTL then force-expire in DB
	expires := common.GetTimestamp() + int64((2 * time.Hour).Seconds())
	code, err := CreateUserRedemptionCode(context.Background(), CreateUserRedemptionInput{
		CreatorUserID: 12,
		Kind:          model.UserRedemptionKindSingle,
		TotalQuota:    redemptionQuota(1),
		ExpiresAt:     expires,
	})
	require.NoError(t, err)
	require.NoError(t, model.DB.Model(&model.UserRedemptionCode{}).Where("id = ?", code.ID).Update("expires_at", common.GetTimestamp()-10).Error)
	n, err := FinalizeExpiredUserRedemptionCodes(context.Background(), 50)
	require.NoError(t, err)
	require.GreaterOrEqual(t, n, 1)
	var user model.User
	require.NoError(t, model.DB.First(&user, 12).Error)
	require.Equal(t, redemptionQuota(3), user.Quota)
	var stored model.UserRedemptionCode
	require.NoError(t, model.DB.First(&stored, code.ID).Error)
	require.Equal(t, model.UserRedemptionStatusExpiredPending, stored.Status)
}

func TestClaimExpiredMarksPending(t *testing.T) {
	setupUserRedemptionTestDB(t)
	seedWalletUser(t, 20, redemptionQuota(4))
	expires := common.GetTimestamp() + int64((2 * time.Hour).Seconds())
	code, err := CreateUserRedemptionCode(context.Background(), CreateUserRedemptionInput{
		CreatorUserID: 20,
		Kind:          model.UserRedemptionKindSingle,
		TotalQuota:    redemptionQuota(1),
		ExpiresAt:     expires,
	})
	require.NoError(t, err)
	require.NoError(t, model.DB.Model(&model.UserRedemptionCode{}).Where("id = ?", code.ID).Update("expires_at", common.GetTimestamp()-10).Error)

	_, err = ClaimUserRedemptionCode(context.Background(), 21, code.Code)
	require.ErrorIs(t, err, ErrUserRedemptionExpired)

	var stored model.UserRedemptionCode
	require.NoError(t, model.DB.First(&stored, code.ID).Error)
	require.Equal(t, model.UserRedemptionStatusExpiredPending, stored.Status)
	require.Zero(t, stored.RefundedQuota)

	var creator model.User
	require.NoError(t, model.DB.First(&creator, 20).Error)
	require.Equal(t, redemptionQuota(3), creator.Quota)
}

func TestAdminListUserRedemptionsSeesAllCreators(t *testing.T) {
	setupUserRedemptionTestDB(t)
	// creators: common user + peer admin; listing is not hierarchy-scoped for admins
	require.NoError(t, model.DB.Create(&model.User{Id: 3, Username: "user3", Password: "x", DisplayName: "u", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: "default", AffCode: "a3", Quota: redemptionQuota(1)}).Error)
	require.NoError(t, model.DB.Create(&model.User{Id: 4, Username: "admin2", Password: "x", DisplayName: "a2", Role: common.RoleAdminUser, Status: common.UserStatusEnabled, Group: "default", AffCode: "a4", Quota: redemptionQuota(1)}).Error)

	expires := common.GetTimestamp() + int64((2 * time.Hour).Seconds())
	_, err := CreateUserRedemptionCode(context.Background(), CreateUserRedemptionInput{CreatorUserID: 3, Kind: model.UserRedemptionKindSingle, TotalQuota: redemptionQuota(1), ExpiresAt: expires})
	require.NoError(t, err)
	_, err = CreateUserRedemptionCode(context.Background(), CreateUserRedemptionInput{CreatorUserID: 4, Kind: model.UserRedemptionKindSingle, TotalQuota: redemptionQuota(1), ExpiresAt: expires})
	require.NoError(t, err)

	// non-admin cannot list all codes even if CreatorUserID is omitted
	page, err := ListUserRedemptionCodes(context.Background(), UserIdentity{ID: 3, Role: common.RoleCommonUser}, UserRedemptionListQuery{
		Page:     1,
		PageSize: 50,
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), page.Total)
	require.Equal(t, 3, page.Items[0].CreatorUserID)

	// any admin sees all creators
	page, err = ListUserRedemptionCodes(context.Background(), UserIdentity{ID: 4, Role: common.RoleAdminUser}, UserRedemptionListQuery{
		Page:     1,
		PageSize: 50,
	})
	require.NoError(t, err)
	require.Equal(t, int64(2), page.Total)

	// optional creator filter still works for admins
	page, err = ListUserRedemptionCodes(context.Background(), UserIdentity{ID: 4, Role: common.RoleAdminUser}, UserRedemptionListQuery{
		CreatorUserID: 4,
		Page:          1,
		PageSize:      20,
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), page.Total)
	require.Equal(t, 4, page.Items[0].CreatorUserID)
}

func TestAdminListUserRedemptionsFiltersSoftDeletedHistory(t *testing.T) {
	setupUserRedemptionTestDB(t)
	seedWalletUser(t, 7, redemptionQuota(4))
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", 7).Update("username", "alice-owner").Error)
	require.NoError(t, model.DB.Create(&model.User{
		Id: 8, Username: "admin8", AffCode: "admin8",
		Status: common.UserStatusEnabled, Role: common.RoleAdminUser,
	}).Error)
	expires := common.GetTimestamp() + int64((2 * time.Hour).Seconds())
	_, err := CreateUserRedemptionCode(context.Background(), CreateUserRedemptionInput{
		CreatorUserID: 7, Kind: model.UserRedemptionKindSingle,
		TotalQuota: redemptionQuota(1), ExpiresAt: expires,
	})
	require.NoError(t, err)
	deletedCode, err := CreateUserRedemptionCode(context.Background(), CreateUserRedemptionInput{
		CreatorUserID: 7, Kind: model.UserRedemptionKindSingle,
		TotalQuota: redemptionQuota(1), ExpiresAt: expires,
	})
	require.NoError(t, err)
	_, err = AdminMutateUserRedemptionCode(context.Background(), UserRedemptionAdminCommand{
		Actor:      UserIdentity{ID: 8, Role: common.RoleAdminUser},
		CodeID:     deletedCode.ID,
		Action:     UserRedemptionActionDelete,
		Settlement: UserRedemptionSettlementRefund,
		Reason:     "archive",
	})
	require.NoError(t, err)

	activeOnly, err := ListUserRedemptionCodes(context.Background(), UserIdentity{ID: 8, Role: common.RoleAdminUser}, UserRedemptionListQuery{
		Page: 1, PageSize: 20,
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), activeOnly.Total)

	deletedOnly, err := ListUserRedemptionCodes(context.Background(), UserIdentity{ID: 8, Role: common.RoleAdminUser}, UserRedemptionListQuery{
		Deleted: "true", Page: 1, PageSize: 20,
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), deletedOnly.Total)
	require.True(t, deletedOnly.Items[0].Deleted)

	allByUsername, err := ListUserRedemptionCodes(context.Background(), UserIdentity{ID: 8, Role: common.RoleAdminUser}, UserRedemptionListQuery{
		CreatorUsername: "alice", Deleted: "all", Page: 1, PageSize: 20,
	})
	require.NoError(t, err)
	require.Equal(t, int64(2), allByUsername.Total)

	ownerHistory, err := ListUserRedemptionCodes(context.Background(), UserIdentity{ID: 7, Role: common.RoleCommonUser}, UserRedemptionListQuery{
		Page: 1, PageSize: 20,
	})
	require.NoError(t, err)
	require.Equal(t, int64(2), ownerHistory.Total)
}

func TestAdminOwnerListReturnsCompleteHistoryAcrossPages(t *testing.T) {
	setupUserRedemptionTestDB(t)
	const ownerID = 9
	const otherID = 10
	require.NoError(t, model.DB.Create(&model.User{
		Id: ownerID, Username: "owner-admin", AffCode: "owner-admin-aff",
		Status: common.UserStatusEnabled, Role: common.RoleAdminUser,
	}).Error)
	require.NoError(t, model.DB.Create(&model.User{
		Id: otherID, Username: "other-owner", AffCode: "other-owner-aff",
		Status: common.UserStatusEnabled, Role: common.RoleCommonUser,
	}).Error)

	now := common.GetTimestamp()
	rows := make([]model.UserRedemptionCode, 0, 46)
	for i := 0; i < 45; i++ {
		row := model.UserRedemptionCode{
			Code: fmt.Sprintf("OWNER-HISTORY-%02d", i), CreatorUserID: ownerID,
			IssuedBy: ownerID, RewardType: model.UserRedemptionRewardBalance,
			Kind: model.UserRedemptionKindSingle, TotalQuota: redemptionQuota(1),
			ClaimedQuota: redemptionQuota(1), MaxClaims: 1, ClaimCount: 1,
			Status: model.UserRedemptionStatusExhausted, ExpiresAt: now + 3600,
			ReserveOpID: fmt.Sprintf("owner-history-reserve-%02d", i),
		}
		if i%7 == 0 {
			row.Status = model.UserRedemptionStatusDeleted
			row.DeletedAt = now
			row.DeletedBy = ownerID
		}
		rows = append(rows, row)
	}
	rows = append(rows, model.UserRedemptionCode{
		Code: "OTHER-OWNER-CODE", CreatorUserID: otherID, IssuedBy: otherID,
		RewardType: model.UserRedemptionRewardBalance, Kind: model.UserRedemptionKindSingle,
		TotalQuota: redemptionQuota(1), MaxClaims: 1, Status: model.UserRedemptionStatusActive,
		ExpiresAt: now + 3600, ReserveOpID: "other-owner-reserve",
	})
	require.NoError(t, model.DB.Create(&rows).Error)

	seen := make(map[int64]struct{}, 45)
	deletedSeen := 0
	for pageNumber, expectedItems := range []int{20, 20, 5} {
		page, err := ListUserRedemptionCodes(context.Background(), UserIdentity{
			ID: ownerID, Role: common.RoleAdminUser,
		}, UserRedemptionListQuery{
			OwnerOnly: true, CreatorUserID: otherID,
			Page: pageNumber + 1, PageSize: 20,
		})
		require.NoError(t, err)
		require.Equal(t, int64(45), page.Total)
		require.Len(t, page.Items, expectedItems)
		for _, item := range page.Items {
			require.Equal(t, ownerID, item.CreatorUserID)
			_, duplicate := seen[item.ID]
			require.False(t, duplicate)
			seen[item.ID] = struct{}{}
			if item.Deleted {
				deletedSeen++
			}
		}
	}
	require.Len(t, seen, 45)
	require.Equal(t, 7, deletedSeen)
}

func TestAdminCanGetAnyUserRedemptionDetail(t *testing.T) {
	setupUserRedemptionTestDB(t)
	require.NoError(t, model.DB.Create(&model.User{Id: 5, Username: "user5", Password: "x", DisplayName: "u5", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: "default", AffCode: "a5", Quota: redemptionQuota(1)}).Error)
	require.NoError(t, model.DB.Create(&model.User{Id: 6, Username: "admin6", Password: "x", DisplayName: "a6", Role: common.RoleAdminUser, Status: common.UserStatusEnabled, Group: "default", AffCode: "a6", Quota: 0}).Error)
	expires := common.GetTimestamp() + int64((2 * time.Hour).Seconds())
	code, err := CreateUserRedemptionCode(context.Background(), CreateUserRedemptionInput{CreatorUserID: 5, Kind: model.UserRedemptionKindSingle, TotalQuota: redemptionQuota(1), ExpiresAt: expires})
	require.NoError(t, err)

	detail, err := GetUserRedemptionDetail(context.Background(), UserIdentity{ID: 6, Role: common.RoleAdminUser}, code.ID, true)
	require.NoError(t, err)
	require.Equal(t, code.ID, detail.Code.ID)

	_, err = GetUserRedemptionDetail(context.Background(), UserIdentity{ID: 7, Role: common.RoleCommonUser}, code.ID, false)
	require.ErrorIs(t, err, ErrUserRedemptionForbidden)

	_, err = GetUserRedemptionDetail(context.Background(), UserIdentity{ID: 0, Role: common.RoleRootUser}, code.ID, false)
	require.ErrorIs(t, err, ErrUserRedemptionInvalid)
}

func TestUserRedemptionDetailRedactsClaimantsForCreatorAndRevealsThemToAdmin(t *testing.T) {
	setupUserRedemptionTestDB(t)
	seedWalletUser(t, 15, redemptionQuota(2))
	require.NoError(t, model.DB.Create(&model.User{
		Id: 16, Username: "claimant", AffCode: "claimant",
		Status: common.UserStatusEnabled, Role: common.RoleCommonUser,
	}).Error)
	require.NoError(t, model.DB.Create(&model.User{
		Id: 17, Username: "admin17", AffCode: "admin17",
		Status: common.UserStatusEnabled, Role: common.RoleAdminUser,
	}).Error)
	code, err := CreateUserRedemptionCode(context.Background(), CreateUserRedemptionInput{
		CreatorUserID: 15,
		Kind:          model.UserRedemptionKindSingle,
		TotalQuota:    redemptionQuota(1),
		ExpiresAt:     common.GetTimestamp() + int64((2 * time.Hour).Seconds()),
	})
	require.NoError(t, err)
	_, err = ClaimUserRedemptionCode(context.Background(), 16, code.Code)
	require.NoError(t, err)

	creatorDetail, err := GetUserRedemptionDetail(context.Background(), UserIdentity{ID: 15, Role: common.RoleCommonUser}, code.ID, true)
	require.NoError(t, err)
	require.Equal(t, int64(0), creatorDetail.Code.RemainingQuota)
	require.Len(t, creatorDetail.Claims, 1)
	require.Equal(t, "c***t", creatorDetail.Claims[0].ClaimantDisplay)
	require.Zero(t, creatorDetail.Claims[0].ClaimantUserID)
	require.Empty(t, creatorDetail.Claims[0].ClaimantUsername)

	adminDetail, err := GetUserRedemptionDetail(context.Background(), UserIdentity{ID: 17, Role: common.RoleAdminUser}, code.ID, true)
	require.NoError(t, err)
	require.Len(t, adminDetail.Claims, 1)
	require.Equal(t, 16, adminDetail.Claims[0].ClaimantUserID)
	require.Equal(t, "claimant", adminDetail.Claims[0].ClaimantUsername)
	require.Empty(t, adminDetail.Claims[0].ClaimantDisplay)
}

func TestAdminCancelUserRedemptionCode(t *testing.T) {
	setupUserRedemptionTestDB(t)
	seedWalletUser(t, 30, redemptionQuota(5))
	require.NoError(t, model.DB.Create(&model.User{Id: 31, Username: "admin31", Password: "x", DisplayName: "a31", Role: common.RoleAdminUser, Status: common.UserStatusEnabled, Group: "default", AffCode: "a31", Quota: 0}).Error)
	expires := common.GetTimestamp() + int64((2 * time.Hour).Seconds())
	code, err := CreateUserRedemptionCode(context.Background(), CreateUserRedemptionInput{
		CreatorUserID: 30,
		Kind:          model.UserRedemptionKindSingle,
		TotalQuota:    redemptionQuota(1),
		ExpiresAt:     expires,
	})
	require.NoError(t, err)

	// peer common user cannot cancel
	_, err = CancelUserRedemptionCode(context.Background(), UserIdentity{ID: 99, Role: common.RoleCommonUser}, code.ID)
	require.ErrorIs(t, err, ErrUserRedemptionForbidden)

	// administrators must use the governed mutation so settlement and reason are explicit
	_, err = CancelUserRedemptionCode(context.Background(), UserIdentity{ID: 31, Role: common.RoleAdminUser}, code.ID)
	require.ErrorIs(t, err, ErrUserRedemptionForbidden)

	out, err := AdminMutateUserRedemptionCode(context.Background(), UserRedemptionAdminCommand{
		Actor:      UserIdentity{ID: 31, Role: common.RoleAdminUser},
		CodeID:     code.ID,
		Action:     UserRedemptionActionTerminate,
		Settlement: UserRedemptionSettlementRefund,
		Reason:     "admin policy",
	})
	require.NoError(t, err)
	require.Equal(t, model.UserRedemptionStatusRefunded, out.Status)

	var creator model.User
	require.NoError(t, model.DB.First(&creator, 30).Error)
	require.Equal(t, redemptionQuota(5), creator.Quota)

	var cancelEvent model.AuditOutboxEvent
	require.NoError(t, model.DB.Where("event_type = ? AND actor_id = ?", "user.redemption.terminate_refund", 31).First(&cancelEvent).Error)
	require.Equal(t, "user.redemption.terminate_refund", cancelEvent.EventType)
	require.Equal(t, 31, cancelEvent.ActorID)
	require.Equal(t, 30, cancelEvent.LogUserID)
}

func TestAdminUserRedemptionDisableAndEnable(t *testing.T) {
	setupUserRedemptionTestDB(t)
	standardQuota := redemptionQuota(1)
	seedWalletUser(t, 50, standardQuota*5)
	seedWalletUser(t, 51, 0)
	require.NoError(t, model.DB.Create(&model.User{
		Id: 52, Username: "admin52", AffCode: "admin52", Quota: 0,
		Status: common.UserStatusEnabled, Role: common.RoleAdminUser,
	}).Error)

	code, err := CreateUserRedemptionCode(context.Background(), CreateUserRedemptionInput{
		CreatorUserID: 50,
		Kind:          model.UserRedemptionKindSingle,
		TotalQuota:    standardQuota,
		ExpiresAt:     common.GetTimestamp() + int64((2 * time.Hour).Seconds()),
	})
	require.NoError(t, err)

	disabled, err := AdminMutateUserRedemptionCode(context.Background(), UserRedemptionAdminCommand{
		Actor:  UserIdentity{ID: 52, Role: common.RoleAdminUser},
		CodeID: code.ID,
		Action: UserRedemptionActionDisable,
		Reason: "risk review",
	})
	require.NoError(t, err)
	require.Equal(t, model.UserRedemptionStatusDisabled, disabled.Status)
	require.Equal(t, 52, disabled.DisabledBy)
	require.Equal(t, "risk review", disabled.DisabledReason)

	_, err = ClaimUserRedemptionCode(context.Background(), 51, code.Code)
	require.ErrorIs(t, err, ErrUserRedemptionCodeDisabled)

	enabled, err := AdminMutateUserRedemptionCode(context.Background(), UserRedemptionAdminCommand{
		Actor:  UserIdentity{ID: 52, Role: common.RoleAdminUser},
		CodeID: code.ID,
		Action: UserRedemptionActionEnable,
		Reason: "review complete",
	})
	require.NoError(t, err)
	require.Equal(t, model.UserRedemptionStatusActive, enabled.Status)
	require.Zero(t, enabled.DisabledAt)
	require.Zero(t, enabled.DisabledBy)
	require.Empty(t, enabled.DisabledReason)
}

func TestAdminUserRedemptionDisableAuditUsesOwnerLogAndAdminActor(t *testing.T) {
	setupUserRedemptionTestDB(t)
	seedWalletUser(t, 55, redemptionQuota(2))
	require.NoError(t, model.DB.Create(&model.User{
		Id: 56, Username: "admin56", AffCode: "admin56",
		Status: common.UserStatusEnabled, Role: common.RoleAdminUser,
	}).Error)
	code, err := CreateUserRedemptionCode(context.Background(), CreateUserRedemptionInput{
		CreatorUserID: 55,
		Kind:          model.UserRedemptionKindSingle,
		TotalQuota:    redemptionQuota(1),
		ExpiresAt:     common.GetTimestamp() + int64((2 * time.Hour).Seconds()),
	})
	require.NoError(t, err)

	_, err = AdminMutateUserRedemptionCode(context.Background(), UserRedemptionAdminCommand{
		Actor:  UserIdentity{ID: 56, Role: common.RoleAdminUser},
		CodeID: code.ID,
		Action: UserRedemptionActionDisable,
		Reason: "audit this hold",
	})
	require.NoError(t, err)

	var event model.AuditOutboxEvent
	require.NoError(t, model.DB.Where("event_type = ?", "user.redemption.disable").First(&event).Error)
	require.Equal(t, 56, event.ActorID)
	require.Equal(t, 55, event.LogUserID)
	require.Contains(t, event.Payload, `"reason":"audit this hold"`)
}

func TestAdminUserRedemptionTerminateSettlement(t *testing.T) {
	tests := []struct {
		name             string
		settlement       UserRedemptionSettlement
		wantStatus       string
		wantCreatorQuota int64
		wantRefunded     int64
		wantForfeited    int64
	}{
		{
			name:             "refunds remaining quota",
			settlement:       UserRedemptionSettlementRefund,
			wantStatus:       model.UserRedemptionStatusRefunded,
			wantCreatorQuota: redemptionQuota(5),
			wantRefunded:     redemptionQuota(2),
		},
		{
			name:             "forfeits remaining quota",
			settlement:       UserRedemptionSettlementForfeit,
			wantStatus:       model.UserRedemptionStatusForfeited,
			wantCreatorQuota: redemptionQuota(3),
			wantForfeited:    redemptionQuota(2),
		},
	}

	for index, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupUserRedemptionTestDB(t)
			creatorID := 60 + index*10
			adminID := creatorID + 1
			seedWalletUser(t, creatorID, redemptionQuota(5))
			require.NoError(t, model.DB.Create(&model.User{
				Id: adminID, Username: fmt.Sprintf("admin%d", adminID), AffCode: fmt.Sprintf("admin%d", adminID),
				Status: common.UserStatusEnabled, Role: common.RoleAdminUser,
			}).Error)

			code, err := CreateUserRedemptionCode(context.Background(), CreateUserRedemptionInput{
				CreatorUserID: creatorID,
				Kind:          model.UserRedemptionKindSingle,
				TotalQuota:    redemptionQuota(2),
				ExpiresAt:     common.GetTimestamp() + int64((2 * time.Hour).Seconds()),
			})
			require.NoError(t, err)

			terminated, err := AdminMutateUserRedemptionCode(context.Background(), UserRedemptionAdminCommand{
				Actor:      UserIdentity{ID: adminID, Role: common.RoleAdminUser},
				CodeID:     code.ID,
				Action:     UserRedemptionActionTerminate,
				Settlement: test.settlement,
				Reason:     "policy decision",
			})
			require.NoError(t, err)
			require.Equal(t, test.wantStatus, terminated.Status)
			require.Equal(t, test.wantRefunded, terminated.RefundedQuota)
			require.Equal(t, test.wantForfeited, terminated.ForfeitedQuota)
			require.Zero(t, UserRedemptionRemainingQuota(*terminated))

			var creator model.User
			require.NoError(t, model.DB.First(&creator, creatorID).Error)
			require.Equal(t, test.wantCreatorQuota, creator.Quota)
		})
	}
}

func TestAdminUserRedemptionTerminationIsIdempotentAndRejectsConflictingSettlement(t *testing.T) {
	setupUserRedemptionTestDB(t)
	seedWalletUser(t, 80, redemptionQuota(4))
	require.NoError(t, model.DB.Create(&model.User{
		Id: 81, Username: "admin81", AffCode: "admin81",
		Status: common.UserStatusEnabled, Role: common.RoleAdminUser,
	}).Error)

	code, err := CreateUserRedemptionCode(context.Background(), CreateUserRedemptionInput{
		CreatorUserID: 80,
		Kind:          model.UserRedemptionKindSingle,
		TotalQuota:    redemptionQuota(1),
		ExpiresAt:     common.GetTimestamp() + int64((2 * time.Hour).Seconds()),
	})
	require.NoError(t, err)
	command := UserRedemptionAdminCommand{
		Actor:      UserIdentity{ID: 81, Role: common.RoleAdminUser},
		CodeID:     code.ID,
		Action:     UserRedemptionActionTerminate,
		Settlement: UserRedemptionSettlementRefund,
		Reason:     "duplicate delivery",
	}

	_, err = AdminMutateUserRedemptionCode(context.Background(), command)
	require.NoError(t, err)
	_, err = AdminMutateUserRedemptionCode(context.Background(), command)
	require.NoError(t, err)

	var refundEntries int64
	require.NoError(t, model.DB.Model(&model.WalletLedgerEntry{}).
		Where("user_id = ? AND category = ?", 80, userRedemptionCategoryRefund).
		Count(&refundEntries).Error)
	require.Equal(t, int64(1), refundEntries)

	command.Settlement = UserRedemptionSettlementForfeit
	_, err = AdminMutateUserRedemptionCode(context.Background(), command)
	require.ErrorIs(t, err, ErrUserRedemptionConflict)
}

func TestAdminBatchUserRedemptionSettlementReturnsPartialResults(t *testing.T) {
	setupUserRedemptionTestDB(t)
	seedWalletUser(t, 82, redemptionQuota(6))
	require.NoError(t, model.DB.Create(&model.User{
		Id: 83, Username: "admin83", AffCode: "admin83",
		Status: common.UserStatusEnabled, Role: common.RoleAdminUser,
	}).Error)
	first, err := CreateUserRedemptionCode(context.Background(), CreateUserRedemptionInput{
		CreatorUserID: 82, Kind: model.UserRedemptionKindSingle,
		TotalQuota: redemptionQuota(1), ExpiresAt: common.GetTimestamp() + int64((2 * time.Hour).Seconds()),
	})
	require.NoError(t, err)
	second, err := CreateUserRedemptionCode(context.Background(), CreateUserRedemptionInput{
		CreatorUserID: 82, Kind: model.UserRedemptionKindSingle,
		TotalQuota: redemptionQuota(1), ExpiresAt: common.GetTimestamp() + int64((2 * time.Hour).Seconds()),
	})
	require.NoError(t, err)

	results, err := AdminBatchSettleUserRedemptionCodes(context.Background(), UserIdentity{ID: 83, Role: common.RoleAdminUser}, UserRedemptionBatchDispositionRequest{
		IDs:         []int64{first.ID, 999_999, second.ID},
		Disposition: UserRedemptionSettlementRefund,
		Reason:      "batch review",
	})
	require.NoError(t, err)
	require.Len(t, results, 3)
	require.True(t, results[0].Success)
	require.False(t, results[1].Success)
	require.Equal(t, "not_found", results[1].Code)
	require.True(t, results[2].Success)

	var settled []model.UserRedemptionCode
	require.NoError(t, model.DB.Where("id IN ?", []int64{first.ID, second.ID}).Order("id asc").Find(&settled).Error)
	require.Equal(t, []string{model.UserRedemptionStatusRefunded, model.UserRedemptionStatusRefunded}, []string{settled[0].Status, settled[1].Status})
}

func TestAdminListUserRedemptionsOrdersNewestFirstEvenWithPendingSettlement(t *testing.T) {
	setupUserRedemptionTestDB(t)
	seedWalletUser(t, 84, redemptionQuota(24))
	require.NoError(t, model.DB.Create(&model.User{
		Id: 85, Username: "admin85", AffCode: "admin85",
		Status: common.UserStatusEnabled, Role: common.RoleAdminUser,
	}).Error)
	var newestPending *model.UserRedemptionCode
	for i := 0; i < 21; i++ {
		pending, err := CreateUserRedemptionCode(context.Background(), CreateUserRedemptionInput{
			CreatorUserID: 84, Kind: model.UserRedemptionKindSingle,
			TotalQuota: redemptionQuota(1), ExpiresAt: common.GetTimestamp() + int64((2 * time.Hour).Seconds()),
		})
		require.NoError(t, err)
		require.NoError(t, model.DB.Model(&model.UserRedemptionCode{}).
			Where("id = ?", pending.ID).
			Update("status", model.UserRedemptionStatusExpiredPending).Error)
		newestPending = pending
	}
	newest, err := CreateUserRedemptionCode(context.Background(), CreateUserRedemptionInput{
		CreatorUserID: 84, Kind: model.UserRedemptionKindSingle,
		TotalQuota: redemptionQuota(1), ExpiresAt: common.GetTimestamp() + int64((2 * time.Hour).Seconds()),
	})
	require.NoError(t, err)

	result, err := ListUserRedemptionCodes(context.Background(), UserIdentity{ID: 85, Role: common.RoleAdminUser}, UserRedemptionListQuery{
		Page: 1, PageSize: 20, Deleted: "false",
	})
	require.NoError(t, err)
	require.Equal(t, int64(22), result.Total)
	require.Len(t, result.Items, 20)
	require.Equal(t, newest.ID, result.Items[0].ID)
	require.Equal(t, model.UserRedemptionStatusActive, result.Items[0].Status)
	require.Equal(t, newestPending.ID, result.Items[1].ID)
	require.Equal(t, model.UserRedemptionStatusExpiredPending, result.Items[1].Status)
}

func TestAdminUserRedemptionSoftDeletePreservesFinancialState(t *testing.T) {
	t.Run("active code is refunded and retained", func(t *testing.T) {
		setupUserRedemptionTestDB(t)
		seedWalletUser(t, 90, redemptionQuota(3))
		require.NoError(t, model.DB.Create(&model.User{
			Id: 91, Username: "admin91", AffCode: "admin91",
			Status: common.UserStatusEnabled, Role: common.RoleAdminUser,
		}).Error)
		code, err := CreateUserRedemptionCode(context.Background(), CreateUserRedemptionInput{
			CreatorUserID: 90,
			Kind:          model.UserRedemptionKindSingle,
			TotalQuota:    redemptionQuota(1),
			ExpiresAt:     common.GetTimestamp() + int64((2 * time.Hour).Seconds()),
		})
		require.NoError(t, err)

		deleted, err := AdminMutateUserRedemptionCode(context.Background(), UserRedemptionAdminCommand{
			Actor:      UserIdentity{ID: 91, Role: common.RoleAdminUser},
			CodeID:     code.ID,
			Action:     UserRedemptionActionDelete,
			Settlement: UserRedemptionSettlementRefund,
			Reason:     "remove from active inventory",
		})
		require.NoError(t, err)
		require.Equal(t, model.UserRedemptionStatusDeleted, deleted.Status)
		require.NotZero(t, deleted.DeletedAt)
		require.Equal(t, 91, deleted.DeletedBy)
		require.Equal(t, "remove from active inventory", deleted.DeleteReason)

		var creator model.User
		require.NoError(t, model.DB.First(&creator, 90).Error)
		require.Equal(t, redemptionQuota(3), creator.Quota)
		require.NoError(t, model.DB.First(&model.UserRedemptionCode{}, code.ID).Error)
	})

	t.Run("exhausted code becomes deleted without settlement", func(t *testing.T) {
		setupUserRedemptionTestDB(t)
		seedWalletUser(t, 100, redemptionQuota(2))
		seedWalletUser(t, 101, 0)
		require.NoError(t, model.DB.Create(&model.User{
			Id: 102, Username: "admin102", AffCode: "admin102",
			Status: common.UserStatusEnabled, Role: common.RoleAdminUser,
		}).Error)
		code, err := CreateUserRedemptionCode(context.Background(), CreateUserRedemptionInput{
			CreatorUserID: 100,
			Kind:          model.UserRedemptionKindSingle,
			TotalQuota:    redemptionQuota(1),
			ExpiresAt:     common.GetTimestamp() + int64((2 * time.Hour).Seconds()),
		})
		require.NoError(t, err)
		_, err = ClaimUserRedemptionCode(context.Background(), 101, code.Code)
		require.NoError(t, err)

		deleted, err := AdminMutateUserRedemptionCode(context.Background(), UserRedemptionAdminCommand{
			Actor:  UserIdentity{ID: 102, Role: common.RoleAdminUser},
			CodeID: code.ID,
			Action: UserRedemptionActionDelete,
			Reason: "archive completed code",
		})
		require.NoError(t, err)
		require.Equal(t, model.UserRedemptionStatusDeleted, deleted.Status)
		require.NotZero(t, deleted.DeletedAt)
		require.Zero(t, deleted.RefundedQuota)
		require.Zero(t, deleted.ForfeitedQuota)
	})
}

func TestFinalizeExpiredUserRedemptionCodesMarksDisabledCodesPendingWithoutRefund(t *testing.T) {
	setupUserRedemptionTestDB(t)
	seedWalletUser(t, 110, redemptionQuota(3))
	require.NoError(t, model.DB.Create(&model.User{
		Id: 111, Username: "admin111", AffCode: "admin111",
		Status: common.UserStatusEnabled, Role: common.RoleAdminUser,
	}).Error)
	code, err := CreateUserRedemptionCode(context.Background(), CreateUserRedemptionInput{
		CreatorUserID: 110,
		Kind:          model.UserRedemptionKindSingle,
		TotalQuota:    redemptionQuota(1),
		ExpiresAt:     common.GetTimestamp() + int64((2 * time.Hour).Seconds()),
	})
	require.NoError(t, err)
	_, err = AdminMutateUserRedemptionCode(context.Background(), UserRedemptionAdminCommand{
		Actor:  UserIdentity{ID: 111, Role: common.RoleAdminUser},
		CodeID: code.ID,
		Action: UserRedemptionActionDisable,
		Reason: "hold until expiry",
	})
	require.NoError(t, err)
	require.NoError(t, model.DB.Model(&model.UserRedemptionCode{}).
		Where("id = ?", code.ID).
		Update("expires_at", common.GetTimestamp()-10).Error)

	count, err := FinalizeExpiredUserRedemptionCodes(context.Background(), 50)
	require.NoError(t, err)
	require.Equal(t, 1, count)

	var finalized model.UserRedemptionCode
	require.NoError(t, model.DB.First(&finalized, code.ID).Error)
	require.Equal(t, model.UserRedemptionStatusExpiredPending, finalized.Status)
	require.Zero(t, finalized.RefundedQuota)

	var creator model.User
	require.NoError(t, model.DB.First(&creator, 110).Error)
	require.Equal(t, redemptionQuota(2), creator.Quota)

	var event model.AuditOutboxEvent
	require.NoError(t, model.DB.Where("event_type = ?", "user.redemption.expire_pending").First(&event).Error)
}

func TestClaimExpiredUserRedemptionMarksPendingWithoutRefund(t *testing.T) {
	setupUserRedemptionTestDB(t)
	seedWalletUser(t, 112, redemptionQuota(3))
	seedWalletUser(t, 113, 0)
	code, err := CreateUserRedemptionCode(context.Background(), CreateUserRedemptionInput{
		CreatorUserID: 112,
		Kind:          model.UserRedemptionKindSingle,
		TotalQuota:    redemptionQuota(1),
		ExpiresAt:     common.GetTimestamp() + int64((2 * time.Hour).Seconds()),
	})
	require.NoError(t, err)
	require.NoError(t, model.DB.Model(&model.UserRedemptionCode{}).
		Where("id = ?", code.ID).
		Update("expires_at", common.GetTimestamp()-10).Error)

	_, err = ClaimUserRedemptionCode(context.Background(), 113, code.Code)
	require.ErrorIs(t, err, ErrUserRedemptionExpired)

	var expired model.UserRedemptionCode
	require.NoError(t, model.DB.First(&expired, code.ID).Error)
	require.Equal(t, model.UserRedemptionStatusExpiredPending, expired.Status)
	require.Zero(t, expired.RefundedQuota)

	var creator model.User
	require.NoError(t, model.DB.First(&creator, 112).Error)
	require.Equal(t, redemptionQuota(2), creator.Quota)
}

func TestUserRedemptionLifecycleEnqueuesBusinessAuditEvents(t *testing.T) {
	setupUserRedemptionTestDB(t)
	seedWalletUser(t, 120, redemptionQuota(6))
	seedWalletUser(t, 121, 0)
	expires := common.GetTimestamp() + int64((2 * time.Hour).Seconds())

	claimedCode, err := CreateUserRedemptionCode(context.Background(), CreateUserRedemptionInput{
		CreatorUserID: 120, Kind: model.UserRedemptionKindSingle,
		TotalQuota: redemptionQuota(1), ExpiresAt: expires,
	})
	require.NoError(t, err)
	_, err = ClaimUserRedemptionCode(context.Background(), 121, claimedCode.Code)
	require.NoError(t, err)

	cancelledCode, err := CreateUserRedemptionCode(context.Background(), CreateUserRedemptionInput{
		CreatorUserID: 120, Kind: model.UserRedemptionKindSingle,
		TotalQuota: redemptionQuota(1), ExpiresAt: expires,
	})
	require.NoError(t, err)
	_, err = CancelUserRedemptionCode(context.Background(), UserIdentity{ID: 120, Role: common.RoleCommonUser}, cancelledCode.ID)
	require.NoError(t, err)

	expiredCode, err := CreateUserRedemptionCode(context.Background(), CreateUserRedemptionInput{
		CreatorUserID: 120, Kind: model.UserRedemptionKindSingle,
		TotalQuota: redemptionQuota(1), ExpiresAt: expires,
	})
	require.NoError(t, err)
	require.NoError(t, model.DB.Model(&model.UserRedemptionCode{}).
		Where("id = ?", expiredCode.ID).
		Update("expires_at", common.GetTimestamp()-10).Error)
	_, err = FinalizeExpiredUserRedemptionCodes(context.Background(), 50)
	require.NoError(t, err)

	wantEvents := []struct {
		eventType string
		actorID   int
		logUserID int
	}{
		{"user.redemption.create", 120, 120},
		{"user.redemption.claim", 121, 121},
		{"user.redemption.cancel_pending", 120, 120},
		{"user.redemption.expire_pending", 0, 120},
	}
	for _, want := range wantEvents {
		var event model.AuditOutboxEvent
		require.NoError(t, model.DB.Where("event_type = ?", want.eventType).First(&event).Error, want.eventType)
		require.Equal(t, want.actorID, event.ActorID, want.eventType)
		require.Equal(t, want.logUserID, event.LogUserID, want.eventType)
	}
}

func TestFinalizeExpiredUserRedemptionCodesNoopNotCounted(t *testing.T) {
	setupUserRedemptionTestDB(t)
	seedWalletUser(t, 40, redemptionQuota(4))
	expires := common.GetTimestamp() + int64((2 * time.Hour).Seconds())
	code, err := CreateUserRedemptionCode(context.Background(), CreateUserRedemptionInput{
		CreatorUserID: 40,
		Kind:          model.UserRedemptionKindSingle,
		TotalQuota:    redemptionQuota(1),
		ExpiresAt:     expires,
	})
	require.NoError(t, err)
	// not expired yet — finalize should count 0
	n, err := FinalizeExpiredUserRedemptionCodes(context.Background(), 50)
	require.NoError(t, err)
	require.Equal(t, 0, n)

	require.NoError(t, model.DB.Model(&model.UserRedemptionCode{}).Where("id = ?", code.ID).Update("expires_at", common.GetTimestamp()-10).Error)
	n, err = FinalizeExpiredUserRedemptionCodes(context.Background(), 50)
	require.NoError(t, err)
	require.Equal(t, 1, n)

	// second pass is a no-op and must not inflate the counter
	n, err = FinalizeExpiredUserRedemptionCodes(context.Background(), 50)
	require.NoError(t, err)
	require.Equal(t, 0, n)
}

func TestUserRedemptionExpiresAtBoundary(t *testing.T) {
	require.True(t, isUserRedemptionExpired(100, 100))
	require.True(t, isUserRedemptionExpired(99, 100))
	require.False(t, isUserRedemptionExpired(101, 100))
}

func TestCreateUserRedemptionReportsInsufficientBalanceAboveSingleRequestLimit(t *testing.T) {
	setupUserRedemptionTestDB(t)
	seedWalletUser(t, 130, int64(common.MaxQuota))

	_, err := CreateUserRedemptionCode(context.Background(), CreateUserRedemptionInput{
		CreatorUserID: 130,
		Kind:          model.UserRedemptionKindSingle,
		TotalQuota:    int64(common.MaxQuota) + 1,
		ExpiresAt:     common.GetTimestamp() + int64((2 * time.Hour).Seconds()),
	})
	var balanceError *UserRedemptionInsufficientBalanceError
	require.ErrorAs(t, err, &balanceError)
	require.Equal(t, int64(common.MaxQuota)+1, balanceError.Requested)
	require.Equal(t, int64(common.MaxQuota), balanceError.Available)
}

func TestAdminRefundInvalidatesCreatorCache(t *testing.T) {
	setupUserRedemptionTestDB(t)
	seedWalletUser(t, 131, redemptionQuota(3))
	require.NoError(t, model.DB.Create(&model.User{
		Id: 132, Username: "admin132", AffCode: "admin132",
		Status: common.UserStatusEnabled, Role: common.RoleAdminUser,
	}).Error)
	code, err := CreateUserRedemptionCode(context.Background(), CreateUserRedemptionInput{
		CreatorUserID: 131,
		Kind:          model.UserRedemptionKindSingle,
		TotalQuota:    redemptionQuota(1),
		ExpiresAt:     common.GetTimestamp() + int64((2 * time.Hour).Seconds()),
	})
	require.NoError(t, err)

	previousInvalidator := invalidateUserRedemptionCache
	invalidated := make([]int, 0, 1)
	invalidateUserRedemptionCache = func(userID int) error {
		invalidated = append(invalidated, userID)
		return nil
	}
	t.Cleanup(func() { invalidateUserRedemptionCache = previousInvalidator })

	_, err = AdminMutateUserRedemptionCode(context.Background(), UserRedemptionAdminCommand{
		Actor:      UserIdentity{ID: 132, Role: common.RoleAdminUser},
		CodeID:     code.ID,
		Action:     UserRedemptionActionTerminate,
		Settlement: UserRedemptionSettlementRefund,
		Reason:     "refund remaining quota",
	})
	require.NoError(t, err)
	require.Equal(t, []int{131}, invalidated)
}

func TestListUserRedemptionCodesDoesNotOverflowOffsetForHugePage(t *testing.T) {
	setupUserRedemptionTestDB(t)
	seedWalletUser(t, 140, redemptionQuota(2))
	_, err := CreateUserRedemptionCode(context.Background(), CreateUserRedemptionInput{
		CreatorUserID: 140,
		Kind:          model.UserRedemptionKindSingle,
		TotalQuota:    redemptionQuota(1),
		ExpiresAt:     common.GetTimestamp() + int64((2 * time.Hour).Seconds()),
	})
	require.NoError(t, err)

	maxInt := int(^uint(0) >> 1)
	page, err := ListUserRedemptionCodes(context.Background(), UserIdentity{ID: 140, Role: common.RoleCommonUser}, UserRedemptionListQuery{
		Page:     maxInt,
		PageSize: 20,
	})

	require.NoError(t, err)
	require.Empty(t, page.Items)
	require.Equal(t, int64(1), page.Total)
}
