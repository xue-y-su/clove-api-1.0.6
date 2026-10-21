package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
	"github.com/stripe/stripe-go/v81"
	"github.com/waffo-com/waffo-go/core"
)

func TestCreemPaymentCommissionReversalInput(t *testing.T) {
	event := &CreemWebhookEvent{Id: "evt_refund", EventType: "refund.created"}
	event.Object.Id = "ref_creem"
	event.Object.Status = "succeeded"
	event.Object.RefundAmount = 1210
	event.Object.RefundCurrency = "eur"
	event.Object.Checkout.RequestId = "creem-trade-no"

	input, process, err := creemPaymentCommissionReversalInput(event, "")
	require.NoError(t, err)
	require.True(t, process)
	require.Equal(t, model.PaymentCommissionReversalInput{
		TradeNo: "creem-trade-no", ExpectedProvider: model.PaymentProviderCreem,
		RefundSourceType: "creem_refund", RefundSourceID: "ref_creem",
		RefundedAmount: "12.10", RefundedCurrency: "EUR",
	}, input)

	event.Object.Status = "pending"
	_, process, err = creemPaymentCommissionReversalInput(event, "")
	require.NoError(t, err)
	require.False(t, process)
}

func TestCreemDisputeCommissionReversalInputUsesResolvedTrade(t *testing.T) {
	event := &CreemWebhookEvent{Id: "evt_dispute", EventType: "dispute.created"}
	event.Object.Id = "disp_creem"
	event.Object.Amount = 1331
	event.Object.Currency = "eur"

	input, process, err := creemPaymentCommissionReversalInput(event, "resolved-trade")
	require.NoError(t, err)
	require.True(t, process)
	require.Equal(t, "resolved-trade", input.TradeNo)
	require.Equal(t, "creem_dispute", input.RefundSourceType)
	require.Equal(t, "13.31", input.RefundedAmount)
}

func TestWaffoPaymentCommissionReversalInput(t *testing.T) {
	result := &core.RefundNotificationResult{
		OrigPaymentRequestID: "waffo-trade", RefundRequestID: "refund-request",
		RefundAmount: "5.25", UserCurrency: "usd", RefundStatus: core.RefundStatusPartiallyRefunded,
	}
	input, process, err := waffoPaymentCommissionReversalInput(result)
	require.NoError(t, err)
	require.True(t, process)
	require.Equal(t, model.PaymentCommissionReversalInput{
		TradeNo: "waffo-trade", ExpectedProvider: model.PaymentProviderWaffo,
		RefundSourceType: "waffo_refund", RefundSourceID: "refund-request",
		RefundedAmount: "5.25", RefundedCurrency: "USD",
	}, input)
}

func TestStripePaymentCommissionReversalInputsUseIndividualRefunds(t *testing.T) {
	charge := &stripe.Charge{
		ID: "ch_123", Currency: stripe.CurrencyUSD,
		Refunds: &stripe.RefundList{Data: []*stripe.Refund{
			{ID: "re_1", Amount: 250, Currency: stripe.CurrencyUSD, Status: stripe.RefundStatusSucceeded},
			{ID: "re_pending", Amount: 100, Currency: stripe.CurrencyUSD, Status: stripe.RefundStatusPending},
		}},
	}
	inputs, err := stripeChargeRefundCommissionReversalInputs(charge, "stripe-trade")
	require.NoError(t, err)
	require.Equal(t, []model.PaymentCommissionReversalInput{{
		TradeNo: "stripe-trade", ExpectedProvider: model.PaymentProviderStripe,
		RefundSourceType: "stripe_refund", RefundSourceID: "re_1",
		RefundedAmount: "2.50", RefundedCurrency: "USD",
	}}, inputs)

	dispute := &stripe.Dispute{ID: "dp_1", Amount: 725, Currency: stripe.CurrencyUSD}
	input, err := stripeDisputeCommissionReversalInput(dispute, "stripe-trade")
	require.NoError(t, err)
	require.Equal(t, "stripe_dispute", input.RefundSourceType)
	require.Equal(t, "7.25", input.RefundedAmount)
}
