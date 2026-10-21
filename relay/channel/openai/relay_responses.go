package openai

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

func isBillableResponsesOutput(output *dto.ResponsesOutput) bool {
	if output == nil {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(output.Status)) {
	case "failed", "cancelled", "canceled", "incomplete", "partial":
		return false
	}
	if output.Type == dto.ResponsesOutputTypeImageGenerationCall {
		return strings.TrimSpace(output.Result) != ""
	}
	return true
}

func confirmResponsesOutput(info *relaycommon.RelayInfo, output *dto.ResponsesOutput, outputIndex *int) error {
	if info == nil || !isBillableResponsesOutput(output) {
		return nil
	}
	name := ""
	switch output.Type {
	case dto.BuildInCallWebSearchCall:
		name = dto.BuildInToolWebSearchPreview
	case dto.BuildInCallFileSearchCall:
		name = dto.BuildInToolFileSearch
	case dto.BuildInCallFunctionCall, dto.BuildInCallToolUse:
		name = strings.TrimSpace(output.Name)
	case dto.ResponsesOutputTypeImageGenerationCall:
		name = dto.BuildInToolImageGeneration
	default:
		return nil
	}
	if name == "" {
		return nil
	}
	payload, _ := common.Marshal(output)
	aliases := make([]string, 0, 1)
	if output.CallId != "" {
		aliases = append(aliases, output.CallId)
	}
	return info.ConfirmToolEvent(relaycommon.ToolEvent{
		Name: name, StableID: output.ID, Aliases: aliases, OutputIndex: outputIndex,
		Payload: payload, ImageQuality: output.Quality, ImageSize: output.Size,
	})
}

func confirmResponsesStreamEvidence(info *relaycommon.RelayInfo, response *dto.ResponsesStreamResponse) error {
	if response == nil {
		return nil
	}
	switch response.Type {
	case dto.ResponsesOutputTypeItemDone:
		if response.Item != nil && response.Item.ID == "" {
			response.Item.ID = response.ItemID
		}
		return confirmResponsesOutput(info, response.Item, response.OutputIndex)
	case "response.completed", "response.done":
		if response.Response == nil || relaycommon.IsNonBillableResponsesStatus(response.Response.Status) {
			return nil
		}
		for index := range response.Response.Output {
			if err := confirmResponsesOutput(info, &response.Response.Output[index], &index); err != nil {
				return err
			}
		}
	}
	return nil
}

func OaiResponsesHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	defer service.CloseResponseBodyGracefully(resp)

	// read response body
	var responsesResponse dto.OpenAIResponsesResponse
	responseBody, err := io.ReadAll(common.LimitUpstreamResponseBody(resp.Body))
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}
	err = common.Unmarshal(responseBody, &responsesResponse)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	if oaiError := responsesResponse.GetOpenAIError(); oaiError != nil && oaiError.Type != "" {
		return nil, types.WithOpenAIError(*oaiError, resp.StatusCode)
	}

	if !relaycommon.IsNonBillableResponsesStatus(responsesResponse.Status) {
		for index := range responsesResponse.Output {
			if err := confirmResponsesOutput(info, &responsesResponse.Output[index], &index); err != nil {
				return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
			}
		}
	}

	// 写入新的 response body
	service.IOCopyBytesGracefully(c, resp, responseBody)

	// compute usage
	usage := dto.Usage{}
	if responsesResponse.Usage != nil {
		usage.PromptTokens = responsesResponse.Usage.InputTokens
		usage.CompletionTokens = responsesResponse.Usage.OutputTokens
		usage.TotalTokens = responsesResponse.Usage.TotalTokens
		if responsesResponse.Usage.InputTokensDetails != nil {
			usage.PromptTokensDetails.CachedTokens = responsesResponse.Usage.InputTokensDetails.CachedTokens
			usage.PromptTokensDetails.CacheWriteTokens = responsesResponse.Usage.InputTokensDetails.CacheWriteTokens
		}
	}
	return &usage, nil
}

func OaiResponsesStreamHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	if resp == nil || resp.Body == nil {
		logger.LogError(c, "invalid response or response body")
		return nil, types.NewError(fmt.Errorf("invalid response"), types.ErrorCodeBadResponse)
	}

	defer service.CloseResponseBodyGracefully(resp)

	var usage = &dto.Usage{}
	var responseTextBuilder strings.Builder

	helper.StreamScannerHandler(c, resp, info, func(data string, sr *helper.StreamResult) {

		// 检查当前数据是否包含 completed 状态和 usage 信息
		var streamResponse dto.ResponsesStreamResponse
		if err := common.UnmarshalJsonStr(data, &streamResponse); err != nil {
			logger.LogError(c, "failed to unmarshal stream response: "+err.Error())
			sr.Error(err)
			return
		}
		if err := confirmResponsesStreamEvidence(info, &streamResponse); err != nil {
			logger.LogError(c, "failed to confirm tool billing evidence: "+err.Error())
			sr.Error(err)
			return
		}
		sendResponsesStreamData(c, streamResponse, data)
		switch streamResponse.Type {
		case "response.completed":
			if streamResponse.Response != nil {
				if streamResponse.Response.Usage != nil {
					if streamResponse.Response.Usage.InputTokens != 0 {
						usage.PromptTokens = streamResponse.Response.Usage.InputTokens
					}
					if streamResponse.Response.Usage.OutputTokens != 0 {
						usage.CompletionTokens = streamResponse.Response.Usage.OutputTokens
					}
					if streamResponse.Response.Usage.TotalTokens != 0 {
						usage.TotalTokens = streamResponse.Response.Usage.TotalTokens
					}
					if streamResponse.Response.Usage.InputTokensDetails != nil {
						usage.PromptTokensDetails.CachedTokens = streamResponse.Response.Usage.InputTokensDetails.CachedTokens
						usage.PromptTokensDetails.CacheWriteTokens = streamResponse.Response.Usage.InputTokensDetails.CacheWriteTokens
					}
				}
			}
		case "response.output_text.delta":
			// 处理输出文本
			responseTextBuilder.WriteString(streamResponse.Delta)
		}
	})

	if usage.CompletionTokens == 0 {
		// 计算输出文本的 token 数量
		tempStr := responseTextBuilder.String()
		if len(tempStr) > 0 {
			// 非正常结束，使用输出文本的 token 数量
			completionTokens := service.CountTextToken(tempStr, info.UpstreamModelName)
			usage.CompletionTokens = completionTokens
		}
	}

	if usage.PromptTokens == 0 && usage.CompletionTokens != 0 {
		usage.PromptTokens = info.GetEstimatePromptTokens()
	}

	usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens

	return usage, nil
}
