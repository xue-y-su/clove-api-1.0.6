package controller

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

type createUserRedemptionRequest struct {
	RewardType string          `json:"reward_type"`
	Kind       string          `json:"kind"`
	Quota      json.RawMessage `json:"quota,omitempty"` // optional raw quota for API clients
	TotalQuota json.RawMessage `json:"total_quota,omitempty"`
	MaxClaims  int             `json:"max_claims"`
	PlanID     int             `json:"plan_id"`
	Units      int             `json:"units"`
	ExpiresAt  int64           `json:"expires_at"`
	Amount     json.RawMessage `json:"amount,omitempty"`
}

type claimUserRedemptionRequest struct {
	Code               string `json:"code"`
	ExpectedRewardType string `json:"expected_reward_type"`
}

type adminUserRedemptionMutationRequest struct {
	Settlement service.UserRedemptionSettlement `json:"settlement"`
	Reason     string                           `json:"reason"`
}

func apiUserRedemptionError(c *gin.Context, err error) {
	key := ""
	code := "USER_REDEMPTION_ERROR"
	field := ""
	var args map[string]any
	var insufficient *service.UserRedemptionInsufficientBalanceError
	var rewardTypeMismatch *service.UserRedemptionRewardTypeMismatchError
	switch {
	case errors.As(err, &insufficient):
		key = i18n.MsgUserRedemptionInsufficientDetail
		code = "REDEMPTION_AMOUNT_EXCEEDS_BALANCE"
		field = "amount"
		args = map[string]any{
			"Requested": common.WalletAmountFromQuota(insufficient.Requested),
			"Available": common.WalletAmountFromQuota(insufficient.Available),
		}
	case errors.Is(err, common.ErrWalletAmountRequired):
		key = i18n.MsgUserRedemptionAmountRequired
		code = "REDEMPTION_AMOUNT_REQUIRED"
		field = "amount"
	case errors.Is(err, common.ErrWalletAmountNonPositive):
		key = i18n.MsgUserRedemptionAmountNonPositive
		code = "REDEMPTION_AMOUNT_NON_POSITIVE"
		field = "amount"
	case errors.Is(err, common.ErrWalletAmountPrecision):
		key = i18n.MsgUserRedemptionAmountPrecision
		code = "REDEMPTION_AMOUNT_PRECISION"
		field = "amount"
	case errors.Is(err, common.ErrWalletAmountOverflow):
		key = i18n.MsgUserRedemptionAmountOverflow
		code = "REDEMPTION_AMOUNT_OVERFLOW"
		field = "amount"
	case errors.Is(err, common.ErrWalletAmountInvalid):
		key = i18n.MsgUserRedemptionInvalid
		code = "REDEMPTION_AMOUNT_INVALID"
		field = "amount"
	case errors.Is(err, service.ErrWalletInvalidAdjustment):
		key = i18n.MsgUserRedemptionInvalid
		code = "REDEMPTION_WALLET_ADJUSTMENT_INVALID"
		field = "amount"
	case errors.Is(err, service.ErrUserRedemptionInvalid):
		key = i18n.MsgUserRedemptionInvalid
		code = "USER_REDEMPTION_INVALID"
	case errors.Is(err, service.ErrUserRedemptionNotFound):
		key = i18n.MsgUserRedemptionNotFound
	case errors.Is(err, service.ErrUserRedemptionForbidden):
		key = i18n.MsgUserRedemptionForbidden
	case errors.Is(err, service.ErrUserRedemptionInactive):
		key = i18n.MsgUserRedemptionInactive
	case errors.Is(err, service.ErrUserRedemptionCodeDisabled):
		key = i18n.MsgUserRedemptionCodeDisabled
		code = "USER_REDEMPTION_DISABLED"
	case errors.Is(err, service.ErrUserRedemptionExpired):
		key = i18n.MsgUserRedemptionExpired
	case errors.Is(err, service.ErrUserRedemptionAlreadyClaimed):
		key = i18n.MsgUserRedemptionAlreadyClaimed
	case errors.Is(err, service.ErrUserRedemptionExhausted):
		key = i18n.MsgUserRedemptionExhausted
	case errors.Is(err, service.ErrUserRedemptionInsufficient):
		key = i18n.MsgUserRedemptionInsufficientBalance
	case errors.As(err, &rewardTypeMismatch):
		key = i18n.MsgUserRedemptionRewardTypeMismatch
		code = "USER_REDEMPTION_REWARD_TYPE_MISMATCH"
	case errors.Is(err, service.ErrUserRedemptionPlanDisabled):
		key = i18n.MsgUserRedemptionPlanDisabled
		code = "USER_REDEMPTION_PLAN_DISABLED"
	case errors.Is(err, service.ErrUserRedemptionBalancePaymentDisabled):
		key = i18n.MsgUserRedemptionBalancePayDisabled
		code = "USER_REDEMPTION_BALANCE_PAYMENT_DISABLED"
	case errors.Is(err, service.ErrUserRedemptionPlanPurchaseLimit):
		key = i18n.MsgUserRedemptionPlanPurchaseLimit
		code = "USER_REDEMPTION_PLAN_PURCHASE_LIMIT"
	case errors.Is(err, service.ErrUserRedemptionConflict):
		key = i18n.MsgUserRedemptionConflict
	case errors.Is(err, service.ErrUserRedemptionDisabled):
		key = i18n.MsgUserRedemptionDisabled
	case errors.Is(err, service.ErrUserRedemptionModeDisabled):
		key = i18n.MsgUserRedemptionModeDisabled
	case errors.Is(err, service.ErrUserRedemptionActiveLimit):
		key = i18n.MsgUserRedemptionActiveLimit
	case errors.Is(err, service.ErrUserRedemptionClaimsLimit):
		key = i18n.MsgUserRedemptionClaimsLimit
	case errors.Is(err, service.ErrUserRedemptionExpiryLimit):
		key = i18n.MsgUserRedemptionExpiryLimit
	default:
		common.ApiError(c, err)
		return
	}
	message := common.TranslateMessage(c, key, args)
	payload := gin.H{
		"success":    false,
		"code":       code,
		"message":    message,
		"request_id": c.GetString(common.RequestIdKey),
	}
	if field != "" {
		payload["field"] = field
	}
	if rewardTypeMismatch != nil {
		payload["actual_reward_type"] = rewardTypeMismatch.Actual
	}
	c.JSON(http.StatusOK, payload)
}

