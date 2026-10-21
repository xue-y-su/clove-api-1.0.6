package service

import (
	"context"
	"errors"
	"math"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"gorm.io/gorm"
)

var ErrTokenBalanceKeyInvalid = errors.New("api key invalid")

const (
	TokenBalanceStatusActive       = "active"
	TokenBalanceStatusDisabled     = "disabled"
	TokenBalanceStatusExpired      = "expired"
	TokenBalanceStatusExhausted    = "exhausted"
	TokenBalanceStatusUserDisabled = "user_disabled"
)

const (
	TokenBalanceRestrictionSubscription = "subscription_quota"
	TokenBalanceRestrictionWallet       = "wallet_quota"
	TokenBalanceRestrictionFunding      = "funding_quota"
	TokenBalanceRestrictionUnavailable  = "key_unavailable"
)

// QuotaAmount represents either a finite quota value or an unlimited value.
// Unlimited values never use a numeric sentinel.
type QuotaAmount struct {
	Unlimited bool  `json:"unlimited"`
	Value     int64 `json:"value"`
}

type TokenBalanceFunding struct {
	Available QuotaAmount `json:"available"`
	Used      int64       `json:"used"`
	Total     QuotaAmount `json:"total"`
}

type TokenBalanceWalletFunding struct {
	Available QuotaAmount `json:"available"`
}

type tokenBalanceSubscriptionInput struct {
	Remaining           int64
	Used                int64
	Total               int64
	Unlimited           bool
	AllowWalletOverflow bool
}

type tokenBalanceCalculationInput struct {
	Preference      string
	TokenUnlimited  bool
	TokenRemaining  int64
	TokenUsed       int64
	WalletRemaining int64
	Subscriptions   []tokenBalanceSubscriptionInput
}

type tokenBalanceCalculation struct {
	TokenUnlimited       bool                       `json:"token_unlimited"`
	TokenTotal           int64                      `json:"token_total"`
	TokenRemaining       int64                      `json:"token_remaining"`
	TokenUsed            int64                      `json:"token_used"`
	CurrentAvailable     QuotaAmount                `json:"current_available"`
	TokenCallableQuota   QuotaAmount                `json:"token_callable_quota"`
	Restricted           int64                      `json:"restricted"`
	RestrictionReason    string                     `json:"restriction_reason,omitempty"`
	SingleRequestLimit   QuotaAmount                `json:"single_request_limit"`
	ParticipatingSources []string                   `json:"participating_sources"`
	Wallet               *TokenBalanceWalletFunding `json:"wallet,omitempty"`
	Subscription         *TokenBalanceFunding       `json:"subscription,omitempty"`
}

type TokenBalanceGroup struct {
	Mode            string   `json:"mode"`
	Name            string   `json:"name,omitempty"`
	EffectiveGroups []string `json:"effective_groups"`
}

type TokenBalanceModelLimits struct {
	Enabled bool     `json:"enabled"`
	Count   int      `json:"count"`
	Models  []string `json:"models,omitempty"`
}

type TokenBalanceDisplay struct {
	Type         string  `json:"type"`
	QuotaPerUnit float64 `json:"quota_per_unit"`
	ExchangeRate float64 `json:"exchange_rate"`
	Symbol       string  `json:"symbol"`
}

type TokenBalanceSnapshot struct {
	TokenID           int                     `json:"-"`
	UserID            int                     `json:"-"`
	MaskedKey         string                  `json:"masked_key"`
	Status            string                  `json:"status"`
	ExpiresAt         int64                   `json:"expires_at"`
	BillingPreference string                  `json:"billing_preference"`
	Group             TokenBalanceGroup       `json:"group"`
	ModelLimits       TokenBalanceModelLimits `json:"model_limits"`
	Balance           tokenBalanceCalculation `json:"balance"`
	Display           TokenBalanceDisplay     `json:"display"`
}

func nonNegative(value int64) int64 {
	if value < 0 {
		return 0
	}
	return value
}

