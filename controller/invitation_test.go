package controller

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	appi18n "github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestInvitationUserHandlersApplyReadAndWithdraw(t *testing.T) {
	setupRegistrationAffControllerTest(t)
	seedInvitationControllerUser(t, 9, "applicant", common.RoleCommonUser)

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/invitation/applications", map[string]any{
		"reason": "invite my project team",
	}, 9)
	CreateInvitationApplication(ctx)
	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	var application model.InviteCodeApplication
	require.NoError(t, common.Unmarshal(response.Data, &application))
	require.Equal(t, 9, application.UserID)

	ctx, recorder = newAuthenticatedContext(t, http.MethodGet, "/api/invitation/self", nil, 9)
	GetInvitationSelf(ctx)
	response = decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	var state service.InvitationUserState
	require.NoError(t, common.Unmarshal(response.Data, &state))
	require.NotNil(t, state.Application)
	require.Equal(t, application.ID, state.Application.ID)
	require.False(t, state.CanApply)

	ctx, recorder = newAuthenticatedContext(t, http.MethodPost, fmt.Sprintf("/api/invitation/applications/%d/withdraw", application.ID), nil, 9)
	ctx.Params = gin.Params{{Key: "id", Value: fmt.Sprint(application.ID)}}
	WithdrawInvitationApplication(ctx)
	response = decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)

	state, err := service.GetInvitationUserState(context.Background(), 9)
	require.NoError(t, err)
	require.Equal(t, model.InviteApplicationStatusWithdrawn, state.Application.Status)
	require.True(t, state.CanApply)
}

func TestGetInvitationSelfIncludesReasonAndPublicInvitationLink(t *testing.T) {
	setupRegistrationAffControllerTest(t)
	seedInvitationControllerUser(t, 9, "invite-owner", common.RoleCommonUser)
	ownerID := 9
	require.NoError(t, model.DB.Create(&model.InviteCode{
		OwnerUserID: ownerID, Code: "SAFE234567", ActiveOwnerSlot: &ownerID,
		Kind: model.InviteCodeKindMulti, MaxUses: 3, Status: model.InviteCodeStatusActive,
		ExpiresAt: time.Now().Add(24 * time.Hour).Unix(), IssuedBy: 1,
	}).Error)
	previousAddress := system_setting.ServerAddress
	system_setting.ServerAddress = "https://clove.example/base?source=console"
	t.Cleanup(func() { system_setting.ServerAddress = previousAddress })

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/invitation/self", nil, 9)
	GetInvitationSelf(ctx)
	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	var state service.InvitationUserState
	require.NoError(t, common.Unmarshal(response.Data, &state))
	require.False(t, state.CanApply)
	require.Equal(t, "active_code", state.ApplyUnavailableReason)
	require.NotNil(t, state.Code)
	require.Equal(t, "https://clove.example/base?aff=SAFE234567&source=console", state.Code.InvitationLink)
}

