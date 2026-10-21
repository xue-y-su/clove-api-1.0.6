package service

import (
	"errors"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type settlementOrderFundingStub struct {
	settleCalls int
	settleErr   error
}

func (*settlementOrderFundingStub) Source() string       { return BillingSourceWallet }
func (*settlementOrderFundingStub) PreConsume(int) error { return nil }
func (s *settlementOrderFundingStub) Settle(int) error   { s.settleCalls++; return s.settleErr }
func (*settlementOrderFundingStub) Refund() error        { return nil }

type refundRetryFundingStub struct {
	failures int32
	calls    atomic.Int32
}

func (*refundRetryFundingStub) Source() string       { return BillingSourceWallet }
func (*refundRetryFundingStub) PreConsume(int) error { return nil }
func (*refundRetryFundingStub) Settle(int) error     { return nil }
func (s *refundRetryFundingStub) Refund() error {
	call := s.calls.Add(1)
	if call <= s.failures {
		return errors.New("transient refund failure")
	}
	return nil
}

func TestBillingSessionDoesNotCommitFundingWhenTokenAdjustmentFails(t *testing.T) {
	expected := errors.New("token adjustment failed")
	funding := &settlementOrderFundingStub{}
	session := &BillingSession{
		relayInfo:        &relaycommon.RelayInfo{TokenId: 10, TokenKey: "test-token"},
		funding:          funding,
		preConsumedQuota: 10,
		tokenConsumed:    10,
	}
	originalDecrease := decreaseBillingTokenQuota
	decreaseBillingTokenQuota = func(int, string, int) error { return expected }
	t.Cleanup(func() { decreaseBillingTokenQuota = originalDecrease })

	err := session.Settle(15)

	require.ErrorIs(t, err, expected)
	require.Zero(t, funding.settleCalls)
	require.False(t, session.fundingSettled)
	require.False(t, session.settled)
}

func TestBillingSessionRollsBackTokenAdjustmentWhenFundingCommitFails(t *testing.T) {
	expected := errors.New("funding commit failed")
	funding := &settlementOrderFundingStub{settleErr: expected}
	session := &BillingSession{
		relayInfo:        &relaycommon.RelayInfo{TokenId: 11, TokenKey: "test-token"},
		funding:          funding,
		preConsumedQuota: 10,
		tokenConsumed:    10,
	}
	originalDecrease := decreaseBillingTokenQuota
	originalIncrease := increaseBillingTokenQuota
	decreaseCalls := 0
	increaseCalls := 0
	decreaseBillingTokenQuota = func(int, string, int) error { decreaseCalls++; return nil }
	increaseBillingTokenQuota = func(int, string, int) error { increaseCalls++; return nil }
	t.Cleanup(func() {
		decreaseBillingTokenQuota = originalDecrease
		increaseBillingTokenQuota = originalIncrease
	})

	err := session.Settle(15)

	require.ErrorIs(t, err, expected)
	require.Equal(t, 1, funding.settleCalls)
	require.Equal(t, 1, decreaseCalls)
	require.Equal(t, 1, increaseCalls)
	require.False(t, session.settled)
}

func TestBillingSessionRefundRetriesTransientFundingFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	funding := &refundRetryFundingStub{failures: 2}
	session := &BillingSession{
		relayInfo:        &relaycommon.RelayInfo{UserId: 12, TokenId: 12, TokenKey: "refund-token"},
		funding:          funding,
		preConsumedQuota: 1,
		tokenConsumed:    1,
	}
	originalIncrease := increaseBillingTokenQuota
	increaseBillingTokenQuota = func(int, string, int) error { return nil }
	t.Cleanup(func() { increaseBillingTokenQuota = originalIncrease })

	session.Refund(ctx)

	require.Eventually(t, func() bool { return funding.calls.Load() == 3 }, 2*time.Second, 20*time.Millisecond)
	require.True(t, session.refunded)
}

func TestBillingSessionRefundFailureRemainsRetryable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	funding := &refundRetryFundingStub{failures: 10}
	session := &BillingSession{
		relayInfo:        &relaycommon.RelayInfo{UserId: 13, TokenId: 13, TokenKey: "refund-token"},
		funding:          funding,
		preConsumedQuota: 1,
		tokenConsumed:    1,
	}
	originalIncrease := increaseBillingTokenQuota
	increaseBillingTokenQuota = func(int, string, int) error { return nil }
	t.Cleanup(func() { increaseBillingTokenQuota = originalIncrease })

	session.Refund(ctx)
	require.Eventually(t, func() bool { return funding.calls.Load() >= 1 }, time.Second, 20*time.Millisecond)
	session.Refund(ctx)

	require.Eventually(t, func() bool { return funding.calls.Load() >= 2 }, time.Second, 20*time.Millisecond)
	require.False(t, session.refunded)
}

func TestBillingSessionRefundExposesFinalFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	funding := &refundRetryFundingStub{failures: 10}
	session := &BillingSession{
		relayInfo:        &relaycommon.RelayInfo{UserId: 14, TokenId: 14, TokenKey: "refund-token"},
		funding:          funding,
		preConsumedQuota: 1,
		tokenConsumed:    1,
	}
	originalIncrease := increaseBillingTokenQuota
	increaseBillingTokenQuota = func(int, string, int) error { return nil }
	t.Cleanup(func() { increaseBillingTokenQuota = originalIncrease })

	err := session.RefundWithError(ctx)

	require.ErrorContains(t, err, "transient refund failure")
	require.EqualValues(t, 3, funding.calls.Load())
	require.True(t, session.NeedsRefund())
}
