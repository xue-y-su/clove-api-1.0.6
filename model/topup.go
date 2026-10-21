package model

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type TopUp struct {
	Id              int     `json:"id"`
	UserId          int     `json:"user_id" gorm:"index"`
	Amount          int64   `json:"amount"`
	Money           float64 `json:"money"`
	TradeNo         string  `json:"trade_no" gorm:"unique;type:varchar(255);index"`
	PaymentMethod   string  `json:"payment_method" gorm:"type:varchar(50)"`
	PaymentProvider string  `json:"payment_provider" gorm:"type:varchar(50);default:''"`
	ProviderStoreID string  `json:"provider_store_id,omitempty" gorm:"type:varchar(128);not null;default:''"`
	EligibleQuota   int64   `json:"eligible_quota" gorm:"type:bigint;not null;default:0"`
	PaidAmount      string  `json:"paid_amount" gorm:"type:varchar(64);not null;default:''"`
	PaidCurrency    string  `json:"paid_currency" gorm:"type:varchar(16);not null;default:''"`
	PayerEmail      string  `json:"-" gorm:"type:varchar(320);not null;default:''"`
	PayerName       string  `json:"-" gorm:"type:varchar(255);not null;default:''"`
	CreateTime      int64   `json:"create_time"`
	CompleteTime    int64   `json:"complete_time"`
	Status          string  `json:"status"`
}

const (
	PaymentMethodStripe       = "stripe"
	PaymentMethodCreem        = "creem"
	PaymentMethodWaffo        = "waffo"
	PaymentMethodWaffoPancake = "waffo_pancake"
	PaymentMethodBalance      = "balance"
)

const (
	PaymentProviderEpay         = "epay"
	PaymentProviderStripe       = "stripe"
	PaymentProviderCreem        = "creem"
	PaymentProviderWaffo        = "waffo"
	PaymentProviderWaffoPancake = "waffo_pancake"
	PaymentProviderBalance      = "balance"
)

var (
	ErrPaymentMethodMismatch = errors.New("payment method mismatch")
	ErrTopUpNotFound         = errors.New("topup not found")
	ErrTopUpStatusInvalid    = errors.New("topup status invalid")
	ErrCashPaymentInvalid    = errors.New("cash payment snapshot is invalid")
)

type CashPaymentSettlement struct {
	PaidAmount   string
	PaidCurrency string
}

type PaymentCommissionReversalInput struct {
	TradeNo          string
	ExpectedProvider string
	RefundSourceType string
	RefundSourceID   string
	RefundedAmount   string
	RefundedCurrency string
}

func CalculateCashPaymentEligibleQuota(paidAmount, currencyUnitsPerUSD string) (int64, error) {
	amount, err := decimal.NewFromString(strings.TrimSpace(paidAmount))
	if err != nil || !amount.IsPositive() {
		return 0, ErrCashPaymentInvalid
	}
	rate, err := decimal.NewFromString(strings.TrimSpace(currencyUnitsPerUSD))
	if err != nil || !rate.IsPositive() {
		return 0, ErrCashPaymentInvalid
	}
	quota := amount.Div(rate).Mul(decimal.NewFromFloat(common.QuotaPerUnit)).Truncate(0)
	value := quota.BigInt()
	if value.Sign() <= 0 || value.Cmp(big.NewInt(math.MaxInt64)) > 0 {
		return 0, ErrCashPaymentInvalid
	}
	return value.Int64(), nil
}

func CalculateCashPaymentEligibleQuotaForCurrency(paidAmount, currency string) (int64, error) {
	currency = strings.ToUpper(strings.TrimSpace(currency))
	rate := decimal.Zero
	switch currency {
	case "USD":
		rate = decimal.NewFromInt(1)
	case "CNY":
		if math.IsNaN(operation_setting.Price) || math.IsInf(operation_setting.Price, 0) || operation_setting.Price <= 0 {
			return 0, ErrCashPaymentInvalid
		}
		rate = decimal.NewFromFloat(operation_setting.Price)
	case "CUSTOM":
		customRate := operation_setting.GetGeneralSetting().CustomCurrencyExchangeRate
		if math.IsNaN(customRate) || math.IsInf(customRate, 0) || customRate <= 0 {
			return 0, ErrCashPaymentInvalid
		}
		rate = decimal.NewFromFloat(customRate)
	default:
		return 0, ErrCashPaymentInvalid
	}
	return CalculateCashPaymentEligibleQuota(paidAmount, rate.String())
}