func TestValidateInvitationCodeReturnsPublicAvailabilityWithoutOwnerData(t *testing.T) {
	setupRegistrationAffControllerTest(t)
	seedInvitationControllerUser(t, 9, "invite-owner", common.RoleCommonUser)
	ownerID := 9
	require.NoError(t, model.DB.Create(&model.InviteCode{
		OwnerUserID: ownerID, Code: "ABCD2345", ActiveOwnerSlot: &ownerID,
		Kind: model.InviteCodeKindMulti, MaxUses: 3, Status: model.InviteCodeStatusActive,
		ExpiresAt: time.Now().Add(24 * time.Hour).Unix(), IssuedBy: 1,
	}).Error)

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/invitation/validate", map[string]any{
		"code": "abcd2345",
	}, 0)
	ctx.Request.Header.Set("Accept-Language", "zh-CN")
	ValidateInvitationCode(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	var availability struct {
		Valid   bool   `json:"valid"`
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	require.NoError(t, common.Unmarshal(response.Data, &availability))
	require.True(t, availability.Valid)
	require.Equal(t, "invitation_available", availability.Code)
	require.Equal(t, "该邀请码可用", availability.Message)
	require.NotContains(t, string(response.Data), "owner_user_id")
	require.NotContains(t, string(response.Data), "max_uses")

	ctx, recorder = newAuthenticatedContext(t, http.MethodPost, "/api/invitation/validate", map[string]any{
		"code": "NOTFOUND",
	}, 0)
	ctx.Request.Header.Set("Accept-Language", "zh-CN")
	ValidateInvitationCode(ctx)
	response = decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	require.NoError(t, common.Unmarshal(response.Data, &availability))
	require.False(t, availability.Valid)
	require.Equal(t, "invitation_unavailable", availability.Code)
	require.Equal(t, "邀请码不可用", availability.Message)
}

func TestInvitationAdminHandlersReviewListIssueAndRevoke(t *testing.T) {
	setupRegistrationAffControllerTest(t)
	seedInvitationControllerUser(t, 1, "primary-root", common.RoleRootUser)
	seedInvitationControllerUser(t, 9, "applicant", common.RoleCommonUser)
	seedInvitationControllerUser(t, 10, "direct-target", common.RoleCommonUser)
	application, err := service.ApplyForInviteCode(context.Background(), 9, "invite my project team")
	require.NoError(t, err)

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, fmt.Sprintf("/api/invitation/admin/applications/%d/review", application.ID), map[string]any{
		"approve": true, "kind": model.InviteCodeKindMulti, "max_uses": 5,
		"expires_at": time.Now().Add(48 * time.Hour).Unix(), "review_reason": "approved",
	}, 1)
	ctx.Params = gin.Params{{Key: "id", Value: fmt.Sprint(application.ID)}}
	ReviewInvitationApplication(ctx)
	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	require.True(t, common.GetContextKeyBool(ctx, constant.ContextKeyAuditLogged))
	var reviewedCode model.InviteCode
	require.NoError(t, common.Unmarshal(response.Data, &reviewedCode))
	require.Equal(t, 9, reviewedCode.OwnerUserID)

	ctx, recorder = newAuthenticatedContext(t, http.MethodGet, "/api/invitation/admin/applications?page=1&page_size=20&status=approved", nil, 1)
	ListInvitationApplications(ctx)
	response = decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	var applicationList service.InvitationApplicationListResult
	require.NoError(t, common.Unmarshal(response.Data, &applicationList))
	require.EqualValues(t, 1, applicationList.Total)
	require.Equal(t, "applicant", applicationList.Items[0].ApplicantUsername)

	ctx, recorder = newAuthenticatedContext(t, http.MethodPost, "/api/invitation/admin/codes", map[string]any{
		"target_user_id": 10, "reason": "support team invite", "kind": model.InviteCodeKindSingle,
		"max_uses": 1, "expires_at": time.Now().Add(24 * time.Hour).Unix(),
	}, 1)
	CreateInvitationCode(ctx)
	response = decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	require.True(t, common.GetContextKeyBool(ctx, constant.ContextKeyAuditLogged))
	var directCode model.InviteCode
	require.NoError(t, common.Unmarshal(response.Data, &directCode))
	require.Equal(t, 10, directCode.OwnerUserID)

	ctx, recorder = newAuthenticatedContext(t, http.MethodGet, "/api/invitation/admin/codes?keyword=direct-target", nil, 1)
	ListInvitationCodes(ctx)
	response = decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	var codeList service.InvitationCodeListResult
	require.NoError(t, common.Unmarshal(response.Data, &codeList))
	require.EqualValues(t, 1, codeList.Total)

	ctx, recorder = newAuthenticatedContext(t, http.MethodPost, fmt.Sprintf("/api/invitation/admin/codes/%d/revoke", directCode.ID), map[string]any{
		"reason": "rotated by owner request",
	}, 1)
	ctx.Params = gin.Params{{Key: "id", Value: fmt.Sprint(directCode.ID)}}
	RevokeInvitationCode(ctx)
	response = decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	require.True(t, common.GetContextKeyBool(ctx, constant.ContextKeyAuditLogged))
	var stored model.InviteCode
	require.NoError(t, model.DB.First(&stored, directCode.ID).Error)
	require.Equal(t, model.InviteCodeStatusRevoked, stored.Status)

	ctx, recorder = newAuthenticatedContext(t, http.MethodGet, "/api/invitation/admin/uses?page=1&page_size=20", nil, 1)
	ListInvitationUses(ctx)
	response = decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	var useList service.InvitationUseListResult
	require.NoError(t, common.Unmarshal(response.Data, &useList))
	require.Zero(t, useList.Total)
}

