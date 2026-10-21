package model

import (
	"path/filepath"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestFilterChannelsByRequestPathAndModelRejectsUnsupportedAlphaSearchChannels(t *testing.T) {
	alphaConfig := &dto.AdvancedCustomConfig{Routes: []dto.AdvancedCustomRoute{
		{IncomingPath: "/v1/alpha/search", UpstreamPath: "/v1/alpha/search"},
	}}
	chatConfig := &dto.AdvancedCustomConfig{Routes: []dto.AdvancedCustomRoute{
		{IncomingPath: "/v1/chat/completions", UpstreamPath: "/v1/chat/completions"},
	}}

	channelSyncLock.Lock()
	previousChannels := channelsIDM
	previousConfigs := channel2advancedCustomConfig
	channelsIDM = map[int]*Channel{
		1: {Id: 1, Type: constant.ChannelTypeOpenAI},
		2: {Id: 2, Type: constant.ChannelTypeCodex},
		3: {Id: 3, Type: constant.ChannelTypeSub2API},
		4: {Id: 4, Type: constant.ChannelTypeAdvancedCustom},
		5: {Id: 5, Type: constant.ChannelTypeAdvancedCustom},
	}
	channel2advancedCustomConfig = map[int]*dto.AdvancedCustomConfig{
		4: alphaConfig,
		5: chatConfig,
	}
	channelSyncLock.Unlock()
	t.Cleanup(func() {
		channelSyncLock.Lock()
		channelsIDM = previousChannels
		channel2advancedCustomConfig = previousConfigs
		channelSyncLock.Unlock()
	})

	channelSyncLock.RLock()
	filtered := filterChannelsByRequestPathAndModel(
		[]int{1, 2, 3, 4, 5},
		"/v1/alpha/search",
		"gpt-alpha",
	)
	channelSyncLock.RUnlock()

	require.Equal(t, []int{2, 3, 4}, filtered)
}

func TestFilterAbilitiesByRequestPathAndModelRejectsUnsupportedAlphaSearchChannels(t *testing.T) {
	originalDB := DB
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "alpha-search-filter.db")), &gorm.Config{})
	require.NoError(t, err)
	DB = db
	t.Cleanup(func() {
		DB = originalDB
		sqlDB, dbErr := db.DB()
		require.NoError(t, dbErr)
		require.NoError(t, sqlDB.Close())
	})
	require.NoError(t, DB.AutoMigrate(&Channel{}))

	require.NoError(t, DB.Create([]*Channel{
		{Id: 11, Type: constant.ChannelTypeOpenAI, Name: "openai"},
		{Id: 12, Type: constant.ChannelTypeCodex, Name: "codex"},
		{Id: 13, Type: constant.ChannelTypeSub2API, Name: "sub2api"},
	}).Error)

	filtered := filterAbilitiesByRequestPathAndModel([]Ability{
		{ChannelId: 11, Model: "gpt-alpha"},
		{ChannelId: 12, Model: "gpt-alpha"},
		{ChannelId: 13, Model: "gpt-alpha"},
	}, "/v1/alpha/search", "gpt-alpha")

	require.Equal(t, []int{12, 13}, abilityChannelIDs(filtered))
}

func abilityChannelIDs(abilities []Ability) []int {
	ids := make([]int, 0, len(abilities))
	for _, ability := range abilities {
		ids = append(ids, ability.ChannelId)
	}
	return ids
}
