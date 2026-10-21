package controller

import (
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/stretchr/testify/require"
	"github.com/stripe/stripe-go/v81"
)

func TestStripeCheckoutFailureKeepsSubscriptionOrderPendingForLateWebhook(t *testing.T) {
	confirmPaymentComplianceForTest(t)
	db := setupEpayTopupIntegrationDB(t)
	seedTokenOwner(t, db, 795, "stripe-subscription-failure", "stripe-subscription-failure@example.com")
	plan := &model.SubscriptionPlan{
		Title:         "Stripe checkout failure plan",
		PriceAmount:   1,
		Enabled:       true,
		StripePriceId: "price_subscription_failure",
	}
	require.NoError(t, db.Create(plan).Error)
	model.InvalidateSubscriptionPlanCache(plan.Id)

	previousAPISecret := setting.StripeApiSecret
	previousWebhookSecret := setting.StripeWebhookSecret
	previousStripeKey := stripe.Key
	t.Cleanup(func() {
		setting.StripeApiSecret = previousAPISecret
		setting.StripeWebhookSecret = previousWebhookSecret
		stripe.Key = previousStripeKey
	})
	setting.StripeApiSecret = "sk_test_subscription_failure"
	setting.StripeWebhookSecret = "whsec_test_subscription_failure"
	useStripeAPIBackendForTest(t, &http.Client{Transport: paymentRequestBlockingTransport{}})

	context, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/subscription/stripe/pay", SubscriptionStripePayRequest{
		PlanId: plan.Id,
	}, 795)
	SubscriptionRequestStripePay(context)

	require.Equal(t, http.StatusOK, recorder.Code)
	response := decodeAPIResponse(t, recorder)
	require.Equal(t, "error", response.Message)
	require.JSONEq(t, `"拉起支付失败"`, string(response.Data))

	var order model.SubscriptionOrder
	require.NoError(t, db.Where("user_id = ?", 795).First(&order).Error)
	require.Equal(t, common.TopUpStatusPending, order.Status)
}
