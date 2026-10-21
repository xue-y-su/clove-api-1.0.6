package relay

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	rootcommon "github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type alphaSearchBillingStub struct {
	preConsumed    int
	reserved       atomic.Int64
	reserveChanges atomic.Int32
	refunds        atomic.Int32
}

func newAlphaSearchBillingStub(preConsumed int) *alphaSearchBillingStub {
	stub := &alphaSearchBillingStub{preConsumed: preConsumed}
	stub.reserved.Store(int64(preConsumed))
	return stub
}

func (*alphaSearchBillingStub) Settle(int) error { return nil }
func (s *alphaSearchBillingStub) Refund(*gin.Context) {
	s.refunds.Add(1)
}
func (*alphaSearchBillingStub) NeedsRefund() bool { return true }
func (s *alphaSearchBillingStub) GetPreConsumedQuota() int {
	return s.preConsumed
}
func (s *alphaSearchBillingStub) Reserve(target int) error {
	return s.ReserveToTarget(target)
}
func (s *alphaSearchBillingStub) ReserveToTarget(target int) error {
	for {
		current := s.reserved.Load()
		if int64(target) <= current {
			return nil
		}
		if s.reserved.CompareAndSwap(current, int64(target)) {
			s.reserveChanges.Add(1)
			return nil
		}
	}
}

func TestBuildAlphaSearchRequestBodyPreservesUnknownFieldsAndOnlyMapsTopLevelModel(t *testing.T) {
	raw := []byte(`{
		"model":"gpt-public",
		"input":[{"role":"user","content":"model=gpt-public"}],
		"metadata":{"model":"keep-me"},
		"commands":{"search_query":[{"q":"weather","recency":1}]},
		"future_field":{"nested":true}
	}`)

	out, err := buildAlphaSearchRequestBody(raw, "gpt-public", "gpt-upstream")
	require.NoError(t, err)

	var body map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(out, &body))
	require.JSONEq(t, `"gpt-upstream"`, string(body["model"]))
	require.JSONEq(t, `{"model":"keep-me"}`, string(body["metadata"]))
	require.JSONEq(t, `[{"role":"user","content":"model=gpt-public"}]`, string(body["input"]))
	require.JSONEq(t, `{"search_query":[{"q":"weather","recency":1}]}`, string(body["commands"]))
	require.JSONEq(t, `{"nested":true}`, string(body["future_field"]))
}

func TestBuildAlphaSearchRequestBodyWithoutMappingKeepsExactBytes(t *testing.T) {
	raw := []byte("{\n  \"model\": \"gpt-public\", \"future\": 1.2300\n}")

	out, err := buildAlphaSearchRequestBody(raw, "gpt-public", "gpt-public")
	require.NoError(t, err)
	require.Equal(t, raw, out)
}

