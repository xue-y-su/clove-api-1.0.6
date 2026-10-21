package relay

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

type alphaSearchCapable interface {
	SupportsAlphaSearch() bool
}

type alphaSearchBillingFinalizer func(*gin.Context, *relaycommon.RelayInfo, *dto.Usage, service.DeliveryState) error

func AlphaSearchHelper(c *gin.Context, info *relaycommon.RelayInfo) *types.NewAPIError {
	return alphaSearchHelper(c, info, service.FinalizeRelayBilling)
}

func alphaSearchHelper(c *gin.Context, info *relaycommon.RelayInfo, finalize alphaSearchBillingFinalizer) *types.NewAPIError {
	if info == nil {
		return types.NewErrorWithStatusCode(
			errors.New("alpha search relay info is required"),
			types.ErrorCodeInvalidRequest,
			http.StatusBadRequest,
			types.ErrOptionWithSkipRetry(),
		)
	}
	info.InitChannelMeta(c)

	request, ok := info.Request.(*dto.AlphaSearchRequest)
	if !ok {
		return types.NewErrorWithStatusCode(
			fmt.Errorf("invalid request type, expected *dto.AlphaSearchRequest, got %T", info.Request),
			types.ErrorCodeInvalidRequest,
			http.StatusBadRequest,
			types.ErrOptionWithSkipRetry(),
		)
	}

	adaptor := GetAdaptor(info.ApiType)
	if adaptor == nil {
		return types.NewError(fmt.Errorf("invalid api type: %d", info.ApiType), types.ErrorCodeInvalidApiType, types.ErrOptionWithSkipRetry())
	}
	capability, supported := adaptor.(alphaSearchCapable)
	if !supported || !capability.SupportsAlphaSearch() {
		return types.NewErrorWithStatusCode(
			fmt.Errorf("channel type %d does not support /v1/alpha/search", info.ChannelType),
			types.ErrorCodeInvalidRequest,
			http.StatusServiceUnavailable,
			types.ErrOptionWithNoRecordErrorLog(),
		)
	}

	if err := helper.ModelMappedHelper(c, info, request); err != nil {
		return types.NewError(err, types.ErrorCodeChannelModelMappedError, types.ErrOptionWithSkipRetry())
	}
	jsonData, err := buildAlphaSearchRequestBody(request.RawBody, info.OriginModelName, info.UpstreamModelName)
	if err != nil {
		return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}
	if len(info.ParamOverride) > 0 {
		jsonData, err = relaycommon.ApplyParamOverrideWithRelayInfo(jsonData, info)
		if err != nil {
			return newAPIErrorFromParamOverride(err)
		}
	}

	body, size, closer, err := relaycommon.NewOutboundJSONBody(jsonData)
	if err != nil {
		return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}
	defer closer.Close()
	info.UpstreamRequestBodySize = size

	// The event remains provisional in the current retry attempt. ConfirmToolEvent
	// reserves its price before the upstream call; only a 2xx path commits it.
	if err := info.ConfirmToolEvent(alphaSearchToolEvent(info)); err != nil {
		return service.RelayBillingFinalizationError(err)
	}

	adaptor.Init(info)
	response, err := adaptor.DoRequest(c, info, body)
	if err != nil {
		return types.NewOpenAIError(err, types.ErrorCodeDoRequestFailed, http.StatusInternalServerError)
	}
	httpResponse, ok := response.(*http.Response)
	if !ok || httpResponse == nil {
		return types.NewOpenAIError(errors.New("invalid alpha search http response"), types.ErrorCodeDoRequestFailed, http.StatusInternalServerError)
	}
	defer httpResponse.Body.Close()

	if httpResponse.StatusCode < http.StatusOK || httpResponse.StatusCode >= http.StatusMultipleChoices {
		apiErr := service.RelayErrorHandler(c.Request.Context(), httpResponse, false)
		service.ResetStatusCode(apiErr, c.GetString("status_code_mapping"))
		return apiErr
	}

	responseBody, err := io.ReadAll(common.LimitUpstreamResponseBody(httpResponse.Body))
	if err != nil {
		return types.NewError(err, types.ErrorCodeReadResponseBodyFailed, types.ErrOptionWithSkipRetry())
	}
	if finalize == nil {
		return types.NewError(errors.New("alpha search billing finalizer is required"), types.ErrorCodeUpdateDataError, types.ErrOptionWithSkipRetry())
	}
	if err := finalize(c, info, &dto.Usage{}, service.DeliveryState{}); err != nil {
		return service.RelayBillingFinalizationError(err)
	}

	if contentType := httpResponse.Header.Get("Content-Type"); contentType != "" {
		c.Header("Content-Type", contentType)
	}
	c.Status(httpResponse.StatusCode)
	if _, err := c.Writer.Write(responseBody); err != nil {
		return types.NewError(err, types.ErrorCodeDoRequestFailed, types.ErrOptionWithSkipRetry(), types.ErrOptionWithNoRecordErrorLog())
	}
	return nil
}

func alphaSearchToolEvent(info *relaycommon.RelayInfo) relaycommon.ToolEvent {
	requestID := "request"
	if info != nil && info.RequestId != "" {
		requestID = info.RequestId
	}
	return relaycommon.ToolEvent{
		Name:     dto.BuildInToolWebSearchPreview,
		StableID: "alpha-search:" + requestID,
	}
}

// buildAlphaSearchRequestBody returns the original bytes unless model mapping
// is active. Mapping decodes to RawMessage values so no other field is typed,
// interpreted, or rewritten.
func buildAlphaSearchRequestBody(rawBody []byte, originModel, upstreamModel string) ([]byte, error) {
	if len(rawBody) == 0 {
		return nil, errors.New("empty alpha search request body")
	}
	if upstreamModel == "" || upstreamModel == originModel {
		return rawBody, nil
	}

	var body map[string]json.RawMessage
	if err := json.Unmarshal(rawBody, &body); err != nil {
		return nil, err
	}
	if body == nil {
		return nil, errors.New("alpha search request body must be a JSON object")
	}
	mappedModel, err := json.Marshal(upstreamModel)
	if err != nil {
		return nil, err
	}
	body["model"] = mappedModel
	return json.Marshal(body)
}
