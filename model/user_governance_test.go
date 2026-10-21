package model

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupGovernanceTestState resets the shared test database so each governance
// test starts from a clean users table, and disables Redis/batch updates so
// assertions reflect direct database state.
func setupGovernanceTestState(t *testing.T) {
	t.Helper()
	require.NoError(t, DB.Exec("DELETE FROM users").Error)

	oldRedis := common.RedisEnabled
	oldBatch := common.BatchUpdateEnabled
	common.RedisEnabled = false
	common.BatchUpdateEnabled = false
	t.Cleanup(func() {
		common.RedisEnabled = oldRedis
		common.BatchUpdateEnabled = oldBatch
		DB.Exec("DELETE FROM users")
	})
}

// setupGovernanceOptions installs the governance-related option defaults so the
// avatar/display-name validators read bounded, explicit policy values. The
// previous OptionMap is restored on cleanup.
func setupGovernanceOptions(t *testing.T) {
	t.Helper()
	common.OptionMapRWMutex.Lock()
	previous := common.OptionMap
	common.OptionMap = map[string]string{
		"AvatarUploadEnabled":            "true",
		"AvatarAllowedDomains":           "img.scdn.io",
		"UserDisplayNameMaxLength":       "40",
		"UserDisplayNameCooldownSeconds": "604800",
	}
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = previous
		common.OptionMapRWMutex.Unlock()
	})
}

func TestValidateUsername(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid lowercase", "alice", false},
		{"valid mixed with digits", "Alice_123", false},
		{"valid hyphen", "a-b", false},
		{"valid Chinese", "用户名称", false},
		{"valid mixed Chinese and ASCII", "用户_Alice-123", false},
		{"twenty Chinese characters", strings.Repeat("用", 20), false},
		{"empty rejected", "", true},
		{"too long", strings.Repeat("a", 21), true},
		{"too many Chinese characters", strings.Repeat("用", 21), true},
		{"invalid char dot", "alice.bob", true},
		{"invalid char space", "alice bob", true},
		{"emoji rejected", "用户😀", true},
		{"non Chinese Unicode rejected", "álîce", true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateUsername(test.input)
			if test.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateDisplayName(t *testing.T) {
	setupGovernanceOptions(t)
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"simple", "Alice", false},
		{"empty rejected", "", true},
		{"whitespace only rejected", "   ", true},
		{"max length allowed", strings.Repeat("a", 40), false},
		{"over max rejected", strings.Repeat("a", 41), true},
		{"html angle rejected", "Alice <b>", true},
		{"script rejected", "<script>alert(1)</script>", true},
		{"double space rejected", "Alice  Bob", true},
		{"control char rejected", "Alice\x00", true},
		{"bidi override rejected", "Alice\u202e", true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateDisplayName(test.input)
			if test.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateAvatarURL(t *testing.T) {
	setupGovernanceOptions(t)
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"empty clears avatar", "", false},
		{"allowed domain https", "https://img.scdn.io/avatar.png", false},
		{"allowed domain case-insensitive", "https://IMG.SCDN.IO/avatar.png", false},
		{"http rejected", "http://img.scdn.io/avatar.png", true},
		{"wrong domain rejected", "https://evil.com/avatar.png", true},
		{"ip rejected", "https://127.0.0.1/avatar.png", true},
		{"ipv6 rejected", "https://[::1]/avatar.png", true},
		{"localhost rejected", "https://localhost/avatar.png", true},
		{"localhost case variant rejected", "https://LOCALHOST/avatar.png", true},
		{"localhost port rejected", "https://localhost:443/avatar.png", true},
		{"allowed host port rejected", "https://img.scdn.io:443/avatar.png", true},
		{"fragment rejected", "https://img.scdn.io/avatar.png#fragment", true},
		{"userinfo rejected", "https://user:pass@img.scdn.io/avatar.png", true},
		{"subdomain rejected exact match only", "https://sub.img.scdn.io/avatar.png", true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateAvatarURL(test.input)
			if test.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDefaultAvatarUploadPolicyAllowsImageHostCDN(t *testing.T) {
	setupTopupWalletTestDB(t)
	common.OptionMapRWMutex.Lock()
	previous := common.OptionMap
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = previous
		common.OptionMapRWMutex.Unlock()
	})

	InitOptionMap()

	policy := GetAvatarUploadPolicy()
	require.Contains(t, policy.AllowedDomains, "img.scdn.io")
	require.Contains(t, policy.AllowedDomains, "img.cdn1.vip")
	require.NoError(t, ValidateAvatarURL("https://img.cdn1.vip/i/avatar.png"))
}

func TestUpdateOwnProfileReturnsCommittedProfileWhenCacheInvalidationFails(t *testing.T) {
	setupGovernanceTestState(t)
	setupGovernanceOptions(t)
	user := &User{
		Id: 91, Username: "profile-cache-user", Password: "hashed", DisplayName: "Before",
		Role: common.RoleCommonUser, Status: common.UserStatusEnabled, AffCode: "profile-cache-aff",
	}
	require.NoError(t, DB.Create(user).Error)

	previousInvalidator := invalidateOwnProfileCache
	invalidateOwnProfileCache = func(int) error { return errors.New("redis unavailable") }
	t.Cleanup(func() { invalidateOwnProfileCache = previousInvalidator })

	updated, err := UpdateOwnProfile(user.Id, OwnProfilePatch{
		DisplayName: "After", HasDisplayName: true,
	}, time.Unix(1_800_000_000, 0))
	require.NoError(t, err)
	require.Equal(t, "After", updated.DisplayName)

	var stored User
	require.NoError(t, DB.First(&stored, user.Id).Error)
	require.Equal(t, "After", stored.DisplayName)
}

// TestInsertDefaultsDisplayNameToUsername locks the new-user data contract:
// creation defaults DisplayName to Username.
func TestInsertDefaultsDisplayNameToUsername(t *testing.T) {
	setupGovernanceTestState(t)
	user := &User{
		Username: "newuser",
		Password: "validpassword",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
	}
	require.NoError(t, user.Insert(0))

	var got User
	require.NoError(t, DB.Where("username = ?", "newuser").First(&got).Error)
	assert.Equal(t, "newuser", got.DisplayName)
}

// TestEditWithTxBackfillsEmptyDisplayNameFromUsername locks the lazy backfill
// contract: a legacy user with an empty display name is backfilled from the
// username at the update boundary, without a destructive bulk rewrite.
func TestEditWithTxBackfillsEmptyDisplayNameFromUsername(t *testing.T) {
	setupGovernanceTestState(t)
	user := &User{
		Id:          1,
		Username:    "legacyuser",
		Password:    "hashed",
		DisplayName: "",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		AffCode:     "legacy-aff",
	}
	require.NoError(t, DB.Create(user).Error)

	toEdit := &User{Id: user.Id, Username: user.Username, DisplayName: ""}
	require.NoError(t, toEdit.EditWithTx(DB, false))

	var got User
	require.NoError(t, DB.First(&got, user.Id).Error)
	assert.Equal(t, user.Username, got.DisplayName)
}
