package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/model_catalog_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupModelCatalogControllerTest(t *testing.T) {
	t.Helper()
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Option{}))
	previousRatios := ratio_setting.GroupRatio2JSONString()
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"vip":2}`))
	require.NoError(t, db.Create(&model.Channel{
		Id:     1921,
		Name:   "catalog-admin-test",
		Type:   constant.ChannelTypeOpenAI,
		Status: common.ChannelStatusEnabled,
	}).Error)
	require.NoError(t, db.Create(&[]model.Ability{
		{Group: "default", Model: "default-model", ChannelId: 1921, Enabled: true},
		{Group: "vip", Model: "vip-model", ChannelId: 1921, Enabled: true},
	}).Error)
	common.OptionMapRWMutex.Lock()
	if common.OptionMap == nil {
		common.OptionMap = make(map[string]string)
	}
	previous, existed := common.OptionMap[model_catalog_setting.OptionKey]
	delete(common.OptionMap, model_catalog_setting.OptionKey)
	common.OptionMapRWMutex.Unlock()
	model.InitChannelCache()
	model.InvalidatePricingCache()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(previousRatios))
		common.OptionMapRWMutex.Lock()
		if existed {
			common.OptionMap[model_catalog_setting.OptionKey] = previous
		} else {
			delete(common.OptionMap, model_catalog_setting.OptionKey)
		}
		common.OptionMapRWMutex.Unlock()
		model.InvalidatePricingCache()
	})
}

func TestModelCatalogVisibilityUpdateUsesETagAndPublishesSelectedGroups(t *testing.T) {
	setupModelCatalogControllerTest(t)

	getRecorder := httptest.NewRecorder()
	getContext, _ := gin.CreateTestContext(getRecorder)
	getContext.Request = httptest.NewRequest(http.MethodGet, "/api/option/model-catalog-visibility", nil)
	GetModelCatalogVisibility(getContext)
	require.Equal(t, http.StatusOK, getRecorder.Code)
	var getResponse struct {
		Success bool                    `json:"success"`
		Data    model.CatalogAdminState `json:"data"`
	}
	require.NoError(t, common.Unmarshal(getRecorder.Body.Bytes(), &getResponse))
	require.True(t, getResponse.Success)
	require.NotNil(t, getResponse.Data.Config)
	assert.False(t, getResponse.Data.Config.Enabled)
	assert.Equal(t, model_catalog_setting.DefaultModeAll, getResponse.Data.Config.DefaultMode)
	assert.Equal(t, 0, getResponse.Data.Summary.RestrictedGroupCount)
	assert.Len(t, getResponse.Data.Groups, 2)
	assert.Len(t, getResponse.Data.UserGroups, 2)
	disabledSnapshot, err := model.GetCatalogPricingSnapshotForUserGroup("default")
	require.NoError(t, err)
	assert.Contains(t, disabledSnapshot.AllGroup, "default")
	assert.Contains(t, disabledSnapshot.AllGroup, "vip")
	assert.Len(t, disabledSnapshot.Pricing, 2)

	body := `{"config":{"version":3,"enabled":true,"default_mode":"all","rules":{"vip":{"mode":"selected","user_groups":["default"]}}},"expected_etag":"` + getResponse.Data.ETag + `"}`
	putRecorder := httptest.NewRecorder()
	putContext, _ := gin.CreateTestContext(putRecorder)
	putContext.Request = httptest.NewRequest(http.MethodPut, "/api/option/model-catalog-visibility", strings.NewReader(body))
	putContext.Request.Header.Set("Content-Type", "application/json")
	putContext.Set("id", 1)
	UpdateModelCatalogVisibility(putContext)
	require.Equal(t, http.StatusOK, putRecorder.Code)
	var putResponse struct {
		Success bool                    `json:"success"`
		Data    model.CatalogAdminState `json:"data"`
	}
	require.NoError(t, common.Unmarshal(putRecorder.Body.Bytes(), &putResponse))
	require.True(t, putResponse.Success)
	require.NotNil(t, putResponse.Data.Config)
	assert.True(t, putResponse.Data.Config.Enabled)
	assert.Equal(t, []string{"default"}, putResponse.Data.Config.Rules["vip"].UserGroups)
	assert.Equal(t, 2, putResponse.Data.Summary.GroupCount)
	assert.Equal(t, 2, putResponse.Data.Summary.ModelCount)
	assert.Equal(t, 1, putResponse.Data.Summary.RestrictedGroupCount)

	snapshot, err := model.GetCatalogPricingSnapshot()
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"default": "默认分组"}, snapshot.AllGroup)
	defaultSnapshot, err := model.GetCatalogPricingSnapshotForUserGroup("default")
	require.NoError(t, err)
	assert.Contains(t, defaultSnapshot.AllGroup, "vip")

	idempotentBody, err := common.Marshal(modelCatalogVisibilityUpdateRequest{
		Config:       *putResponse.Data.Config,
		ExpectedETag: putResponse.Data.ETag,
	})
	require.NoError(t, err)
	idempotentRecorder := httptest.NewRecorder()
	idempotentContext, _ := gin.CreateTestContext(idempotentRecorder)
	idempotentContext.Request = httptest.NewRequest(http.MethodPut, "/api/option/model-catalog-visibility", strings.NewReader(string(idempotentBody)))
	idempotentContext.Request.Header.Set("Content-Type", "application/json")
	idempotentContext.Set("id", 1)
	UpdateModelCatalogVisibility(idempotentContext)
	require.Equal(t, http.StatusOK, idempotentRecorder.Code)
}

func TestModelCatalogVisibilityRejectsStaleETag(t *testing.T) {
	setupModelCatalogControllerTest(t)
	body := `{"config":{"version":3,"enabled":false,"default_mode":"all","rules":{}},"expected_etag":"stale"}`
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPut, "/api/option/model-catalog-visibility", strings.NewReader(body))
	UpdateModelCatalogVisibility(ctx)
	assert.Equal(t, http.StatusConflict, recorder.Code)
}

func TestGenericOptionUpdateRejectsModelCatalogVisibilityKey(t *testing.T) {
	setupModelCatalogControllerTest(t)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPut, "/api/option/", strings.NewReader(
		`{"key":"model_catalog_setting.group_visibility","value":"{}"}`,
	))
	UpdateOption(ctx)
	assert.NotContains(t, recorder.Body.String(), `"success":true`)
}
