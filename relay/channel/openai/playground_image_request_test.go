package openai

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestPlaygroundImageRequestPreservesVisibleParametersUpstream(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service.InitHttpClient()

	type upstreamImageRequest struct {
		Model          string `json:"model"`
		Prompt         string `json:"prompt"`
		N              uint   `json:"n"`
		Size           string `json:"size"`
		Quality        string `json:"quality"`
		Background     string `json:"background"`
		Style          string `json:"style"`
		ResponseFormat string `json:"response_format"`
	}

	captured := make(chan upstreamImageRequest, 1)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		require.Equal(t, http.MethodPost, request.Method)
		require.Equal(t, "/v1/images/generations", request.URL.Path)
		var body upstreamImageRequest
		require.NoError(t, common.DecodeJson(request.Body, &body))
		captured <- body
		writer.Header().Set("Content-Type", "application/json")
		_, err := writer.Write([]byte(`{"data":[]}`))
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)

	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest(http.MethodPost, "/api/playground/image", nil)
	context.Request.Header.Set("Content-Type", "application/json")
	info := &relaycommon.RelayInfo{
		RelayMode:      relayconstant.RelayModeImagesGenerations,
		RequestURLPath: "/v1/images/generations",
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:    constant.ChannelTypeOpenAI,
			ChannelBaseUrl: server.URL,
			ApiKey:         "test-key",
		},
	}
	n := uint(2)
	request := dto.ImageRequest{
		Model:          "gpt-image-2",
		Prompt:         "a precise production diagram",
		N:              &n,
		Size:           "1536x1024",
		Quality:        "hd",
		Background:     json.RawMessage(`"transparent"`),
		Style:          json.RawMessage(`"vivid"`),
		ResponseFormat: "url",
	}

	converted, err := (&Adaptor{}).ConvertImageRequest(context, info, request)
	require.NoError(t, err)
	body, err := common.Marshal(converted)
	require.NoError(t, err)
	response, err := (&Adaptor{}).DoRequest(context, info, bytes.NewReader(body))
	require.NoError(t, err)
	require.NotNil(t, response)
	require.NoError(t, response.(*http.Response).Body.Close())

	require.Equal(t, upstreamImageRequest{
		Model:          "gpt-image-2",
		Prompt:         "a precise production diagram",
		N:              2,
		Size:           "1536x1024",
		Quality:        "hd",
		Background:     "transparent",
		Style:          "vivid",
		ResponseFormat: "url",
	}, <-captured)
}
