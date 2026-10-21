package helper

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/gin-gonic/gin"
)

func validatePlaygroundReasoning(c *gin.Context, request *dto.GeneralOpenAIRequest) error {
	if c == nil || c.Request == nil || request == nil || !strings.HasPrefix(c.Request.URL.Path, "/pg/chat/completions") {
		return nil
	}
	effort := strings.ToLower(strings.TrimSpace(request.ReasoningEffort))
	if effort == "" {
		return nil
	}
	if effort == "off" {
		effort = "none"
	}
	if effort != "none" && effort != "low" && effort != "medium" && effort != "high" {
		return errors.New("playground reasoning_effort must be one of: off, low, medium, high")
	}
	request.ReasoningEffort = effort
	modelName := strings.ToLower(strings.TrimSpace(request.Model))
	channelType := common.GetContextKeyInt(c, constant.ContextKeyChannelType)
	if !playgroundModelSupportsReasoning(modelName) || !playgroundChannelSupportsReasoning(channelType, modelName) {
		request.ReasoningEffort = ""
		common.SetContextKey(c, constant.ContextKeyPlaygroundReasoningControlIgnored, true)
	}
	return nil
}

func playgroundModelSupportsReasoning(modelName string) bool {
	return dto.IsOpenAIReasoningOModel(modelName) ||
		dto.IsOpenAIGPT5Model(modelName) ||
		strings.HasPrefix(modelName, "grok-3-mini")
}

func playgroundChannelSupportsReasoning(channelType int, modelName string) bool {
	if strings.HasPrefix(modelName, "grok-3-mini") {
		return channelType == constant.ChannelTypeXai ||
			channelType == constant.ChannelTypeOpenRouter ||
			channelType == constant.ChannelTypeAdvancedCustom
	}
	switch channelType {
	case constant.ChannelTypeOpenAI,
		constant.ChannelTypeAzure,
		constant.ChannelTypeOpenRouter,
		constant.ChannelTypeAdvancedCustom:
		return true
	default:
		return false
	}
}
