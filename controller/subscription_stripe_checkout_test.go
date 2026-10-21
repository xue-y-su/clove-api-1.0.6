package controller

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stripe/stripe-go/v81"
)

type stripeSubscriptionCheckoutTransport struct {
	recurring    bool
	checkoutForm url.Values
}

func (transport *stripeSubscriptionCheckoutTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	body := `{ "id":"price_one_time", "object":"price", "currency":"usd", "unit_amount":1250, "product":"prod_plan", "type":"one_time" }`
	if strings.HasPrefix(request.URL.Path, "/v1/prices/") && transport.recurring {
		body = `{ "id":"price_recurring", "object":"price", "currency":"usd", "unit_amount":1250, "product":"prod_plan", "type":"recurring", "recurring":{"interval":"month"} }`
	}
	if request.URL.Path == "/v1/checkout/sessions" {
		encoded, err := io.ReadAll(request.Body)
		if err != nil {
			return nil, err
		}
		transport.checkoutForm, err = url.ParseQuery(string(encoded))
		if err != nil {
			return nil, err
		}
		body = `{ "id":"cs_subscription", "object":"checkout.session", "url":"https://checkout.stripe.test/cs_subscription" }`
	}
	return &http.Response{
		StatusCode: http.StatusOK, Header: make(http.Header),
		Body: io.NopCloser(strings.NewReader(body)), Request: request,
	}, nil
}

func configureStripeSubscriptionCheckoutTest(t *testing.T, transport http.RoundTripper) {
	t.Helper()
	previousSecret := setting.StripeApiSecret
	previousKey := stripe.Key
	setting.StripeApiSecret = "sk_test_subscription"
	useStripeAPIBackendForTest(t, &http.Client{Transport: transport})
	t.Cleanup(func() {
		setting.StripeApiSecret = previousSecret
		stripe.Key = previousKey
	})
}

func TestStripeRecurringPlanCreatesOneTimeCheckoutFromSnapshot(t *testing.T) {
	transport := &stripeSubscriptionCheckoutTransport{recurring: true}
	configureStripeSubscriptionCheckoutTest(t, transport)

	link, err := genStripeSubscriptionLink("trade-recurring", "", "buyer@example.com", "price_recurring", "Snapshot plan", "USD", 12.5)
	require.NoError(t, err)
	assert.Equal(t, "https://checkout.stripe.test/cs_subscription", link)
	require.NotNil(t, transport.checkoutForm)
	assert.Equal(t, "payment", transport.checkoutForm.Get("mode"))
	assert.Equal(t, "usd", transport.checkoutForm.Get("line_items[0][price_data][currency]"))
	assert.Equal(t, "1250", transport.checkoutForm.Get("line_items[0][price_data][unit_amount]"))
	assert.Equal(t, "prod_plan", transport.checkoutForm.Get("line_items[0][price_data][product]"))
	assert.Empty(t, transport.checkoutForm.Get("line_items[0][price]"))
	assert.Equal(t, "trade-recurring", transport.checkoutForm.Get("payment_intent_data[metadata][trade_no]"))
	assert.Empty(t, transport.checkoutForm.Get("subscription_data[metadata][trade_no]"))
}

func TestStripeOneTimePlanKeepsConfiguredPriceInPaymentCheckout(t *testing.T) {
	transport := &stripeSubscriptionCheckoutTransport{}
	configureStripeSubscriptionCheckoutTest(t, transport)

	_, err := genStripeSubscriptionLink("trade-one-time", "cus_123", "", "price_one_time", "Snapshot plan", "USD", 12.5)
	require.NoError(t, err)
	require.NotNil(t, transport.checkoutForm)
	assert.Equal(t, "payment", transport.checkoutForm.Get("mode"))
	assert.Equal(t, "price_one_time", transport.checkoutForm.Get("line_items[0][price]"))
	assert.Empty(t, transport.checkoutForm.Get("line_items[0][price_data][unit_amount]"))
	assert.Equal(t, "cus_123", transport.checkoutForm.Get("customer"))
}
