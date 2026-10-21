package model

type WalletLedgerEntry struct {
	ID            int64  `json:"id" gorm:"primaryKey;autoIncrement"`
	UserID        int    `json:"user_id" gorm:"not null;index"`
	OperationID   string `json:"operation_id" gorm:"type:varchar(191);not null;uniqueIndex"`
	Category      string `json:"category" gorm:"type:varchar(64);not null;index"`
	Delta         int64  `json:"delta" gorm:"type:bigint;not null"`
	BalanceAfter  int64  `json:"balance_after" gorm:"type:bigint;not null"`
	ReferenceType string `json:"reference_type" gorm:"type:varchar(64);not null;default:'';index"`
	ReferenceID   string `json:"reference_id" gorm:"type:varchar(191);not null;default:'';index"`
	Context       string `json:"context" gorm:"type:varchar(255);not null;default:''"`
	CreatedAt     int64  `json:"created_at" gorm:"autoCreateTime"`
}
