package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func channelForProxyValidation(rawProxy string) *model.Channel {
	channel := &model.Channel{
		Type: constant.ChannelTypeOpenAI,
		Key:  "test-key",
	}
	channel.SetSetting(dto.ChannelSettings{Proxy: rawProxy})
	return channel
}

func TestValidateChannelRejectsProxySuffixAndUnsupportedScheme(t *testing.T) {
	for _, rawProxy := range []string{
		"ftp://proxy.example",
		"http://proxy.example/path",
		"socks5://user:secret@proxy.example?x=1",
	} {
		t.Run(rawProxy, func(t *testing.T) {
			err := validateChannel(channelForProxyValidation(rawProxy), true)
			require.Error(t, err)
			require.Contains(t, err.Error(), "proxy")
			require.NotContains(t, err.Error(), "secret")
		})
	}
}

func TestValidateChannelAcceptsCredentialedProxyWithoutExposingIt(t *testing.T) {
	channel := channelForProxyValidation("https://user:secret@proxy.example:8443")
	err := validateChannel(channel, false)
	require.NoError(t, err)
}

func TestValidateChannelRejectsMissingChannel(t *testing.T) {
	err := validateChannel(nil, true)

	require.EqualError(t, err, "channel cannot be empty")
}
