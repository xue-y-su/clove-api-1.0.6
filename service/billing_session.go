package service

import (
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ---------------------------------------------------------------------------
// BillingSession — 统一计费会话
// ---------------------------------------------------------------------------

// BillingSession 封装单次请求的预扣费/结算/退款生命周期。
// 实现 relaycommon.BillingSettler 接口。
type BillingSession struct {
	relayInfo        *relaycommon.RelayInfo
	funding          FundingSource
	preConsumedQuota int  // 实际预扣额度（信任用户可能为 0）
	tokenConsumed    int  // 令牌额度实际扣减量
	trusted          bool // 是否命中信任额度旁路
	fundingSettled   bool // funding.Settle 已成功，资金来源已提交
	settled          bool // Settle 全部完成（资金 + 令牌）
	refunded         bool // Refund 已调用
	mu               sync.Mutex
}

var (
	ErrBillingSessionRefunded = errors.New("billing session already refunded")
	ErrBillingSessionSettled  = errors.New("billing session already settled")
	decreaseBillingTokenQuota = model.DecreaseTokenQuota
	increaseBillingTokenQuota = model.IncreaseTokenQuota
)

var billingTransactionConflictRetryDelays = [...]time.Duration{
	10 * time.Millisecond,
	25 * time.Millisecond,
	50 * time.Millisecond,
	100 * time.Millisecond,
}

// Each retry must open a new transaction so wallet and token state are
// reloaded together; retrying only the wallet update would break atomicity.
func runBillingTransactionWithRetry(transaction func() error) error {
	for attempt := 0; ; attempt++ {
		err := transaction()
		if !errors.Is(err, model.ErrWalletConcurrentMutation) || attempt >= len(billingTransactionConflictRetryDelays) {
			return err
		}
		time.Sleep(billingTransactionConflictRetryDelays[attempt])
	}
}

type transactionalFundingSource interface {
	reserveWithTx(tx *gorm.DB, delta int, phase string) (commit func(), err error)
	settleWithTx(tx *gorm.DB, delta int) (commit func(), err error)
	refundWithTx(tx *gorm.DB) (commit func(), err error)
}

type tokenReservationError struct {
	err error
}

func (e *tokenReservationError) Error() string { return e.err.Error() }
func (e *tokenReservationError) Unwrap() error { return e.err }

// Settle 根据实际消耗额度进行结算。标准钱包/订阅资金源与令牌额度在
// 同一事务提交；兼容型外部 FundingSource 仍使用可补偿的旧接口。
func (s *BillingSession) Settle(actualQuota int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.refunded {
		return ErrBillingSessionRefunded
	}
	if s.settled {
		return nil
	}
	delta := actualQuota - s.preConsumedQuota
	if funding, ok := s.funding.(transactionalFundingSource); ok {
		return s.settleTransactionalLocked(delta, funding)
	}
	if delta == 0 {
		if err := s.funding.Settle(0); err != nil {
			return err
		}
		s.fundingSettled = true
		s.settled = true
		return nil
	}
	// Adjust token accounting before committing the funding source. If the
	// funding commit then fails, undo the token change so both ledgers remain at
	// their pre-settlement reservation.
	tokenAdjusted := false
	if !s.relayInfo.IsPlayground {
		var tokenErr error
		if delta > 0 {
			tokenErr = decreaseBillingTokenQuota(s.relayInfo.TokenId, s.relayInfo.TokenKey, delta)
		} else {
			tokenErr = increaseBillingTokenQuota(s.relayInfo.TokenId, s.relayInfo.TokenKey, -delta)
		}
		if tokenErr != nil {
			return tokenErr
		}
		tokenAdjusted = true
	}

	var subscriptionBefore int64
	if sub, ok := s.funding.(*SubscriptionFunding); ok {
		subscriptionBefore = sub.preConsumed
	}
	if err := s.funding.Settle(delta); err != nil {
		if tokenAdjusted {
			var rollbackErr error
			if delta > 0 {
				rollbackErr = increaseBillingTokenQuota(s.relayInfo.TokenId, s.relayInfo.TokenKey, delta)
			} else {
				rollbackErr = decreaseBillingTokenQuota(s.relayInfo.TokenId, s.relayInfo.TokenKey, -delta)
			}
			if rollbackErr != nil {
				common.SysLog(fmt.Sprintf("error rolling back token quota after funding settlement failed (userId=%d, tokenId=%d, delta=%d): %s",
					s.relayInfo.UserId, s.relayInfo.TokenId, delta, rollbackErr.Error()))
				return fmt.Errorf("%w (token rollback failed: %v)", err, rollbackErr)
			}
		}
		return err
	}
	s.fundingSettled = true
	if s.funding.Source() == BillingSourceSubscription {
		if sub, ok := s.funding.(*SubscriptionFunding); ok {
			s.relayInfo.SubscriptionPostDelta += sub.preConsumed - subscriptionBefore
		}
	}
	s.tokenConsumed += delta
	if s.tokenConsumed < 0 {
		s.tokenConsumed = 0
	}
	s.settled = true
	return nil
}

func (s *BillingSession) settleTransactionalLocked(delta int, funding transactionalFundingSource) error {
	var commitFunding func()
	subscriptionBefore := int64(0)
	if sub, ok := s.funding.(*SubscriptionFunding); ok {
		subscriptionBefore = sub.preConsumed
	}

	err := runBillingTransactionWithRetry(func() error {
		commitFunding = nil
		return model.DB.Transaction(func(tx *gorm.DB) error {
			var err error
			commitFunding, err = funding.settleWithTx(tx, delta)
			if err != nil {
				return err
			}
			if s.relayInfo.IsPlayground || delta == 0 {
				return nil
			}
			return model.AdjustTokenQuotaWithTx(tx, s.relayInfo.TokenId, -delta)
		})
	})
	if err != nil {
		return err
	}
	if commitFunding != nil {
		commitFunding()
	}
	if !s.relayInfo.IsPlayground {
		model.SyncTokenQuotaCacheDelta(s.relayInfo.TokenKey, -delta)
	}
	s.fundingSettled = true
	if sub, ok := s.funding.(*SubscriptionFunding); ok {
		s.relayInfo.SubscriptionPostDelta += sub.preConsumed - subscriptionBefore
	}
	s.tokenConsumed += delta
	if s.tokenConsumed < 0 {
		s.tokenConsumed = 0
	}
	s.settled = true
	return nil
}

// Refund preserves the BillingSettler compatibility contract while surfacing
// the terminal retry failure through RefundWithError for controllers that can
// persist or alert on recovery work.
func (s *BillingSession) Refund(c *gin.Context) {
	if err := s.RefundWithError(c); err != nil {
		common.SysLog("error refunding billing session: " + err.Error())
	}
}

// RefundWithError synchronously retries the durable refund and marks the
// session only after both funding and token quota have committed. A failed
// refund remains retryable on the next call.
func (s *BillingSession) RefundWithError(c *gin.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.settled || s.refunded || !s.needsRefundLocked() {
		return nil
	}

	logger.LogInfo(c, fmt.Sprintf("用户 %d 请求失败, 返还预扣费（token_quota=%s, funding=%s）",
		s.relayInfo.UserId,
		logger.FormatQuota(s.tokenConsumed),
		s.funding.Source(),
	))

	var err error
	if funding, ok := s.funding.(transactionalFundingSource); ok {
		err = refundWithRetry(func() error {
			var commitFunding func()
			txErr := model.DB.Transaction(func(tx *gorm.DB) error {
				var fundingErr error
				commitFunding, fundingErr = funding.refundWithTx(tx)
				if fundingErr != nil {
					return fundingErr
				}
				if s.tokenConsumed <= 0 || s.relayInfo.IsPlayground {
					return nil
				}
				return model.AdjustTokenQuotaWithTx(tx, s.relayInfo.TokenId, s.tokenConsumed)
			})
			if txErr != nil {
				return txErr
			}
			if commitFunding != nil {
				commitFunding()
			}
			if s.tokenConsumed > 0 && !s.relayInfo.IsPlayground {
				model.SyncTokenQuotaCacheDelta(s.relayInfo.TokenKey, s.tokenConsumed)
			}
			return nil
		})
	} else {
		err = refundWithRetry(s.funding.Refund)
		if err == nil && s.tokenConsumed > 0 && !s.relayInfo.IsPlayground {
			err = refundWithRetry(func() error {
				return increaseBillingTokenQuota(s.relayInfo.TokenId, s.relayInfo.TokenKey, s.tokenConsumed)
			})
		}
	}
	if err != nil {
		return err
	}
	s.refunded = true
	s.tokenConsumed = 0
	return nil
}

// NeedsRefund 返回是否存在需要退还的预扣状态。
func (s *BillingSession) NeedsRefund() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.needsRefundLocked()
}

