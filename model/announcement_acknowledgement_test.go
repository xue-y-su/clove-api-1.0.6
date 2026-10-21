package model

import (
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupAnnouncementAcknowledgementTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	previousDB := DB
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	DB = db
	require.NoError(t, db.AutoMigrate(&AnnouncementAcknowledgement{}))
	t.Cleanup(func() {
		DB = previousDB
		sqlDB, sqlErr := db.DB()
		if sqlErr == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func TestRecordAnnouncementAcknowledgementIsIdempotentPerUser(t *testing.T) {
	db := setupAnnouncementAcknowledgementTestDB(t)
	signature := strings.Repeat("a", 64)

	created, err := RecordAnnouncementAcknowledgement(11, signature, 100)
	require.NoError(t, err)
	assert.True(t, created)

	created, err = RecordAnnouncementAcknowledgement(11, signature, 200)
	require.NoError(t, err)
	assert.False(t, created)

	created, err = RecordAnnouncementAcknowledgement(12, signature, 300)
	require.NoError(t, err)
	assert.True(t, created)

	var records []AnnouncementAcknowledgement
	require.NoError(t, db.Order("user_id ASC").Find(&records).Error)
	require.Len(t, records, 2)
	assert.Equal(t, int64(100), records[0].AcknowledgedAt)
	assert.Equal(t, 11, records[0].UserID)
	assert.Equal(t, 12, records[1].UserID)
}

func TestHasAnnouncementAcknowledgementIsUserScoped(t *testing.T) {
	setupAnnouncementAcknowledgementTestDB(t)
	signature := strings.Repeat("b", 64)
	_, err := RecordAnnouncementAcknowledgement(21, signature, 100)
	require.NoError(t, err)

	acknowledged, err := HasAnnouncementAcknowledgement(21, signature)
	require.NoError(t, err)
	assert.True(t, acknowledged)

	acknowledged, err = HasAnnouncementAcknowledgement(22, signature)
	require.NoError(t, err)
	assert.False(t, acknowledged)
}
