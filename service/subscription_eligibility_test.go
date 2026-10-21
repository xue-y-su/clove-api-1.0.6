package service

import (
	"testing"

	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/stretchr/testify/require"
)

func TestSubscriptionAppliesOnlyToItsOwnGroup(t *testing.T) {
	previousGroups := setting.UserUsableGroups2JSONString()
	previousAutoGroups := setting.AutoGroups2JsonString()
	previousSpecialGroups := ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup.ReadAll()
	t.Cleanup(func() {
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(previousGroups))
		require.NoError(t, setting.UpdateAutoGroupsByJsonString(previousAutoGroups))
		special := ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup
		special.Clear()
		special.AddAll(previousSpecialGroups)
	})

	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default","pro":"Pro","standard":"Standard"}`))
	ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup.Clear()
	ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup.Set("pro", map[string]string{"+:pro-child": "Child"})

	require.True(t, SubscriptionAppliesToGroups("", []string{"unrelated"}))
	require.True(t, SubscriptionAppliesToGroups("pro", []string{"pro"}))
	require.False(t, SubscriptionAppliesToGroups("pro", []string{"pro-child"}))
	require.False(t, SubscriptionAppliesToGroups("pro", []string{"standard"}))
	require.False(t, SubscriptionAppliesToGroups("pro", []string{"default"}))

	ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup.Set("pro", map[string]string{"-:pro-child": "Removed"})
	require.False(t, SubscriptionAppliesToGroups("pro", []string{"pro-child"}))
}

func TestResolveTokenEffectiveGroupsDeduplicatesAutoCandidates(t *testing.T) {
	previousGroups := setting.UserUsableGroups2JSONString()
	previousAutoGroups := setting.AutoGroups2JsonString()
	previousRatios := ratio_setting.GroupRatio2JSONString()
	t.Cleanup(func() {
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(previousGroups))
		require.NoError(t, setting.UpdateAutoGroupsByJsonString(previousAutoGroups))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(previousRatios))
	})

	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default","pro":"Pro"}`))
	require.NoError(t, setting.UpdateAutoGroupsByJsonString(`["pro","default","pro"]`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"pro":1}`))
	require.Equal(t, []string{"pro", "default"}, ResolveTokenEffectiveGroups("pro", "auto"))
	require.Equal(t, []string{"pro"}, ResolveTokenEffectiveGroups("pro", ""))
}
