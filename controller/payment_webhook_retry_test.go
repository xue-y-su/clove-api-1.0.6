package controller

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func registerPaymentQueryFailure(t *testing.T, db *gorm.DB, callbackName string, modelNames ...string) error {
	t.Helper()
	failure := errors.New("forced payment database outage")
	wanted := make(map[string]struct{}, len(modelNames))
	for _, name := range modelNames {
		wanted[name] = struct{}{}
	}
	require.NoError(t, db.Callback().Query().Before("gorm:query").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement == nil || tx.Statement.Schema == nil {
			return
		}
		if _, ok := wanted[tx.Statement.Schema.Name]; ok {
			tx.AddError(failure)
		}
	}))
	t.Cleanup(func() { _ = db.Callback().Query().Remove(callbackName) })
	return failure
}

func TestPaymentOrderQueriesAndTransactionsPreserveDatabaseFailures(t *testing.T) {
	db := setupEpayTopupIntegrationDB(t)
	failure := registerPaymentQueryFailure(t, db, "payment-retry:model-query-failure", "TopUp", "SubscriptionOrder")

	_, err := model.FindTopUpByTradeNo("db-outage-topup")
	require.ErrorIs(t, err, failure)
	_, err = model.FindSubscriptionOrderByTradeNo("db-outage-subscription")
	require.ErrorIs(t, err, failure)
	require.ErrorIs(t, model.UpdatePendingTopUpStatus("db-outage-topup", model.PaymentProviderStripe, "failed"), failure)
	require.ErrorIs(t, model.CompleteSubscriptionOrder("db-outage-subscription", "", model.PaymentProviderStripe, ""), failure)
}

func TestStripeWebhookReturnsRetryableServerErrorOnOrderQueryFailure(t *testing.T) {
	db := setupEpayTopupIntegrationDB(t)
	registerPaymentQueryFailure(t, db, "payment-retry:stripe-topup-query", "TopUp")

	originalSecret := setting.StripeWebhookSecret
	t.Cleanup(func() { setting.StripeWebhookSecret = originalSecret })
	setting.StripeWebhookSecret = "whsec_retry_test"

	payload := `{"id":"evt_db_retry","type":"checkout.session.completed","data":{"object":{"client_reference_id":"stripe-db-outage","status":"complete","payment_status":"paid","amount_total":100,"currency":"usd"}}}`
	timestamp := time.Now().Unix()
	signedPayload := fmt.Sprintf("%d.%s", timestamp, payload)
	mac := hmac.New(sha256.New, []byte(setting.StripeWebhookSecret))
	_, _ = mac.Write([]byte(signedPayload))
	signature := fmt.Sprintf("t=%d,v1=%s", timestamp, hex.EncodeToString(mac.Sum(nil)))

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/stripe/webhook", StripeWebhook)
	request := httptest.NewRequest(http.MethodPost, "/api/stripe/webhook", strings.NewReader(payload))
	request.Header.Set("Stripe-Signature", signature)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusInternalServerError, recorder.Code)
}

func TestCreemWebhookReturnsRetryableServerErrorOnOrderQueryFailure(t *testing.T) {
	db := setupEpayTopupIntegrationDB(t)
	registerPaymentQueryFailure(t, db, "payment-retry:creem-topup-query", "TopUp")

	originalSecret := setting.CreemWebhookSecret
	t.Cleanup(func() { setting.CreemWebhookSecret = originalSecret })
	setting.CreemWebhookSecret = "creem_retry_test"

	payload := `{"id":"evt_db_retry","eventType":"checkout.completed","object":{"request_id":"creem-db-outage","order":{"id":"order_db_retry","status":"paid","type":"onetime","amount_paid":100,"currency":"USD"}}}`
	signature := generateCreemSignature(payload, setting.CreemWebhookSecret)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/creem/webhook", CreemWebhook)
	request := httptest.NewRequest(http.MethodPost, "/api/creem/webhook", strings.NewReader(payload))
	request.Header.Set(CreemSignatureHeader, signature)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusInternalServerError, recorder.Code)
}
