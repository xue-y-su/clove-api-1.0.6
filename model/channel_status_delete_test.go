package model

import (
	"fmt"
	"slices"
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestDeleteChannelByStatusPreservesChannelReenabledAfterSelection(t *testing.T) {
	db := setupChannelModelPublicationTestDB(t)
	protectedID, deletedID := seedChannelStatusDeleteFixtures(
		t,
		db,
		common.ChannelStatusManuallyDisabled,
		common.ChannelStatusManuallyDisabled,
	)
	reenableChannelAfterStatusSelection(t, db, protectedID)

	rows, err := DeleteChannelByStatus(common.ChannelStatusManuallyDisabled)
	require.NoError(t, err)
	require.Equal(t, int64(1), rows)
	requireChannelStatusDeleteResult(t, db, protectedID, deletedID)
}

func TestDeleteDisabledChannelPreservesChannelReenabledAfterSelection(t *testing.T) {
	db := setupChannelModelPublicationTestDB(t)
	protectedID, deletedID := seedChannelStatusDeleteFixtures(
		t,
		db,
		common.ChannelStatusAutoDisabled,
		common.ChannelStatusManuallyDisabled,
	)
	reenableChannelAfterStatusSelection(t, db, protectedID)

	rows, err := DeleteDisabledChannel()
	require.NoError(t, err)
	require.Equal(t, int64(1), rows)
	requireChannelStatusDeleteResult(t, db, protectedID, deletedID)
}

func seedChannelStatusDeleteFixtures(t *testing.T, db *gorm.DB, protectedStatus, deletedStatus int) (int, int) {
	t.Helper()
	const (
		protectedID = 801
		deletedID   = 802
		modelName   = "status-delete-model"
	)
	channels := []Channel{
		{Id: protectedID, Type: constant.ChannelTypeSub2API, Key: "protected", Name: "protected", Status: protectedStatus},
		{Id: deletedID, Type: constant.ChannelTypeSub2API, Key: "deleted", Name: "deleted", Status: deletedStatus},
	}
	require.NoError(t, db.Create(&channels).Error)
	abilities := []Ability{
		{Group: "default", Model: modelName, ChannelId: protectedID, Enabled: false},
		{Group: "default", Model: modelName, ChannelId: deletedID, Enabled: false},
	}
	require.NoError(t, db.Create(&abilities).Error)
	require.NoError(t, ApproveChannelModelPublications(db, protectedID, []string{modelName}))
	require.NoError(t, ApproveChannelModelPublications(db, deletedID, []string{modelName}))
	return protectedID, deletedID
}

func reenableChannelAfterStatusSelection(t *testing.T, db *gorm.DB, channelID int) {
	t.Helper()
	callbackName := fmt.Sprintf("test:reenable-channel-after-status-selection:%s", t.Name())
	var once sync.Once
	require.NoError(t, db.Callback().Query().After("gorm:query").Register(callbackName, func(tx *gorm.DB) {
		ids, ok := tx.Statement.Dest.(*[]int)
		if tx.Statement.Table != "channels" || !ok || !slices.Contains(*ids, channelID) {
			return
		}
		once.Do(func() {
			err := tx.Session(&gorm.Session{NewDB: true}).
				Model(&Channel{}).
				Where("id = ?", channelID).
				UpdateColumn("status", common.ChannelStatusEnabled).Error
			tx.AddError(err)
		})
	}))
	t.Cleanup(func() {
		_ = db.Callback().Query().Remove(callbackName)
	})
}

func requireChannelStatusDeleteResult(t *testing.T, db *gorm.DB, protectedID, deletedID int) {
	t.Helper()
	const modelName = "status-delete-model"

	var protected Channel
	require.NoError(t, db.First(&protected, protectedID).Error)
	require.Equal(t, common.ChannelStatusEnabled, protected.Status)
	require.True(t, IsChannelModelPublicationApproved(protectedID, modelName))

	var protectedAbilityCount int64
	require.NoError(t, db.Model(&Ability{}).Where("channel_id = ?", protectedID).Count(&protectedAbilityCount).Error)
	require.Equal(t, int64(1), protectedAbilityCount)

	var deletedChannelCount int64
	require.NoError(t, db.Model(&Channel{}).Where("id = ?", deletedID).Count(&deletedChannelCount).Error)
	require.Zero(t, deletedChannelCount)
	require.False(t, IsChannelModelPublicationApproved(deletedID, modelName))

	var deletedAbilityCount int64
	require.NoError(t, db.Model(&Ability{}).Where("channel_id = ?", deletedID).Count(&deletedAbilityCount).Error)
	require.Zero(t, deletedAbilityCount)
}
