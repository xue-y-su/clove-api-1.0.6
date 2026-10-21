package relay

import (
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type relayFinalizationFailureStub struct {
	err           error
	preserveCalls int
}

func (s *relayFinalizationFailureStub) Settle(int) error        { return s.err }
func (*relayFinalizationFailureStub) Refund(*gin.Context)       {}
func (*relayFinalizationFailureStub) NeedsRefund() bool         { return false }
func (*relayFinalizationFailureStub) GetPreConsumedQuota() int  { return 0 }
func (*relayFinalizationFailureStub) Reserve(int) error         { return nil }
func (*relayFinalizationFailureStub) ReserveToTarget(int) error { return nil }
func (s *relayFinalizationFailureStub) PreserveReservationAfterDelivery() {
	s.preserveCalls++
}

type relayForwardStateWriter struct {
	gin.ResponseWriter
	forwarded bool
}

func (w *relayForwardStateWriter) ResponseForwarded() bool { return w.forwarded }

func TestFinalizeRelayResponseBillingReturnsSettlementError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("POST", "/v1/embeddings", nil)
	info := &relaycommon.RelayInfo{
		OriginModelName: "billing-error-model",
		UsingGroup:      "default",
		Billing:         &relayFinalizationFailureStub{err: errors.New("settlement failed")},
		PriceData: types.PriceData{
			ModelRatio:      1,
			CompletionRatio: 1,
			GroupRatioInfo:  types.GroupRatioInfo{GroupRatio: 1},
		},
	}

	apiErr := finalizeRelayResponseBilling(ctx, info, &dto.Usage{PromptTokens: 1, TotalTokens: 1}, nil)

	require.NotNil(t, apiErr)
	require.Equal(t, types.ErrorCodeUpdateDataError, apiErr.GetErrorCode())
}

func TestFinalizeRelayResponseBillingUsesActualForwardedState(t *testing.T) {
	for _, tc := range []struct {
		name              string
		forwarded         bool
		wantPreserveCalls int
	}{
		{name: "stream declared but no bytes forwarded", forwarded: false, wantPreserveCalls: 0},
		{name: "stream bytes already forwarded", forwarded: true, wantPreserveCalls: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			ctx.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
			ctx.Writer = &relayForwardStateWriter{ResponseWriter: ctx.Writer, forwarded: tc.forwarded}
			billing := &relayFinalizationFailureStub{err: errors.New("settlement failed")}
			info := &relaycommon.RelayInfo{
				IsStream:        true,
				OriginModelName: "billing-stream-model",
				UsingGroup:      "default",
				Billing:         billing,
				PriceData: types.PriceData{
					ModelRatio:      1,
					CompletionRatio: 1,
					GroupRatioInfo:  types.GroupRatioInfo{GroupRatio: 1},
				},
			}

			apiErr := finalizeRelayResponseBilling(ctx, info, &dto.Usage{PromptTokens: 1, TotalTokens: 1}, nil)

			require.NotNil(t, apiErr)
			require.Equal(t, tc.wantPreserveCalls, billing.preserveCalls)
		})
	}
}
