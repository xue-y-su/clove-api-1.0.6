package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func preserveOptionTestGlobals(t *testing.T) {
	t.Helper()

	common.OptionMapRWMutex.Lock()
	previousMap := common.OptionMap
	common.OptionMap = map[string]string{}
	common.OptionMapRWMutex.Unlock()
	previousSMTPServer := common.SMTPServer
	previousSMTPAccount := common.SMTPAccount

	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = previousMap
		common.OptionMapRWMutex.Unlock()
		common.SMTPServer = previousSMTPServer
		common.SMTPAccount = previousSMTPAccount
	})
}

func TestUpdateOptionRepairsLegacyNilSMTPValue(t *testing.T) {
	db := openSourceAttributionModelTestDB(t)
	require.NoError(t, db.AutoMigrate(&Option{}))
	preserveOptionTestGlobals(t)

	require.NoError(t, UpdateOption("SMTPServer", "<nil>"))

	var stored Option
	require.NoError(t, db.First(&stored, "key = ?", "SMTPServer").Error)
	require.Empty(t, stored.Value)
	require.Empty(t, common.SMTPServer)
}

func TestLoadOptionsRepairsPersistedLegacyNilSMTPValue(t *testing.T) {
	db := openSourceAttributionModelTestDB(t)
	require.NoError(t, db.AutoMigrate(&Option{}))
	preserveOptionTestGlobals(t)
	require.NoError(t, db.Create(&Option{Key: "SMTPAccount", Value: "<nil>"}).Error)

	loadOptionsFromDatabase()

	var stored Option
	require.NoError(t, db.First(&stored, "key = ?", "SMTPAccount").Error)
	require.Empty(t, stored.Value)
	require.Empty(t, common.SMTPAccount)
}
