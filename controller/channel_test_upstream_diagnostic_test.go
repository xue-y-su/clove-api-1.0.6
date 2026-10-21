package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
)

func TestTestChannelRecordsNon2xxUpstreamDiagnostic(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	service.InitHttpClient()
	previousModelRatios := ratio_setting.ModelRatio2JSONString()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(previousModelRatios))
		model.InvalidatePricingCache()
	})
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{"diagnostic-model":1}`))
	model.InvalidatePricingCache()
	require.NoError(t, db.Create(&model.User{
		Id: 828, Username: "diagnostic-channel-test", Password: "hashed-password",
		DisplayName: "diagnostic-channel-test", AffCode: "diagnostic-channel-test-aff",
		Quota: 1_000_000, Group: "default", Role: common.RoleCommonUser,
		Status: common.UserStatusEnabled,
	}).Error)

	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusBadGateway)
		_, _ = writer.Write([]byte(`<html>upstream unavailable</html>`))
	}))
	t.Cleanup(upstream.Close)
	baseURL := upstream.URL
	channel := &model.Channel{
		Id: 829, Type: constant.ChannelTypeOpenAI, Key: "diagnostic-key",
		Status: common.ChannelStatusEnabled, Name: "diagnostic-channel", BaseURL: &baseURL,
		Models: "diagnostic-model", Group: "default",
	}

	previousRecorder := recordUpstreamDiagnostic
	t.Cleanup(func() { recordUpstreamDiagnostic = previousRecorder })
	called := 0
	recordUpstreamDiagnostic = func(
		_ context.Context,
		apiErr *types.NewAPIError,
		channelID int,
		_ string,
		other map[string]interface{},
		_ operation_setting.UpstreamDiagnosticSnapshot,
	) error {
		called++
		require.Equal(t, channel.Id, channelID)
		require.Nil(t, other)
		_, ok := service.UpstreamDiagnosticFromError(apiErr)
		require.True(t, ok)
		return nil
	}

	result := testChannel(
		context.Background(), channel, 828, "diagnostic-model", "", false,
	)

	require.Error(t, result.localErr)
	require.Equal(t, 1, called)
}
