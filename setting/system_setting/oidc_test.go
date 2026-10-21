package system_setting

import (
	"testing"

	"github.com/QuantumNous/new-api/setting/config"
	"github.com/stretchr/testify/require"
)

func TestOIDCSettingsGetEffectiveDisplayName(t *testing.T) {
	tests := []struct {
		name        string
		displayName string
		want        string
	}{
		{name: "blank", displayName: "", want: "OIDC"},
		{name: "whitespace", displayName: "   ", want: "OIDC"},
		{name: "custom", displayName: "  Company SSO  ", want: "Company SSO"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settings := OIDCSettings{DisplayName: tt.displayName}
			require.Equal(t, tt.want, settings.GetEffectiveDisplayName())
		})
	}
}

func TestOIDCSettingsDisplayNamePersistence(t *testing.T) {
	settings := &OIDCSettings{DisplayName: "Company SSO"}
	manager := config.NewConfigManager()
	manager.Register("oidc", settings)
	saved := make(map[string]string)

	require.NoError(t, manager.SaveToDB(func(key, value string) error {
		saved[key] = value
		return nil
	}))
	require.Equal(t, "Company SSO", saved["oidc.display_name"])

	settings.DisplayName = ""
	require.NoError(t, manager.LoadFromDB(saved))
	require.Equal(t, "Company SSO", settings.DisplayName)
}
