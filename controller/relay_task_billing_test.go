package controller

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type failingTaskBilling struct {
	err         error
	settleCalls int
}

func (b *failingTaskBilling) Settle(int) error {
	b.settleCalls++
	return b.err
}
func (*failingTaskBilling) Refund(*gin.Context)       {}
func (*failingTaskBilling) NeedsRefund() bool         { return true }
func (*failingTaskBilling) GetPreConsumedQuota() int  { return 10 }
func (*failingTaskBilling) Reserve(int) error         { return nil }
func (*failingTaskBilling) ReserveToTarget(int) error { return nil }

func TestFinalizeTaskSubmissionWithholdsResponseWhenSettlementFails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	buffer := newRelayResponseBuffer(ctx.Writer, 1024)
	_, err := buffer.Write([]byte(`{"id":"upstream-task"}`))
	require.NoError(t, err)
	settleErr := errors.New("forced task settlement failure")
	billing := &failingTaskBilling{err: settleErr}
	info := &relaycommon.RelayInfo{Billing: billing}

	taskErr := finalizeTaskSubmission(ctx, info, &relay.TaskSubmitResult{Quota: 25}, buffer)

	require.NotNil(t, taskErr)
	require.ErrorIs(t, taskErr.Error, settleErr)
	require.Equal(t, 1, billing.settleCalls)
	require.Empty(t, recorder.Body.String(), "the upstream success body must stay buffered when settlement fails")
}

func TestFinalizeTaskSubmissionCommitsSettledResponseWhenUsageProjectionFails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	buffer := newRelayResponseBuffer(ctx.Writer, 1024)
	buffer.Header().Set("Content-Type", "application/json")
	buffer.WriteHeader(http.StatusCreated)
	_, err := buffer.Write([]byte(`{"id":"upstream-task"}`))
	require.NoError(t, err)

	originalLog := logTaskSubmissionConsumption
	originalInsert := insertSubmittedTask
	logTaskSubmissionConsumption = func(*gin.Context, *relaycommon.RelayInfo) error {
		return errors.New("forced task log projection failure")
	}
	insertCalls := 0
	insertSubmittedTask = func(*model.Task) error { insertCalls++; return nil }
	t.Cleanup(func() {
		logTaskSubmissionConsumption = originalLog
		insertSubmittedTask = originalInsert
	})

	billing := &failingTaskBilling{}
	info := &relaycommon.RelayInfo{
		Billing:       billing,
		TaskRelayInfo: &relaycommon.TaskRelayInfo{PublicTaskID: "public-task"},
		ChannelMeta:   &relaycommon.ChannelMeta{},
	}
	taskErr := finalizeTaskSubmission(ctx, info, &relay.TaskSubmitResult{
		Platform: constant.TaskPlatform("test"),
		Quota:    25,
	}, buffer)

	require.Nil(t, taskErr)
	require.Equal(t, 1, billing.settleCalls)
	require.Equal(t, 1, insertCalls)
	require.Equal(t, http.StatusCreated, recorder.Code)
	require.JSONEq(t, `{"id":"upstream-task"}`, recorder.Body.String())
}

func TestFinalizeTaskSubmissionCommitsSettledResponseWhenTaskProjectionFails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	buffer := newRelayResponseBuffer(ctx.Writer, 1024)
	buffer.Header().Set("Content-Type", "application/json")
	_, err := buffer.Write([]byte(`{"id":"upstream-task"}`))
	require.NoError(t, err)

	originalLog := logTaskSubmissionConsumption
	originalInsert := insertSubmittedTask
	logTaskSubmissionConsumption = func(*gin.Context, *relaycommon.RelayInfo) error { return nil }
	insertSubmittedTask = func(*model.Task) error { return errors.New("forced task projection failure") }
	t.Cleanup(func() {
		logTaskSubmissionConsumption = originalLog
		insertSubmittedTask = originalInsert
	})

	billing := &failingTaskBilling{}
	info := &relaycommon.RelayInfo{
		Billing:       billing,
		TaskRelayInfo: &relaycommon.TaskRelayInfo{PublicTaskID: "public-task"},
		ChannelMeta:   &relaycommon.ChannelMeta{},
	}
	taskErr := finalizeTaskSubmission(ctx, info, &relay.TaskSubmitResult{
		Platform: constant.TaskPlatform("test"),
		Quota:    25,
	}, buffer)

	require.Nil(t, taskErr)
	require.Equal(t, 1, billing.settleCalls)
	require.JSONEq(t, `{"id":"upstream-task"}`, recorder.Body.String())
}
