package controller

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/gin-gonic/gin"
	"github.com/stripe/stripe-go/v81"
	"github.com/stripe/stripe-go/v81/checkout/session"
	"github.com/stripe/stripe-go/v81/webhook"
	"github.com/thanhpk/randstr"
	"gorm.io/gorm"
)

var stripeAdaptor = &StripeAdaptor{}

// StripePayRequest represents a payment request for Stripe checkout.
type StripePayRequest struct {
	// Amount is the quantity of units to purchase.
	Amount int64 `json:"amount"`
	// PaymentMethod specifies the payment method (e.g., "stripe").
	PaymentMethod string `json:"payment_method"`
	// SuccessURL is the optional custom URL to redirect after successful payment.
	// If empty, defaults to the server's console log page.
	SuccessURL string `json:"success_url,omitempty"`
	// CancelURL is the optional custom URL to redirect when payment is canceled.
	// If empty, defaults to the server's console topup page.
	CancelURL string `json:"cancel_url,omitempty"`
}

type StripeAdaptor struct {
}

func (*StripeAdaptor) RequestAmount(c *gin.Context, req *StripePayRequest) {
	if req.Amount < getStripeMinTopup() {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": fmt.Sprintf("充值数量不能小于 %d", getStripeMinTopup())})
		return
	}
	if _, err := normalizeAndValidateWalletTopUpAmount(req.Amount); err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "充值额度超出允许范围"})
		return
	}
	id := c.GetInt("id")
	group, err := model.GetUserGroup(id, true)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "获取用户分组失败"})
		return
	}
	payMoney := getStripePayMoney(float64(req.Amount), group)
	if payMoney <= 0.01 {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "充值金额过低"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "success", "data": strconv.FormatFloat(payMoney, 'f', 2, 64)})
}

func (*StripeAdaptor) RequestPay(c *gin.Context, req *StripePayRequest) {
	if req.PaymentMethod != model.PaymentMethodStripe {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "不支持的支付渠道"})
		return
	}
	if req.Amount < getStripeMinTopup() {
		c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("充值数量不能小于 %d", getStripeMinTopup()), "data": 10})
		return
	}
	amount, amountErr := normalizeAndValidateWalletTopUpAmount(req.Amount)
	if amountErr != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "充值额度超出允许范围"})
		return
	}
	if amount > 10000 {
		c.JSON(http.StatusOK, gin.H{"message": "充值数量不能大于 10000", "data": 10})
		return
	}

	if req.SuccessURL != "" && common.ValidateRedirectURL(req.SuccessURL) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "支付成功重定向URL不在可信任域名列表中", "data": ""})
		return
	}

	if req.CancelURL != "" && common.ValidateRedirectURL(req.CancelURL) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "支付取消重定向URL不在可信任域名列表中", "data": ""})
		return
	}

	id := c.GetInt("id")
	user, err := model.GetUserById(id, false)
	if err != nil || user == nil {
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("Stripe 支付请求用户不存在 user_id=%d error=%v", id, err))
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "用户不存在"})
		return
	}
	chargedMoney := getStripePayMoney(float64(req.Amount), user.Group)
	if chargedMoney <= 0.01 {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "充值金额过低"})
		return
	}

	reference := fmt.Sprintf("new-api-ref-%d-%d-%s", user.Id, time.Now().UnixMilli(), randstr.String(4))
	referenceId := "ref_" + common.Sha1([]byte(reference))

	topUp := &model.TopUp{
		UserId:          id,
		Amount:          amount,
		Money:           chargedMoney,
		TradeNo:         referenceId,
		PaymentMethod:   model.PaymentMethodStripe,
		PaymentProvider: model.PaymentProviderStripe,
		CreateTime:      time.Now().Unix(),
		Status:          common.TopUpStatusPending,
	}
	topUp.PaidAmount = strconv.FormatFloat(chargedMoney, 'f', 2, 64)
	topUp.PaidCurrency = "USD"
	topUp.EligibleQuota, err = model.CalculateCashPaymentEligibleQuotaForCurrency(topUp.PaidAmount, topUp.PaidCurrency)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "支付金额无效"})
		return
	}
	err = topUp.Insert()
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Stripe 创建充值订单失败 user_id=%d trade_no=%s amount=%d error=%q", id, referenceId, req.Amount, err.Error()))
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "创建订单失败"})
		return
	}

	payLink, err := genStripeLink(referenceId, user.StripeCustomer, user.Email, amount, req.SuccessURL, req.CancelURL)
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("Stripe 创建 Checkout Session 失败 user_id=%d trade_no=%s requested_amount=%d amount=%d error=%q", id, referenceId, req.Amount, amount, err.Error()))
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "拉起支付失败"})
		return
	}
	logger.LogInfo(c.Request.Context(), fmt.Sprintf("Stripe 充值订单创建成功 user_id=%d trade_no=%s requested_amount=%d amount=%d money=%.2f", id, referenceId, req.Amount, amount, chargedMoney))
	c.JSON(http.StatusOK, gin.H{
		"message": "success",
		"data": gin.H{
			"pay_link": payLink,
		},
	})
}

