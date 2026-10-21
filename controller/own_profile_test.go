package controller

import (
	"net/http"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service/authz"
	"github.com/stretchr/testify/require"
)

func setupOwnProfilePolicy(t *testing.T, allowedDomains string) {
	t.Helper()

	common.OptionMapRWMutex.Lock()
	previous := common.OptionMap
	next := make(map[string]string, len(previous)+4)
	for key, value := range previous {
		next[key] = value
	}
	next["AvatarUploadEnabled"] = "true"
	next["AvatarUploadEndpoint"] = "https://img.scdn.io/api/v1.php"
	next["AvatarAllowedDomains"] = allowedDomains
	next["UserDisplayNameMaxLength"] = "40"
	next["UserDisplayNameCooldownSeconds"] = "604800"
	common.OptionMap = next
	common.OptionMapRWMutex.Unlock()

	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = previous
		common.OptionMapRWMutex.Unlock()
	})
}

func TestUpdateSelfRejectsAvatarOutsideAllowlist(t *testing.T) {
	setupUserRootBindingControllerTest(t)
	setupOwnProfilePolicy(t, "img.scdn.io")
	seedUserRootBindingUser(t, 21, "avatar-user", common.RoleCommonUser)

	ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/user/self", map[string]any{
		"display_name": "Avatar User",
		"avatar_url":   "https://evil.example/avatar.png",
	}, 21)
	ctx.Set("username", "avatar-user")
	ctx.Set("role", common.RoleCommonUser)

	UpdateSelf(ctx)

	response := decodeAPIResponse(t, recorder)
	require.False(t, response.Success)

	var user model.User
	require.NoError(t, model.DB.First(&user, 21).Error)
	require.Empty(t, user.AvatarURL)
}

func TestManageUserDisableIncrementsSecurityVersion(t *testing.T) {
	setupUserRootBindingControllerTest(t)
	seedUserRootBindingUser(t, 1, "primary-root", common.RoleRootUser)
	seedUserRootBindingUser(t, 22, "managed-user", common.RoleCommonUser)

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/user/manage", map[string]any{
		"id":     22,
		"action": "disable",
	}, 1)
	ctx.Set("username", "primary-root")
	ctx.Set("role", common.RoleRootUser)

	ManageUser(ctx)
	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)

	var user model.User
	require.NoError(t, model.DB.First(&user, 22).Error)
	require.Greater(t, user.SecurityVersion, int64(1))
}

func TestUpdateSelfRejectsDisplayNameDuringCooldown(t *testing.T) {
	setupUserRootBindingControllerTest(t)
	setupOwnProfilePolicy(t, "img.scdn.io")
	seedUserRootBindingUser(t, 23, "cooldown-user", common.RoleCommonUser)
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", 23).Updates(map[string]any{
		"display_name":            "Previous Name",
		"display_name_updated_at": time.Now().Add(-24 * time.Hour).Unix(),
	}).Error)

	ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/user/self", map[string]any{
		"display_name": "New Name",
	}, 23)
	ctx.Set("username", "cooldown-user")
	ctx.Set("role", common.RoleCommonUser)

	UpdateSelf(ctx)
	response := decodeAPIResponse(t, recorder)
	require.False(t, response.Success)
	require.Contains(t, response.Message, "cooldown")
}

func TestUpdateSelfCanClearAvatar(t *testing.T) {
	setupUserRootBindingControllerTest(t)
	setupOwnProfilePolicy(t, "img.scdn.io")
	seedUserRootBindingUser(t, 27, "clear-avatar", common.RoleCommonUser)
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", 27).Update("avatar_url", "https://img.scdn.io/avatar.png").Error)

	ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/user/self", map[string]any{
		"display_name": "clear-avatar",
		"avatar_url":   "",
	}, 27)
	ctx.Set("username", "clear-avatar")
	ctx.Set("role", common.RoleCommonUser)

	UpdateSelf(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	var user model.User
	require.NoError(t, model.DB.First(&user, 27).Error)
	require.Empty(t, user.AvatarURL)
}

