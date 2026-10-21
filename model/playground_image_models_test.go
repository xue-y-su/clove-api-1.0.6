package model

import (
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestGetGroupEnabledModelsForEndpointUsesRoutableEnabledAbilities(t *testing.T) {
	originalDB := DB
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:playground-image-models-%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	DB = db
	t.Cleanup(func() { DB = originalDB })
	require.NoError(t, db.AutoMigrate(&Ability{}, &Channel{}, &Model{}))

	channel := &Channel{Id: 1, Name: "images", Type: constant.ChannelTypeOpenAI, Status: common.ChannelStatusEnabled}
	require.NoError(t, db.Create(channel).Error)
	require.NoError(t, db.Create(&[]Ability{
		{Group: "default", Model: "gpt-image-1", ChannelId: channel.Id, Enabled: true},
		{Group: "default", Model: "gpt-4o", ChannelId: channel.Id, Enabled: true},
		{Group: "default", Model: "dall-e-3", ChannelId: channel.Id, Enabled: false},
		{Group: "vip", Model: "imagen-4", ChannelId: channel.Id, Enabled: true},
	}).Error)
	require.NoError(t, db.Create(&Model{ModelName: "gpt-image-1", Status: 1}).Error)
	require.NoError(t, db.Model(&Model{}).Where("model_name = ?", "gpt-image-1").Update("status", 0).Error)

	models, err := GetGroupEnabledModelsForEndpoint("default", constant.EndpointTypeImageGeneration)

	require.NoError(t, err)
	require.Empty(t, models, "disabled model metadata and non-image abilities must be excluded")

	require.NoError(t, db.Model(&Model{}).Where("model_name = ?", "gpt-image-1").Update("status", 1).Error)
	models, err = GetGroupEnabledModelsForEndpoint("default", constant.EndpointTypeImageGeneration)
	require.NoError(t, err)
	require.Equal(t, []string{"gpt-image-1"}, models)
}
