package operation_setting

import (
	"errors"
	"math"
	"math/big"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/setting/config"
	"github.com/shopspring/decimal"
)

const InvitationIncentiveSettingConfigName = "invitation_incentive_setting"

type InvitationIncentiveSetting struct {
	Enabled                                      bool   `json:"enabled"`
	InviterRegistrationRewardEnabled             bool   `json:"inviter_registration_reward_enabled"`
	InviteeRegistrationGiftEnabled               bool   `json:"invitee_registration_gift_enabled"`
	CashCommissionEnabled                        bool   `json:"cash_commission_enabled"`
	CashCommissionRate                           string `json:"cash_commission_rate"`
	AdminBalanceRedemptionCommissionEnabled      bool   `json:"admin_balance_redemption_commission_enabled"`
	AdminBalanceRedemptionCommissionRate         string `json:"admin_balance_redemption_commission_rate"`
	AdminSubscriptionRedemptionCommissionEnabled bool   `json:"admin_subscription_redemption_commission_enabled"`
	AdminSubscriptionRedemptionCommissionRate    string `json:"admin_subscription_redemption_commission_rate"`
}

type InvitationIncentivePublicPolicy struct {
	Enabled               bool   `json:"enabled"`
	CashCommissionEnabled bool   `json:"cash_commission_enabled"`
	CashCommissionRate    string `json:"cash_commission_rate"`
}

func DefaultInvitationIncentiveSetting() InvitationIncentiveSetting {
	return InvitationIncentiveSetting{
		Enabled:                                      true,
		InviterRegistrationRewardEnabled:             true,
		InviteeRegistrationGiftEnabled:               true,
		CashCommissionEnabled:                        false,
		CashCommissionRate:                           "0.00",
		AdminBalanceRedemptionCommissionEnabled:      false,
		AdminBalanceRedemptionCommissionRate:         "0.00",
		AdminSubscriptionRedemptionCommissionEnabled: false,
		AdminSubscriptionRedemptionCommissionRate:    "0.00",
	}
}

var invitationIncentiveSetting = DefaultInvitationIncentiveSetting()

func init() {
	config.GlobalConfig.Register(InvitationIncentiveSettingConfigName, &invitationIncentiveSetting)
}

func GetInvitationIncentiveSetting() *InvitationIncentiveSetting {
	return &invitationIncentiveSetting
}

func ParseInvitationCommissionRate(input string) (int64, string, error) {
	rate, err := decimal.NewFromString(strings.TrimSpace(input))
	if err != nil {
		return 0, "", errors.New("invitation commission rate must be a valid number")
	}
	if rate.IsNegative() {
		return 0, "", errors.New("invitation commission rate must not be negative")
	}
	if rate.Exponent() < -2 {
		return 0, "", errors.New("invitation commission rate must have at most two decimal places")
	}

	scaled := rate.Mul(decimal.NewFromInt(100))
	if !scaled.IsInteger() {
		return 0, "", errors.New("invitation commission rate must have at most two decimal places")
	}
	value := scaled.BigInt()
	if value.Sign() < 0 || value.Cmp(big.NewInt(math.MaxInt64)) > 0 {
		return 0, "", errors.New("invitation commission rate is too large")
	}
	return value.Int64(), rate.StringFixed(2), nil
}

func NormalizeInvitationIncentiveSetting(setting InvitationIncentiveSetting) (InvitationIncentiveSetting, error) {
	var err error
	_, setting.CashCommissionRate, err = ParseInvitationCommissionRate(setting.CashCommissionRate)
	if err != nil {
		return InvitationIncentiveSetting{}, err
	}
	_, setting.AdminBalanceRedemptionCommissionRate, err = ParseInvitationCommissionRate(setting.AdminBalanceRedemptionCommissionRate)
	if err != nil {
		return InvitationIncentiveSetting{}, err
	}
	_, setting.AdminSubscriptionRedemptionCommissionRate, err = ParseInvitationCommissionRate(setting.AdminSubscriptionRedemptionCommissionRate)
	if err != nil {
		return InvitationIncentiveSetting{}, err
	}
	return setting, nil
}

func InvitationIncentiveSettingsToOptions(setting InvitationIncentiveSetting) map[string]string {
	prefix := InvitationIncentiveSettingConfigName + "."
	return map[string]string{
		prefix + "enabled": strconv.FormatBool(setting.Enabled),
		prefix + "inviter_registration_reward_enabled":              strconv.FormatBool(setting.InviterRegistrationRewardEnabled),
		prefix + "invitee_registration_gift_enabled":                strconv.FormatBool(setting.InviteeRegistrationGiftEnabled),
		prefix + "cash_commission_enabled":                          strconv.FormatBool(setting.CashCommissionEnabled),
		prefix + "cash_commission_rate":                             setting.CashCommissionRate,
		prefix + "admin_balance_redemption_commission_enabled":      strconv.FormatBool(setting.AdminBalanceRedemptionCommissionEnabled),
		prefix + "admin_balance_redemption_commission_rate":         setting.AdminBalanceRedemptionCommissionRate,
		prefix + "admin_subscription_redemption_commission_enabled": strconv.FormatBool(setting.AdminSubscriptionRedemptionCommissionEnabled),
		prefix + "admin_subscription_redemption_commission_rate":    setting.AdminSubscriptionRedemptionCommissionRate,
	}
}

func GetInvitationIncentivePublicPolicy() (InvitationIncentivePublicPolicy, error) {
	setting := GetInvitationIncentiveSetting()
	_, normalizedRate, err := ParseInvitationCommissionRate(setting.CashCommissionRate)
	if err != nil {
		return InvitationIncentivePublicPolicy{}, err
	}
	return InvitationIncentivePublicPolicy{
		Enabled:               setting.Enabled,
		CashCommissionEnabled: setting.CashCommissionEnabled,
		CashCommissionRate:    normalizedRate,
	}, nil
}
