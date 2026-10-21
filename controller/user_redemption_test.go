package controller

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	appi18n "github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestApiUserRedemptionErrorHidesWalletAdjustmentInternals(t *testing.T) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/user-redemption/", nil)

	expectedMessage := common.TranslateMessage(ctx, appi18n.MsgUserRedemptionInvalid)
	apiUserRedemptionError(ctx, service.ErrWalletInvalidAdjustment)

	response := decodeAPIResponse(t, recorder)
	require.False(t, response.Success)
	require.Equal(t, expectedMessage, response.Message)
	require.NotContains(t, response.Message, "wallet adjustment is invalid")
}

func TestApiUserRedemptionErrorMapsSubscriptionFailures(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		code   string
		actual string
	}{
		{
			name:   "reward type mismatch",
			err:    &service.UserRedemptionRewardTypeMismatchError{Expected: model.UserRedemptionRewardBalance, Actual: model.UserRedemptionRewardSubscription},
			code:   "USER_REDEMPTION_REWARD_TYPE_MISMATCH",
			actual: model.UserRedemptionRewardSubscription,
		},
		{name: "plan disabled", err: service.ErrUserRedemptionPlanDisabled, code: "USER_REDEMPTION_PLAN_DISABLED"},
		{name: "balance payment disabled", err: service.ErrUserRedemptionBalancePaymentDisabled, code: "USER_REDEMPTION_BALANCE_PAYMENT_DISABLED"},
		{name: "purchase limit", err: service.ErrUserRedemptionPlanPurchaseLimit, code: "USER_REDEMPTION_PLAN_PURCHASE_LIMIT"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest(http.MethodPost, "/api/user-redemption/claim", nil)

			apiUserRedemptionError(ctx, tt.err)

			var response struct {
				Success          bool   `json:"success"`
				Code             string `json:"code"`
				Message          string `json:"message"`
				ActualRewardType string `json:"actual_reward_type"`
			}
			require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
			require.False(t, response.Success)
			require.Equal(t, tt.code, response.Code)
			require.NotEmpty(t, response.Message)
			require.Equal(t, tt.actual, response.ActualRewardType)
		})
	}
}

func TestAPIWalletRedemptionErrorReturnsStableSafeReasons(t *testing.T) {
	tests := []struct {
		name string
		err  error
		code string
	}{
		{name: "empty code", err: service.ErrWalletRedemptionInvalid, code: "WALLET_REDEMPTION_INVALID"},
		{name: "ambiguous code", err: service.ErrWalletRedemptionAmbiguous, code: "WALLET_REDEMPTION_CONFLICT"},
		{name: "own user code", err: service.ErrUserRedemptionForbidden, code: "USER_REDEMPTION_OWN_CODE"},
		{name: "already claimed", err: service.ErrUserRedemptionAlreadyClaimed, code: "USER_REDEMPTION_ALREADY_CLAIMED"},
		{name: "disabled by administrator", err: service.ErrUserRedemptionCodeDisabled, code: "USER_REDEMPTION_DISABLED"},
		{name: "expired user code", err: service.ErrUserRedemptionExpired, code: "USER_REDEMPTION_EXPIRED"},
		{name: "exhausted user code", err: service.ErrUserRedemptionExhausted, code: "USER_REDEMPTION_EXHAUSTED"},
		{name: "unknown code", err: service.ErrUserRedemptionNotFound, code: "WALLET_REDEMPTION_UNAVAILABLE"},
		{name: "internal failure", err: errors.New("unexpected database failure"), code: "WALLET_REDEMPTION_RETRY"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest(http.MethodPost, "/api/user/topup", nil)
			ctx.Set(common.RequestIdKey, "wallet-redemption-test")

			apiWalletRedemptionError(ctx, tt.err)

			var response struct {
				Success   bool   `json:"success"`
				Code      string `json:"code"`
				Message   string `json:"message"`
				RequestID string `json:"request_id"`
			}
			require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
			require.False(t, response.Success)
			require.Equal(t, tt.code, response.Code)
			require.NotEmpty(t, response.Message)
			require.NotContains(t, response.Message, tt.err.Error())
			require.Equal(t, "wallet-redemption-test", response.RequestID)
		})
	}
}

