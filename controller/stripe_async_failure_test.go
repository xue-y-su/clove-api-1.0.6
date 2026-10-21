package controller

import (
	"context"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
	"github.com/stripe/stripe-go/v81"
)

func TestStripeAsyncPaymentFailureMarksSubscriptionOrderFailed(t *testing.T) {
	db := setupEpayTopupIntegrationDB(t)
	seedTokenOwner(t, db, 784, "stripe-async-user", "stripe-async@example.com")
	plan := &model.SubscriptionPlan{
		Title:         "Stripe plan",
		PriceAmount:   1,
		Enabled:       true,
		StripePriceId: "price_local",
	}
	require.NoError(t, db.Create(plan).Error)
	order := &model.SubscriptionOrder{
		UserId:          784,
		PlanId:          plan.Id,
		Money:           1,
		TradeNo:         "stripe-async-subscription-failure",
		PaymentMethod:   model.PaymentMethodStripe,
		PaymentProvider: model.PaymentProviderStripe,
		Status:          common.TopUpStatusPending,
	}
	require.NoError(t, order.Insert())

	event := stripe.Event{Data: &stripe.EventData{Object: map[string]interface{}{
		"client_reference_id": order.TradeNo,
	}}}
	sessionAsyncPaymentFailed(context.Background(), event, "127.0.0.1")

	storedOrder := model.GetSubscriptionOrderByTradeNo(order.TradeNo)
	require.NotNil(t, storedOrder)
	require.Equal(t, common.TopUpStatusFailed, storedOrder.Status)
}

func TestStripeAsyncPaymentFailureMarksOnlyPendingStripeTopUpFailed(t *testing.T) {
	db := setupEpayTopupIntegrationDB(t)
	seedTokenOwner(t, db, 785, "stripe-async-topup-user", "stripe-async-topup@example.com")
	pending := &model.TopUp{
		UserId: 785, TradeNo: "stripe-async-topup-failure", PaymentMethod: model.PaymentMethodStripe,
		PaymentProvider: model.PaymentProviderStripe, Status: common.TopUpStatusPending,
	}
	completed := &model.TopUp{
		UserId: 785, TradeNo: "stripe-async-completed-topup", PaymentMethod: model.PaymentMethodStripe,
		PaymentProvider: model.PaymentProviderStripe, Status: common.TopUpStatusSuccess,
	}
	require.NoError(t, db.Create(&[]*model.TopUp{pending, completed}).Error)

	for _, tradeNo := range []string{pending.TradeNo, completed.TradeNo} {
		event := stripe.Event{Data: &stripe.EventData{Object: map[string]interface{}{
			"client_reference_id": tradeNo,
		}}}
		sessionAsyncPaymentFailed(context.Background(), event, "127.0.0.1")
	}

	require.Equal(t, common.TopUpStatusFailed, model.GetTopUpByTradeNo(pending.TradeNo).Status)
	require.Equal(t, common.TopUpStatusSuccess, model.GetTopUpByTradeNo(completed.TradeNo).Status)
}