func TestInvitationRewardPayoutRequestHandlersSubmitListAndApprove(t *testing.T) {
	setupRegistrationAffControllerTest(t)
	seedInvitationControllerUser(t, 1, "primary-root", common.RoleRootUser)
	seedInvitationControllerUser(t, 9, "inviter", common.RoleCommonUser)
	seedInvitationControllerUser(t, 10, "invitee", common.RoleCommonUser)
	ownerID := 9
	invite := model.InviteCode{
		OwnerUserID: ownerID, Code: "reward-handler-code", ActiveOwnerSlot: &ownerID,
		Kind: model.InviteCodeKindSingle, MaxUses: 1, Status: model.InviteCodeStatusExhausted,
		UsedCount: 1, ExpiresAt: time.Now().Add(time.Hour).Unix(), IssuedBy: 1,
	}
	require.NoError(t, model.DB.Create(&invite).Error)
	use := model.InviteCodeUse{
		InviteCodeID: invite.ID, InviterUserID: 9, RegisteredUserID: 10, RegisteredAt: time.Now().Unix(),
		InviterReward: 10, InviteeReward: 0, InviterRewardStatus: model.InviteRewardStatusAvailable,
		InviteeRewardStatus: model.InviteRewardStatusSkipped, AuditEventID: "invitation-reward-handler-use",
	}
	require.NoError(t, model.DB.Create(&use).Error)
	payout := model.InviteRewardPayout{
		InviteCodeUseID: use.ID, RecipientKind: model.InviteRewardRecipientInviter, RecipientUserID: 9,
		Amount: 10, Status: model.InviteRewardStatusAvailable,
	}
	require.NoError(t, model.DB.Create(&payout).Error)

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/invitation/rewards", nil, 9)
	GetInvitationRewardSummary(ctx)
	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	var summary service.InvitationRewardSummary
	require.NoError(t, common.Unmarshal(response.Data, &summary))
	require.EqualValues(t, 1, summary.InvitedCount)
	require.EqualValues(t, 10, summary.TotalEarned)
	require.EqualValues(t, 10, summary.Available)

	paymentSetting := operation_setting.GetPaymentSetting()
	previousPaymentSetting := *paymentSetting
	paymentSetting.ComplianceConfirmed = true
	paymentSetting.ComplianceTermsVersion = operation_setting.CurrentComplianceTermsVersion
	t.Cleanup(func() { *paymentSetting = previousPaymentSetting })
	ctx, recorder = newAuthenticatedContext(t, http.MethodPost, "/api/invitation/rewards/payout-requests", nil, 9)
	CreateInvitationRewardPayoutRequest(ctx)
	response = decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	require.True(t, common.GetContextKeyBool(ctx, constant.ContextKeyAuditLogged))
	var request model.InviteRewardPayoutRequest
	require.NoError(t, common.Unmarshal(response.Data, &request))
	require.EqualValues(t, 10, request.Amount)

	ctx, recorder = newAuthenticatedContext(t, http.MethodGet, "/api/invitation/admin/reward-payout-requests?page=1&page_size=20&status=pending", nil, 1)
	ListInvitationRewardPayoutRequests(ctx)
	response = decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	var list service.InvitationRewardPayoutRequestListResult
	require.NoError(t, common.Unmarshal(response.Data, &list))
	require.EqualValues(t, 1, list.Total)
	require.Equal(t, "inviter", list.Items[0].Username)
	require.Equal(t, 1, list.Items[0].ItemCount)

	ctx, recorder = newAuthenticatedContext(t, http.MethodPost, fmt.Sprintf("/api/invitation/admin/reward-payout-requests/%d/review", request.ID), map[string]any{
		"approve": true,
	}, 1)
	ctx.Params = gin.Params{{Key: "id", Value: fmt.Sprint(request.ID)}}
	ReviewInvitationRewardPayoutRequest(ctx)
	response = decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	require.True(t, common.GetContextKeyBool(ctx, constant.ContextKeyAuditLogged))

	ctx, recorder = newAuthenticatedContext(t, http.MethodGet, "/api/invitation/rewards", nil, 9)
	GetInvitationRewardSummary(ctx)
	response = decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	require.NoError(t, common.Unmarshal(response.Data, &summary))
	require.Zero(t, summary.Available)
	require.Zero(t, summary.PendingPayout)
	require.EqualValues(t, 10, summary.Paid)
	var inviter model.User
	require.NoError(t, model.DB.First(&inviter, 9).Error)
	require.EqualValues(t, 10, inviter.Quota)

	ctx, recorder = newAuthenticatedContext(t, http.MethodGet, fmt.Sprintf("/api/invitation/admin/reward-payout-requests/%d", request.ID), nil, 1)
	ctx.Params = gin.Params{{Key: "id", Value: fmt.Sprint(request.ID)}}
	GetInvitationRewardPayoutRequestDetail(ctx)
	response = decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	var detail service.InvitationRewardPayoutRequestDetail
	require.NoError(t, common.Unmarshal(response.Data, &detail))
	require.Equal(t, request.ID, detail.Request.ID)
	require.Equal(t, request.OperationID, detail.Request.OperationID)
	require.Equal(t, "primary-root", detail.Request.ReviewerUsername)
	require.Len(t, detail.Rewards, 1)
	require.Equal(t, "invitee", detail.Rewards[0].SourceUsername)
	require.Empty(t, detail.Rewards[0].RegistrationIP)
	require.Empty(t, detail.Actions)

	ctx, recorder = newAuthenticatedContext(t, http.MethodPost, fmt.Sprintf("/api/invitation/admin/reward-payout-requests/%d/freeze", request.ID), map[string]any{
		"reason": "confirmed fraudulent invitation",
	}, 1)
	ctx.Params = gin.Params{{Key: "id", Value: fmt.Sprint(request.ID)}}
	FreezeInvitationRewardPayoutRequest(ctx)
	response = decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	require.True(t, common.GetContextKeyBool(ctx, constant.ContextKeyAuditLogged))

	ctx, recorder = newAuthenticatedContext(t, http.MethodGet, fmt.Sprintf("/api/invitation/admin/reward-payout-requests/%d", request.ID), nil, 1)
	ctx.Params = gin.Params{{Key: "id", Value: fmt.Sprint(request.ID)}}
	GetInvitationRewardPayoutRequestDetail(ctx)
	response = decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	require.NoError(t, common.Unmarshal(response.Data, &detail))
	require.Equal(t, model.InviteRewardPayoutRequestStatusFrozen, detail.Request.Status)
	require.Len(t, detail.Actions, 1)
	require.Equal(t, model.InviteRewardPayoutActionFreeze, detail.Actions[0].Action)
	require.Equal(t, "primary-root", detail.Actions[0].ActorUsername)

	ctx, recorder = newAuthenticatedContext(t, http.MethodPost, fmt.Sprintf("/api/invitation/admin/reward-payout-requests/%d/unfreeze", request.ID), map[string]any{
		"reason": "manual review cleared the account",
	}, 1)
	ctx.Params = gin.Params{{Key: "id", Value: fmt.Sprint(request.ID)}}
	UnfreezeInvitationRewardPayoutRequest(ctx)
	response = decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	require.True(t, common.GetContextKeyBool(ctx, constant.ContextKeyAuditLogged))
}