func apiWalletRedemptionError(c *gin.Context, err error) {
	key := i18n.MsgRedeemFailed
	code := "WALLET_REDEMPTION_RETRY"
	switch {
	case errors.Is(err, service.ErrWalletRedemptionInvalid):
		key = i18n.MsgRedemptionInvalid
		code = "WALLET_REDEMPTION_INVALID"
	case errors.Is(err, service.ErrWalletRedemptionAmbiguous):
		key = i18n.MsgWalletRedemptionConflict
		code = "WALLET_REDEMPTION_CONFLICT"
	case errors.Is(err, service.ErrUserRedemptionForbidden):
		key = i18n.MsgWalletRedemptionOwnCode
		code = "USER_REDEMPTION_OWN_CODE"
	case errors.Is(err, service.ErrUserRedemptionAlreadyClaimed):
		key = i18n.MsgUserRedemptionAlreadyClaimed
		code = "USER_REDEMPTION_ALREADY_CLAIMED"
	case errors.Is(err, service.ErrUserRedemptionCodeDisabled):
		key = i18n.MsgUserRedemptionCodeDisabled
		code = "USER_REDEMPTION_DISABLED"
	case errors.Is(err, service.ErrUserRedemptionExpired), errors.Is(err, model.ErrRedemptionExpired):
		key = i18n.MsgRedemptionExpired
		code = "USER_REDEMPTION_EXPIRED"
	case errors.Is(err, service.ErrUserRedemptionExhausted), errors.Is(err, model.ErrRedemptionUsed):
		key = i18n.MsgRedemptionUsed
		code = "USER_REDEMPTION_EXHAUSTED"
	case errors.Is(err, service.ErrUserRedemptionNotFound), errors.Is(err, service.ErrUserRedemptionInactive), errors.Is(err, model.ErrRedemptionInvalid):
		key = i18n.MsgRedemptionInvalid
		code = "WALLET_REDEMPTION_UNAVAILABLE"
	}
	c.JSON(http.StatusOK, gin.H{
		"success":    false,
		"code":       code,
		"message":    common.TranslateMessage(c, key),
		"request_id": c.GetString(common.RequestIdKey),
	})
}

func userRedemptionJSONValueSet(value json.RawMessage) bool {
	raw := strings.TrimSpace(string(value))
	return raw != "" && raw != "null"
}

func parseUserRedemptionRawQuota(value json.RawMessage) (int64, error) {
	raw := strings.TrimSpace(common.JsonRawMessageToString(value))
	quota, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		if errors.Is(err, strconv.ErrRange) {
			return 0, common.ErrWalletAmountOverflow
		}
		return 0, common.ErrWalletAmountInvalid
	}
	if quota <= 0 {
		return 0, common.ErrWalletAmountNonPositive
	}
	return quota, nil
}

