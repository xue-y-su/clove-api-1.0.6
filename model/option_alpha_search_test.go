package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/require"
)

func preserveAlphaSearchTestGlobals(t *testing.T) {
	t.Helper()
	setting := operation_setting.GetAlphaSearchSetting()
	previous := *setting
	setting.Enabled = true
	operation_setting.RebuildAlphaSearchSnapshot()
	t.Cleanup(func() {
		*setting = previous
		operation_setting.RebuildAlphaSearchSnapshot()
	})
}

func seedAlphaSearchOption(t *testing.T, dbValue string) {
	t.Helper()
	key := operation_setting.AlphaSearchSettingConfigName + ".enabled"
	require.NoError(t, DB.Create(&Option{Key: key, Value: dbValue}).Error)
	common.OptionMapRWMutex.Lock()
	common.OptionMap[key] = dbValue
	common.OptionMapRWMutex.Unlock()
}

func TestUpdateOptionRejectsInvalidAlphaSearchValueBeforePersistence(t *testing.T) {
	db := openSourceAttributionModelTestDB(t)
	require.NoError(t, db.AutoMigrate(&Option{}))
	preserveOptionTestGlobals(t)
	preserveAlphaSearchTestGlobals(t)
	seedAlphaSearchOption(t, "true")

	key := operation_setting.AlphaSearchSettingConfigName + ".enabled"
	require.Error(t, UpdateOption(key, "yes"))

	var stored Option
	require.NoError(t, db.First(&stored, "key = ?", key).Error)
	require.Equal(t, "true", stored.Value)
	common.OptionMapRWMutex.RLock()
	memoryValue := common.OptionMap[key]
	common.OptionMapRWMutex.RUnlock()
	require.Equal(t, "true", memoryValue)
	require.True(t, operation_setting.IsAlphaSearchEnabled())
}

func TestUpdateOptionsBulkRejectsInvalidAlphaSearchValueAtomically(t *testing.T) {
	db := openSourceAttributionModelTestDB(t)
	require.NoError(t, db.AutoMigrate(&Option{}))
	preserveOptionTestGlobals(t)
	preserveAlphaSearchTestGlobals(t)
	seedAlphaSearchOption(t, "true")
	require.NoError(t, db.Create(&Option{Key: "SMTPServer", Value: "old.example.test"}).Error)
	common.OptionMapRWMutex.Lock()
	common.OptionMap["SMTPServer"] = "old.example.test"
	common.OptionMapRWMutex.Unlock()
	common.SMTPServer = "old.example.test"

	key := operation_setting.AlphaSearchSettingConfigName + ".enabled"
	require.Error(t, UpdateOptionsBulk(map[string]string{
		key:          "yes",
		"SMTPServer": "new.example.test",
	}))

	var alphaOption Option
	require.NoError(t, db.First(&alphaOption, "key = ?", key).Error)
	require.Equal(t, "true", alphaOption.Value)
	var smtpOption Option
	require.NoError(t, db.First(&smtpOption, "key = ?", "SMTPServer").Error)
	require.Equal(t, "old.example.test", smtpOption.Value)
	common.OptionMapRWMutex.RLock()
	require.Equal(t, "true", common.OptionMap[key])
	require.Equal(t, "old.example.test", common.OptionMap["SMTPServer"])
	common.OptionMapRWMutex.RUnlock()
	require.Equal(t, "old.example.test", common.SMTPServer)
	require.True(t, operation_setting.IsAlphaSearchEnabled())
}
