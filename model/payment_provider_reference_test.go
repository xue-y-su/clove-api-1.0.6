package model

import (
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestPaymentProviderReferenceIsIdempotentAndConflictSafe(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s/provider-reference.db", t.TempDir())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&PaymentProviderReference{}))
	sqlDB, err := db.DB()
	require.NoError(t, err)
	previousDB := DB
	DB = db
	t.Cleanup(func() {
		DB = previousDB
		require.NoError(t, sqlDB.Close())
	})

	for range 2 {
		require.NoError(t, RecordPaymentProviderReference(PaymentProviderReferenceInput{
			Provider: PaymentProviderStripe, ReferenceType: "charge", ExternalID: "ch_123", TradeNo: "order-123",
		}))
	}
	tradeNo, found, err := FindPaymentTradeNoByProviderReference(PaymentProviderStripe, "charge", "ch_123")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "order-123", tradeNo)

	err = RecordPaymentProviderReference(PaymentProviderReferenceInput{
		Provider: PaymentProviderStripe, ReferenceType: "charge", ExternalID: "ch_123", TradeNo: "other-order",
	})
	require.ErrorIs(t, err, ErrPaymentProviderReferenceConflict)
	_, found, err = FindPaymentTradeNoByProviderReference(PaymentProviderStripe, "charge", "missing")
	require.NoError(t, err)
	require.False(t, found)
}
