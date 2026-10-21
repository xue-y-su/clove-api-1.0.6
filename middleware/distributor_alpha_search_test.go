package middleware

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestChannelSupportsRequestPathRejectsUnsupportedAlphaSearchAffinityChannel(t *testing.T) {
	alphaCustom := &model.Channel{Type: constant.ChannelTypeAdvancedCustom}
	alphaCustom.SetOtherSettings(dto.ChannelOtherSettings{
		AdvancedCustom: &dto.AdvancedCustomConfig{Routes: []dto.AdvancedCustomRoute{
			{IncomingPath: "/v1/alpha/search", UpstreamPath: "/v1/alpha/search"},
		}},
	})

	require.False(t, channelSupportsRequestPath(
		&model.Channel{Type: constant.ChannelTypeOpenAI},
		"/v1/alpha/search",
		"gpt-alpha",
	))
	require.True(t, channelSupportsRequestPath(
		&model.Channel{Type: constant.ChannelTypeCodex},
		"/v1/alpha/search",
		"gpt-alpha",
	))
	require.True(t, channelSupportsRequestPath(
		&model.Channel{Type: constant.ChannelTypeSub2API},
		"/v1/alpha/search",
		"gpt-alpha",
	))
	require.True(t, channelSupportsRequestPath(alphaCustom, "/v1/alpha/search", "gpt-alpha"))
}
