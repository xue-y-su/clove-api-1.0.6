package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"
)

func MidjourneyErrorWrapper(code int, desc string) *dto.MidjourneyResponse {
	return &dto.MidjourneyResponse{
		Code:        code,
		Description: desc,
	}
}

func MidjourneyErrorWithStatusCodeWrapper(code int, desc string, statusCode int) *dto.MidjourneyResponseWithStatusCode {
	return &dto.MidjourneyResponseWithStatusCode{
		StatusCode: statusCode,
		Response:   *MidjourneyErrorWrapper(code, desc),
	}
}

//// OpenAIErrorWrapper wraps an error into an OpenAIErrorWithStatusCode
//func OpenAIErrorWrapper(err error, code string, statusCode int) *dto.OpenAIErrorWithStatusCode {
//	text := err.Error()
//	lowerText := strings.ToLower(text)
//	if !strings.HasPrefix(lowerText, "get file base64 from url") && !strings.HasPrefix(lowerText, "mime type is not supported") {
//		if strings.Contains(lowerText, "post") || strings.Contains(lowerText, "dial") || strings.Contains(lowerText, "http") {
//			common.SysLog(fmt.Sprintf("error: %s", text))
//			text = "请求上游地址失败"
//		}
//	}
//	openAIError := dto.OpenAIError{
//		Message: text,
//		Type:    "new_api_error",
//		Code:    code,
//	}
//	return &dto.OpenAIErrorWithStatusCode{
//		Error:      openAIError,
//		StatusCode: statusCode,
//	}
//}
//
//func OpenAIErrorWrapperLocal(err error, code string, statusCode int) *dto.OpenAIErrorWithStatusCode {
//	openaiErr := OpenAIErrorWrapper(err, code, statusCode)
//	openaiErr.LocalError = true
//	return openaiErr
//}

func ClaudeErrorWrapper(err error, code string, statusCode int) *dto.ClaudeErrorWithStatusCode {
	text := err.Error()
	lowerText := strings.ToLower(text)
	if !strings.HasPrefix(lowerText, "get file base64 from url") {
		if strings.Contains(lowerText, "post") || strings.Contains(lowerText, "dial") || strings.Contains(lowerText, "http") {
			common.SysLog(fmt.Sprintf("error: %s", text))
			text = "请求上游地址失败"
		}
	}
	claudeError := types.ClaudeError{
		Message: text,
		Type:    "new_api_error",
	}
	return &dto.ClaudeErrorWithStatusCode{
		Error:      claudeError,
		StatusCode: statusCode,
	}
}

func ClaudeErrorWrapperLocal(err error, code string, statusCode int) *dto.ClaudeErrorWithStatusCode {
	claudeErr := ClaudeErrorWrapper(err, code, statusCode)
	claudeErr.LocalError = true
	return claudeErr
}

type upstreamDiagnosticCarrier struct {
	cause      error
	diagnostic UpstreamDiagnostic
}

func (carrier *upstreamDiagnosticCarrier) Error() string {
	return carrier.cause.Error()
}

func (carrier *upstreamDiagnosticCarrier) Unwrap() error {
	return carrier.cause
}

func UpstreamDiagnosticFromError(apiErr *types.NewAPIError) (UpstreamDiagnostic, bool) {
	if apiErr == nil || apiErr.Err == nil {
		return UpstreamDiagnostic{}, false
	}
	var carrier *upstreamDiagnosticCarrier
	if !errors.As(apiErr.Err, &carrier) {
		return UpstreamDiagnostic{}, false
	}
	return carrier.diagnostic, true
}

