package controller

import (
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestWalletPaymentRequestsRejectIncompleteGatewayConfigurationBeforeCreatingOrder(t *testing.T) {
	confirmPaymentComplianceForTest(t)
	db := setupEpayTopupIntegrationDB(t)
	seedTokenOwner(t, db, 780, "payment-gateway-user", "payment-gateway@example.com")

	previousStripeAPISecret := setting.StripeApiSecret
	previousStripeWebhookSecret := setting.StripeWebhookSecret
	previousStripePriceID := setting.StripePriceId
	previousStripeMinTopUp := setting.StripeMinTopUp
	previousCreemAPIKey := setting.CreemApiKey
	previousCreemProducts := setting.CreemProducts
	previousCreemWebhookSecret := setting.CreemWebhookSecret
	previousCreemTestMode := setting.CreemTestMode
	previousWaffoEnabled := setting.WaffoEnabled
	previousWaffoSandbox := setting.WaffoSandbox
	previousWaffoAPIKey := setting.WaffoApiKey
	previousWaffoPrivateKey := setting.WaffoPrivateKey
	previousWaffoPublicCert := setting.WaffoPublicCert
	previousWaffoMinTopUp := setting.WaffoMinTopUp
	t.Cleanup(func() {
		setting.StripeApiSecret = previousStripeAPISecret
		setting.StripeWebhookSecret = previousStripeWebhookSecret
		setting.StripePriceId = previousStripePriceID
		setting.StripeMinTopUp = previousStripeMinTopUp
		setting.CreemApiKey = previousCreemAPIKey
		setting.CreemProducts = previousCreemProducts
		setting.CreemWebhookSecret = previousCreemWebhookSecret
		setting.CreemTestMode = previousCreemTestMode
		setting.WaffoEnabled = previousWaffoEnabled
		setting.WaffoSandbox = previousWaffoSandbox
		setting.WaffoApiKey = previousWaffoAPIKey
		setting.WaffoPrivateKey = previousWaffoPrivateKey
		setting.WaffoPublicCert = previousWaffoPublicCert
		setting.WaffoMinTopUp = previousWaffoMinTopUp
	})

	testCases := []struct {
		name     string
		path     string
		body     any
		handler  func(*gin.Context)
		prepare  func()
		expected string
	}{
		{
			name:    "Stripe",
			path:    "/api/user/stripe/pay",
			body:    StripePayRequest{Amount: 1, PaymentMethod: model.PaymentMethodStripe},
			handler: RequestStripePay,
			prepare: func() {
				setting.StripeApiSecret = ""
				setting.StripeWebhookSecret = ""
				setting.StripePriceId = ""
				setting.StripeMinTopUp = 1
			},
			expected: "Stripe 支付未启用或配置不完整",
		},
		{
			name:    "Creem",
			path:    "/api/user/creem/pay",
			body:    CreemPayRequest{ProductId: "product", PaymentMethod: model.PaymentMethodCreem},
			handler: RequestCreemPay,
			prepare: func() {
				setting.CreemApiKey = ""
				setting.CreemProducts = `[{"productId":"product","name":"Product","price":1,"currency":"USD","quota":500000}]`
				setting.CreemWebhookSecret = ""
				setting.CreemTestMode = false
			},
			expected: "Creem 支付未启用或配置不完整",
		},
		{
			name:    "Waffo",
			path:    "/api/user/waffo/pay",
			body:    WaffoPayRequest{Amount: 1},
			handler: RequestWaffoPay,
			prepare: func() {
				setting.WaffoEnabled = true
				setting.WaffoSandbox = false
				setting.WaffoApiKey = ""
				setting.WaffoPrivateKey = ""
				setting.WaffoPublicCert = ""
				setting.WaffoMinTopUp = 1
			},
			expected: "Waffo 支付未启用或配置不完整",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			testCase.prepare()
			context, recorder := newAuthenticatedContext(t, http.MethodPost, testCase.path, testCase.body, 780)
			testCase.handler(context)

			require.Equal(t, http.StatusOK, recorder.Code)
			response := decodeAPIResponse(t, recorder)
			require.Equal(t, "error", response.Message)
			require.JSONEq(t, `"`+testCase.expected+`"`, string(response.Data))

			var orderCount int64
			require.NoError(t, model.DB.Model(&model.TopUp{}).Where("user_id = ?", 780).Count(&orderCount).Error)
			require.Zero(t, orderCount)
		})
	}
}
