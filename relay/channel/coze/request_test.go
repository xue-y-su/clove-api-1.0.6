package coze

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertCozeChatRequestBuildsValidDefaultUserID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	context.Set(common.RequestIdKey, "request-123")

	request, err := convertCozeChatRequest(context, dto.GeneralOpenAIRequest{
		Messages: []dto.Message{{Role: "user", Content: "hello"}},
	})

	require.NoError(t, err)
	assert.Equal(t, "chatcmpl-request-123", request.UserId)
	_, err = common.Marshal(request)
	require.NoError(t, err)
}

func TestConvertCozeChatRequestConvertsMultimodalContent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	context.Set(common.RequestIdKey, "request-123")

	request, err := convertCozeChatRequest(context, dto.GeneralOpenAIRequest{
		Messages: []dto.Message{{
			Role: "user",
			Content: []any{
				map[string]any{"type": "text", "text": "describe"},
				map[string]any{"type": "image_url", "image_url": map[string]any{"url": "https://example.com/image.png"}},
				map[string]any{"type": "file", "file": map[string]any{"file_id": "file-123"}},
				map[string]any{"type": "video_url", "video_url": "https://example.com/video.mp4"},
			},
		}},
	})

	require.NoError(t, err)
	require.Len(t, request.AdditionalMessages, 1)
	message := request.AdditionalMessages[0]
	assert.Equal(t, "object_string", message.ContentType)
	content, ok := message.Content.(string)
	require.True(t, ok)
	var parts []CozeContentPart
	require.NoError(t, common.UnmarshalJsonStr(content, &parts))
	require.Len(t, parts, 4)
	assert.Equal(t, "text", parts[0].Type)
	assert.Equal(t, "image", parts[1].Type)
	assert.Equal(t, "https://example.com/image.png", parts[1].FileURL)
	assert.Equal(t, "file-123", parts[2].FileID)
	assert.Equal(t, "video", parts[3].Type)
}

func TestConvertCozeChatRequestRejectsInlineMedia(t *testing.T) {
	gin.SetMode(gin.TestMode)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	request, err := convertCozeChatRequest(context, dto.GeneralOpenAIRequest{
		Messages: []dto.Message{{
			Role: "user",
			Content: []any{
				map[string]any{"type": "image_url", "image_url": map[string]any{"url": "data:image/png;base64,abc"}},
			},
		}},
	})

	require.Error(t, err)
	assert.Nil(t, request)
}
