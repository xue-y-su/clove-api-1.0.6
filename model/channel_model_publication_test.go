package model

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupChannelModelPublicationTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	previousDB := DB
	previousMainType := common.MainDatabaseType()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	DB = db
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	resetChannelModelPublicationCache()
	require.NoError(t, db.AutoMigrate(&Channel{}, &Ability{}, &ChannelModelPublication{}))
	t.Cleanup(func() {
		resetChannelModelPublicationCache()
		DB = previousDB
		common.SetMainDatabaseType(previousMainType)
		sqlDB, dbErr := db.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func TestChannelModelPublicationIsIsolatedByChannel(t *testing.T) {
	db := setupChannelModelPublicationTestDB(t)

	require.NoError(t, ApproveChannelModelPublications(db, 101, []string{"shared-model"}))
	require.True(t, IsChannelModelPublicationApproved(101, "shared-model"))
	require.False(t, IsChannelModelPublicationApproved(102, "shared-model"))
	require.False(t, IsChannelModelPublicationApproved(101, "other-model"))
}

func TestSavingSub2APIChannelModelsDoesNotApprovePublication(t *testing.T) {
	db := setupChannelModelPublicationTestDB(t)
	channel := Channel{
		Id:     201,
		Type:   constant.ChannelTypeSub2API,
		Key:    "secret",
		Name:   "sub2api",
		Models: "candidate-model",
	}
	require.NoError(t, db.Create(&channel).Error)

	require.False(t, IsChannelModelPublicationApproved(channel.Id, "candidate-model"))
}

func TestRevokingChannelModelPublicationInvalidatesCache(t *testing.T) {
	db := setupChannelModelPublicationTestDB(t)
	require.NoError(t, ApproveChannelModelPublications(db, 301, []string{"candidate-model"}))
	require.True(t, IsChannelModelPublicationApproved(301, "candidate-model"))

	require.NoError(t, RevokeChannelModelPublications(db, 301, []string{"candidate-model"}))
	require.False(t, IsChannelModelPublicationApproved(301, "candidate-model"))
}

func TestChannelDeletionCleansPublicationRows(t *testing.T) {
	db := setupChannelModelPublicationTestDB(t)
	channel := Channel{Id: 401, Type: constant.ChannelTypeSub2API, Key: "secret", Name: "delete-me"}
	require.NoError(t, db.Create(&channel).Error)
	require.NoError(t, ApproveChannelModelPublications(db, channel.Id, []string{"candidate-model"}))
	require.True(t, IsChannelModelPublicationApproved(channel.Id, "candidate-model"))

	require.NoError(t, channel.Delete())
	require.False(t, IsChannelModelPublicationApproved(channel.Id, "candidate-model"))
	var count int64
	require.NoError(t, db.Model(&ChannelModelPublication{}).Where("channel_id = ?", channel.Id).Count(&count).Error)
	require.Zero(t, count)
}

func TestBatchChannelDeletionCleansPublicationRows(t *testing.T) {
	db := setupChannelModelPublicationTestDB(t)
	channels := []Channel{
		{Id: 501, Type: constant.ChannelTypeSub2API, Key: "a", Name: "first"},
		{Id: 502, Type: constant.ChannelTypeSub2API, Key: "b", Name: "second"},
	}
	require.NoError(t, db.Create(&channels).Error)
	require.NoError(t, ApproveChannelModelPublications(db, 501, []string{"same"}))
	require.NoError(t, ApproveChannelModelPublications(db, 502, []string{"same"}))

	require.NoError(t, BatchDeleteChannels([]int{501}))
	require.False(t, IsChannelModelPublicationApproved(501, "same"))
	require.True(t, IsChannelModelPublicationApproved(502, "same"))
}

func TestBatchChannelDeletionSupportsLegacyFixturesWithoutPublicationTable(t *testing.T) {
	db := setupChannelModelPublicationTestDB(t)
	channel := Channel{Id: 601, Type: constant.ChannelTypeOpenAI, Key: "legacy", Name: "legacy-fixture"}
	require.NoError(t, db.Create(&channel).Error)
	require.NoError(t, db.Migrator().DropTable(&ChannelModelPublication{}))

	require.NoError(t, BatchDeleteChannels([]int{channel.Id}))

	var count int64
	require.NoError(t, db.Model(&Channel{}).Where("id = ?", channel.Id).Count(&count).Error)
	require.Zero(t, count)
}

func TestChannelModelPublicationQueryErrorIsNotNegativeCached(t *testing.T) {
	db := setupChannelModelPublicationTestDB(t)
	require.NoError(t, db.Migrator().DropTable(&ChannelModelPublication{}))
	require.False(t, IsChannelModelPublicationApproved(701, "candidate-model"))

	require.NoError(t, db.AutoMigrate(&ChannelModelPublication{}))
	require.NoError(t, db.Create(&ChannelModelPublication{
		ChannelID:   701,
		PublicModel: "candidate-model",
		Approved:    true,
		CreatedAt:   common.GetTimestamp(),
		UpdatedAt:   common.GetTimestamp(),
	}).Error)

	require.True(t, IsChannelModelPublicationApproved(701, "candidate-model"))
}

func TestInvalidateChannelModelPublicationStateClearsPostCommitStaleEntry(t *testing.T) {
	db := setupChannelModelPublicationTestDB(t)
	tx := db.Begin()
	require.NoError(t, tx.Error)
	require.NoError(t, ApproveChannelModelPublications(tx, 702, []string{"candidate-model"}))

	key := channelModelPublicationKey{channelID: 702, model: "candidate-model"}
	channelModelPublicationCache.Lock()
	channelModelPublicationCache.entries[key] = channelModelPublicationCacheEntry{
		approved:   false,
		generation: channelModelPublicationCache.generation,
		expiresAt:  channelModelPublicationCacheNow().Add(channelModelPublicationCacheTTL),
	}
	channelModelPublicationCache.Unlock()
	require.NoError(t, tx.Commit().Error)

	InvalidateChannelModelPublicationState([]int{702}, []string{"candidate-model"})
	require.True(t, IsChannelModelPublicationApproved(702, "candidate-model"))
}

func TestChannelModelPublicationCacheRejectsQueryStartedBeforeInvalidation(t *testing.T) {
	setupChannelModelPublicationTestDB(t)
	key := channelModelPublicationKey{channelID: 703, model: "candidate-model"}

	channelModelPublicationCache.RLock()
	queryGeneration := channelModelPublicationCache.generation
	channelModelPublicationCache.RUnlock()

	invalidateChannelModelPublicationCache([]int{key.channelID}, []string{key.model})
	require.False(t, cacheChannelModelPublicationResult(key, true, queryGeneration))

	channelModelPublicationCache.RLock()
	_, cached := channelModelPublicationCache.entries[key]
	channelModelPublicationCache.RUnlock()
	require.False(t, cached)
}

func TestChannelModelPublicationCacheRefreshesRemoteChangesAfterTTL(t *testing.T) {
	db := setupChannelModelPublicationTestDB(t)
	now := time.Unix(1_000, 0)
	previousNow := channelModelPublicationCacheNow
	channelModelPublicationCacheNow = func() time.Time { return now }
	t.Cleanup(func() { channelModelPublicationCacheNow = previousNow })

	const (
		channelID = 704
		modelName = "candidate-model"
	)
	require.False(t, IsChannelModelPublicationApproved(channelID, modelName))
	require.NoError(t, db.Create(&ChannelModelPublication{
		ChannelID:   channelID,
		PublicModel: modelName,
		Approved:    true,
		CreatedAt:   common.GetTimestamp(),
		UpdatedAt:   common.GetTimestamp(),
	}).Error)

	// Another process does not share this process-local invalidation signal.
	require.False(t, IsChannelModelPublicationApproved(channelID, modelName))
	now = now.Add(channelModelPublicationCacheTTL + time.Second)
	require.True(t, IsChannelModelPublicationApproved(channelID, modelName))

	require.NoError(t, db.Where("channel_id = ? AND public_model = ?", channelID, modelName).
		Delete(&ChannelModelPublication{}).Error)
	require.True(t, IsChannelModelPublicationApproved(channelID, modelName))
	now = now.Add(channelModelPublicationCacheTTL + time.Second)
	require.False(t, IsChannelModelPublicationApproved(channelID, modelName))
}
