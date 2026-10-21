package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/model_catalog_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type pricingResponse struct {
	Success              bool                           `json:"success"`
	Data                 []model.Pricing                `json:"data"`
	Vendors              []model.PricingVendor          `json:"vendors"`
	GroupRatio           map[string]float64             `json:"group_ratio"`
	UsableGroup          map[string]string              `json:"usable_group"`
	AllGroup             map[string]string              `json:"all_group"`
	SupportedEndpointMap map[string]common.EndpointInfo `json:"supported_endpoint"`
	AutoGroups           []string                       `json:"auto_groups"`
	PricingVersion       string                         `json:"pricing_version"`
}

func TestGetPricingKeepsLockedModelsGroupsAndPricesVisible(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	setCatalogVisibilityOption(t, model_catalog_setting.EnabledVisibility())
	previousUsableGroups := setting.UserUsableGroups2JSONString()
	previousGroupRatios := ratio_setting.GroupRatio2JSONString()
	t.Cleanup(func() {
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(previousUsableGroups))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(previousGroupRatios))
		model.InvalidatePricingCache()
	})
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default"}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"vip":0.5}`))

	require.NoError(t, db.Create(&model.User{
		Id:       1901,
		Username: "pricing-catalog-user",
		Password: "password",
		Group:    "default",
		Status:   common.UserStatusEnabled,
	}).Error)
	require.NoError(t, db.Create(&model.Channel{
		Id:     1901,
		Name:   "pricing-catalog",
		Type:   constant.ChannelTypeOpenAI,
		Status: common.ChannelStatusEnabled,
	}).Error)
	require.NoError(t, db.Create(&[]model.Ability{
		{Group: "default", Model: "catalog-default-model", ChannelId: 1901, Enabled: true},
		{Group: "vip", Model: "catalog-vip-model", ChannelId: 1901, Enabled: true},
	}).Error)
	model.InitChannelCache()
	model.InvalidatePricingCache()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/pricing", nil)
	ctx.Set("id", 1901)

	GetPricing(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response pricingResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)
	models := make([]string, 0, len(response.Data))
	for _, item := range response.Data {
		models = append(models, item.ModelName)
	}
	assert.ElementsMatch(t, []string{"catalog-default-model", "catalog-vip-model"}, models)
	assert.Equal(t, map[string]string{"default": "Default"}, response.UsableGroup)
	assert.Contains(t, response.AllGroup, "default")
	assert.Contains(t, response.AllGroup, "vip")
	assert.Equal(t, 1.0, response.GroupRatio["default"])
	assert.Equal(t, 0.5, response.GroupRatio["vip"])
}

func TestGetPricingRestoresOfficialUsableGroupRulesWhenCustomVisibilityIsDisabled(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	setCatalogVisibilityOption(t, model_catalog_setting.DefaultVisibility())
	previousUsableGroups := setting.UserUsableGroups2JSONString()
	previousGroupRatios := ratio_setting.GroupRatio2JSONString()
	t.Cleanup(func() {
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(previousUsableGroups))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(previousGroupRatios))
		model.InvalidatePricingCache()
	})
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default"}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"vip":0.5}`))

	require.NoError(t, db.Create(&model.User{
		Id:       1902,
		Username: "pricing-original-rules-user",
		Password: "password",
		Group:    "default",
		Status:   common.UserStatusEnabled,
	}).Error)
	require.NoError(t, db.Create(&model.Channel{
		Id:     1902,
		Name:   "pricing-original-rules",
		Type:   constant.ChannelTypeOpenAI,
		Status: common.ChannelStatusEnabled,
	}).Error)
	require.NoError(t, db.Create(&[]model.Ability{
		{Group: "default", Model: "catalog-default-model", ChannelId: 1902, Enabled: true},
		{Group: "vip", Model: "catalog-vip-model", ChannelId: 1902, Enabled: true},
		{Group: "default", Model: "catalog-mixed-model", ChannelId: 1902, Enabled: true},
		{Group: "vip", Model: "catalog-mixed-model", ChannelId: 1902, Enabled: true},
		{Group: "all", Model: "catalog-all-model", ChannelId: 1902, Enabled: true},
	}).Error)
	model.InitChannelCache()
	model.InvalidatePricingCache()

	response := getPricingResponseForUser(t, 1902)
	assert.ElementsMatch(t, []string{
		"catalog-all-model",
		"catalog-default-model",
		"catalog-mixed-model",
	}, recorderModelNames(response.Data))
	assert.NotContains(t, recorderModelNames(response.Data), "catalog-vip-model")
	assert.Equal(t, map[string]string{"default": "Default"}, response.UsableGroup)
	assert.Equal(t, map[string]string{"default": "Default"}, response.AllGroup)
	assert.Equal(t, map[string]float64{"default": 1}, response.GroupRatio)
}

