package operation_setting

import (
	"fmt"
	"sync/atomic"

	"github.com/QuantumNous/new-api/setting/config"
)

const AlphaSearchSettingConfigName = "alpha_search_setting"

type AlphaSearchSetting struct {
	Enabled bool `json:"enabled"`
}

type AlphaSearchSnapshot struct {
	Enabled bool
}

var alphaSearchSetting = AlphaSearchSetting{Enabled: true}
var alphaSearchSnapshot atomic.Pointer[AlphaSearchSnapshot]

func init() {
	config.GlobalConfig.Register(AlphaSearchSettingConfigName, &alphaSearchSetting)
	RebuildAlphaSearchSnapshot()
}

func GetAlphaSearchSetting() *AlphaSearchSetting {
	return &alphaSearchSetting
}

func CurrentAlphaSearchSnapshot() AlphaSearchSnapshot {
	if snapshot := alphaSearchSnapshot.Load(); snapshot != nil {
		return *snapshot
	}
	RebuildAlphaSearchSnapshot()
	return *alphaSearchSnapshot.Load()
}

func RebuildAlphaSearchSnapshot() {
	setting := alphaSearchSetting
	alphaSearchSnapshot.Store(&AlphaSearchSnapshot{Enabled: setting.Enabled})
}

func IsAlphaSearchEnabled() bool {
	return CurrentAlphaSearchSnapshot().Enabled
}

func ValidateAlphaSearchOption(key, value string) error {
	switch key {
	case "enabled":
		if value != "true" && value != "false" {
			return fmt.Errorf("enabled must be true or false")
		}
	default:
		return fmt.Errorf("unknown alpha search option %q", key)
	}
	return nil
}
