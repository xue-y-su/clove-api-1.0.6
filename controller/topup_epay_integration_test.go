package controller

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/Calcium-Ion/go-epay/epay"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type epayPurchaseResponse struct {
	Message string            `json:"message"`
	Data    map[string]string `json:"data"`
	URL     string            `json:"url"`
}

func setupEpayTopupIntegrationDB(t *testing.T) *gorm.DB {
	t.Helper()

	previousDB := model.DB
	previousLogDB := model.LOG_DB
	previousMainDatabaseType := common.MainDatabaseType()
	previousLogDatabaseType := common.LogDatabaseType()
	previousRedisEnabled := common.RedisEnabled

	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	common.RedisEnabled = false

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)

	model.DB = db
	model.LOG_DB = db
	t.Cleanup(func() {
		model.DB = previousDB
		model.LOG_DB = previousLogDB
		common.SetDatabaseTypes(previousMainDatabaseType, previousLogDatabaseType)
		common.RedisEnabled = previousRedisEnabled
		_ = sqlDB.Close()
	})

	require.NoError(t, db.AutoMigrate(
		&model.User{},
		&model.TopUp{},
		&model.WalletLedgerEntry{},
		&model.AuditOutboxEvent{},
		&model.Log{},
		&model.IPSecurityEvent{},
		&model.SubscriptionPlan{},
		&model.SubscriptionOrder{},
		&model.UserSubscription{},
		&model.SubscriptionPurchaseCounter{},
	))
	return db
}

func configureEpayTopupIntegration(t *testing.T, gatewayURL string, callbackURL string) {
	t.Helper()

	confirmPaymentComplianceForTest(t)
	previousPayAddress := operation_setting.PayAddress
	previousCallbackAddress := operation_setting.CustomCallbackAddress
	previousEpayID := operation_setting.EpayId
	previousEpayKey := operation_setting.EpayKey
	previousPrice := operation_setting.Price
	previousMinTopup := operation_setting.MinTopUp
	previousPayMethods := operation_setting.PayMethods
	previousServerAddress := system_setting.ServerAddress
	generalSetting := operation_setting.GetGeneralSetting()
	previousQuotaDisplayType := generalSetting.QuotaDisplayType
	t.Cleanup(func() {
		operation_setting.PayAddress = previousPayAddress
		operation_setting.CustomCallbackAddress = previousCallbackAddress
		operation_setting.EpayId = previousEpayID
		operation_setting.EpayKey = previousEpayKey
		operation_setting.Price = previousPrice
		operation_setting.MinTopUp = previousMinTopup
		operation_setting.PayMethods = previousPayMethods
		system_setting.ServerAddress = previousServerAddress
		generalSetting.QuotaDisplayType = previousQuotaDisplayType
	})

	operation_setting.PayAddress = gatewayURL
	operation_setting.CustomCallbackAddress = callbackURL
	operation_setting.EpayId = "local-epay-partner"
	operation_setting.EpayKey = "local-epay-signing-key"
	operation_setting.Price = 1
	operation_setting.MinTopUp = 1
	operation_setting.PayMethods = []map[string]string{{"type": "alipay"}}
	system_setting.ServerAddress = callbackURL
	generalSetting.QuotaDisplayType = operation_setting.QuotaDisplayTypeUSD
}

func signedEpayCallbackParams(tradeNo string, paymentType string) map[string]string {
	params := map[string]string{
		"pid":          operation_setting.EpayId,
		"type":         paymentType,
		"trade_no":     "local-gateway-trade",
		"out_trade_no": tradeNo,
		"name":         "local topup",
		"money":        "1.00",
		"trade_status": epay.StatusTradeSuccess,
	}
	return epay.GenerateParams(params, operation_setting.EpayKey)
}

func valuesFromStringMap(params map[string]string) url.Values {
	values := make(url.Values, len(params))
	for key, value := range params {
		values.Set(key, value)
	}
	return values
}

func postEpayCallback(t *testing.T, callbackURL string, params map[string]string) string {
	t.Helper()

	response, err := http.PostForm(callbackURL, valuesFromStringMap(params))
	require.NoError(t, err)
	defer response.Body.Close()
	require.Equal(t, http.StatusOK, response.StatusCode)
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	return string(body)
}

