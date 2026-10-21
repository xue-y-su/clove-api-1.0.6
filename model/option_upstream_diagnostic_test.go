package model

import (
	"testing"

	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/require"
)

func TestHandleConfigUpdateRebuildsUpstreamDiagnosticSnapshot(t *testing.T) {
	setting := operation_setting.GetUpstreamDiagnosticSetting()
	previous := *setting
	t.Cleanup(func() {
		*setting = previous
		operation_setting.RebuildUpstreamDiagnosticSnapshot()
	})

	handled, err := handleConfigUpdate(
		operation_setting.UpstreamDiagnosticSettingConfigName+".preview_chars",
		"4096",
	)
	require.NoError(t, err)
	require.True(t, handled)
	require.Equal(t, 4096, operation_setting.CurrentUpstreamDiagnosticSnapshot().PreviewChars)
}
