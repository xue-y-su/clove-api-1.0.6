package sub2api

import (
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/claude"
	"github.com/QuantumNous/new-api/relay/channel/gemini"
	"github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

var errEndpointNotSupported = errors.New("endpoint not supported by Sub2API")

type Adaptor struct {
	openAI openai.Adaptor
	claude claude.Adaptor
	gemini gemini.Adaptor
}

func (*Adaptor) SupportsAlphaSearch() bool { return true }

func (a *Adaptor) Init(info *relaycommon.RelayInfo) {
	a.openAI.Init(info)
	a.claude.Init(info)
	a.gemini.Init(info)
}

func (a *Adaptor) delegateForFormat(format types.RelayFormat) channel.Adaptor {
	switch format {
	case types.RelayFormatClaude:
		return &a.claude
	case types.RelayFormatGemini:
		return &a.gemini
	default:
		return &a.openAI
	}
}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	baseURL := strings.TrimRight(info.ChannelBaseUrl, "/")
	if info.RelayMode == relayconstant.RelayModeAlphaSearch {
		return relaycommon.GetFullRequestURL(baseURL, "/v1/alpha/search", info.ChannelType), nil
	}
	return relaycommon.GetFullRequestURL(baseURL, info.RequestURLPath, info.ChannelType), nil
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, headers *http.Header, info *relaycommon.RelayInfo) error {
	channel.SetupApiRequestHeader(info, c, headers)
	headers.Set("Authorization", "Bearer "+info.ApiKey)

	switch info.GetFinalRequestRelayFormat() {
	case types.RelayFormatClaude:
		headers.Set("x-api-key", info.ApiKey)
		if headers.Get("anthropic-version") == "" {
			version := c.Request.Header.Get("anthropic-version")
			if version == "" {
				version = "2023-06-01"
			}
			headers.Set("anthropic-version", version)
		}
	case types.RelayFormatGemini:
		headers.Set("x-goog-api-key", info.ApiKey)
	}
	return nil
}

func (a *Adaptor) ConvertOpenAIRequest(_ *gin.Context, _ *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error) {
	if request == nil {
		return nil, errors.New("request is nil")
	}
	return request, nil
}

func (a *Adaptor) ConvertOpenAIResponsesRequest(_ *gin.Context, _ *relaycommon.RelayInfo, request dto.OpenAIResponsesRequest) (any, error) {
	return request, nil
}

func (a *Adaptor) ConvertEmbeddingRequest(_ *gin.Context, _ *relaycommon.RelayInfo, request dto.EmbeddingRequest) (any, error) {
	return request, nil
}

func (a *Adaptor) ConvertClaudeRequest(_ *gin.Context, _ *relaycommon.RelayInfo, request *dto.ClaudeRequest) (any, error) {
	if request == nil {
		return nil, errors.New("request is nil")
	}
	return request, nil
}

func (a *Adaptor) ConvertGeminiRequest(_ *gin.Context, _ *relaycommon.RelayInfo, request *dto.GeminiChatRequest) (any, error) {
	if request == nil {
		return nil, errors.New("request is nil")
	}
	return request, nil
}

func (a *Adaptor) ConvertImageRequest(_ *gin.Context, _ *relaycommon.RelayInfo, request dto.ImageRequest) (any, error) {
	return request, nil
}

func (a *Adaptor) ConvertRerankRequest(_ *gin.Context, _ int, _ dto.RerankRequest) (any, error) {
	return nil, errEndpointNotSupported
}

func (a *Adaptor) ConvertAudioRequest(_ *gin.Context, _ *relaycommon.RelayInfo, _ dto.AudioRequest) (io.Reader, error) {
	return nil, errEndpointNotSupported
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, body io.Reader) (any, error) {
	return channel.DoApiRequest(a, c, info, body)
}

func (a *Adaptor) DoResponse(c *gin.Context, response *http.Response, info *relaycommon.RelayInfo) (any, *types.NewAPIError) {
	return a.delegateForFormat(info.GetFinalRequestRelayFormat()).DoResponse(c, response, info)
}

func (a *Adaptor) GetModelList() []string { return ModelList }

func (a *Adaptor) GetChannelName() string { return ChannelName }