func TestInvitationRewardSummaryPrivacyOmitsSourceIdentifiers(t *testing.T) {
	setupRegistrationAffControllerTest(t)
	seedInvitationControllerUser(t, 9, "privacy-inviter", common.RoleCommonUser)
	seedInvitationControllerUser(t, 10, "private-source-user", common.RoleCommonUser)
	use := model.InviteCodeUse{
		InviteCodeID: 1, InviterUserID: 9, RegisteredUserID: 10,
		Source: "password", AuditEventID: "privacy-use",
	}
	require.NoError(t, model.DB.Create(&use).Error)
	require.NoError(t, model.DB.Create(&model.InviteRewardPayout{
		InviteCodeUseID: use.ID, TriggerType: model.InviteRewardTriggerCashTopUp,
		TriggerID: "private-provider-order-number", RecipientKind: model.InviteRewardRecipientInviter,
		RecipientUserID: 9, EligibleQuota: 1_000, RateBasisPoints: 1_000,
		Amount: 100, Status: model.InviteRewardStatusAvailable,
	}).Error)
	setting := operation_setting.GetInvitationIncentiveSetting()
	setting.Enabled = true
	setting.CashCommissionEnabled = true
	setting.CashCommissionRate = "10.00"

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/invitation/rewards", nil, 9)
	GetInvitationRewardSummary(ctx)
	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	var summary service.InvitationRewardSummary
	require.NoError(t, common.Unmarshal(response.Data, &summary))
	require.True(t, summary.CashCommissionVisible)
	require.Equal(t, "10.00", summary.CashCommissionRate)
	require.NotContains(t, recorder.Body.String(), "private-provider-order-number")
	require.NotContains(t, recorder.Body.String(), "private-source-user")
	require.NotContains(t, recorder.Body.String(), "provider_payload")
}

