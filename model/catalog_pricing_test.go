package model

import (
	"errors"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/setting/model_catalog_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func catalogProjectionFixture() PricingSourceSnapshot {
	return PricingSourceSnapshot{
		Generation: 7,
		Pricing: []Pricing{
			{
				ModelName:              "mixed",
				VendorID:               1,
				EnableGroup:            []string{"default", "secret"},
				SupportedEndpointTypes: []constant.EndpointType{constant.EndpointTypeOpenAI},
			},
			{
				ModelName:              "secret-only",
				VendorID:               2,
				EnableGroup:            []string{"secret"},
				SupportedEndpointTypes: []constant.EndpointType{constant.EndpointTypeAnthropic},
			},
			{ModelName: "expanded", EnableGroup: []string{"all"}},
		},
		Vendors: []PricingVendor{
			{ID: 1, Name: "Visible Vendor"},
			{ID: 2, Name: "Secret Vendor"},
		},
		SupportedEndpoints: map[string]common.EndpointInfo{
			string(constant.EndpointTypeOpenAI):    {Path: "/v1/chat/completions", Method: "POST"},
			string(constant.EndpointTypeAnthropic): {Path: "/v1/messages", Method: "POST"},
		},
		GroupRatio: map[string]float64{"default": 1, "secret": 3},
		GroupDescriptions: map[string]string{
			"default": "Default",
			"secret":  "Secret",
		},
	}
}

func selectedCatalogVisibility(groups ...string) model_catalog_setting.Visibility {
	visibility := model_catalog_setting.HiddenByDefaultVisibility()
	for _, group := range groups {
		visibility.Rules[group] = model_catalog_setting.GroupRule{
			Mode:       model_catalog_setting.ModeAll,
			UserGroups: []string{},
		}
	}
	return visibility
}

func mustCatalogJSON(t *testing.T, value any) string {
	t.Helper()
	encoded, err := common.Marshal(value)
	require.NoError(t, err)
	return string(encoded)
}

func TestCatalogProjectionHidesStructuralGroupData(t *testing.T) {
	got, err := buildCatalogProjection(catalogProjectionFixture(), selectedCatalogVisibility("default"))
	require.NoError(t, err)
	require.Len(t, got.Pricing, 2)
	assert.Equal(t, "expanded", got.Pricing[0].ModelName)
	assert.Equal(t, []string{"default"}, got.Pricing[0].EnableGroup)
	assert.Equal(t, "mixed", got.Pricing[1].ModelName)
	assert.Equal(t, []string{"default"}, got.Pricing[1].EnableGroup)
	assert.Equal(t, map[string]string{"default": "Default"}, got.AllGroup)
	assert.Equal(t, map[string]float64{"default": 1}, got.GroupRatio)
	assert.Equal(t, []PricingVendor{{ID: 1, Name: "Visible Vendor"}}, got.Vendors)
	assert.Equal(t, map[string]common.EndpointInfo{
		string(constant.EndpointTypeOpenAI): {Path: "/v1/chat/completions", Method: "POST"},
	}, got.SupportedEndpoints)
	assert.NotContains(t, mustCatalogJSON(t, got), "secret")
}

func TestCatalogProjectionAppliesIndependentRulesPerUserGroup(t *testing.T) {
	visibility := model_catalog_setting.EnabledVisibility()
	visibility.Rules["secret"] = model_catalog_setting.GroupRule{
		Mode:       model_catalog_setting.ModeSelected,
		UserGroups: []string{"Admin", "test"},
	}

	admin, err := buildCatalogProjectionForUserGroup(catalogProjectionFixture(), visibility, "Admin")
	require.NoError(t, err)
	assert.Contains(t, admin.AllGroup, "secret")
	assert.ElementsMatch(t, []string{"mixed", "secret-only", "expanded"}, catalogModelNames(admin.Pricing))

	testUser, err := buildCatalogProjectionForUserGroup(catalogProjectionFixture(), visibility, "test")
	require.NoError(t, err)
	assert.Contains(t, testUser.AllGroup, "secret")

	for _, userGroup := range []string{"default", ""} {
		projected, projectionErr := buildCatalogProjectionForUserGroup(
			catalogProjectionFixture(),
			visibility,
			userGroup,
		)
		require.NoError(t, projectionErr)
		assert.NotContains(t, projected.AllGroup, "secret")
		assert.Equal(t, []string{"expanded", "mixed"}, catalogModelNames(projected.Pricing))
		assert.Equal(t, []PricingVendor{{ID: 1, Name: "Visible Vendor"}}, projected.Vendors)
		assert.Equal(t, map[string]common.EndpointInfo{
			string(constant.EndpointTypeOpenAI): {Path: "/v1/chat/completions", Method: "POST"},
		}, projected.SupportedEndpoints)
	}
}

func TestCatalogProjectionBypassesCustomRulesWhenVisibilityIsDisabled(t *testing.T) {
	visibility := model_catalog_setting.DefaultVisibility()
	visibility.Rules["secret"] = model_catalog_setting.GroupRule{
		Mode:       model_catalog_setting.ModeSelected,
		UserGroups: []string{"Admin", "test"},
	}

	for _, userGroup := range []string{"Admin", "default", ""} {
		projected, err := buildCatalogProjectionForUserGroup(
			catalogProjectionFixture(),
			visibility,
			userGroup,
		)
		require.NoError(t, err)
		assert.Contains(t, projected.AllGroup, "default")
		assert.Contains(t, projected.AllGroup, "secret")
		assert.Contains(t, catalogModelNames(projected.Pricing), "secret-only")
	}
}

func catalogModelNames(pricing []Pricing) []string {
	names := make([]string, 0, len(pricing))
	for _, item := range pricing {
		names = append(names, item.ModelName)
	}
	return names
}

func TestCatalogProjectionDoesNotMutateSource(t *testing.T) {
	source := catalogProjectionFixture()
	before := mustCatalogJSON(t, source)
	_, err := buildCatalogProjection(source, selectedCatalogVisibility("default"))
	require.NoError(t, err)
	assert.JSONEq(t, before, mustCatalogJSON(t, source))
}

func TestCatalogProjectionAllowsExplicitEmptyCatalog(t *testing.T) {
	got, err := buildCatalogProjection(catalogProjectionFixture(), selectedCatalogVisibility())
	require.NoError(t, err)
	assert.Empty(t, got.Pricing)
	assert.Empty(t, got.AllGroup)
	assert.Empty(t, got.GroupRatio)
	assert.Empty(t, got.Vendors)
	assert.Empty(t, got.SupportedEndpoints)
	assert.NotEmpty(t, got.Version)
}

func TestCatalogProjectionAllModeIncludesFutureRealGroups(t *testing.T) {
	source := catalogProjectionFixture()
	source.GroupDescriptions["future"] = "Future"
	source.GroupRatio["future"] = 2
	source.Pricing = append(source.Pricing, Pricing{ModelName: "future-model", EnableGroup: []string{"future"}})

	got, err := buildCatalogProjection(source, model_catalog_setting.EnabledVisibility())
	require.NoError(t, err)
	assert.Contains(t, got.AllGroup, "future")
	assert.Contains(t, got.GroupRatio, "future")
	assert.Equal(t, []string{"default", "future", "secret"}, got.Pricing[0].EnableGroup)
}

func TestCatalogProjectionVersionIsStableAndChangesWithPublicProjection(t *testing.T) {
	first, err := buildCatalogProjection(catalogProjectionFixture(), selectedCatalogVisibility("default"))
	require.NoError(t, err)
	source := catalogProjectionFixture()
	source.GroupDescriptions = map[string]string{"secret": "Secret", "default": "Default"}
	source.GroupRatio = map[string]float64{"secret": 3, "default": 1}
	second, err := buildCatalogProjection(source, selectedCatalogVisibility("default", "removed-stale"))
	require.NoError(t, err)
	assert.Equal(t, first.Version, second.Version)

	hidden, err := buildCatalogProjection(catalogProjectionFixture(), selectedCatalogVisibility("secret"))
	require.NoError(t, err)
	assert.NotEqual(t, first.Version, hidden.Version)
}

func TestCatalogProjectionVersionIgnoresLegacyMarkerPlacement(t *testing.T) {
	firstSource := catalogProjectionFixture()
	firstSource.Pricing[0].PricingVersion = "legacy-v1"
	secondSource := catalogProjectionFixture()
	secondSource.Pricing[1].PricingVersion = "legacy-v1"

	first, err := buildCatalogProjection(firstSource, model_catalog_setting.EnabledVisibility())
	require.NoError(t, err)
	second, err := buildCatalogProjection(secondSource, model_catalog_setting.EnabledVisibility())
	require.NoError(t, err)

	assert.Equal(t, first.Version, second.Version)
	assert.Equal(t, first.Pricing, second.Pricing)
	require.NotEmpty(t, first.Pricing)
	assert.Equal(t, "legacy-v1", first.Pricing[0].PricingVersion)
	for _, item := range first.Pricing[1:] {
		assert.Empty(t, item.PricingVersion)
	}
}

func TestCatalogPricingSnapshotCloneDoesNotShareMutableData(t *testing.T) {
	original, err := buildCatalogProjection(catalogProjectionFixture(), selectedCatalogVisibility("default"))
	require.NoError(t, err)
	cacheRatio := 0.5
	original.Pricing[0].CacheRatio = &cacheRatio
	clone := cloneCatalogPricingSnapshot(original)
	clone.Pricing[0].EnableGroup[0] = "mutated"
	*clone.Pricing[0].CacheRatio = 9
	clone.AllGroup["default"] = "Mutated"
	clone.GroupRatio["default"] = 9
	clone.SupportedEndpoints[string(constant.EndpointTypeOpenAI)] = common.EndpointInfo{Path: "/mutated"}

	assert.Equal(t, []string{"default"}, original.Pricing[0].EnableGroup)
	assert.Equal(t, 0.5, *original.Pricing[0].CacheRatio)
	assert.Equal(t, "Default", original.AllGroup["default"])
	assert.Equal(t, 1.0, original.GroupRatio["default"])
	assert.Equal(t, "/v1/chat/completions", original.SupportedEndpoints[string(constant.EndpointTypeOpenAI)].Path)
}

func TestCatalogPricingSourceCloneDoesNotShareMutableData(t *testing.T) {
	original := catalogProjectionFixture()
	clone := clonePricingSourceSnapshot(original)
	clone.Pricing[0].EnableGroup[0] = "mutated"
	clone.Pricing[0].SupportedEndpointTypes[0] = "mutated"
	clone.Vendors[0].Name = "Mutated"
	clone.GroupRatio["default"] = 9
	clone.GroupDescriptions["default"] = "Mutated"
	clone.SupportedEndpoints[string(constant.EndpointTypeOpenAI)] = common.EndpointInfo{Path: "/mutated"}

	assert.Equal(t, []string{"default", "secret"}, original.Pricing[0].EnableGroup)
	assert.Equal(t, []constant.EndpointType{constant.EndpointTypeOpenAI}, original.Pricing[0].SupportedEndpointTypes)
	assert.Equal(t, "Visible Vendor", original.Vendors[0].Name)
	assert.Equal(t, 1.0, original.GroupRatio["default"])
	assert.Equal(t, "Default", original.GroupDescriptions["default"])
	assert.Equal(t, "/v1/chat/completions", original.SupportedEndpoints[string(constant.EndpointTypeOpenAI)].Path)
}

func TestCatalogProjectionRefreshFailureKeepsLastValidSnapshot(t *testing.T) {
	resetCatalogPricingTestState(t)
	source := catalogProjectionFixture()
	loadCatalogPricingSource = func() (PricingSourceSnapshot, error) {
		return source, nil
	}
	first, err := GetCatalogPricingSnapshot()
	require.NoError(t, err)

	loadCatalogPricingSource = func() (PricingSourceSnapshot, error) {
		return PricingSourceSnapshot{}, errors.New("source unavailable")
	}
	InvalidateCatalogPricing()
	retained, err := GetCatalogPricingSnapshot()
	require.NoError(t, err)
	assert.Equal(t, first.Version, retained.Version)
}

func TestCatalogProjectionRefreshFailureWithoutHistoryReturnsError(t *testing.T) {
	resetCatalogPricingTestState(t)
	loadCatalogPricingSource = func() (PricingSourceSnapshot, error) {
		return PricingSourceSnapshot{}, errors.New("source unavailable")
	}
	_, err := GetCatalogPricingSnapshot()
	require.ErrorContains(t, err, "source unavailable")
}

func TestCatalogProjectionMalformedStoredVisibilityFailsClosed(t *testing.T) {
	resetCatalogPricingTestState(t)
	setCatalogVisibilityOptionForTest(t, `{"version":1,"mode":"broken","groups":[]}`)
	loadCatalogPricingSource = func() (PricingSourceSnapshot, error) {
		return catalogProjectionFixture(), nil
	}

	got, err := GetCatalogPricingSnapshot()
	require.NoError(t, err)
	assert.Empty(t, got.Pricing)
	assert.Empty(t, got.AllGroup)
	assert.Empty(t, got.GroupRatio)
	assert.NotEmpty(t, got.Version)
}

func TestCatalogAdminStateRefreshFailureKeepsLastValidState(t *testing.T) {
	resetCatalogPricingTestState(t)
	raw := mustCatalogJSON(t, model_catalog_setting.EnabledVisibility())
	loadCatalogVisibility = func() (string, bool, error) {
		return raw, true, nil
	}
	loadCatalogUserGroups = func() (map[string]string, error) {
		return map[string]string{"Admin": "Administrators"}, nil
	}
	loadCatalogPricingSource = func() (PricingSourceSnapshot, error) {
		return catalogProjectionFixture(), nil
	}

	first, err := GetCatalogAdminState()
	require.NoError(t, err)
	require.NotNil(t, first.Config)
	first.Config.Rules["default"] = model_catalog_setting.GroupRule{
		Mode:       model_catalog_setting.ModeSelected,
		UserGroups: []string{"Admin"},
	}

	loadCatalogPricingSource = func() (PricingSourceSnapshot, error) {
		return PricingSourceSnapshot{}, errors.New("temporary pricing failure")
	}
	loadCatalogVisibility = func() (string, bool, error) {
		return "", false, errors.New("temporary database failure")
	}

	retained, err := GetCatalogAdminState()
	require.NoError(t, err)
	require.NotNil(t, retained.Config)
	assert.Equal(t, first.ETag, retained.ETag)
	assert.Equal(t, first.Summary, retained.Summary)
	assert.NotEqual(t, model_catalog_setting.ModeSelected, retained.Config.Rules["default"].Mode)
}

func resetCatalogPricingTestState(t *testing.T) {
	t.Helper()
	previousLoader := loadCatalogPricingSource
	previousVisibilityLoader := loadCatalogVisibility
	previousUserGroupLoader := loadCatalogUserGroups
	catalogPricingPointer.Store(nil)
	catalogAdminStatePointer.Store(nil)
	catalogInvalidationEpoch.Store(0)
	setCatalogVisibilityOptionForTest(t, "")
	t.Cleanup(func() {
		loadCatalogPricingSource = previousLoader
		loadCatalogVisibility = previousVisibilityLoader
		loadCatalogUserGroups = previousUserGroupLoader
		catalogPricingPointer.Store(nil)
		catalogAdminStatePointer.Store(nil)
		catalogInvalidationEpoch.Store(0)
	})
}

func setCatalogVisibilityOptionForTest(t *testing.T, value string) {
	t.Helper()
	common.OptionMapRWMutex.Lock()
	if common.OptionMap == nil {
		common.OptionMap = make(map[string]string)
	}
	previous, existed := common.OptionMap[model_catalog_setting.OptionKey]
	common.OptionMap[model_catalog_setting.OptionKey] = value
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		defer common.OptionMapRWMutex.Unlock()
		if existed {
			common.OptionMap[model_catalog_setting.OptionKey] = previous
		} else {
			delete(common.OptionMap, model_catalog_setting.OptionKey)
		}
	})
}

func TestCatalogCacheInvalidatesForModelAndVendorMetadata(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&Model{}, &Vendor{}))
	require.NoError(t, DB.Exec("DELETE FROM models").Error)
	require.NoError(t, DB.Exec("DELETE FROM vendors").Error)

	beforeModel := catalogInvalidationEpoch.Load()
	modelMeta := Model{ModelName: "catalog-invalidation-model", Status: 1}
	require.NoError(t, modelMeta.Insert())
	assert.Greater(t, catalogInvalidationEpoch.Load(), beforeModel)

	beforeVendor := catalogInvalidationEpoch.Load()
	vendor := Vendor{Name: "Catalog Invalidation Vendor", Status: 1}
	require.NoError(t, vendor.Insert())
	assert.Greater(t, catalogInvalidationEpoch.Load(), beforeVendor)
}

func TestCatalogCacheInvalidatesForCatalogAffectingOption(t *testing.T) {
	previous := ratio_setting.GroupRatio2JSONString()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(previous))
	})
	before := catalogInvalidationEpoch.Load()
	require.NoError(t, applyOptionRuntimeValue("GroupRatio", `{"default":1,"catalog-test":2}`))
	assert.Greater(t, catalogInvalidationEpoch.Load(), before)
}
