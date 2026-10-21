package source_attribution_setting

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestSettingsToOptionsPersistsTargetUserID(t *testing.T) {
	values, err := SettingsToOptions(Settings{
		Enabled: true,
		Options: []Option{{
			Id: "partner", Name: "Partner", TargetUserID: 42, SortOrder: 1,
		}},
	})
	require.NoError(t, err)

	var options []map[string]any
	require.NoError(t, common.UnmarshalJsonStr(values[OptionsKey], &options))
	require.Len(t, options, 1)
	require.EqualValues(t, 42, options[0]["target_user_id"])
	require.NotContains(t, options[0], "target_aff_code")
}
