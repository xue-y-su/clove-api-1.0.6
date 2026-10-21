package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSettleTestQuotaUsesTieredBilling(t *testing.T) {
	info := &relaycommon.RelayInfo{
		TieredBillingSnapshot: &billingexpr.BillingSnapshot{
			BillingMode:   "tiered_expr",
			ExprString:    `param("stream") == true ? tier("stream", p * 3) : tier("base", p * 2)`,
			ExprHash:      billingexpr.ExprHashString(`param("stream") == true ? tier("stream", p * 3) : tier("base", p * 2)`),
			GroupRatio:    1,
			EstimatedTier: "stream",
			QuotaPerUnit:  common.QuotaPerUnit,
			ExprVersion:   1,
		},
		BillingRequestInput: &billingexpr.RequestInput{
			Body: []byte(`{"stream":true}`),
		},
	}

	quota, result := settleTestQuota(info, types.PriceData{
		ModelRatio:      1,
		CompletionRatio: 2,
	}, &dto.Usage{
		PromptTokens: 1000,
	})

	require.Equal(t, 1500, quota)
	require.NotNil(t, result)
	require.Equal(t, "stream", result.MatchedTier)
}

func TestBuildTestLogOtherInjectsTieredInfo(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())

	info := &relaycommon.RelayInfo{
		TieredBillingSnapshot: &billingexpr.BillingSnapshot{
			BillingMode: "tiered_expr",
			ExprString:  `tier("base", p * 2)`,
		},
		ChannelMeta: &relaycommon.ChannelMeta{},
	}
	priceData := types.PriceData{
		GroupRatioInfo: types.GroupRatioInfo{GroupRatio: 1},
	}
	usage := &dto.Usage{
		PromptTokensDetails: dto.InputTokenDetails{
			CachedTokens: 12,
		},
	}

	other := buildTestLogOther(ctx, info, priceData, usage, &billingexpr.TieredResult{
		MatchedTier: "base",
	})

	require.Equal(t, "tiered_expr", other["billing_mode"])
	require.Equal(t, "base", other["matched_tier"])
	require.NotEmpty(t, other["expr_b64"])
}

func TestResolveChannelTestUserIDUsesRequestUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Set("id", 2)

	userID, err := resolveChannelTestUserID(ctx)

	require.NoError(t, err)
	require.Equal(t, 2, userID)
}

func TestSelectChannelsForAutomaticTestPassiveRecoveryOnlyUsesAutoDisabled(t *testing.T) {
	channels := []*model.Channel{
		{Id: 1, Status: common.ChannelStatusEnabled},
		{Id: 2, Status: common.ChannelStatusAutoDisabled},
		{Id: 3, Status: common.ChannelStatusManuallyDisabled},
	}

	selected := selectChannelsForAutomaticTest(channels, operation_setting.ChannelTestModePassiveRecovery)

	require.Len(t, selected, 1)
	require.Equal(t, 2, selected[0].Id)
}

func TestSelectChannelsForAutomaticTestScheduledSkipsManualDisabled(t *testing.T) {
	channels := []*model.Channel{
		{Id: 1, Status: common.ChannelStatusEnabled},
		{Id: 2, Status: common.ChannelStatusAutoDisabled},
		{Id: 3, Status: common.ChannelStatusManuallyDisabled},
	}

	selected := selectChannelsForAutomaticTest(channels, operation_setting.ChannelTestModeScheduledAll)

	require.Len(t, selected, 2)
	require.Equal(t, 1, selected[0].Id)
	require.Equal(t, 2, selected[1].Id)
}

func TestTestAllChannelsRejectsExistingActiveTask(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.SystemTask{}, &model.SystemTaskLock{}))

	existing, err := model.CreateSystemTask(model.SystemTaskTypeChannelTest, nil, nil)
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/channel/test", nil)

	TestAllChannels(ctx)

	require.Equal(t, http.StatusConflict, recorder.Code)
	require.Contains(t, recorder.Body.String(), existing.TaskID)
	require.Contains(t, recorder.Body.String(), "已有通道测试任务正在运行或等待中")
}

func TestChannelTestRelaysToLocalOpenAIUpstreamAndRecordsUsage(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	service.InitHttpClient()
	require.NoError(t, db.AutoMigrate(&model.Log{}))
	previousModelRatios := ratio_setting.ModelRatio2JSONString()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(previousModelRatios))
		model.InvalidatePricingCache()
	})
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{"local-upstream-model":1}`))
	model.InvalidatePricingCache()
	require.NoError(t, db.Create(&model.User{
		Id:          808,
		Username:    "channel-test-user",
		Password:    "hashed-password",
		DisplayName: "channel-test-user",
		AffCode:     "channel-test-aff",
		Quota:       10_000_000,
		Group:       "default",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
	}).Error)

	type upstreamRequest struct {
		Path          string
		Authorization string
		Model         string
	}
	requests := make(chan upstreamRequest, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var payload struct {
			Model string `json:"model"`
		}
		if err := common.DecodeJson(request.Body, &payload); err != nil {
			http.Error(writer, "invalid request", http.StatusBadRequest)
			return
		}
		requests <- upstreamRequest{
			Path:          request.URL.Path,
			Authorization: request.Header.Get("Authorization"),
			Model:         payload.Model,
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"id":"chatcmpl-local","object":"chat.completion","created":1710000000,"model":"local-upstream-model","choices":[{"index":0,"message":{"role":"assistant","content":"local response"},"finish_reason":"stop"}],"usage":{"prompt_tokens":2,"completion_tokens":1,"total_tokens":3}}`))
	}))
	t.Cleanup(upstream.Close)

	baseURL := upstream.URL
	channel := &model.Channel{
		Id:      809,
		Type:    constant.ChannelTypeOpenAI,
		Key:     "local-upstream-key",
		Status:  common.ChannelStatusEnabled,
		Name:    "local-openai",
		BaseURL: &baseURL,
		Models:  "local-upstream-model",
		Group:   "default",
		Other:   "",
	}

	result := testChannel(context.Background(), channel, 808, "local-upstream-model", "", false)
	require.NoError(t, result.localErr)
	require.Nil(t, result.newAPIError)

	received := <-requests
	require.Equal(t, "/v1/chat/completions", received.Path)
	require.Equal(t, "Bearer local-upstream-key", received.Authorization)
	require.Equal(t, "local-upstream-model", received.Model)

	var logCount int64
	require.NoError(t, db.Model(&model.Log{}).
		Where("user_id = ? AND channel_id = ? AND type = ?", 808, channel.Id, model.LogTypeConsume).
		Count(&logCount).Error)
	require.EqualValues(t, 1, logCount)
}

