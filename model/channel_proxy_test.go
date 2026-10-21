package model

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/require"
)

func TestChannelValidateSettingsRejectsProxyURLsWithLegacySuffixes(t *testing.T) {
	channel := Channel{}
	channel.SetSetting(dto.ChannelSettings{Proxy: "http://proxy.example:8080/path"})

	err := channel.ValidateSettings()
	require.Error(t, err)
	require.Contains(t, err.Error(), "proxy")
}

func TestChannelValidateSettingsRejectsInvalidHTTPTransport(t *testing.T) {
	tests := []struct {
		name    string
		setting dto.ChannelSettings
		wantErr string
	}{
		{
			name:    "auto with shards",
			setting: dto.ChannelSettings{HTTPProtocol: dto.HTTPProtocolAuto, HTTP2ConnectionShards: 4},
		},
		{
			name:    "unsupported protocol",
			setting: dto.ChannelSettings{HTTPProtocol: "http3"},
			wantErr: "http_protocol",
		},
		{
			name:    "http1 with multiple shards",
			setting: dto.ChannelSettings{HTTPProtocol: dto.HTTPProtocolHTTP1, HTTP2ConnectionShards: 2},
			wantErr: "http2_connection_shards",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			channel := Channel{}
			channel.SetSetting(test.setting)
			err := channel.ValidateSettings()
			if test.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, test.wantErr)
		})
	}
}
