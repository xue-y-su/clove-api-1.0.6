package model

import (
	"sort"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func setSub2APITestBillingMaps(t *testing.T, modes, expressions map[string]string) {
	t.Helper()
	rawModes, err := common.Marshal(modes)
	require.NoError(t, err)
	rawExpressions, err := common.Marshal(expressions)
	require.NoError(t, err)
	require.NoError(t, config.UpdateConfigFromMap(
		config.GlobalConfig.Get("billing_setting"),
		map[string]string{
			billing_setting.BillingModeField: string(rawModes),
			billing_setting.BillingExprField: string(rawExpressions),
		},
	))
}

func pricingModelNames(pricing []Pricing) []string {
	names := make([]string, 0, len(pricing))
	for _, item := range pricing {
		names = append(names, item.ModelName)
	}
	sort.Strings(names)
	return names
}

func TestSub2APIAbilityRequiresApprovalRealGroupAndPersistedPricing(t *testing.T) {
	db := setupChannelModelPublicationTestDB(t)
	require.NoError(t, db.AutoMigrate(&Option{}, &Model{}, &Vendor{}))

	previousMemoryCacheEnabled := common.MemoryCacheEnabled
	previousModelRatios := ratio_setting.ModelRatio2JSONString()
	previousModelPrices := ratio_setting.ModelPrice2JSONString()
	previousBillingModes := billing_setting.GetBillingModeCopy()
	previousBillingExpressions := billing_setting.GetBillingExprCopy()
	common.MemoryCacheEnabled = false
	t.Cleanup(func() {
		common.MemoryCacheEnabled = previousMemoryCacheEnabled
		require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(previousModelRatios))
		require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(previousModelPrices))
		setSub2APITestBillingMaps(t, previousBillingModes, previousBillingExpressions)
		InvalidatePricingCache()
	})

	channels := []Channel{
		{Id: 801, Type: constant.ChannelTypeSub2API, Key: "sub2", Status: common.ChannelStatusEnabled, Name: "sub2"},
		{Id: 802, Type: constant.ChannelTypeOpenAI, Key: "openai", Status: common.ChannelStatusEnabled, Name: "openai"},
	}
	require.NoError(t, db.Create(&channels).Error)
	abilities := []Ability{
		{Group: "default", Model: "sub2-unapproved", ChannelId: 801, Enabled: true},
		{Group: "default", Model: "sub2-no-price", ChannelId: 801, Enabled: true},
		{Group: "default", Model: "sub2-ratio", ChannelId: 801, Enabled: true},
		{Group: "default", Model: "sub2-fixed", ChannelId: 801, Enabled: true},
		{Group: "default", Model: "sub2-tiered", ChannelId: 801, Enabled: true},
		{Group: "auto", Model: "sub2-auto-group", ChannelId: 801, Enabled: true},
		{Group: "default", Model: "shared-model", ChannelId: 801, Enabled: true},
		{Group: "default", Model: "shared-model", ChannelId: 802, Enabled: true},
		{Group: "default", Model: "manual-fallback", ChannelId: 802, Enabled: true},
	}
	require.NoError(t, db.Create(&abilities).Error)
	require.NoError(t, ApproveChannelModelPublications(db, 801, []string{
		"sub2-no-price",
		"sub2-ratio",
		"sub2-fixed",
		"sub2-tiered",
		"sub2-auto-group",
	}))

	modelRatios := `{"sub2-unapproved":1,"sub2-ratio":1.25,"sub2-auto-group":1}`
	modelPrices := `{"sub2-fixed":0.02}`
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(modelRatios))
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(modelPrices))
	setSub2APITestBillingMaps(t,
		map[string]string{"sub2-tiered": billing_setting.BillingModeTieredExpr},
		map[string]string{"sub2-tiered": "p / 1000"},
	)
	for key, value := range map[string]string{
		"ModelRatio":                   modelRatios,
		"ModelPrice":                   modelPrices,
		"billing_setting.billing_mode": `{"sub2-tiered":"tiered_expr"}`,
		"billing_setting.billing_expr": `{"sub2-tiered":"p / 1000"}`,
	} {
		require.NoError(t, db.Save(&Option{Key: key, Value: value}).Error)
	}

	InvalidatePricingCache()
	names := pricingModelNames(GetPricing())
	assert.Contains(t, names, "sub2-ratio")
	assert.Contains(t, names, "sub2-fixed")
	assert.Contains(t, names, "sub2-tiered")
	assert.Contains(t, names, "shared-model")
	assert.Contains(t, names, "manual-fallback")
	assert.NotContains(t, names, "sub2-unapproved")
	assert.NotContains(t, names, "sub2-no-price")
	assert.NotContains(t, names, "sub2-auto-group")
}

func TestLoadExplicitCatalogPricingModelsRequiresCompleteTieredExpression(t *testing.T) {
	db := setupChannelModelPublicationTestDB(t)
	require.NoError(t, db.AutoMigrate(&Option{}))
	options := []Option{
		{Key: "ModelRatio", Value: `{"ratio-model":1}`},
		{Key: "ModelPrice", Value: `{"fixed-model":0}`},
		{Key: "billing_setting.billing_mode", Value: `{"tiered-model":"tiered_expr","missing-expr":"tiered_expr","ratio-mode":"ratio"}`},
		{Key: "billing_setting.billing_expr", Value: `{"tiered-model":"p / 1000","missing-expr":"   ","ratio-mode":"p"}`},
	}
	require.NoError(t, db.Create(&options).Error)

	explicit, err := loadExplicitCatalogPricingModels()
	require.NoError(t, err)
	assert.Equal(t, map[string]struct{}{
		"ratio-model":  {},
		"fixed-model":  {},
		"tiered-model": {},
	}, explicit)
}

func TestLoadExplicitCatalogPricingOptionsQuotesReservedKeyForMySQL(t *testing.T) {
	db, err := gorm.Open(mysql.New(mysql.Config{
		DSN:                       "gorm:gorm@tcp(localhost:9910)/gorm?charset=utf8mb4&parseTime=True&loc=Local",
		SkipInitializeWithVersion: true,
	}), &gorm.Config{DryRun: true, DisableAutomaticPing: true})
	require.NoError(t, err)

	var options []Option
	result := explicitCatalogPricingOptionsQuery(db).Find(&options)
	require.NoError(t, result.Error)
	query := result.Statement.SQL.String()
	assert.Contains(t, query, "`key` IN")
}
