package common

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFinalizeBillingOnceRunsOneSuccessfulFinalizer(t *testing.T) {
	info := &RelayInfo{}
	var calls atomic.Int32
	var wg sync.WaitGroup
	errs := make(chan error, 32)
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- info.FinalizeBillingOnce(func() error {
				calls.Add(1)
				return nil
			})
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	require.EqualValues(t, 1, calls.Load())
}

func TestFinalizeBillingOnceCachesFailure(t *testing.T) {
	info := &RelayInfo{}
	expected := errors.New("settlement failed")
	var calls atomic.Int32

	first := info.FinalizeBillingOnce(func() error {
		calls.Add(1)
		return expected
	})
	second := info.FinalizeBillingOnce(func() error {
		calls.Add(1)
		return nil
	})

	require.ErrorIs(t, first, expected)
	require.ErrorIs(t, second, expected)
	require.EqualValues(t, 1, calls.Load())
}