func TestGetPricingAppliesSpecialUsableGroupImmediatelyAfterOptionUpdate(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Option{}))
	setCatalogVisibilityOption(t, model_catalog_setting.EnabledVisibility())
	previousUsableGroups := setting.UserUsableGroups2JSONString()
	previousGroupRatios := ratio_setting.GroupRatio2JSONString()
	previousSpecialGroups := ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup.ReadAll()
	t.Cleanup(func() {
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(previousUsableGroups))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(previousGroupRatios))
		specialGroups := ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup
		specialGroups.Clear()
		specialGroups.AddAll(previousSpecialGroups)
		model.InvalidatePricingCache()
	})

	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default"}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"grok":0.9,"special":0.2}`))
	require.NoError(t, db.Create(&model.User{
		Id:       1921,
		Username: "special-group-user",
		Password: "password",
		Group:    "grok",
		Status:   common.UserStatusEnabled,
	}).Error)
	require.NoError(t, db.Create(&model.Channel{
		Id:     1921,
		Name:   "special-group-channel",
		Type:   constant.ChannelTypeOpenAI,
		Status: common.ChannelStatusEnabled,
	}).Error)
	require.NoError(t, db.Create(&model.Ability{
		Group:     "special",
		Model:     "special-group-model",
		ChannelId: 1921,
		Enabled:   true,
	}).Error)
	model.InitChannelCache()
	model.InvalidatePricingCache()

	require.NoError(t, model.UpdateOption("group_ratio_setting.group_special_usable_group", `{}`))
	before := getPricingResponseForUser(t, 1921)
	assert.NotContains(t, before.UsableGroup, "special")

	require.NoError(t, model.UpdateOption(
		"group_ratio_setting.group_special_usable_group",
		`{"grok":{"+:special":"Special"}}`,
	))
	after := getPricingResponseForUser(t, 1921)
	assert.Equal(t, "Special", after.UsableGroup["special"])
}

func getPricingResponseForUser(t *testing.T, userID int) pricingResponse {
	t.Helper()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/pricing", nil)
	ctx.Set("id", userID)

	GetPricing(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response pricingResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)
	return response
}

func TestGetPricingNeverSerializesHiddenGroup(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	previousUsableGroups := setting.UserUsableGroups2JSONString()
	previousGroupRatios := ratio_setting.GroupRatio2JSONString()
	visibility := model_catalog_setting.HiddenByDefaultVisibility()
	visibility.Rules["default"] = model_catalog_setting.GroupRule{
		Mode:       model_catalog_setting.ModeAll,
		UserGroups: []string{},
	}
	setCatalogVisibilityOption(t, visibility)
	t.Cleanup(func() {
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(previousUsableGroups))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(previousGroupRatios))
		model.InvalidatePricingCache()
	})
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default","secret":"Secret"}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"secret":3}`))
	require.NoError(t, db.Create(&model.Channel{
		Id:     1911,
		Name:   "pricing-hidden-catalog",
		Type:   constant.ChannelTypeOpenAI,
		Status: common.ChannelStatusEnabled,
	}).Error)
	require.NoError(t, db.Create(&[]model.Ability{
		{Group: "default", Model: "mixed-model", ChannelId: 1911, Enabled: true},
		{Group: "secret", Model: "mixed-model", ChannelId: 1911, Enabled: true},
		{Group: "secret", Model: "secret-only-model", ChannelId: 1911, Enabled: true},
	}).Error)
	model.InitChannelCache()
	model.InvalidatePricingCache()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/pricing", nil)
	GetPricing(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.NotContains(t, recorder.Body.String(), "secret")
	var response pricingResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)
	require.Len(t, response.Data, 1)
	assert.Equal(t, "mixed-model", response.Data[0].ModelName)
	assert.Equal(t, []string{"default"}, response.Data[0].EnableGroup)
	assert.Equal(t, map[string]string{"default": "Default"}, response.AllGroup)
	assert.Equal(t, map[string]float64{"default": 1}, response.GroupRatio)
	assert.NotEmpty(t, response.PricingVersion)
	assert.Equal(t, "private, no-store", recorder.Header().Get("Cache-Control"))
}