func TestEpayLocalGatewayCreditsWalletAndDeduplicatesCallback(t *testing.T) {
	db := setupEpayTopupIntegrationDB(t)

	callbackRouter := gin.New()
	callbackRouter.POST("/api/user/epay/notify", EpayNotify)
	callbackServer := httptest.NewServer(callbackRouter)
	t.Cleanup(callbackServer.Close)

	callbackParams := make(chan map[string]string, 1)
	gatewayServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/submit.php" {
			http.NotFound(writer, request)
			return
		}
		if err := request.ParseForm(); err != nil {
			http.Error(writer, err.Error(), http.StatusBadRequest)
			return
		}
		if request.PostForm.Get("notify_url") != callbackServer.URL+"/api/user/epay/notify" {
			http.Error(writer, "unexpected notify URL", http.StatusBadRequest)
			return
		}
		callback := signedEpayCallbackParams(request.PostForm.Get("out_trade_no"), request.PostForm.Get("type"))
		response, err := http.PostForm(request.PostForm.Get("notify_url"), valuesFromStringMap(callback))
		if err != nil {
			http.Error(writer, err.Error(), http.StatusBadGateway)
			return
		}
		defer response.Body.Close()
		body, err := io.ReadAll(response.Body)
		if err != nil {
			http.Error(writer, err.Error(), http.StatusBadGateway)
			return
		}
		writer.Header().Set("X-Callback-Response", string(body))
		callbackParams <- callback
		writer.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(gatewayServer.Close)
	configureEpayTopupIntegration(t, gatewayServer.URL, callbackServer.URL)

	user := seedTokenOwner(t, db, 771, "epay-integration-user", "epay-integration@example.com")
	require.NoError(t, db.Model(user).Update("group", "default").Error)

	requestContext, requestRecorder := newAuthenticatedContext(t, http.MethodPost, "/api/user/pay", EpayRequest{
		Amount:        3,
		PaymentMethod: "alipay",
	}, user.Id)
	RequestEpay(requestContext)
	require.Equal(t, http.StatusOK, requestRecorder.Code)

	var purchase epayPurchaseResponse
	require.NoError(t, common.Unmarshal(requestRecorder.Body.Bytes(), &purchase))
	require.Equal(t, "success", purchase.Message)
	require.Equal(t, gatewayServer.URL+"/submit.php", purchase.URL)
	require.NotEmpty(t, purchase.Data["out_trade_no"])

	purchaseResponse, err := http.PostForm(purchase.URL, valuesFromStringMap(purchase.Data))
	require.NoError(t, err)
	defer purchaseResponse.Body.Close()
	require.Equal(t, http.StatusOK, purchaseResponse.StatusCode)
	require.Equal(t, "success", purchaseResponse.Header.Get("X-Callback-Response"))
	_, err = io.ReadAll(purchaseResponse.Body)
	require.NoError(t, err)

	callback := <-callbackParams
	var topUp model.TopUp
	require.NoError(t, db.Where("trade_no = ?", purchase.Data["out_trade_no"]).First(&topUp).Error)
	require.Equal(t, common.TopUpStatusSuccess, topUp.Status)
	require.Equal(t, "1.00", topUp.PaidAmount)
	require.Equal(t, "USD", topUp.PaidCurrency)
	settledEligibleQuota, err := model.CalculateCashPaymentEligibleQuota("1.00", "1")
	require.NoError(t, err)
	require.Equal(t, settledEligibleQuota, topUp.EligibleQuota)

	expectedQuota, err := common.QuotaFromFloatStrict(3 * common.QuotaPerUnit)
	require.NoError(t, err)
	var updatedUser model.User
	require.NoError(t, db.First(&updatedUser, user.Id).Error)
	require.Equal(t, int64(expectedQuota), updatedUser.Quota)

	var ledgerCount int64
	require.NoError(t, db.Model(&model.WalletLedgerEntry{}).Where("operation_id = ?", "topup:"+topUp.TradeNo).Count(&ledgerCount).Error)
	require.EqualValues(t, 1, ledgerCount)
	var auditCount int64
	require.NoError(t, db.Model(&model.AuditOutboxEvent{}).Where("event_id = ?", "wallet:topup:"+topUp.TradeNo).Count(&auditCount).Error)
	require.EqualValues(t, 1, auditCount)
	var auditLog model.Log
	require.NoError(t, db.Where("user_id = ? AND type = ?", user.Id, model.LogTypeTopup).First(&auditLog).Error)
	require.Equal(t, "127.0.0.1", auditLog.Ip)

	require.Equal(t, "success", postEpayCallback(t, callbackServer.URL+"/api/user/epay/notify", callback))
	require.NoError(t, db.First(&updatedUser, user.Id).Error)
	require.Equal(t, int64(expectedQuota), updatedUser.Quota)
	require.NoError(t, db.Model(&model.WalletLedgerEntry{}).Where("operation_id = ?", "topup:"+topUp.TradeNo).Count(&ledgerCount).Error)
	require.EqualValues(t, 1, ledgerCount)
}

