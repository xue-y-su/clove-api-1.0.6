package service

import (
	"bytes"
	"errors"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type finalizerBillingStub struct {
	settleErr error
	calls     atomic.Int32
}

func (s *finalizerBillingStub) Settle(int) error {
	s.calls.Add(1)
	return s.settleErr
}
func (*finalizerBillingStub) Refund(*gin.Context)       {}
func (*finalizerBillingStub) NeedsRefund() bool         { return false }
func (*finalizerBillingStub) GetPreConsumedQuota() int  { return 0 }
func (*finalizerBillingStub) Reserve(int) error         { return nil }
func (*finalizerBillingStub) ReserveToTarget(int) error { return nil }

type errorAwarePreserverBillingStub struct {
	finalizerBillingStub
	preserveErr         error
	errorPreserveCalls  atomic.Int32
	legacyPreserveCalls atomic.Int32
}

func (s *errorAwarePreserverBillingStub) PreserveReservationAfterDeliveryWithError() error {
	s.errorPreserveCalls.Add(1)
	return s.preserveErr
}

func (s *errorAwarePreserverBillingStub) PreserveReservationAfterDelivery() {
	s.legacyPreserveCalls.Add(1)
}

func newRelayBillingFinalizerFixture(t *testing.T, userID int, billing relaycommon.BillingSettler) (*gin.Context, *relaycommon.RelayInfo) {
	t.Helper()
	truncate(t)
	require.NoError(t, model.DB.AutoMigrate(&model.IPSecurityEvent{}))
	t.Cleanup(func() { model.DB.Exec("DELETE FROM ip_security_events") })
	user := model.User{Id: userID, Username: "finalizer-user", Quota: 1000, Status: common.UserStatusEnabled, Group: "default"}
	require.NoError(t, model.DB.Create(&user).Error)
	channel := model.Channel{Id: userID + 1, Name: "finalizer-channel", Status: common.ChannelStatusEnabled}
	require.NoError(t, model.DB.Create(&channel).Error)
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
	info := &relaycommon.RelayInfo{
		UserId:          user.Id,
		UserQuota:       user.Quota,
		TokenId:         userID + 2,
		OriginModelName: "gpt-finalizer",
		UsingGroup:      "default",
		StartTime:       time.Now(),
		Billing:         billing,
		PriceData: types.PriceData{
			ModelRatio:      1,
			CompletionRatio: 1,
			GroupRatioInfo:  types.GroupRatioInfo{GroupRatio: 1},
		},
		ChannelMeta: &relaycommon.ChannelMeta{ChannelId: channel.Id},
	}
	return ctx, info
}

func TestFinalizeRelayBillingFailureWritesNoStatsOrConsumeLog(t *testing.T) {
	expected := errors.New("funding settlement failed")
	billing := &finalizerBillingStub{settleErr: expected}
	ctx, info := newRelayBillingFinalizerFixture(t, 98700, billing)
	usage := &dto.Usage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15}

	first := FinalizeRelayBilling(ctx, info, usage, DeliveryState{})
	second := FinalizeRelayBilling(ctx, info, usage, DeliveryState{})

	require.ErrorIs(t, first, expected)
	require.ErrorIs(t, second, expected)
	require.EqualValues(t, 1, billing.calls.Load())
	var stored model.User
	require.NoError(t, model.DB.First(&stored, info.UserId).Error)
	require.Zero(t, stored.UsedQuota)
	require.Zero(t, stored.RequestCount)
	var logCount int64
	require.NoError(t, model.DB.Model(&model.Log{}).Where("user_id = ?", info.UserId).Count(&logCount).Error)
	require.Zero(t, logCount)
}

func TestFinalizeRelayBillingSuccessWritesStatsAndConsumeLogOnce(t *testing.T) {
	billing := &finalizerBillingStub{}
	ctx, info := newRelayBillingFinalizerFixture(t, 98710, billing)
	usage := &dto.Usage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15}

	require.NoError(t, FinalizeRelayBilling(ctx, info, usage, DeliveryState{}))
	require.NoError(t, FinalizeRelayBilling(ctx, info, usage, DeliveryState{}))

	require.EqualValues(t, 1, billing.calls.Load())
	var stored model.User
	require.NoError(t, model.DB.First(&stored, info.UserId).Error)
	require.Equal(t, 15, stored.UsedQuota)
	require.Equal(t, 1, stored.RequestCount)
	var logCount int64
	require.NoError(t, model.DB.Model(&model.Log{}).Where("user_id = ?", info.UserId).Count(&logCount).Error)
	require.EqualValues(t, 1, logCount)
}

func TestFinalizeRelayBillingDeliveredStreamPreservesReservationWhenExtraChargeFails(t *testing.T) {
	setupBillingReservationTest(t)
	user := seedBillingReservationUser(t, 98720, 100, "wallet_only")
	ctx := newBillingReservationContext()
	info := newBillingReservationInfo(user, "billing-delivered-stream")
	info.IsStream = true
	info.StartTime = time.Now()
	info.PriceData = types.PriceData{
		ModelRatio:      1,
		CompletionRatio: 1,
		GroupRatioInfo:  types.GroupRatioInfo{GroupRatio: 1},
	}
	session, apiErr := NewBillingSession(ctx, info, 100)
	require.Nil(t, apiErr)
	info.Billing = session

	err := FinalizeRelayBilling(ctx, info, &dto.Usage{PromptTokens: 150, TotalTokens: 150}, DeliveryState{
		Stream:            true,
		ResponseForwarded: true,
	})

	require.ErrorIs(t, err, model.ErrWalletInsufficientBalance)
	require.False(t, session.NeedsRefund(), "delivered output must keep the amount already reserved")
	session.Refund(ctx)
	var stored model.User
	require.NoError(t, model.DB.First(&stored, user.Id).Error)
	require.Zero(t, stored.Quota)
}

