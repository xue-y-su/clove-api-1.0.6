package service

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// FundingSource — 资金来源接口（钱包 or 订阅）
// ---------------------------------------------------------------------------

// FundingSource 抽象了预扣费的资金来源。
type FundingSource interface {
	// Source 返回资金来源标识："wallet" 或 "subscription"
	Source() string
	// PreConsume 从该资金来源预扣 amount 额度
	PreConsume(amount int) error
	// Settle 根据差额调整资金来源（正数补扣，负数退还）
	Settle(delta int) error
	// Refund 退还所有预扣费
	Refund() error
}

// ---------------------------------------------------------------------------
// WalletFunding — 钱包资金来源实现
// ---------------------------------------------------------------------------

type WalletFunding struct {
	userId    int
	consumed  int // 实际预扣的用户额度
	requestId string
	seq       uint64
}

func NewWalletFunding(userID int, requestID string) *WalletFunding {
	if requestID == "" {
		requestID = common.NewRequestId()
	}
	return &WalletFunding{userId: userID, requestId: requestID}
}

func (w *WalletFunding) Source() string { return BillingSourceWallet }

func (w *WalletFunding) nextOp(phase string) string {
	n := atomic.AddUint64(&w.seq, 1)
	// Unique per mutation so refund+retry never hits a false idempotent no-op.
	return fmt.Sprintf("relay:%s:%s:%d:%d", w.requestId, phase, n, time.Now().UnixNano())
}

func (w *WalletFunding) adjust(delta int, phase string) error {
	if delta == 0 {
		return nil
	}
	_, err := model.AdjustWallet(w.userId, int64(delta), w.nextOp(phase), "relay", model.WalletAdjustReference{
		Type:    "relay_request",
		ID:      w.requestId,
		Context: phase,
	})
	return err
}

func (w *WalletFunding) PreConsume(amount int) error {
	if amount <= 0 {
		return nil
	}
	if err := w.adjust(-amount, "pre"); err != nil {
		return err
	}
	w.consumed = amount
	return nil
}

func (w *WalletFunding) Settle(delta int) error {
	if delta == 0 {
		return nil
	}
	// Billing settle delta: + means charge more, - means refund. Wallet ledger uses signed balance delta.
	phase := "settle_debit"
	if delta < 0 {
		phase = "settle_credit"
	}
	if err := w.adjust(-delta, phase); err != nil {
		return err
	}
	w.consumed += delta
	if w.consumed < 0 {
		w.consumed = 0
	}
	return nil
}

func (w *WalletFunding) Refund() error {
	if w.consumed <= 0 {
		return nil
	}
	// Non-retryable compensation entry; unique op avoids false idempotency after prior refund.
	amount := w.consumed
	if err := w.adjust(amount, "refund"); err != nil {
		return err
	}
	w.consumed = 0
	return nil
}

func (w *WalletFunding) reserveWithTx(tx *gorm.DB, delta int, phase string) (func(), error) {
	if delta <= 0 {
		return func() {}, nil
	}
	operationID := w.nextOp(phase)
	if _, err := model.AdjustWalletWithTx(tx, w.userId, -int64(delta), operationID, "relay", model.WalletAdjustReference{
		Type: "relay_request", ID: w.requestId, Context: phase,
	}); err != nil {
		return nil, err
	}
	return func() {
		w.consumed += delta
		_ = model.InvalidateUserCache(w.userId)
	}, nil
}

func (w *WalletFunding) settleWithTx(tx *gorm.DB, delta int) (func(), error) {
	if delta == 0 {
		return func() {}, nil
	}
	phase := "settle_debit"
	if delta < 0 {
		phase = "settle_credit"
	}
	operationID := w.nextOp(phase)
	if _, err := model.AdjustWalletWithTx(tx, w.userId, -int64(delta), operationID, "relay", model.WalletAdjustReference{
		Type: "relay_request", ID: w.requestId, Context: phase,
	}); err != nil {
		return nil, err
	}
	return func() {
		w.consumed += delta
		if w.consumed < 0 {
			w.consumed = 0
		}
		_ = model.InvalidateUserCache(w.userId)
	}, nil
}

func (w *WalletFunding) refundWithTx(tx *gorm.DB) (func(), error) {
	if w.consumed <= 0 {
		return func() {}, nil
	}
	amount := w.consumed
	operationID := w.nextOp("refund")
	if _, err := model.AdjustWalletWithTx(tx, w.userId, int64(amount), operationID, "relay", model.WalletAdjustReference{
		Type: "relay_request", ID: w.requestId, Context: "refund",
	}); err != nil {
		return nil, err
	}
	return func() {
		w.consumed = 0
		_ = model.InvalidateUserCache(w.userId)
	}, nil
}

// ---------------------------------------------------------------------------
// SubscriptionFunding — 订阅资金来源实现
// ---------------------------------------------------------------------------

type SubscriptionFunding struct {
	requestId      string
	userId         int
	modelName      string
	amount         int64 // 预扣的订阅额度（subConsume）
	eligibility    model.SubscriptionEligibility
	permitOverflow bool
	subscriptionId int
	preConsumed    int64
	walletOverflow int // 部分预扣时从钱包补扣的差额（0 = 无混合预扣）
	walletSeq      uint64
	// 以下字段在 PreConsume 成功后填充，供 RelayInfo 同步使用
	AmountTotal      int64
	AmountUsedBefore int64 // 用于检测重试（AmountUsedAfter == AmountUsedBefore 表示返回了已有记录）
	AmountUsedAfter  int64
	PlanId           int
	PlanTitle        string
}

