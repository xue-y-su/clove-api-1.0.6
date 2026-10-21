package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/require"
)

func TestValidateOperationSettingOptionRejectsInvalidDiagnosticValue(t *testing.T) {
	key := operation_setting.UpstreamDiagnosticSettingConfigName + ".preview_chars"
	require.Error(t, validateOperationSettingOption(key, "255"))
	require.NoError(t, validateOperationSettingOption(key, "256"))
}
