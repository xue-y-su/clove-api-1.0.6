package openai

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestOpenRouterPlaygroundReasoningNoneDisablesReasoning(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(nil)
	info := &relaycommon.RelayInfo{
		OriginModelName: "gpt-5-mini",
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       constant.ChannelTypeOpenRouter,
			UpstreamModelName: "gpt-5-mini",
		},
	}
	request := &dto.GeneralOpenAIRequest{
		Model:           "gpt-5-mini",
		ReasoningEffort: "none",
	}

	converted, err := (&Adaptor{}).ConvertOpenAIRequest(ctx, info, request)

	require.NoError(t, err)
	convertedRequest := converted.(*dto.GeneralOpenAIRequest)
	require.JSONEq(t, `{"enabled":false}`, string(convertedRequest.Reasoning))
	require.Empty(t, convertedRequest.ReasoningEffort)
}
