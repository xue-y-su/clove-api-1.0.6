package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

var (
	ErrWalletInsufficientBalance = model.ErrWalletInsufficientBalance
	ErrWalletOperationConflict   = model.ErrWalletOperationConflict
	ErrWalletInvalidAdjustment   = model.ErrWalletInvalidAdjustment
	ErrWalletConcurrentMutation  = model.ErrWalletConcurrentMutation
	ErrWalletInvalidQuery        = errors.New("wallet ledger query is invalid")
)

type WalletReference struct {
	Type    string
	ID      string
	Context string
}

type WalletLedgerItem struct {
	ID            int64                     `json:"id"`
	Category      string                    `json:"category"`
	Delta         int64                     `json:"delta"`
	BalanceAfter  int64                     `json:"balance_after"`
	ReferenceType string                    `json:"reference_type"`
	ReferenceID   string                    `json:"reference_id"`
	CreatedAt     int64                     `json:"created_at"`
	EntryCount    int64                     `json:"entry_count"`
	Entries       []WalletLedgerEntryDetail `json:"entries,omitempty"`
}

type WalletLedgerEntryDetail struct {
	ID           int64  `json:"id"`
	Category     string `json:"category"`
	Delta        int64  `json:"delta"`
	BalanceAfter int64  `json:"balance_after"`
	CreatedAt    int64  `json:"created_at"`
}

type WalletLedgerPage struct {
	Items           []WalletLedgerItem `json:"items"`
	Total           int64              `json:"total"`
	Page            int                `json:"page"`
	PageSize        int                `json:"page_size"`
	ActivatedAt     int64              `json:"activated_at"`
	HistoryComplete bool               `json:"history_complete"`
}

func AdjustWithTx(tx *gorm.DB, userID int, delta int64, operationID, category string, reference WalletReference) (*model.WalletLedgerEntry, error) {
	return model.AdjustWalletWithTx(tx, userID, delta, operationID, category, model.WalletAdjustReference{
		Type:    reference.Type,
		ID:      reference.ID,
		Context: reference.Context,
	})
}

func adjustAllowDebtWithTx(tx *gorm.DB, userID int, delta int64, operationID, category string, reference WalletReference) (*model.WalletLedgerEntry, error) {
	return model.AdjustWalletAllowDebtWithTx(tx, userID, delta, operationID, category, model.WalletAdjustReference{
		Type:    reference.Type,
		ID:      reference.ID,
		Context: reference.Context,
	})
}

func adjustUnscopedUserWithTx(tx *gorm.DB, userID int, delta int64, operationID, category string, reference WalletReference) (*model.WalletLedgerEntry, error) {
	return model.AdjustWalletUnscopedWithTx(tx, userID, delta, operationID, category, model.WalletAdjustReference{
		Type:    reference.Type,
		ID:      reference.ID,
		Context: reference.Context,
	})
}

func Adjust(userID int, delta int64, operationID, category string, reference WalletReference) (*model.WalletLedgerEntry, error) {
	var entry *model.WalletLedgerEntry
	if err := model.DB.Transaction(func(tx *gorm.DB) error {
		var err error
		entry, err = AdjustWithTx(tx, userID, delta, operationID, category, reference)
		return err
	}); err != nil {
		return nil, err
	}
	if err := model.InvalidateUserCache(userID); err != nil {
		return nil, fmt.Errorf("wallet adjustment committed but cache invalidation failed: %w", err)
	}
	return entry, nil
}

func ListWalletLedger(ctx context.Context, userID, page, pageSize int) (*WalletLedgerPage, error) {
	return ListWalletLedgerWithEntries(ctx, userID, page, pageSize, false)
}