func (s *BillingSession) needsRefundLocked() bool {
	if s.settled || s.refunded || s.fundingSettled {
		// fundingSettled 时资金来源已提交结算，不能再退预扣费
		return false
	}
	if s.tokenConsumed > 0 {
		return true
	}
	// 订阅可能在 tokenConsumed=0 时仍预扣了额度（含混合预扣的钱包溢出部分）
	if sub, ok := s.funding.(*SubscriptionFunding); ok && (sub.preConsumed > 0 || sub.walletOverflow > 0) {
		return true
	}
	return false
}

// GetPreConsumedQuota 返回实际预扣的额度。
func (s *BillingSession) GetPreConsumedQuota() int {
	return s.preConsumedQuota
}

// PreserveReservationAfterDelivery commits the amount already reserved when a
// response has reached the client but a later supplemental charge fails.
func (s *BillingSession) PreserveReservationAfterDelivery() {
	if err := s.PreserveReservationAfterDeliveryWithError(); err != nil {
		common.SysLog("error preserving delivered billing reservation: " + err.Error())
	}
}

func (s *BillingSession) PreserveReservationAfterDeliveryWithError() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.refunded {
		return ErrBillingSessionRefunded
	}
	if s.settled {
		return nil
	}
	if funding, ok := s.funding.(transactionalFundingSource); ok {
		return s.settleTransactionalLocked(0, funding)
	}
	if err := s.funding.Settle(0); err != nil {
		return err
	}
	s.fundingSettled = true
	s.settled = true
	return nil
}

