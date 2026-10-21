package common

import (
	"errors"
	"math"
	"strings"

	"github.com/shopspring/decimal"
)

var (
	ErrWalletAmountRequired    = errors.New("wallet amount is required")
	ErrWalletAmountInvalid     = errors.New("wallet amount is invalid")
	ErrWalletAmountNonPositive = errors.New("wallet amount must be positive")
	ErrWalletAmountPrecision   = errors.New("wallet amount has unsupported precision")
	ErrWalletAmountOverflow    = errors.New("wallet amount exceeds int64 quota range")
)

func WalletQuotaFromAmountString(value string) (int64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, ErrWalletAmountInvalid
	}
	amount, err := decimal.NewFromString(value)
	if err != nil {
		return 0, ErrWalletAmountInvalid
	}
	if !amount.IsPositive() {
		return 0, ErrWalletAmountNonPositive
	}
	unit := decimal.NewFromFloat(QuotaPerUnit)
	if !unit.IsPositive() {
		return 0, ErrWalletAmountInvalid
	}
	quota := amount.Mul(unit)
	if !quota.Equal(quota.Truncate(0)) {
		return 0, ErrWalletAmountPrecision
	}
	if quota.GreaterThan(decimal.NewFromInt(math.MaxInt64)) {
		return 0, ErrWalletAmountOverflow
	}
	return quota.IntPart(), nil
}

func WalletAmountFromQuota(quota int64) string {
	unit := decimal.NewFromFloat(QuotaPerUnit)
	if !unit.IsPositive() {
		return "0.00"
	}
	return decimal.NewFromInt(quota).Div(unit).StringFixed(2)
}