func saturatingAdd(left, right int64) int64 {
	left = nonNegative(left)
	right = nonNegative(right)
	if left > math.MaxInt64-right {
		return math.MaxInt64
	}
	return left + right
}

func addQuota(left, right QuotaAmount) QuotaAmount {
	if left.Unlimited || right.Unlimited {
		return QuotaAmount{Unlimited: true}
	}
	return QuotaAmount{Value: saturatingAdd(left.Value, right.Value)}
}

func maxQuota(left, right QuotaAmount) QuotaAmount {
	if left.Unlimited || right.Unlimited {
		return QuotaAmount{Unlimited: true}
	}
	if right.Value > left.Value {
		return right
	}
	return left
}

func capQuotaByToken(value QuotaAmount, tokenUnlimited bool, tokenRemaining int64) QuotaAmount {
	if tokenUnlimited {
		return value
	}
	tokenRemaining = nonNegative(tokenRemaining)
	if value.Unlimited || value.Value > tokenRemaining {
		return QuotaAmount{Value: tokenRemaining}
	}
	return QuotaAmount{Value: nonNegative(value.Value)}
}

func aggregateSubscriptionFunding(subscriptions []tokenBalanceSubscriptionInput) (TokenBalanceFunding, QuotaAmount, bool) {
	funding := TokenBalanceFunding{}
	singleRequest := QuotaAmount{}
	allowWalletOverflow := true
	for _, subscription := range subscriptions {
		available := QuotaAmount{Unlimited: subscription.Unlimited, Value: nonNegative(subscription.Remaining)}
		total := QuotaAmount{Unlimited: subscription.Unlimited, Value: nonNegative(subscription.Total)}
		funding.Available = addQuota(funding.Available, available)
		funding.Total = addQuota(funding.Total, total)
		funding.Used = saturatingAdd(funding.Used, subscription.Used)
		singleRequest = maxQuota(singleRequest, available)
		if !subscription.AllowWalletOverflow {
			allowWalletOverflow = false
		}
	}
	return funding, singleRequest, allowWalletOverflow
}

func calculateTokenBalance(input tokenBalanceCalculationInput) tokenBalanceCalculation {
	wallet := TokenBalanceWalletFunding{
		Available: QuotaAmount{Value: nonNegative(input.WalletRemaining)},
	}
	subscription, subscriptionSingle, allowWalletOverflow := aggregateSubscriptionFunding(input.Subscriptions)

	result := tokenBalanceCalculation{
		TokenUnlimited: input.TokenUnlimited,
		TokenRemaining: nonNegative(input.TokenRemaining),
		TokenUsed:      nonNegative(input.TokenUsed),
	}
	if !input.TokenUnlimited {
		result.TokenTotal = saturatingAdd(result.TokenRemaining, result.TokenUsed)
	}

	var cumulative QuotaAmount
	var single QuotaAmount
	switch common.NormalizeBillingPreference(input.Preference) {
	case "subscription_only":
		result.ParticipatingSources = []string{BillingSourceSubscription}
		result.Subscription = &subscription
		cumulative = subscription.Available
		single = subscriptionSingle
	case "wallet_only":
		result.ParticipatingSources = []string{BillingSourceWallet}
		result.Wallet = &wallet
		cumulative = wallet.Available
		single = wallet.Available
	case "wallet_first":
		result.ParticipatingSources = []string{BillingSourceWallet}
		result.Wallet = &wallet
		cumulative = wallet.Available
		single = wallet.Available
		if len(input.Subscriptions) > 0 {
			result.ParticipatingSources = append(result.ParticipatingSources, BillingSourceSubscription)
			result.Subscription = &subscription
			cumulative = addQuota(cumulative, subscription.Available)
			single = maxQuota(single, subscriptionSingle)
		}
	case "subscription_first":
		fallthrough
	default:
		if len(input.Subscriptions) == 0 {
			result.ParticipatingSources = []string{BillingSourceWallet}
			result.Wallet = &wallet
			cumulative = wallet.Available
			single = wallet.Available
			break
		}
		result.ParticipatingSources = []string{BillingSourceSubscription}
		result.Subscription = &subscription
		cumulative = subscription.Available
		single = subscriptionSingle
		if allowWalletOverflow {
			result.ParticipatingSources = append(result.ParticipatingSources, BillingSourceWallet)
			result.Wallet = &wallet
			cumulative = addQuota(cumulative, wallet.Available)
			single = maxQuota(single, wallet.Available)
		}
	}

	result.TokenCallableQuota = capQuotaByToken(cumulative, input.TokenUnlimited, result.TokenRemaining)
	result.CurrentAvailable = result.TokenCallableQuota
	result.SingleRequestLimit = capQuotaByToken(single, input.TokenUnlimited, result.TokenRemaining)
	if !input.TokenUnlimited {
		result.Restricted = nonNegative(result.TokenRemaining - result.TokenCallableQuota.Value)
		if result.Restricted > 0 {
			switch {
			case len(result.ParticipatingSources) == 1 && result.ParticipatingSources[0] == BillingSourceSubscription:
				result.RestrictionReason = TokenBalanceRestrictionSubscription
			case len(result.ParticipatingSources) == 1 && result.ParticipatingSources[0] == BillingSourceWallet:
				result.RestrictionReason = TokenBalanceRestrictionWallet
			default:
				result.RestrictionReason = TokenBalanceRestrictionFunding
			}
		}
	}
	return result
}