func (s *BillingSession) Reserve(targetQuota int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if targetQuota <= s.preConsumedQuota {
		return nil
	}
	if s.settled {
		return ErrBillingSessionSettled
	}
	if s.refunded {
		return ErrBillingSessionRefunded
	}

	delta := targetQuota - s.preConsumedQuota
	if delta <= 0 {
		return nil
	}
	s.trusted = false

	if funding, ok := s.funding.(transactionalFundingSource); ok {
		if err := s.reserveTransactionalLocked(delta, "reserve", funding); err != nil {
			return s.wrapReservationError(err)
		}
	} else {
		if err := s.reserveFunding(delta); err != nil {
			return err
		}
		if err := s.reserveToken(delta); err != nil {
			s.rollbackFundingReserve(delta)
			return err
		}
	}

	s.preConsumedQuota += delta
	s.tokenConsumed += delta
	s.syncRelayInfo()
	return nil
}

func (s *BillingSession) reserveTransactionalLocked(delta int, phase string, funding transactionalFundingSource) error {
	if delta <= 0 {
		return nil
	}
	var commitFunding func()
	err := runBillingTransactionWithRetry(func() error {
		commitFunding = nil
		return model.DB.Transaction(func(tx *gorm.DB) error {
			var fundingErr error
			commitFunding, fundingErr = funding.reserveWithTx(tx, delta, phase)
			if fundingErr != nil {
				return fundingErr
			}
			if s.relayInfo.IsPlayground {
				return nil
			}
			if tokenErr := s.reserveTokenWithTx(tx, delta); tokenErr != nil {
				return &tokenReservationError{err: tokenErr}
			}
			return nil
		})
	})
	if err != nil {
		return err
	}
	if commitFunding != nil {
		commitFunding()
	}
	if !s.relayInfo.IsPlayground {
		model.SyncTokenQuotaCacheDelta(s.relayInfo.TokenKey, -delta)
	}
	return nil
}