func ReversePaymentInvitationCommission(input PaymentCommissionReversalInput) (bool, error) {
	input.TradeNo = strings.TrimSpace(input.TradeNo)
	input.ExpectedProvider = strings.TrimSpace(input.ExpectedProvider)
	input.RefundSourceType = strings.TrimSpace(input.RefundSourceType)
	input.RefundSourceID = strings.TrimSpace(input.RefundSourceID)
	input.RefundedAmount = strings.TrimSpace(input.RefundedAmount)
	input.RefundedCurrency = strings.ToUpper(strings.TrimSpace(input.RefundedCurrency))
	if DB == nil || input.TradeNo == "" || input.ExpectedProvider == "" || input.RefundSourceType == "" ||
		input.RefundSourceID == "" || input.RefundedAmount == "" {
		return false, ErrCashPaymentInvalid
	}

	created := false
	recipientUserID := 0
	err := DB.Transaction(func(tx *gorm.DB) error {
		var topUp TopUp
		if err := LockTopUpByTradeNo(tx, input.TradeNo, &topUp); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrTopUpNotFound
			}
			return err
		}
		if topUp.PaymentProvider != input.ExpectedProvider {
			return ErrPaymentMethodMismatch
		}
		if topUp.Status != common.TopUpStatusSuccess {
			return ErrTopUpStatusInvalid
		}
		var err error
		created, recipientUserID, err = reversePaymentInvitationCommissionTx(tx, &topUp, input)
		return err
	})
	if err != nil {
		return false, err
	}
	if recipientUserID > 0 {
		_ = InvalidateUserCache(recipientUserID)
	}
	return created, nil
}

func reversePaymentInvitationCommissionTx(tx *gorm.DB, topUp *TopUp, input PaymentCommissionReversalInput) (bool, int, error) {
	if tx == nil || topUp == nil {
		return false, 0, ErrCashPaymentInvalid
	}
	triggerID := strings.TrimSpace(input.TradeNo)
	if len(triggerID) > 191 {
		triggerID = "sha1:" + common.Sha1([]byte(triggerID))
	}
	var reward InviteRewardPayout
	err := lockForUpdate(tx).Where(
		"trigger_type IN ? AND trigger_id = ? AND recipient_kind = ?",
		[]string{InviteRewardTriggerCashTopUp, InviteRewardTriggerCashSubscription}, triggerID, InviteRewardRecipientInviter,
	).First(&reward).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, 0, nil
	}
	if err != nil {
		return false, 0, err
	}
	paidCurrency := strings.ToUpper(strings.TrimSpace(topUp.PaidCurrency))
	refundedCurrency := strings.ToUpper(strings.TrimSpace(input.RefundedCurrency))
	if refundedCurrency == "" {
		refundedCurrency = paidCurrency
	}
	if paidCurrency == "" || paidCurrency != refundedCurrency || reward.EligibleQuota <= 0 {
		return false, 0, ErrCashPaymentInvalid
	}
	paidAmount, err := decimal.NewFromString(strings.TrimSpace(topUp.PaidAmount))
	if err != nil || !paidAmount.IsPositive() {
		return false, 0, ErrCashPaymentInvalid
	}
	refundedAmount, err := decimal.NewFromString(strings.TrimSpace(input.RefundedAmount))
	if err != nil || !refundedAmount.IsPositive() {
		return false, 0, ErrCashPaymentInvalid
	}
	refundedBaseValue := decimal.NewFromInt(reward.EligibleQuota).
		Mul(refundedAmount).Div(paidAmount).Truncate(0).BigInt()
	if refundedBaseValue.Sign() <= 0 || !refundedBaseValue.IsInt64() {
		return false, 0, ErrCashPaymentInvalid
	}
	_, created, err := ReverseInvitationCommissionWithTx(tx, InvitationCommissionReversalInput{
		TriggerType: reward.TriggerType, TriggerID: reward.TriggerID,
		RefundSourceType: input.RefundSourceType, RefundSourceID: input.RefundSourceID,
		RefundedBase: refundedBaseValue.Int64(),
	})
	return created, reward.RecipientUserID, err
}

