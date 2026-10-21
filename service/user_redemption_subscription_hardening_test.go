package service

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestPendingSubscriptionRefundUsesEntitlementSnapshot(t *testing.T) {
	setupUserRedemptionTestDB(t)
	plan := seedSubscriptionRedemptionPlan(t, 2, true, true)
	code, unitQuota := createPartiallyClaimedSubscriptionRedemptionForTest(t, 2101, plan, 3, 2102)

	require.NoError(t, model.DB.Model(&model.UserRedemptionCode{}).Where("id = ?", code.ID).Updates(map[string]any{
		"status":        model.UserRedemptionStatusRefundPending,
		"claimed_quota": 0,
		"terminal_at":   common.GetTimestamp(),
	}).Error)

	count, err := FinalizePendingSubscriptionRedemptionRefunds(context.Background(), 10)
	require.NoError(t, err)
	require.Equal(t, 1, count)

	var refunded model.UserRedemptionCode
	require.NoError(t, model.DB.First(&refunded, code.ID).Error)
	require.Equal(t, unitQuota*2, refunded.RefundedQuota)
	var creator model.User
	require.NoError(t, model.DB.First(&creator, 2101).Error)
	require.Equal(t, unitQuota*2+1, creator.Quota)
}

func TestCancelSubscriptionRefundUsesEntitlementSnapshot(t *testing.T) {
	setupUserRedemptionTestDB(t)
	plan := seedSubscriptionRedemptionPlan(t, 2, true, true)
	code, unitQuota := createPartiallyClaimedSubscriptionRedemptionForTest(t, 2103, plan, 3, 2104)
	require.NoError(t, model.DB.Model(&model.UserRedemptionCode{}).Where("id = ?", code.ID).
		Update("claimed_quota", 0).Error)

	cancelled, err := CancelUserRedemptionCode(context.Background(), UserIdentity{ID: 2103, Role: common.RoleCommonUser}, code.ID)
	require.NoError(t, err)
	require.Equal(t, unitQuota*2, cancelled.RefundedQuota)
	var creator model.User
	require.NoError(t, model.DB.First(&creator, 2103).Error)
	require.Equal(t, unitQuota*2+1, creator.Quota)
}

func TestAdminForfeitSubscriptionUsesEntitlementSnapshot(t *testing.T) {
	setupUserRedemptionTestDB(t)
	plan := seedSubscriptionRedemptionPlan(t, 2, true, true)
	code, unitQuota := createPartiallyClaimedSubscriptionRedemptionForTest(t, 2105, plan, 3, 2106)
	require.NoError(t, model.DB.Model(&model.UserRedemptionCode{}).Where("id = ?", code.ID).
		Update("claimed_quota", 0).Error)

	forfeited, err := AdminMutateUserRedemptionCode(context.Background(), UserRedemptionAdminCommand{
		Actor:      UserIdentity{ID: 9100, Role: common.RoleAdminUser},
		CodeID:     code.ID,
		Action:     UserRedemptionActionTerminate,
		Settlement: UserRedemptionSettlementForfeit,
		Reason:     "policy settlement",
	})
	require.NoError(t, err)
	require.Equal(t, unitQuota*2, forfeited.ForfeitedQuota)
}

func TestPendingSubscriptionRefundCreditsSoftDeletedCreator(t *testing.T) {
	setupUserRedemptionTestDB(t)
	plan := seedSubscriptionRedemptionPlan(t, 2, true, true)
	code := createSubscriptionRedemptionForTest(t, 2111, plan, 2)
	unitQuota := subscriptionRedemptionUnitQuota(t, plan.PriceAmount)
	require.NoError(t, model.DB.Model(&model.UserRedemptionCode{}).Where("id = ?", code.ID).Updates(map[string]any{
		"status":      model.UserRedemptionStatusRefundPending,
		"terminal_at": common.GetTimestamp(),
	}).Error)
	require.NoError(t, model.DB.Delete(&model.User{}, 2111).Error)

	count, err := FinalizePendingSubscriptionRedemptionRefunds(context.Background(), 10)
	require.NoError(t, err)
	require.Equal(t, 1, count)

	var creator model.User
	require.NoError(t, model.DB.Unscoped().First(&creator, 2111).Error)
	require.Equal(t, unitQuota*2+1, creator.Quota)
	var refunded model.UserRedemptionCode
	require.NoError(t, model.DB.First(&refunded, code.ID).Error)
	require.Equal(t, model.UserRedemptionStatusRefunded, refunded.Status)
}

func TestPendingSubscriptionRefundDoesNotBlockBehindHardDeletedCreator(t *testing.T) {
	setupUserRedemptionTestDB(t)
	plan := seedSubscriptionRedemptionPlan(t, 1, true, true)
	orphaned := createSubscriptionRedemptionForTest(t, 2121, plan, 1)
	require.NoError(t, model.DB.Model(&model.UserRedemptionCode{}).Where("id = ?", orphaned.ID).Updates(map[string]any{
		"status":      model.UserRedemptionStatusRefundPending,
		"terminal_at": common.GetTimestamp(),
	}).Error)
	require.NoError(t, model.DB.Unscoped().Delete(&model.User{}, 2121).Error)

	valid := createSubscriptionRedemptionForTest(t, 2122, plan, 1)
	unitQuota := subscriptionRedemptionUnitQuota(t, plan.PriceAmount)
	require.NoError(t, model.DB.Model(&model.UserRedemptionCode{}).Where("id = ?", valid.ID).Updates(map[string]any{
		"status":      model.UserRedemptionStatusRefundPending,
		"terminal_at": common.GetTimestamp(),
	}).Error)

	count, err := FinalizePendingSubscriptionRedemptionRefunds(context.Background(), 10)
	require.NoError(t, err)
	require.Equal(t, 2, count)

	var orphanedStored model.UserRedemptionCode
	require.NoError(t, model.DB.First(&orphanedStored, orphaned.ID).Error)
	require.Equal(t, model.UserRedemptionStatusForfeited, orphanedStored.Status)
	require.Equal(t, unitQuota, orphanedStored.ForfeitedQuota)
	require.Equal(t, "creator_deleted", orphanedStored.TerminalReason)

	var validStored model.UserRedemptionCode
	require.NoError(t, model.DB.First(&validStored, valid.ID).Error)
	require.Equal(t, model.UserRedemptionStatusRefunded, validStored.Status)
	var validCreator model.User
	require.NoError(t, model.DB.First(&validCreator, 2122).Error)
	require.Equal(t, unitQuota+1, validCreator.Quota)
}

