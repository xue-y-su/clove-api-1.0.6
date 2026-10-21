package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChannelBalanceUpdateCoordinatorRunsSingleFlight(t *testing.T) {
	coordinator := &channelBalanceUpdateCoordinator{}
	started := make(chan struct{})
	release := make(chan struct{})
	completed := make(chan error, 1)

	first := coordinator.Start(func() error {
		close(started)
		<-release
		return nil
	}, func(err error) {
		completed <- err
	})
	require.True(t, first)
	<-started

	second := coordinator.Start(func() error {
		return nil
	}, nil)
	assert.False(t, second)

	close(release)
	require.NoError(t, <-completed)

	thirdCompleted := make(chan error, 1)
	third := coordinator.Start(func() error { return nil }, func(err error) {
		thirdCompleted <- err
	})
	require.True(t, third)
	require.NoError(t, <-thirdCompleted)
}