func CompleteCashTopUpWithCommissionTx(tx *gorm.DB, topUp *TopUp, creditedQuota int64, provider string, settlement *CashPaymentSettlement) error {
	if tx == nil || topUp == nil || topUp.Id <= 0 || creditedQuota <= 0 {
		return ErrCashPaymentInvalid
	}
	provider = strings.TrimSpace(provider)
	if _, err := CreditTopUpWithTx(tx, topUp.UserId, creditedQuota, topUp.TradeNo, provider); err != nil {
		return err
	}

	paidAmount := strings.TrimSpace(topUp.PaidAmount)
	paidCurrency := strings.ToUpper(strings.TrimSpace(topUp.PaidCurrency))
	eligibleQuota := topUp.EligibleQuota
	if paidAmount == "" || eligibleQuota <= 0 {
		paidAmount = decimal.NewFromFloat(topUp.Money).StringFixed(2)
		currencyUnitsPerUSD := "1"
		if provider == PaymentProviderEpay {
			currencyUnitsPerUSD = decimal.NewFromFloat(operation_setting.Price).String()
		}
		var err error
		eligibleQuota, err = CalculateCashPaymentEligibleQuota(paidAmount, currencyUnitsPerUSD)
		if err != nil {
			return err
		}
		if paidCurrency == "" {
			paidCurrency = "USD"
		}
	}

	if settlement != nil && strings.TrimSpace(settlement.PaidAmount) != "" {
		originalAmount, err := decimal.NewFromString(paidAmount)
		if err != nil || !originalAmount.IsPositive() {
			return ErrCashPaymentInvalid
		}
		settledAmount, err := decimal.NewFromString(strings.TrimSpace(settlement.PaidAmount))
		if err != nil || !settledAmount.IsPositive() {
			return ErrCashPaymentInvalid
		}
		settledQuota := decimal.NewFromInt(eligibleQuota).Mul(settledAmount).Div(originalAmount).Truncate(0).BigInt()
		if settledQuota.Sign() <= 0 || !settledQuota.IsInt64() {
			return ErrCashPaymentInvalid
		}
		eligibleQuota = settledQuota.Int64()
		paidAmount = settledAmount.StringFixed(2)
		if currency := strings.ToUpper(strings.TrimSpace(settlement.PaidCurrency)); currency != "" {
			paidCurrency = currency
		}
	}

	if topUp.EligibleQuota != eligibleQuota || topUp.PaidAmount != paidAmount || topUp.PaidCurrency != paidCurrency {
		if err := tx.Model(&TopUp{}).Where("id = ?", topUp.Id).Updates(map[string]any{
			"eligible_quota": eligibleQuota,
			"paid_amount":    paidAmount,
			"paid_currency":  paidCurrency,
		}).Error; err != nil {
			return err
		}
		topUp.EligibleQuota = eligibleQuota
		topUp.PaidAmount = paidAmount
		topUp.PaidCurrency = paidCurrency
	}

	if provider == PaymentProviderBalance || provider == "admin" {
		return nil
	}
	setting := operation_setting.GetInvitationIncentiveSetting()
	if !setting.Enabled || !setting.CashCommissionEnabled {
		return nil
	}
	rateBasisPoints, _, err := operation_setting.ParseInvitationCommissionRate(setting.CashCommissionRate)
	if err != nil {
		return err
	}
	triggerID := strings.TrimSpace(topUp.TradeNo)
	if len(triggerID) > 191 {
		triggerID = "sha1:" + common.Sha1([]byte(triggerID))
	}
	_, _, err = CreateInvitationCommissionWithTx(tx, InvitationCommissionInput{
		InviteeUserID: topUp.UserId, TriggerType: InviteRewardTriggerCashTopUp,
		TriggerID: triggerID, EligibleQuota: eligibleQuota, RateBasisPoints: rateBasisPoints,
	})
	return err
}

