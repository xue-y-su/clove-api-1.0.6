package service

import (
	"context"
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestFinalizeTaskTerminalSuccessSettlesPersistedQuota(t *testing.T) {
	truncate(t)
	const userID, tokenID, channelID = 41, 41, 41
	seedUser(t, userID, 5000)
	seedToken(t, tokenID, userID, "sk-terminal-success", 1000)
	seedChannel(t, channelID)
	task := makeTask(userID, channelID, 5000, tokenID, BillingSourceWallet, 0)
	task.TaskID = "task_terminal_success"
	require.NoError(t, model.DB.Create(task).Error)

	applied, err := FinalizeTaskTerminal(context.Background(), task.ID, model.TaskTerminalMutation{
		FromStatus:  model.TaskStatusInProgress,
		ToStatus:    model.TaskStatusSuccess,
		ActualQuota: 3000,
		ResultURL:   "https://example.test/result",
	})
	require.NoError(t, err)
	require.True(t, applied)
	require.Equal(t, 7000, getUserQuota(t, userID))
	require.Equal(t, 3000, getTokenRemainQuota(t, tokenID))
	stored := getStoredTask(t, task.ID)
	require.Equal(t, model.TaskStatus(model.TaskStatusSuccess), stored.Status)
	require.Equal(t, 3000, stored.Quota)
	require.Equal(t, "https://example.test/result", stored.PrivateData.ResultURL)
	require.Equal(t, int64(1), countLogs(t))
}

func TestFinalizeTaskTerminalFailureIsIdempotentAfterRestart(t *testing.T) {
	truncate(t)
	const userID, taskQuota = 42, 1800
	seedUser(t, userID, 4200)
	task := makeTask(userID, 0, taskQuota, 0, BillingSourceWallet, 0)
	task.TaskID = "task_terminal_restart"
	task.Status = model.TaskStatusFailure
	require.NoError(t, model.DB.Create(task).Error)

	first, err := FinalizeTaskTerminal(context.Background(), task.ID, model.TaskTerminalMutation{
		FromStatus: model.TaskStatusFailure,
		ToStatus:   model.TaskStatusFailure,
		Reason:     "worker stopped",
	})
	require.NoError(t, err)
	require.True(t, first)
	second, err := FinalizeTaskTerminal(context.Background(), task.ID, model.TaskTerminalMutation{
		FromStatus: model.TaskStatusFailure,
		ToStatus:   model.TaskStatusFailure,
		Reason:     "retry after restart",
	})
	require.NoError(t, err)
	require.False(t, second)
	require.Equal(t, 6000, getUserQuota(t, userID))
	require.Equal(t, int64(1), countLogs(t))
	require.Zero(t, getStoredTask(t, task.ID).Quota)
}

func TestFinalizeTaskTerminalSuccessFailureRaceHasOneWinner(t *testing.T) {
	truncate(t)
	const userID, taskQuota = 43, 2400
	seedUser(t, userID, 7600)
	task := makeTask(userID, 0, taskQuota, 0, BillingSourceWallet, 0)
	task.TaskID = "task_terminal_race"
	require.NoError(t, model.DB.Create(task).Error)

	start := make(chan struct{})
	var wg sync.WaitGroup
	results := make(chan bool, 2)
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		applied, _ := FinalizeTaskTerminal(context.Background(), task.ID, model.TaskTerminalMutation{
			FromStatus:  model.TaskStatusInProgress,
			ToStatus:    model.TaskStatusSuccess,
			ActualQuota: 1200,
		})
		results <- applied
	}()
	go func() {
		defer wg.Done()
		<-start
		applied, _ := FinalizeTaskTerminal(context.Background(), task.ID, model.TaskTerminalMutation{
			FromStatus: model.TaskStatusInProgress,
			ToStatus:   model.TaskStatusFailure,
			Reason:     "upstream failed",
		})
		results <- applied
	}()
	close(start)
	wg.Wait()
	close(results)

	winners := 0
	for applied := range results {
		if applied {
			winners++
		}
	}
	require.Equal(t, 1, winners)
	stored := getStoredTask(t, task.ID)
	require.Contains(t, []model.TaskStatus{model.TaskStatusSuccess, model.TaskStatusFailure}, stored.Status)
	require.Equal(t, int64(1), countLogs(t))
}