func TestGetPricingSelectedEmptyIsSuccessfulEmptyCatalog(t *testing.T) {
	setupModelListControllerTestDB(t)
	setCatalogVisibilityOption(t, model_catalog_setting.HiddenByDefaultVisibility())
	model.InvalidatePricingCache()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/pricing", nil)
	GetPricing(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response pricingResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.True(t, response.Success)
	assert.Empty(t, response.Data)
	assert.Empty(t, response.AllGroup)
	assert.Empty(t, response.GroupRatio)
}

func TestGetPricingProjectsCatalogForCurrentUserGroup(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	previousUsableGroups := setting.UserUsableGroups2JSONString()
	previousGroupRatios := ratio_setting.GroupRatio2JSONString()
	t.Cleanup(func() {
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(previousUsableGroups))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(previousGroupRatios))
		model.InvalidatePricingCache()
	})
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default","secret":"Secret"}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"secret":3}`))

	visibility := model_catalog_setting.EnabledVisibility()
	visibility.Rules["secret"] = model_catalog_setting.GroupRule{
		Mode:       model_catalog_setting.ModeSelected,
		UserGroups: []string{"Admin", "test"},
	}
	setCatalogVisibilityOption(t, visibility)

	require.NoError(t, db.Create(&[]model.User{
		{Id: 1931, Username: "catalog-admin", Password: "password", Group: "Admin", AffCode: "catalog-admin", Status: common.UserStatusEnabled},
		{Id: 1932, Username: "catalog-test", Password: "password", Group: "test", AffCode: "catalog-test", Status: common.UserStatusEnabled},
		{Id: 1933, Username: "catalog-default", Password: "password", Group: "default", AffCode: "catalog-default", Status: common.UserStatusEnabled},
	}).Error)
	require.NoError(t, db.Create(&model.Channel{
		Id: 1931, Name: "user-scoped-catalog", Type: constant.ChannelTypeOpenAI, Status: common.ChannelStatusEnabled,
	}).Error)
	require.NoError(t, db.Create(&[]model.Ability{
		{Group: "default", Model: "public-model", ChannelId: 1931, Enabled: true},
		{Group: "secret", Model: "secret-model", ChannelId: 1931, Enabled: true},
	}).Error)
	model.InitChannelCache()
	model.InvalidatePricingCache()

	for _, userID := range []int{1931, 1932} {
		response := getPricingResponseForUser(t, userID)
		assert.Contains(t, response.AllGroup, "secret")
		assert.Contains(t, recorderModelNames(response.Data), "secret-model")
	}
	response := getPricingResponseForUser(t, 1933)
	assert.NotContains(t, response.AllGroup, "secret")
	assert.NotContains(t, recorderModelNames(response.Data), "secret-model")
}

func recorderModelNames(pricing []model.Pricing) []string {
	names := make([]string, 0, len(pricing))
	for _, item := range pricing {
		names = append(names, item.ModelName)
	}
	return names
}

func setCatalogVisibilityOption(t *testing.T, visibility model_catalog_setting.Visibility) {
	t.Helper()
	raw, err := model_catalog_setting.Marshal(visibility)
	require.NoError(t, err)
	common.OptionMapRWMutex.Lock()
	if common.OptionMap == nil {
		common.OptionMap = make(map[string]string)
	}
	previous, existed := common.OptionMap[model_catalog_setting.OptionKey]
	common.OptionMap[model_catalog_setting.OptionKey] = raw
	common.OptionMapRWMutex.Unlock()
	model.InvalidateCatalogPricing()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		if existed {
			common.OptionMap[model_catalog_setting.OptionKey] = previous
		} else {
			delete(common.OptionMap, model_catalog_setting.OptionKey)
		}
		common.OptionMapRWMutex.Unlock()
		model.InvalidateCatalogPricing()
	})
}