func TestInvitationOwnerCanRevokeOwnCodeButNotAnotherUsersCode(t *testing.T) {
	setupRegistrationAffControllerTest(t)
	seedInvitationControllerUser(t, 1, "primary-root", common.RoleRootUser)
	seedInvitationControllerUser(t, 9, "owner", common.RoleCommonUser)
	seedInvitationControllerUser(t, 10, "other", common.RoleCommonUser)

	ownerID := 9
	require.NoError(t, model.DB.Create(&model.InviteCode{
		OwnerUserID: 9, Code: "owner-revoke-code", Kind: model.InviteCodeKindSingle,
		MaxUses: 1, Status: model.InviteCodeStatusActive, ActiveOwnerSlot: &ownerID,
		ExpiresAt: time.Now().Add(time.Hour).Unix(), IssuedBy: 1,
	}).Error)

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/invitation/codes/1/revoke", map[string]any{
		"reason": "user no longer needs this code",
	}, 9)
	ctx.Params = gin.Params{{Key: "id", Value: "1"}}
	RevokeInvitationCode(ctx)
	require.True(t, decodeAPIResponse(t, recorder).Success)

	var stored model.InviteCode
	require.NoError(t, model.DB.Where("code = ?", "owner-revoke-code").First(&stored).Error)
	require.Equal(t, model.InviteCodeStatusRevoked, stored.Status)

	otherID := 10
	require.NoError(t, model.DB.Create(&model.InviteCode{
		OwnerUserID: 10, Code: "other-revoke-code", Kind: model.InviteCodeKindSingle,
		MaxUses: 1, Status: model.InviteCodeStatusActive, ActiveOwnerSlot: &otherID,
		ExpiresAt: time.Now().Add(time.Hour).Unix(), IssuedBy: 1,
	}).Error)

	ctx, recorder = newAuthenticatedContext(t, http.MethodPost, "/api/invitation/codes/2/revoke", map[string]any{
		"reason": "attempt to revoke another user's code",
	}, 9)
	ctx.Params = gin.Params{{Key: "id", Value: "2"}}
	RevokeInvitationCode(ctx)
	require.False(t, decodeAPIResponse(t, recorder).Success)
}

func TestInvitationAuditActionsHaveFriendlyTemplates(t *testing.T) {
	actions := []string{
		"invitation.application_created",
		"invitation.application_withdrawn",
		"invitation.application_reviewed",
		"invitation.code_issued",
		"invitation.code_revoked",
		"invitation.code_used",
		"invitation.reward_payout_submitted",
		"invitation.reward_payout_reviewed",
		"invitation.reward_payout_frozen",
		"invitation.reward_payout_unfrozen",
	}
	params := map[string]interface{}{
		"application_id": 1, "target_user_id": 2, "status": "paid", "invite_code_id": 3,
		"request_id": 4, "amount": 5, "item_count": 2, "operation_id": "invite-reward-payout:test",
	}
	for _, action := range actions {
		rendered := auditContentEN(action, params)
		require.NotEqual(t, action, rendered)
		require.NotContains(t, rendered, "${")
	}
}