func TestBuildTestRequestCreatesAlphaSearchEnvelope(t *testing.T) {
	request := buildTestRequest(
		"gpt-alpha-search",
		string(constant.EndpointTypeOpenAIAlphaSearch),
		nil,
		true,
	)

	alphaRequest, ok := request.(*dto.AlphaSearchRequest)
	require.True(t, ok)
	require.Equal(t, "gpt-alpha-search", alphaRequest.Model)
	require.False(t, alphaRequest.IsStream(nil))
	require.JSONEq(t, `{"model":"gpt-alpha-search","query":"test"}`, string(alphaRequest.RawBody))
}

func TestChannelTestAlphaSearchIsUnbilledAdminDiagnostic(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	service.InitHttpClient()
	require.NoError(t, db.AutoMigrate(&model.Log{}, &model.WalletLedgerEntry{}))
	require.NoError(t, db.Create(&model.User{
		Id:           818,
		Username:     "alpha-channel-test-user",
		Password:     "hashed-password",
		DisplayName:  "alpha-channel-test-user",
		AffCode:      "alpha-channel-test-aff",
		Quota:        1_234_567,
		UsedQuota:    321,
		RequestCount: 7,
		Group:        "default",
		Role:         common.RoleCommonUser,
		Status:       common.UserStatusEnabled,
	}).Error)

	type upstreamRequest struct {
		Path          string
		Authorization string
		Model         string
		Query         string
	}
	requests := make(chan upstreamRequest, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var payload struct {
			Model string `json:"model"`
			Query string `json:"query"`
		}
		if err := common.DecodeJson(request.Body, &payload); err != nil {
			http.Error(writer, "invalid request", http.StatusBadRequest)
			return
		}
		requests <- upstreamRequest{
			Path:          request.URL.Path,
			Authorization: request.Header.Get("Authorization"),
			Model:         payload.Model,
			Query:         payload.Query,
		}
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusCreated)
		_, _ = writer.Write([]byte(`{"results":[{"title":"ok"}]}`))
	}))
	t.Cleanup(upstream.Close)

	baseURL := upstream.URL
	channel := &model.Channel{
		Id:      819,
		Type:    constant.ChannelTypeAdvancedCustom,
		Key:     "alpha-upstream-key",
		Status:  common.ChannelStatusEnabled,
		Name:    "local-alpha-search",
		BaseURL: &baseURL,
		Models:  "gpt-alpha-search",
		Group:   "default",
	}
	channel.SetOtherSettings(dto.ChannelOtherSettings{
		AdvancedCustom: &dto.AdvancedCustomConfig{Routes: []dto.AdvancedCustomRoute{
			{
				IncomingPath: "/v1/alpha/search",
				UpstreamPath: "/v1/alpha/search",
			},
		}},
	})

	result := testChannel(
		context.Background(),
		channel,
		818,
		"gpt-alpha-search",
		string(constant.EndpointTypeOpenAIAlphaSearch),
		true,
	)
	require.NoError(t, result.localErr)
	require.Nil(t, result.newAPIError)
	require.True(t, result.context.GetBool(channelTestAdminDiagnosticContextKey))

	received := <-requests
	require.Equal(t, "/v1/alpha/search", received.Path)
	require.Equal(t, "Bearer alpha-upstream-key", received.Authorization)
	require.Equal(t, "gpt-alpha-search", received.Model)
	require.Equal(t, "test", received.Query)

	var user model.User
	require.NoError(t, db.First(&user, 818).Error)
	require.EqualValues(t, 1_234_567, user.Quota)
	require.Equal(t, 321, user.UsedQuota)
	require.Equal(t, 7, user.RequestCount)

	var logCount int64
	require.NoError(t, db.Model(&model.Log{}).Where("user_id = ? AND channel_id = ?", 818, channel.Id).Count(&logCount).Error)
	require.Zero(t, logCount)

	var ledgerCount int64
	require.NoError(t, db.Model(&model.WalletLedgerEntry{}).Where("user_id = ?", 818).Count(&ledgerCount).Error)
	require.Zero(t, ledgerCount)
}