func TestDisablePlanAuditsEachSealedSubscriptionCode(t *testing.T) {
	setupUserRedemptionTestDB(t)
	plan := seedSubscriptionRedemptionPlan(t, 1, true, true)
	first := createSubscriptionRedemptionForTest(t, 2131, plan, 1)
	second := createSubscriptionRedemptionForTest(t, 2132, plan, 1)

	require.NoError(t, SetSubscriptionPlanEnabled(context.Background(), plan.Id, false, 9101))

	var events []model.AuditOutboxEvent
	require.NoError(t, model.DB.Where("event_type = ?", "user.redemption.subscription.plan_disabled").Order("id ASC").Find(&events).Error)
	require.Len(t, events, 2)
	require.Equal(t, []string{
		"user-redemption:" + strconv.FormatInt(first.ID, 10) + ":plan-disabled",
		"user-redemption:" + strconv.FormatInt(second.ID, 10) + ":plan-disabled",
	}, []string{events[0].EventID, events[1].EventID})
	for _, event := range events {
		var payload map[string]any
		require.NoError(t, common.Unmarshal([]byte(event.Payload), &payload))
		require.EqualValues(t, plan.Id, payload["plan_id"])
	}
}

func TestSubscriptionClaimAuditIncludesPlanAndSubscription(t *testing.T) {
	setupUserRedemptionTestDB(t)
	plan := seedSubscriptionRedemptionPlan(t, 1, true, true)
	code := createSubscriptionRedemptionForTest(t, 2141, plan, 1)
	seedWalletUser(t, 2142, 0)

	claim, err := claimSubscriptionRedemptionForTest(2142, code.Code)
	require.NoError(t, err)

	var event model.AuditOutboxEvent
	require.NoError(t, model.DB.Where("event_type = ?", "user.redemption.subscription.claim").First(&event).Error)
	var payload map[string]any
	require.NoError(t, common.Unmarshal([]byte(event.Payload), &payload))
	require.EqualValues(t, plan.Id, payload["plan_id"])
	require.EqualValues(t, claim.UserSubscriptionID, payload["user_subscription_id"])
}

func TestFreeSubscriptionRefundStillWritesBusinessAudit(t *testing.T) {
	setupUserRedemptionTestDB(t)
	plan := seedSubscriptionRedemptionPlan(t, 0, true, true)
	code := createSubscriptionRedemptionForTest(t, 2151, plan, 1)
	require.NoError(t, model.DB.Model(&model.UserRedemptionCode{}).Where("id = ?", code.ID).Updates(map[string]any{
		"status":      model.UserRedemptionStatusRefundPending,
		"terminal_at": common.GetTimestamp(),
	}).Error)

	count, err := FinalizePendingSubscriptionRedemptionRefunds(context.Background(), 10)
	require.NoError(t, err)
	require.Equal(t, 1, count)
	var eventCount int64
	require.NoError(t, model.DB.Model(&model.AuditOutboxEvent{}).
		Where("event_type = ?", "user.redemption.subscription.refund").Count(&eventCount).Error)
	require.EqualValues(t, 1, eventCount)
}

func TestConcurrentAdminBindAndSubscriptionClaimRespectPurchaseLimit(t *testing.T) {
	setupUserRedemptionTestDB(t)
	sqlDB, err := model.DB.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(4)

	plan := seedSubscriptionRedemptionPlan(t, 1, true, true)
	require.NoError(t, model.DB.Model(plan).Update("max_purchase_per_user", 1).Error)
	plan.MaxPurchasePerUser = 1
	code := createSubscriptionRedemptionForTest(t, 2161, plan, 1)
	seedWalletUser(t, 2162, 0)

	start := make(chan struct{})
	results := make(chan error, 2)
	go func() {
		<-start
		_, bindErr := model.AdminBindSubscription(2162, plan.Id, "concurrency test")
		results <- bindErr
	}()
	go func() {
		<-start
		_, claimErr := claimSubscriptionRedemptionForTest(2162, code.Code)
		results <- claimErr
	}()
	close(start)

	successes := 0
	limitErrors := 0
	for range 2 {
		err := <-results
		switch {
		case err == nil:
			successes++
		case errors.Is(err, ErrUserRedemptionPlanPurchaseLimit) || strings.Contains(err.Error(), "购买上限"):
			limitErrors++
		default:
			require.NoError(t, err)
		}
	}
	require.Equal(t, 1, successes)
	require.Equal(t, 1, limitErrors)

	var count int64
	require.NoError(t, model.DB.Model(&model.UserSubscription{}).
		Where("user_id = ? AND plan_id = ?", 2162, plan.Id).Count(&count).Error)
	require.EqualValues(t, 1, count)
}
