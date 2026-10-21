package model

import (
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func openUserBindingModelTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	gin.SetMode(gin.TestMode)
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	common.RedisEnabled = false

	previousDB := DB
	previousLogDB := LOG_DB

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)

	DB = db
	LOG_DB = db
	require.NoError(t, db.AutoMigrate(&User{}, &UserOAuthBinding{}, &PasskeyCredential{}))

	t.Cleanup(func() {
		DB = previousDB
		LOG_DB = previousLogDB
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func TestEnsureCanRemoveLoginBindingRejectsLastBuiltInOAuth(t *testing.T) {
	openUserBindingModelTestDB(t)
	user := User{
		Id:       1,
		Username: "github-only",
		Password: "",
		GitHubId: "gh-1",
		Status:   common.UserStatusEnabled,
		AffCode:  "abcd",
	}
	require.NoError(t, DB.Create(&user).Error)

	err := EnsureCanRemoveLoginBinding(user.Id, "github", 0)
	require.Error(t, err)
	require.Contains(t, err.Error(), "last login method")
}

func TestEnsureCanRemoveLoginBindingAllowsWhenPasswordRemains(t *testing.T) {
	openUserBindingModelTestDB(t)
	user := User{
		Id:       1,
		Username: "password-plus-github",
		Password: "hashed-password",
		GitHubId: "gh-1",
		Status:   common.UserStatusEnabled,
		AffCode:  "abcd",
	}
	require.NoError(t, DB.Create(&user).Error)

	require.NoError(t, EnsureCanRemoveLoginBinding(user.Id, "github", 0))
}

func TestDeleteUserOAuthBindingWithLoginProtectionRejectsLastCustomOAuth(t *testing.T) {
	openUserBindingModelTestDB(t)
	user := User{
		Id:       1,
		Username: "custom-only",
		Password: "",
		Status:   common.UserStatusEnabled,
		AffCode:  "abcd",
	}
	require.NoError(t, DB.Create(&user).Error)
	require.NoError(t, DB.Create(&UserOAuthBinding{UserId: user.Id, ProviderId: 9, ProviderUserId: "custom-1"}).Error)

	_, err := DeleteUserOAuthBindingWithLoginProtection(user.Id, 9)
	require.Error(t, err)
	require.Contains(t, err.Error(), "last login method")
}

func TestDeleteUserOAuthBindingWithLoginProtectionAllowsWhenPasskeyRemains(t *testing.T) {
	openUserBindingModelTestDB(t)
	user := User{
		Id:       1,
		Username: "custom-plus-passkey",
		Password: "",
		Status:   common.UserStatusEnabled,
		AffCode:  "abcd",
	}
	require.NoError(t, DB.Create(&user).Error)
	require.NoError(t, DB.Create(&UserOAuthBinding{UserId: user.Id, ProviderId: 9, ProviderUserId: "custom-1"}).Error)
	require.NoError(t, DB.Create(&PasskeyCredential{UserID: user.Id, CredentialID: "passkey-1"}).Error)

	updatedVersion, err := DeleteUserOAuthBindingWithLoginProtection(user.Id, 9)
	require.NoError(t, err)
	require.Greater(t, updatedVersion, user.SecurityVersion)
	var count int64
	require.NoError(t, DB.Model(&UserOAuthBinding{}).Where("user_id = ?", user.Id).Count(&count).Error)
	require.Equal(t, int64(0), count)
	var stored User
	require.NoError(t, DB.First(&stored, user.Id).Error)
	require.Equal(t, stored.SecurityVersion, updatedVersion)
}

func TestBindEmailToUserIncrementsSecurityVersion(t *testing.T) {
	openUserBindingModelTestDB(t)
	user := User{
		Id:       2,
		Username: "email-binding-user",
		Password: "hashed-password",
		Status:   common.UserStatusEnabled,
		AffCode:  "email-binding",
	}
	require.NoError(t, DB.Create(&user).Error)
	before := user.SecurityVersion

	require.NoError(t, BindEmailToUser(&user, "user@example.com"))
	require.Equal(t, "user@example.com", user.Email)
	require.Greater(t, user.SecurityVersion, before)
}

func TestClearBindingIncrementsSecurityVersion(t *testing.T) {
	openUserBindingModelTestDB(t)
	user := User{
		Id:       3,
		Username: "clear-binding-user",
		Password: "hashed-password",
		GitHubId: "github-3",
		Status:   common.UserStatusEnabled,
		AffCode:  "clear-binding",
	}
	require.NoError(t, DB.Create(&user).Error)
	before := user.SecurityVersion

	updatedVersion, err := user.ClearBinding("github")
	require.NoError(t, err)
	require.Empty(t, user.GitHubId)
	require.Greater(t, updatedVersion, before)
	require.Equal(t, updatedVersion, user.SecurityVersion)
}

func TestClearBindingRollsBackWhenSecurityVersionBumpFails(t *testing.T) {
	openUserBindingModelTestDB(t)
	user := User{
		Id:       6,
		Username: "clear-binding-rollback",
		Password: "hashed-password",
		GitHubId: "github-6",
		Status:   common.UserStatusEnabled,
		AffCode:  "clear-rollback",
	}
	require.NoError(t, DB.Create(&user).Error)
	before := user.SecurityVersion
	require.NoError(t, DB.Exec(`
		CREATE TRIGGER fail_clear_binding_security_version
		BEFORE UPDATE OF security_version ON users
		WHEN NEW.id = 6
		BEGIN
			SELECT RAISE(ABORT, 'security version write failed');
		END;
	`).Error)

	updatedVersion, err := user.ClearBinding("github")
	require.Error(t, err)
	require.Zero(t, updatedVersion)

	var stored User
	require.NoError(t, DB.First(&stored, user.Id).Error)
	require.Equal(t, "github-6", stored.GitHubId)
	require.Equal(t, before, stored.SecurityVersion)
}

func TestUpdateUserOAuthBindingIncrementsSecurityVersion(t *testing.T) {
	openUserBindingModelTestDB(t)
	user := User{
		Id:       4,
		Username: "custom-binding-user",
		Password: "hashed-password",
		Status:   common.UserStatusEnabled,
		AffCode:  "custom-binding",
	}
	require.NoError(t, DB.Create(&user).Error)
	before := user.SecurityVersion

	require.NoError(t, UpdateUserOAuthBinding(user.Id, 9, "custom-4"))
	require.NoError(t, DB.First(&user, user.Id).Error)
	require.Greater(t, user.SecurityVersion, before)
}

func TestDeleteUserOAuthBindingIncrementsSecurityVersion(t *testing.T) {
	openUserBindingModelTestDB(t)
	user := User{
		Id:       5,
		Username: "delete-custom-binding",
		Password: "hashed-password",
		Status:   common.UserStatusEnabled,
		AffCode:  "delete-custom",
	}
	require.NoError(t, DB.Create(&user).Error)
	require.NoError(t, DB.Create(&UserOAuthBinding{UserId: user.Id, ProviderId: 10, ProviderUserId: "custom-5"}).Error)
	before := user.SecurityVersion

	updatedVersion, err := DeleteUserOAuthBindingWithLoginProtection(user.Id, 10)
	require.NoError(t, err)
	require.NoError(t, DB.First(&user, user.Id).Error)
	require.Greater(t, updatedVersion, before)
	require.Equal(t, user.SecurityVersion, updatedVersion)
}

func TestDeleteUserOAuthBindingRollsBackWhenSecurityVersionBumpFails(t *testing.T) {
	openUserBindingModelTestDB(t)
	user := User{
		Id:       7,
		Username: "delete-binding-rollback",
		Password: "hashed-password",
		Status:   common.UserStatusEnabled,
		AffCode:  "delete-rollback",
	}
	require.NoError(t, DB.Create(&user).Error)
	require.NoError(t, DB.Create(&UserOAuthBinding{UserId: user.Id, ProviderId: 11, ProviderUserId: "custom-7"}).Error)
	before := user.SecurityVersion
	require.NoError(t, DB.Exec(`
		CREATE TRIGGER fail_delete_binding_security_version
		BEFORE UPDATE OF security_version ON users
		WHEN NEW.id = 7
		BEGIN
			SELECT RAISE(ABORT, 'security version write failed');
		END;
	`).Error)

	updatedVersion, err := DeleteUserOAuthBindingWithLoginProtection(user.Id, 11)
	require.Error(t, err)
	require.Zero(t, updatedVersion)

	var count int64
	require.NoError(t, DB.Model(&UserOAuthBinding{}).Where("user_id = ? AND provider_id = ?", user.Id, 11).Count(&count).Error)
	require.Equal(t, int64(1), count)
	var stored User
	require.NoError(t, DB.First(&stored, user.Id).Error)
	require.Equal(t, before, stored.SecurityVersion)
}
