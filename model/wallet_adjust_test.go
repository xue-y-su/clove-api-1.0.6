package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestAdjustWalletAllowDebtPreservesOrdinaryDebitProtection(t *testing.T) {
	setupTopupWalletTestDB(t)
	require.NoError(t, DB.Create(&User{
		Id: 21, Username: "debt-user", AffCode: "debt-aff", Quota: 5,
		Status: common.UserStatusEnabled,
	}).Error)
	reference := WalletAdjustReference{
		Type: "invitation_reward_payout", ID: "42", Context: "freeze:1",
	}

	err := DB.Transaction(func(tx *gorm.DB) error {
		_, adjustErr := AdjustWalletWithTx(tx, 21, -10, "ordinary-debit", "test_debit", reference)
		return adjustErr
	})
	require.ErrorIs(t, err, ErrWalletInsufficientBalance)

	var unchanged User
	require.NoError(t, DB.First(&unchanged, 21).Error)
	require.EqualValues(t, 5, unchanged.Quota)

	var entry *WalletLedgerEntry
	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		var adjustErr error
		entry, adjustErr = AdjustWalletAllowDebtWithTx(
			tx, 21, -10, "invitation-freeze:42:1", "invitation_reward_freeze", reference,
		)
		return adjustErr
	}))
	require.EqualValues(t, -5, entry.BalanceAfter)

	var indebted User
	require.NoError(t, DB.First(&indebted, 21).Error)
	require.EqualValues(t, -5, indebted.Quota)
}
