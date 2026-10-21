package model

import (
	"fmt"
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupInvitationCommissionTest(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s/commission.db?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)", t.TempDir())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(8)
	require.NoError(t, db.AutoMigrate(
		&User{}, &Checkin{}, &InviteCode{}, &InviteCodeUse{}, &InviteRewardPayout{}, &InviteRewardReversal{},
		&InviteRewardPayoutRequest{}, &InviteRewardPayoutRequestItem{}, &WalletLedgerEntry{}, &AuditOutboxEvent{}, &Log{},
	))

	previousDB := DB
	previousLogDB := LOG_DB
	previousDatabaseType := common.MainDatabaseType()
	previousSetting := *operation_setting.GetInvitationIncentiveSetting()
	DB = db
	LOG_DB = db
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	setting := operation_setting.DefaultInvitationIncentiveSetting()
	setting.CashCommissionEnabled = true
	setting.CashCommissionRate = "10.00"
	setting.AdminBalanceRedemptionCommissionEnabled = true
	setting.AdminBalanceRedemptionCommissionRate = "10.00"
	setting.AdminSubscriptionRedemptionCommissionEnabled = true
	setting.AdminSubscriptionRedemptionCommissionRate = "10.00"
	*operation_setting.GetInvitationIncentiveSetting() = setting
	t.Cleanup(func() {
		DB = previousDB
		LOG_DB = previousLogDB
		common.SetMainDatabaseType(previousDatabaseType)
		*operation_setting.GetInvitationIncentiveSetting() = previousSetting
		require.NoError(t, sqlDB.Close())
	})

	inviter := User{Id: 10, Username: "commission-inviter", AffCode: "commission-inviter-aff", Status: common.UserStatusEnabled}
	invitee := User{
		Id: 20, Username: "commission-invitee", AffCode: "commission-invitee-aff",
		InviterId: inviter.Id, InviterSource: InviterSourceInviteCode, Status: common.UserStatusEnabled,
	}
	require.NoError(t, db.Create(&inviter).Error)
	require.NoError(t, db.Create(&invitee).Error)
	code := InviteCode{
		OwnerUserID: inviter.Id, Code: "COMMISSION1", Kind: InviteCodeKindMulti,
		MaxUses: 10, UsedCount: 1, Status: InviteCodeStatusActive, IssuedBy: inviter.Id,
	}
	require.NoError(t, db.Create(&code).Error)
	require.NoError(t, db.Create(&InviteCodeUse{
		InviteCodeID: code.ID, InviterUserID: inviter.Id, RegisteredUserID: invitee.Id,
		Source: "password", InviterRewardStatus: InviteRewardStatusSkipped,
		InviteeRewardStatus: InviteRewardStatusSkipped, AuditEventID: "commission-invite-use",
	}).Error)
	return db
}

func seedInvitationCommissionReversalState(t *testing.T, db *gorm.DB, status string) (InviteRewardPayout, *InviteRewardPayoutRequest) {
	t.Helper()
	use := InviteCodeUse{}
	require.NoError(t, db.Where("registered_user_id = ?", 20).First(&use).Error)
	reward := InviteRewardPayout{
		InviteCodeUseID: use.ID, TriggerType: InviteRewardTriggerCashTopUp,
		TriggerID: "refund-source-" + status, RecipientKind: InviteRewardRecipientInviter,
		RecipientUserID: 10, EligibleQuota: 1_000, RateBasisPoints: 1_000, Amount: 100,
		ReversalState: InviteRewardReversalNone, Status: status,
	}
	require.NoError(t, db.Create(&reward).Error)
	if status == InviteRewardStatusAvailable {
		return reward, nil
	}
	requestStatus := InviteRewardPayoutRequestStatusPending
	if status == InviteRewardStatusPaid {
		requestStatus = InviteRewardPayoutRequestStatusPaid
	}
	if status == InviteRewardStatusFrozen {
		requestStatus = InviteRewardPayoutRequestStatusFrozen
	}
	pendingUserSlot := 10
	request := &InviteRewardPayoutRequest{
		OperationID: "refund-request-" + status, UserID: 10, Amount: 100,
		Status: requestStatus,
	}
	if requestStatus == InviteRewardPayoutRequestStatusPending {
		request.PendingUserSlot = &pendingUserSlot
	}
	require.NoError(t, db.Create(request).Error)
	require.NoError(t, db.Create(&InviteRewardPayoutRequestItem{RequestID: request.ID, RewardID: reward.ID}).Error)
	return reward, request
}