func TestEpayNotifyRejectsInvalidSignature(t *testing.T) {
	db := setupEpayTopupIntegrationDB(t)
	callbackRouter := gin.New()
	callbackRouter.POST("/api/user/epay/notify", EpayNotify)
	callbackServer := httptest.NewServer(callbackRouter)
	t.Cleanup(callbackServer.Close)
	configureEpayTopupIntegration(t, "http://local-gateway.invalid", callbackServer.URL)

	user := seedTokenOwner(t, db, 772, "epay-signature-user", "epay-signature@example.com")
	topUp := &model.TopUp{
		UserId:          user.Id,
		Amount:          3,
		Money:           3,
		TradeNo:         "EPAY-INVALID-SIGNATURE",
		PaymentMethod:   "alipay",
		PaymentProvider: model.PaymentProviderEpay,
		Status:          common.TopUpStatusPending,
		CreateTime:      common.GetTimestamp(),
	}
	require.NoError(t, db.Create(topUp).Error)

	params := signedEpayCallbackParams(topUp.TradeNo, topUp.PaymentMethod)
	params["sign"] = "invalid"
	require.Equal(t, "fail", postEpayCallback(t, callbackServer.URL+"/api/user/epay/notify", params))

	var storedTopUp model.TopUp
	require.NoError(t, db.Where("trade_no = ?", topUp.TradeNo).First(&storedTopUp).Error)
	require.Equal(t, common.TopUpStatusPending, storedTopUp.Status)
	var storedUser model.User
	require.NoError(t, db.First(&storedUser, user.Id).Error)
	require.Zero(t, storedUser.Quota)
}

func TestEpayNotifyAcceptsHistoricalOrderAfterCheckoutAddressIsCleared(t *testing.T) {
	db := setupEpayTopupIntegrationDB(t)
	callbackRouter := gin.New()
	callbackRouter.POST("/api/user/epay/notify", EpayNotify)
	callbackServer := httptest.NewServer(callbackRouter)
	t.Cleanup(callbackServer.Close)
	configureEpayTopupIntegration(t, "http://local-gateway.invalid", callbackServer.URL)

	user := seedTokenOwner(t, db, 774, "epay-address-cleared", "epay-address-cleared@example.com")
	eligibleQuota, err := model.CalculateCashPaymentEligibleQuota("1.00", "1")
	require.NoError(t, err)
	topUp := &model.TopUp{
		UserId: user.Id, Amount: 1, Money: 1, TradeNo: "EPAY-ADDRESS-CLEARED",
		PaymentMethod: "alipay", PaymentProvider: model.PaymentProviderEpay,
		PaidAmount: "1.00", PaidCurrency: "CNY", EligibleQuota: eligibleQuota,
		Status: common.TopUpStatusPending, CreateTime: common.GetTimestamp(),
	}
	require.NoError(t, db.Create(topUp).Error)
	operation_setting.PayAddress = ""

	require.Equal(t, "success", postEpayCallback(t, callbackServer.URL+"/api/user/epay/notify", signedEpayCallbackParams(topUp.TradeNo, topUp.PaymentMethod)))
	var stored model.TopUp
	require.NoError(t, db.Where("trade_no = ?", topUp.TradeNo).First(&stored).Error)
	require.Equal(t, common.TopUpStatusSuccess, stored.Status)
}

func TestEpayNotifyReturnsServerErrorOnOrderQueryFailure(t *testing.T) {
	db := setupEpayTopupIntegrationDB(t)
	callbackRouter := gin.New()
	callbackRouter.POST("/api/user/epay/notify", EpayNotify)
	callbackServer := httptest.NewServer(callbackRouter)
	t.Cleanup(callbackServer.Close)
	configureEpayTopupIntegration(t, "http://local-gateway.invalid", callbackServer.URL)

	failure := errors.New("forced Epay order query outage")
	const callbackName = "epay-notify:topup-query-failure"
	require.NoError(t, db.Callback().Query().Before("gorm:query").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Schema != nil && tx.Statement.Schema.Name == "TopUp" {
			tx.AddError(failure)
		}
	}))
	t.Cleanup(func() { _ = db.Callback().Query().Remove(callbackName) })

	response, err := http.PostForm(
		callbackServer.URL+"/api/user/epay/notify",
		valuesFromStringMap(signedEpayCallbackParams("EPAY-DB-OUTAGE", "alipay")),
	)
	require.NoError(t, err)
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusInternalServerError, response.StatusCode)
	require.Equal(t, "fail", string(body))
}

