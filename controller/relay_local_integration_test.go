package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/compliance_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/setting/source_attribution_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func disableTokenAdmissionRequirements(t *testing.T) {
	t.Helper()

	common.OptionMapRWMutex.Lock()
	previousOptions := common.OptionMap
	options := make(map[string]string, len(previousOptions)+4)
	for key, value := range previousOptions {
		options[key] = value
	}
	options[compliance_setting.StatementEnabledKey] = "false"
	options[compliance_setting.TokenAPIEnforcementEnabledKey] = "false"
	options[source_attribution_setting.EnabledKey] = "false"
	options[source_attribution_setting.TokenAPIEnforcementEnabledKey] = "false"
	common.OptionMap = options
	common.OptionMapRWMutex.Unlock()

	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = previousOptions
		common.OptionMapRWMutex.Unlock()
	})
}

func TestRelayRouteAuthenticatesSelectsLocalChannelAndSettlesUsage(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(
		&model.Token{},
		&model.Log{},
		&model.WalletLedgerEntry{},
		&model.AuditOutboxEvent{},
		&model.UserSubscription{},
		&model.Checkin{},
		&model.IPSecurityEvent{},
	))
	disableTokenAdmissionRequirements(t)
	service.InitHttpClient()

	previousMemoryCacheEnabled := common.MemoryCacheEnabled
	previousCountToken := constant.CountToken
	previousLogConsumeEnabled := common.LogConsumeEnabled
	common.MemoryCacheEnabled = false
	constant.CountToken = false
	common.LogConsumeEnabled = true
	t.Cleanup(func() {
		common.MemoryCacheEnabled = previousMemoryCacheEnabled
		constant.CountToken = previousCountToken
		common.LogConsumeEnabled = previousLogConsumeEnabled
	})

	previousModelRatios := ratio_setting.ModelRatio2JSONString()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(previousModelRatios))
		model.InvalidatePricingCache()
	})
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{"local-route-model":1}`))
	model.InvalidatePricingCache()

	type upstreamRequest struct {
		Authorization string
		Model         string
	}
	requests := make(chan upstreamRequest, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/chat/completions" {
			http.NotFound(writer, request)
			return
		}
		var payload struct {
			Model string `json:"model"`
		}
		if err := common.DecodeJson(request.Body, &payload); err != nil {
			http.Error(writer, "invalid request", http.StatusBadRequest)
			return
		}
		requests <- upstreamRequest{
			Authorization: request.Header.Get("Authorization"),
			Model:         payload.Model,
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"id":"chatcmpl-route","object":"chat.completion","created":1710000000,"model":"local-route-model","choices":[{"index":0,"message":{"role":"assistant","content":"route response"},"finish_reason":"stop"}],"usage":{"prompt_tokens":2,"completion_tokens":1,"total_tokens":3}}`))
	}))
	t.Cleanup(upstream.Close)

	require.NoError(t, db.Create(&model.User{
		Id:          820,
		Username:    "relay-route-user",
		Password:    "hashed-password",
		DisplayName: "relay-route-user",
		AffCode:     "relay-route-aff",
		Quota:       10_000_000,
		Group:       "default",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
	}).Error)
	require.NoError(t, db.Create(&model.Token{
		Id:             821,
		UserId:         820,
		Key:            "relay-route-key",
		Name:           "relay-route-key",
		Status:         common.TokenStatusEnabled,
		ExpiredTime:    -1,
		UnlimitedQuota: true,
		Group:          "default",
	}).Error)
	baseURL := upstream.URL
	require.NoError(t, db.Create(&model.Channel{
		Id:          822,
		Type:        constant.ChannelTypeOpenAI,
		Key:         "local-route-upstream-key",
		Status:      common.ChannelStatusEnabled,
		Name:        "local-route-channel",
		BaseURL:     &baseURL,
		Models:      "local-route-model",
		Group:       "default",
		CreatedTime: time.Now().Unix(),
	}).Error)
	require.NoError(t, db.Create(&model.Ability{
		Group:     "default",
		Model:     "local-route-model",
		ChannelId: 822,
		Enabled:   true,
	}).Error)

	router := gin.New()
	router.POST("/v1/chat/completions", middleware.TokenAuth(), middleware.Distribute(), func(c *gin.Context) {
		Relay(c, types.RelayFormatOpenAI)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"local-route-model","messages":[{"role":"user","content":"hello"}]}`))
	request.Header.Set("Authorization", "Bearer sk-relay-route-key")
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"content":"route response"`)
	select {
	case received := <-requests:
		require.Equal(t, "Bearer local-route-upstream-key", received.Authorization)
		require.Equal(t, "local-route-model", received.Model)
	case <-time.After(2 * time.Second):
		t.Fatal("local upstream did not receive the relayed request")
	}

	var logCount int64
	require.NoError(t, db.Model(&model.Log{}).
		Where("user_id = ? AND token_id = ? AND channel_id = ? AND type = ?", 820, 821, 822, model.LogTypeConsume).
		Count(&logCount).Error)
	require.EqualValues(t, 1, logCount)
}
