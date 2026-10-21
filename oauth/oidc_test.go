package oauth

import (
	"testing"

	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/stretchr/testify/require"
)

func TestOIDCProviderGetName(t *testing.T) {
	settings := system_setting.GetOIDCSettings()
	original := settings.DisplayName
	t.Cleanup(func() { settings.DisplayName = original })
	provider := &OIDCProvider{}

	settings.DisplayName = ""
	require.Equal(t, "OIDC", provider.GetName())
	settings.DisplayName = "  Company SSO  "
	require.Equal(t, "Company SSO", provider.GetName())
}