func (topUp *TopUp) Insert() error {
	var err error
	err = DB.Create(topUp).Error
	return err
}

func (topUp *TopUp) Update() error {
	var err error
	err = DB.Save(topUp).Error
	return err
}

func GetTopUpById(id int) *TopUp {
	var topUp *TopUp
	var err error
	err = DB.Where("id = ?", id).First(&topUp).Error
	if err != nil {
		return nil
	}
	return topUp
}

func LockTopUpByTradeNo(tx *gorm.DB, tradeNo string, topUp *TopUp) error {
	if topUp == nil {
		return errors.New("topup is nil")
	}
	refCol := "`trade_no`"
	if common.UsingMainDatabase(common.DatabaseTypePostgreSQL) {
		refCol = `"trade_no"`
	}
	return lockForUpdate(tx).Where(refCol+" = ?", tradeNo).First(topUp).Error
}

func FindTopUpByTradeNo(tradeNo string) (*TopUp, error) {
	if strings.TrimSpace(tradeNo) == "" {
		return nil, gorm.ErrRecordNotFound
	}
	var topUp TopUp
	if err := DB.Where("trade_no = ?", tradeNo).First(&topUp).Error; err != nil {
		return nil, err
	}
	return &topUp, nil
}

func GetTopUpByTradeNo(tradeNo string) *TopUp {
	topUp, _ := FindTopUpByTradeNo(tradeNo)
	return topUp
}

func UpdatePendingTopUpStatus(tradeNo string, expectedPaymentProvider string, targetStatus string) error {
	if tradeNo == "" {
		return errors.New("未提供支付单号")
	}

	refCol := "`trade_no`"
	if common.UsingMainDatabase(common.DatabaseTypePostgreSQL) {
		refCol = `"trade_no"`
	}

	return DB.Transaction(func(tx *gorm.DB) error {
		topUp := &TopUp{}
		if err := lockForUpdate(tx).Where(refCol+" = ?", tradeNo).First(topUp).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrTopUpNotFound
			}
			return err
		}
		if expectedPaymentProvider != "" && topUp.PaymentProvider != expectedPaymentProvider {
			return ErrPaymentMethodMismatch
		}
		if topUp.Status != common.TopUpStatusPending {
			return ErrTopUpStatusInvalid
		}

		topUp.Status = targetStatus
		return tx.Save(topUp).Error
	})
}

func Recharge(referenceId string, customerId string, callerIp string) error {
	return RechargeWithSettlement(referenceId, customerId, callerIp, nil)
}