func (s *BillingSession) reserveTokenWithTx(tx *gorm.DB, delta int) error {
	var token model.Token
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Select("id", "remain_quota").
		Where("id = ?", s.relayInfo.TokenId).
		Take(&token).Error; err != nil {
		return err
	}
	if !s.relayInfo.TokenUnlimited && token.RemainQuota < delta {
		return fmt.Errorf("token quota is not enough, token remain quota: %s, need quota: %s",
			logger.FormatQuota(token.RemainQuota), logger.FormatQuota(delta))
	}
	return model.AdjustTokenQuotaWithTx(tx, s.relayInfo.TokenId, -delta)
}

func (s *BillingSession) wrapReservationError(err error) error {
	var tokenErr *tokenReservationError
	if errors.As(err, &tokenErr) {
		return types.NewErrorWithStatusCode(tokenErr.err, types.ErrorCodePreConsumeTokenQuotaFailed, http.StatusForbidden,
			types.ErrOptionWithSkipRetry(), types.ErrOptionWithNoRecordErrorLog())
	}
	if _, ok := s.funding.(*SubscriptionFunding); ok {
		return types.NewErrorWithStatusCode(
			fmt.Errorf("订阅额度不足或未配置订阅: %s", err.Error()),
			types.ErrorCodeInsufficientUserQuota,
			http.StatusForbidden,
			types.ErrOptionWithSkipRetry(),
			types.ErrOptionWithNoRecordErrorLog(),
		)
	}
	return types.NewError(err, types.ErrorCodeUpdateDataError, types.ErrOptionWithSkipRetry())
}

func (s *BillingSession) ReserveToTarget(targetQuota int) error {
	return s.Reserve(targetQuota)
}

// ---------------------------------------------------------------------------
// PreConsume — 统一预扣费入口（含信任额度旁路）
// ---------------------------------------------------------------------------

// preConsume 执行预扣费：信任检查 -> 令牌预扣 -> 资金来源预扣。
// 任一步骤失败时原子回滚已完成的步骤。
func (s *BillingSession) preConsume(c *gin.Context, quota int) *types.NewAPIError {
	effectiveQuota := quota

	// ---- 信任额度旁路 ----
	if s.shouldTrust(c) {
		s.trusted = true
		effectiveQuota = 0
		logger.LogInfo(c, fmt.Sprintf("用户 %d 额度充足, 信任且不需要预扣费 (funding=%s)", s.relayInfo.UserId, s.funding.Source()))
	} else if effectiveQuota > 0 {
		logger.LogInfo(c, fmt.Sprintf("用户 %d 需要预扣费 %s (funding=%s)", s.relayInfo.UserId, logger.FormatQuota(effectiveQuota), s.funding.Source()))
	}

	if effectiveQuota > 0 {
		if funding, ok := s.funding.(transactionalFundingSource); ok {
			if err := s.reserveTransactionalLocked(effectiveQuota, "pre", funding); err != nil {
				var tokenErr *tokenReservationError
				if errors.As(err, &tokenErr) {
					return types.NewErrorWithStatusCode(tokenErr.err, types.ErrorCodePreConsumeTokenQuotaFailed, http.StatusForbidden,
						types.ErrOptionWithSkipRetry(), types.ErrOptionWithNoRecordErrorLog())
				}
				if errors.Is(err, model.ErrNoActiveSubscription) ||
					errors.Is(err, model.ErrSubscriptionQuotaInsufficient) ||
					errors.Is(err, model.ErrWalletInsufficientBalance) {
					return types.NewErrorWithStatusCode(fmt.Errorf("订阅额度不足或未配置订阅: %s", err.Error()), types.ErrorCodeInsufficientUserQuota, http.StatusForbidden, types.ErrOptionWithSkipRetry(), types.ErrOptionWithNoRecordErrorLog())
				}
				return types.NewError(err, types.ErrorCodeUpdateDataError, types.ErrOptionWithSkipRetry())
			}
			s.tokenConsumed = effectiveQuota
		} else {
			// Compatibility path for external/test FundingSource implementations.
			if err := PreConsumeTokenQuota(s.relayInfo, effectiveQuota); err != nil {
				return types.NewErrorWithStatusCode(err, types.ErrorCodePreConsumeTokenQuotaFailed, http.StatusForbidden, types.ErrOptionWithSkipRetry(), types.ErrOptionWithNoRecordErrorLog())
			}
			s.tokenConsumed = effectiveQuota
			if err := s.funding.PreConsume(effectiveQuota); err != nil {
				if s.tokenConsumed > 0 && !s.relayInfo.IsPlayground {
					if rollbackErr := model.IncreaseTokenQuota(s.relayInfo.TokenId, s.relayInfo.TokenKey, s.tokenConsumed); rollbackErr != nil {
						common.SysLog(fmt.Sprintf("error rolling back token quota (userId=%d, tokenId=%d, amount=%d, fundingErr=%s): %s",
							s.relayInfo.UserId, s.relayInfo.TokenId, s.tokenConsumed, err.Error(), rollbackErr.Error()))
					}
					s.tokenConsumed = 0
				}
				return preConsumeFundingError(err)
			}
		}
	} else if err := s.funding.PreConsume(0); err != nil {
		return preConsumeFundingError(err)
	}

	s.preConsumedQuota = effectiveQuota

	// ---- 同步 RelayInfo 兼容字段 ----
	s.syncRelayInfo()

	return nil
}