func setupUserRedemptionControllerTest(t *testing.T) {
	t.Helper()
	setupUserRootBindingControllerTest(t)
	require.NoError(t, model.DB.AutoMigrate(
		&model.User{},
		&model.Redemption{},
		&model.WalletLedgerEntry{},
		&model.SubscriptionPlan{},
		&model.UserSubscription{},
		&model.SubscriptionPurchaseCounter{},
		&model.UserRedemptionCode{},
		&model.UserRedemptionClaim{},
		&model.UserRedemptionSubscriptionEntitlement{},
		&model.AuditOutboxEvent{},
	))
}

func seedControllerSubscriptionRedemption(t *testing.T, creatorID int) (*model.SubscriptionPlan, *model.UserRedemptionCode) {
	t.Helper()
	plan := &model.SubscriptionPlan{
		Title:               "controller subscription plan",
		PriceAmount:         1,
		Currency:            "USD",
		DurationUnit:        model.SubscriptionDurationMonth,
		DurationValue:       1,
		Enabled:             true,
		AllowBalancePay:     common.GetPointer(true),
		AllowWalletOverflow: common.GetPointer(true),
		TotalAmount:         1_000,
		QuotaResetPeriod:    model.SubscriptionResetNever,
	}
	require.NoError(t, model.DB.Create(plan).Error)
	code := &model.UserRedemptionCode{
		Code:          fmt.Sprintf("CONTROLLER-SUB-%d", creatorID),
		CreatorUserID: creatorID,
		RewardType:    model.UserRedemptionRewardSubscription,
		Kind:          model.UserRedemptionKindFixed,
		TotalQuota:    int64(common.QuotaFromFloat(common.QuotaPerUnit)),
		MaxClaims:     1,
		Status:        model.UserRedemptionStatusActive,
		ExpiresAt:     common.GetTimestamp() + 7200,
		ReserveOpID:   fmt.Sprintf("controller-sub-reserve-%d", creatorID),
	}
	require.NoError(t, model.DB.Create(code).Error)
	require.NoError(t, model.DB.Create(&model.UserRedemptionSubscriptionEntitlement{
		CodeID:              code.ID,
		PlanID:              plan.Id,
		PlanTitle:           plan.Title,
		Currency:            plan.Currency,
		PriceAmount:         plan.PriceAmount,
		UnitPaidQuota:       code.TotalQuota,
		DurationUnit:        plan.DurationUnit,
		DurationValue:       plan.DurationValue,
		TotalAmount:         plan.TotalAmount,
		QuotaResetPeriod:    plan.QuotaResetPeriod,
		AllowWalletOverflow: true,
	}).Error)
	return plan, code
}

func TestAdminUpdateSubscriptionPlanStatusSealsSubscriptionRedemptions(t *testing.T) {
	setupUserRedemptionControllerTest(t)
	confirmPaymentComplianceForTest(t)
	plan, code := seedControllerSubscriptionRedemption(t, 9101)
	ctx, recorder := newAuthenticatedContext(t, http.MethodPatch, fmt.Sprintf("/api/subscription/admin/plans/%d", plan.Id), map[string]any{"enabled": false}, 9001)
	ctx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(plan.Id)}}

	AdminUpdateSubscriptionPlanStatus(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	var stored model.UserRedemptionCode
	require.NoError(t, model.DB.First(&stored, code.ID).Error)
	require.Equal(t, model.UserRedemptionStatusRefundPending, stored.Status)
}

func TestAdminUpdateSubscriptionPlanSealsSubscriptionRedemptions(t *testing.T) {
	setupUserRedemptionControllerTest(t)
	confirmPaymentComplianceForTest(t)
	plan, code := seedControllerSubscriptionRedemption(t, 9102)
	plan.Enabled = false
	ctx, recorder := newAuthenticatedContext(t, http.MethodPut, fmt.Sprintf("/api/subscription/admin/plans/%d", plan.Id), map[string]any{"plan": plan}, 9002)
	ctx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(plan.Id)}}

	AdminUpdateSubscriptionPlan(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	var stored model.UserRedemptionCode
	require.NoError(t, model.DB.First(&stored, code.ID).Error)
	require.Equal(t, model.UserRedemptionStatusRefundPending, stored.Status)
}

