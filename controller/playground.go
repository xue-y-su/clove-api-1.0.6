package controller

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

func Playground(c *gin.Context) {
	var newAPIError *types.NewAPIError

	defer func() {
		writePlaygroundErrorResponse(c, newAPIError)
	}()

	relayInfo, err := relaycommon.GenRelayInfo(c, types.RelayFormatOpenAI, nil, nil)
	if err != nil {
		newAPIError = types.NewError(err, types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
		return
	}
	if setupErr := setupPlaygroundRelayContext(c, relayInfo.UsingGroup); setupErr != nil {
		newAPIError = setupErr
		return
	}

	searchSettings := service.GetPlaygroundSearchSettingsFromOptions()
	searchSettings.NativeRouteCapable = playgroundNativeSearchRouteCapable(relayInfo, searchSettings.NativeChannelIDs)
	searchOutcome, setupErr := applyPlaygroundSearchFallbackToContext(c, searchSettings)
	if setupErr != nil {
		newAPIError = setupErr
		return
	}
	c.Writer = newPlaygroundMetadataWriter(c.Writer, searchOutcome)

	recordPlaygroundAudit(c, relayInfo.RelayMode, relayInfo.UsingGroup)

	Relay(c, types.RelayFormatOpenAI)
}

func playgroundNativeSearchRouteCapable(relayInfo *relaycommon.RelayInfo, allowedChannelIDs map[int]struct{}) bool {
	if relayInfo == nil || relayInfo.ChannelMeta == nil {
		return false
	}
	if _, ok := allowedChannelIDs[relayInfo.ChannelMeta.ChannelId]; !ok {
		return false
	}
	switch relayInfo.ChannelMeta.ChannelType {
	case constant.ChannelTypeOpenAI, constant.ChannelTypeAzure, constant.ChannelTypeAdvancedCustom:
		return true
	default:
		return false
	}
}

func PlaygroundImageGenerations(c *gin.Context) {
	var newAPIError *types.NewAPIError

	defer func() {
		writePlaygroundErrorResponse(c, newAPIError)
	}()

	relayInfo, err := relaycommon.GenRelayInfo(c, types.RelayFormatOpenAIImage, nil, nil)
	if err != nil {
		newAPIError = types.NewError(err, types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
		return
	}

	if setupErr := setupPlaygroundRelayContext(c, relayInfo.UsingGroup); setupErr != nil {
		newAPIError = setupErr
		return
	}

	recordPlaygroundAudit(c, relayInfo.RelayMode, relayInfo.UsingGroup)

	Relay(c, types.RelayFormatOpenAIImage)
}

func writePlaygroundErrorResponse(c *gin.Context, apiError *types.NewAPIError) {
	if apiError == nil {
		return
	}
	code := "playground_request_failed"
	key := i18n.MsgPlaygroundRequestFailed
	switch apiError.GetErrorCode() {
	case types.ErrorCodeInvalidRequest, types.ErrorCodeBadRequestBody:
		code = "playground_invalid_request"
		key = i18n.MsgPlaygroundInvalidRequest
	case types.ErrorCodeAccessDenied:
		code = "playground_access_denied"
		key = i18n.MsgPlaygroundAccessDenied
	case types.ErrorCodeDoRequestFailed:
		switch {
		case errors.Is(apiError, service.ErrPlaygroundSearchUnavailable):
			code = "playground_search_unavailable"
			key = i18n.MsgPlaygroundSearchUnavailable
		case errors.Is(apiError, service.ErrPlaygroundSearchNoResults):
			code = "playground_search_no_results"
			key = i18n.MsgPlaygroundSearchNoResults
		default:
			code = "playground_search_failed"
			key = i18n.MsgPlaygroundSearchFailed
		}
	default:
		common.SysError("playground setup failed: " + apiError.ErrorWithStatusCode())
	}
	message := common.TranslateMessage(c, key)
	publicError := apiError.ToOpenAIError()
	publicError.Message = message
	publicError.Type = "playground_error"
	publicError.Code = code
	publicError.Param = ""
	statusCode := apiError.StatusCode
	if statusCode < 400 {
		statusCode = http.StatusInternalServerError
	}
	c.JSON(statusCode, gin.H{
		"success":    false,
		"code":       code,
		"message":    message,
		"request_id": c.GetString(common.RequestIdKey),
		"error":      publicError,
	})
}

type playgroundAuditRequest struct {
	Model            string                   `json:"model"`
	Tools            []map[string]interface{} `json:"tools"`
	WebSearchOptions map[string]interface{}   `json:"web_search_options"`
	Size             string                   `json:"size"`
	N                interface{}              `json:"n"`
	ResponseFormat   string                   `json:"response_format"`
}

func applyPlaygroundSearchFallbackToContext(c *gin.Context, settings service.PlaygroundSearchSettings) (service.PlaygroundSearchOutcome, *types.NewAPIError) {
	var request dto.GeneralOpenAIRequest
	if err := common.UnmarshalBodyReusable(c, &request); err != nil {
		return service.PlaygroundSearchOutcome{}, types.NewError(err, types.ErrorCodeBadRequestBody, types.ErrOptionWithSkipRetry())
	}

	outcome := service.ApplyPlaygroundSearchFallback(c.Request.Context(), &request, settings)
	setPlaygroundSearchContext(c, outcome)
	if outcome.Status == service.PlaygroundSearchOutcomeUnavailable || outcome.Status == service.PlaygroundSearchOutcomeFailed {
		cause := outcome.Cause
		if cause == nil {
			cause = service.ErrPlaygroundSearchFailed
		}
		if outcome.Status == service.PlaygroundSearchOutcomeFailed {
			logger.LogError(c, fmt.Sprintf("playground web search failed request_id=%s: %v", c.GetString(common.RequestIdKey), cause))
		}
		return outcome, types.NewErrorWithStatusCode(cause, types.ErrorCodeDoRequestFailed, http.StatusServiceUnavailable, types.ErrOptionWithSkipRetry())
	}

	if outcome.Status != service.PlaygroundSearchOutcomeTavily {
		return outcome, nil
	}

	updatedBody, err := common.Marshal(&request)
	if err != nil {
		return outcome, types.NewError(err, types.ErrorCodeBadRequestBody, types.ErrOptionWithSkipRetry())
	}
	if err := replaceReusableRequestBody(c, updatedBody); err != nil {
		return outcome, types.NewError(err, types.ErrorCodeBadRequestBody, types.ErrOptionWithSkipRetry())
	}
	return outcome, nil
}

func setPlaygroundSearchContext(c *gin.Context, outcome service.PlaygroundSearchOutcome) {
	if outcome.Status == "" || outcome.Status == service.PlaygroundSearchOutcomeNone {
		return
	}
	c.Set("playground_web_search_path", outcome.Status)
	if outcome.ContextSize != "" {
		c.Set("playground_web_search_context_size", outcome.ContextSize)
	}
	if outcome.Status == service.PlaygroundSearchOutcomeTavily {
		c.Set("playground_tavily_search", true)
	}
	if len(outcome.Sources) > 0 {
		c.Set("playground_web_search_source_count", len(outcome.Sources))
	}
}

func replaceReusableRequestBody(c *gin.Context, body []byte) error {
	storage, err := common.CreateBodyStorage(body)
	if err != nil {
		return err
	}
	if previous, exists := c.Get(common.KeyBodyStorage); exists && previous != nil {
		if oldStorage, ok := previous.(common.BodyStorage); ok {
			_ = oldStorage.Close()
		}
	}
	c.Set(common.KeyRequestBody, body)
	c.Set(common.KeyBodyStorage, storage)
	c.Request.Body = io.NopCloser(bytes.NewReader(body))
	c.Request.ContentLength = int64(len(body))
	return nil
}

func buildPlaygroundAuditEvent(c *gin.Context, relayMode int, group string) (string, map[string]interface{}) {
	req := playgroundAuditRequest{}
	if err := common.UnmarshalBodyReusable(c, &req); err != nil {
		return "", nil
	}
	feature := "chat"
	action := ""
	params := map[string]interface{}{
		"feature": feature,
		"group":   group,
	}
	if req.Model != "" {
		params["model"] = req.Model
	}
	if c != nil && c.Request != nil && c.Request.URL != nil {
		params["route"] = c.Request.URL.Path
	}

	switch relayMode {
	case relayconstant.RelayModeImagesGenerations:
		feature = "image_generation"
		action = "playground.image_generation"
		params["feature"] = feature
		if req.Size != "" {
			params["size"] = req.Size
		}
		if req.N != nil {
			params["n"] = req.N
		}
		if req.ResponseFormat != "" {
			params["response_format"] = req.ResponseFormat
		}
	case relayconstant.RelayModeChatCompletions:
		searchPath := ""
		if c != nil {
			searchPath = strings.TrimSpace(c.GetString("playground_web_search_path"))
		}
		if !playgroundRequestHasWebSearch(req) && searchPath == "" {
			return "", nil
		}
		action = "playground.web_search"
		if searchPath != "" {
			params["web_search_path"] = searchPath
		}
		if c != nil && strings.TrimSpace(c.GetString("playground_web_search_context_size")) != "" {
			params["web_search_context_size"] = strings.TrimSpace(c.GetString("playground_web_search_context_size"))
		} else if size, ok := req.WebSearchOptions["search_context_size"].(string); ok && strings.TrimSpace(size) != "" {
			params["web_search_context_size"] = size
		}
		if c != nil {
			if sourceCount := c.GetInt("playground_web_search_source_count"); sourceCount > 0 {
				params["web_search_source_count"] = sourceCount
			}
		}
	default:
		return "", nil
	}
	return action, params
}

func playgroundRequestHasWebSearch(req playgroundAuditRequest) bool {
	if req.WebSearchOptions != nil {
		return true
	}
	for _, tool := range req.Tools {
		if typ, ok := tool["type"].(string); ok && typ == "web_search_preview" {
			return true
		}
	}
	return false
}

func recordPlaygroundAudit(c *gin.Context, relayMode int, group string) {
	action, params := buildPlaygroundAuditEvent(c, relayMode, group)
	if action == "" {
		return
	}
	recordUserSecurityAudit(c, c.GetInt("id"), action, params)
}

func setupPlaygroundRelayContext(c *gin.Context, group string) *types.NewAPIError {
	useAccessToken := c.GetBool("use_access_token")
	if useAccessToken {
		return types.NewError(errors.New("暂不支持使用 access token"), types.ErrorCodeAccessDenied, types.ErrOptionWithSkipRetry())
	}

	userId := c.GetInt("id")

	// Write user context to ensure acceptUnsetRatio is available.
	userCache, err := model.GetUserCache(userId)
	if err != nil {
		return types.NewError(err, types.ErrorCodeQueryDataError, types.ErrOptionWithSkipRetry())
	}
	userCache.WriteContext(c)

	tempToken := &model.Token{
		UserId: userId,
		Name:   fmt.Sprintf("playground-%s", group),
		Group:  group,
	}
	_ = middleware.SetupContextForToken(c, tempToken)
	common.SetContextKey(c, constant.ContextKeyUsingGroup, group)
	return nil
}