func TestGetSelfReturnsProfileGovernanceFields(t *testing.T) {
	setupUserRootBindingControllerTest(t)
	setupOwnProfilePolicy(t, "img.scdn.io")
	seedUserRootBindingUser(t, 28, "profile-fields", common.RoleCommonUser)
	now := time.Now().Unix()
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", 28).Updates(map[string]any{
		"avatar_url":              "https://img.scdn.io/avatar.png",
		"display_name_updated_at": now,
	}).Error)

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/user/self", nil, 28)
	ctx.Set("role", common.RoleCommonUser)
	GetSelf(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	var data struct {
		AvatarURL               string `json:"avatar_url"`
		DisplayNameNextChangeAt int64  `json:"display_name_next_change_at"`
		AvatarUpload            struct {
			Enabled        bool     `json:"enabled"`
			Endpoint       string   `json:"endpoint"`
			AllowedDomains []string `json:"allowed_domains"`
		} `json:"avatar_upload"`
	}
	require.NoError(t, common.Unmarshal(response.Data, &data))
	require.Equal(t, "https://img.scdn.io/avatar.png", data.AvatarURL)
	require.Equal(t, now+604800, data.DisplayNameNextChangeAt)
	require.True(t, data.AvatarUpload.Enabled)
	require.Equal(t, "https://img.scdn.io/api/v1.php", data.AvatarUpload.Endpoint)
	require.Equal(t, []string{"img.scdn.io"}, data.AvatarUpload.AllowedDomains)
}

func TestUpdateUserIncrementsSecurityVersion(t *testing.T) {
	setupUserRootBindingControllerTest(t)
	seedUserRootBindingUser(t, 1, "primary-root", common.RoleRootUser)
	seedUserRootBindingUser(t, 24, "managed-profile", common.RoleCommonUser)

	ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/user/", map[string]any{
		"id":           24,
		"username":     "managed-renamed",
		"display_name": "Managed Profile",
		"role":         common.RoleCommonUser,
		"group":        "default",
	}, 1)
	ctx.Set("username", "primary-root")
	ctx.Set("role", common.RoleRootUser)

	UpdateUser(ctx)
	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)

	var user model.User
	require.NoError(t, model.DB.First(&user, 24).Error)
	require.Greater(t, user.SecurityVersion, int64(1))
}

func TestUpdateUserGroupChangeKeepsCurrentSessionValid(t *testing.T) {
	setupUserRootBindingControllerTest(t)
	seedUserRootBindingUser(t, 1, "primary-root", common.RoleRootUser)
	seedUserRootBindingUser(t, 28, "managed-group", common.RoleCommonUser)

	var before model.User
	require.NoError(t, model.DB.First(&before, 28).Error)

	ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/user/", map[string]any{
		"id":           28,
		"username":     "managed-group",
		"display_name": "managed-group",
		"role":         common.RoleCommonUser,
		"group":        "grok",
	}, 1)
	ctx.Set("username", "primary-root")
	ctx.Set("role", common.RoleRootUser)

	UpdateUser(ctx)
	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)

	var updated model.User
	require.NoError(t, model.DB.First(&updated, 28).Error)
	require.Equal(t, "grok", updated.Group)
	require.Equal(t, before.SecurityVersion, updated.SecurityVersion)
}

func TestUpdateAdminGroupWithUnchangedPermissionsKeepsCurrentSessionValid(t *testing.T) {
	setupUserRootBindingControllerTest(t)
	seedUserRootBindingUser(t, 1, "primary-root", common.RoleRootUser)
	seedUserRootBindingUser(t, 29, "managed-admin", common.RoleAdminUser)
	permissions := authz.Capabilities(29, common.RoleAdminUser)

	var before model.User
	require.NoError(t, model.DB.First(&before, 29).Error)
	ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/user/", map[string]any{
		"id":                29,
		"username":          "managed-admin",
		"display_name":      "managed-admin",
		"role":              common.RoleAdminUser,
		"group":             "grok",
		"admin_permissions": permissions,
	}, 1)
	ctx.Set("username", "primary-root")
	ctx.Set("role", common.RoleRootUser)

	UpdateUser(ctx)
	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)

	var updated model.User
	require.NoError(t, model.DB.First(&updated, 29).Error)
	require.Equal(t, "grok", updated.Group)
	require.Equal(t, before.SecurityVersion, updated.SecurityVersion)
}

