package model

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsDuplicateUsernameErrorRecognizesMySQLUsernameConstraint(t *testing.T) {
	err := errors.New("Error 1062 (23000): Duplicate entry '111' for key 'users.username'")

	require.True(t, IsDuplicateUsernameError(err))
}

func TestIsDuplicateUsernameErrorIgnoresOtherUniqueConstraints(t *testing.T) {
	err := errors.New("Error 1062 (23000): Duplicate entry 'abcd' for key 'users.aff_code'")

	require.False(t, IsDuplicateUsernameError(err))
}

func TestIsDuplicateAffCodeErrorRecognizesSupportedDatabaseConstraints(t *testing.T) {
	tests := []error{
		errors.New("UNIQUE constraint failed: users.aff_code"),
		errors.New("Error 1062 (23000): Duplicate entry 'OxUH' for key 'users.idx_users_aff_code'"),
		errors.New(`duplicate key value violates unique constraint "idx_users_aff_code"`),
	}

	for _, err := range tests {
		require.True(t, IsDuplicateAffCodeError(err), err.Error())
	}
}

func TestIsDuplicateAffCodeErrorIgnoresOtherUniqueConstraints(t *testing.T) {
	err := errors.New("Error 1062 (23000): Duplicate entry 'alice' for key 'users.username'")

	require.False(t, IsDuplicateAffCodeError(err))
}