func TestRequestEpayRejectsAmountThatExceedsWalletQuota(t *testing.T) {
	db := setupEpayTopupIntegrationDB(t)
	configureEpayTopupIntegration(t, "http://local-gateway.invalid", "http://local-callback.invalid")

	user := seedTokenOwner(t, db, 774, "epay-overflow-user", "epay-overflow@example.com")
	require.NoError(t, db.Model(user).Update("group", "default").Error)
	require.Greater(t, common.QuotaPerUnit, 0.0)
	overflowAmount := int64(common.MaxQuota)/int64(common.QuotaPerUnit) + 1

	requestContext, requestRecorder := newAuthenticatedContext(t, http.MethodPost, "/api/user/pay", EpayRequest{
		Amount:        overflowAmount,
		PaymentMethod: "alipay",
	}, user.Id)
	RequestEpay(requestContext)

	require.Equal(t, http.StatusOK, requestRecorder.Code)
	response := decodeAPIResponse(t, requestRecorder)
	require.Equal(t, "error", response.Message)
	var topUpCount int64
	require.NoError(t, db.Model(&model.TopUp{}).Count(&topUpCount).Error)
	require.Zero(t, topUpCount)
}

func TestSubscriptionEpayLocalGatewayCompletesOrderAndPreservesProviderAudit(t *testing.T) {
	db := setupEpayTopupIntegrationDB(t)

	callbackRouter := gin.New()
	callbackRouter.POST("/api/subscription/epay/notify", SubscriptionEpayNotify)
	callbackServer := httptest.NewServer(callbackRouter)
	t.Cleanup(callbackServer.Close)

	callbackParams := make(chan map[string]string, 1)
	gatewayServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/submit.php" {
			http.NotFound(writer, request)
			return
		}
		if err := request.ParseForm(); err != nil {
			http.Error(writer, err.Error(), http.StatusBadRequest)
			return
		}
		if request.PostForm.Get("notify_url") != callbackServer.URL+"/api/subscription/epay/notify" {
			http.Error(writer, "unexpected notify URL", http.StatusBadRequest)
			return
		}
		callback := signedEpayCallbackParams(request.PostForm.Get("out_trade_no"), request.PostForm.Get("type"))
		response, err := http.PostForm(request.PostForm.Get("notify_url"), valuesFromStringMap(callback))
		if err != nil {
			http.Error(writer, err.Error(), http.StatusBadGateway)
			return
		}
		defer response.Body.Close()
		body, err := io.ReadAll(response.Body)
		if err != nil {
			http.Error(writer, err.Error(), http.StatusBadGateway)
			return
		}
		writer.Header().Set("X-Callback-Response", string(body))
		callbackParams <- callback
		writer.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(gatewayServer.Close)
	configureEpayTopupIntegration(t, gatewayServer.URL, callbackServer.URL)

	user := seedTokenOwner(t, db, 775, "epay-subscription-user", "epay-subscription@example.com")
	require.NoError(t, db.Model(user).Update("group", "default").Error)
	plan := &model.SubscriptionPlan{
		Title:         "Local ePay Plan",
		PriceAmount:   9.99,
		Currency:      "USD",
		DurationUnit:  model.SubscriptionDurationMonth,
		DurationValue: 1,
		Enabled:       true,
		TotalAmount:   1000,
	}
	require.NoError(t, db.Create(plan).Error)
	model.InvalidateSubscriptionPlanCache(plan.Id)

	requestContext, requestRecorder := newAuthenticatedContext(t, http.MethodPost, "/api/subscription/epay/pay", SubscriptionEpayPayRequest{
		PlanId:        plan.Id,
		PaymentMethod: "alipay",
	}, user.Id)
	SubscriptionRequestEpay(requestContext)
	require.Equal(t, http.StatusOK, requestRecorder.Code)

	var purchase epayPurchaseResponse
	require.NoError(t, common.Unmarshal(requestRecorder.Body.Bytes(), &purchase))
	require.Equal(t, "success", purchase.Message)
	require.Equal(t, gatewayServer.URL+"/submit.php", purchase.URL)

	purchaseResponse, err := http.PostForm(purchase.URL, valuesFromStringMap(purchase.Data))
	require.NoError(t, err)
	defer purchaseResponse.Body.Close()
	require.Equal(t, http.StatusOK, purchaseResponse.StatusCode)
	require.Equal(t, "success", purchaseResponse.Header.Get("X-Callback-Response"))

	callback := <-callbackParams
	var order model.SubscriptionOrder
	require.NoError(t, db.Where("trade_no = ?", purchase.Data["out_trade_no"]).First(&order).Error)
	require.Equal(t, common.TopUpStatusSuccess, order.Status)
	require.Equal(t, model.PaymentProviderEpay, order.PaymentProvider)
	require.Equal(t, "alipay", order.PaymentMethod)

	var topUp model.TopUp
	require.NoError(t, db.Where("trade_no = ?", order.TradeNo).First(&topUp).Error)
	require.Equal(t, common.TopUpStatusSuccess, topUp.Status)
	require.Equal(t, model.PaymentProviderEpay, topUp.PaymentProvider)
	require.Equal(t, "alipay", topUp.PaymentMethod)
	var subscriptionCount int64
	require.NoError(t, db.Model(&model.UserSubscription{}).Where("user_id = ? AND plan_id = ?", user.Id, plan.Id).Count(&subscriptionCount).Error)
	require.EqualValues(t, 1, subscriptionCount)

	require.Equal(t, "success", postEpayCallback(t, callbackServer.URL+"/api/subscription/epay/notify", callback))
	require.NoError(t, db.Model(&model.UserSubscription{}).Where("user_id = ? AND plan_id = ?", user.Id, plan.Id).Count(&subscriptionCount).Error)
	require.EqualValues(t, 1, subscriptionCount)
}

