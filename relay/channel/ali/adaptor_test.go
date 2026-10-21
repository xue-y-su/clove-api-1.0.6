package ali

import (
	"encoding/json"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/require"
)

func TestConvertOpenAIRequestFiltersThinkingBudgetByUpstreamModel(t *testing.T) {
	tests := []struct {
		name          string
		upstreamModel string
		wantBudget    bool
	}{
		{name: "qwen", upstreamModel: "qwen-plus", wantBudget: true},
		{name: "qwq", upstreamModel: "provider/QwQ-32B", wantBudget: true},
		{name: "other", upstreamModel: "deepseek-chat", wantBudget: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := &dto.GeneralOpenAIRequest{
				Model:          tt.upstreamModel,
				ThinkingBudget: json.RawMessage(`128`),
			}
			info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: tt.upstreamModel}}

			convertedValue, err := (&Adaptor{}).ConvertOpenAIRequest(nil, info, request)
			require.NoError(t, err)
			converted := convertedValue.(*dto.GeneralOpenAIRequest)
			if tt.wantBudget {
				require.Equal(t, json.RawMessage(`128`), converted.ThinkingBudget)
			} else {
				require.Nil(t, converted.ThinkingBudget)
			}
		})
	}
}