func CreateUserRedemption(c *gin.Context) {
	var req createUserRedemptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	rewardType := strings.TrimSpace(strings.ToLower(req.RewardType))
	if rewardType == "" {
		rewardType = model.UserRedemptionRewardBalance
	}
	quotaSources := 0
	if userRedemptionJSONValueSet(req.Quota) {
		quotaSources++
	}
	if userRedemptionJSONValueSet(req.TotalQuota) {
		quotaSources++
	}
	if userRedemptionJSONValueSet(req.Amount) {
		quotaSources++
	}
	if rewardType == model.UserRedemptionRewardSubscription {
		if quotaSources != 0 || strings.TrimSpace(req.Kind) != "" || req.MaxClaims != 0 || req.PlanID <= 0 || req.Units <= 0 {
			apiUserRedemptionError(c, service.ErrUserRedemptionInvalid)
			return
		}
		code, err := service.CreateSubscriptionRedemptionCode(c.Request.Context(), service.CreateSubscriptionRedemptionInput{
			CreatorUserID: c.GetInt("id"),
			PlanID:        req.PlanID,
			Units:         req.Units,
			ExpiresAt:     req.ExpiresAt,
		})
		if err != nil {
			apiUserRedemptionError(c, err)
			return
		}
		view, err := getUserRedemptionCodeViewForContext(c, code.ID)
		if err != nil {
			apiUserRedemptionError(c, err)
			return
		}
		common.ApiSuccess(c, view)
		return
	}
	if rewardType != model.UserRedemptionRewardBalance || req.PlanID != 0 || req.Units != 0 {
		apiUserRedemptionError(c, service.ErrUserRedemptionInvalid)
		return
	}
	if quotaSources > 1 {
		apiUserRedemptionError(c, service.ErrUserRedemptionInvalid)
		return
	}

	if quotaSources == 0 {
		apiUserRedemptionError(c, common.ErrWalletAmountRequired)
		return
	}

	var totalQuota int64
	var conversionErr error
	if userRedemptionJSONValueSet(req.Quota) {
		totalQuota, conversionErr = parseUserRedemptionRawQuota(req.Quota)
	} else if userRedemptionJSONValueSet(req.TotalQuota) {
		totalQuota, conversionErr = parseUserRedemptionRawQuota(req.TotalQuota)
	} else {
		totalQuota, conversionErr = common.WalletQuotaFromAmountString(common.JsonRawMessageToString(req.Amount))
	}
	if conversionErr != nil {
		apiUserRedemptionError(c, conversionErr)
		return
	}
	code, err := service.CreateUserRedemptionCode(c.Request.Context(), service.CreateUserRedemptionInput{
		CreatorUserID: c.GetInt("id"),
		Kind:          req.Kind,
		TotalQuota:    totalQuota,
		MaxClaims:     req.MaxClaims,
		ExpiresAt:     req.ExpiresAt,
	})
	if err != nil {
		apiUserRedemptionError(c, err)
		return
	}
	view, err := getUserRedemptionCodeViewForContext(c, code.ID)
	if err != nil {
		apiUserRedemptionError(c, err)
		return
	}
	common.ApiSuccess(c, view)
}

func ClaimUserRedemption(c *gin.Context) {
	var req claimUserRedemptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	claim, err := service.ClaimUserRedemptionCodeWithInput(c.Request.Context(), service.ClaimUserRedemptionInput{
		ClaimerUserID:      c.GetInt("id"),
		Code:               req.Code,
		ExpectedRewardType: req.ExpectedRewardType,
	})
	if err != nil {
		apiUserRedemptionError(c, err)
		return
	}
	common.ApiSuccess(c, claim)
}

func ListMyUserRedemptions(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", strconv.Itoa(common.ItemsPerPage)))
	result, err := service.ListUserRedemptionCodes(c.Request.Context(), service.UserIdentity{
		ID:   c.GetInt("id"),
		Role: c.GetInt("role"),
	}, service.UserRedemptionListQuery{
		OwnerOnly:  true,
		Deleted:    strings.TrimSpace(c.Query("deleted")),
		Status:     strings.TrimSpace(c.Query("status")),
		RewardType: strings.TrimSpace(c.Query("reward_type")),
		Page:       page,
		PageSize:   pageSize,
	})
	if err != nil {
		apiUserRedemptionError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

func GetMyUserRedemption(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		apiUserRedemptionError(c, service.ErrUserRedemptionInvalid)
		return
	}
	detail, err := service.GetUserRedemptionDetail(c.Request.Context(), service.UserIdentity{
		ID:   c.GetInt("id"),
		Role: c.GetInt("role"),
	}, id, true)
	if err != nil {
		apiUserRedemptionError(c, err)
		return
	}
	common.ApiSuccess(c, detail)
}

func CancelMyUserRedemption(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		apiUserRedemptionError(c, service.ErrUserRedemptionInvalid)
		return
	}
	code, err := service.CancelUserRedemptionCode(c.Request.Context(), service.UserIdentity{
		ID:   c.GetInt("id"),
		Role: c.GetInt("role"),
	}, id)
	if err != nil {
		apiUserRedemptionError(c, err)
		return
	}
	view, err := getUserRedemptionCodeViewForContext(c, code.ID)
	if err != nil {
		apiUserRedemptionError(c, err)
		return
	}
	common.ApiSuccess(c, view)
}