func RechargeWithSettlement(referenceId string, customerId string, callerIp string, settlement *CashPaymentSettlement) (err error) {
	if referenceId == "" {
		return errors.New("未提供支付单号")
	}

	var quota int
	completed := false
	topUp := &TopUp{}

	refCol := "`trade_no`"
	if common.UsingMainDatabase(common.DatabaseTypePostgreSQL) {
		refCol = `"trade_no"`
	}

	err = DB.Transaction(func(tx *gorm.DB) error {
		err := lockForUpdate(tx).Where(refCol+" = ?", referenceId).First(topUp).Error
		if err != nil {
			return errors.New("充值订单不存在")
		}

		if topUp.PaymentProvider != PaymentProviderStripe {
			return ErrPaymentMethodMismatch
		}

		if topUp.Status == common.TopUpStatusSuccess {
			return nil
		}

		if topUp.Status != common.TopUpStatusPending {
			return errors.New("充值订单状态错误")
		}

		dAmount := decimal.NewFromInt(topUp.Amount)
		dQuotaPerUnit := decimal.NewFromFloat(common.QuotaPerUnit)
		quota, err = common.QuotaFromDecimalStrict(dAmount.Mul(dQuotaPerUnit).Truncate(0))
		if err != nil || quota <= 0 {
			return errors.New("无效的充值额度")
		}

		topUp.CompleteTime = common.GetTimestamp()
		topUp.Status = common.TopUpStatusSuccess
		err = tx.Save(topUp).Error
		if err != nil {
			return err
		}

		if customerId != "" {
			if err := tx.Model(&User{}).Where("id = ?", topUp.UserId).Update("stripe_customer", customerId).Error; err != nil {
				return err
			}
		}
		if err := CompleteCashTopUpWithCommissionTx(tx, topUp, int64(quota), PaymentProviderStripe, settlement); err != nil {
			return err
		}
		completed = true
		return nil
	})

	if err != nil {
		common.SysError("topup failed: " + err.Error())
		return errors.New("充值失败，请稍后重试")
	}
	if completed {
		_ = InvalidateUserCache(topUp.UserId)
		RecordTopupLog(topUp.UserId, fmt.Sprintf("使用在线充值成功，充值金额: %v，支付金额：%d", logger.FormatQuota(quota), topUp.Amount), callerIp, topUp.PaymentMethod, PaymentMethodStripe)
	}

	return nil
}

// topUpQueryWindowSeconds 限制充值记录查询的时间窗口（秒）。
const topUpQueryWindowSeconds int64 = 30 * 24 * 60 * 60

// topUpQueryCutoff 返回允许查询的最早 create_time（秒级 Unix 时间戳）。
func topUpQueryCutoff() int64 {
	return common.GetTimestamp() - topUpQueryWindowSeconds
}