func normalizeBalanceKey(key string) (string, bool) {
	key = strings.TrimSpace(key)
	if len(key) == 0 || len(key) > 256 {
		return "", false
	}
	return key, key != ""
}

func resolveTokenBalanceStatus(token *model.Token, user *model.UserBase, now int64) string {
	if user == nil || user.Status != common.UserStatusEnabled {
		return TokenBalanceStatusUserDisabled
	}
	if token == nil {
		return TokenBalanceStatusDisabled
	}
	switch token.Status {
	case common.TokenStatusDisabled:
		return TokenBalanceStatusDisabled
	case common.TokenStatusExpired:
		return TokenBalanceStatusExpired
	case common.TokenStatusExhausted:
		return TokenBalanceStatusExhausted
	case common.TokenStatusEnabled:
		// Continue with runtime checks below.
	default:
		return TokenBalanceStatusDisabled
	}
	if token.ExpiredTime != -1 && token.ExpiredTime <= now {
		return TokenBalanceStatusExpired
	}
	if !token.UnlimitedQuota && token.RemainQuota <= 0 {
		return TokenBalanceStatusExhausted
	}
	return TokenBalanceStatusActive
}

func buildTokenBalanceGroup(userGroup, tokenGroup string) TokenBalanceGroup {
	group := TokenBalanceGroup{EffectiveGroups: ResolveTokenEffectiveGroups(userGroup, tokenGroup)}
	switch strings.TrimSpace(tokenGroup) {
	case "":
		group.Mode = "user"
		group.Name = strings.TrimSpace(userGroup)
	case "auto":
		group.Mode = "auto"
	default:
		group.Mode = "fixed"
		group.Name = strings.TrimSpace(tokenGroup)
	}
	return group
}

func buildTokenBalanceModelLimits(token *model.Token) TokenBalanceModelLimits {
	result := TokenBalanceModelLimits{Models: []string{}}
	if token == nil || !token.ModelLimitsEnabled {
		return result
	}
	seen := make(map[string]struct{})
	for _, modelName := range token.GetModelLimits() {
		modelName = strings.TrimSpace(modelName)
		if modelName == "" {
			continue
		}
		seen[modelName] = struct{}{}
	}
	result.Enabled = true
	result.Models = make([]string, 0, len(seen))
	for modelName := range seen {
		result.Models = append(result.Models, modelName)
	}
	sort.Strings(result.Models)
	result.Count = len(result.Models)
	return result
}