func TestInvitationCommissionReversalStateMatrix(t *testing.T) {
	for _, status := range []string{
		InviteRewardStatusAvailable,
		InviteRewardStatusPendingPayout,
		InviteRewardStatusPaid,
		InviteRewardStatusFrozen,
	} {
		t.Run(status, func(t *testing.T) {
			db := setupInvitationCommissionTest(t)
			reward, request := seedInvitationCommissionReversalState(t, db, status)
			startingQuota := int64(0)
			if status == InviteRewardStatusPaid {
				startingQuota = 10
				require.NoError(t, db.Model(&User{}).Where("id = ?", 10).Update("quota", startingQuota).Error)
			}
			partialInput := InvitationCommissionReversalInput{
				TriggerType: reward.TriggerType, TriggerID: reward.TriggerID,
				RefundSourceType: "cash_refund", RefundSourceID: "refund-partial", RefundedBase: 400,
			}
			var partial *InviteRewardReversal
			var partialCreated bool
			require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
				var err error
				partial, partialCreated, err = ReverseInvitationCommissionWithTx(tx, partialInput)
				return err
			}))
			require.True(t, partialCreated)
			require.Equal(t, int64(40), partial.Amount)
			require.Equal(t, int64(40), partial.CumulativeReversedAmount)

			var replayed *InviteRewardReversal
			var replayCreated bool
			require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
				var err error
				replayed, replayCreated, err = ReverseInvitationCommissionWithTx(tx, partialInput)
				return err
			}))
			require.False(t, replayCreated)
			require.Equal(t, partial.ID, replayed.ID)

			require.NoError(t, db.First(&reward, reward.ID).Error)
			require.Equal(t, int64(40), reward.ReversedAmount)
			require.Equal(t, InviteRewardReversalPartial, reward.ReversalState)
			if request != nil {
				require.NoError(t, db.First(request, request.ID).Error)
				require.Equal(t, int64(100), request.Amount)
				require.Equal(t, requestStatusForReward(status), request.Status)
			}

			fullInput := partialInput
			fullInput.RefundSourceID = "refund-full"
			fullInput.RefundedBase = 600
			var full *InviteRewardReversal
			require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
				var err error
				full, _, err = ReverseInvitationCommissionWithTx(tx, fullInput)
				return err
			}))
			require.Equal(t, int64(60), full.Amount)
			require.Equal(t, int64(100), full.CumulativeReversedAmount)
			require.NoError(t, db.First(&reward, reward.ID).Error)
			require.Equal(t, int64(100), reward.ReversedAmount)
			require.Equal(t, InviteRewardReversalFull, reward.ReversalState)

			if request != nil {
				require.NoError(t, db.First(request, request.ID).Error)
				if status == InviteRewardStatusPendingPayout {
					require.Equal(t, InviteRewardPayoutRequestStatusCancelled, request.Status)
					require.Nil(t, request.PendingUserSlot)
				}
			}
			var reversalCount int64
			require.NoError(t, db.Model(&InviteRewardReversal{}).Count(&reversalCount).Error)
			require.EqualValues(t, 2, reversalCount)
			var reversalLedger []WalletLedgerEntry
			require.NoError(t, db.Where("category = ?", "invitation_commission_reversal").Order("id ASC").Find(&reversalLedger).Error)
			if status == InviteRewardStatusPaid {
				require.Len(t, reversalLedger, 2)
				require.Equal(t, "invitation-commission-reversal:"+fmt.Sprint(reward.ID)+":cash_refund:refund-partial", reversalLedger[0].OperationID)
				var inviter User
				require.NoError(t, db.First(&inviter, 10).Error)
				require.Equal(t, int64(-90), inviter.Quota)
			} else {
				require.Empty(t, reversalLedger)
			}
			var walletAuditCount int64
			require.NoError(t, db.Model(&AuditOutboxEvent{}).Where("event_type = ?", "wallet.adjust").Count(&walletAuditCount).Error)
			require.Zero(t, walletAuditCount)
			var reversalAuditCount int64
			require.NoError(t, db.Model(&AuditOutboxEvent{}).Where("event_type = ?", "invitation.commission_reversed").Count(&reversalAuditCount).Error)
			require.EqualValues(t, 2, reversalAuditCount)
		})
	}
}