func TestAlphaSearchHelperReservesBeforeUpstreamAndFinalizesBeforeDelivery(t *testing.T) {
	billing := newAlphaSearchBillingStub(25)
	var reservedAtUpstream atomic.Int64
	var upstreamBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reservedAtUpstream.Store(billing.reserved.Load())
		upstreamBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/vnd.clove.search+json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"result":"ok"}`))
	}))
	t.Cleanup(server.Close)

	raw := []byte(`{"model":"gpt-public","query":"hello","metadata":{"model":"nested"},"future_field":true}`)
	ctx, recorder, info := newAlphaSearchHandlerFixture(t, server.URL, raw, billing)
	ctx.Set("model_mapping", `{"gpt-public":"gpt-upstream"}`)
	finalizedBeforeWrite := false
	finalizer := func(c *gin.Context, relayInfo *relaycommon.RelayInfo, usage *dto.Usage, _ service.DeliveryState) error {
		finalizedBeforeWrite = !c.Writer.Written()
		require.Zero(t, usage.TotalTokens)
		require.NoError(t, relayInfo.CommitToolBillingAttempt())
		result, err := relayInfo.ToolBillingMeter.TargetToolQuota()
		require.NoError(t, err)
		require.Len(t, result.Items, 1)
		require.Equal(t, dto.BuildInToolWebSearchPreview, result.Items[0].Name)
		require.Equal(t, 1, result.Items[0].Count)
		return nil
	}

	apiErr := alphaSearchHelper(ctx, info, finalizer)
	require.Nil(t, apiErr)
	require.Greater(t, reservedAtUpstream.Load(), int64(billing.preConsumed))
	require.EqualValues(t, 1, billing.reserveChanges.Load())
	require.True(t, finalizedBeforeWrite)
	require.Equal(t, http.StatusCreated, recorder.Code)
	require.Equal(t, "application/vnd.clove.search+json", recorder.Header().Get("Content-Type"))
	require.JSONEq(t, `{"result":"ok"}`, recorder.Body.String())

	var forwarded map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(upstreamBody, &forwarded))
	require.JSONEq(t, `"gpt-upstream"`, string(forwarded["model"]))
	require.JSONEq(t, `{"model":"nested"}`, string(forwarded["metadata"]))
	require.JSONEq(t, `true`, string(forwarded["future_field"]))
}

func TestAlphaSearchHelperRetriesFailedAttemptAndChargesSuccessfulAttemptOnce(t *testing.T) {
	billing := newAlphaSearchBillingStub(10)
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if attempts.Add(1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":{"message":"retry me"}}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result":"ok"}`))
	}))
	t.Cleanup(server.Close)

	ctx, _, info := newAlphaSearchHandlerFixture(t, server.URL, []byte(`{"model":"gpt-public","query":"hello"}`), billing)
	finalizerCalls := 0
	finalizer := func(_ *gin.Context, relayInfo *relaycommon.RelayInfo, _ *dto.Usage, _ service.DeliveryState) error {
		finalizerCalls++
		return relayInfo.CommitToolBillingAttempt()
	}

	firstErr := alphaSearchHelper(ctx, info, finalizer)
	require.NotNil(t, firstErr)
	require.Zero(t, finalizerCalls)
	info.DiscardToolBillingAttempt()
	firstResult, err := info.ToolBillingMeter.TargetToolQuota()
	require.NoError(t, err)
	require.Zero(t, firstResult.ToolQuota)

	info.BeginToolBillingAttempt()
	secondErr := alphaSearchHelper(ctx, info, finalizer)
	require.Nil(t, secondErr)
	require.Equal(t, 1, finalizerCalls)
	require.EqualValues(t, 2, attempts.Load())
	require.EqualValues(t, 1, billing.reserveChanges.Load(), "retry must not increase the request-level reservation")
	result, err := info.ToolBillingMeter.TargetToolQuota()
	require.NoError(t, err)
	require.Len(t, result.Items, 1)
	require.Equal(t, 1, result.Items[0].Count)
}

func TestAlphaSearchHelperUnsupportedAdaptorIsRetryableWithoutBillingOrErrorLog(t *testing.T) {
	billing := newAlphaSearchBillingStub(0)
	ctx, _, info := newAlphaSearchHandlerFixture(t, "https://unused.example", []byte(`{"model":"gpt-public"}`), billing)
	rootcommon.SetContextKey(ctx, constant.ContextKeyChannelType, constant.ChannelTypeOpenAI)
	finalizer := func(*gin.Context, *relaycommon.RelayInfo, *dto.Usage, service.DeliveryState) error {
		return errors.New("must not finalize")
	}

	apiErr := alphaSearchHelper(ctx, info, finalizer)
	require.NotNil(t, apiErr)
	require.Equal(t, http.StatusServiceUnavailable, apiErr.StatusCode)
	require.False(t, types.IsSkipRetryError(apiErr))
	require.False(t, types.IsRecordErrorLog(apiErr))
	require.Zero(t, billing.reserveChanges.Load())
	require.Zero(t, billing.reserved.Load())
}