func TestInvitationHandlersRejectMalformedInput(t *testing.T) {
	setupRegistrationAffControllerTest(t)
	seedInvitationControllerUser(t, 1, "primary-root", common.RoleRootUser)

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/invitation/applications/not-an-id/withdraw", nil, 1)
	ctx.Params = gin.Params{{Key: "id", Value: "not-an-id"}}
	WithdrawInvitationApplication(ctx)
	require.False(t, decodeAPIResponse(t, recorder).Success)

	ctx, recorder = newAuthenticatedContext(t, http.MethodPost, "/api/invitation/admin/codes", map[string]any{
		"target_user_id": 0,
	}, 1)
	CreateInvitationCode(ctx)
	require.False(t, decodeAPIResponse(t, recorder).Success)
}

func TestCreateInvitationCodeLocalizesForbiddenTarget(t *testing.T) {
	setupRegistrationAffControllerTest(t)
	seedInvitationControllerUser(t, 1, "primary-root", common.RoleRootUser)

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/invitation/admin/codes", map[string]any{
		"target_user_id": 999, "reason": "production verification", "kind": model.InviteCodeKindSingle,
		"max_uses": 1, "expires_at": time.Now().Add(24 * time.Hour).Unix(),
	}, 1)
	ctx.Request.Header.Set("Accept-Language", "zh-CN")
	CreateInvitationCode(ctx)

	response := decodeAPIResponse(t, recorder)
	require.False(t, response.Success)
	require.Equal(t, "目标用户不存在，或您无权向该用户发放邀请码。", response.Message)
	require.NotContains(t, response.Message, "invite_target_forbidden")
}

func TestInvitationErrorsAreLocalized(t *testing.T) {
	require.NoError(t, appi18n.Init())
	tests := []struct {
		name    string
		err     error
		code    string
		message string
	}{
		{name: "pending application", err: service.ErrInviteApplicationPending, code: "invitation_application_pending", message: "您已有待审核的邀请码申请。"},
		{name: "reapply cooldown", err: &service.InviteReapplyError{RetryAt: time.Now().Add(time.Hour).Unix()}, code: "invitation_reapply_later", message: "当前仍在邀请码申请冷却期，请在页面显示的时间后重试。"},
		{name: "applications disabled", err: service.ErrInviteApplicationsDisabled, code: "invitation_applications_disabled", message: "邀请码申请当前已关闭。"},
		{name: "active code", err: service.ErrInviteActiveCode, code: "invitation_active_code", message: "该用户已有生效中的邀请码。"},
		{name: "forbidden target", err: service.ErrInviteTargetForbidden, code: "invitation_target_forbidden", message: "目标用户不存在，或您无权向该用户发放邀请码。"},
		{name: "invalid application", err: service.ErrInviteApplicationInvalid, code: "invitation_application_invalid", message: "邀请码申请无效，或已不再等待审核。"},
		{name: "invalid issue", err: service.ErrInviteIssueInvalid, code: "invitation_issue_invalid", message: "请检查目标用户 ID、邀请码类型、使用次数、过期时间和发放原因。"},
		{name: "invalid code", err: service.ErrInviteCodeInvalid, code: "invitation_code_invalid", message: "邀请码无效，或已不再生效。"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/invitation/test", nil, 1)
			ctx.Request.Header.Set("Accept-Language", "zh-CN")
			ctx.Set(common.RequestIdKey, "invitation-request")
			invitationAPIError(ctx, tt.err)

			response := decodeAPIResponse(t, recorder)
			require.False(t, response.Success)
			require.Equal(t, tt.message, response.Message)
			require.NotContains(t, response.Message, "invite_")
			var details struct {
				Code      string `json:"code"`
				RequestID string `json:"request_id"`
			}
			require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &details))
			require.Equal(t, tt.code, details.Code)
			require.Equal(t, "invitation-request", details.RequestID)
		})
	}
}

func seedInvitationControllerUser(t *testing.T, id int, username string, role int) {
	t.Helper()
	require.NoError(t, model.DB.Create(&model.User{
		Id: id, Username: username, Password: "password", DisplayName: username,
		Role: role, Status: common.UserStatusEnabled, Group: "default", AffCode: fmt.Sprintf("legacy-%d", id),
	}).Error)
}