func preConsumeFundingError(err error) *types.NewAPIError {
	if errors.Is(err, model.ErrNoActiveSubscription) ||
		errors.Is(err, model.ErrSubscriptionQuotaInsufficient) ||
		errors.Is(err, model.ErrWalletInsufficientBalance) {
		return types.NewErrorWithStatusCode(
			fmt.Errorf("订阅额度不足或未配置订阅: %s", err.Error()),
			types.ErrorCodeInsufficientUserQuota,
			http.StatusForbidden,
			types.ErrOptionWithSkipRetry(),
			types.ErrOptionWithNoRecordErrorLog(),
		)
	}
	return types.NewError(err, types.ErrorCodeUpdateDataError, types.ErrOptionWithSkipRetry())
}

// rollbackSubscriptionAndToken 回滚已完成的订阅预扣和令牌预扣，用于混合预扣失败时的清理。
func (s *BillingSession) rollbackSubscriptionAndToken(sub *SubscriptionFunding) {
	if sub.preConsumed > 0 {
		if err := refundWithRetry(func() error {
			return model.RefundSubscriptionPreConsume(sub.requestId)
		}); err != nil {
			common.SysLog(fmt.Sprintf("error rolling back subscription pre-consume (userId=%d, requestId=%s): %s",
				sub.userId, sub.requestId, err.Error()))
			// 退款失败不重置 preConsumed，保留状态以便后续诊断
		} else {
			sub.preConsumed = 0
		}
	}
	if s.tokenConsumed > 0 && !s.relayInfo.IsPlayground {
		if err := model.IncreaseTokenQuota(s.relayInfo.TokenId, s.relayInfo.TokenKey, s.tokenConsumed); err != nil {
			common.SysLog(fmt.Sprintf("error rolling back token quota (userId=%d, tokenId=%d, amount=%d): %s",
				s.relayInfo.UserId, s.relayInfo.TokenId, s.tokenConsumed, err.Error()))
		}
		s.tokenConsumed = 0
	}
}

func (s *BillingSession) reserveFunding(delta int) error {
	switch funding := s.funding.(type) {
	case *WalletFunding:
		if err := funding.adjust(-delta, "reserve"); err != nil {
			return types.NewError(err, types.ErrorCodeUpdateDataError, types.ErrOptionWithSkipRetry())
		}
		funding.consumed += delta
		return nil
	case *SubscriptionFunding:
		target := funding.preConsumed + int64(funding.walletOverflow) + int64(delta)
		if err := funding.setTarget(target); err != nil {
			return types.NewErrorWithStatusCode(
				fmt.Errorf("订阅额度不足或未配置订阅: %s", err.Error()),
				types.ErrorCodeInsufficientUserQuota,
				http.StatusForbidden,
				types.ErrOptionWithSkipRetry(),
				types.ErrOptionWithNoRecordErrorLog(),
			)
		}
		return nil
	default:
		return types.NewError(fmt.Errorf("unsupported funding source: %s", s.funding.Source()), types.ErrorCodeUpdateDataError, types.ErrOptionWithSkipRetry())
	}
}

