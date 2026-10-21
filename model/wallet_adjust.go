package model

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrWalletInsufficientBalance = errors.New("wallet balance is insufficient")
	ErrWalletOperationConflict   = errors.New("wallet operation id conflicts with an existing adjustment")
	ErrWalletInvalidAdjustment   = errors.New("wallet adjustment is invalid")
	ErrWalletConcurrentMutation  = errors.New("wallet balance changed concurrently")
)

type WalletInsufficientBalanceError struct {
	Balance int64
}

func (e *WalletInsufficientBalanceError) Error() string {
	return ErrWalletInsufficientBalance.Error()
}

func (e *WalletInsufficientBalanceError) Unwrap() error {
	return ErrWalletInsufficientBalance
}

// WalletAdjustReference is an optional business reference attached to a ledger row.
type WalletAdjustReference struct {
	Type    string
	ID      string
	Context string
}

type walletAdjustOptions struct {
	allowDebt            bool
	skipCheckinLifecycle bool
	includeDeletedUser   bool
}

// AdjustWalletWithTx applies a signed quota delta with a unique operation_id.
// It is the single write path for durable wallet mutations that must appear in
// wallet_ledger_entries (topup, admin changes, invitation, user redemption).
func AdjustWalletWithTx(tx *gorm.DB, userID int, delta int64, operationID, category string, reference WalletAdjustReference) (*WalletLedgerEntry, error) {
	return adjustWalletWithTx(tx, userID, delta, operationID, category, reference, walletAdjustOptions{})
}

// AdjustWalletAllowDebtWithTx applies an adjustment that may leave the wallet
// negative. It is reserved for reversing previously credited business value.
func AdjustWalletAllowDebtWithTx(tx *gorm.DB, userID int, delta int64, operationID, category string, reference WalletAdjustReference) (*WalletLedgerEntry, error) {
	return adjustWalletWithTx(tx, userID, delta, operationID, category, reference, walletAdjustOptions{allowDebt: true})
}

func AdjustWalletUnscopedWithTx(tx *gorm.DB, userID int, delta int64, operationID, category string, reference WalletAdjustReference) (*WalletLedgerEntry, error) {
	return adjustWalletWithTx(tx, userID, delta, operationID, category, reference, walletAdjustOptions{includeDeletedUser: true})
}