func seedWalletControllerUser(t *testing.T, id int, username string, quota int64) {
	t.Helper()
	require.NoError(t, model.DB.Create(&model.User{
		Id:          id,
		Username:    username,
		Password:    "password-hash",
		DisplayName: username,
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		Group:       "default",
		Quota:       quota,
		AffCode:     username + "-aff",
	}).Error)
}

func TestCreateClaimUserRedemptionAPI(t *testing.T) {
	setupUserRedemptionControllerTest(t)
	redemptionUnit := int64(common.QuotaFromFloat(common.QuotaPerUnit))
	seedWalletControllerUser(t, 101, "creator", redemptionUnit*10)
	seedWalletControllerUser(t, 102, "claimer", 0)

	expires := common.GetTimestamp() + int64((2 * time.Hour).Seconds())
	createCtx, createRec := newAuthenticatedContext(t, http.MethodPost, "/api/user-redemption/", map[string]any{
		"kind":        "single",
		"total_quota": redemptionUnit * 2,
		"max_claims":  1,
		"expires_at":  expires,
	}, 101)
	createCtx.Set("role", common.RoleCommonUser)
	CreateUserRedemption(createCtx)
	createResp := decodeAPIResponse(t, createRec)
	require.True(t, createResp.Success, createResp.Message)
	require.Contains(t, string(createResp.Data), `"remaining_quota"`)

	var creator model.User
	require.NoError(t, model.DB.First(&creator, 101).Error)
	require.Equal(t, redemptionUnit*8, creator.Quota)

	var code model.UserRedemptionCode
	require.NoError(t, model.DB.Where("creator_user_id = ?", 101).First(&code).Error)
	require.Equal(t, model.UserRedemptionStatusActive, code.Status)

	claimCtx, claimRec := newAuthenticatedContext(t, http.MethodPost, "/api/user-redemption/claim", map[string]any{
		"code": code.Code,
	}, 102)
	claimCtx.Set("role", common.RoleCommonUser)
	ClaimUserRedemption(claimCtx)
	claimResp := decodeAPIResponse(t, claimRec)
	require.True(t, claimResp.Success, claimResp.Message)

	var claimer model.User
	require.NoError(t, model.DB.First(&claimer, 102).Error)
	require.Equal(t, redemptionUnit*2, claimer.Quota)
}

func TestCreateAndClaimSubscriptionRedemptionAPI(t *testing.T) {
	setupUserRedemptionControllerTest(t)
	unitQuota := int64(common.QuotaFromFloat(common.QuotaPerUnit))
	seedWalletControllerUser(t, 103, "subscription-code-creator", unitQuota*3)
	seedWalletControllerUser(t, 104, "subscription-code-claimer", 0)
	plan := &model.SubscriptionPlan{
		Title:               "API subscription plan",
		Subtitle:            "snapshot subtitle",
		PriceAmount:         1,
		Currency:            "USD",
		DurationUnit:        model.SubscriptionDurationMonth,
		DurationValue:       1,
		Enabled:             true,
		AllowBalancePay:     common.GetPointer(true),
		AllowWalletOverflow: common.GetPointer(true),
		TotalAmount:         2_000,
		QuotaResetPeriod:    model.SubscriptionResetMonthly,
	}
	require.NoError(t, model.DB.Create(plan).Error)

	createCtx, createRec := newAuthenticatedContext(t, http.MethodPost, "/api/user-redemption/", map[string]any{
		"reward_type": model.UserRedemptionRewardSubscription,
		"plan_id":     plan.Id,
		"units":       2,
		"expires_at":  common.GetTimestamp() + int64((2 * time.Hour).Seconds()),
	}, 103)
	createCtx.Set("role", common.RoleCommonUser)
	CreateUserRedemption(createCtx)

	createResp := decodeAPIResponse(t, createRec)
	require.True(t, createResp.Success, createResp.Message)
	require.Contains(t, string(createResp.Data), `"reward_type":"subscription"`)
	require.Contains(t, string(createResp.Data), `"plan_title":"API subscription plan"`)
	require.Contains(t, string(createResp.Data), `"unit_paid_quota":500000`)
	var code model.UserRedemptionCode
	require.NoError(t, model.DB.Where("creator_user_id = ?", 103).First(&code).Error)
	require.Equal(t, model.UserRedemptionRewardSubscription, code.RewardType)
	require.Equal(t, 2, code.MaxClaims)

	claimCtx, claimRec := newAuthenticatedContext(t, http.MethodPost, "/api/user-redemption/claim", map[string]any{
		"code":                 code.Code,
		"expected_reward_type": model.UserRedemptionRewardSubscription,
	}, 104)
	claimCtx.Set("role", common.RoleCommonUser)
	ClaimUserRedemption(claimCtx)

	claimResp := decodeAPIResponse(t, claimRec)
	require.True(t, claimResp.Success, claimResp.Message)
	require.Contains(t, string(claimResp.Data), `"reward_type":"subscription"`)
	require.Contains(t, string(claimResp.Data), `"user_subscription_id":`)
	var subscription model.UserSubscription
	require.NoError(t, model.DB.Where("user_id = ? AND plan_id = ?", 104, plan.Id).First(&subscription).Error)
	require.Equal(t, "redemption", subscription.Source)
}

