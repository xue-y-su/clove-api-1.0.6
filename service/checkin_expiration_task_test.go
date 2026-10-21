package service

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestCheckinExpirationTaskEnabledOnlyOnMaster(t *testing.T) {
	previous := common.IsMasterNode
	t.Cleanup(func() { common.IsMasterNode = previous })

	common.IsMasterNode = false
	require.False(t, checkinExpirationTaskEnabled())

	common.IsMasterNode = true
	require.True(t, checkinExpirationTaskEnabled())
}

func TestCheckinExpirationRunnerInvokesSettlementOnce(t *testing.T) {
	previous := settleDueCheckinRewards
	t.Cleanup(func() {
		settleDueCheckinRewards = previous
		checkinExpirationRunning.Store(false)
	})

	calls := 0
	settleDueCheckinRewards = func(limit int) (int, error) {
		calls++
		require.Equal(t, checkinExpirationBatchSize, limit)
		return 0, nil
	}

	runCheckinExpirationOnce()

	require.Equal(t, 1, calls)
}

func TestCheckinExpirationRunnerSkipsOverlappingRun(t *testing.T) {
	previous := settleDueCheckinRewards
	t.Cleanup(func() {
		settleDueCheckinRewards = previous
		checkinExpirationRunning.Store(false)
	})

	calls := 0
	settleDueCheckinRewards = func(limit int) (int, error) {
		calls++
		return 0, nil
	}
	checkinExpirationRunning.Store(true)

	runCheckinExpirationOnce()

	require.Zero(t, calls)
}
