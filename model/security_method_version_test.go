package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func setupSecurityMethodVersionTest(t *testing.T) {
	t.Helper()
	db := openUserBindingModelTestDB(t)
	require.NoError(t, db.AutoMigrate(&TwoFA{}, &TwoFABackupCode{}))
}

func seedSecurityMethodUser(t *testing.T, id int) User {
	t.Helper()
	user := User{
		Id:       id,
		Username: "security-method-user",
		Password: "hashed-password",
		Status:   common.UserStatusEnabled,
		AffCode:  "security-method",
	}
	require.NoError(t, DB.Create(&user).Error)
	return user
}

func TestEnableTwoFAIncrementsSecurityVersion(t *testing.T) {
	setupSecurityMethodVersionTest(t)
	user := seedSecurityMethodUser(t, 20)
	twoFA := TwoFA{UserId: user.Id, Secret: "secret"}
	require.NoError(t, DB.Create(&twoFA).Error)
	before := user.SecurityVersion

	require.NoError(t, twoFA.Enable())
	require.NoError(t, DB.First(&user, user.Id).Error)
	require.Greater(t, user.SecurityVersion, before)
}

func TestDisableTwoFAIncrementsSecurityVersion(t *testing.T) {
	setupSecurityMethodVersionTest(t)
	user := seedSecurityMethodUser(t, 21)
	require.NoError(t, DB.Create(&TwoFA{UserId: user.Id, Secret: "secret", IsEnabled: true}).Error)
	before := user.SecurityVersion

	require.NoError(t, DisableTwoFA(user.Id))
	require.NoError(t, DB.First(&user, user.Id).Error)
	require.Greater(t, user.SecurityVersion, before)
}

func TestRegisterPasskeyCredentialIncrementsSecurityVersion(t *testing.T) {
	setupSecurityMethodVersionTest(t)
	user := seedSecurityMethodUser(t, 22)
	before := user.SecurityVersion

	require.NoError(t, RegisterPasskeyCredential(&PasskeyCredential{UserID: user.Id, CredentialID: "passkey-22"}))
	require.NoError(t, DB.First(&user, user.Id).Error)
	require.Greater(t, user.SecurityVersion, before)
}

func TestDeletePasskeyByUserIDIncrementsSecurityVersion(t *testing.T) {
	setupSecurityMethodVersionTest(t)
	user := seedSecurityMethodUser(t, 23)
	require.NoError(t, DB.Create(&PasskeyCredential{UserID: user.Id, CredentialID: "passkey-23"}).Error)
	before := user.SecurityVersion

	require.NoError(t, DeletePasskeyByUserID(user.Id))
	require.NoError(t, DB.First(&user, user.Id).Error)
	require.Greater(t, user.SecurityVersion, before)
}
