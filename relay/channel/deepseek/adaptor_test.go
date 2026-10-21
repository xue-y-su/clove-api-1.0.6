package deepseek

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/constant"
	"github.com/stretchr/testify/require"
)

func TestGetRequestURLResponses(t *testing.T) {
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{ChannelBaseUrl: "https://api.deepseek.com"},
		RelayMode:   constant.RelayModeResponses,
	}

	requestURL, err := (&Adaptor{}).GetRequestURL(info)
	require.NoError(t, err)
	require.Equal(t, "https://api.deepseek.com/responses", requestURL)
}

func TestConvertOpenAIResponsesRequest(t *testing.T) {
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "deepseek-v4-pro-max"},
	}

	convertedValue, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(nil, info, dto.OpenAIResponsesRequest{
		Model: "client-model",
	})
	require.NoError(t, err)
	converted := convertedValue.(dto.OpenAIResponsesRequest)
	require.Equal(t, "deepseek-v4-pro", converted.Model)
	require.NotNil(t, converted.Reasoning)
	require.Equal(t, "max", converted.Reasoning.Effort)
	require.Equal(t, "deepseek-v4-pro", info.UpstreamModelName)
	require.Equal(t, "max", info.ReasoningEffort)
}
