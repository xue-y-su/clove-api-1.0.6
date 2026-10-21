package model

import (
	"errors"
	"strings"
)

// Common errors
var (
	ErrDatabase = errors.New("database error")
)

// User auth errors
var (
	ErrInvalidCredentials         = errors.New("invalid credentials")
	ErrUserEmptyCredentials       = errors.New("empty credentials")
	ErrUserDisabled               = errors.New("user disabled")
	ErrNicknameCooldown           = errors.New("display name is still in cooldown")
	ErrEmailAlreadyTaken          = errors.New("email already taken")
	ErrEmailNotFound              = errors.New("email not found")
	ErrEmailAmbiguous             = errors.New("email matches multiple users")
	ErrAffCodeGenerationExhausted = errors.New("generated affiliation code attempts exhausted")
)

// Token auth errors
var (
	ErrTokenNotProvided = errors.New("token not provided")
	ErrTokenInvalid     = errors.New("token invalid")
)

// Redemption errors
var (
	ErrRedeemFailed           = errors.New("redeem.failed")
	ErrRedemptionInvalid      = errors.New("redemption code is invalid")
	ErrRedemptionUsed         = errors.New("redemption code has already been used")
	ErrRedemptionExpired      = errors.New("redemption code has expired")
	ErrRedemptionCodeConflict = errors.New("redemption code conflicts with an existing code")
)

// 2FA errors
var ErrTwoFANotEnabled = errors.New("2fa not enabled")

func IsDuplicateUsernameError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	if !strings.Contains(message, "duplicate") && !strings.Contains(message, "unique constraint") && !strings.Contains(message, "1062") {
		return false
	}
	return strings.Contains(message, "username")
}

func IsDuplicateAffCodeError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	if !strings.Contains(message, "duplicate") && !strings.Contains(message, "unique constraint") && !strings.Contains(message, "1062") {
		return false
	}
	return strings.Contains(message, "aff_code") || strings.Contains(message, "idx_users_aff_code")
}