func TestCreateSubscriptionRedemptionAPIRejectsBalanceFields(t *testing.T) {
	setupUserRedemptionControllerTest(t)
	seedWalletControllerUser(t, 105, "ambiguous-subscription-code", int64(common.QuotaFromFloat(common.QuotaPerUnit))*3)
	plan := &model.SubscriptionPlan{
		Title: "Ambiguous plan", PriceAmount: 1, Currency: "USD",
		DurationUnit: model.SubscriptionDurationMonth, DurationValue: 1,
		Enabled: true, AllowBalancePay: common.GetPointer(true),
	}
	require.NoError(t, model.DB.Create(plan).Error)
	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/user-redemption/", map[string]any{
		"reward_type": model.UserRedemptionRewardSubscription,
		"plan_id":     plan.Id, "units": 1,
		"total_quota": int64(common.QuotaFromFloat(common.QuotaPerUnit)),
		"expires_at":  common.GetTimestamp() + int64((2 * time.Hour).Seconds()),
	}, 105)
	ctx.Set("role", common.RoleCommonUser)

	CreateUserRedemption(ctx)

	response := decodeAPIResponse(t, recorder)
	require.False(t, response.Success)
	require.Equal(t, "USER_REDEMPTION_INVALID", response.Code)
}

func TestCreateUserRedemptionCompatibilityAmountUsesUSDQuotaUnit(t *testing.T) {
	setupUserRedemptionControllerTest(t)
	standardUnit := int64(common.QuotaFromFloat(common.QuotaPerUnit))
	seedWalletControllerUser(t, 301, "display-amount-creator", standardUnit+1000)

	expires := common.GetTimestamp() + int64((2 * time.Hour).Seconds())
	createCtx, createRec := newAuthenticatedContext(t, http.MethodPost, "/api/user-redemption/", map[string]any{
		"kind": "single", "amount": 1, "expires_at": expires,
	}, 301)
	createCtx.Set("role", common.RoleCommonUser)
	CreateUserRedemption(createCtx)
	response := decodeAPIResponse(t, createRec)
	require.True(t, response.Success, response.Message)

	var code model.UserRedemptionCode
	require.NoError(t, model.DB.Where("creator_user_id = ?", 301).First(&code).Error)
	require.Equal(t, standardUnit, code.TotalQuota)
}

func TestCreateUserRedemptionAcceptsLargeDecimalStringAmount(t *testing.T) {
	setupUserRedemptionControllerTest(t)
	largeQuota := int64(5000 * common.QuotaPerUnit)
	seedWalletControllerUser(t, 305, "large-amount-creator", largeQuota)

	expires := common.GetTimestamp() + int64((2 * time.Hour).Seconds())
	createCtx, createRec := newAuthenticatedContext(t, http.MethodPost, "/api/user-redemption/", map[string]any{
		"kind": "single", "amount": "5000.00", "expires_at": expires,
	}, 305)
	createCtx.Set("role", common.RoleCommonUser)
	CreateUserRedemption(createCtx)

	response := decodeAPIResponse(t, createRec)
	require.True(t, response.Success, response.Message)
	var code model.UserRedemptionCode
	require.NoError(t, model.DB.Where("creator_user_id = ?", 305).First(&code).Error)
	require.Equal(t, largeQuota, code.TotalQuota)
}