func CancelAdminUserRedemption(c *gin.Context) {
	mutateAdminUserRedemption(c, service.UserRedemptionActionTerminate)
}

func BatchSettleAdminUserRedemptions(c *gin.Context) {
	var request service.UserRedemptionBatchDispositionRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		apiUserRedemptionError(c, service.ErrUserRedemptionInvalid)
		return
	}
	results, err := service.AdminBatchSettleUserRedemptionCodes(c.Request.Context(), service.UserIdentity{
		ID:   c.GetInt("id"),
		Role: c.GetInt("role"),
	}, request)
	if err != nil {
		apiUserRedemptionError(c, err)
		return
	}
	common.ApiSuccess(c, results)
}

func ListAdminUserRedemptions(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", strconv.Itoa(common.ItemsPerPage)))
	creatorID, _ := strconv.Atoi(c.Query("creator_user_id"))
	createdFrom, _ := strconv.ParseInt(c.Query("created_from"), 10, 64)
	createdTo, _ := strconv.ParseInt(c.Query("created_to"), 10, 64)
	expiresFrom, _ := strconv.ParseInt(c.Query("expires_from"), 10, 64)
	expiresTo, _ := strconv.ParseInt(c.Query("expires_to"), 10, 64)
	result, err := service.ListUserRedemptionCodes(c.Request.Context(), service.UserIdentity{
		ID:   c.GetInt("id"),
		Role: c.GetInt("role"),
	}, service.UserRedemptionListQuery{
		CreatorUserID:   creatorID,
		CreatorUsername: strings.TrimSpace(c.Query("creator_username")),
		Status:          strings.TrimSpace(c.Query("status")),
		RewardType:      strings.TrimSpace(c.Query("reward_type")),
		Kind:            strings.TrimSpace(c.Query("kind")),
		Deleted:         strings.TrimSpace(c.Query("deleted")),
		CreatedFrom:     createdFrom,
		CreatedTo:       createdTo,
		ExpiresFrom:     expiresFrom,
		ExpiresTo:       expiresTo,
		Page:            page,
		PageSize:        pageSize,
	})
	if err != nil {
		apiUserRedemptionError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

func GetAdminUserRedemption(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		apiUserRedemptionError(c, service.ErrUserRedemptionInvalid)
		return
	}
	detail, err := service.GetUserRedemptionDetail(c.Request.Context(), service.UserIdentity{
		ID:   c.GetInt("id"),
		Role: c.GetInt("role"),
	}, id, true)
	if err != nil {
		apiUserRedemptionError(c, err)
		return
	}
	common.ApiSuccess(c, detail)
}

func DisableAdminUserRedemption(c *gin.Context) {
	mutateAdminUserRedemption(c, service.UserRedemptionActionDisable)
}

func EnableAdminUserRedemption(c *gin.Context) {
	mutateAdminUserRedemption(c, service.UserRedemptionActionEnable)
}

func TerminateAdminUserRedemption(c *gin.Context) {
	mutateAdminUserRedemption(c, service.UserRedemptionActionTerminate)
}

func DeleteAdminUserRedemption(c *gin.Context) {
	mutateAdminUserRedemption(c, service.UserRedemptionActionDelete)
}

func mutateAdminUserRedemption(c *gin.Context, action service.UserRedemptionAdminAction) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		apiUserRedemptionError(c, service.ErrUserRedemptionInvalid)
		return
	}
	var request adminUserRedemptionMutationRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiError(c, err)
		return
	}
	code, err := service.AdminMutateUserRedemptionCode(c.Request.Context(), service.UserRedemptionAdminCommand{
		Actor: service.UserIdentity{
			ID:   c.GetInt("id"),
			Role: c.GetInt("role"),
		},
		CodeID:     id,
		Action:     action,
		Settlement: request.Settlement,
		Reason:     request.Reason,
	})
	if err != nil {
		apiUserRedemptionError(c, err)
		return
	}
	view, err := getUserRedemptionCodeViewForContext(c, code.ID)
	if err != nil {
		apiUserRedemptionError(c, err)
		return
	}
	common.ApiSuccess(c, view)
}

func getUserRedemptionCodeViewForContext(c *gin.Context, codeID int64) (service.UserRedemptionCodeView, error) {
	detail, err := service.GetUserRedemptionDetail(c.Request.Context(), service.UserIdentity{
		ID:   c.GetInt("id"),
		Role: c.GetInt("role"),
	}, codeID, false)
	if err != nil {
		return service.UserRedemptionCodeView{}, err
	}
	return detail.Code, nil
}
