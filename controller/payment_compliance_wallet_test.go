package controller

import (
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/common"
	appi18n "github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestWalletPaymentRequestsRequirePaymentCompliance(t *testing.T) {
	require.NoError(t, appi18n.Init())
	db := setupEpayTopupIntegrationDB(t)
	seedTokenOwner(t, db, 779, "payment-compliance-user", "payment-compliance@example.com")

	paymentSetting := operation_setting.GetPaymentSetting()
	previousConfirmed := paymentSetting.ComplianceConfirmed
	previousTermsVersion := paymentSetting.ComplianceTermsVersion
	previousStripeAPISecret := setting.StripeApiSecret
	previousCreemProducts := setting.CreemProducts
	previousWaffoEnabled := setting.WaffoEnabled
	t.Cleanup(func() {
		paymentSetting.ComplianceConfirmed = previousConfirmed
		paymentSetting.ComplianceTermsVersion = previousTermsVersion
		setting.StripeApiSecret = previousStripeAPISecret
		setting.CreemProducts = previousCreemProducts
		setting.WaffoEnabled = previousWaffoEnabled
	})
	paymentSetting.ComplianceConfirmed = false
	paymentSetting.ComplianceTermsVersion = operation_setting.CurrentComplianceTermsVersion
	setting.StripeApiSecret = ""
	setting.CreemProducts = "[]"
	setting.WaffoEnabled = false

	testCases := []struct {
		name    string
		path    string
		body    any
		handler gin.HandlerFunc
	}{
		{
			name:    "ePay",
			path:    "/api/user/pay",
			body:    EpayRequest{Amount: 1, PaymentMethod: "alipay"},
			handler: RequestEpay,
		},
		{
			name:    "Stripe",
			path:    "/api/user/stripe/pay",
			body:    StripePayRequest{Amount: 1, PaymentMethod: "stripe"},
			handler: RequestStripePay,
		},
		{
			name:    "Creem",
			path:    "/api/user/creem/pay",
			body:    CreemPayRequest{ProductId: "product", PaymentMethod: "creem"},
			handler: RequestCreemPay,
		},
		{
			name:    "Waffo",
			path:    "/api/user/waffo/pay",
			body:    WaffoPayRequest{Amount: 1},
			handler: RequestWaffoPay,
		},
		{
			name:    "Waffo Pancake",
			path:    "/api/user/waffo-pancake/pay",
			body:    WaffoPancakePayRequest{Amount: 1},
			handler: RequestWaffoPancakePay,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			context, recorder := newAuthenticatedContext(t, http.MethodPost, testCase.path, testCase.body, 779)
			context.Request.Header.Set("Accept-Language", appi18n.LangEn)
			expectedMessage := common.TranslateMessage(context, appi18n.MsgPaymentComplianceRequired)

			testCase.handler(context)

			require.Equal(t, http.StatusOK, recorder.Code)
			response := decodeAPIResponse(t, recorder)
			require.False(t, response.Success)
			require.Equal(t, expectedMessage, response.Message)
		})
	}
}
