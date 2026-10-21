package controller

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/require"
	"github.com/stripe/stripe-go/v81"
)

type stripeCheckoutCaptureTransport struct {
	form url.Values
}

func (transport *stripeCheckoutCaptureTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	body, err := io.ReadAll(request.Body)
	if err != nil {
		return nil, err
	}
	transport.form, err = url.ParseQuery(string(body))
	if err != nil {
		return nil, err
	}

	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body: io.NopCloser(strings.NewReader(
			`{"id":"cs_test_wallet","object":"checkout.session","url":"https://checkout.stripe.test/cs_test_wallet"}`,
		)),
		Request: request,
	}, nil
}

func useStripeAPIBackendForTest(t *testing.T, client *http.Client) {
	t.Helper()
	previousBackend := stripe.GetBackend(stripe.APIBackend)
	stripe.SetBackend(stripe.APIBackend, stripe.NewBackends(client).API)
	t.Cleanup(func() {
		stripe.SetBackend(stripe.APIBackend, previousBackend)
	})
}

func configureStripeWalletPaymentTest(t *testing.T) {
	t.Helper()
	confirmPaymentComplianceForTest(t)

	previousAPISecret := setting.StripeApiSecret
	previousWebhookSecret := setting.StripeWebhookSecret
	previousPriceID := setting.StripePriceId
	previousUnitPrice := setting.StripeUnitPrice
	previousMinTopUp := setting.StripeMinTopUp
	generalSetting := operation_setting.GetGeneralSetting()
	previousQuotaDisplayType := generalSetting.QuotaDisplayType
	previousTopupRatios := common.TopupGroupRatio2JSONString()
	t.Cleanup(func() {
		setting.StripeApiSecret = previousAPISecret
		setting.StripeWebhookSecret = previousWebhookSecret
		setting.StripePriceId = previousPriceID
		setting.StripeUnitPrice = previousUnitPrice
		setting.StripeMinTopUp = previousMinTopUp
		generalSetting.QuotaDisplayType = previousQuotaDisplayType
		require.NoError(t, common.UpdateTopupGroupRatioByJSONString(previousTopupRatios))
	})

	setting.StripeApiSecret = "sk_test_wallet"
	setting.StripeWebhookSecret = "whsec_test_wallet"
	setting.StripePriceId = "price_test_wallet"
	setting.StripeUnitPrice = 3.25
	setting.StripeMinTopUp = 1
	generalSetting.QuotaDisplayType = operation_setting.QuotaDisplayTypeUSD
	require.NoError(t, common.UpdateTopupGroupRatioByJSONString(`{"default":1}`))
}

func stripeWalletOverflowAmount(t *testing.T) int64 {
	t.Helper()
	require.Greater(t, common.QuotaPerUnit, 0.0)
	return int64(common.MaxQuota)/int64(common.QuotaPerUnit) + 1
}

func TestStripeAmountPreviewRejectsAmountThatExceedsWalletQuota(t *testing.T) {
	db := setupEpayTopupIntegrationDB(t)
	configureStripeWalletPaymentTest(t)
	seedTokenOwner(t, db, 791, "stripe-overflow-preview", "stripe-overflow-preview@example.com")

	context, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/user/stripe/amount", StripePayRequest{
		Amount: stripeWalletOverflowAmount(t),
	}, 791)
	RequestStripeAmount(context)

	require.Equal(t, http.StatusOK, recorder.Code)
	response := decodeAPIResponse(t, recorder)
	require.Equal(t, "error", response.Message)
	require.JSONEq(t, `"充值额度超出允许范围"`, string(response.Data))
}

func TestStripeAmountPreviewUsesItsFixedUnitPriceWithoutInternalDiscounts(t *testing.T) {
	db := setupEpayTopupIntegrationDB(t)
	configureStripeWalletPaymentTest(t)
	seedTokenOwner(t, db, 790, "stripe-fixed-price", "stripe-fixed-price@example.com")

	paymentSetting := operation_setting.GetPaymentSetting()
	previousDiscounts := paymentSetting.AmountDiscount
	t.Cleanup(func() {
		paymentSetting.AmountDiscount = previousDiscounts
	})
	paymentSetting.AmountDiscount = map[int]float64{2: 0.8}
	require.NoError(t, common.UpdateTopupGroupRatioByJSONString(`{"default":1.5}`))

	context, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/user/stripe/amount", StripePayRequest{
		Amount: 2,
	}, 790)
	RequestStripeAmount(context)

	require.Equal(t, http.StatusOK, recorder.Code)
	response := decodeAPIResponse(t, recorder)
	require.Equal(t, "success", response.Message)
	require.JSONEq(t, `"6.50"`, string(response.Data))
}

