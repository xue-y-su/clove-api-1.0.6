package controller

import (
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func setupPeerRootAuthorizationTest(t *testing.T) {
	t.Helper()
	setupUserRootBindingControllerTest(t)
	seedUserRootBindingUser(t, 1, "primary-root", common.RoleRootUser)
	seedUserRootBindingUser(t, 2, "secondary-root", common.RoleRootUser)
	seedUserRootBindingUser(t, 3, "peer-root", common.RoleRootUser)
}

func TestSecondaryRootCannotUpdatePeerRootCredentialsOrBindings(t *testing.T) {
	setupPeerRootAuthorizationTest(t)
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", 3).Updates(map[string]any{
		"password":  "original-password-hash",
		"email":     "peer-root@example.test",
		"github_id": "peer-root-github",
	}).Error)

	ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/user/", map[string]any{
		"id":           3,
		"username":     "peer-root",
		"display_name": "Peer Root",
		"password":     "Takeover123!",
		"email":        "attacker@example.test",
		"github_id":    "attacker-github",
		"role":         common.RoleRootUser,
		"group":        "default",
	}, 2)
	ctx.Set("username", "secondary-root")
	ctx.Set("role", common.RoleRootUser)

	UpdateUser(ctx)

	response := decodeAPIResponse(t, recorder)
	require.False(t, response.Success)
	var target model.User
	require.NoError(t, model.DB.First(&target, 3).Error)
	require.Equal(t, "original-password-hash", target.Password)
	require.Equal(t, "peer-root@example.test", target.Email)
	require.Equal(t, "peer-root-github", target.GitHubId)
}

func TestPrimaryRootCanUpdatePeerRoot(t *testing.T) {
	setupPeerRootAuthorizationTest(t)

	ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/user/", map[string]any{
		"id":           3,
		"username":     "peer-root-renamed",
		"display_name": "Peer Root",
		"role":         common.RoleRootUser,
		"group":        "default",
	}, 1)
	ctx.Set("username", "primary-root")
	ctx.Set("role", common.RoleRootUser)

	UpdateUser(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	var target model.User
	require.NoError(t, model.DB.First(&target, 3).Error)
	require.Equal(t, "peer-root-renamed", target.Username)
}

func TestSecondaryRootCannotResetPeerRootPasskey(t *testing.T) {
	setupPeerRootAuthorizationTest(t)
	require.NoError(t, model.DB.Create(&model.PasskeyCredential{
		UserID: 3, CredentialID: "peer-root-passkey", PublicKey: "peer-root-public-key",
	}).Error)

	ctx, recorder := newAuthenticatedContext(t, http.MethodDelete, "/api/user/3/reset_passkey", nil, 2)
	ctx.Set("username", "secondary-root")
	ctx.Set("role", common.RoleRootUser)
	ctx.Params = gin.Params{{Key: "id", Value: "3"}}

	AdminResetPasskey(ctx)

	response := decodeAPIResponse(t, recorder)
	require.False(t, response.Success)
	var count int64
	require.NoError(t, model.DB.Model(&model.PasskeyCredential{}).Where("user_id = ?", 3).Count(&count).Error)
	require.Equal(t, int64(1), count)
}

func TestSecondaryRootCannotDisablePeerRootTwoFA(t *testing.T) {
	setupPeerRootAuthorizationTest(t)
	require.NoError(t, model.DB.AutoMigrate(&model.TwoFA{}, &model.TwoFABackupCode{}))
	require.NoError(t, model.DB.Create(&model.TwoFA{
		UserId: 3, Secret: "JBSWY3DPEHPK3PXP", IsEnabled: true,
	}).Error)

	ctx, recorder := newAuthenticatedContext(t, http.MethodDelete, "/api/user/3/2fa", nil, 2)
	ctx.Set("username", "secondary-root")
	ctx.Set("role", common.RoleRootUser)
	ctx.Params = gin.Params{{Key: "id", Value: "3"}}

	AdminDisable2FA(ctx)

	response := decodeAPIResponse(t, recorder)
	require.False(t, response.Success)
	var count int64
	require.NoError(t, model.DB.Model(&model.TwoFA{}).Where("user_id = ?", 3).Count(&count).Error)
	require.Equal(t, int64(1), count)
}

