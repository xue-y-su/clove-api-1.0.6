package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/require"
	waffoutils "github.com/waffo-com/waffo-go/utils"
)

func confirmPaymentComplianceForTest(t *testing.T) {
	t.Helper()
	paymentSetting := operation_setting.GetPaymentSetting()
	originalConfirmed := paymentSetting.ComplianceConfirmed
	originalTermsVersion := paymentSetting.ComplianceTermsVersion
	t.Cleanup(func() {
		paymentSetting.ComplianceConfirmed = originalConfirmed
		paymentSetting.ComplianceTermsVersion = originalTermsVersion
	})
	paymentSetting.ComplianceConfirmed = true
	paymentSetting.ComplianceTermsVersion = operation_setting.CurrentComplianceTermsVersion
}

func TestStripeWebhookEnabledOnlyRequiresSigningSecret(t *testing.T) {
	confirmPaymentComplianceForTest(t)
	originalAPISecret := setting.StripeApiSecret
	originalWebhookSecret := setting.StripeWebhookSecret
	originalPriceID := setting.StripePriceId
	t.Cleanup(func() {
		setting.StripeApiSecret = originalAPISecret
		setting.StripeWebhookSecret = originalWebhookSecret
		setting.StripePriceId = originalPriceID
	})

	setting.StripeWebhookSecret = ""
	setting.StripeApiSecret = "sk_test_123"
	setting.StripePriceId = "price_123"
	require.False(t, isStripeWebhookEnabled())

	setting.StripeWebhookSecret = "whsec_test"
	require.True(t, isStripeWebhookEnabled())

	setting.StripeApiSecret = ""
	setting.StripePriceId = ""
	operation_setting.GetPaymentSetting().ComplianceConfirmed = false
	require.True(t, isStripeWebhookEnabled())
}

func TestCreemWebhookEnabledOnlyRequiresSigningSecret(t *testing.T) {
	confirmPaymentComplianceForTest(t)
	originalAPIKey := setting.CreemApiKey
	originalProducts := setting.CreemProducts
	originalWebhookSecret := setting.CreemWebhookSecret
	t.Cleanup(func() {
		setting.CreemApiKey = originalAPIKey
		setting.CreemProducts = originalProducts
		setting.CreemWebhookSecret = originalWebhookSecret
	})

	setting.CreemWebhookSecret = ""
	setting.CreemApiKey = "creem_api_key"
	setting.CreemProducts = `[{"productId":"prod_123"}]`
	require.False(t, isCreemWebhookEnabled())

	setting.CreemWebhookSecret = "creem_secret"
	require.True(t, isCreemWebhookEnabled())

	setting.CreemApiKey = ""
	setting.CreemProducts = "[]"
	operation_setting.GetPaymentSetting().ComplianceConfirmed = false
	require.True(t, isCreemWebhookEnabled())
}

func TestWaffoWebhookEnabledOnlyRequiresCurrentVerificationCredentials(t *testing.T) {
	confirmPaymentComplianceForTest(t)
	originalEnabled := setting.WaffoEnabled
	originalSandbox := setting.WaffoSandbox
	originalAPIKey := setting.WaffoApiKey
	originalPrivateKey := setting.WaffoPrivateKey
	originalPublicCert := setting.WaffoPublicCert
	originalSandboxAPIKey := setting.WaffoSandboxApiKey
	originalSandboxPrivateKey := setting.WaffoSandboxPrivateKey
	originalSandboxPublicCert := setting.WaffoSandboxPublicCert
	t.Cleanup(func() {
		setting.WaffoEnabled = originalEnabled
		setting.WaffoSandbox = originalSandbox
		setting.WaffoApiKey = originalAPIKey
		setting.WaffoPrivateKey = originalPrivateKey
		setting.WaffoPublicCert = originalPublicCert
		setting.WaffoSandboxApiKey = originalSandboxAPIKey
		setting.WaffoSandboxPrivateKey = originalSandboxPrivateKey
		setting.WaffoSandboxPublicCert = originalSandboxPublicCert
	})

	setting.WaffoEnabled = true
	setting.WaffoSandbox = false
	setting.WaffoApiKey = ""
	setting.WaffoPrivateKey = "private"
	setting.WaffoPublicCert = "public"
	require.True(t, isWaffoWebhookEnabled())
	require.False(t, isWaffoTopUpEnabled())

	setting.WaffoApiKey = "api"
	require.True(t, isWaffoWebhookEnabled())
	require.True(t, isWaffoTopUpEnabled())

	setting.WaffoApiKey = ""
	setting.WaffoEnabled = false
	operation_setting.GetPaymentSetting().ComplianceConfirmed = false
	require.True(t, isWaffoWebhookEnabled())
	require.False(t, isWaffoTopUpEnabled())

	setting.WaffoEnabled = true
	setting.WaffoSandbox = true
	setting.WaffoSandboxApiKey = ""
	setting.WaffoSandboxPrivateKey = "sandbox_private"
	setting.WaffoSandboxPublicCert = "sandbox_public"
	require.True(t, isWaffoWebhookEnabled())
	require.False(t, isWaffoTopUpEnabled())

	setting.WaffoSandboxApiKey = "sandbox_api"
	operation_setting.GetPaymentSetting().ComplianceConfirmed = true
	require.True(t, isWaffoWebhookEnabled())
	require.True(t, isWaffoTopUpEnabled())
}

