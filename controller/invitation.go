package controller

import (
	"errors"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

type invitationApplicationRequest struct {
	Reason string `json:"reason"`
}

type invitationReviewRequest struct {
	Approve      bool   `json:"approve"`
	Kind         string `json:"kind"`
	MaxUses      int    `json:"max_uses"`
	ExpiresAt    int64  `json:"expires_at"`
	ReviewReason string `json:"review_reason"`
}

type invitationDirectIssueRequest struct {
	TargetUserID int    `json:"target_user_id"`
	Reason       string `json:"reason"`
	Kind         string `json:"kind"`
	MaxUses      int    `json:"max_uses"`
	ExpiresAt    int64  `json:"expires_at"`
}

type invitationRevokeRequest struct {
	Reason string `json:"reason"`
}

type invitationRewardReviewRequest struct {
	Approve      bool   `json:"approve"`
	ReviewReason string `json:"review_reason"`
}

type invitationRewardMutationRequest struct {
	Reason string `json:"reason"`
}

type invitationValidationRequest struct {
	Code string `json:"code"`
}

type invitationValidationResponse struct {
	Valid   bool   `json:"valid"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

func invitationAPIError(c *gin.Context, err error) {
	key := ""
	code := "invitation_operation_failed"
	switch {
	case errors.Is(err, service.ErrInviteApplicationPending):
		key = i18n.MsgInvitationApplicationPending
		code = "invitation_application_pending"
	case errors.Is(err, service.ErrInviteReapplyLater):
		key = i18n.MsgInvitationReapplyLater
		code = "invitation_reapply_later"
	case errors.Is(err, service.ErrInviteApplicationsDisabled):
		key = i18n.MsgInvitationApplicationsDisabled
		code = "invitation_applications_disabled"
	case errors.Is(err, service.ErrInviteActiveCode):
		key = i18n.MsgInvitationActiveCode
		code = "invitation_active_code"
	case errors.Is(err, service.ErrInviteTargetForbidden):
		key = i18n.MsgInvitationTargetForbidden
		code = "invitation_target_forbidden"
	case errors.Is(err, service.ErrInviteApplicationInvalid):
		key = i18n.MsgInvitationApplicationInvalid
		code = "invitation_application_invalid"
	case errors.Is(err, service.ErrInviteIssueInvalid):
		key = i18n.MsgInvitationIssueInvalid
		code = "invitation_issue_invalid"
	case errors.Is(err, service.ErrInviteCodeInvalid):
		key = i18n.MsgInvitationCodeInvalid
		code = "invitation_code_invalid"
	case errors.Is(err, service.ErrInviteRewardUnavailable):
		key = i18n.MsgInvitationRewardUnavailable
		code = "invitation_reward_unavailable"
	case errors.Is(err, service.ErrInviteRewardReviewInvalid):
		key = i18n.MsgInvitationRewardReviewInvalid
		code = "invitation_reward_review_invalid"
	case errors.Is(err, service.ErrInviteRewardMutationInvalid):
		key = i18n.MsgInvitationRewardStateInvalid
		code = "invitation_reward_state_invalid"
	default:
		common.SysError("invitation workflow failed: " + err.Error())
		key = i18n.MsgOperationFailed
	}
	common.ApiErrorCodeI18n(c, code, key)
}

func ValidateInvitationCode(c *gin.Context) {
	var request invitationValidationRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidInput)
		return
	}
	validation, err := service.ValidateInvitationCodePublic(c.Request.Context(), request.Code)
	if err != nil {
		invitationAPIError(c, err)
		return
	}
	messageKey := i18n.MsgInvitationCodeUnavailable
	if validation.Valid {
		messageKey = i18n.MsgInvitationCodeAvailable
	}
	common.ApiSuccess(c, invitationValidationResponse{
		Valid: validation.Valid, Code: validation.Code,
		Message: common.TranslateMessage(c, messageKey),
	})
}

func GetInvitationSelf(c *gin.Context) {
	state, err := service.GetInvitationUserState(c.Request.Context(), c.GetInt("id"))
	if err != nil {
		invitationAPIError(c, err)
		return
	}
	common.ApiSuccess(c, state)
}

func GetInvitationRewardSummary(c *gin.Context) {
	summary, err := service.GetInvitationRewardSummary(c.Request.Context(), c.GetInt("id"))
	if err != nil {
		invitationAPIError(c, err)
		return
	}
	common.ApiSuccess(c, summary)
}

func CreateInvitationRewardPayoutRequest(c *gin.Context) {
	if !requirePaymentCompliance(c) {
		return
	}
	request, err := service.SubmitInvitationRewardPayout(c.Request.Context(), c.GetInt("id"))
	if err != nil {
		invitationAPIError(c, err)
		return
	}
	markAuditLogged(c)
	common.ApiSuccess(c, request)
}

func CreateInvitationApplication(c *gin.Context) {
	var request invitationApplicationRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiError(c, err)
		return
	}
	application, err := service.ApplyForInviteCode(c.Request.Context(), c.GetInt("id"), request.Reason)
	if err != nil {
		invitationAPIError(c, err)
		return
	}
	common.ApiSuccess(c, application)
}

func WithdrawInvitationApplication(c *gin.Context) {
	applicationID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || applicationID <= 0 {
		invitationAPIError(c, service.ErrInviteApplicationInvalid)
		return
	}
	if err := service.WithdrawInviteCodeApplication(c.Request.Context(), c.GetInt("id"), applicationID); err != nil {
		invitationAPIError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func ListInvitationApplications(c *gin.Context) {
	result, err := service.ListInvitationApplications(c.Request.Context(), invitationListQuery(c))
	if err != nil {
		invitationAPIError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

func ReviewInvitationApplication(c *gin.Context) {
	applicationID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || applicationID <= 0 {
		invitationAPIError(c, service.ErrInviteApplicationInvalid)
		return
	}
	var request invitationReviewRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiError(c, err)
		return
	}
	code, err := service.ReviewInviteCodeApplication(c.Request.Context(), service.InviteCodeReviewRequest{
		ReviewerID:    c.GetInt("id"),
		ApplicationID: applicationID,
		Approve:       request.Approve,
		Issue: service.InviteCodeIssueSpec{
			Kind: request.Kind, MaxUses: request.MaxUses, ExpiresAt: request.ExpiresAt,
		},
		ReviewReason: request.ReviewReason,
	})
	if err != nil {
		invitationAPIError(c, err)
		return
	}
	markAuditLogged(c)
	common.ApiSuccess(c, code)
}

func ListInvitationCodes(c *gin.Context) {
	result, err := service.ListInvitationCodes(c.Request.Context(), invitationListQuery(c))
	if err != nil {
		invitationAPIError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

func CreateInvitationCode(c *gin.Context) {
	var request invitationDirectIssueRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiError(c, err)
		return
	}
	code, err := service.DirectIssueInviteCode(c.Request.Context(), service.InviteCodeDirectIssueRequest{
		IssuerID:     c.GetInt("id"),
		TargetUserID: request.TargetUserID,
		Reason:       request.Reason,
		Issue: service.InviteCodeIssueSpec{
			Kind: request.Kind, MaxUses: request.MaxUses, ExpiresAt: request.ExpiresAt,
		},
	})
	if err != nil {
		invitationAPIError(c, err)
		return
	}
	markAuditLogged(c)
	common.ApiSuccess(c, code)
}

func RevokeInvitationCode(c *gin.Context) {
	codeID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || codeID <= 0 {
		invitationAPIError(c, service.ErrInviteCodeInvalid)
		return
	}
	var request invitationRevokeRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := service.RevokeInviteCode(c.Request.Context(), c.GetInt("id"), codeID, request.Reason); err != nil {
		invitationAPIError(c, err)
		return
	}
	markAuditLogged(c)
	common.ApiSuccess(c, nil)
}

func ListInvitationUses(c *gin.Context) {
	result, err := service.ListInvitationUses(c.Request.Context(), invitationListQuery(c))
	if err != nil {
		invitationAPIError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

func ListInvitationRewardPayoutRequests(c *gin.Context) {
	result, err := service.ListInvitationRewardPayoutRequests(c.Request.Context(), invitationListQuery(c))
	if err != nil {
		invitationAPIError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

func ReviewInvitationRewardPayoutRequest(c *gin.Context) {
	requestID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || requestID <= 0 {
		invitationAPIError(c, service.ErrInviteRewardReviewInvalid)
		return
	}
	var request invitationRewardReviewRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidInput)
		return
	}
	if err := service.ReviewInvitationRewardPayoutRequest(c.Request.Context(), service.ReviewInvitationRewardPayoutRequestInput{
		ReviewerID: c.GetInt("id"), RequestID: requestID, Approve: request.Approve, ReviewReason: request.ReviewReason,
	}); err != nil {
		invitationAPIError(c, err)
		return
	}
	markAuditLogged(c)
	common.ApiSuccess(c, nil)
}

func GetInvitationRewardPayoutRequestDetail(c *gin.Context) {
	requestID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || requestID <= 0 {
		invitationAPIError(c, service.ErrInviteRewardMutationInvalid)
		return
	}
	detail, err := service.GetInvitationRewardPayoutRequestDetail(c.Request.Context(), c.GetInt("id"), requestID)
	if err != nil {
		invitationAPIError(c, err)
		return
	}
	common.ApiSuccess(c, detail)
}

func FreezeInvitationRewardPayoutRequest(c *gin.Context) {
	mutateInvitationRewardPayoutRequest(c, true)
}

func UnfreezeInvitationRewardPayoutRequest(c *gin.Context) {
	mutateInvitationRewardPayoutRequest(c, false)
}

func mutateInvitationRewardPayoutRequest(c *gin.Context, freeze bool) {
	requestID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || requestID <= 0 {
		invitationAPIError(c, service.ErrInviteRewardMutationInvalid)
		return
	}
	var request invitationRewardMutationRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidInput)
		return
	}
	input := service.InvitationRewardPayoutMutationInput{
		RequestID: requestID, ActorID: c.GetInt("id"), Reason: request.Reason,
	}
	if freeze {
		err = service.FreezeInvitationRewardPayoutRequest(c.Request.Context(), input)
	} else {
		err = service.UnfreezeInvitationRewardPayoutRequest(c.Request.Context(), input)
	}
	if err != nil {
		invitationAPIError(c, err)
		return
	}
	markAuditLogged(c)
	common.ApiSuccess(c, nil)
}

func invitationListQuery(c *gin.Context) service.InvitationAdminListQuery {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	return service.InvitationAdminListQuery{
		ActorID: c.GetInt("id"), Page: page, PageSize: pageSize,
		Status: c.Query("status"), Keyword: c.Query("keyword"),
	}
}
