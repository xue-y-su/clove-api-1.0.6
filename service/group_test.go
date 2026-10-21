package service

import (
	"testing"

	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/stretchr/testify/require"
)

func TestGetUserUsableGroupsAppliesAvailabilityOverridesDeterministically(t *testing.T) {
	previousGroups := setting.UserUsableGroups2JSONString()
	previousRatios := ratio_setting.GroupRatio2JSONString()
	previousSpecialGroups := ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup.ReadAll()
	t.Cleanup(func() {
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(previousGroups))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(previousRatios))
		special := ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup
		special.Clear()
		special.AddAll(previousSpecialGroups)
	})

	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default","standard":"Standard"}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"standard":1,"premium":1,"legacy":1,"vip":1}`))
	special := ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup
	special.Clear()
	special.Set("vip", map[string]string{
		"+:premium":  "Premium",
		"+:standard": "Added before removal",
		"+:deleted":  "Deleted pricing group",
		"-:standard": "Removed",
		"legacy":     "Legacy addition",
		"-:vip":      "The user's own group is always restored",
	})

	groups := GetUserUsableGroups("vip")
	require.Equal(t, "Default", groups["default"])
	require.Equal(t, "Premium", groups["premium"])
	require.Equal(t, "Legacy addition", groups["legacy"])
	require.NotContains(t, groups, "deleted")
	require.NotContains(t, groups, "standard")
	require.Equal(t, "用户分组", groups["vip"])
}
