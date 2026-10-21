package controller

import (
	"net/http"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/registration_setting"
	"github.com/stretchr/testify/require"
)

func setupInvitationRegistrationControllerTest(t *testing.T) {
	t.Helper()
	setupRegistrationAffControllerTest(t)
	require.NoError(t, model.DB.AutoMigrate(
		&model.InviteCodeApplication{}, &model.InviteCode{}, &model.InviteCodeUse{},
		&model.WalletLedgerEntry{}, &model.AuditOutboxEvent{},
	))
}

func TestPasswordRegistrationRejectsLegacyAffAndConsumesApprovedInvitation(t *testing.T) {
	setupInvitationRegistrationControllerTest(t)
	legacy := model.User{
		Id: 1, Username: "legacy-inviter", Password: "hashed", DisplayName: "Legacy Inviter",
		Role: common.RoleCommonUser, Status: common.UserStatusEnabled, AffCode: "legacy-only",
	}
	require.NoError(t, model.DB.Create(&legacy).Error)
	ownerID := legacy.Id
	invite := model.InviteCode{
		OwnerUserID: ownerID, Code: "approved-code", ActiveOwnerSlot: &ownerID,
		Kind: model.InviteCodeKindSingle, MaxUses: 1, Status: model.InviteCodeStatusActive,
		ExpiresAt: time.Now().Add(24 * time.Hour).Unix(), IssuedBy: 1,
	}
	require.NoError(t, model.DB.Create(&invite).Error)
	require.NoError(t, model.UpdateOptionsBulk(map[string]string{registration_setting.AffRequiredKey: "true"}))

	legacyCtx, legacyRecorder := newAuthenticatedContext(t, http.MethodPost, "/api/user/register", map[string]any{
		"username": "legacy-rejected", "password": "password123", "aff_code": legacy.AffCode,
	}, 0)
	Register(legacyCtx)
	require.False(t, decodeAPIResponse(t, legacyRecorder).Success)

	approvedCtx, approvedRecorder := newAuthenticatedContext(t, http.MethodPost, "/api/user/register", map[string]any{
		"username": "approved-user", "password": "password123", "aff_code": invite.Code,
	}, 0)
	approvedCtx.Request.RemoteAddr = "203.0.113.55:12345"
	Register(approvedCtx)
	require.True(t, decodeAPIResponse(t, approvedRecorder).Success)
	var user model.User
	require.NoError(t, model.DB.First(&user, "username = ?", "approved-user").Error)
	require.Equal(t, ownerID, user.InviterId)
	var use model.InviteCodeUse
	require.NoError(t, model.DB.First(&use, "registered_user_id = ?", user.Id).Error)
	require.Equal(t, "203.0.113.55", use.RegistrationIP)
	var stored model.InviteCode
	require.NoError(t, model.DB.First(&stored, invite.ID).Error)
	require.Equal(t, model.InviteCodeStatusExhausted, stored.Status)
}

func TestOptionalRegistrationRejectsSuppliedInvalidInvitation(t *testing.T) {
	setupInvitationRegistrationControllerTest(t)
	common.OptionMapRWMutex.Lock()
	common.OptionMap[registration_setting.AffRequiredKey] = "false"
	common.OptionMapRWMutex.Unlock()

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/user/register", map[string]any{
		"username": "invalid-supplied", "password": "password123", "aff_code": "not-approved",
	}, 0)
	Register(ctx)

	require.False(t, decodeAPIResponse(t, recorder).Success)
	var count int64
	require.NoError(t, model.DB.Model(&model.User{}).Where("username = ?", "invalid-supplied").Count(&count).Error)
	require.Zero(t, count)
}