func TestUpdateAdminPermissionsStillInvalidatesCurrentSession(t *testing.T) {
	setupUserRootBindingControllerTest(t)
	seedUserRootBindingUser(t, 1, "primary-root", common.RoleRootUser)
	seedUserRootBindingUser(t, 30, "permission-admin", common.RoleAdminUser)
	permissions := authz.Capabilities(30, common.RoleAdminUser)
	permissions[authz.ResourceChannel][authz.ActionSensitiveWrite] = true

	var before model.User
	require.NoError(t, model.DB.First(&before, 30).Error)
	ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/user/", map[string]any{
		"id":                30,
		"username":          "permission-admin",
		"display_name":      "permission-admin",
		"role":              common.RoleAdminUser,
		"group":             "default",
		"admin_permissions": permissions,
	}, 1)
	ctx.Set("username", "primary-root")
	ctx.Set("role", common.RoleRootUser)

	UpdateUser(ctx)
	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)

	var updated model.User
	require.NoError(t, model.DB.First(&updated, 30).Error)
	require.Greater(t, updated.SecurityVersion, before.SecurityVersion)
}

func TestUpdateUserRollsBackWhenSecurityVersionBumpFails(t *testing.T) {
	setupUserRootBindingControllerTest(t)
	seedUserRootBindingUser(t, 1, "primary-root", common.RoleRootUser)
	seedUserRootBindingUser(t, 25, "original-name", common.RoleCommonUser)

	// Simulate a failed security-version write. The user mutation must be in
	// the same transaction, otherwise the account changes while old sessions
	// remain valid.
	require.NoError(t, model.DB.Exec(`
		CREATE TRIGGER fail_security_version_update
		BEFORE UPDATE OF security_version ON users
		WHEN NEW.id = 25
		BEGIN
			SELECT RAISE(ABORT, 'security version write failed');
		END;
	`).Error)

	ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/user/", map[string]any{
		"id":           25,
		"username":     "updated-name",
		"display_name": "Updated Name",
		"role":         common.RoleCommonUser,
		"group":        "default",
	}, 1)
	ctx.Set("username", "primary-root")
	ctx.Set("role", common.RoleRootUser)

	UpdateUser(ctx)
	response := decodeAPIResponse(t, recorder)
	require.False(t, response.Success)

	var user model.User
	require.NoError(t, model.DB.First(&user, 25).Error)
	require.Equal(t, "original-name", user.Username)
	require.Equal(t, "original-name", user.DisplayName)
}

func TestManageUserRollsBackWhenSecurityVersionBumpFails(t *testing.T) {
	setupUserRootBindingControllerTest(t)
	seedUserRootBindingUser(t, 1, "primary-root", common.RoleRootUser)
	seedUserRootBindingUser(t, 26, "managed-user", common.RoleCommonUser)

	require.NoError(t, model.DB.Exec(`
		CREATE TRIGGER fail_managed_security_version_update
		BEFORE UPDATE OF security_version ON users
		WHEN NEW.id = 26
		BEGIN
			SELECT RAISE(ABORT, 'security version write failed');
		END;
	`).Error)

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/user/manage", map[string]any{
		"id":                  26,
		"action":              "disable",
		"ban_reason_public":   "public reason",
		"ban_reason_internal": "internal reason",
	}, 1)
	ctx.Set("username", "primary-root")
	ctx.Set("role", common.RoleRootUser)

	ManageUser(ctx)
	response := decodeAPIResponse(t, recorder)
	require.False(t, response.Success)

	var user model.User
	require.NoError(t, model.DB.First(&user, 26).Error)
	require.Equal(t, common.UserStatusEnabled, user.Status)
	require.Empty(t, user.BanReasonPublic)
	require.Empty(t, user.BanReasonInternal)
}

func TestManageUserRejectsUnknownAction(t *testing.T) {
	setupUserRootBindingControllerTest(t)
	seedUserRootBindingUser(t, 1, "primary-root", common.RoleRootUser)
	seedUserRootBindingUser(t, 27, "managed-user", common.RoleCommonUser)

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/user/manage", map[string]any{
		"id":     27,
		"action": "disable_typo",
	}, 1)
	ctx.Set("username", "primary-root")
	ctx.Set("role", common.RoleRootUser)

	ManageUser(ctx)
	response := decodeAPIResponse(t, recorder)
	require.False(t, response.Success)

	var user model.User
	require.NoError(t, model.DB.First(&user, 27).Error)
	require.Equal(t, common.UserStatusEnabled, user.Status)
}
