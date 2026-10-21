package controller

import (
	"bytes"
	"errors"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type errorAwareRefundBillingStub struct {
	err         error
	errorCalls  int
	legacyCalls int
}

func (*errorAwareRefundBillingStub) Settle(int) error          { return nil }
func (s *errorAwareRefundBillingStub) Refund(*gin.Context)     { s.legacyCalls++ }
func (*errorAwareRefundBillingStub) NeedsRefund() bool         { return true }
func (*errorAwareRefundBillingStub) GetPreConsumedQuota() int  { return 1 }
func (*errorAwareRefundBillingStub) Reserve(int) error         { return nil }
func (*errorAwareRefundBillingStub) ReserveToTarget(int) error { return nil }
func (s *errorAwareRefundBillingStub) RefundWithError(*gin.Context) error {
	s.errorCalls++
	return s.err
}

type legacyRefundBillingStub struct{ calls int }

func (*legacyRefundBillingStub) Settle(int) error          { return nil }
func (s *legacyRefundBillingStub) Refund(*gin.Context)     { s.calls++ }
func (*legacyRefundBillingStub) NeedsRefund() bool         { return true }
func (*legacyRefundBillingStub) GetPreConsumedQuota() int  { return 1 }
func (*legacyRefundBillingStub) Reserve(int) error         { return nil }
func (*legacyRefundBillingStub) ReserveToTarget(int) error { return nil }

func TestRefundRelayBillingAfterFailurePrefersErrorAwareContractAndLogsCriticalFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
	expected := errors.New("durable refund exhausted retries")
	billing := &errorAwareRefundBillingStub{err: expected}
	info := &relaycommon.RelayInfo{RequestId: "refund-critical-request", UserId: 42, Billing: billing}

	var logs bytes.Buffer
	common.LogWriterMu.Lock()
	original := gin.DefaultErrorWriter
	gin.DefaultErrorWriter = &logs
	common.LogWriterMu.Unlock()
	t.Cleanup(func() {
		common.LogWriterMu.Lock()
		gin.DefaultErrorWriter = original
		common.LogWriterMu.Unlock()
	})

	refundRelayBillingAfterFailure(ctx, info, "relay")

	require.Equal(t, 1, billing.errorCalls)
	require.Zero(t, billing.legacyCalls)
	require.Contains(t, logs.String(), "CRITICAL billing refund failed")
	require.Contains(t, logs.String(), expected.Error())
}

func TestRefundRelayBillingAfterFailureFallsBackToLegacyContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	billing := &legacyRefundBillingStub{}
	info := &relaycommon.RelayInfo{Billing: billing}

	refundRelayBillingAfterFailure(ctx, info, "task")

	require.Equal(t, 1, billing.calls)
}

func TestRelayAndTaskDefersUseSharedRefundRecovery(t *testing.T) {
	source, err := os.ReadFile("relay.go")
	require.NoError(t, err)
	require.Equal(t, 2, strings.Count(string(source), "refundRelayBillingAfterFailure(c, relayInfo"))
}
