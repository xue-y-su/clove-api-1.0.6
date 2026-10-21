package controller

import (
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/stretchr/testify/require"
)

func TestCreemCheckoutFailureKeepsWalletOrderPendingForLateWebhook(t *testing.T) {
	confirmPaymentComplianceForTest(t)
	db := setupEpayTopupIntegrationDB(t)
	seedTokenOwner(t, db, 782, "creem-checkout-user", "creem-checkout@example.com")

	previousAPIKey := setting.CreemApiKey
	previousProducts := setting.CreemProducts
	previousWebhookSecret := setting.CreemWebhookSecret
	previousTestMode := setting.CreemTestMode
	previousTransport := http.DefaultTransport
	t.Cleanup(func() {
		setting.CreemApiKey = previousAPIKey
		setting.CreemProducts = previousProducts
		setting.CreemWebhookSecret = previousWebhookSecret
		setting.CreemTestMode = previousTestMode
		http.DefaultTransport = previousTransport
	})
	setting.CreemApiKey = "creem_local"
	setting.CreemProducts = `[{"productId":"product","name":"Product","price":1,"currency":"USD","quota":500000}]`
	setting.CreemWebhookSecret = "creem_local"
	setting.CreemTestMode = false
	http.DefaultTransport = paymentRequestBlockingTransport{}

	context, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/user/creem/pay", CreemPayRequest{
		ProductId:     "product",
		PaymentMethod: model.PaymentMethodCreem,
	}, 782)
	RequestCreemPay(context)

	require.Equal(t, http.StatusOK, recorder.Code)
	response := decodeAPIResponse(t, recorder)
	require.Equal(t, "error", response.Message)
	require.JSONEq(t, `"拉起支付失败"`, string(response.Data))

	var topUp model.TopUp
	require.NoError(t, db.Where("user_id = ?", 782).First(&topUp).Error)
	require.Equal(t, common.TopUpStatusPending, topUp.Status)
}

func TestCreemCheckoutFailureKeepsSubscriptionOrderPendingForLateWebhook(t *testing.T) {
	confirmPaymentComplianceForTest(t)
	db := setupEpayTopupIntegrationDB(t)
	seedTokenOwner(t, db, 783, "creem-subscription-user", "creem-subscription@example.com")
	plan := &model.SubscriptionPlan{
		Title:          "Creem plan",
		PriceAmount:    1,
		Enabled:        true,
		CreemProductId: "product",
	}
	require.NoError(t, db.Create(plan).Error)
	model.InvalidateSubscriptionPlanCache(plan.Id)

	previousAPIKey := setting.CreemApiKey
	previousWebhookSecret := setting.CreemWebhookSecret
	previousTestMode := setting.CreemTestMode
	previousTransport := http.DefaultTransport
	t.Cleanup(func() {
		setting.CreemApiKey = previousAPIKey
		setting.CreemWebhookSecret = previousWebhookSecret
		setting.CreemTestMode = previousTestMode
		http.DefaultTransport = previousTransport
	})
	setting.CreemApiKey = "creem_local"
	setting.CreemWebhookSecret = "creem_local"
	setting.CreemTestMode = false
	http.DefaultTransport = paymentRequestBlockingTransport{}

	context, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/subscription/creem/pay", SubscriptionCreemPayRequest{PlanId: plan.Id}, 783)
	SubscriptionRequestCreemPay(context)

	require.Equal(t, http.StatusOK, recorder.Code)
	response := decodeAPIResponse(t, recorder)
	require.Equal(t, "error", response.Message)
	require.JSONEq(t, `"拉起支付失败"`, string(response.Data))

	var order model.SubscriptionOrder
	require.NoError(t, db.Where("user_id = ?", 783).First(&order).Error)
	require.Equal(t, common.TopUpStatusPending, order.Status)
}
