package controller

import (
	"errors"
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/stripe/stripe-go/v81"
)

type paymentRequestBlockingTransport struct{}

func (paymentRequestBlockingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("unexpected outbound payment request")
}

func TestWalletPaymentRequestsRejectMissingUser(t *testing.T) {
	confirmPaymentComplianceForTest(t)
	setupEpayTopupIntegrationDB(t)
	previousTransport := http.DefaultTransport
	blocker := paymentRequestBlockingTransport{}
	http.DefaultTransport = blocker
	stripe.SetHTTPClient(&http.Client{Transport: blocker})
	t.Cleanup(func() {
		http.DefaultTransport = previousTransport
		stripe.SetHTTPClient(http.DefaultClient)
	})

	previousStripeAPISecret := setting.StripeApiSecret
	previousStripeWebhookSecret := setting.StripeWebhookSecret
	previousStripePriceID := setting.StripePriceId
	previousStripeMinTopUp := setting.StripeMinTopUp
	previousCreemAPIKey := setting.CreemApiKey
	previousCreemProducts := setting.CreemProducts
	previousCreemWebhookSecret := setting.CreemWebhookSecret
	t.Cleanup(func() {
		setting.StripeApiSecret = previousStripeAPISecret
		setting.StripeWebhookSecret = previousStripeWebhookSecret
		setting.StripePriceId = previousStripePriceID
		setting.StripeMinTopUp = previousStripeMinTopUp
		setting.CreemApiKey = previousCreemAPIKey
		setting.CreemProducts = previousCreemProducts
		setting.CreemWebhookSecret = previousCreemWebhookSecret
	})
	setting.StripeApiSecret = "sk_test_local"
	setting.StripeWebhookSecret = "whsec_local"
	setting.StripePriceId = "price_local"
	setting.StripeMinTopUp = 1
	setting.CreemApiKey = "creem_local"
	setting.CreemProducts = `[{"productId":"product","name":"Product","price":1,"currency":"USD","quota":500000}]`
	setting.CreemWebhookSecret = "creem_local"

	testCases := []struct {
		name    string
		path    string
		body    any
		handler gin.HandlerFunc
	}{
		{
			name:    "Stripe",
			path:    "/api/user/stripe/pay",
			body:    StripePayRequest{Amount: 1, PaymentMethod: model.PaymentMethodStripe},
			handler: RequestStripePay,
		},
		{
			name:    "Creem",
			path:    "/api/user/creem/pay",
			body:    CreemPayRequest{ProductId: "product", PaymentMethod: model.PaymentMethodCreem},
			handler: RequestCreemPay,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			context, recorder := newAuthenticatedContext(t, http.MethodPost, testCase.path, testCase.body, 781)

			require.NotPanics(t, func() {
				testCase.handler(context)
			})

			require.Equal(t, http.StatusOK, recorder.Code)
			response := decodeAPIResponse(t, recorder)
			require.Equal(t, "error", response.Message)
			require.JSONEq(t, `"用户不存在"`, string(response.Data))
		})
	}
}
