package common

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestApiErrorCodeReturnsStableWorkflowResponse(t *testing.T) {
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/api/workflow", nil)
	context.Set(RequestIdKey, "request-visible")

	ApiErrorCode(context, " workflow_invalid ", "  Please check the submitted values.  ")

	var response struct {
		Success   bool   `json:"success"`
		Code      string `json:"code"`
		Message   string `json:"message"`
		RequestID string `json:"request_id"`
	}
	require.NoError(t, Unmarshal(recorder.Body.Bytes(), &response))
	require.False(t, response.Success)
	require.Equal(t, "workflow_invalid", response.Code)
	require.Equal(t, "Please check the submitted values.", response.Message)
	require.Equal(t, "request-visible", response.RequestID)
}
