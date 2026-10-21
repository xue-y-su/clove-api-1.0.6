package ip_security_setting

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func withIPSecurityOptions(t *testing.T, options map[string]string) {
	t.Helper()
	common.OptionMapRWMutex.Lock()
	previous := common.OptionMap
	common.OptionMap = options
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = previous
		common.OptionMapRWMutex.Unlock()
	})
}

func TestIPSecuritySettingsUseProductionDefaults(t *testing.T) {
	withIPSecurityOptions(t, map[string]string{})

	settings := GetSettings()

	assert.True(t, settings.Enabled)
	assert.Equal(t, 90, settings.RetentionDays)
	assert.Equal(t, 60, settings.ProjectionIntervalSeconds)
	assert.Equal(t, 500, settings.ProjectionBatchSize)
	assert.Equal(t, 2, settings.SharedRegistrationUserThreshold)
	assert.Equal(t, 2, settings.SharedAPIUserThreshold)
	assert.Equal(t, 10, settings.UnauthorizedThreshold)
}

func TestIPSecuritySettingsValidateRuntimeBounds(t *testing.T) {
	valid := Settings{
		Enabled:                         true,
		RetentionDays:                   90,
		ProjectionIntervalSeconds:       60,
		ProjectionBatchSize:             500,
		SharedRegistrationUserThreshold: 3,
		SharedAPIUserThreshold:          5,
		UnauthorizedThreshold:           10,
	}
	require.NoError(t, ValidateSettings(valid))

	tests := []Settings{
		{Enabled: true, RetentionDays: -1, ProjectionIntervalSeconds: 60, ProjectionBatchSize: 500},
		{Enabled: true, RetentionDays: 3651, ProjectionIntervalSeconds: 60, ProjectionBatchSize: 500},
		{Enabled: true, RetentionDays: 90, ProjectionIntervalSeconds: 4, ProjectionBatchSize: 500},
		{Enabled: true, RetentionDays: 90, ProjectionIntervalSeconds: 3601, ProjectionBatchSize: 500},
		{Enabled: true, RetentionDays: 90, ProjectionIntervalSeconds: 60, ProjectionBatchSize: 9},
		{Enabled: true, RetentionDays: 90, ProjectionIntervalSeconds: 60, ProjectionBatchSize: 5001},
		{Enabled: true, RetentionDays: 90, ProjectionIntervalSeconds: 60, ProjectionBatchSize: 500, SharedRegistrationUserThreshold: 0, SharedAPIUserThreshold: 5, UnauthorizedThreshold: 10},
		{Enabled: true, RetentionDays: 90, ProjectionIntervalSeconds: 60, ProjectionBatchSize: 500, SharedRegistrationUserThreshold: 3, SharedAPIUserThreshold: 0, UnauthorizedThreshold: 10},
		{Enabled: true, RetentionDays: 90, ProjectionIntervalSeconds: 60, ProjectionBatchSize: 500, SharedRegistrationUserThreshold: 3, SharedAPIUserThreshold: 5, UnauthorizedThreshold: 0},
	}
	for _, settings := range tests {
		require.Error(t, ValidateSettings(settings))
	}
}

func TestIPSecuritySettingsRoundTripOptions(t *testing.T) {
	options := SettingsToOptions(Settings{
		Enabled:                         false,
		RetentionDays:                   0,
		ProjectionIntervalSeconds:       15,
		ProjectionBatchSize:             100,
		SharedRegistrationUserThreshold: 7,
		SharedAPIUserThreshold:          11,
		UnauthorizedThreshold:           13,
	})
	withIPSecurityOptions(t, options)

	settings := GetSettings()

	assert.False(t, settings.Enabled)
	assert.Zero(t, settings.RetentionDays)
	assert.Equal(t, 15, settings.ProjectionIntervalSeconds)
	assert.Equal(t, 100, settings.ProjectionBatchSize)
	assert.Equal(t, 7, settings.SharedRegistrationUserThreshold)
	assert.Equal(t, 11, settings.SharedAPIUserThreshold)
	assert.Equal(t, 13, settings.UnauthorizedThreshold)
}
