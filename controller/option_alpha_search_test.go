package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/require"
)

func TestValidateOperationSettingOptionAcceptsOnlyStrictAlphaSearchBooleans(t *testing.T) {
	key := operation_setting.AlphaSearchSettingConfigName + ".enabled"
	for _, value := range []string{"true", "false"} {
		require.NoError(t, validateOperationSettingOption(key, value), value)
	}
	for _, value := range []string{"yes", "1", "TRUE", " true ", ""} {
		require.Error(t, validateOperationSettingOption(key, value), value)
	}
	require.Error(t, validateOperationSettingOption(
		operation_setting.AlphaSearchSettingConfigName+".unknown",
		"true",
	))
}
