package controller

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func signedWaffoPancakeWebhook(t *testing.T, privateKey *rsa.PrivateKey, body map[string]any) (string, string) {
	t.Helper()
	payload, err := json.Marshal(body)
	require.NoError(t, err)
	timestamp := strconv.FormatInt(time.Now().UnixMilli(), 10)
	digest := sha256.Sum256([]byte(timestamp + "." + string(payload)))
	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, digest[:])
	require.NoError(t, err)
	return string(payload), fmt.Sprintf("t=%s,v1=%s", timestamp, base64.StdEncoding.EncodeToString(signature))
}

func TestWaffoPancakeWebhookRejectsSignedEventsFromAnotherStoreWithoutWrites(t *testing.T) {
	db := openTokenControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(
		&model.User{},
		&model.TopUp{},
		&model.SubscriptionOrder{},
		&model.PaymentRefund{},
		&model.WalletLedgerEntry{},
	))

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
	setting.WaffoPancakeMerchantID = "MER_local"
	setting.WaffoPancakePrivateKey = "private"
	setting.WaffoPancakeStoreID = "STO_local"
	setting.WaffoPancakeProductID = "PRO_local"

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	publicDER, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	require.NoError(t, err)
	t.Setenv("WAFFO_WEBHOOK_PROD_PUBLIC_KEY", string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDER})))

	const userID = 97531
	require.NoError(t, db.Create(&model.User{
		Id: userID, Username: "pancake-store-guard", AffCode: "pancake-store-guard", Quota: 1000,
		Role: common.RoleCommonUser, Status: common.UserStatusEnabled,
	}).Error)
	const walletTrade = "WAFFO_PANCAKE-97531-wallet"
	const subscriptionTrade = "WAFFO_PANCAKE_SUB-97531-subscription"
	const refundTrade = "WAFFO_PANCAKE-97531-refund"
	require.NoError(t, db.Create(&[]model.TopUp{
		{UserId: userID, Amount: 100, Money: 1, TradeNo: walletTrade, PaymentMethod: model.PaymentMethodWaffoPancake, PaymentProvider: model.PaymentProviderWaffoPancake, Status: common.TopUpStatusPending},
		{UserId: userID, Amount: 100, Money: 1, TradeNo: refundTrade, PaymentMethod: model.PaymentMethodWaffoPancake, PaymentProvider: model.PaymentProviderWaffoPancake, Status: common.TopUpStatusSuccess, PaidAmount: "1.00", PaidCurrency: "USD"},
	}).Error)
	require.NoError(t, db.Create(&model.SubscriptionOrder{
		UserId: userID, PlanId: 1, TradeNo: subscriptionTrade, PaymentMethod: model.PaymentMethodWaffoPancake,
		PaymentProvider: model.PaymentProviderWaffoPancake, Status: common.TopUpStatusPending,
	}).Error)

	testCases := []struct {
		name      string
		eventType string
		tradeNo   string
		data      map[string]any
	}{
		{name: "wallet topup", eventType: "order.completed", tradeNo: walletTrade},
		{name: "subscription", eventType: "order.completed", tradeNo: subscriptionTrade},
		{name: "refund", eventType: "refund.succeeded", tradeNo: refundTrade, data: map[string]any{
			"refundTicketMerchantExternalId": "refund-wrong-store", "refundStatus": "succeeded",
		}},
	}

	gin.SetMode(gin.TestMode)
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			data := map[string]any{
				"orderId": "ORD_other", "orderMerchantExternalId": testCase.tradeNo,
				"merchantProvidedBuyerIdentity": fmt.Sprintf("new-api-user-%d", userID),
				"amount":                        "1.00", "currency": "USD",
			}
			for key, value := range testCase.data {
				data[key] = value
			}
			payload, signature := signedWaffoPancakeWebhook(t, privateKey, map[string]any{
				"id": "evt_" + strings.ReplaceAll(testCase.name, " ", "_"), "eventType": testCase.eventType,
				"eventId": "event_" + testCase.tradeNo, "storeId": "STO_other", "mode": "prod", "data": data,
			})
			router := gin.New()
			router.POST("/api/waffo-pancake/webhook/:env", WaffoPancakeWebhook)
			request := httptest.NewRequest(http.MethodPost, "/api/waffo-pancake/webhook/prod", strings.NewReader(payload))
			request.Header.Set("X-Waffo-Signature", signature)
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)
			require.Equal(t, http.StatusOK, recorder.Code)
			require.Equal(t, "OK", recorder.Body.String())

			var wallet model.TopUp
			require.NoError(t, db.Where("trade_no = ?", walletTrade).First(&wallet).Error)
			require.Equal(t, common.TopUpStatusPending, wallet.Status)
			var subscription model.SubscriptionOrder
			require.NoError(t, db.Where("trade_no = ?", subscriptionTrade).First(&subscription).Error)
			require.Equal(t, common.TopUpStatusPending, subscription.Status)
			var user model.User
			require.NoError(t, db.First(&user, userID).Error)
			require.EqualValues(t, 1000, user.Quota)
			var refundCount int64
			require.NoError(t, db.Model(&model.PaymentRefund{}).Count(&refundCount).Error)
			require.Zero(t, refundCount)
			var ledgerCount int64
			require.NoError(t, db.Model(&model.WalletLedgerEntry{}).Count(&ledgerCount).Error)
			require.Zero(t, ledgerCount)
		})
	}
}