func TestAlphaSearchHelperSettlementFailureDoesNotDeliverUpstreamResponse(t *testing.T) {
	billing := newAlphaSearchBillingStub(0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"result":"must-not-leak"}`))
	}))
	t.Cleanup(server.Close)

	ctx, recorder, info := newAlphaSearchHandlerFixture(t, server.URL, []byte(`{"model":"gpt-public"}`), billing)
	expected := errors.New("settlement failed")

	apiErr := alphaSearchHelper(ctx, info, func(*gin.Context, *relaycommon.RelayInfo, *dto.Usage, service.DeliveryState) error {
		return expected
	})
	require.ErrorIs(t, apiErr, expected)
	require.False(t, recorder.Flushed)
	require.Empty(t, recorder.Body.String())
}

func TestAlphaSearchHelperRejectsOversizedSuccessBodyBeforeBillingOrDelivery(t *testing.T) {
	originalLimit := constant.UpstreamResponseBodyLimitMB
	constant.UpstreamResponseBodyLimitMB = 1
	t.Cleanup(func() { constant.UpstreamResponseBodyLimitMB = originalLimit })

	billing := newAlphaSearchBillingStub(0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(bytes.Repeat([]byte("x"), 1<<20+1))
	}))
	t.Cleanup(server.Close)

	ctx, recorder, info := newAlphaSearchHandlerFixture(t, server.URL, []byte(`{"model":"gpt-public"}`), billing)
	finalizerCalls := 0

	apiErr := alphaSearchHelper(ctx, info, func(*gin.Context, *relaycommon.RelayInfo, *dto.Usage, service.DeliveryState) error {
		finalizerCalls++
		return nil
	})

	require.NotNil(t, apiErr)
	require.Equal(t, types.ErrorCodeReadResponseBodyFailed, apiErr.GetErrorCode())
	require.Zero(t, finalizerCalls)
	require.Empty(t, recorder.Body.String())
}

func newAlphaSearchHandlerFixture(t *testing.T, baseURL string, raw []byte, billing *alphaSearchBillingStub) (*gin.Context, *httptest.ResponseRecorder, *relaycommon.RelayInfo) {
	t.Helper()
	service.InitHttpClient()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/alpha/search", bytes.NewReader(raw))
	ctx.Request.Header.Set("Content-Type", "application/json")
	rootcommon.SetContextKey(ctx, constant.ContextKeyOriginalModel, "gpt-public")
	rootcommon.SetContextKey(ctx, constant.ContextKeyChannelType, constant.ChannelTypeAdvancedCustom)
	rootcommon.SetContextKey(ctx, constant.ContextKeyChannelId, 73)
	rootcommon.SetContextKey(ctx, constant.ContextKeyChannelBaseUrl, baseURL)
	rootcommon.SetContextKey(ctx, constant.ContextKeyChannelKey, "sk-test")
	rootcommon.SetContextKey(ctx, constant.ContextKeyChannelSetting, dto.ChannelSettings{})
	rootcommon.SetContextKey(ctx, constant.ContextKeyChannelOtherSetting, dto.ChannelOtherSettings{
		AdvancedCustom: &dto.AdvancedCustomConfig{Routes: []dto.AdvancedCustomRoute{
			{
				IncomingPath: "/v1/alpha/search",
				UpstreamPath: "/v1/alpha/search",
			},
		}},
	})

	request := &dto.AlphaSearchRequest{Model: "gpt-public", RawBody: append(json.RawMessage(nil), raw...)}
	info := &relaycommon.RelayInfo{
		RelayFormat:     types.RelayFormatOpenAIAlphaSearch,
		RelayMode:       relayconstant.RelayModeAlphaSearch,
		RequestURLPath:  "/v1/alpha/search",
		RequestId:       "req-alpha-search",
		Request:         request,
		OriginModelName: "gpt-public",
		StartTime:       time.Now(),
		Billing:         billing,
		PriceData: types.PriceData{
			ModelRatio:      1,
			CompletionRatio: 1,
			GroupRatioInfo:  types.GroupRatioInfo{GroupRatio: 1},
		},
	}
	info.InitializeToolBillingMeter()
	info.SetToolBillingBaseQuota(billing.preConsumed)
	info.BeginToolBillingAttempt()
	return ctx, recorder, info
}