func TestFinalizeBillingForDeliveryLogsCriticalPreservationFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	settleErr := errors.New("settlement failed after delivery")
	preserveErr := errors.New("reservation terminal state persistence failed")
	billing := &errorAwarePreserverBillingStub{preserveErr: preserveErr}
	info := &relaycommon.RelayInfo{RequestId: "preserve-critical-request", UserId: 98725, Billing: billing}

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

	err := finalizeBillingForDelivery(ctx, info, DeliveryState{ResponseForwarded: true}, func() error {
		return settleErr
	})

	require.ErrorIs(t, err, settleErr)
	require.EqualValues(t, 1, billing.errorPreserveCalls.Load())
	require.Zero(t, billing.legacyPreserveCalls.Load())
	require.Contains(t, logs.String(), "CRITICAL billing reservation preservation failed")
	require.Contains(t, logs.String(), preserveErr.Error())
}

func TestFinalizeAudioBillingReturnsSettlementErrorBeforeWritingSuccessAccounting(t *testing.T) {
	expected := errors.New("audio settlement failed")
	billing := &finalizerBillingStub{settleErr: expected}
	ctx, info := newRelayBillingFinalizerFixture(t, 98730, billing)
	usage := &dto.Usage{PromptTokens: 10, TotalTokens: 10}
	usage.PromptTokensDetails.AudioTokens = 10

	err := FinalizeAudioBilling(ctx, info, usage, "", DeliveryState{})

	require.ErrorIs(t, err, expected)
	var stored model.User
	require.NoError(t, model.DB.First(&stored, info.UserId).Error)
	require.Zero(t, stored.UsedQuota)
	require.Zero(t, stored.RequestCount)
	var logCount int64
	require.NoError(t, model.DB.Model(&model.Log{}).Where("user_id = ?", info.UserId).Count(&logCount).Error)
	require.Zero(t, logCount)
}

func TestFinalizeWssBillingReturnsSettlementErrorBeforeWritingSuccessAccounting(t *testing.T) {
	expected := errors.New("realtime settlement failed")
	billing := &finalizerBillingStub{settleErr: expected}
	ctx, info := newRelayBillingFinalizerFixture(t, 98740, billing)
	info.IsStream = true
	usage := &dto.RealtimeUsage{InputTokens: 10, TotalTokens: 10}
	usage.InputTokenDetails.TextTokens = 10

	err := FinalizeWssBilling(ctx, info, info.OriginModelName, usage, "", DeliveryState{
		Stream:            true,
		ResponseForwarded: true,
	})

	require.ErrorIs(t, err, expected)
	var stored model.User
	require.NoError(t, model.DB.First(&stored, info.UserId).Error)
	require.Zero(t, stored.UsedQuota)
	require.Zero(t, stored.RequestCount)
	var logCount int64
	require.NoError(t, model.DB.Model(&model.Log{}).Where("user_id = ?", info.UserId).Count(&logCount).Error)
	require.Zero(t, logCount)
}

func TestFinalizeRelayBillingKeepsSettledSuccessWhenUserStatsProjectionFails(t *testing.T) {
	billing := &finalizerBillingStub{}
	ctx, info := newRelayBillingFinalizerFixture(t, 98750, billing)
	expected := errors.New("forced user stats persistence failure")
	const callbackName = "test:relay_finalizer_user_stats_failure"
	require.NoError(t, model.DB.Callback().Update().Before("gorm:update").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement.Schema != nil && tx.Statement.Schema.Name == "User" {
			tx.AddError(expected)
		}
	}))
	defer func() { require.NoError(t, model.DB.Callback().Update().Remove(callbackName)) }()

	err := FinalizeRelayBilling(ctx, info, &dto.Usage{PromptTokens: 5, TotalTokens: 5}, DeliveryState{})

	require.NoError(t, err)
	require.EqualValues(t, 1, billing.calls.Load())
	var logCount int64
	require.NoError(t, model.DB.Model(&model.Log{}).Where("user_id = ?", info.UserId).Count(&logCount).Error)
	require.EqualValues(t, 1, logCount)
}

func TestFinalizeRelayBillingKeepsSettledSuccessWhenConsumeLogProjectionFails(t *testing.T) {
	billing := &finalizerBillingStub{}
	ctx, info := newRelayBillingFinalizerFixture(t, 98760, billing)
	expected := errors.New("forced consume log persistence failure")
	const callbackName = "test:relay_finalizer_log_failure"
	require.NoError(t, model.DB.Callback().Create().Before("gorm:create").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement.Schema != nil && tx.Statement.Schema.Name == "Log" {
			tx.AddError(expected)
		}
	}))
	defer func() { require.NoError(t, model.DB.Callback().Create().Remove(callbackName)) }()

	err := FinalizeRelayBilling(ctx, info, &dto.Usage{PromptTokens: 5, TotalTokens: 5}, DeliveryState{})

	require.NoError(t, err)
	require.EqualValues(t, 1, billing.calls.Load())
}