func TestInvitationCommissionReversalAccumulatesFractionalRefundBases(t *testing.T) {
	db := setupInvitationCommissionTest(t)
	use := InviteCodeUse{}
	require.NoError(t, db.Where("registered_user_id = ?", 20).First(&use).Error)
	reward := InviteRewardPayout{
		InviteCodeUseID: use.ID, TriggerType: InviteRewardTriggerCashTopUp, TriggerID: "fractional-refunds",
		RecipientKind: InviteRewardRecipientInviter, RecipientUserID: 10,
		EligibleQuota: 10, RateBasisPoints: 1_000, Amount: 1,
		ReversalState: InviteRewardReversalNone, Status: InviteRewardStatusAvailable,
	}
	require.NoError(t, db.Create(&reward).Error)

	reverse := func(sourceID string) *InviteRewardReversal {
		var reversal *InviteRewardReversal
		require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
			var err error
			reversal, _, err = ReverseInvitationCommissionWithTx(tx, InvitationCommissionReversalInput{
				TriggerType: reward.TriggerType, TriggerID: reward.TriggerID,
				RefundSourceType: "cash_refund", RefundSourceID: sourceID, RefundedBase: 5,
			})
			return err
		}))
		return reversal
	}
	first := reverse("fractional-1")
	second := reverse("fractional-2")
	require.Zero(t, first.Amount)
	require.Equal(t, int64(1), second.Amount)
	require.Equal(t, int64(1), second.CumulativeReversedAmount)
}

func TestInvitationCommissionReversalSkipsTransactionWithoutReward(t *testing.T) {
	db := setupInvitationCommissionTest(t)
	var reversal *InviteRewardReversal
	var created bool
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		var err error
		reversal, created, err = ReverseInvitationCommissionWithTx(tx, InvitationCommissionReversalInput{
			TriggerType: InviteRewardTriggerCashTopUp, TriggerID: "no-commission-order",
			RefundSourceType: "cash_refund", RefundSourceID: "refund-without-reward", RefundedBase: 100,
		})
		return err
	}))
	require.Nil(t, reversal)
	require.False(t, created)
}

func requestStatusForReward(status string) string {
	switch status {
	case InviteRewardStatusPendingPayout:
		return InviteRewardPayoutRequestStatusPending
	case InviteRewardStatusPaid:
		return InviteRewardPayoutRequestStatusPaid
	case InviteRewardStatusFrozen:
		return InviteRewardPayoutRequestStatusFrozen
	default:
		return ""
	}
}

func invitationCommissionPlan(t *testing.T, db *gorm.DB, price float64) *SubscriptionPlan {
	t.Helper()
	plan := &SubscriptionPlan{
		Title: "commission subscription", PriceAmount: price, Currency: "USD",
		DurationUnit: SubscriptionDurationMonth, DurationValue: 1, Enabled: true,
		AllowBalancePay: common.GetPointer(true), AllowWalletOverflow: common.GetPointer(true),
		TotalAmount: 1_000, QuotaResetPeriod: SubscriptionResetNever,
	}
	require.NoError(t, db.Create(plan).Error)
	return plan
}

func TestInvitationCommissionSourceMatrixCashAndBalanceSubscriptions(t *testing.T) {
	db := setupInvitationCommissionTest(t)
	require.NoError(t, db.AutoMigrate(
		&SubscriptionPlan{}, &SubscriptionOrder{}, &UserSubscription{}, &SubscriptionPurchaseCounter{}, &TopUp{}, &WalletLedgerEntry{}, &Checkin{},
	))
	plan := invitationCommissionPlan(t, db, 10)
	snapshotQuota, err := CalculateCashPaymentEligibleQuota("10.00", "1")
	require.NoError(t, err)
	order := SubscriptionOrder{
		UserId: 20, PlanId: plan.Id, Money: 10, TradeNo: "cash-subscription-source",
		PaymentMethod: PaymentMethodStripe, PaymentProvider: PaymentProviderStripe,
		EligibleQuota: snapshotQuota, PaidAmount: "10.00", PaidCurrency: "USD",
		Status: common.TopUpStatusPending, CreateTime: common.GetTimestamp(),
	}
	require.NoError(t, db.Create(&order).Error)

	require.NoError(t, CompleteSubscriptionOrderWithSettlement(
		order.TradeNo, "{}", PaymentProviderStripe, PaymentMethodStripe,
		&CashPaymentSettlement{PaidAmount: "8.00", PaidCurrency: "USD"},
	))
	wantEligibleQuota, err := CalculateCashPaymentEligibleQuota("8.00", "1")
	require.NoError(t, err)
	var cashReward InviteRewardPayout
	require.NoError(t, db.Where("trigger_type = ?", InviteRewardTriggerCashSubscription).First(&cashReward).Error)
	require.Equal(t, order.TradeNo, cashReward.TriggerID)
	require.Equal(t, wantEligibleQuota, cashReward.EligibleQuota)
	require.Equal(t, wantEligibleQuota/10, cashReward.Amount)

	require.NoError(t, db.Model(&User{}).Where("id = ?", 20).Update("quota", snapshotQuota*2).Error)
	require.NoError(t, PurchaseSubscriptionWithBalance(20, plan.Id))
	var rewardCount int64
	require.NoError(t, db.Model(&InviteRewardPayout{}).Count(&rewardCount).Error)
	require.EqualValues(t, 1, rewardCount)
}

