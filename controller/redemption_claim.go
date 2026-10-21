package controller

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
)

type redemptionClaimRequest struct {
	Code string `json:"code"`
}

func apiRedemptionClaimError(c *gin.Context, err error) {
	var rewardTypeMismatch *service.UserRedemptionRewardTypeMismatchError
	switch {
	case errors.As(err, &rewardTypeMismatch),
		errors.Is(err, service.ErrUserRedemptionPlanDisabled),
		errors.Is(err, service.ErrUserRedemptionBalancePaymentDisabled),
		errors.Is(err, service.ErrUserRedemptionPlanPurchaseLimit):
		apiUserRedemptionError(c, err)
	default:
		apiWalletRedemptionError(c, err)
	}
}

func ClaimRedemption(c *gin.Context) {
	if !operation_setting.IsPaymentComplianceConfirmed() {
		common.ApiErrorI18n(c, i18n.MsgPaymentComplianceRequired)
		return
	}
	var request redemptionClaimRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiError(c, err)
		return
	}
	request.Code = strings.TrimSpace(request.Code)
	result, err := service.RedeemCode(c.Request.Context(), c.GetInt("id"), request.Code)
	if err != nil {
		apiRedemptionClaimError(c, err)
		return
	}
	recordUserSecurityAudit(c, c.GetInt("id"), "user.redeem_code", map[string]interface{}{
		"quota":           logger.LogQuota64(result.Quota),
		"reward_type":     result.RewardType,
		"code_type":       result.Source,
		"subscription_id": result.SubscriptionID,
	})
	common.ApiSuccess(c, result)
}
