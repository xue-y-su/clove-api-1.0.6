package controller

import (
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/require"
)

func setupWalletAmountValidationTest(t *testing.T, userID int) int64 {
	t.Helper()
	confirmPaymentComplianceForTest(t)

	db := setupEpayTopupIntegrationDB(t)
	user := seedTokenOwner(t, db, userID, "wallet-amount-user", "wallet-amount@example.com")
	require.NoError(t, db.Model(user).Update("group", "default").Error)

	generalSetting := operation_setting.GetGeneralSetting()
	previousQuotaDisplayType := generalSetting.QuotaDisplayType
	previousWaffoMinTopUp := setting.WaffoMinTopUp
	previousWaffoUnitPrice := setting.WaffoUnitPrice
	previousWaffoEnabled := setting.WaffoEnabled
	previousWaffoSandbox := setting.WaffoSandbox
	previousWaffoAPIKey := setting.WaffoApiKey
	previousWaffoPrivateKey := setting.WaffoPrivateKey
	previousWaffoPublicCert := setting.WaffoPublicCert
	previousWaffoPancakeMinTopUp := setting.WaffoPancakeMinTopUp
	previousWaffoPancakeUnitPrice := setting.WaffoPancakeUnitPrice
	t.Cleanup(func() {
		generalSetting.QuotaDisplayType = previousQuotaDisplayType
		setting.WaffoMinTopUp = previousWaffoMinTopUp
		setting.WaffoUnitPrice = previousWaffoUnitPrice
		setting.WaffoEnabled = previousWaffoEnabled
		setting.WaffoSandbox = previousWaffoSandbox
		setting.WaffoApiKey = previousWaffoAPIKey
		setting.WaffoPrivateKey = previousWaffoPrivateKey
		setting.WaffoPublicCert = previousWaffoPublicCert
		setting.WaffoPancakeMinTopUp = previousWaffoPancakeMinTopUp
		setting.WaffoPancakeUnitPrice = previousWaffoPancakeUnitPrice
	})
	generalSetting.QuotaDisplayType = operation_setting.QuotaDisplayTypeUSD
	setting.WaffoMinTopUp = 1
	setting.WaffoUnitPrice = 1
	setting.WaffoEnabled = true
	setting.WaffoSandbox = false
	setting.WaffoApiKey = "api"
	setting.WaffoPrivateKey = "private"
	setting.WaffoPublicCert = "public"
	setting.WaffoPancakeMinTopUp = 1
	setting.WaffoPancakeUnitPrice = 1

	require.Greater(t, common.QuotaPerUnit, 0.0)
	return int64(common.MaxQuota)/int64(common.QuotaPerUnit) + 1
}

func TestWaffoAmountPreviewRejectsAmountThatExceedsWalletQuota(t *testing.T) {
	overflowAmount := setupWalletAmountValidationTest(t, 776)

	context, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/user/waffo/amount", WaffoPayRequest{Amount: overflowAmount}, 776)
	RequestWaffoAmount(context)

	require.Equal(t, http.StatusOK, recorder.Code)
	response := decodeAPIResponse(t, recorder)
	require.Equal(t, "error", response.Message)
}

func TestWaffoPayRejectsAmountThatExceedsWalletQuotaBeforeCreatingOrder(t *testing.T) {
	overflowAmount := setupWalletAmountValidationTest(t, 778)

	context, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/user/waffo/pay", WaffoPayRequest{Amount: overflowAmount}, 778)
	RequestWaffoPay(context)

	require.Equal(t, http.StatusOK, recorder.Code)
	response := decodeAPIResponse(t, recorder)
	require.Equal(t, "error", response.Message)
	require.JSONEq(t, `"充值额度超出允许范围"`, string(response.Data))
	var topUpCount int64
	require.NoError(t, model.DB.Model(&model.TopUp{}).Where("user_id = ?", 778).Count(&topUpCount).Error)
	require.Zero(t, topUpCount)
}

func TestWaffoPancakeAmountPreviewRejectsAmountThatExceedsWalletQuota(t *testing.T) {
	overflowAmount := setupWalletAmountValidationTest(t, 777)

	context, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/user/waffo-pancake/amount", WaffoPancakePayRequest{Amount: overflowAmount}, 777)
	RequestWaffoPancakeAmount(context)

	require.Equal(t, http.StatusOK, recorder.Code)
	response := decodeAPIResponse(t, recorder)
	require.Equal(t, "error", response.Message)
}

func TestWalletTopUpRejectsTokenAmountsThatCannotBeCreditedExactly(t *testing.T) {
	generalSetting := operation_setting.GetGeneralSetting()
	previousQuotaDisplayType := generalSetting.QuotaDisplayType
	t.Cleanup(func() {
		generalSetting.QuotaDisplayType = previousQuotaDisplayType
	})
	generalSetting.QuotaDisplayType = operation_setting.QuotaDisplayTypeTokens

	unit := int64(common.QuotaPerUnit)
	require.Greater(t, unit, int64(1))

	_, err := normalizeAndValidateWalletTopUpAmount(unit + 1)
	require.Error(t, err)
}
