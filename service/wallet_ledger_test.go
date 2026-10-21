package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupWalletLedgerServiceTest(t *testing.T) {
	t.Helper()
	require.NoError(t, model.DB.AutoMigrate(&model.WalletLedgerEntry{}, &model.AuditOutboxEvent{}))
	require.NoError(t, model.DB.Exec("DELETE FROM wallet_ledger_entries").Error)
	require.NoError(t, model.DB.Exec("DELETE FROM audit_outbox_events").Error)
	require.NoError(t, model.DB.Exec("DELETE FROM users").Error)
	t.Cleanup(func() {
		model.DB.Exec("DELETE FROM wallet_ledger_entries")
		model.DB.Exec("DELETE FROM audit_outbox_events")
		model.DB.Exec("DELETE FROM users")
	})
}

func seedWalletLedgerUser(t *testing.T, id int, quota int64) {
	t.Helper()
	require.NoError(t, model.DB.Create(&model.User{
		Id:       id,
		Username: fmt.Sprintf("wallet-ledger-user-%d", id),
		Quota:    quota,
		Status:   common.UserStatusEnabled,
		AffCode:  fmt.Sprintf("wallet-ledger-%d", id),
	}).Error)
}

func TestWalletLedgerAdjustWithTxIsIdempotentByOperationID(t *testing.T) {
	setupWalletLedgerServiceTest(t)
	seedWalletLedgerUser(t, 7, 100)

	require.NoError(t, model.DB.Transaction(func(tx *gorm.DB) error {
		first, err := AdjustWithTx(tx, 7, -25, "case:7", "test", WalletReference{Type: "case", ID: "7"})
		require.NoError(t, err)
		second, err := AdjustWithTx(tx, 7, -25, "case:7", "test", WalletReference{Type: "case", ID: "7"})
		require.NoError(t, err)
		require.Equal(t, first.ID, second.ID)
		return nil
	}))

	var user model.User
	require.NoError(t, model.DB.First(&user, 7).Error)
	require.Equal(t, int64(75), user.Quota)
	var count int64
	require.NoError(t, model.DB.Model(&model.WalletLedgerEntry{}).Count(&count).Error)
	require.EqualValues(t, 1, count)
}

func TestWalletLedgerAdjustWithTxRejectsInsufficientBalance(t *testing.T) {
	setupWalletLedgerServiceTest(t)
	seedWalletLedgerUser(t, 8, 10)

	err := model.DB.Transaction(func(tx *gorm.DB) error {
		_, err := AdjustWithTx(tx, 8, -11, "case:8", "test", WalletReference{})
		return err
	})
	require.ErrorIs(t, err, ErrWalletInsufficientBalance)

	var user model.User
	require.NoError(t, model.DB.First(&user, 8).Error)
	require.Equal(t, int64(10), user.Quota)
	var count int64
	require.NoError(t, model.DB.Model(&model.WalletLedgerEntry{}).Count(&count).Error)
	require.Zero(t, count)
}

func TestWalletLedgerAdjustWithTxRollsBackWithOuterTransaction(t *testing.T) {
	setupWalletLedgerServiceTest(t)
	seedWalletLedgerUser(t, 9, 100)
	sentinel := errors.New("rollback wallet adjustment")

	err := model.DB.Transaction(func(tx *gorm.DB) error {
		_, err := AdjustWithTx(tx, 9, -30, "case:9", "test", WalletReference{})
		require.NoError(t, err)
		return sentinel
	})
	require.ErrorIs(t, err, sentinel)

	var user model.User
	require.NoError(t, model.DB.First(&user, 9).Error)
	require.Equal(t, int64(100), user.Quota)
	var count int64
	require.NoError(t, model.DB.Model(&model.WalletLedgerEntry{}).Count(&count).Error)
	require.Zero(t, count)
}

func TestWalletLedgerRejectsOperationIDReuseWithDifferentPayload(t *testing.T) {
	setupWalletLedgerServiceTest(t)
	seedWalletLedgerUser(t, 10, 100)

	_, err := Adjust(10, -10, "case:10", "test", WalletReference{Type: "case", ID: "10"})
	require.NoError(t, err)
	_, err = Adjust(10, 5, "case:10", "test", WalletReference{Type: "case", ID: "10"})
	require.ErrorIs(t, err, ErrWalletOperationConflict)

	var user model.User
	require.NoError(t, model.DB.First(&user, 10).Error)
	require.Equal(t, int64(90), user.Quota)
}