func TestCreateUserRedemptionAcceptsOneHourExpiryAfterTransportDelay(t *testing.T) {
	setupUserRedemptionControllerTest(t)
	totalQuota := int64(common.QuotaFromFloat(12.34 * common.QuotaPerUnit))
	seedWalletControllerUser(t, 303, "one-hour-red-packet", totalQuota+1000)

	createCtx, createRec := newAuthenticatedContext(t, http.MethodPost, "/api/user-redemption/", map[string]any{
		"kind":       "random",
		"amount":     12.34,
		"max_claims": 3,
		"expires_at": common.GetTimestamp() + int64(time.Hour/time.Second) - 1,
	}, 303)
	createCtx.Set("role", common.RoleCommonUser)

	CreateUserRedemption(createCtx)

	response := decodeAPIResponse(t, createRec)
	require.True(t, response.Success, response.Message)
	var code model.UserRedemptionCode
	require.NoError(t, model.DB.Where("creator_user_id = ?", 303).First(&code).Error)
	require.Equal(t, totalQuota, code.TotalQuota)
	require.Equal(t, 3, code.MaxClaims)
}

func TestGetUserRedemptionDetailIncludesEmptyClaimsArray(t *testing.T) {
	setupUserRedemptionControllerTest(t)
	standardUnit := int64(common.QuotaFromFloat(common.QuotaPerUnit))
	seedWalletControllerUser(t, 304, "empty-claim-history", standardUnit*2)
	code, err := service.CreateUserRedemptionCode(t.Context(), service.CreateUserRedemptionInput{
		CreatorUserID: 304,
		Kind:          model.UserRedemptionKindSingle,
		TotalQuota:    standardUnit,
		ExpiresAt:     common.GetTimestamp() + int64((2 * time.Hour).Seconds()),
	})
	require.NoError(t, err)

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, fmt.Sprintf("/api/user-redemption/%d", code.ID), nil, 304)
	ctx.Params = gin.Params{{Key: "id", Value: fmt.Sprint(code.ID)}}
	ctx.Set("role", common.RoleCommonUser)

	GetMyUserRedemption(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	require.Contains(t, string(response.Data), `"claims":[]`)
}

func TestCreateUserRedemptionRejectsAmbiguousQuotaSources(t *testing.T) {
	setupUserRedemptionControllerTest(t)
	standardUnit := int64(common.QuotaFromFloat(common.QuotaPerUnit))
	seedWalletControllerUser(t, 302, "ambiguous-quota", standardUnit*5)
	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/user-redemption/", map[string]any{
		"kind":        "single",
		"amount":      1,
		"total_quota": standardUnit * 2,
		"expires_at":  common.GetTimestamp() + int64((2 * time.Hour).Seconds()),
	}, 302)
	ctx.Set("role", common.RoleCommonUser)

	CreateUserRedemption(ctx)

	response := decodeAPIResponse(t, recorder)
	require.False(t, response.Success)
	var count int64
	require.NoError(t, model.DB.Model(&model.UserRedemptionCode{}).Where("creator_user_id = ?", 302).Count(&count).Error)
	require.Zero(t, count)
}

func TestCreateUserRedemptionAPILocalizesInsufficientWalletBalance(t *testing.T) {
	setupUserRedemptionControllerTest(t)
	seedWalletControllerUser(t, 103, "insufficient-creator", 0)

	expires := common.GetTimestamp() + int64((2 * time.Hour).Seconds())
	createCtx, createRec := newAuthenticatedContext(t, http.MethodPost, "/api/user-redemption/", map[string]any{
		"kind":        "single",
		"total_quota": common.QuotaFromFloat(common.QuotaPerUnit),
		"expires_at":  expires,
	}, 103)
	createCtx.Set("role", common.RoleCommonUser)
	createCtx.Request.Header.Set("Accept-Language", "zh-CN")
	CreateUserRedemption(createCtx)

	response := decodeAPIResponse(t, createRec)
	require.False(t, response.Success)
	require.Equal(t, "兑换码额度 1.00 超过当前可用余额 0.00", response.Message)
}