func GetUserTopUps(userId int, pageInfo *common.PageInfo) (topups []*TopUp, total int64, err error) {
	// Start transaction
	tx := DB.Begin()
	if tx.Error != nil {
		return nil, 0, tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	cutoff := topUpQueryCutoff()

	// Get total count within transaction
	err = tx.Model(&TopUp{}).Where("user_id = ? AND create_time >= ?", userId, cutoff).Count(&total).Error
	if err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	// Get paginated topups within same transaction
	err = tx.Where("user_id = ? AND create_time >= ?", userId, cutoff).Order("id desc").Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Find(&topups).Error
	if err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	// Commit transaction
	if err = tx.Commit().Error; err != nil {
		return nil, 0, err
	}

	return topups, total, nil
}

// GetAllTopUps 获取全平台的充值记录（管理员使用，不限制时间窗口）
func GetAllTopUps(pageInfo *common.PageInfo) (topups []*TopUp, total int64, err error) {
	tx := DB.Begin()
	if tx.Error != nil {
		return nil, 0, tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err = tx.Model(&TopUp{}).Count(&total).Error; err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	if err = tx.Order("id desc").Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Find(&topups).Error; err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	if err = tx.Commit().Error; err != nil {
		return nil, 0, err
	}

	return topups, total, nil
}

// searchTopUpCountHardLimit 搜索充值记录时 COUNT 的安全上限，
// 防止对超大表执行无界 COUNT 触发 DoS。
const searchTopUpCountHardLimit = 10000

// SearchUserTopUps 按订单号搜索某用户的充值记录
func SearchUserTopUps(userId int, keyword string, pageInfo *common.PageInfo) (topups []*TopUp, total int64, err error) {
	tx := DB.Begin()
	if tx.Error != nil {
		return nil, 0, tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	query := tx.Model(&TopUp{}).Where("user_id = ? AND create_time >= ?", userId, topUpQueryCutoff())
	if keyword != "" {
		pattern, perr := sanitizeLikePattern(keyword)
		if perr != nil {
			tx.Rollback()
			return nil, 0, perr
		}
		query = query.Where("trade_no LIKE ? ESCAPE '!'", pattern)
	}

	if err = query.Limit(searchTopUpCountHardLimit).Count(&total).Error; err != nil {
		tx.Rollback()
		common.SysError("failed to count search topups: " + err.Error())
		return nil, 0, errors.New("搜索充值记录失败")
	}

	if err = query.Order("id desc").Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Find(&topups).Error; err != nil {
		tx.Rollback()
		common.SysError("failed to search topups: " + err.Error())
		return nil, 0, errors.New("搜索充值记录失败")
	}

	if err = tx.Commit().Error; err != nil {
		return nil, 0, err
	}
	return topups, total, nil
}

// SearchAllTopUps 按订单号搜索全平台充值记录（管理员使用，不限制时间窗口）
func SearchAllTopUps(keyword string, pageInfo *common.PageInfo) (topups []*TopUp, total int64, err error) {
	tx := DB.Begin()
	if tx.Error != nil {
		return nil, 0, tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	query := tx.Model(&TopUp{})
	if keyword != "" {
		pattern, perr := sanitizeLikePattern(keyword)
		if perr != nil {
			tx.Rollback()
			return nil, 0, perr
		}
		query = query.Where("trade_no LIKE ? ESCAPE '!'", pattern)
	}

	if err = query.Limit(searchTopUpCountHardLimit).Count(&total).Error; err != nil {
		tx.Rollback()
		common.SysError("failed to count search topups: " + err.Error())
		return nil, 0, errors.New("搜索充值记录失败")
	}

	if err = query.Order("id desc").Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Find(&topups).Error; err != nil {
		tx.Rollback()
		common.SysError("failed to search topups: " + err.Error())
		return nil, 0, errors.New("搜索充值记录失败")
	}

	if err = tx.Commit().Error; err != nil {
		return nil, 0, err
	}
	return topups, total, nil
}

// ManualCompleteTopUp 管理员手动完成订单并给用户充值
func ManualCompleteTopUp(tradeNo string, callerIp string) error {
	if tradeNo == "" {
		return errors.New("未提供订单号")
	}

	refCol := "`trade_no`"
	if common.UsingMainDatabase(common.DatabaseTypePostgreSQL) {
		refCol = `"trade_no"`
	}

	var userId int
	var quotaToAdd int
	var payMoney float64
	var paymentMethod string
	completed := false

	err := DB.Transaction(func(tx *gorm.DB) error {
		topUp := &TopUp{}
		// 行级锁，避免并发补单
		if err := lockForUpdate(tx).Where(refCol+" = ?", tradeNo).First(topUp).Error; err != nil {
			return errors.New("充值订单不存在")
		}

		// 幂等处理：已成功直接返回
		if topUp.Status == common.TopUpStatusSuccess {
			return nil
		}

		if topUp.Status != common.TopUpStatusPending {
			return errors.New("订单状态不是待支付，无法补单")
		}

		var quotaErr error
		quotaToAdd, quotaErr = cashTopUpCreditedQuota(topUp)
		if quotaErr != nil || quotaToAdd <= 0 {
			return errors.New("无效的充值额度")
		}

		// 标记完成
		topUp.CompleteTime = common.GetTimestamp()
		topUp.Status = common.TopUpStatusSuccess
		if err := tx.Save(topUp).Error; err != nil {
			return err
		}

		if _, err := CreditTopUpWithTx(tx, topUp.UserId, int64(quotaToAdd), topUp.TradeNo, "admin"); err != nil {
			return err
		}

		userId = topUp.UserId
		payMoney = topUp.Money
		paymentMethod = topUp.PaymentMethod
		completed = true
		return nil
	})

	if err != nil {
		return err
	}
	if completed && userId > 0 {
		_ = InvalidateUserCache(userId)
		RecordTopupLog(userId, fmt.Sprintf("管理员补单成功，充值金额: %v，支付金额：%f", logger.FormatQuota(quotaToAdd), payMoney), callerIp, paymentMethod, "admin")
	}
	return nil
}

func cashTopUpCreditedQuota(topUp *TopUp) (int, error) {
	if topUp == nil || topUp.Amount <= 0 {
		return 0, ErrCashPaymentInvalid
	}
	quota := decimal.NewFromInt(topUp.Amount)
	if topUp.PaymentProvider != PaymentProviderCreem {
		quota = quota.Mul(decimal.NewFromFloat(common.QuotaPerUnit)).Truncate(0)
	}
	value, err := common.QuotaFromDecimalStrict(quota)
	if err != nil || value <= 0 {
		return 0, ErrCashPaymentInvalid
	}
	return value, nil
}

func RechargeCreem(referenceId string, customerEmail string, customerName string, callerIp string) error {
	return RechargeCreemWithSettlement(referenceId, customerEmail, customerName, callerIp, nil)
}

func normalizePayerField(value string, maxRunes int) string {
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if len(runes) > maxRunes {
		return string(runes[:maxRunes])
	}
	return value
}

func RechargeCreemWithSettlement(referenceId string, customerEmail string, customerName string, callerIp string, settlement *CashPaymentSettlement) (err error) {
	if referenceId == "" {
		return errors.New("未提供支付单号")
	}

	var quota int
	completed := false
	topUp := &TopUp{}

	refCol := "`trade_no`"
	if common.UsingMainDatabase(common.DatabaseTypePostgreSQL) {
		refCol = `"trade_no"`
	}

	err = DB.Transaction(func(tx *gorm.DB) error {
		err := lockForUpdate(tx).Where(refCol+" = ?", referenceId).First(topUp).Error
		if err != nil {
			return errors.New("充值订单不存在")
		}

		if topUp.PaymentProvider != PaymentProviderCreem {
			return ErrPaymentMethodMismatch
		}

		if topUp.Status == common.TopUpStatusSuccess {
			return nil
		}

		if topUp.Status != common.TopUpStatusPending {
			return errors.New("充值订单状态错误")
		}

		quota, err = common.QuotaFromDecimalStrict(decimal.NewFromInt(topUp.Amount))
		if err != nil || quota <= 0 {
			return errors.New("无效的充值额度")
		}

		topUp.PayerEmail = normalizePayerField(customerEmail, 320)
		topUp.PayerName = normalizePayerField(customerName, 255)
		topUp.CompleteTime = common.GetTimestamp()
		topUp.Status = common.TopUpStatusSuccess
		err = tx.Save(topUp).Error
		if err != nil {
			return err
		}

		if err := CompleteCashTopUpWithCommissionTx(tx, topUp, int64(quota), PaymentProviderCreem, settlement); err != nil {
			return err
		}
		completed = true
		return nil
	})

	if err != nil {
		common.SysError("creem topup failed: " + err.Error())
		return errors.New("充值失败，请稍后重试")
	}
	if completed {
		_ = InvalidateUserCache(topUp.UserId)
		RecordTopupLog(topUp.UserId, fmt.Sprintf("使用Creem充值成功，充值额度: %v，支付金额：%.2f", quota, topUp.Money), callerIp, topUp.PaymentMethod, PaymentMethodCreem)
	}

	return nil
}

func RechargeWaffo(tradeNo string, callerIp string) error {
	return RechargeWaffoWithSettlement(tradeNo, callerIp, nil)
}

func RechargeWaffoWithSettlement(tradeNo string, callerIp string, settlement *CashPaymentSettlement) (err error) {
	if tradeNo == "" {
		return errors.New("未提供支付单号")
	}

	var quotaToAdd int
	topUp := &TopUp{}

	refCol := "`trade_no`"
	if common.UsingMainDatabase(common.DatabaseTypePostgreSQL) {
		refCol = `"trade_no"`
	}

	err = DB.Transaction(func(tx *gorm.DB) error {
		err := lockForUpdate(tx).Where(refCol+" = ?", tradeNo).First(topUp).Error
		if err != nil {
			return errors.New("充值订单不存在")
		}

		if topUp.PaymentProvider != PaymentProviderWaffo {
			return ErrPaymentMethodMismatch
		}

		if topUp.Status == common.TopUpStatusSuccess {
			return nil // 幂等：已成功直接返回
		}

		if topUp.Status != common.TopUpStatusPending {
			return errors.New("充值订单状态错误")
		}

		dAmount := decimal.NewFromInt(topUp.Amount)
		dQuotaPerUnit := decimal.NewFromFloat(common.QuotaPerUnit)
		quotaToAdd, err = common.QuotaFromDecimalStrict(dAmount.Mul(dQuotaPerUnit).Truncate(0))
		if err != nil || quotaToAdd <= 0 {
			return errors.New("无效的充值额度")
		}

		topUp.CompleteTime = common.GetTimestamp()
		topUp.Status = common.TopUpStatusSuccess
		if err := tx.Save(topUp).Error; err != nil {
			return err
		}

		if err := CompleteCashTopUpWithCommissionTx(tx, topUp, int64(quotaToAdd), PaymentProviderWaffo, settlement); err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		common.SysError("waffo topup failed: " + err.Error())
		return errors.New("充值失败，请稍后重试")
	}

	if quotaToAdd > 0 {
		_ = InvalidateUserCache(topUp.UserId)
		RecordTopupLog(topUp.UserId, fmt.Sprintf("Waffo充值成功，充值额度: %v，支付金额: %.2f", logger.FormatQuota(quotaToAdd), topUp.Money), callerIp, topUp.PaymentMethod, PaymentMethodWaffo)
	}

	return nil
}

func RechargeWaffoPancake(tradeNo string, callerIP string) error {
	return RechargeWaffoPancakeWithSettlement(tradeNo, callerIP, nil)
}

func RechargeWaffoPancakeWithSettlement(tradeNo string, callerIP string, settlement *CashPaymentSettlement) (err error) {
	if tradeNo == "" {
		return errors.New("未提供支付单号")
	}

	var quotaToAdd int
	topUp := &TopUp{}

	refCol := "`trade_no`"
	if common.UsingMainDatabase(common.DatabaseTypePostgreSQL) {
		refCol = `"trade_no"`
	}

	err = DB.Transaction(func(tx *gorm.DB) error {
		err := lockForUpdate(tx).Where(refCol+" = ?", tradeNo).First(topUp).Error
		if err != nil {
			return errors.New("充值订单不存在")
		}

		if topUp.PaymentProvider != PaymentProviderWaffoPancake {
			return ErrPaymentMethodMismatch
		}

		if topUp.Status == common.TopUpStatusSuccess {
			return nil
		}

		if topUp.Status != common.TopUpStatusPending {
			return errors.New("充值订单状态错误")
		}

		quotaToAdd, err = common.QuotaFromDecimalStrict(decimal.NewFromInt(topUp.Amount).Mul(decimal.NewFromFloat(common.QuotaPerUnit)).Truncate(0))
		if err != nil || quotaToAdd <= 0 {
			return errors.New("无效的充值额度")
		}

		topUp.CompleteTime = common.GetTimestamp()
		topUp.Status = common.TopUpStatusSuccess
		if err := tx.Save(topUp).Error; err != nil {
			return err
		}

		if err := CompleteCashTopUpWithCommissionTx(tx, topUp, int64(quotaToAdd), PaymentProviderWaffoPancake, settlement); err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		common.SysError("waffo pancake topup failed: " + err.Error())
		return errors.New("充值失败，请稍后重试")
	}

	if quotaToAdd > 0 {
		_ = InvalidateUserCache(topUp.UserId)
		RecordLog(topUp.UserId, LogTypeTopup, fmt.Sprintf("Waffo Pancake充值成功，充值额度: %v，支付金额: %.2f", logger.FormatQuota(quotaToAdd), topUp.Money), callerIP)
	}

	return nil
}
