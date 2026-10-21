package dify

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const difyTinyPNG = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII="

func difyImageRequest(imageURL string) dto.GeneralOpenAIRequest {
	message := dto.Message{Role: "user"}
	message.SetMediaContent([]dto.MediaContent{{
		Type: dto.ContentTypeImageURL,
		ImageUrl: &dto.MessageImageUrl{
			Url:      imageURL,
			MimeType: "image/png",
		},
	}})
	return dto.GeneralOpenAIRequest{Messages: []dto.Message{message}}
}

func TestRequestOpenAI2DifyPropagatesUploadFailure(t *testing.T) {
	service.InitHttpClient()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusBadGateway)
		_, _ = writer.Write([]byte(`{"message":"upload failed"}`))
	}))
	t.Cleanup(server.Close)
	gin.SetMode(gin.TestMode)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	request, err := requestOpenAI2Dify(context, &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{ChannelBaseUrl: server.URL, ApiKey: "secret"},
	}, difyImageRequest("data:image/png;base64,"+difyTinyPNG))

	require.Error(t, err)
	assert.Nil(t, request)
}

func TestRequestOpenAI2DifyMapsRemoteImageType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	request, err := requestOpenAI2Dify(context, &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{ChannelBaseUrl: "https://api.dify.ai"},
	}, difyImageRequest("https://example.com/image.png"))

	require.NoError(t, err)
	require.Len(t, request.Files, 1)
	assert.Equal(t, "image", request.Files[0].Type)
	assert.Equal(t, "remote_url", request.Files[0].TransferMode)
}