func adjustWalletWithTx(tx *gorm.DB, userID int, delta int64, operationID, category string, reference WalletAdjustReference, options walletAdjustOptions) (*WalletLedgerEntry, error) {
	operationID = strings.TrimSpace(operationID)
	category = strings.TrimSpace(category)
	reference.Type = strings.TrimSpace(reference.Type)
	reference.ID = strings.TrimSpace(reference.ID)
	reference.Context = strings.TrimSpace(reference.Context)
	if tx == nil || userID <= 0 || operationID == "" || len(operationID) > 191 || category == "" || len(category) > 64 ||
		len(reference.Type) > 64 || len(reference.ID) > 191 || len(reference.Context) > 255 {
		return nil, ErrWalletInvalidAdjustment
	}

	entry := WalletLedgerEntry{}
	result := tx.Where("operation_id = ?", operationID).Limit(1).Find(&entry)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 1 {
		if matchErr := matchWalletLedgerEntry(&entry, userID, delta, category, reference); matchErr != nil {
			return nil, matchErr
		}
		if err := enqueueWalletLedgerAuditWithTx(tx, &entry); err != nil {
			return nil, err
		}
		return &entry, nil
	}

	user := struct {
		ID    int
		Quota int64
	}{}
	userQuery := func() *gorm.DB {
		query := tx.Model(&User{})
		if options.includeDeletedUser {
			query = query.Unscoped()
		}
		return query
	}
	if err := lockForUpdate(userQuery()).Select("id", "quota").First(&user, userID).Error; err != nil {
		return nil, err
	}

	entry = WalletLedgerEntry{}
	result = lockForUpdate(tx).Where("operation_id = ?", operationID).Limit(1).Find(&entry)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 1 {
		if matchErr := matchWalletLedgerEntry(&entry, userID, delta, category, reference); matchErr != nil {
			return nil, matchErr
		}
		if err := enqueueWalletLedgerAuditWithTx(tx, &entry); err != nil {
			return nil, err
		}
		return &entry, nil
	}

	now := time.Now().Unix()
	if delta < 0 && !options.skipCheckinLifecycle {
		if _, err := settleDueCheckinRewardsForUserWithTx(tx, userID, now); err != nil {
			return nil, err
		}
		if err := userQuery().Select("quota").First(&user, userID).Error; err != nil {
			return nil, err
		}
	}

	newBalance, ok := checkedWalletQuotaBalance(user.Quota, delta)
	if !ok {
		return nil, ErrWalletInvalidAdjustment
	}
	if newBalance < 0 && delta <= 0 && !options.allowDebt {
		return nil, &WalletInsufficientBalanceError{Balance: user.Quota}
	}

	entry = WalletLedgerEntry{
		UserID:        userID,
		OperationID:   operationID,
		Category:      category,
		Delta:         delta,
		BalanceAfter:  newBalance,
		ReferenceType: reference.Type,
		ReferenceID:   reference.ID,
		Context:       reference.Context,
	}
	result = tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "operation_id"}},
		DoNothing: true,
	}).Create(&entry)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		if err := lockForUpdate(tx).Where("operation_id = ?", operationID).First(&entry).Error; err != nil {
			return nil, err
		}
		if matchErr := matchWalletLedgerEntry(&entry, userID, delta, category, reference); matchErr != nil {
			return nil, matchErr
		}
		if err := enqueueWalletLedgerAuditWithTx(tx, &entry); err != nil {
			return nil, err
		}
		return &entry, nil
	}

	result = userQuery().
		Where("id = ? AND quota = ?", userID, user.Quota).
		UpdateColumn("quota", newBalance)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected != 1 {
		return nil, ErrWalletConcurrentMutation
	}
	if !options.skipCheckinLifecycle {
		if err := consumeActiveCheckinRewardWithTx(tx, userID, delta, now); err != nil {
			return nil, err
		}
	}
	if err := enqueueWalletLedgerAuditWithTx(tx, &entry); err != nil {
		return nil, err
	}
	return &entry, nil
}

func OverrideWalletQuotaWithTx(tx *gorm.DB, userID int, quota int64, operationID string) (int64, error) {
	if tx == nil || userID <= 0 || quota < 0 || strings.TrimSpace(operationID) == "" {
		return 0, ErrWalletInvalidAdjustment
	}
	user := struct {
		ID    int
		Quota int64
	}{}
	if err := lockForUpdate(tx).Model(&User{}).Select("id", "quota").First(&user, userID).Error; err != nil {
		return 0, err
	}
	previous := user.Quota
	if quota < user.Quota {
		if _, err := settleDueCheckinRewardsForUserWithTx(tx, userID, time.Now().Unix()); err != nil {
			return 0, err
		}
		if err := tx.Model(&User{}).Select("quota").First(&user, userID).Error; err != nil {
			return 0, err
		}
	}
	if quota == user.Quota {
		return previous, nil
	}
	_, err := AdjustWalletWithTx(tx, userID, quota-user.Quota, operationID, "admin_adjust", WalletAdjustReference{
		Type: "admin_quota", ID: fmt.Sprintf("%d", userID), Context: "override",
	})
	return previous, err
}

func matchWalletLedgerEntry(entry *WalletLedgerEntry, userID int, delta int64, category string, reference WalletAdjustReference) error {
	if entry.UserID != userID || entry.Delta != delta || entry.Category != category ||
		entry.ReferenceType != reference.Type || entry.ReferenceID != reference.ID || entry.Context != reference.Context {
		return ErrWalletOperationConflict
	}
	return nil
}

func checkedWalletQuotaBalance(balance, delta int64) (int64, bool) {
	const maxStoredQuota = int64(^uint64(0) >> 1)
	if delta > 0 && balance > maxStoredQuota-delta {
		return 0, false
	}
	const minStoredQuota = -maxStoredQuota - 1
	if delta < 0 && balance < minStoredQuota-delta {
		return 0, false
	}
	return balance + delta, true
}