func TestSecondaryRootCannotReadOrUnbindPeerRootCustomOAuth(t *testing.T) {
	setupPeerRootAuthorizationTest(t)
	require.NoError(t, model.DB.AutoMigrate(&model.CustomOAuthProvider{}))
	require.NoError(t, model.DB.Create(&model.CustomOAuthProvider{
		Id: 9, Name: "Peer Provider", Slug: "peer-provider",
	}).Error)
	require.NoError(t, model.DB.Create(&model.UserOAuthBinding{
		UserId: 3, ProviderId: 9, ProviderUserId: "peer-root-external-id",
	}).Error)

	readCtx, readRecorder := newAuthenticatedContext(t, http.MethodGet, "/api/user/3/oauth/bindings", nil, 2)
	readCtx.Set("username", "secondary-root")
	readCtx.Set("role", common.RoleRootUser)
	readCtx.Params = gin.Params{{Key: "id", Value: "3"}}
	GetUserOAuthBindingsByAdmin(readCtx)
	require.False(t, decodeAPIResponse(t, readRecorder).Success)

	deleteCtx, deleteRecorder := newAuthenticatedContext(t, http.MethodDelete, "/api/user/3/oauth/bindings/9", nil, 2)
	deleteCtx.Set("username", "secondary-root")
	deleteCtx.Set("role", common.RoleRootUser)
	deleteCtx.Params = gin.Params{
		{Key: "id", Value: "3"},
		{Key: "provider_id", Value: "9"},
	}
	UnbindCustomOAuthByAdmin(deleteCtx)
	require.False(t, decodeAPIResponse(t, deleteRecorder).Success)

	var count int64
	require.NoError(t, model.DB.Model(&model.UserOAuthBinding{}).
		Where("user_id = ? AND provider_id = ?", 3, 9).Count(&count).Error)
	require.Equal(t, int64(1), count)
}

func TestSecondaryRootCannotEnablePeerRoot(t *testing.T) {
	setupPeerRootAuthorizationTest(t)
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", 3).Updates(map[string]any{
		"status":              common.UserStatusDisabled,
		"ban_reason_public":   "account disabled",
		"ban_reason_internal": "internal reason",
	}).Error)

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/user/manage", map[string]any{
		"id":     3,
		"action": "enable",
	}, 2)
	ctx.Set("username", "secondary-root")
	ctx.Set("role", common.RoleRootUser)

	ManageUser(ctx)

	response := decodeAPIResponse(t, recorder)
	require.False(t, response.Success)
	var target model.User
	require.NoError(t, model.DB.First(&target, 3).Error)
	require.Equal(t, common.UserStatusDisabled, target.Status)
	require.Equal(t, "account disabled", target.BanReasonPublic)
	require.Equal(t, "internal reason", target.BanReasonInternal)
}

func TestSecondaryRootCannotChangePeerRootQuota(t *testing.T) {
	setupPeerRootAuthorizationTest(t)

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/user/manage", map[string]any{
		"id":     3,
		"action": "add_quota",
		"mode":   "add",
		"value":  100,
	}, 2)
	ctx.Set("username", "secondary-root")
	ctx.Set("role", common.RoleRootUser)

	ManageUser(ctx)

	response := decodeAPIResponse(t, recorder)
	require.False(t, response.Success)
	var target model.User
	require.NoError(t, model.DB.First(&target, 3).Error)
	require.Zero(t, target.Quota)
}

func TestManageUserPreservesPrimaryRootAndLowerRoleMatrix(t *testing.T) {
	t.Run("primary root manages peer root", func(t *testing.T) {
		setupPeerRootAuthorizationTest(t)
		ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/user/manage", map[string]any{
			"id":     3,
			"action": "add_quota",
			"mode":   "add",
			"value":  25,
		}, 1)
		ctx.Set("username", "primary-root")
		ctx.Set("role", common.RoleRootUser)

		ManageUser(ctx)

		response := decodeAPIResponse(t, recorder)
		require.True(t, response.Success, response.Message)
		var target model.User
		require.NoError(t, model.DB.First(&target, 3).Error)
		require.Equal(t, int64(25), target.Quota)
		var ledgerCount int64
		require.NoError(t, model.DB.Model(&model.WalletLedgerEntry{}).Where("user_id = ? AND category = ?", 3, "admin_adjust").Count(&ledgerCount).Error)
		require.EqualValues(t, 1, ledgerCount)
	})

	t.Run("secondary root manages lower role", func(t *testing.T) {
		setupPeerRootAuthorizationTest(t)
		require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", 3).Update("role", common.RoleAdminUser).Error)
		ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/user/manage", map[string]any{
			"id":     3,
			"action": "add_quota",
			"mode":   "add",
			"value":  25,
		}, 2)
		ctx.Set("username", "secondary-root")
		ctx.Set("role", common.RoleRootUser)

		ManageUser(ctx)

		response := decodeAPIResponse(t, recorder)
		require.True(t, response.Success, response.Message)
		var target model.User
		require.NoError(t, model.DB.First(&target, 3).Error)
		require.Equal(t, int64(25), target.Quota)
	})
}