func TestListWalletLedgerReturnsScopedPageWithoutInternalFields(t *testing.T) {
	setupWalletLedgerServiceTest(t)
	seedWalletLedgerUser(t, 11, 100)
	seedWalletLedgerUser(t, 12, 100)
	require.NoError(t, model.DB.Create(&[]model.WalletLedgerEntry{
		{UserID: 11, OperationID: "ledger:11:one", Category: "topup", Delta: 10, BalanceAfter: 80, ReferenceType: "topup", ReferenceID: "one", Context: "internal-one", CreatedAt: 100},
		{UserID: 11, OperationID: "ledger:11:two", Category: "usage", Delta: -5, BalanceAfter: 75, ReferenceType: "request", ReferenceID: "two", Context: "internal-two", CreatedAt: 200},
		{UserID: 11, OperationID: "ledger:11:three", Category: "invite_reward", Delta: 25, BalanceAfter: 100, ReferenceType: "invite", ReferenceID: "three", Context: "internal-three", CreatedAt: 300},
		{UserID: 12, OperationID: "ledger:12:private", Category: "topup", Delta: 999, BalanceAfter: 999, ReferenceType: "topup", ReferenceID: "private", Context: "other-user-secret", CreatedAt: 50},
	}).Error)

	page, err := ListWalletLedger(context.Background(), 11, 2, 2)
	require.NoError(t, err)
	require.EqualValues(t, 3, page.Total)
	require.Equal(t, 2, page.Page)
	require.Equal(t, 2, page.PageSize)
	require.EqualValues(t, 100, page.ActivatedAt)
	require.False(t, page.HistoryComplete)
	require.Len(t, page.Items, 1)
	require.Equal(t, "one", page.Items[0].ReferenceID)

	payload, err := json.Marshal(page)
	require.NoError(t, err)
	require.NotContains(t, string(payload), "ledger:11:one")
	require.NotContains(t, string(payload), "internal-one")
	require.NotContains(t, string(payload), "other-user-secret")
}

func TestListWalletLedgerGroupsReferencesBeforePagination(t *testing.T) {
	setupWalletLedgerServiceTest(t)
	seedWalletLedgerUser(t, 13, 100)
	require.NoError(t, model.DB.Create(&[]model.WalletLedgerEntry{
		{UserID: 13, OperationID: "ledger:13:reserve", Category: "relay", Delta: -30, BalanceAfter: 70, ReferenceType: "relay_request", ReferenceID: "req-1", CreatedAt: 400},
		{UserID: 13, OperationID: "ledger:13:settle", Category: "relay", Delta: 10, BalanceAfter: 80, ReferenceType: "relay_request", ReferenceID: "req-1", CreatedAt: 500},
		{UserID: 13, OperationID: "ledger:13:topup", Category: "topup", Delta: 50, BalanceAfter: 130, ReferenceType: "topup", ReferenceID: "order-1", CreatedAt: 300},
		{UserID: 13, OperationID: "ledger:13:reward", Category: "invite_reward", Delta: 5, BalanceAfter: 135, ReferenceType: "invite", ReferenceID: "reward-1", CreatedAt: 200},
	}).Error)

	firstPage, err := ListWalletLedger(context.Background(), 13, 1, 2)
	require.NoError(t, err)
	require.EqualValues(t, 3, firstPage.Total)
	require.Len(t, firstPage.Items, 2)
	require.Equal(t, "req-1", firstPage.Items[0].ReferenceID)
	require.EqualValues(t, -20, firstPage.Items[0].Delta)
	require.EqualValues(t, 80, firstPage.Items[0].BalanceAfter)
	require.EqualValues(t, 2, firstPage.Items[0].EntryCount)
	require.Empty(t, firstPage.Items[0].Entries)
	require.Equal(t, "order-1", firstPage.Items[1].ReferenceID)

	secondPage, err := ListWalletLedgerWithEntries(context.Background(), 13, 2, 2, true)
	require.NoError(t, err)
	require.EqualValues(t, 3, secondPage.Total)
	require.Len(t, secondPage.Items, 1)
	require.Equal(t, "reward-1", secondPage.Items[0].ReferenceID)
	require.EqualValues(t, 1, secondPage.Items[0].EntryCount)
	require.Len(t, secondPage.Items[0].Entries, 1)
	require.EqualValues(t, 5, secondPage.Items[0].Entries[0].Delta)
	require.EqualValues(t, 200, secondPage.Items[0].Entries[0].CreatedAt)
}

func TestListWalletLedgerUsesLatestCreatedAtRepresentative(t *testing.T) {
	setupWalletLedgerServiceTest(t)
	seedWalletLedgerUser(t, 14, 100)

	// Insert the latest entry first so the larger ID belongs to the older row.
	require.NoError(t, model.DB.Create(&model.WalletLedgerEntry{
		UserID:        14,
		OperationID:   "ledger:14:latest",
		Category:      "settlement",
		Delta:         10,
		BalanceAfter:  110,
		ReferenceType: "relay_request",
		ReferenceID:   "req-latest",
		CreatedAt:     200,
	}).Error)
	require.NoError(t, model.DB.Create(&model.WalletLedgerEntry{
		UserID:        14,
		OperationID:   "ledger:14:older",
		Category:      "reserve",
		Delta:         -20,
		BalanceAfter:  90,
		ReferenceType: "relay_request",
		ReferenceID:   "req-latest",
		CreatedAt:     100,
	}).Error)

	page, err := ListWalletLedger(context.Background(), 14, 1, 10)
	require.NoError(t, err)
	require.Len(t, page.Items, 1)
	item := page.Items[0]
	require.Equal(t, "req-latest", item.ReferenceID)
	require.EqualValues(t, 200, item.CreatedAt)
	require.Equal(t, "settlement", item.Category)
	require.EqualValues(t, 110, item.BalanceAfter)
	require.EqualValues(t, -10, item.Delta)
	require.EqualValues(t, 2, item.EntryCount)
}
