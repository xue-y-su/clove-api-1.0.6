package sub2api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/channel/claude"
	"github.com/QuantumNous/new-api/relay/channel/gemini"
	"github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSub2APISelectsProtocolDelegate(t *testing.T) {
	adaptor := &Adaptor{}
	require.True(t, adaptor.SupportsAlphaSearch())

	require.IsType(t, &openai.Adaptor{}, adaptor.delegateForFormat(types.RelayFormatOpenAI))
	require.IsType(t, &openai.Adaptor{}, adaptor.delegateForFormat(types.RelayFormatOpenAIResponses))
	require.IsType(t, &openai.Adaptor{}, adaptor.delegateForFormat(types.RelayFormatOpenAIAlphaSearch))
	require.IsType(t, &claude.Adaptor{}, adaptor.delegateForFormat(types.RelayFormatClaude))
	require.IsType(t, &gemini.Adaptor{}, adaptor.delegateForFormat(types.RelayFormatGemini))
}

func TestSub2APISetupRequestHeaderUsesProtocolAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name   string
		format types.RelayFormat
		assert func(*testing.T, http.Header)
	}{
		{
			name:   "openai",
			format: types.RelayFormatOpenAI,
			assert: func(t *testing.T, headers http.Header) {
				require.Equal(t, "Bearer secret", headers.Get("Authorization"))
				require.Empty(t, headers.Get("x-api-key"))
				require.Empty(t, headers.Get("x-goog-api-key"))
			},
		},
		{
			name:   "claude",
			format: types.RelayFormatClaude,
			assert: func(t *testing.T, headers http.Header) {
				require.Equal(t, "Bearer secret", headers.Get("Authorization"))
				require.Equal(t, "secret", headers.Get("x-api-key"))
				require.Equal(t, "2023-06-01", headers.Get("anthropic-version"))
			},
		},
		{
			name:   "gemini",
			format: types.RelayFormatGemini,
			assert: func(t *testing.T, headers http.Header) {
				require.Equal(t, "Bearer secret", headers.Get("Authorization"))
				require.Equal(t, "secret", headers.Get("x-goog-api-key"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
			info := &relaycommon.RelayInfo{
				RelayFormat: tt.format,
				ChannelMeta: &relaycommon.ChannelMeta{
					ApiKey: "secret",
				},
			}
			headers := make(http.Header)
			require.NoError(t, (&Adaptor{}).SetupRequestHeader(ctx, &headers, info))
			tt.assert(t, headers)
		})
	}
}

func TestSub2APIUsesFinalFormatForAuthAndResponseDelegation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	info := &relaycommon.RelayInfo{
		RelayFormat:             types.RelayFormatOpenAI,
		FinalRequestRelayFormat: types.RelayFormatClaude,
		ChannelMeta: &relaycommon.ChannelMeta{
			ApiKey: "secret",
		},
	}
	headers := make(http.Header)

	require.NoError(t, (&Adaptor{}).SetupRequestHeader(ctx, &headers, info))
	require.Equal(t, "secret", headers.Get("x-api-key"))
}

func TestSub2APIAlphaSearchURLAndUnsupportedEndpoints(t *testing.T) {
	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeAlphaSearch,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:    constant.ChannelTypeSub2API,
			ChannelBaseUrl: "https://sub2api.example/",
		},
	}

	url, err := adaptor.GetRequestURL(info)
	require.NoError(t, err)
	require.Equal(t, "https://sub2api.example/v1/alpha/search", url)

	result, err := adaptor.ConvertRerankRequest(nil, relayconstant.RelayModeRerank, dto.RerankRequest{})
	require.Nil(t, result)
	require.Error(t, err)

	audio, err := adaptor.ConvertAudioRequest(nil, info, dto.AudioRequest{})
	require.Nil(t, audio)
	require.Error(t, err)
}