func TestSubscriptionEpayNotifyReturnsServerErrorOnOrderQueryFailure(t *testing.T) {
	db := setupEpayTopupIntegrationDB(t)
	callbackRouter := gin.New()
	callbackRouter.POST("/api/subscription/epay/notify", SubscriptionEpayNotify)
	callbackServer := httptest.NewServer(callbackRouter)
	t.Cleanup(callbackServer.Close)
	configureEpayTopupIntegration(t, "http://local-gateway.invalid", callbackServer.URL)
	registerPaymentQueryFailure(t, db, "payment-retry:epay-subscription-query", "SubscriptionOrder")

	response, err := http.PostForm(
		callbackServer.URL+"/api/subscription/epay/notify",
		valuesFromStringMap(signedEpayCallbackParams("EPAY-SUBSCRIPTION-DB-OUTAGE", "alipay")),
	)
	require.NoError(t, err)
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusInternalServerError, response.StatusCode)
	require.Equal(t, "fail", string(body))
}

func TestEpayNotifyDoesNotAcknowledgeBeforeWalletCreditCommits(t *testing.T) {
	db := setupEpayTopupIntegrationDB(t)
	callbackRouter := gin.New()
	callbackRouter.POST("/api/user/epay/notify", EpayNotify)
	callbackServer := httptest.NewServer(callbackRouter)
	t.Cleanup(callbackServer.Close)
	configureEpayTopupIntegration(t, "http://local-gateway.invalid", callbackServer.URL)

	user := seedTokenOwner(t, db, 773, "epay-commit-user", "epay-commit@example.com")
	topUp := &model.TopUp{
		UserId:          user.Id,
		Amount:          3,
		Money:           3,
		TradeNo:         "EPAY-COMMIT-FAILURE",
		PaymentMethod:   "alipay",
		PaymentProvider: model.PaymentProviderEpay,
		Status:          common.TopUpStatusPending,
		CreateTime:      common.GetTimestamp(),
	}
	require.NoError(t, db.Create(topUp).Error)

	// Missing audit storage simulates a durable accounting transaction failure.
	require.NoError(t, db.Migrator().DropTable(&model.AuditOutboxEvent{}))
	require.Equal(t, "fail", postEpayCallback(t, callbackServer.URL+"/api/user/epay/notify", signedEpayCallbackParams(topUp.TradeNo, topUp.PaymentMethod)))

	var storedTopUp model.TopUp
	require.NoError(t, db.Where("trade_no = ?", topUp.TradeNo).First(&storedTopUp).Error)
	require.Equal(t, common.TopUpStatusPending, storedTopUp.Status)
	var storedUser model.User
	require.NoError(t, db.First(&storedUser, user.Id).Error)
	require.Zero(t, storedUser.Quota)
}