func TestInvitationCommissionSourceMatrixFreeSubscriptionCompletesWithoutReward(t *testing.T) {
	db := setupInvitationCommissionTest(t)
	require.NoError(t, db.AutoMigrate(
		&SubscriptionPlan{}, &SubscriptionOrder{}, &UserSubscription{}, &SubscriptionPurchaseCounter{}, &TopUp{},
	))
	plan := invitationCommissionPlan(t, db, 0)
	order := SubscriptionOrder{
		UserId: 20, PlanId: plan.Id, Money: 0, TradeNo: "free-subscription-source",
		PaymentMethod: PaymentMethodStripe, PaymentProvider: PaymentProviderStripe,
		Status: common.TopUpStatusPending, CreateTime: common.GetTimestamp(),
	}
	require.NoError(t, db.Create(&order).Error)

	require.NoError(t, CompleteSubscriptionOrder(order.TradeNo, "{}", PaymentProviderStripe, PaymentMethodStripe))
	var rewardCount int64
	require.NoError(t, db.Model(&InviteRewardPayout{}).Count(&rewardCount).Error)
	require.Zero(t, rewardCount)
}

func TestInvitationCommissionSourceMatrixAdminBalanceRedemption(t *testing.T) {
	db := setupInvitationCommissionTest(t)
	require.NoError(t, db.AutoMigrate(&Redemption{}, &UserRedemptionCode{}, &WalletLedgerEntry{}))
	redemption := Redemption{
		Key: "0123456789abcdef0123456789abcdef", Name: "official balance",
		Quota: 1_000, Status: common.RedemptionCodeStatusEnabled, CreatedTime: common.GetTimestamp(),
	}
	require.NoError(t, redemption.Insert())

	quota, err := Redeem(redemption.Key, 20)
	require.NoError(t, err)
	require.Equal(t, redemption.Quota, quota)
	var reward InviteRewardPayout
	require.NoError(t, db.Where("trigger_type = ?", InviteRewardTriggerAdminBalanceRedemption).First(&reward).Error)
	require.Equal(t, fmt.Sprintf("%d", redemption.Id), reward.TriggerID)
	require.Equal(t, int64(redemption.Quota), reward.EligibleQuota)
	require.Equal(t, int64(100), reward.Amount)
}

func TestCreateInvitationCommissionCalculatesExactRatesAndFloor(t *testing.T) {
	tests := []struct {
		name string
		base int64
		rate int64
		want int64
	}{
		{name: "two hundred percent", base: 900, rate: 20_000, want: 1_800},
		{name: "five point twenty five percent", base: 10_000, rate: 525, want: 525},
		{name: "floor fractional quota", base: 101, rate: 525, want: 5},
	}
	for index, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db := setupInvitationCommissionTest(t)
			input := InvitationCommissionInput{
				InviteeUserID: 20, TriggerType: InviteRewardTriggerCashTopUp,
				TriggerID: fmt.Sprintf("cash-%d", index), EligibleQuota: test.base, RateBasisPoints: test.rate,
			}
			reward, created, err := CreateInvitationCommissionWithTx(db, input)
			require.NoError(t, err)
			require.True(t, created)
			require.Equal(t, test.want, reward.Amount)
			require.Equal(t, test.base, reward.EligibleQuota)
			require.Equal(t, test.rate, reward.RateBasisPoints)
			require.Equal(t, InviteRewardStatusAvailable, reward.Status)
			require.Equal(t, InviteRewardReversalNone, reward.ReversalState)

			replayed, created, err := CreateInvitationCommissionWithTx(db, input)
			require.NoError(t, err)
			require.False(t, created)
			require.Equal(t, reward.ID, replayed.ID)
			var rewardCount, eventCount int64
			require.NoError(t, db.Model(&InviteRewardPayout{}).Count(&rewardCount).Error)
			require.NoError(t, db.Model(&AuditOutboxEvent{}).Count(&eventCount).Error)
			require.Equal(t, int64(1), rewardCount)
			require.Equal(t, int64(1), eventCount)
		})
	}
}