func ParseUpstreamError(resp *http.Response, snapshot operation_setting.UpstreamDiagnosticSnapshot) (*types.NewAPIError, *UpstreamDiagnostic) {
	statusCode := 0
	if resp != nil {
		statusCode = resp.StatusCode
	}
	newApiErr := types.InitOpenAIError(types.ErrorCodeBadResponseStatusCode, statusCode)
	diagnostic := &UpstreamDiagnostic{Status: statusCode, ErrorCategory: "empty_body"}
	if resp == nil || resp.Body == nil {
		newApiErr.Err = fmt.Errorf("bad response status code %d", statusCode)
		diagnostic.Digest = diagnosticDigest(nil)
		return newApiErr, diagnostic
	}
	defer CloseResponseBodyGracefully(resp)

	responseBody, readErr := io.ReadAll(io.LimitReader(resp.Body, upstreamDiagnosticBodyLimitBytes+1))
	diagnostic.RawLength = int64(len(responseBody))
	diagnostic.Truncated = len(responseBody) > upstreamDiagnosticBodyLimitBytes
	diagnostic.Digest = diagnosticDigest(responseBody)
	if diagnostic.Truncated {
		responseBody = responseBody[:upstreamDiagnosticBodyLimitBytes]
	}
	if readErr != nil {
		diagnostic.ErrorCategory = "read_error"
		newApiErr.Err = fmt.Errorf("bad response status code %d", statusCode)
	} else {
		var errResponse dto.GeneralErrorResponse
		if err := common.Unmarshal(responseBody, &errResponse); err != nil {
			diagnostic.ErrorCategory = "invalid_json"
			newApiErr.Err = fmt.Errorf("bad response status code %d", statusCode)
		} else if common.GetJsonType(errResponse.Error) == "object" {
			if openAIError := errResponse.TryToOpenAIError(); openAIError != nil {
				newApiErr = types.WithOpenAIError(*openAIError, statusCode)
				diagnostic.ErrorCategory = "structured_error"
			} else {
				message := strings.TrimSpace(errResponse.ToMessage())
				if message == "" {
					diagnostic.ErrorCategory = "empty_message"
					newApiErr.Err = fmt.Errorf("bad response status code %d", statusCode)
				} else {
					diagnostic.ErrorCategory = "structured_error"
					newApiErr = types.NewOpenAIError(errors.New(message), types.ErrorCodeBadResponseStatusCode, statusCode)
				}
			}
		} else {
			message := strings.TrimSpace(errResponse.ToMessage())
			if message == "" {
				diagnostic.ErrorCategory = "empty_message"
				newApiErr.Err = fmt.Errorf("bad response status code %d", statusCode)
			} else {
				diagnostic.ErrorCategory = "structured_error"
				newApiErr = types.NewOpenAIError(errors.New(message), types.ErrorCodeBadResponseStatusCode, statusCode)
			}
		}
	}

	fallback := diagnostic.ErrorCategory != "structured_error"
	diagnostic.Captured = snapshot.Enabled &&
		(snapshot.CaptureMode == operation_setting.UpstreamDiagnosticCaptureAllErrors || fallback)
	if diagnostic.Captured {
		diagnostic.Preview = truncateDiagnosticPreview(
			redactUpstreamDiagnostic(responseBody),
			snapshot.PreviewChars,
			diagnostic.Truncated,
		)
	}
	return newApiErr, diagnostic
}

func RelayErrorHandler(_ context.Context, resp *http.Response, _ bool) *types.NewAPIError {
	newApiErr, diagnostic := ParseUpstreamError(resp, operation_setting.CurrentUpstreamDiagnosticSnapshot())
	if newApiErr == nil || diagnostic == nil {
		return newApiErr
	}
	cause := newApiErr.Err
	if cause == nil {
		cause = errors.New(newApiErr.Error())
	}
	newApiErr.Err = &upstreamDiagnosticCarrier{cause: cause, diagnostic: *diagnostic}
	return newApiErr
}

func ResetStatusCode(newApiErr *types.NewAPIError, statusCodeMappingStr string) {
	if newApiErr == nil {
		return
	}
	if statusCodeMappingStr == "" || statusCodeMappingStr == "{}" {
		return
	}
	statusCodeMapping := make(map[string]any)
	err := common.Unmarshal([]byte(statusCodeMappingStr), &statusCodeMapping)
	if err != nil {
		return
	}
	if newApiErr.StatusCode == http.StatusOK {
		return
	}
	codeStr := strconv.Itoa(newApiErr.StatusCode)
	if value, ok := statusCodeMapping[codeStr]; ok {
		intCode, ok := parseStatusCodeMappingValue(value)
		if !ok {
			return
		}
		newApiErr.StatusCode = intCode
	}
}

func parseStatusCodeMappingValue(value any) (int, bool) {
	switch v := value.(type) {
	case string:
		if v == "" {
			return 0, false
		}
		statusCode, err := strconv.Atoi(v)
		if err != nil {
			return 0, false
		}
		return statusCode, true
	case float64:
		if v != math.Trunc(v) {
			return 0, false
		}
		return int(v), true
	case int:
		return v, true
	case json.Number:
		statusCode, err := strconv.Atoi(v.String())
		if err != nil {
			return 0, false
		}
		return statusCode, true
	default:
		return 0, false
	}
}

func TaskErrorWrapperLocal(err error, code string, statusCode int) *dto.TaskError {
	openaiErr := TaskErrorWrapper(err, code, statusCode)
	openaiErr.LocalError = true
	return openaiErr
}

func TaskErrorWrapper(err error, code string, statusCode int) *dto.TaskError {
	text := err.Error()
	lowerText := strings.ToLower(text)
	if strings.Contains(lowerText, "post") || strings.Contains(lowerText, "dial") || strings.Contains(lowerText, "http") {
		common.SysLog(fmt.Sprintf("error: %s", text))
		//text = "请求上游地址失败"
		text = common.MaskSensitiveInfo(text)
	}
	//避免暴露内部错误
	taskError := &dto.TaskError{
		Code:       code,
		Message:    text,
		StatusCode: statusCode,
		Error:      err,
	}

	return taskError
}

// TaskErrorFromAPIError 将 PreConsumeBilling 返回的 NewAPIError 转换为 TaskError。
func TaskErrorFromAPIError(apiErr *types.NewAPIError) *dto.TaskError {
	if apiErr == nil {
		return nil
	}
	return &dto.TaskError{
		Code:       string(apiErr.GetErrorCode()),
		Message:    apiErr.Err.Error(),
		StatusCode: apiErr.StatusCode,
		Error:      apiErr.Err,
	}
}
