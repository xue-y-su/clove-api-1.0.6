package operation_setting

import (
	"testing"

	"github.com/QuantumNous/new-api/setting/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTokenSettingAllowsCustomKeysByDefault(t *testing.T) {
	assert.True(t, GetTokenSetting().AllowCustomKeys)
}

func TestTokenSettingCustomKeyPolicyLoadsFromConfigMap(t *testing.T) {
	previous := *GetTokenSetting()
	t.Cleanup(func() { *GetTokenSetting() = previous })

	require.NoError(t, config.UpdateConfigFromMap(GetTokenSetting(), map[string]string{
		"allow_custom_keys": "false",
	}))

	assert.False(t, GetTokenSetting().AllowCustomKeys)
}