func RequestStripeAmount(c *gin.Context) {
	var req StripePayRequest
	err := c.ShouldBindJSON(&req)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "参数错误"})
		return
	}
	stripeAdaptor.RequestAmount(c, &req)
}

func RequestStripePay(c *gin.Context) {
	if !requirePaymentCompliance(c) {
		return
	}
	if !isStripeTopUpEnabled() {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "Stripe 支付未启用或配置不完整"})
		return
	}

	var req StripePayRequest
	err := c.ShouldBindJSON(&req)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "参数错误"})
		return
	}
	stripeAdaptor.RequestPay(c, &req)
}

func StripeWebhook(c *gin.Context) {
	ctx := c.Request.Context()
	if !isStripeWebhookEnabled() {
		logger.LogWarn(ctx, fmt.Sprintf("Stripe webhook 被拒绝 reason=webhook_disabled path=%q client_ip=%s", c.Request.RequestURI, c.ClientIP()))
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	payload, err := io.ReadAll(c.Request.Body)
	if err != nil {
		logger.LogError(ctx, fmt.Sprintf("Stripe webhook 读取请求体失败 path=%q client_ip=%s error=%q", c.Request.RequestURI, c.ClientIP(), err.Error()))
		c.AbortWithStatus(http.StatusServiceUnavailable)
		return
	}

	signature := c.GetHeader("Stripe-Signature")
	logger.LogInfo(ctx, fmt.Sprintf("Stripe webhook 收到请求 path=%q client_ip=%s signature_present=%t body_bytes=%d", c.Request.RequestURI, c.ClientIP(), signature != "", len(payload)))
	event, err := webhook.ConstructEventWithOptions(payload, signature, setting.StripeWebhookSecret, webhook.ConstructEventOptions{
		IgnoreAPIVersionMismatch: true,
	})

	if err != nil {
		logger.LogWarn(ctx, fmt.Sprintf("Stripe webhook 验签失败 path=%q client_ip=%s error=%q", c.Request.RequestURI, c.ClientIP(), err.Error()))
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	callerIp := c.ClientIP()
	logger.LogInfo(ctx, fmt.Sprintf("Stripe webhook 验签成功 event_type=%s client_ip=%s path=%q", string(event.Type), callerIp, c.Request.RequestURI))
	var processingErr error
	switch event.Type {
	case stripe.EventTypeCheckoutSessionCompleted:
		processingErr = sessionCompleted(ctx, event, callerIp)
	case stripe.EventTypeCheckoutSessionExpired:
		processingErr = sessionExpired(ctx, event)
	case stripe.EventTypeCheckoutSessionAsyncPaymentSucceeded:
		processingErr = sessionAsyncPaymentSucceeded(ctx, event, callerIp)
	case stripe.EventTypeCheckoutSessionAsyncPaymentFailed:
		processingErr = sessionAsyncPaymentFailed(ctx, event, callerIp)
	case stripe.EventTypeChargeSucceeded:
		if err := recordStripeChargeReference(event); err != nil {
			logger.LogError(ctx, fmt.Sprintf("Stripe 支付引用记录失败 event_id=%s error=%q", event.ID, err.Error()))
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
	case stripe.EventTypeChargeRefunded:
		if err := handleStripeChargeRefund(event); err != nil {
			logger.LogError(ctx, fmt.Sprintf("Stripe 退款处理失败 event_id=%s error=%q", event.ID, err.Error()))
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
	case stripe.EventTypeChargeDisputeCreated:
		if err := handleStripeDispute(event); err != nil {
			logger.LogError(ctx, fmt.Sprintf("Stripe 拒付处理失败 event_id=%s error=%q", event.ID, err.Error()))
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
	default:
		logger.LogInfo(ctx, fmt.Sprintf("Stripe webhook 忽略事件 event_type=%s client_ip=%s", string(event.Type), callerIp))
	}
	if processingErr != nil {
		logger.LogError(ctx, fmt.Sprintf("Stripe webhook 业务处理失败 event_id=%s event_type=%s error=%q", event.ID, event.Type, processingErr.Error()))
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	c.Status(http.StatusOK)
}

func stripeEventObject(event stripe.Event, target any) error {
	if event.Data == nil || len(event.Data.Raw) == 0 {
		return model.ErrCashPaymentInvalid
	}
	return common.Unmarshal(event.Data.Raw, target)
}

func recordStripeChargeReference(event stripe.Event) error {
	var charge stripe.Charge
	if err := stripeEventObject(event, &charge); err != nil {
		return err
	}
	tradeNo := strings.TrimSpace(charge.Metadata["trade_no"])
	if tradeNo == "" {
		return nil
	}
	references := []model.PaymentProviderReferenceInput{
		{Provider: model.PaymentProviderStripe, ReferenceType: "charge", ExternalID: charge.ID, TradeNo: tradeNo},
	}
	if charge.PaymentIntent != nil {
		references = append(references, model.PaymentProviderReferenceInput{
			Provider: model.PaymentProviderStripe, ReferenceType: "payment_intent", ExternalID: charge.PaymentIntent.ID, TradeNo: tradeNo,
		})
	}
	for _, reference := range references {
		if strings.TrimSpace(reference.ExternalID) == "" {
			continue
		}
		if err := model.RecordPaymentProviderReference(reference); err != nil {
			return err
		}
	}
	return nil
}

func resolveStripeChargeTradeNo(charge *stripe.Charge) (string, error) {
	if charge == nil {
		return "", model.ErrCashPaymentInvalid
	}
	direct := strings.TrimSpace(charge.Metadata["trade_no"])
	paymentIntentID := ""
	if charge.PaymentIntent != nil {
		paymentIntentID = charge.PaymentIntent.ID
	}
	return resolvePaymentTradeNo(model.PaymentProviderStripe, direct,
		model.PaymentProviderReferenceInput{ReferenceType: "charge", ExternalID: charge.ID},
		model.PaymentProviderReferenceInput{ReferenceType: "payment_intent", ExternalID: paymentIntentID},
	)
}

func handleStripeChargeRefund(event stripe.Event) error {
	var charge stripe.Charge
	if err := stripeEventObject(event, &charge); err != nil {
		return err
	}
	tradeNo, err := resolveStripeChargeTradeNo(&charge)
	if err != nil {
		return err
	}
	inputs, err := stripeChargeRefundCommissionReversalInputs(&charge, tradeNo)
	if err != nil {
		return err
	}
	LockOrder(tradeNo)
	defer UnlockOrder(tradeNo)
	for _, input := range inputs {
		if _, err := model.ProcessPaymentRefund(input); err != nil {
			return err
		}
	}
	return nil
}

func handleStripeDispute(event stripe.Event) error {
	var dispute stripe.Dispute
	if err := stripeEventObject(event, &dispute); err != nil {
		return err
	}
	direct := strings.TrimSpace(dispute.Metadata["trade_no"])
	chargeID := ""
	if dispute.Charge != nil {
		chargeID = dispute.Charge.ID
		if direct == "" {
			direct = strings.TrimSpace(dispute.Charge.Metadata["trade_no"])
		}
	}
	paymentIntentID := ""
	if dispute.PaymentIntent != nil {
		paymentIntentID = dispute.PaymentIntent.ID
	}
	tradeNo, err := resolvePaymentTradeNo(model.PaymentProviderStripe, direct,
		model.PaymentProviderReferenceInput{ReferenceType: "charge", ExternalID: chargeID},
		model.PaymentProviderReferenceInput{ReferenceType: "payment_intent", ExternalID: paymentIntentID},
	)
	if err != nil {
		return err
	}
	input, err := stripeDisputeCommissionReversalInput(&dispute, tradeNo)
	if err != nil {
		return err
	}
	LockOrder(tradeNo)
	defer UnlockOrder(tradeNo)
	_, err = model.ProcessPaymentRefund(input)
	return err
}

func sessionCompleted(ctx context.Context, event stripe.Event, callerIp string) error {
	customerId := event.GetObjectValue("customer")
	referenceId := event.GetObjectValue("client_reference_id")
	status := event.GetObjectValue("status")
	if "complete" != status {
		logger.LogWarn(ctx, fmt.Sprintf("Stripe checkout.completed 状态异常，忽略处理 trade_no=%s status=%s client_ip=%s", referenceId, status, callerIp))
		return nil
	}

	paymentStatus := event.GetObjectValue("payment_status")
	if paymentStatus != "paid" {
		logger.LogInfo(ctx, fmt.Sprintf("Stripe Checkout 支付未完成，等待异步结果 trade_no=%s payment_status=%s client_ip=%s", referenceId, paymentStatus, callerIp))
		return nil
	}

	return fulfillOrder(ctx, event, referenceId, customerId, callerIp)
}

// sessionAsyncPaymentSucceeded handles delayed payment methods (bank transfer, SEPA, etc.)
// that confirm payment after the checkout session completes.
func sessionAsyncPaymentSucceeded(ctx context.Context, event stripe.Event, callerIp string) error {
	customerId := event.GetObjectValue("customer")
	referenceId := event.GetObjectValue("client_reference_id")
	logger.LogInfo(ctx, fmt.Sprintf("Stripe 异步支付成功 trade_no=%s client_ip=%s", referenceId, callerIp))

	return fulfillOrder(ctx, event, referenceId, customerId, callerIp)
}

// sessionAsyncPaymentFailed marks orders as failed when delayed payment methods
// ultimately fail (e.g. bank transfer not received, SEPA rejected).
func sessionAsyncPaymentFailed(ctx context.Context, event stripe.Event, callerIp string) error {
	referenceId := event.GetObjectValue("client_reference_id")
	logger.LogWarn(ctx, fmt.Sprintf("Stripe 异步支付失败 trade_no=%s client_ip=%s", referenceId, callerIp))

	if len(referenceId) == 0 {
		logger.LogWarn(ctx, fmt.Sprintf("Stripe 异步支付失败事件缺少订单号 client_ip=%s", callerIp))
		return nil
	}

	LockOrder(referenceId)
	defer UnlockOrder(referenceId)

	if err := model.FailSubscriptionOrder(referenceId, model.PaymentProviderStripe); err == nil {
		logger.LogInfo(ctx, fmt.Sprintf("Stripe 订阅订单已标记为支付失败 trade_no=%s client_ip=%s", referenceId, callerIp))
		return nil
	} else if !errors.Is(err, model.ErrSubscriptionOrderNotFound) {
		if errors.Is(err, model.ErrPaymentMethodMismatch) {
			logger.LogWarn(ctx, fmt.Sprintf("Stripe 异步支付失败但订阅订单支付网关不匹配 trade_no=%s client_ip=%s", referenceId, callerIp))
			return nil
		}
		logger.LogError(ctx, fmt.Sprintf("Stripe 标记订阅订单支付失败状态失败 trade_no=%s client_ip=%s error=%q", referenceId, callerIp, err.Error()))
		return err
	}

	err := model.UpdatePendingTopUpStatus(referenceId, model.PaymentProviderStripe, common.TopUpStatusFailed)
	if errors.Is(err, model.ErrTopUpNotFound) {
		logger.LogWarn(ctx, fmt.Sprintf("Stripe 异步支付失败但本地订单不存在 trade_no=%s client_ip=%s", referenceId, callerIp))
		return nil
	}
	if errors.Is(err, model.ErrPaymentMethodMismatch) {
		logger.LogWarn(ctx, fmt.Sprintf("Stripe 异步支付失败但订单支付网关不匹配 trade_no=%s client_ip=%s", referenceId, callerIp))
		return nil
	}
	if errors.Is(err, model.ErrTopUpStatusInvalid) {
		logger.LogInfo(ctx, fmt.Sprintf("Stripe 异步支付失败但订单状态已变化，忽略处理 trade_no=%s client_ip=%s", referenceId, callerIp))
		return nil
	}
	if err != nil {
		logger.LogError(ctx, fmt.Sprintf("Stripe 标记充值订单失败状态失败 trade_no=%s client_ip=%s error=%q", referenceId, callerIp, err.Error()))
		return err
	}
	logger.LogInfo(ctx, fmt.Sprintf("Stripe 充值订单已标记为失败 trade_no=%s client_ip=%s", referenceId, callerIp))
	return nil
}

// fulfillOrder is the shared logic for crediting quota after payment is confirmed.
func fulfillOrder(ctx context.Context, event stripe.Event, referenceId string, customerId string, callerIp string) error {
	if len(referenceId) == 0 {
		logger.LogWarn(ctx, fmt.Sprintf("Stripe 完成订单时缺少订单号 client_ip=%s", callerIp))
		return nil
	}

	LockOrder(referenceId)
	defer UnlockOrder(referenceId)
	payload := map[string]any{
		"customer":     customerId,
		"amount_total": event.GetObjectValue("amount_total"),
		"currency":     strings.ToUpper(event.GetObjectValue("currency")),
		"event_type":   string(event.Type),
	}
	var settlement *model.CashPaymentSettlement
	if total, parseErr := strconv.ParseInt(event.GetObjectValue("amount_total"), 10, 64); parseErr == nil && total > 0 {
		settlement = &model.CashPaymentSettlement{
			PaidAmount:   strconv.FormatFloat(float64(total)/100, 'f', 2, 64),
			PaidCurrency: strings.ToUpper(event.GetObjectValue("currency")),
		}
	}
	if err := model.CompleteSubscriptionOrderWithSettlement(referenceId, common.GetJsonString(payload), model.PaymentProviderStripe, "", settlement); err == nil {
		logger.LogInfo(ctx, fmt.Sprintf("Stripe 订阅订单处理成功 trade_no=%s event_type=%s client_ip=%s", referenceId, string(event.Type), callerIp))
		return nil
	} else if err != nil && !errors.Is(err, model.ErrSubscriptionOrderNotFound) {
		return err
	}
	topUp, err := model.FindTopUpByTradeNo(referenceId)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		logger.LogWarn(ctx, fmt.Sprintf("Stripe 完成订单时本地订单不存在 trade_no=%s client_ip=%s", referenceId, callerIp))
		return nil
	}
	if err != nil {
		return err
	}
	if topUp.PaymentProvider != model.PaymentProviderStripe {
		return model.ErrPaymentMethodMismatch
	}
	if topUp.Status == common.TopUpStatusSuccess {
		return nil
	}
	if topUp.Status != common.TopUpStatusPending {
		logger.LogInfo(ctx, fmt.Sprintf("Stripe 完成订单时本地订单已结束 trade_no=%s status=%s", referenceId, topUp.Status))
		return nil
	}

	err = model.RechargeWithSettlement(referenceId, customerId, callerIp, settlement)
	if err != nil {
		return err
	}

	total, _ := strconv.ParseFloat(event.GetObjectValue("amount_total"), 64)
	currency := strings.ToUpper(event.GetObjectValue("currency"))
	logger.LogInfo(ctx, fmt.Sprintf("Stripe 充值成功 trade_no=%s amount_total=%.2f currency=%s event_type=%s client_ip=%s", referenceId, total/100, currency, string(event.Type), callerIp))
	return nil
}

func sessionExpired(ctx context.Context, event stripe.Event) error {
	referenceId := event.GetObjectValue("client_reference_id")
	status := event.GetObjectValue("status")
	if "expired" != status {
		logger.LogWarn(ctx, fmt.Sprintf("Stripe checkout.expired 状态异常，忽略处理 trade_no=%s status=%s", referenceId, status))
		return nil
	}

	if len(referenceId) == 0 {
		logger.LogWarn(ctx, "Stripe checkout.expired 缺少订单号")
		return nil
	}

	// Subscription order expiration
	LockOrder(referenceId)
	defer UnlockOrder(referenceId)
	if err := model.ExpireSubscriptionOrder(referenceId, model.PaymentProviderStripe); err == nil {
		logger.LogInfo(ctx, fmt.Sprintf("Stripe 订阅订单已过期 trade_no=%s", referenceId))
		return nil
	} else if err != nil && !errors.Is(err, model.ErrSubscriptionOrderNotFound) {
		if errors.Is(err, model.ErrPaymentMethodMismatch) {
			logger.LogWarn(ctx, fmt.Sprintf("Stripe 订阅订单过期事件支付网关不匹配 trade_no=%s", referenceId))
			return nil
		}
		logger.LogError(ctx, fmt.Sprintf("Stripe 订阅订单过期处理失败 trade_no=%s error=%q", referenceId, err.Error()))
		return err
	}

	err := model.UpdatePendingTopUpStatus(referenceId, model.PaymentProviderStripe, common.TopUpStatusExpired)
	if errors.Is(err, model.ErrTopUpNotFound) {
		logger.LogWarn(ctx, fmt.Sprintf("Stripe 充值订单不存在，无法标记过期 trade_no=%s", referenceId))
		return nil
	}
	if errors.Is(err, model.ErrPaymentMethodMismatch) || errors.Is(err, model.ErrTopUpStatusInvalid) {
		logger.LogInfo(ctx, fmt.Sprintf("Stripe 充值订单过期事件已无需处理 trade_no=%s", referenceId))
		return nil
	}
	if err != nil {
		logger.LogError(ctx, fmt.Sprintf("Stripe 充值订单过期处理失败 trade_no=%s error=%q", referenceId, err.Error()))
		return err
	}

	logger.LogInfo(ctx, fmt.Sprintf("Stripe 充值订单已过期 trade_no=%s", referenceId))
	return nil
}

// genStripeLink generates a Stripe Checkout session URL for payment.
// It creates a new checkout session with the specified parameters and returns the payment URL.
//
// Parameters:
//   - referenceId: unique reference identifier for the transaction
//   - customerId: existing Stripe customer ID (empty string if new customer)
//   - email: customer email address for new customer creation
//   - amount: quantity of units to purchase
//   - successURL: custom URL to redirect after successful payment (empty for default)
//   - cancelURL: custom URL to redirect when payment is canceled (empty for default)
//
// Returns the checkout session URL or an error if the session creation fails.
func genStripeLink(referenceId string, customerId string, email string, amount int64, successURL string, cancelURL string) (string, error) {
	if !strings.HasPrefix(setting.StripeApiSecret, "sk_") && !strings.HasPrefix(setting.StripeApiSecret, "rk_") {
		return "", fmt.Errorf("无效的Stripe API密钥")
	}

	stripe.Key = setting.StripeApiSecret

	// Use custom URLs if provided, otherwise use defaults
	if successURL == "" {
		successURL = paymentReturnPath("/console/log")
	}
	if cancelURL == "" {
		cancelURL = paymentReturnPath("/console/topup")
	}

	params := &stripe.CheckoutSessionParams{
		ClientReferenceID: stripe.String(referenceId),
		SuccessURL:        stripe.String(successURL),
		CancelURL:         stripe.String(cancelURL),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				Price:    stripe.String(setting.StripePriceId),
				Quantity: stripe.Int64(amount),
			},
		},
		Mode:                stripe.String(string(stripe.CheckoutSessionModePayment)),
		AllowPromotionCodes: stripe.Bool(setting.StripePromotionCodesEnabled),
		Metadata:            map[string]string{"trade_no": referenceId},
		PaymentIntentData: &stripe.CheckoutSessionPaymentIntentDataParams{
			Metadata: map[string]string{"trade_no": referenceId},
		},
	}

	if "" == customerId {
		if "" != email {
			params.CustomerEmail = stripe.String(email)
		}

		params.CustomerCreation = stripe.String(string(stripe.CheckoutSessionCustomerCreationAlways))
	} else {
		params.Customer = stripe.String(customerId)
	}

	result, err := session.New(params)
	if err != nil {
		return "", err
	}

	return result.URL, nil
}

func getStripePayMoney(amount float64, _ string) float64 {
	if operation_setting.GetQuotaDisplayType() == operation_setting.QuotaDisplayTypeTokens {
		amount = amount / common.QuotaPerUnit
	}
	// Stripe Checkout charges the configured fixed Price ID. Group ratios and
	// local amount discounts cannot be represented by that price, so promotions
	// for this gateway must use Stripe's native promotion codes.
	return amount * setting.StripeUnitPrice
}

func getStripeMinTopup() int64 {
	minTopup := setting.StripeMinTopUp
	if operation_setting.GetQuotaDisplayType() == operation_setting.QuotaDisplayTypeTokens {
		minTopup = common.QuotaFromFloat(float64(minTopup) * common.QuotaPerUnit)
	}
	return int64(minTopup)
}