func TestWaffoPancakeWebhookUsesBundledSigningKeys(t *testing.T) {
	confirmPaymentComplianceForTest(t)
	originalMerchantID := setting.WaffoPancakeMerchantID
	originalPrivateKey := setting.WaffoPancakePrivateKey
	originalStoreID := setting.WaffoPancakeStoreID
	originalProductID := setting.WaffoPancakeProductID
	t.Cleanup(func() {
		setting.WaffoPancakeMerchantID = originalMerchantID
		setting.WaffoPancakePrivateKey = originalPrivateKey
		setting.WaffoPancakeStoreID = originalStoreID
		setting.WaffoPancakeProductID = originalProductID
	})

	// Public verification keys are bundled in the SDK. Clearing checkout-only
	// credentials must not reject callbacks for already-created orders.
	setting.WaffoPancakeMerchantID = ""
	setting.WaffoPancakePrivateKey = ""
	setting.WaffoPancakeStoreID = ""
	setting.WaffoPancakeProductID = ""
	operation_setting.GetPaymentSetting().ComplianceConfirmed = false
	require.True(t, isWaffoPancakeWebhookEnabled())
}

func TestWaffoWebhookHandlerDoesNotRequireCheckoutAPIKey(t *testing.T) {
	keyPair, err := waffoutils.GenerateKeyPair()
	require.NoError(t, err)

	originalSandbox := setting.WaffoSandbox
	originalAPIKey := setting.WaffoSandboxApiKey
	originalPrivateKey := setting.WaffoSandboxPrivateKey
	originalPublicCert := setting.WaffoSandboxPublicCert
	t.Cleanup(func() {
		setting.WaffoSandbox = originalSandbox
		setting.WaffoSandboxApiKey = originalAPIKey
		setting.WaffoSandboxPrivateKey = originalPrivateKey
		setting.WaffoSandboxPublicCert = originalPublicCert
	})
	setting.WaffoSandbox = true
	setting.WaffoSandboxApiKey = ""
	setting.WaffoSandboxPrivateKey = keyPair.PrivateKey
	setting.WaffoSandboxPublicCert = keyPair.PublicKey

	handler, err := getWaffoWebhookHandler()
	require.NoError(t, err)
	payload := `{"eventType":"PAYMENT_NOTIFICATION"}`
	signature, err := waffoutils.Sign(payload, keyPair.PrivateKey)
	require.NoError(t, err)
	require.True(t, handler.VerifySignature(payload, signature))
}

func TestEpayWebhookEnabledDoesNotRequireActivePayMethodsOrCompliance(t *testing.T) {
	confirmPaymentComplianceForTest(t)
	originalPayAddress := operation_setting.PayAddress
	originalEpayID := operation_setting.EpayId
	originalEpayKey := operation_setting.EpayKey
	originalPayMethods := operation_setting.PayMethods
	t.Cleanup(func() {
		operation_setting.PayAddress = originalPayAddress
		operation_setting.EpayId = originalEpayID
		operation_setting.EpayKey = originalEpayKey
		operation_setting.PayMethods = originalPayMethods
	})

	operation_setting.PayAddress = "https://pay.example.com"
	operation_setting.EpayId = "epay_id"
	operation_setting.EpayKey = ""
	operation_setting.PayMethods = []map[string]string{{"type": "alipay"}}
	require.False(t, isEpayWebhookEnabled())

	operation_setting.EpayKey = "epay_key"
	require.True(t, isEpayWebhookEnabled())

	operation_setting.PayMethods = nil
	operation_setting.PayAddress = ""
	operation_setting.GetPaymentSetting().ComplianceConfirmed = false
	require.True(t, isEpayWebhookEnabled())
	require.False(t, isEpayTopUpEnabled())
}
