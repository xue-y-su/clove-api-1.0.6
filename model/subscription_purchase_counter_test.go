package model

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupSubscriptionPurchaseCounterDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:subscription_counter_%d?mode=memory&cache=shared&_pragma=busy_timeout(5000)", time.Now().UnixNano())), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(8)
	t.Cleanup(func() { _ = sqlDB.Close() })
	require.NoError(t, db.AutoMigrate(
		&User{},
		&SubscriptionPlan{},
		&SubscriptionOrder{},
		&UserSubscription{},
		&UserRedemptionClaim{},
		&UserRedemptionSubscriptionEntitlement{},
		&SubscriptionPurchaseCounter{},
	))
	return db
}

func TestStricterPositiveSubscriptionLimit(t *testing.T) {
	tests := []struct {
		name              string
		snapshot, current int
		want              int
	}{
		{name: "both unlimited", snapshot: 0, current: 0, want: 0},
		{name: "current limited", snapshot: 0, current: 2, want: 2},
		{name: "snapshot limited", snapshot: 3, current: 0, want: 3},
		{name: "current stricter", snapshot: 3, current: 2, want: 2},
		{name: "snapshot stricter", snapshot: 1, current: 4, want: 1},
		{name: "invalid values are unlimited", snapshot: -1, current: -2, want: 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, StricterPositiveSubscriptionLimit(test.snapshot, test.current))
		})
	}
}

func TestSubscriptionPurchaseCounterIsMonotonicAcrossDeletionAndRollback(t *testing.T) {
	db := setupSubscriptionPurchaseCounterDB(t)
	require.NoError(t, reserveSubscriptionPurchaseSlotTx(db, 41, 7, 0))
	require.NoError(t, db.Create(&UserSubscription{UserId: 41, PlanId: 7, Status: "active"}).Error)
	require.NoError(t, db.Where("user_id = ? AND plan_id = ?", 41, 7).Delete(&UserSubscription{}).Error)

	var counter SubscriptionPurchaseCounter
	require.NoError(t, db.Where("user_id = ? AND plan_id = ?", 41, 7).First(&counter).Error)
	assert.EqualValues(t, 1, counter.UsedCount)

	rollbackErr := errors.New("force rollback")
	err := db.Transaction(func(tx *gorm.DB) error {
		require.NoError(t, reserveSubscriptionPurchaseSlotTx(tx, 41, 7, 0))
		return rollbackErr
	})
	require.ErrorIs(t, err, rollbackErr)
	require.NoError(t, db.Where("user_id = ? AND plan_id = ?", 41, 7).First(&counter).Error)
	assert.EqualValues(t, 1, counter.UsedCount)
}

func TestSubscriptionPurchaseCounterRecordsUnlimitedAndFailsClosedWhenCorrupt(t *testing.T) {
	db := setupSubscriptionPurchaseCounterDB(t)
	require.NoError(t, reserveSubscriptionPurchaseSlotTx(db, 42, 8, 0))
	require.NoError(t, reserveSubscriptionPurchaseSlotTx(db, 42, 8, 0))

	var counter SubscriptionPurchaseCounter
	require.NoError(t, db.Where("user_id = ? AND plan_id = ?", 42, 8).First(&counter).Error)
	assert.EqualValues(t, 2, counter.UsedCount)

	require.NoError(t, db.Model(&counter).Update("used_count", -1).Error)
	err := reserveSubscriptionPurchaseSlotTx(db, 42, 8, 0)
	require.ErrorIs(t, err, ErrSubscriptionPurchaseCounterCorrupt)
}

func TestSubscriptionPurchaseCounterAllowsOnlyOneConcurrentReservation(t *testing.T) {
	db := setupSubscriptionPurchaseCounterDB(t)
	start := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			results <- db.Transaction(func(tx *gorm.DB) error {
				return reserveSubscriptionPurchaseSlotTx(tx, 51, 9, 1)
			})
		}()
	}
	close(start)
	wg.Wait()
	close(results)

	successes := 0
	limitErrors := 0
	for err := range results {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, ErrSubscriptionPurchaseLimit):
			limitErrors++
		default:
			require.NoError(t, err)
		}
	}
	assert.Equal(t, 1, successes)
	assert.Equal(t, 1, limitErrors)
}

func TestBackfillSubscriptionPurchaseCountersUsesStrongestEvidenceAndNeverDecreases(t *testing.T) {
	db := setupSubscriptionPurchaseCounterDB(t)
	now := common.GetTimestamp()
	require.NoError(t, db.Create(&SubscriptionPurchaseCounter{UserID: 61, PlanID: 10, UsedCount: 5}).Error)
	require.NoError(t, db.Create(&UserSubscription{UserId: 61, PlanId: 10, Status: "active"}).Error)
	require.NoError(t, db.Create(&SubscriptionOrder{UserId: 61, PlanId: 10, TradeNo: "backfill-order-1", Status: common.TopUpStatusSuccess}).Error)
	require.NoError(t, db.Create(&SubscriptionOrder{UserId: 62, PlanId: 11, TradeNo: "backfill-order-2", Status: common.TopUpStatusSuccess}).Error)
	require.NoError(t, db.Create(&UserRedemptionSubscriptionEntitlement{CodeID: 1, PlanID: 11}).Error)
	require.NoError(t, db.Create(&UserRedemptionSubscriptionEntitlement{CodeID: 2, PlanID: 11}).Error)
	require.NoError(t, db.Create(&UserRedemptionClaim{CodeID: 1, ClaimerUserID: 62, RewardType: UserRedemptionRewardSubscription, UserSubscriptionID: 201, CreditOpID: "backfill-claim-1", ClaimedAt: now}).Error)
	require.NoError(t, db.Create(&UserRedemptionClaim{CodeID: 2, ClaimerUserID: 62, RewardType: UserRedemptionRewardSubscription, UserSubscriptionID: 202, CreditOpID: "backfill-claim-2", ClaimedAt: now}).Error)
	require.NoError(t, db.Create(&UserSubscription{Id: 201, UserId: 62, PlanId: 11, Status: "active"}).Error)
	require.NoError(t, db.Create(&UserSubscription{Id: 202, UserId: 62, PlanId: 11, Status: "expired"}).Error)

	updated, err := BackfillSubscriptionPurchaseCounters(db)
	require.NoError(t, err)
	assert.EqualValues(t, 1, updated)

	var counters []SubscriptionPurchaseCounter
	require.NoError(t, db.Order("user_id, plan_id").Find(&counters).Error)
	require.Len(t, counters, 2)
	assert.EqualValues(t, 5, counters[0].UsedCount)
	assert.EqualValues(t, 3, counters[1].UsedCount)

	updated, err = BackfillSubscriptionPurchaseCounters(db)
	require.NoError(t, err)
	assert.Zero(t, updated)
}