func TestWaffoPancakeOrderStoreSnapshotSurvivesConfigurationRotation(t *testing.T) {
	db := openTokenControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.TopUp{}, &model.SubscriptionOrder{}))

	originalStoreID := setting.WaffoPancakeStoreID
	t.Cleanup(func() { setting.WaffoPancakeStoreID = originalStoreID })
	setting.WaffoPancakeStoreID = "STO_rotated"

	const userID = 97532
	wallet := &model.TopUp{
		UserId: userID, TradeNo: "WAFFO_PANCAKE-store-snapshot-wallet",
		PaymentProvider: model.PaymentProviderWaffoPancake, ProviderStoreID: "STO_original",
		Status: common.TopUpStatusPending,
	}
	subscription := &model.SubscriptionOrder{
		UserId: userID, PlanId: 1, TradeNo: "WAFFO_PANCAKE_SUB-store-snapshot-subscription",
		PaymentProvider: model.PaymentProviderWaffoPancake, ProviderStoreID: "STO_original",
		Status: common.TopUpStatusPending,
	}
	legacy := &model.TopUp{
		UserId: userID, TradeNo: "WAFFO_PANCAKE-store-snapshot-legacy",
		PaymentProvider: model.PaymentProviderWaffoPancake, Status: common.TopUpStatusPending,
	}
	require.NoError(t, db.Create(wallet).Error)
	require.NoError(t, db.Create(subscription).Error)
	require.NoError(t, db.Create(legacy).Error)

	walletEvent := &service.WaffoPancakeWebhookEvent{
		StoreID: "STO_original",
		Data: service.WaffoPancakeWebhookData{
			OrderMerchantExternalID:       wallet.TradeNo,
			MerchantProvidedBuyerIdentity: service.WaffoPancakeBuyerIdentityFromUserID(userID),
		},
	}
	tradeNo, err := service.ResolveWaffoPancakeTradeNo(walletEvent)
	require.NoError(t, err)
	require.Equal(t, wallet.TradeNo, tradeNo)

	subscriptionEvent := &service.WaffoPancakeWebhookEvent{
		StoreID: "STO_original",
		Data: service.WaffoPancakeWebhookData{
			OrderMerchantExternalID:       subscription.TradeNo,
			MerchantProvidedBuyerIdentity: service.WaffoPancakeBuyerIdentityFromUserID(userID),
		},
	}
	tradeNo, err = service.ResolveWaffoPancakeSubscriptionTradeNo(subscriptionEvent)
	require.NoError(t, err)
	require.Equal(t, subscription.TradeNo, tradeNo)

	legacyEvent := &service.WaffoPancakeWebhookEvent{
		StoreID: "STO_rotated",
		Data: service.WaffoPancakeWebhookData{
			OrderMerchantExternalID:       legacy.TradeNo,
			MerchantProvidedBuyerIdentity: service.WaffoPancakeBuyerIdentityFromUserID(userID),
		},
	}
	tradeNo, err = service.ResolveWaffoPancakeTradeNo(legacyEvent)
	require.NoError(t, err)
	require.Equal(t, legacy.TradeNo, tradeNo)

	walletEvent.StoreID = "STO_rotated"
	_, err = service.ResolveWaffoPancakeTradeNo(walletEvent)
	require.ErrorIs(t, err, service.ErrWaffoPancakeWebhookOrderInvalid)
}

func TestWaffoPancakeWebhookReturnsRetryableServerErrorOnOrderQueryFailure(t *testing.T) {
	db := openTokenControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.TopUp{}, &model.SubscriptionOrder{}))
	registerPaymentQueryFailure(t, db, "payment-retry:pancake-topup-query", "TopUp")

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	publicDER, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	require.NoError(t, err)
	t.Setenv("WAFFO_WEBHOOK_PROD_PUBLIC_KEY", string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDER})))

	payload, signature := signedWaffoPancakeWebhook(t, privateKey, map[string]any{
		"id": "evt_db_retry", "eventType": "order.completed", "eventId": "event_db_retry",
		"storeId": "STO_snapshot", "mode": "prod",
		"data": map[string]any{
			"orderId": "ORD_db_retry", "orderMerchantExternalId": "WAFFO_PANCAKE-db-retry",
			"merchantProvidedBuyerIdentity": "new-api-user-1", "amount": "1.00", "currency": "USD",
		},
	})
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/waffo-pancake/webhook/:env", WaffoPancakeWebhook)
	request := httptest.NewRequest(http.MethodPost, "/api/waffo-pancake/webhook/prod", strings.NewReader(payload))
	request.Header.Set("X-Waffo-Signature", signature)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusInternalServerError, recorder.Code)
	require.Equal(t, "retry", recorder.Body.String())
}