func enqueueWalletLedgerAuditWithTx(tx *gorm.DB, entry *WalletLedgerEntry) error {
	if entry == nil {
		return ErrWalletInvalidAdjustment
	}
	// Relay billing already emits the canonical consume/refund log for the request.
	if entry.Category == "relay" && entry.ReferenceType == "relay_request" {
		return nil
	}
	// User redemption operations enqueue one aggregate business event in the same transaction.
	if entry.ReferenceType == "user_redemption_code" && strings.HasPrefix(entry.Category, "user_redemption_") {
		return nil
	}
	// Commission reversals emit one dedicated business event with concise user-facing text.
	if entry.Category == "invitation_commission_reversal" {
		return nil
	}
	// Invitation payout state changes emit one canonical business event.
	switch entry.Category {
	case "invitation_reward_payout", "invitation_reward_freeze", "invitation_reward_unfreeze":
		return nil
	}
	balanceBefore := entry.BalanceAfter - entry.Delta
	payload, err := common.Marshal(map[string]any{
		"target_user_id":         entry.UserID,
		"operation_id":           entry.OperationID,
		"category":               entry.Category,
		"delta":                  entry.Delta,
		"delta_display":          logger.LogQuota64(entry.Delta),
		"balance_before":         balanceBefore,
		"balance_before_display": logger.LogQuota64(balanceBefore),
		"balance_after":          entry.BalanceAfter,
		"balance_after_display":  logger.LogQuota64(entry.BalanceAfter),
		"reference_type":         entry.ReferenceType,
		"reference_id":           entry.ReferenceID,
		"reference_context":      entry.Context,
	})
	if err != nil {
		return fmt.Errorf("serialize wallet audit payload: %w", err)
	}
	record := AuditOutboxEvent{
		EventID:            "wallet:" + entry.OperationID,
		EventType:          "wallet.adjust",
		Result:             "success",
		ActorID:            entry.UserID,
		Target:             fmt.Sprintf("user:%d", entry.UserID),
		Payload:            string(payload),
		Status:             AuditOutboxStatusPending,
		LogSnapshotVersion: 1,
	}
	result := tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "event_id"}},
		DoNothing: true,
	}).Create(&record)
	if result.Error != nil {
		return result.Error
	}
	// Idempotent success path: if event already exists with same payload, accept.
	var existing AuditOutboxEvent
	if err := tx.Where("event_id = ?", record.EventID).First(&existing).Error; err != nil {
		return err
	}
	if existing.EventType != record.EventType || existing.Result != record.Result ||
		existing.ActorID != record.ActorID || existing.Target != record.Target || existing.Payload != record.Payload {
		return fmt.Errorf("wallet audit event conflict for %s", record.EventID)
	}
	return nil
}

// AdjustWallet commits a ledger adjustment in its own transaction and invalidates cache.
func AdjustWallet(userID int, delta int64, operationID, category string, reference WalletAdjustReference) (*WalletLedgerEntry, error) {
	var entry *WalletLedgerEntry
	err := DB.Transaction(func(tx *gorm.DB) error {
		var err error
		entry, err = AdjustWalletWithTx(tx, userID, delta, operationID, category, reference)
		return err
	})
	if err != nil {
		return nil, err
	}
	_ = InvalidateUserCache(userID)
	return entry, nil
}

// CreditTopUpWithTx credits a completed top-up into the wallet ledger.
// operation_id is stable on trade_no so retries are idempotent.
func CreditTopUpWithTx(tx *gorm.DB, userID int, quota int64, tradeNo, provider string) (*WalletLedgerEntry, error) {
	if quota <= 0 {
		return nil, ErrWalletInvalidAdjustment
	}
	tradeNo = strings.TrimSpace(tradeNo)
	if tradeNo == "" {
		return nil, ErrWalletInvalidAdjustment
	}
	provider = strings.TrimSpace(provider)
	if provider == "" {
		provider = "topup"
	}
	return AdjustWalletWithTx(tx, userID, quota, "topup:"+tradeNo, "topup", WalletAdjustReference{
		Type:    "topup",
		ID:      tradeNo,
		Context: provider,
	})
}

// NewAdminQuotaOperationID builds a unique operation id for admin quota mutations.
func NewAdminQuotaOperationID(actorID int, mode, requestID string) string {
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		requestID = uuid.NewString()
	}
	op := fmt.Sprintf("admin:%d:quota:%s:%s", actorID, mode, requestID)
	if len(op) > 191 {
		op = op[:191]
	}
	return op
}