func tokenBalanceSubscriptionInputs(subscriptions []model.UserSubscription, now int64) []tokenBalanceSubscriptionInput {
	result := make([]tokenBalanceSubscriptionInput, 0, len(subscriptions))
	for _, subscription := range subscriptions {
		used := nonNegative(subscription.AmountUsed)
		if subscription.NextResetTime > 0 && subscription.NextResetTime <= now {
			used = 0
		}
		unlimited := subscription.AmountTotal == 0
		remaining := int64(0)
		if !unlimited {
			remaining = nonNegative(subscription.AmountTotal - used)
		}
		result = append(result, tokenBalanceSubscriptionInput{
			Remaining:           remaining,
			Used:                used,
			Total:               nonNegative(subscription.AmountTotal),
			Unlimited:           unlimited,
			AllowWalletOverflow: subscription.AllowWalletOverflow,
		})
	}
	return result
}

// BuildTokenBalanceSnapshot returns the minimum redacted data needed by the
// public balance page. It never returns the raw key or user identity fields.
func BuildTokenBalanceSnapshot(rawKey string) (*TokenBalanceSnapshot, error) {
	key, ok := normalizeBalanceKey(rawKey)
	if !ok {
		return nil, ErrTokenBalanceKeyInvalid
	}
	token, err := LookupTokenByKey(context.Background(), key)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTokenBalanceKeyInvalid
		}
		return nil, err
	}
	user, err := model.GetUserCache(token.UserId)
	if err != nil {
		return nil, err
	}

	group := buildTokenBalanceGroup(user.Group, token.Group)
	preference := common.NormalizeBillingPreference(user.GetSetting().BillingPreference)
	now := common.GetTimestamp()
	status := resolveTokenBalanceStatus(token, user, now)
	calculationInput := tokenBalanceCalculationInput{
		Preference:      preference,
		TokenUnlimited:  token.UnlimitedQuota,
		TokenRemaining:  int64(token.RemainQuota),
		TokenUsed:       int64(token.UsedQuota),
		WalletRemaining: int64(user.Quota),
	}
	if status == TokenBalanceStatusActive {
		eligibility := SubscriptionEligibilityForGroups(group.EffectiveGroups)
		summaries, err := model.GetAllActiveUserSubscriptionsByEligibility(token.UserId, eligibility)
		if err != nil {
			return nil, err
		}
		subscriptions := make([]model.UserSubscription, 0, len(summaries))
		for _, summary := range summaries {
			if summary.Subscription != nil {
				subscriptions = append(subscriptions, *summary.Subscription)
			}
		}
		calculationInput.Subscriptions = tokenBalanceSubscriptionInputs(subscriptions, now)
	}
	balance := calculateTokenBalance(calculationInput)
	if status != TokenBalanceStatusActive {
		balance.CurrentAvailable = QuotaAmount{}
		balance.TokenCallableQuota = QuotaAmount{}
		balance.SingleRequestLimit = QuotaAmount{}
		balance.ParticipatingSources = []string{}
		balance.Wallet = nil
		balance.Subscription = nil
		if !balance.TokenUnlimited {
			balance.Restricted = balance.TokenRemaining
		}
		balance.RestrictionReason = TokenBalanceRestrictionUnavailable
	}

	expiresAt := token.ExpiredTime
	if expiresAt < 0 {
		expiresAt = 0
	}
	return &TokenBalanceSnapshot{
		TokenID:           token.Id,
		UserID:            token.UserId,
		MaskedKey:         token.GetMaskedKey(),
		Status:            status,
		ExpiresAt:         expiresAt,
		BillingPreference: preference,
		Group:             group,
		ModelLimits:       buildTokenBalanceModelLimits(token),
		Balance:           balance,
		Display: TokenBalanceDisplay{
			Type:         operation_setting.GetQuotaDisplayType(),
			QuotaPerUnit: common.QuotaPerUnit,
			ExchangeRate: operation_setting.GetUsdToCurrencyRate(operation_setting.USDExchangeRate),
			Symbol:       operation_setting.GetCurrencySymbol(),
		},
	}, nil
}
