package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	rootcommon "github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRelayHandlerDispatchesAlphaSearch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/alpha/search", nil)
	rootcommon.SetContextKey(ctx, constant.ContextKeyChannelType, constant.ChannelTypeOpenAI)
	rootcommon.SetContextKey(ctx, constant.ContextKeyChannelSetting, dto.ChannelSettings{})

	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeAlphaSearch,
		Request: &dto.AlphaSearchRequest{
			Model:   "gpt-public",
			RawBody: json.RawMessage(`{"model":"gpt-public"}`),
		},
	}

	apiErr := relayHandler(ctx, info)

	require.NotNil(t, apiErr)
	require.Equal(t, http.StatusServiceUnavailable, apiErr.StatusCode)
	require.Contains(t, apiErr.Error(), "does not support /v1/alpha/search")
}
