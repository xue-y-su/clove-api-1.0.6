package openai

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetupRequestHeaderKeepsRealtimeBetaOnlyForPreviewModels(t *testing.T) {
	tests := []struct {
		name      string
		model     string
		websocket bool
		wantBeta  bool
	}{
		{name: "GA HTTP", model: "gpt-realtime", wantBeta: false},
		{name: "GA websocket", model: "gpt-realtime-2.1", websocket: true, wantBeta: false},
		{name: "preview HTTP", model: "gpt-4o-realtime-preview", wantBeta: true},
		{name: "preview websocket", model: "gpt-4o-mini-realtime-preview-2024-12-17", websocket: true, wantBeta: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			req := httptest.NewRequest(http.MethodGet, "/v1/realtime", nil)
			if tt.websocket {
				req.Header.Set("Sec-WebSocket-Protocol", "realtime")
			}
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			ctx.Request = req
			header := http.Header{}
			info := &relaycommon.RelayInfo{
				RelayMode: relayconstant.RelayModeRealtime,
				ChannelMeta: &relaycommon.ChannelMeta{
					ChannelType:       constant.ChannelTypeOpenAI,
					ApiKey:            "sk-test",
					UpstreamModelName: tt.model,
				},
			}

			require.NoError(t, (&Adaptor{}).SetupRequestHeader(ctx, &header, info))
			if tt.websocket {
				got := strings.Contains(header.Get("Sec-WebSocket-Protocol"), "openai-beta.realtime-v1")
				assert.Equal(t, tt.wantBeta, got)
				return
			}
			assert.Equal(t, tt.wantBeta, header.Get("openai-beta") == "realtime=v1")
		})
	}
}

func TestOpenAIModelListContainsRealtimeGAModels(t *testing.T) {
	for _, model := range []string{
		"gpt-realtime-2",
		"gpt-realtime-2.1",
		"gpt-realtime-2.1-mini",
		"gpt-realtime-whisper",
		"gpt-realtime-translate",
	} {
		assert.Contains(t, ModelList, model)
	}
}