func ListWalletLedgerWithEntries(ctx context.Context, userID, page, pageSize int, includeEntries bool) (*WalletLedgerPage, error) {
	if model.DB == nil || userID <= 0 {
		return nil, ErrWalletInvalidQuery
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = common.ItemsPerPage
	}
	if pageSize > 100 {
		pageSize = 100
	}

	query := func() *gorm.DB {
		return model.DB.WithContext(ctx).
			Model(&model.WalletLedgerEntry{}).
			Where("user_id = ?", userID)
	}
	groupedReferences := query().
		Select("reference_type, reference_id").
		Group("user_id, reference_type, reference_id")
	var total int64
	if err := model.DB.WithContext(ctx).
		Table("(?) AS wallet_reference_groups", groupedReferences).
		Count(&total).Error; err != nil {
		return nil, err
	}

	type walletLedgerGroup struct {
		ID            int64  `gorm:"column:id"`
		Delta         int64  `gorm:"column:delta"`
		ReferenceType string `gorm:"column:reference_type"`
		ReferenceID   string `gorm:"column:reference_id"`
		CreatedAt     int64  `gorm:"column:created_at"`
		EntryCount    int64  `gorm:"column:entry_count"`
	}
	groups := make([]walletLedgerGroup, 0)
	if err := query().
		Select(`
			(
				SELECT representative.id
				FROM wallet_ledger_entries AS representative
				WHERE representative.user_id = wallet_ledger_entries.user_id
				  AND representative.reference_type = wallet_ledger_entries.reference_type
				  AND representative.reference_id = wallet_ledger_entries.reference_id
				ORDER BY representative.created_at DESC, representative.id DESC
				LIMIT 1
			) AS id,
			SUM(delta) AS delta,
			reference_type,
			reference_id,
			MAX(created_at) AS created_at,
			COUNT(*) AS entry_count`).
		Group("user_id, reference_type, reference_id").
		Order("created_at DESC, id DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Scan(&groups).Error; err != nil {
		return nil, err
	}

	items := make([]WalletLedgerItem, 0, len(groups))
	if len(groups) > 0 {
		ids := make([]int64, 0, len(groups))
		for _, group := range groups {
			ids = append(ids, group.ID)
		}
		representatives := make([]WalletLedgerEntryDetail, 0, len(ids))
		if err := query().
			Select("id", "category", "delta", "balance_after", "created_at").
			Where("id IN ?", ids).
			Scan(&representatives).Error; err != nil {
			return nil, err
		}
		representativeByID := make(map[int64]WalletLedgerEntryDetail, len(representatives))
		for _, representative := range representatives {
			representativeByID[representative.ID] = representative
		}
		for _, group := range groups {
			representative := representativeByID[group.ID]
			items = append(items, WalletLedgerItem{
				ID:            group.ID,
				Category:      representative.Category,
				Delta:         group.Delta,
				BalanceAfter:  representative.BalanceAfter,
				ReferenceType: group.ReferenceType,
				ReferenceID:   group.ReferenceID,
				CreatedAt:     group.CreatedAt,
				EntryCount:    group.EntryCount,
			})
		}
	}

	if includeEntries && len(groups) > 0 {
		conditions := make([]string, 0, len(groups))
		args := make([]any, 0, len(groups)*2)
		for _, group := range groups {
			conditions = append(conditions, "(reference_type = ? AND reference_id = ?)")
			args = append(args, group.ReferenceType, group.ReferenceID)
		}
		type expandedWalletLedgerEntry struct {
			WalletLedgerEntryDetail
			ReferenceType string `gorm:"column:reference_type"`
			ReferenceID   string `gorm:"column:reference_id"`
		}
		expanded := make([]expandedWalletLedgerEntry, 0)
		if err := query().
			Select("id", "category", "delta", "balance_after", "reference_type", "reference_id", "created_at").
			Where(strings.Join(conditions, " OR "), args...).
			Order("created_at DESC, id DESC").
			Scan(&expanded).Error; err != nil {
			return nil, err
		}
		itemByReference := make(map[string]*WalletLedgerItem, len(items))
		for i := range items {
			itemByReference[walletLedgerReferenceKey(items[i].ReferenceType, items[i].ReferenceID)] = &items[i]
		}
		for _, entry := range expanded {
			item := itemByReference[walletLedgerReferenceKey(entry.ReferenceType, entry.ReferenceID)]
			if item != nil {
				item.Entries = append(item.Entries, entry.WalletLedgerEntryDetail)
			}
		}
	}

	activation := struct {
		ActivatedAt int64 `gorm:"column:activated_at"`
	}{}
	if err := query().Select("COALESCE(MIN(created_at), 0) AS activated_at").Scan(&activation).Error; err != nil {
		return nil, err
	}

	return &WalletLedgerPage{
		Items:           items,
		Total:           total,
		Page:            page,
		PageSize:        pageSize,
		ActivatedAt:     activation.ActivatedAt,
		HistoryComplete: false,
	}, nil
}

func walletLedgerReferenceKey(referenceType, referenceID string) string {
	return referenceType + "\x00" + referenceID
}
