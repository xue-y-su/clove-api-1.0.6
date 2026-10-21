package gemini

import (
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const tinyPNGBase64 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII="

func TestParseImageInputRejectsInvalidDataURI(t *testing.T) {
	image, err := ParseImageInput(context.Background(), "data:image/png;base64,not-base64", nil)

	require.Error(t, err)
	assert.Nil(t, image)
}

func TestParseImageInputDownloadsRemoteImage(t *testing.T) {
	imageBytes, err := base64.StdEncoding.DecodeString(tinyPNGBase64)
	require.NoError(t, err)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "image/png")
		_, _ = writer.Write(imageBytes)
	}))
	t.Cleanup(server.Close)

	image, err := ParseImageInput(context.Background(), server.URL, server.Client())

	require.NoError(t, err)
	require.NotNil(t, image)
	assert.Equal(t, "image/png", image.MimeType)
	assert.Equal(t, tinyPNGBase64, image.BytesBase64Encoded)
}

func TestParseImageInputRejectsOversizedRemoteImageFromContentLength(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "image/png")
		writer.Header().Set("Content-Length", "20971521")
		writer.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	image, err := ParseImageInput(context.Background(), server.URL, server.Client())

	require.Error(t, err)
	assert.Nil(t, image)
}

func TestBuildRequestBodyIncludesLastFrameAndReferenceImages(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ginContext, _ := gin.CreateTestContext(recorder)
	ginContext.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", strings.NewReader("{}"))
	ginContext.Request.Header.Set("Content-Type", "application/json")
	dataURI := "data:image/png;base64," + tinyPNGBase64
	ginContext.Set("task_request", relaycommon.TaskSubmitReq{
		Prompt: "animate",
		Images: []string{dataURI},
		Metadata: map[string]any{
			"lastFrame": dataURI,
			"referenceImages": []any{
				map[string]any{"image": dataURI, "referenceType": "asset"},
				map[string]any{"image": dataURI, "referenceType": "style"},
			},
		},
	})
	info := &relaycommon.RelayInfo{TaskRelayInfo: &relaycommon.TaskRelayInfo{}}

	reader, err := (&TaskAdaptor{}).BuildRequestBody(ginContext, info)

	require.NoError(t, err)
	body, err := io.ReadAll(reader)
	require.NoError(t, err)
	var payload VeoRequestPayload
	require.NoError(t, common.Unmarshal(body, &payload))
	require.Len(t, payload.Instances, 1)
	instance := payload.Instances[0]
	require.NotNil(t, instance.Image)
	require.NotNil(t, instance.LastFrame)
	require.Len(t, instance.ReferenceImages, 2)
	assert.Equal(t, "asset", instance.ReferenceImages[0].ReferenceType)
	assert.Equal(t, "style", instance.ReferenceImages[1].ReferenceType)
}

func TestBuildRequestBodyRejectsMoreThanThreeReferenceImages(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ginContext, _ := gin.CreateTestContext(recorder)
	ginContext.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", strings.NewReader("{}"))
	ginContext.Request.Header.Set("Content-Type", "application/json")
	dataURI := "data:image/png;base64," + tinyPNGBase64
	references := make([]any, 4)
	for index := range references {
		references[index] = map[string]any{"image": dataURI, "referenceType": "asset"}
	}
	ginContext.Set("task_request", relaycommon.TaskSubmitReq{
		Prompt:   "animate",
		Metadata: map[string]any{"referenceImages": references},
	})

	reader, err := (&TaskAdaptor{}).BuildRequestBody(ginContext, &relaycommon.RelayInfo{TaskRelayInfo: &relaycommon.TaskRelayInfo{}})

	require.Error(t, err)
	assert.Nil(t, reader)
}

func TestBuildRequestBodyRejectsEmptyReferenceImage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ginContext, _ := gin.CreateTestContext(recorder)
	ginContext.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", strings.NewReader("{}"))
	ginContext.Request.Header.Set("Content-Type", "application/json")
	ginContext.Set("task_request", relaycommon.TaskSubmitReq{
		Prompt:   "animate",
		Metadata: map[string]any{"referenceImages": []any{""}},
	})

	require.NotPanics(t, func() {
		reader, err := (&TaskAdaptor{}).BuildRequestBody(ginContext, &relaycommon.RelayInfo{TaskRelayInfo: &relaycommon.TaskRelayInfo{}})
		require.Error(t, err)
		assert.Nil(t, reader)
	})
}