func TestCancelUserRedemptionAPIMarksPending(t *testing.T) {
	setupUserRedemptionControllerTest(t)
	redemptionUnit := int64(common.QuotaFromFloat(common.QuotaPerUnit))
	seedWalletControllerUser(t, 201, "cancel-user", redemptionUnit*5)
	expires := common.GetTimestamp() + int64((2 * time.Hour).Seconds())

	createCtx, createRec := newAuthenticatedContext(t, http.MethodPost, "/api/user-redemption/", map[string]any{
		"kind":        "single",
		"total_quota": redemptionUnit,
		"expires_at":  expires,
	}, 201)
	createCtx.Set("role", common.RoleCommonUser)
	CreateUserRedemption(createCtx)
	require.True(t, decodeAPIResponse(t, createRec).Success)

	var code model.UserRedemptionCode
	require.NoError(t, model.DB.Where("creator_user_id = ?", 201).First(&code).Error)

	cancelCtx, cancelRec := newAuthenticatedContext(t, http.MethodPost, fmt.Sprintf("/api/user-redemption/%d/cancel", code.ID), nil, 201)
	cancelCtx.Params = gin.Params{{Key: "id", Value: fmt.Sprint(code.ID)}}
	cancelCtx.Set("role", common.RoleCommonUser)
	CancelMyUserRedemption(cancelCtx)
	require.True(t, decodeAPIResponse(t, cancelRec).Success)

	var user model.User
	require.NoError(t, model.DB.First(&user, 201).Error)
	require.Equal(t, redemptionUnit*4, user.Quota)
}

func TestAdminUserRedemptionGovernanceAPI(t *testing.T) {
	setupUserRedemptionControllerTest(t)
	redemptionUnit := int64(common.QuotaFromFloat(common.QuotaPerUnit))
	seedWalletControllerUser(t, 401, "governance-owner", redemptionUnit*3)
	seedWalletControllerUser(t, 402, "governance-admin", 0)
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", 402).Update("role", common.RoleAdminUser).Error)
	code, err := service.CreateUserRedemptionCode(t.Context(), service.CreateUserRedemptionInput{
		CreatorUserID: 401,
		Kind:          model.UserRedemptionKindSingle,
		TotalQuota:    redemptionUnit,
		ExpiresAt:     common.GetTimestamp() + int64((2 * time.Hour).Seconds()),
	})
	require.NoError(t, err)

	disableCtx, disableRec := newAuthenticatedContext(t, http.MethodPost, fmt.Sprintf("/api/user-redemption/admin/%d/disable", code.ID), map[string]any{"reason": "review"}, 402)
	disableCtx.Params = gin.Params{{Key: "id", Value: fmt.Sprint(code.ID)}}
	disableCtx.Set("role", common.RoleAdminUser)
	DisableAdminUserRedemption(disableCtx)
	require.True(t, decodeAPIResponse(t, disableRec).Success)

	enableCtx, enableRec := newAuthenticatedContext(t, http.MethodPost, fmt.Sprintf("/api/user-redemption/admin/%d/enable", code.ID), map[string]any{"reason": "clear"}, 402)
	enableCtx.Params = gin.Params{{Key: "id", Value: fmt.Sprint(code.ID)}}
	enableCtx.Set("role", common.RoleAdminUser)
	EnableAdminUserRedemption(enableCtx)
	require.True(t, decodeAPIResponse(t, enableRec).Success)

	terminateCtx, terminateRec := newAuthenticatedContext(t, http.MethodPost, fmt.Sprintf("/api/user-redemption/admin/%d/terminate", code.ID), map[string]any{
		"settlement": "forfeit",
		"reason":     "policy",
	}, 402)
	terminateCtx.Params = gin.Params{{Key: "id", Value: fmt.Sprint(code.ID)}}
	terminateCtx.Set("role", common.RoleAdminUser)
	TerminateAdminUserRedemption(terminateCtx)
	require.True(t, decodeAPIResponse(t, terminateRec).Success)

	deleteCtx, deleteRec := newAuthenticatedContext(t, http.MethodPost, fmt.Sprintf("/api/user-redemption/admin/%d/delete", code.ID), map[string]any{"reason": "archive"}, 402)
	deleteCtx.Params = gin.Params{{Key: "id", Value: fmt.Sprint(code.ID)}}
	deleteCtx.Set("role", common.RoleAdminUser)
	DeleteAdminUserRedemption(deleteCtx)
	require.True(t, decodeAPIResponse(t, deleteRec).Success)

	var stored model.UserRedemptionCode
	require.NoError(t, model.DB.First(&stored, code.ID).Error)
	require.Equal(t, model.UserRedemptionStatusDeleted, stored.Status)
	require.NotZero(t, stored.DeletedAt)
}

