package controller

import (
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClaimRedemptionReturnsDiscriminatedAdministratorBalanceResult(t *testing.T) {
	setupUserRedemptionControllerTest(t)
	confirmPaymentComplianceForTest(t)
	seedUserRootBindingUser(t, 5101, "unified-claimant", common.RoleCommonUser)
	code := model.Redemption{
		Name: "unified-controller", Key: "51010000000000000000000000000001",
		Status: common.RedemptionCodeStatusEnabled, Quota: 6789, CreatedTime: common.GetTimestamp(),
	}
	require.NoError(t, model.DB.Create(&code).Error)
	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/redemption/claim", map[string]any{
		"code": code.Key,
	}, 5101)

	ClaimRedemption(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success)
	var result struct {
		RewardType     string `json:"reward_type"`
		Source         string `json:"source"`
		Quota          int64  `json:"quota"`
		SubscriptionID int64  `json:"subscription_id"`
	}
	require.NoError(t, common.Unmarshal(response.Data, &result))
	assert.Equal(t, model.UserRedemptionRewardBalance, result.RewardType)
	assert.Equal(t, "administrator", result.Source)
	assert.EqualValues(t, 6789, result.Quota)
	assert.Zero(t, result.SubscriptionID)
}

func TestClaimRedemptionRequiresPaymentCompliance(t *testing.T) {
	setupUserRedemptionControllerTest(t)
	payment := operation_setting.GetPaymentSetting()
	originalConfirmed := payment.ComplianceConfirmed
	originalVersion := payment.ComplianceTermsVersion
	t.Cleanup(func() {
		payment.ComplianceConfirmed = originalConfirmed
		payment.ComplianceTermsVersion = originalVersion
	})
	payment.ComplianceConfirmed = false
	payment.ComplianceTermsVersion = ""
	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/redemption/claim", map[string]any{
		"code": "unused",
	}, 5102)

	ClaimRedemption(ctx)

	response := decodeAPIResponse(t, recorder)
	assert.False(t, response.Success)
	assert.NotEmpty(t, response.Message)
}