func (s *BillingSession) rollbackFundingReserve(delta int) {
	switch funding := s.funding.(type) {
	case *WalletFunding:
		if err := funding.adjust(delta, "reserve_rollback"); err != nil {
			common.SysLog("error rolling back wallet funding reserve: " + err.Error())
		} else {
			funding.consumed -= delta
		}
	case *SubscriptionFunding:
		current := funding.preConsumed + int64(funding.walletOverflow)
		target := current - int64(delta)
		if target < 0 {
			target = 0
		}
		if err := funding.setTarget(target); err != nil {
			common.SysLog("error rolling back subscription funding reserve: " + err.Error())
		}
	}
}

func (s *BillingSession) reserveToken(delta int) error {
	if delta <= 0 || s.relayInfo.IsPlayground {
		return nil
	}
	if err := PreConsumeTokenQuota(s.relayInfo, delta); err != nil {
		return types.NewErrorWithStatusCode(err, types.ErrorCodePreConsumeTokenQuotaFailed, http.StatusForbidden, types.ErrOptionWithSkipRetry(), types.ErrOptionWithNoRecordErrorLog())
	}
	return nil
}

// shouldTrust 统一信任额度检查，适用于钱包和订阅。
func (s *BillingSession) shouldTrust(c *gin.Context) bool {
	// 异步任务（ForcePreConsume=true）必须预扣全额，不允许信任旁路
	if s.relayInfo.ForcePreConsume {
		return false
	}

	trustQuota := common.GetTrustQuota()
	if trustQuota <= 0 {
		return false
	}

	// 检查令牌是否充足
	tokenTrusted := s.relayInfo.TokenUnlimited
	if !tokenTrusted {
		tokenQuota := c.GetInt("token_quota")
		tokenTrusted = tokenQuota > trustQuota
	}
	if !tokenTrusted {
		return false
	}

	switch s.funding.Source() {
	case BillingSourceWallet:
		return s.relayInfo.UserQuota > int64(trustQuota)
	case BillingSourceSubscription:
		// 订阅不能启用信任旁路。原因：
		// 1. PreConsumeUserSubscription 要求 amount>0 来创建预扣记录并锁定订阅
		// 2. SubscriptionFunding.PreConsume 忽略参数，始终用 s.amount 预扣
		// 3. 若信任旁路将 effectiveQuota 设为 0，会导致 preConsumedQuota 与实际订阅预扣不一致
		return false
	default:
		return false
	}
}

// syncRelayInfo 将 BillingSession 的状态同步到 RelayInfo 的兼容字段上。
func (s *BillingSession) syncRelayInfo() {
	info := s.relayInfo
	info.FinalPreConsumedQuota = s.preConsumedQuota
	info.BillingSource = s.funding.Source()

	if sub, ok := s.funding.(*SubscriptionFunding); ok {
		info.SubscriptionId = sub.subscriptionId
		info.SubscriptionPreConsumed = sub.preConsumed
		info.SubscriptionPostDelta = 0
		info.SubscriptionAmountTotal = sub.AmountTotal
		info.SubscriptionAmountUsedAfterPreConsume = sub.AmountUsedAfter
		info.SubscriptionPlanId = sub.PlanId
		info.SubscriptionPlanTitle = sub.PlanTitle
	} else {
		info.SubscriptionId = 0
		info.SubscriptionPreConsumed = 0
	}
}

// ---------------------------------------------------------------------------
// NewBillingSession 工厂 — 根据计费偏好创建会话并处理回退
// ---------------------------------------------------------------------------

