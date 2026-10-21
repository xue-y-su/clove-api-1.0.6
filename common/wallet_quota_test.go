package common

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWalletQuotaFromAmountString(t *testing.T) {
	tests := []struct {
		name      string
		amount    string
		want      int64
		wantError error
	}{
		{name: "large exact amount", amount: "5000.00", want: 2_500_000_000},
		{name: "smallest exact quota", amount: "0.000002", want: 1},
		{name: "non positive", amount: "0", wantError: ErrWalletAmountNonPositive},
		{name: "too precise", amount: "0.000001", wantError: ErrWalletAmountPrecision},
		{name: "overflow", amount: "18446744073709551615", wantError: ErrWalletAmountOverflow},
		{name: "invalid", amount: "not-a-number", wantError: ErrWalletAmountInvalid},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			quota, err := WalletQuotaFromAmountString(test.amount)
			if test.wantError != nil {
				require.True(t, errors.Is(err, test.wantError), err)
				require.Zero(t, quota)
				return
			}
			require.NoError(t, err)
			require.Equal(t, test.want, quota)
		})
	}
}

func TestWalletAmountFromQuota(t *testing.T) {
	require.Equal(t, "5000.00", WalletAmountFromQuota(2_500_000_000))
	require.Equal(t, "0.00", WalletAmountFromQuota(1))
}