func TestCreateInvitationCommissionSkipsIneligibleSourcesQuietly(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*gorm.DB)
	}{
		{name: "no inviter", mutate: func(db *gorm.DB) {
			require.NoError(t, db.Model(&User{}).Where("id = ?", 20).Updates(map[string]any{"inviter_id": 0, "inviter_source": ""}).Error)
		}},
		{name: "source attribution inviter", mutate: func(db *gorm.DB) {
			require.NoError(t, db.Model(&User{}).Where("id = ?", 20).Update("inviter_source", InviterSourceSourceAttribution).Error)
		}},
		{name: "master switch disabled", mutate: func(_ *gorm.DB) {
			operation_setting.GetInvitationIncentiveSetting().Enabled = false
		}},
		{name: "source switch disabled", mutate: func(_ *gorm.DB) {
			operation_setting.GetInvitationIncentiveSetting().CashCommissionEnabled = false
		}},
	}
	for index, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db := setupInvitationCommissionTest(t)
			test.mutate(db)
			reward, created, err := CreateInvitationCommissionWithTx(db, InvitationCommissionInput{
				InviteeUserID: 20, TriggerType: InviteRewardTriggerCashTopUp,
				TriggerID: fmt.Sprintf("skip-%d", index), EligibleQuota: 1_000, RateBasisPoints: 1_000,
			})
			require.NoError(t, err)
			require.False(t, created)
			require.Nil(t, reward)
			var rewardCount, eventCount int64
			require.NoError(t, db.Model(&InviteRewardPayout{}).Count(&rewardCount).Error)
			require.NoError(t, db.Model(&AuditOutboxEvent{}).Count(&eventCount).Error)
			require.Zero(t, rewardCount)
			require.Zero(t, eventCount)
		})
	}
}

func TestCreateInvitationCommissionSkipsZeroRewardAndRejectsInvalidInput(t *testing.T) {
	db := setupInvitationCommissionTest(t)
	reward, created, err := CreateInvitationCommissionWithTx(db, InvitationCommissionInput{
		InviteeUserID: 20, TriggerType: InviteRewardTriggerCashTopUp,
		TriggerID: "zero-rate", EligibleQuota: 1_000, RateBasisPoints: 0,
	})
	require.NoError(t, err)
	require.False(t, created)
	require.Nil(t, reward)

	_, _, err = CreateInvitationCommissionWithTx(db, InvitationCommissionInput{
		InviteeUserID: 20, TriggerType: "unknown", TriggerID: "unknown-1",
		EligibleQuota: 1_000, RateBasisPoints: 1_000,
	})
	require.ErrorIs(t, err, ErrInvitationCommissionInvalid)
	_, _, err = CreateInvitationCommissionWithTx(db, InvitationCommissionInput{
		InviteeUserID: 20, TriggerType: InviteRewardTriggerCashTopUp, TriggerID: "negative-base",
		EligibleQuota: -1, RateBasisPoints: 1_000,
	})
	require.ErrorIs(t, err, ErrInvitationCommissionInvalid)

	var rewardCount, eventCount int64
	require.NoError(t, db.Model(&InviteRewardPayout{}).Count(&rewardCount).Error)
	require.NoError(t, db.Model(&AuditOutboxEvent{}).Count(&eventCount).Error)
	require.Zero(t, rewardCount)
	require.Zero(t, eventCount)
}

func TestCreateInvitationCommissionConcurrentReplayCreatesOneFactAndEvent(t *testing.T) {
	db := setupInvitationCommissionTest(t)
	input := InvitationCommissionInput{
		InviteeUserID: 20, TriggerType: InviteRewardTriggerCashTopUp,
		TriggerID: "concurrent-cash", EligibleQuota: 1_000, RateBasisPoints: 1_000,
	}
	const workers = 8
	results := make(chan error, workers)
	var start sync.WaitGroup
	start.Add(1)
	for range workers {
		go func() {
			start.Wait()
			_, _, err := CreateInvitationCommissionWithTx(db, input)
			results <- err
		}()
	}
	start.Done()
	for range workers {
		require.NoError(t, <-results)
	}

	var rewardCount, eventCount int64
	require.NoError(t, db.Model(&InviteRewardPayout{}).Count(&rewardCount).Error)
	require.NoError(t, db.Model(&AuditOutboxEvent{}).Count(&eventCount).Error)
	require.Equal(t, int64(1), rewardCount)
	require.Equal(t, int64(1), eventCount)
}