// NewBillingSession 根据用户计费偏好创建 BillingSession，处理 subscription_first / wallet_first 的回退。
func NewBillingSession(c *gin.Context, relayInfo *relaycommon.RelayInfo, preConsumedQuota int) (*BillingSession, *types.NewAPIError) {
	if relayInfo == nil {
		return nil, types.NewError(fmt.Errorf("relayInfo is nil"), types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
	}

	pref := common.NormalizeBillingPreference(relayInfo.UserSetting.BillingPreference)
	eligibility := SubscriptionEligibilityForGroups(ResolveRequestEffectiveGroups(relayInfo.UserGroup, relayInfo.UsingGroup))

	// 钱包路径需要先检查用户额度
	tryWallet := func() (*BillingSession, *types.NewAPIError) {
		userQuota, err := model.GetUserQuota(relayInfo.UserId, false)
		if err != nil {
			return nil, types.NewError(err, types.ErrorCodeQueryDataError, types.ErrOptionWithSkipRetry())
		}
		if preConsumedQuota > 0 && userQuota <= 0 {
			return nil, types.NewErrorWithStatusCode(
				fmt.Errorf("用户额度不足, 剩余额度: %s", logger.FormatQuota64(userQuota)),
				types.ErrorCodeInsufficientUserQuota, http.StatusForbidden,
				types.ErrOptionWithSkipRetry(), types.ErrOptionWithNoRecordErrorLog())
		}
		if userQuota-int64(preConsumedQuota) < 0 {
			return nil, types.NewErrorWithStatusCode(
				fmt.Errorf("预扣费额度失败, 用户剩余额度: %s, 需要预扣费额度: %s", logger.FormatQuota64(userQuota), logger.FormatQuota(preConsumedQuota)),
				types.ErrorCodeInsufficientUserQuota, http.StatusForbidden,
				types.ErrOptionWithSkipRetry(), types.ErrOptionWithNoRecordErrorLog())
		}
		relayInfo.UserQuota = userQuota

		session := &BillingSession{
			relayInfo: relayInfo,
			funding:   NewWalletFunding(relayInfo.UserId, relayInfo.RequestId),
		}
		if apiErr := session.preConsume(c, preConsumedQuota); apiErr != nil {
			return nil, apiErr
		}
		return session, nil
	}

	trySubscription := func(permitOverflow bool) (*BillingSession, *types.NewAPIError) {
		subConsume := int64(preConsumedQuota)
		session := &BillingSession{
			relayInfo: relayInfo,
			funding: &SubscriptionFunding{
				requestId:      relayInfo.RequestId,
				userId:         relayInfo.UserId,
				modelName:      relayInfo.OriginModelName,
				amount:         subConsume,
				eligibility:    eligibility,
				permitOverflow: permitOverflow,
			},
		}
		if apiErr := session.preConsume(c, int(subConsume)); apiErr != nil {
			return nil, apiErr
		}
		return session, nil
	}

	switch pref {
	case "subscription_only":
		return trySubscription(false)
	case "wallet_only":
		return tryWallet()
	case "wallet_first":
		session, err := tryWallet()
		if err != nil {
			if err.GetErrorCode() == types.ErrorCodeInsufficientUserQuota {
				return trySubscription(false)
			}
			return nil, err
		}
		return session, nil
	case "subscription_first":
		fallthrough
	default:
		hasSub, subCheckErr := model.HasActiveUserSubscriptionByEligibility(relayInfo.UserId, eligibility)
		if subCheckErr != nil {
			return nil, types.NewError(subCheckErr, types.ErrorCodeQueryDataError, types.ErrOptionWithSkipRetry())
		}
		if !hasSub {
			return tryWallet()
		}
		session, apiErr := trySubscription(true)
		if apiErr != nil {
			if apiErr.GetErrorCode() == types.ErrorCodeInsufficientUserQuota {
				// 仅当用户的活跃订阅允许钱包回退时才回退到钱包，否则返回订阅额度不足错误
				allowOverflow, overflowErr := model.UserActiveSubscriptionsAllowWalletOverflowByEligibility(relayInfo.UserId, eligibility)
				if overflowErr != nil {
					return nil, types.NewError(overflowErr, types.ErrorCodeQueryDataError, types.ErrOptionWithSkipRetry())
				}
				if allowOverflow {
					return tryWallet()
				}
				return nil, apiErr
			}
			return nil, apiErr
		}
		return session, nil
	}
}