func TestBatchSettleAdminUserRedemptionsReturnsPartialResults(t *testing.T) {
	setupUserRedemptionControllerTest(t)
	redemptionUnit := int64(common.QuotaFromFloat(common.QuotaPerUnit))
	seedWalletControllerUser(t, 430, "batch-owner", redemptionUnit*3)
	seedWalletControllerUser(t, 431, "batch-admin", 0)
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", 431).Update("role", common.RoleAdminUser).Error)
	code, err := service.CreateUserRedemptionCode(t.Context(), service.CreateUserRedemptionInput{
		CreatorUserID: 430, Kind: model.UserRedemptionKindSingle, TotalQuota: redemptionUnit,
		ExpiresAt: common.GetTimestamp() + int64((2 * time.Hour).Seconds()),
	})
	require.NoError(t, err)

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/user-redemption/admin/batch-settlement", map[string]any{
		"ids": []int64{code.ID, 999_999}, "disposition": "forfeit", "reason": "batch policy",
	}, 431)
	ctx.Set("role", common.RoleAdminUser)
	BatchSettleAdminUserRedemptions(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	var results []service.UserRedemptionBatchDispositionResult
	require.NoError(t, common.Unmarshal(response.Data, &results))
	require.Len(t, results, 2)
	require.True(t, results[0].Success)
	require.False(t, results[1].Success)
	require.Equal(t, "not_found", results[1].Code)
}

func TestListAdminUserRedemptionsAppliesGovernanceFilters(t *testing.T) {
	setupUserRedemptionControllerTest(t)
	unit := int64(common.QuotaFromFloat(common.QuotaPerUnit))
	seedWalletControllerUser(t, 410, "filter-owner", unit*3)
	seedWalletControllerUser(t, 411, "filter-admin", 0)
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", 411).Update("role", common.RoleAdminUser).Error)
	expires := common.GetTimestamp() + int64((2 * time.Hour).Seconds())
	_, err := service.CreateUserRedemptionCode(t.Context(), service.CreateUserRedemptionInput{
		CreatorUserID: 410, Kind: model.UserRedemptionKindSingle, TotalQuota: unit, ExpiresAt: expires,
	})
	require.NoError(t, err)
	deletedCode, err := service.CreateUserRedemptionCode(t.Context(), service.CreateUserRedemptionInput{
		CreatorUserID: 410, Kind: model.UserRedemptionKindSingle, TotalQuota: unit, ExpiresAt: expires,
	})
	require.NoError(t, err)
	_, err = service.AdminMutateUserRedemptionCode(t.Context(), service.UserRedemptionAdminCommand{
		Actor:      service.UserIdentity{ID: 411, Role: common.RoleAdminUser},
		CodeID:     deletedCode.ID,
		Action:     service.UserRedemptionActionDelete,
		Settlement: service.UserRedemptionSettlementRefund,
		Reason:     "archive",
	})
	require.NoError(t, err)

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/user-redemption/admin/?creator_username=filter&deleted=true", nil, 411)
	ctx.Set("role", common.RoleAdminUser)
	ListAdminUserRedemptions(ctx)
	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	var page service.UserRedemptionListResult
	require.NoError(t, common.Unmarshal(response.Data, &page))
	require.Equal(t, int64(1), page.Total)
	require.True(t, page.Items[0].Deleted)
}