func TestStripePayRejectsAmountThatExceedsWalletQuotaBeforeCreatingOrder(t *testing.T) {
	db := setupEpayTopupIntegrationDB(t)
	configureStripeWalletPaymentTest(t)
	seedTokenOwner(t, db, 792, "stripe-overflow-pay", "stripe-overflow-pay@example.com")
	setting.StripeApiSecret = "not-a-stripe-secret"

	context, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/user/stripe/pay", StripePayRequest{
		Amount:        stripeWalletOverflowAmount(t),
		PaymentMethod: model.PaymentMethodStripe,
	}, 792)
	RequestStripePay(context)

	require.Equal(t, http.StatusOK, recorder.Code)
	response := decodeAPIResponse(t, recorder)
	require.Equal(t, "error", response.Message)
	require.JSONEq(t, `"充值额度超出允许范围"`, string(response.Data))

	var topUpCount int64
	require.NoError(t, db.Model(&model.TopUp{}).Where("user_id = ?", 792).Count(&topUpCount).Error)
	require.Zero(t, topUpCount)
}

func TestStripeCheckoutFailureKeepsWalletOrderPendingForLateWebhook(t *testing.T) {
	db := setupEpayTopupIntegrationDB(t)
	configureStripeWalletPaymentTest(t)
	seedTokenOwner(t, db, 793, "stripe-checkout-failure", "stripe-checkout-failure@example.com")

	previousStripeKey := stripe.Key
	useStripeAPIBackendForTest(t, &http.Client{Transport: paymentRequestBlockingTransport{}})
	t.Cleanup(func() {
		stripe.Key = previousStripeKey
	})

	context, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/user/stripe/pay", StripePayRequest{
		Amount:        1,
		PaymentMethod: model.PaymentMethodStripe,
	}, 793)
	RequestStripePay(context)

	require.Equal(t, http.StatusOK, recorder.Code)
	response := decodeAPIResponse(t, recorder)
	require.Equal(t, "error", response.Message)
	require.JSONEq(t, `"拉起支付失败"`, string(response.Data))

	var topUp model.TopUp
	require.NoError(t, db.Where("user_id = ?", 793).First(&topUp).Error)
	require.Equal(t, common.TopUpStatusPending, topUp.Status)
}

func TestStripePayNormalizesTokenAmountAndCreditsNormalizedQuota(t *testing.T) {
	db := setupEpayTopupIntegrationDB(t)
	configureStripeWalletPaymentTest(t)
	user := seedTokenOwner(t, db, 794, "stripe-token-display", "stripe-token-display@example.com")

	generalSetting := operation_setting.GetGeneralSetting()
	generalSetting.QuotaDisplayType = operation_setting.QuotaDisplayTypeTokens
	requestedTokens := int64(common.QuotaPerUnit) * 2
	require.Greater(t, requestedTokens, int64(0))

	previousStripeKey := stripe.Key
	transport := &stripeCheckoutCaptureTransport{}
	useStripeAPIBackendForTest(t, &http.Client{Transport: transport})
	t.Cleanup(func() {
		stripe.Key = previousStripeKey
	})

	context, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/user/stripe/pay", StripePayRequest{
		Amount:        requestedTokens,
		PaymentMethod: model.PaymentMethodStripe,
	}, user.Id)
	RequestStripePay(context)

	require.Equal(t, http.StatusOK, recorder.Code)
	response := decodeAPIResponse(t, recorder)
	require.Equal(t, "success", response.Message)
	require.Equal(t, "2", transport.form.Get("line_items[0][quantity]"))

	var topUp model.TopUp
	require.NoError(t, db.Where("user_id = ?", user.Id).First(&topUp).Error)
	require.Equal(t, int64(2), topUp.Amount)
	require.Equal(t, 6.5, topUp.Money)
	require.Equal(t, "6.50", topUp.PaidAmount)
	require.Equal(t, "USD", topUp.PaidCurrency)
	expectedEligibleQuota, err := model.CalculateCashPaymentEligibleQuota("6.50", "1")
	require.NoError(t, err)
	require.Equal(t, expectedEligibleQuota, topUp.EligibleQuota)
	require.Equal(t, common.TopUpStatusPending, topUp.Status)

	require.NoError(t, model.Recharge(topUp.TradeNo, "", "127.0.0.1"))
	var updatedUser model.User
	require.NoError(t, db.First(&updatedUser, user.Id).Error)
	expectedQuota, err := common.QuotaFromFloatStrict(2 * common.QuotaPerUnit)
	require.NoError(t, err)
	require.Equal(t, int64(expectedQuota), updatedUser.Quota)
}
