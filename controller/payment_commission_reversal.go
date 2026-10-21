package controller

import (
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/model"
	"github.com/stripe/stripe-go/v81"
	"github.com/waffo-com/waffo-go/core"
)

func paymentMinorAmountString(amount int64) (string, error) {
	if amount <= 0 {
		return "", model.ErrCashPaymentInvalid
	}
	return fmt.Sprintf("%d.%02d", amount/100, amount%100), nil
}

func creemPaymentCommissionReversalInput(event *CreemWebhookEvent, resolvedTradeNo string) (model.PaymentCommissionReversalInput, bool, error) {
	if event == nil {
		return model.PaymentCommissionReversalInput{}, false, model.ErrCashPaymentInvalid
	}
	tradeNo := strings.TrimSpace(event.Object.Checkout.RequestId)
	if tradeNo == "" {
		tradeNo = strings.TrimSpace(event.Object.RequestId)
	}
	if tradeNo == "" {
		tradeNo = strings.TrimSpace(resolvedTradeNo)
	}
	input := model.PaymentCommissionReversalInput{
		TradeNo: tradeNo, ExpectedProvider: model.PaymentProviderCreem,
		RefundSourceID: strings.TrimSpace(event.Object.Id),
	}
	amountMinor := int64(0)
	switch event.EventType {
	case "refund.created":
		if !strings.EqualFold(strings.TrimSpace(event.Object.Status), "succeeded") {
			return input, false, nil
		}
		input.RefundSourceType = "creem_refund"
		amountMinor = event.Object.RefundAmount
		input.RefundedCurrency = strings.ToUpper(strings.TrimSpace(event.Object.RefundCurrency))
	case "dispute.created":
		input.RefundSourceType = "creem_dispute"
		amountMinor = event.Object.Amount
		input.RefundedCurrency = strings.ToUpper(strings.TrimSpace(event.Object.Currency))
	default:
		return input, false, nil
	}
	if input.RefundSourceID == "" {
		input.RefundSourceID = strings.TrimSpace(event.Id)
	}
	amount, err := paymentMinorAmountString(amountMinor)
	if err != nil || input.TradeNo == "" || input.RefundSourceID == "" || input.RefundedCurrency == "" {
		return model.PaymentCommissionReversalInput{}, false, model.ErrCashPaymentInvalid
	}
	input.RefundedAmount = amount
	return input, true, nil
}

func waffoPaymentCommissionReversalInput(result *core.RefundNotificationResult) (model.PaymentCommissionReversalInput, bool, error) {
	if result == nil {
		return model.PaymentCommissionReversalInput{}, false, model.ErrCashPaymentInvalid
	}
	switch result.RefundStatus {
	case core.RefundStatusPartiallyRefunded, core.RefundStatusFullyRefunded:
	case core.RefundStatusInProgress, core.RefundStatusFailed:
		return model.PaymentCommissionReversalInput{}, false, nil
	default:
		return model.PaymentCommissionReversalInput{}, false, model.ErrCashPaymentInvalid
	}
	sourceID := strings.TrimSpace(result.RefundRequestID)
	if sourceID == "" {
		sourceID = strings.TrimSpace(result.AcquiringRefundOrderID)
	}
	if sourceID == "" {
		sourceID = strings.TrimSpace(result.MerchantRefundOrderID)
	}
	input := model.PaymentCommissionReversalInput{
		TradeNo: strings.TrimSpace(result.OrigPaymentRequestID), ExpectedProvider: model.PaymentProviderWaffo,
		RefundSourceType: "waffo_refund", RefundSourceID: sourceID,
		RefundedAmount: strings.TrimSpace(result.RefundAmount), RefundedCurrency: strings.ToUpper(strings.TrimSpace(result.UserCurrency)),
	}
	if input.TradeNo == "" || input.RefundSourceID == "" || input.RefundedAmount == "" {
		return model.PaymentCommissionReversalInput{}, false, model.ErrCashPaymentInvalid
	}
	return input, true, nil
}

func stripeChargeRefundCommissionReversalInputs(charge *stripe.Charge, tradeNo string) ([]model.PaymentCommissionReversalInput, error) {
	if charge == nil || strings.TrimSpace(tradeNo) == "" || charge.Refunds == nil {
		return nil, model.ErrCashPaymentInvalid
	}
	inputs := make([]model.PaymentCommissionReversalInput, 0, len(charge.Refunds.Data))
	for _, refund := range charge.Refunds.Data {
		if refund == nil || refund.Status != stripe.RefundStatusSucceeded {
			continue
		}
		amount, err := paymentMinorAmountString(refund.Amount)
		if err != nil || strings.TrimSpace(refund.ID) == "" {
			return nil, model.ErrCashPaymentInvalid
		}
		currency := strings.ToUpper(string(refund.Currency))
		if currency == "" {
			currency = strings.ToUpper(string(charge.Currency))
		}
		inputs = append(inputs, model.PaymentCommissionReversalInput{
			TradeNo: strings.TrimSpace(tradeNo), ExpectedProvider: model.PaymentProviderStripe,
			RefundSourceType: "stripe_refund", RefundSourceID: refund.ID,
			RefundedAmount: amount, RefundedCurrency: currency,
		})
	}
	return inputs, nil
}

func stripeDisputeCommissionReversalInput(dispute *stripe.Dispute, tradeNo string) (model.PaymentCommissionReversalInput, error) {
	if dispute == nil || strings.TrimSpace(dispute.ID) == "" || strings.TrimSpace(tradeNo) == "" {
		return model.PaymentCommissionReversalInput{}, model.ErrCashPaymentInvalid
	}
	amount, err := paymentMinorAmountString(dispute.Amount)
	if err != nil {
		return model.PaymentCommissionReversalInput{}, err
	}
	return model.PaymentCommissionReversalInput{
		TradeNo: strings.TrimSpace(tradeNo), ExpectedProvider: model.PaymentProviderStripe,
		RefundSourceType: "stripe_dispute", RefundSourceID: dispute.ID,
		RefundedAmount: amount, RefundedCurrency: strings.ToUpper(string(dispute.Currency)),
	}, nil
}

func resolvePaymentTradeNo(provider, directTradeNo string, references ...model.PaymentProviderReferenceInput) (string, error) {
	if tradeNo := strings.TrimSpace(directTradeNo); tradeNo != "" {
		return tradeNo, nil
	}
	for _, reference := range references {
		if strings.TrimSpace(reference.ExternalID) == "" {
			continue
		}
		tradeNo, found, err := model.FindPaymentTradeNoByProviderReference(provider, reference.ReferenceType, reference.ExternalID)
		if err != nil {
			return "", err
		}
		if found {
			return tradeNo, nil
		}
	}
	return "", errors.New("payment provider reference is not mapped")
}