func TestListMyUserRedemptionsRootIncludesSoftDeletedHistory(t *testing.T) {
	setupUserRedemptionControllerTest(t)
	const ownerID = 412
	const otherID = 413
	seedWalletControllerUser(t, ownerID, "root-owner", 0)
	seedWalletControllerUser(t, otherID, "other-owner", 0)
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", ownerID).Update("role", common.RoleRootUser).Error)

	now := common.GetTimestamp()
	rows := []model.UserRedemptionCode{
		{
			Code: "ROOT-OWNER-ACTIVE", CreatorUserID: ownerID, IssuedBy: ownerID,
			RewardType: model.UserRedemptionRewardBalance, Kind: model.UserRedemptionKindSingle,
			TotalQuota: 1, MaxClaims: 1, Status: model.UserRedemptionStatusActive,
			ExpiresAt: now + 3600, ReserveOpID: "root-owner-active-reserve",
		},
		{
			Code: "ROOT-OWNER-DELETED", CreatorUserID: ownerID, IssuedBy: ownerID,
			RewardType: model.UserRedemptionRewardBalance, Kind: model.UserRedemptionKindSingle,
			TotalQuota: 1, MaxClaims: 1, Status: model.UserRedemptionStatusDeleted,
			ExpiresAt: now + 3600, DeletedAt: now, DeletedBy: ownerID,
			ReserveOpID: "root-owner-deleted-reserve",
		},
		{
			Code: "OTHER-OWNER-ACTIVE", CreatorUserID: otherID, IssuedBy: otherID,
			RewardType: model.UserRedemptionRewardBalance, Kind: model.UserRedemptionKindSingle,
			TotalQuota: 1, MaxClaims: 1, Status: model.UserRedemptionStatusActive,
			ExpiresAt: now + 3600, ReserveOpID: "other-owner-active-reserve",
		},
	}
	require.NoError(t, model.DB.Create(&rows).Error)

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet,
		"/api/user-redemption/?page=1&page_size=20&deleted=all", nil, ownerID)
	ctx.Set("role", common.RoleRootUser)
	ListMyUserRedemptions(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	var page service.UserRedemptionListResult
	require.NoError(t, common.Unmarshal(response.Data, &page))
	require.Equal(t, int64(2), page.Total)
	require.Len(t, page.Items, 2)
	require.Equal(t, ownerID, page.Items[0].CreatorUserID)
	require.Equal(t, ownerID, page.Items[1].CreatorUserID)
	require.True(t, page.Items[0].Deleted || page.Items[1].Deleted)
}

func TestCancelAdminUserRedemptionRequiresSettlementAndReason(t *testing.T) {
	setupUserRedemptionControllerTest(t)
	redemptionUnit := int64(common.QuotaFromFloat(common.QuotaPerUnit))
	seedWalletControllerUser(t, 420, "admin-cancel-owner", redemptionUnit*3)
	seedWalletControllerUser(t, 421, "admin-cancel-admin", 0)
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", 421).Update("role", common.RoleAdminUser).Error)
	code, err := service.CreateUserRedemptionCode(t.Context(), service.CreateUserRedemptionInput{
		CreatorUserID: 420,
		Kind:          model.UserRedemptionKindSingle,
		TotalQuota:    redemptionUnit,
		ExpiresAt:     common.GetTimestamp() + int64(time.Hour/time.Second),
	})
	require.NoError(t, err)

	withoutSettlementCtx, withoutSettlementRec := newAuthenticatedContext(t, http.MethodPost,
		fmt.Sprintf("/api/user-redemption/admin/%d/cancel", code.ID),
		map[string]any{"reason": "policy"}, 421)
	withoutSettlementCtx.Params = gin.Params{{Key: "id", Value: fmt.Sprint(code.ID)}}
	withoutSettlementCtx.Set("role", common.RoleAdminUser)
	CancelAdminUserRedemption(withoutSettlementCtx)
	withoutSettlementResponse := decodeAPIResponse(t, withoutSettlementRec)
	require.False(t, withoutSettlementResponse.Success)

	forfeitCtx, forfeitRec := newAuthenticatedContext(t, http.MethodPost,
		fmt.Sprintf("/api/user-redemption/admin/%d/cancel", code.ID),
		map[string]any{"settlement": "forfeit", "reason": "policy"}, 421)
	forfeitCtx.Params = gin.Params{{Key: "id", Value: fmt.Sprint(code.ID)}}
	forfeitCtx.Set("role", common.RoleAdminUser)
	CancelAdminUserRedemption(forfeitCtx)
	forfeitResponse := decodeAPIResponse(t, forfeitRec)
	require.True(t, forfeitResponse.Success, forfeitResponse.Message)

	var stored model.UserRedemptionCode
	require.NoError(t, model.DB.First(&stored, code.ID).Error)
	require.Equal(t, model.UserRedemptionStatusForfeited, stored.Status)
	require.Equal(t, redemptionUnit, stored.ForfeitedQuota)
	require.Zero(t, stored.RefundedQuota)
	require.Equal(t, "policy", stored.TerminalReason)
}