func (s *SubscriptionFunding) Source() string { return BillingSourceSubscription }

func (s *SubscriptionFunding) nextWalletOp(phase string) string {
	n := atomic.AddUint64(&s.walletSeq, 1)
	return fmt.Sprintf("relay:%s:subwallet:%s:%d:%d", s.requestId, phase, n, time.Now().UnixNano())
}

func (s *SubscriptionFunding) adjustWallet(delta int, phase string) error {
	if delta == 0 {
		return nil
	}
	_, err := model.AdjustWallet(s.userId, int64(delta), s.nextWalletOp(phase), "relay", model.WalletAdjustReference{
		Type:    "relay_request",
		ID:      s.requestId,
		Context: "subscription_" + phase,
	})
	return err
}

func (s *SubscriptionFunding) applyReservation(res *model.RelayFundingReservationResult) {
	if res == nil {
		return
	}
	s.subscriptionId = res.UserSubscriptionID
	s.preConsumed = res.SubscriptionQuota
	s.walletOverflow = int(res.WalletQuota)
	s.AmountTotal = res.AmountTotal
	s.AmountUsedBefore = res.AmountUsedBefore
	s.AmountUsedAfter = res.AmountUsedAfter
	s.PlanId = res.PlanID
	s.PlanTitle = res.PlanTitle
}

func (s *SubscriptionFunding) setTarget(target int64) error {
	res, err := model.SetRelayFundingReservationTarget(
		s.requestId,
		s.userId,
		target,
		s.eligibility,
		s.permitOverflow,
	)
	if err != nil {
		return err
	}
	s.applyReservation(res)
	return nil
}

func (s *SubscriptionFunding) PreConsume(_ int) error {
	if s.amount <= 0 {
		return nil
	}
	return s.setTarget(s.amount)
}

func (s *SubscriptionFunding) Settle(delta int) error {
	current := s.preConsumed + int64(s.walletOverflow)
	target := current + int64(delta)
	if target < 0 {
		target = 0
	}
	res, err := model.SettleRelayFundingReservation(
		s.requestId,
		s.userId,
		target,
		s.eligibility,
		s.permitOverflow,
	)
	if err != nil {
		return err
	}
	s.applyReservation(res)
	return nil
}

func (s *SubscriptionFunding) Refund() error {
	if s.preConsumed <= 0 && s.walletOverflow <= 0 {
		return nil
	}
	err := refundWithRetry(func() error {
		return model.RefundRelayFundingReservation(s.requestId)
	})
	if err == nil {
		s.preConsumed = 0
		s.walletOverflow = 0
	}
	return err
}

func (s *SubscriptionFunding) reserveWithTx(tx *gorm.DB, delta int, _ string) (func(), error) {
	if delta <= 0 {
		return func() {}, nil
	}
	target := s.preConsumed + int64(s.walletOverflow) + int64(delta)
	result, err := model.SetRelayFundingReservationTargetWithTx(
		tx,
		s.requestId,
		s.userId,
		target,
		s.eligibility,
		s.permitOverflow,
	)
	if err != nil {
		return nil, err
	}
	return func() {
		s.applyReservation(result)
		_ = model.InvalidateUserCache(s.userId)
	}, nil
}

func (s *SubscriptionFunding) settleWithTx(tx *gorm.DB, delta int) (func(), error) {
	current := s.preConsumed + int64(s.walletOverflow)
	target := current + int64(delta)
	if target < 0 {
		target = 0
	}
	result, err := model.SettleRelayFundingReservationWithTx(
		tx,
		s.requestId,
		s.userId,
		target,
		s.eligibility,
		s.permitOverflow,
	)
	if err != nil {
		return nil, err
	}
	return func() {
		s.applyReservation(result)
		_ = model.InvalidateUserCache(s.userId)
	}, nil
}

func (s *SubscriptionFunding) refundWithTx(tx *gorm.DB) (func(), error) {
	if s.preConsumed <= 0 && s.walletOverflow <= 0 {
		return func() {}, nil
	}
	if err := model.RefundRelayFundingReservationWithTx(tx, s.requestId); err != nil {
		return nil, err
	}
	return func() {
		s.preConsumed = 0
		s.walletOverflow = 0
		_ = model.InvalidateUserCache(s.userId)
	}, nil
}

// refundWithRetry 尝试多次执行退款操作以提高成功率，只能用于基于事务的退款函数！！！！！！
// try to refund with retries, only for refund functions based on transactions!!!
func refundWithRetry(fn func() error) error {
	if fn == nil {
		return nil
	}
	const maxAttempts = 3
	var lastErr error
	for i := 0; i < maxAttempts; i++ {
		if err := fn(); err == nil {
			return nil
		} else {
			lastErr = err
		}
		if i < maxAttempts-1 {
			time.Sleep(time.Duration(200*(i+1)) * time.Millisecond)
		}
	}
	return lastErr
}
