package service

import (
	"context"
	"math"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestInvitationApplicationListsEnforceActorScopeAndPagination(t *testing.T) {
	db := setupInvitationServiceTest(t)
	seedInvitationUser(t, db, 1, "primary-root", common.RoleRootUser)
	seedInvitationUser(t, db, 2, "admin", common.RoleAdminUser)
	seedInvitationUser(t, db, 3, "other-admin", common.RoleAdminUser)
	seedInvitationUser(t, db, 4, "other-root", common.RoleRootUser)
	seedInvitationUser(t, db, 10, "alice", common.RoleCommonUser)
	seedInvitationUser(t, db, 11, "bob", common.RoleCommonUser)

	applications := []model.InviteCodeApplication{
		{UserID: 10, Reason: "alice request", Status: model.InviteApplicationStatusWithdrawn, CreatedAt: 10},
		{UserID: 11, Reason: "bob request", Status: model.InviteApplicationStatusRejected, CreatedAt: 20},
		{UserID: 3, Reason: "admin request", Status: model.InviteApplicationStatusRejected, CreatedAt: 30},
		{UserID: 4, Reason: "root request", Status: model.InviteApplicationStatusRejected, CreatedAt: 40},
	}
	require.NoError(t, db.Create(&applications).Error)

	adminPage, err := ListInvitationApplications(context.Background(), InvitationAdminListQuery{
		ActorID: 2, Page: 1, PageSize: 1,
	})
	require.NoError(t, err)
	require.EqualValues(t, 2, adminPage.Total)
	require.Equal(t, 1, adminPage.Page)
	require.Equal(t, 1, adminPage.PageSize)
	require.Len(t, adminPage.Items, 1)
	require.Equal(t, "bob", adminPage.Items[0].ApplicantUsername)

	rootPage, err := ListInvitationApplications(context.Background(), InvitationAdminListQuery{
		ActorID: 4, Page: 1, PageSize: 20,
	})
	require.NoError(t, err)
	require.EqualValues(t, 3, rootPage.Total)
	require.ElementsMatch(t, []string{"alice", "bob", "other-admin"}, applicationUsernames(rootPage.Items))

	primaryPage, err := ListInvitationApplications(context.Background(), InvitationAdminListQuery{
		ActorID: 1, Page: 1, PageSize: 20,
	})
	require.NoError(t, err)
	require.EqualValues(t, 4, primaryPage.Total)

	filtered, err := ListInvitationApplications(context.Background(), InvitationAdminListQuery{
		ActorID: 1, Page: 1, PageSize: 20, Status: model.InviteApplicationStatusWithdrawn, Keyword: "ali",
	})
	require.NoError(t, err)
	require.EqualValues(t, 1, filtered.Total)
	require.Len(t, filtered.Items, 1)
	require.Equal(t, "alice", filtered.Items[0].ApplicantUsername)
}

func TestInvitationCodeAndUseListsJoinNamesAndProtectIP(t *testing.T) {
	db := setupInvitationServiceTest(t)
	seedInvitationUser(t, db, 1, "primary-root", common.RoleRootUser)
	seedInvitationUser(t, db, 2, "admin", common.RoleAdminUser)
	seedInvitationUser(t, db, 3, "other-admin", common.RoleAdminUser)
	seedInvitationUser(t, db, 4, "other-root", common.RoleRootUser)
	seedInvitationUser(t, db, 10, "alice", common.RoleCommonUser)
	seedInvitationUser(t, db, 20, "joiner-a", common.RoleCommonUser)
	seedInvitationUser(t, db, 21, "joiner-b", common.RoleCommonUser)
	seedInvitationUser(t, db, 22, "joiner-c", common.RoleCommonUser)

	codes := []model.InviteCode{
		{OwnerUserID: 10, Code: "alice-code", Kind: model.InviteCodeKindMulti, MaxUses: 5, UsedCount: 1, Status: model.InviteCodeStatusActive, ExpiresAt: 10_000, IssuedBy: 1, CreatedAt: 10},
		{OwnerUserID: 3, Code: "admin-code", Kind: model.InviteCodeKindMulti, MaxUses: 5, UsedCount: 1, Status: model.InviteCodeStatusRevoked, ExpiresAt: 10_000, IssuedBy: 1, CreatedAt: 20},
		{OwnerUserID: 4, Code: "root-code", Kind: model.InviteCodeKindMulti, MaxUses: 5, UsedCount: 1, Status: model.InviteCodeStatusRevoked, ExpiresAt: 10_000, IssuedBy: 1, CreatedAt: 30},
	}
	require.NoError(t, db.Create(&codes).Error)
	uses := []model.InviteCodeUse{
		{InviteCodeID: codes[0].ID, InviterUserID: 10, RegisteredUserID: 20, RegisteredAt: 100, Source: "password", RegistrationIP: "203.0.113.45", InviterRewardStatus: model.InviteRewardStatusCompleted, InviteeRewardStatus: model.InviteRewardStatusCompleted, AuditEventID: "use-a"},
		{InviteCodeID: codes[1].ID, InviterUserID: 3, RegisteredUserID: 21, RegisteredAt: 200, Source: "oauth", RegistrationIP: "2001:db8::1", InviterRewardStatus: model.InviteRewardStatusCompleted, InviteeRewardStatus: model.InviteRewardStatusCompleted, AuditEventID: "use-b"},
		{InviteCodeID: codes[2].ID, InviterUserID: 4, RegisteredUserID: 22, RegisteredAt: 300, Source: "wechat", RegistrationIP: "198.51.100.8", InviterRewardStatus: model.InviteRewardStatusCompleted, InviteeRewardStatus: model.InviteRewardStatusCompleted, AuditEventID: "use-c"},
	}
	require.NoError(t, db.Create(&uses).Error)

	adminCodes, err := ListInvitationCodes(context.Background(), InvitationAdminListQuery{
		ActorID: 2, Page: 1, PageSize: 20,
	})
	require.NoError(t, err)
	require.EqualValues(t, 1, adminCodes.Total)
	require.Equal(t, "alice", adminCodes.Items[0].OwnerUsername)
	require.Equal(t, "primary-root", adminCodes.Items[0].IssuerUsername)

	rootCodes, err := ListInvitationCodes(context.Background(), InvitationAdminListQuery{
		ActorID: 4, Page: 1, PageSize: 20,
	})
	require.NoError(t, err)
	require.EqualValues(t, 2, rootCodes.Total)

	primaryCodes, err := ListInvitationCodes(context.Background(), InvitationAdminListQuery{
		ActorID: 1, Page: 1, PageSize: 20, Status: model.InviteCodeStatusRevoked, Keyword: "root-code",
	})
	require.NoError(t, err)
	require.EqualValues(t, 1, primaryCodes.Total)
	require.Equal(t, "other-root", primaryCodes.Items[0].OwnerUsername)

	adminUses, err := ListInvitationUses(context.Background(), InvitationAdminListQuery{
		ActorID: 2, Page: 1, PageSize: 20,
	})
	require.NoError(t, err)
	require.EqualValues(t, 1, adminUses.Total)
	require.Equal(t, "alice-code", adminUses.Items[0].InviteCode)
	require.Equal(t, "alice", adminUses.Items[0].InviterUsername)
	require.Equal(t, "joiner-a", adminUses.Items[0].RegisteredUsername)
	require.Equal(t, model.MaskAuditIP("203.0.113.45"), adminUses.Items[0].RegistrationIP)

	rootUses, err := ListInvitationUses(context.Background(), InvitationAdminListQuery{
		ActorID: 4, Page: 1, PageSize: 20, Keyword: "joiner-b",
	})
	require.NoError(t, err)
	require.EqualValues(t, 1, rootUses.Total)
	require.Equal(t, "2001:db8::1", rootUses.Items[0].RegistrationIP)

	primaryUses, err := ListInvitationUses(context.Background(), InvitationAdminListQuery{
		ActorID: 1, Page: 1, PageSize: 20,
	})
	require.NoError(t, err)
	require.EqualValues(t, 3, primaryUses.Total)
}

func TestInvitationAdminListsRejectOrdinaryUsers(t *testing.T) {
	db := setupInvitationServiceTest(t)
	seedInvitationUser(t, db, 9, "ordinary", common.RoleCommonUser)

	_, err := ListInvitationApplications(context.Background(), InvitationAdminListQuery{ActorID: 9})
	require.ErrorIs(t, err, ErrInviteTargetForbidden)
	_, err = ListInvitationCodes(context.Background(), InvitationAdminListQuery{ActorID: 9})
	require.ErrorIs(t, err, ErrInviteTargetForbidden)
	_, err = ListInvitationUses(context.Background(), InvitationAdminListQuery{ActorID: 9})
	require.ErrorIs(t, err, ErrInviteTargetForbidden)
}

func TestInvitationAdminListsHandleHugePageWithoutOffsetOverflow(t *testing.T) {
	db := setupInvitationServiceTest(t)
	seedInvitationUser(t, db, 1, "primary-root", common.RoleRootUser)
	seedInvitationUser(t, db, 10, "owner", common.RoleCommonUser)
	seedInvitationUser(t, db, 11, "registered", common.RoleCommonUser)
	require.NoError(t, db.Create(&model.InviteCodeApplication{
		UserID: 10, Reason: "huge page", Status: model.InviteApplicationStatusWithdrawn,
	}).Error)
	code := model.InviteCode{
		OwnerUserID: 10, Code: "huge-page-code", Kind: model.InviteCodeKindSingle,
		MaxUses: 1, Status: model.InviteCodeStatusRevoked, ExpiresAt: 10_000, IssuedBy: 1,
	}
	require.NoError(t, db.Create(&code).Error)
	require.NoError(t, db.Create(&model.InviteCodeUse{
		InviteCodeID: code.ID, InviterUserID: 10, RegisteredUserID: 11,
		RegisteredAt: 100, Source: "password", AuditEventID: "huge-page-use",
	}).Error)
	query := InvitationAdminListQuery{ActorID: 1, Page: math.MaxInt, PageSize: 20}

	applications, err := ListInvitationApplications(context.Background(), query)
	require.NoError(t, err)
	require.EqualValues(t, 1, applications.Total)
	require.Empty(t, applications.Items)

	codes, err := ListInvitationCodes(context.Background(), query)
	require.NoError(t, err)
	require.EqualValues(t, 1, codes.Total)
	require.Empty(t, codes.Items)

	uses, err := ListInvitationUses(context.Background(), query)
	require.NoError(t, err)
	require.EqualValues(t, 1, uses.Total)
	require.Empty(t, uses.Items)
}

func TestInvitationRewardDetailIncludesCommissionEvidenceWithoutProviderPayload(t *testing.T) {
	db := setupInvitationServiceTest(t)
	seedInvitationUser(t, db, 1, "primary-root", common.RoleRootUser)
	seedInvitationUser(t, db, 10, "inviter", common.RoleCommonUser)
	seedInvitationUser(t, db, 20, "source-user", common.RoleCommonUser)
	code := model.InviteCode{
		OwnerUserID: 10, Code: "evidence-invite-code", Kind: model.InviteCodeKindMulti,
		MaxUses: 10, UsedCount: 1, Status: model.InviteCodeStatusActive, IssuedBy: 10,
	}
	require.NoError(t, db.Create(&code).Error)
	use := model.InviteCodeUse{
		InviteCodeID: code.ID, InviterUserID: 10, RegisteredUserID: 20,
		Source: "password", RegistrationIP: "203.0.113.8", AuditEventID: "evidence-use",
	}
	require.NoError(t, db.Create(&use).Error)
	reward := model.InviteRewardPayout{
		InviteCodeUseID: use.ID, TriggerType: model.InviteRewardTriggerCashSubscription,
		TriggerID: "subscription-order-evidence", RecipientKind: model.InviteRewardRecipientInviter,
		RecipientUserID: 10, EligibleQuota: 1_000, RateBasisPoints: 525,
		Amount: 52, ReversedAmount: 7, ReversalState: model.InviteRewardReversalPartial,
		Status: model.InviteRewardStatusPaid,
	}
	require.NoError(t, db.Create(&reward).Error)
	request := model.InviteRewardPayoutRequest{
		OperationID: "evidence-request", UserID: 10, Amount: 52,
		Status: model.InviteRewardPayoutRequestStatusPaid, ReviewerID: 1,
	}
	require.NoError(t, db.Create(&request).Error)
	require.NoError(t, db.Create(&model.InviteRewardPayoutRequestItem{RequestID: request.ID, RewardID: reward.ID}).Error)

	detail, err := GetInvitationRewardPayoutRequestDetail(context.Background(), 1, request.ID)
	require.NoError(t, err)
	require.Equal(t, int64(7), detail.Request.ReversedAmount)
	require.Equal(t, int64(45), detail.Request.NetAmount)
	require.Len(t, detail.Rewards, 1)
	evidence := detail.Rewards[0]
	require.Equal(t, model.InviteRewardTriggerCashSubscription, evidence.TriggerType)
	require.Equal(t, "subscription-order-evidence", evidence.TriggerID)
	require.Equal(t, int64(1_000), evidence.EligibleQuota)
	require.Equal(t, int64(525), evidence.RateBasisPoints)
	require.Equal(t, int64(52), evidence.Amount)
	require.Equal(t, int64(7), evidence.ReversedAmount)
	require.Equal(t, int64(45), evidence.NetAmount)
	require.Equal(t, model.InviteRewardReversalPartial, evidence.ReversalState)
	require.Equal(t, 20, evidence.SourceUserID)
	require.Equal(t, "source-user", evidence.SourceUsername)
	encoded, err := common.Marshal(detail)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "provider_payload")
}

func applicationUsernames(items []InvitationApplicationListItem) []string {
	usernames := make([]string, 0, len(items))
	for _, item := range items {
		usernames = append(usernames, item.ApplicantUsername)
	}
	return usernames
}
