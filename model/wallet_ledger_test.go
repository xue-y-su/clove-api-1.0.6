package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWalletLedgerOperationIDHasPortableUniqueIndex(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&WalletLedgerEntry{}))
	require.NoError(t, DB.AutoMigrate(&WalletLedgerEntry{}))
	require.True(t, DB.Migrator().HasIndex(&WalletLedgerEntry{}, "OperationID"))
	require.NoError(t, DB.Exec("DELETE FROM wallet_ledger_entries").Error)
	t.Cleanup(func() { DB.Exec("DELETE FROM wallet_ledger_entries") })

	first := WalletLedgerEntry{UserID: 1, OperationID: "unique-operation", Category: "test"}
	second := WalletLedgerEntry{UserID: 1, OperationID: "unique-operation", Category: "test"}
	require.NoError(t, DB.Create(&first).Error)
	require.Error(t, DB.Create(&second).Error)
}
