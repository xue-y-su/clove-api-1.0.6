package channel_test

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaychannel "github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/aws"
	"github.com/QuantumNous/new-api/relay/channel/baidu"
	"github.com/QuantumNous/new-api/relay/channel/claude"
	"github.com/QuantumNous/new-api/relay/channel/cloudflare"
	"github.com/QuantumNous/new-api/relay/channel/cohere"
	"github.com/QuantumNous/new-api/relay/channel/deepseek"
	"github.com/QuantumNous/new-api/relay/channel/dify"
	"github.com/QuantumNous/new-api/relay/channel/gemini"
	"github.com/QuantumNous/new-api/relay/channel/jina"
	"github.com/QuantumNous/new-api/relay/channel/minimax"
	"github.com/QuantumNous/new-api/relay/channel/mistral"
	"github.com/QuantumNous/new-api/relay/channel/mokaai"
	"github.com/QuantumNous/new-api/relay/channel/ollama"
	"github.com/QuantumNous/new-api/relay/channel/palm"
	"github.com/QuantumNous/new-api/relay/channel/perplexity"
	"github.com/QuantumNous/new-api/relay/channel/tencent"
	"github.com/QuantumNous/new-api/relay/channel/vertex"
	"github.com/QuantumNous/new-api/relay/channel/volcengine"
	"github.com/QuantumNous/new-api/relay/channel/xai"
	"github.com/QuantumNous/new-api/relay/channel/xunfei"
	"github.com/QuantumNous/new-api/relay/channel/zhipu"
	"github.com/QuantumNous/new-api/relay/channel/zhipu_4v"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type claudeRequestConverter interface {
	ConvertClaudeRequest(*gin.Context, *relaycommon.RelayInfo, *dto.ClaudeRequest) (any, error)
}

type rerankRequestConverter interface {
	ConvertRerankRequest(*gin.Context, int, dto.RerankRequest) (any, error)
}

func TestUnsupportedClaudeConvertersReturnErrorsInsteadOfPanicking(t *testing.T) {
	adaptors := map[string]claudeRequestConverter{
		"baidu":      &baidu.Adaptor{},
		"cloudflare": &cloudflare.Adaptor{},
		"cohere":     &cohere.Adaptor{},
		"dify":       &dify.Adaptor{},
		"jina":       &jina.Adaptor{},
		"mistral":    &mistral.Adaptor{},
		"mokaai":     &mokaai.Adaptor{},
		"palm":       &palm.Adaptor{},
		"tencent":    &tencent.Adaptor{},
		"xunfei":     &xunfei.Adaptor{},
		"zhipu":      &zhipu.Adaptor{},
	}

	for name, adaptor := range adaptors {
		t.Run(name, func(t *testing.T) {
			var result any
			var err error
			require.NotPanics(t, func() {
				result, err = adaptor.ConvertClaudeRequest(nil, nil, nil)
			})
			require.Nil(t, result)
			require.Error(t, err)
		})
	}
}

func TestUnsupportedRerankConvertersReturnErrors(t *testing.T) {
	adaptors := map[string]rerankRequestConverter{
		"aws":        &aws.Adaptor{},
		"baidu":      &baidu.Adaptor{},
		"claude":     &claude.Adaptor{},
		"deepseek":   &deepseek.Adaptor{},
		"dify":       &dify.Adaptor{},
		"gemini":     &gemini.Adaptor{},
		"minimax":    &minimax.Adaptor{},
		"mistral":    &mistral.Adaptor{},
		"mokaai":     &mokaai.Adaptor{},
		"ollama":     &ollama.Adaptor{},
		"palm":       &palm.Adaptor{},
		"perplexity": &perplexity.Adaptor{},
		"tencent":    &tencent.Adaptor{},
		"vertex":     &vertex.Adaptor{},
		"volcengine": &volcengine.Adaptor{},
		"xai":        &xai.Adaptor{},
		"xunfei":     &xunfei.Adaptor{},
		"zhipu":      &zhipu.Adaptor{},
		"zhipu_4v":   &zhipu_4v.Adaptor{},
	}

	for name, adaptor := range adaptors {
		t.Run(name, func(t *testing.T) {
			result, err := adaptor.ConvertRerankRequest(nil, 0, dto.RerankRequest{})
			require.Nil(t, result)
			require.ErrorIs(t, err, relaychannel.ErrUnsupportedRerankRequest)
		})
	}
}
