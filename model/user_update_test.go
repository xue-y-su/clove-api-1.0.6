package model

import (
	"errors"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupUserUpdateTestState(t *testing.T) {
	t.Helper()
	truncateTables(t)
	require.NoError(t, DB.Exec("DELETE FROM users").Error)

	oldRedisEnabled := common.RedisEnabled
	oldBatchUpdateEnabled := common.BatchUpdateEnabled
	common.RedisEnabled = false
	common.BatchUpdateEnabled = false
	t.Cleanup(func() {
		common.RedisEnabled = oldRedisEnabled
		common.BatchUpdateEnabled = oldBatchUpdateEnabled
	})
}

func TestInsertUserWithGeneratedAffCodeRetriesOnlyAffCodeCollision(t *testing.T) {
	setupUserUpdateTestState(t)
	require.NoError(t, DB.Create(&User{
		Id:       1,
		Username: "existing-aff-owner",
		Password: "password",
		AffCode:  "dupe",
	}).Error)

	generated := []string{"dupe", "next"}
	attempt := 0
	user := User{Username: "new-user", Password: "password"}
	err := DB.Transaction(func(tx *gorm.DB) error {
		return insertUserWithGeneratedAffCode(tx, &user, func() string {
			value := generated[attempt]
			attempt++
			return value
		})
	})

	require.NoError(t, err)
	assert.Equal(t, 2, attempt)
	assert.Equal(t, "next", user.AffCode)
	assert.NotZero(t, user.Id)

	var count int64
	require.NoError(t, DB.Model(&User{}).Where("username = ? AND aff_code = ?", "new-user", "next").Count(&count).Error)
	assert.Equal(t, int64(1), count)
}

func TestInsertUserWithGeneratedAffCodeDoesNotRetryUsernameCollision(t *testing.T) {
	setupUserUpdateTestState(t)
	require.NoError(t, DB.Create(&User{
		Id:       1,
		Username: "duplicate-user",
		Password: "password",
		AffCode:  "seed",
	}).Error)

	attempt := 0
	user := User{Username: "duplicate-user", Password: "password"}
	err := DB.Transaction(func(tx *gorm.DB) error {
		return insertUserWithGeneratedAffCode(tx, &user, func() string {
			attempt++
			return "fresh"
		})
	})

	require.Error(t, err)
	assert.Equal(t, 1, attempt)
	assert.True(t, IsDuplicateUsernameError(err))
}

func TestUserUpdateDoesNotOverwriteAccountingFields(t *testing.T) {
	setupUserUpdateTestState(t)

	user := User{
		Id:           1,
		Username:     "quota-race-user",
		Password:     "password",
		DisplayName:  "before",
		Status:       common.UserStatusEnabled,
		Quota:        1000,
		UsedQuota:    20,
		RequestCount: 3,
	}
	require.NoError(t, DB.Create(&user).Error)

	staleUser, err := GetUserById(user.Id, true)
	require.NoError(t, err)

	require.NoError(t, DB.Model(&User{}).Where("id = ?", user.Id).Updates(map[string]interface{}{
		"quota":         gorm.Expr("quota - ?", 400),
		"used_quota":    gorm.Expr("used_quota + ?", 400),
		"request_count": gorm.Expr("request_count + ?", 1),
	}).Error)

	staleUser.DisplayName = "after"
	require.NoError(t, staleUser.Update(false))

	var got User
	require.NoError(t, DB.First(&got, user.Id).Error)
	assert.Equal(t, "after", got.DisplayName)
	assert.Equal(t, int64(600), got.Quota)
	assert.Equal(t, 420, got.UsedQuota)
	assert.Equal(t, 4, got.RequestCount)
}

func TestUserUpdateDoesNotRestoreStaleGovernanceOrProfileState(t *testing.T) {
	setupUserUpdateTestState(t)

	user := User{
		Id:                1,
		Username:          "governed-user",
		Password:          "password",
		DisplayName:       "before",
		AvatarURL:         "https://img.scdn.io/before.png",
		BanReasonPublic:   "old public reason",
		BanReasonInternal: "old internal reason",
		Role:              common.RoleRootUser,
		Status:            common.UserStatusEnabled,
	}
	require.NoError(t, DB.Create(&user).Error)

	staleUser, err := GetUserById(user.Id, true)
	require.NoError(t, err)

	require.NoError(t, DB.Model(&User{}).Where("id = ?", user.Id).Updates(map[string]interface{}{
		"avatar_url":          "https://img.scdn.io/current.png",
		"ban_reason_public":   "current public reason",
		"ban_reason_internal": "current internal reason",
		"role":                common.RoleAdminUser,
		"status":              common.UserStatusDisabled,
	}).Error)

	staleUser.DisplayName = "intentional profile update"
	require.NoError(t, staleUser.Update(false))

	var got User
	require.NoError(t, DB.First(&got, user.Id).Error)
	assert.Equal(t, "intentional profile update", got.DisplayName)
	assert.Equal(t, "https://img.scdn.io/current.png", got.AvatarURL)
	assert.Equal(t, "current public reason", got.BanReasonPublic)
	assert.Equal(t, "current internal reason", got.BanReasonInternal)
	assert.Equal(t, common.RoleAdminUser, got.Role)
	assert.Equal(t, common.UserStatusDisabled, got.Status)
}

func TestUserUpdatePreservesExplicitGovernanceChanges(t *testing.T) {
	setupUserUpdateTestState(t)

	user := User{
		Id:          1,
		Username:    "governance-update-user",
		Password:    "password",
		DisplayName: "before",
		AvatarURL:   "https://img.scdn.io/before.png",
		Role:        common.RoleAdminUser,
		Status:      common.UserStatusEnabled,
	}
	require.NoError(t, DB.Create(&user).Error)

	loaded, err := GetUserById(user.Id, true)
	require.NoError(t, err)
	loaded.DisplayName = "after"
	loaded.AvatarURL = "https://img.scdn.io/after.png"
	loaded.Role = common.RoleRootUser
	loaded.Status = common.UserStatusDisabled

	require.NoError(t, loaded.UpdateWithTx(DB, false))

	var got User
	require.NoError(t, DB.First(&got, user.Id).Error)
	assert.Equal(t, "after", got.DisplayName)
	assert.Equal(t, "https://img.scdn.io/after.png", got.AvatarURL)
	assert.Equal(t, common.RoleRootUser, got.Role)
	assert.Equal(t, common.UserStatusDisabled, got.Status)
}

func TestValidateAccessTokenRejectsDisabledUser(t *testing.T) {
	setupUserUpdateTestState(t)

	accessToken := "disabled-user-access-token"
	require.NoError(t, DB.Create(&User{
		Id:          12,
		Username:    "disabled-access-token-user",
		Password:    "password",
		Status:      common.UserStatusDisabled,
		AccessToken: &accessToken,
	}).Error)

	user, err := ValidateAccessToken("Bearer " + accessToken)

	require.ErrorIs(t, err, ErrUserDisabled)
	require.NotNil(t, user)
	require.Equal(t, 12, user.Id)
}

func TestUpdateUserSettingOnlyUpdatesSetting(t *testing.T) {
	setupUserUpdateTestState(t)

	user := User{
		Id:           2,
		Username:     "setting-user",
		Password:     "password",
		Status:       common.UserStatusEnabled,
		Quota:        1000,
		UsedQuota:    20,
		RequestCount: 3,
	}
	require.NoError(t, DB.Create(&user).Error)

	require.NoError(t, DB.Model(&User{}).Where("id = ?", user.Id).Updates(map[string]interface{}{
		"quota":         gorm.Expr("quota - ?", 250),
		"used_quota":    gorm.Expr("used_quota + ?", 250),
		"request_count": gorm.Expr("request_count + ?", 1),
	}).Error)

	require.NoError(t, UpdateUserSetting(user.Id, dto.UserSetting{Language: "zh"}))

	var got User
	require.NoError(t, DB.First(&got, user.Id).Error)
	assert.Equal(t, int64(750), got.Quota)
	assert.Equal(t, 270, got.UsedQuota)
	assert.Equal(t, 4, got.RequestCount)
	assert.Equal(t, "zh", got.GetSetting().Language)
}

func TestEnsureEmailAvailableRejectsExistingEmailCaseInsensitive(t *testing.T) {
	setupUserUpdateTestState(t)

	require.NoError(t, DB.Create(&User{
		Username: "existing",
		Password: "old-password",
		Email:    "Taken@Example.com",
		Status:   common.UserStatusEnabled,
	}).Error)

	err := EnsureEmailAvailable(" taken@example.COM ", 0)
	require.ErrorIs(t, err, ErrEmailAlreadyTaken)

	user, err := GetUniqueUserByEmail("TAKEN@example.com")
	require.NoError(t, err)
	assert.Equal(t, "existing", user.Username)

	require.NoError(t, EnsureEmailAvailable("taken@example.com", user.Id))
}

func TestInsertRejectsDuplicateEmailWithoutUniqueIndex(t *testing.T) {
	setupUserUpdateTestState(t)

	require.NoError(t, DB.Create(&User{
		Username: "existing",
		Password: "old-password",
		Email:    "taken@example.com",
		Status:   common.UserStatusEnabled,
	}).Error)

	user := &User{
		Username: "oauth-user",
		Email:    "TAKEN@example.com",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
	}

	err := user.Insert(0)
	require.ErrorIs(t, err, ErrEmailAlreadyTaken)

	var count int64
	require.NoError(t, DB.Model(&User{}).Where("username = ?", "oauth-user").Count(&count).Error)
	assert.Zero(t, count)
}

func TestInsertKeepsBlankPasswordForPasswordlessUser(t *testing.T) {
	setupUserUpdateTestState(t)

	user := &User{
		Username: "passwordless-user",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
	}

	require.NoError(t, user.Insert(0))

	var stored User
	require.NoError(t, DB.Where("username = ?", user.Username).First(&stored).Error)
	assert.Empty(t, stored.Password)
}

func TestValidateAndFillRejectsPasswordlessUser(t *testing.T) {
	setupUserUpdateTestState(t)

	require.NoError(t, DB.Create(&User{
		Username: "passwordless-user",
		Password: "",
		Status:   common.UserStatusEnabled,
	}).Error)

	loginUser := User{
		Username: "passwordless-user",
		Password: "NewPassword123",
	}
	err := loginUser.ValidateAndFill()
	require.ErrorIs(t, err, ErrInvalidCredentials)

	var stored User
	require.NoError(t, DB.Where("username = ?", "passwordless-user").First(&stored).Error)
	assert.Empty(t, stored.Password)
}

func TestResetUserPasswordByEmailRequiresSingleActiveMatch(t *testing.T) {
	setupUserUpdateTestState(t)

	require.NoError(t, DB.Create(&User{
		Username: "duplicate-1",
		Password: "old-1",
		Email:    "legacy@example.com",
		AffCode:  "dupe1",
		Status:   common.UserStatusEnabled,
	}).Error)
	require.NoError(t, DB.Create(&User{
		Username: "duplicate-2",
		Password: "old-2",
		Email:    "LEGACY@example.com",
		AffCode:  "dupe2",
		Status:   common.UserStatusEnabled,
	}).Error)

	err := ResetUserPasswordByEmail("legacy@example.com", "NewPassword123")
	require.ErrorIs(t, err, ErrEmailAmbiguous)

	var duplicates []User
	require.NoError(t, DB.Where("LOWER(email) = ?", "legacy@example.com").Order("username asc").Find(&duplicates).Error)
	require.Len(t, duplicates, 2)
	assert.Equal(t, "old-1", duplicates[0].Password)
	assert.Equal(t, "old-2", duplicates[1].Password)

	require.NoError(t, DB.Create(&User{
		Username: "unique",
		Password: "old",
		Email:    "unique@example.com",
		AffCode:  "unique",
		Status:   common.UserStatusEnabled,
	}).Error)

	require.NoError(t, ResetUserPasswordByEmail("UNIQUE@example.com", "NewPassword123"))

	var unique User
	require.NoError(t, DB.Where("username = ?", "unique").First(&unique).Error)
	assert.True(t, common.ValidatePasswordAndHash("NewPassword123", unique.Password))

	err = ResetUserPasswordByEmail("missing@example.com", "NewPassword123")
	require.True(t, errors.Is(err, ErrEmailNotFound))
}
