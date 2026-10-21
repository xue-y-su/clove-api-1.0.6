package operation_setting

import (
	"runtime"
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/setting/config"
	"github.com/stretchr/testify/require"
)

func TestAlphaSearchSettingDefaultsEnabled(t *testing.T) {
	require.True(t, IsAlphaSearchEnabled())
}

func TestAlphaSearchSettingGlobalConfigRoundTrip(t *testing.T) {
	previous := *GetAlphaSearchSetting()
	t.Cleanup(func() {
		*GetAlphaSearchSetting() = previous
		RebuildAlphaSearchSnapshot()
	})

	require.Same(t,
		GetAlphaSearchSetting(),
		config.GlobalConfig.Get(AlphaSearchSettingConfigName),
	)
	require.NoError(t, config.UpdateConfigFromMap(GetAlphaSearchSetting(), map[string]string{
		"enabled": "false",
	}))
	RebuildAlphaSearchSnapshot()
	require.False(t, IsAlphaSearchEnabled())
}

func TestValidateAlphaSearchOptionAcceptsOnlyStrictBooleans(t *testing.T) {
	for _, value := range []string{"true", "false"} {
		require.NoError(t, ValidateAlphaSearchOption("enabled", value), value)
	}
	for _, value := range []string{"yes", "1", "TRUE", " true ", ""} {
		require.Error(t, ValidateAlphaSearchOption("enabled", value), value)
	}
	require.Error(t, ValidateAlphaSearchOption("unknown", "true"))
}

func TestAlphaSearchSnapshotRefreshesExplicitly(t *testing.T) {
	setting := GetAlphaSearchSetting()
	previous := *setting
	t.Cleanup(func() {
		*setting = previous
		RebuildAlphaSearchSnapshot()
	})

	setting.Enabled = true
	RebuildAlphaSearchSnapshot()
	require.True(t, CurrentAlphaSearchSnapshot().Enabled)

	setting.Enabled = false
	require.True(t, CurrentAlphaSearchSnapshot().Enabled)
	RebuildAlphaSearchSnapshot()
	require.False(t, CurrentAlphaSearchSnapshot().Enabled)
}

func TestAlphaSearchSnapshotConcurrentReaders(t *testing.T) {
	setting := GetAlphaSearchSetting()
	previous := *setting
	t.Cleanup(func() {
		*setting = previous
		RebuildAlphaSearchSnapshot()
	})

	setting.Enabled = true
	RebuildAlphaSearchSnapshot()

	stop := make(chan struct{})
	var readers sync.WaitGroup
	for range 8 {
		readers.Add(1)
		go func() {
			defer readers.Done()
			for {
				select {
				case <-stop:
					return
				default:
					_ = IsAlphaSearchEnabled()
					runtime.Gosched()
				}
			}
		}()
	}

	for index := range 1000 {
		setting.Enabled = index%2 == 0
		RebuildAlphaSearchSnapshot()
	}
	close(stop)
	readers.Wait()
}
