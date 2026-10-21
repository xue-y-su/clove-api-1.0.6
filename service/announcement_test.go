package service

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/announcement_setting"
	"github.com/QuantumNous/new-api/setting/console_setting"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupAnnouncementServiceTest(t *testing.T) {
	t.Helper()
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	previousDB := model.DB
	previousConsole := *console_setting.GetConsoleSetting()
	common.OptionMapRWMutex.Lock()
	previousOptions := common.OptionMap
	common.OptionMap = map[string]string{
		announcement_setting.PopupEnabledKey: "true",
	}
	common.OptionMapRWMutex.Unlock()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	require.NoError(t, db.AutoMigrate(&model.AnnouncementAcknowledgement{}))
	console_setting.GetConsoleSetting().AnnouncementsEnabled = true

	t.Cleanup(func() {
		model.DB = previousDB
		*console_setting.GetConsoleSetting() = previousConsole
		common.OptionMapRWMutex.Lock()
		common.OptionMap = previousOptions
		common.OptionMapRWMutex.Unlock()
		sqlDB, sqlErr := db.DB()
		if sqlErr == nil {
			_ = sqlDB.Close()
		}
	})
}

func setAnnouncementFixture(t *testing.T, announcements []Announcement) {
	t.Helper()
	payload, err := common.Marshal(announcements)
	require.NoError(t, err)
	console_setting.GetConsoleSetting().Announcements = string(payload)
}

func TestAnnouncementSignatureCoversEveryCanonicalField(t *testing.T) {
	base := Announcement{
		ID: 7, Content: "Important", Extra: "Read this", ImageURL: "/api/announcement-media/a.png",
		PublishDate: "2026-07-20T00:00:00Z", Type: "warning", Popup: true,
	}
	baseSignature := AnnouncementSignature(base)
	require.Len(t, baseSignature, 64)

	variants := []Announcement{
		{ID: 8, Content: base.Content, Extra: base.Extra, ImageURL: base.ImageURL, PublishDate: base.PublishDate, Type: base.Type, Popup: base.Popup},
		{ID: base.ID, Content: "Changed", Extra: base.Extra, ImageURL: base.ImageURL, PublishDate: base.PublishDate, Type: base.Type, Popup: base.Popup},
		{ID: base.ID, Content: base.Content, Extra: "Changed", ImageURL: base.ImageURL, PublishDate: base.PublishDate, Type: base.Type, Popup: base.Popup},
		{ID: base.ID, Content: base.Content, Extra: base.Extra, ImageURL: "/api/announcement-media/b.png", PublishDate: base.PublishDate, Type: base.Type, Popup: base.Popup},
		{ID: base.ID, Content: base.Content, Extra: base.Extra, ImageURL: base.ImageURL, PublishDate: "2026-07-21T00:00:00Z", Type: base.Type, Popup: base.Popup},
		{ID: base.ID, Content: base.Content, Extra: base.Extra, ImageURL: base.ImageURL, PublishDate: base.PublishDate, Type: "success", Popup: base.Popup},
		{ID: base.ID, Content: base.Content, Extra: base.Extra, ImageURL: base.ImageURL, PublishDate: base.PublishDate, Type: base.Type, Popup: false},
	}
	for _, variant := range variants {
		assert.NotEqual(t, baseSignature, AnnouncementSignature(variant))
	}

	var legacy Announcement
	require.NoError(t, common.UnmarshalJsonStr(`{
		"id":7,"content":"Important","extra":"Read this",
		"imageUrl":"/api/announcement-media/a.png","publishDate":"2026-07-20T00:00:00Z",
		"type":"warning","popup":true,"content_format":"html"
	}`, &legacy))
	assert.Equal(t, baseSignature, AnnouncementSignature(legacy))
}

func TestAnnouncementSignatureCoversTitleAndPublishingState(t *testing.T) {
	published := true
	unpublished := false
	legacy := Announcement{
		ID: 7, Content: "Important", PublishDate: "2026-07-20T00:00:00Z", Type: "warning", Popup: true,
	}
	explicitlyPublished := legacy
	explicitlyPublished.Published = &published
	require.Equal(t, AnnouncementSignature(legacy), AnnouncementSignature(explicitlyPublished))

	titled := legacy
	titled.Title = "Release"
	require.NotEqual(t, AnnouncementSignature(legacy), AnnouncementSignature(titled))

	hidden := legacy
	hidden.Published = &unpublished
	require.NotEqual(t, AnnouncementSignature(legacy), AnnouncementSignature(hidden))
}

func TestLatestPopupSkipsExplicitlyUnpublishedAnnouncements(t *testing.T) {
	published := true
	unpublished := false
	announcements := []Announcement{
		{ID: 1, Content: "Legacy", PublishDate: "2026-07-20T00:00:00Z", Popup: true},
		{ID: 2, Content: "Visible", PublishDate: "2026-07-21T00:00:00Z", Popup: true, Published: &published},
		{ID: 3, Content: "Hidden", PublishDate: "2026-07-22T00:00:00Z", Popup: true, Published: &unpublished},
	}

	selected := selectLatestPublishedAnnouncement(
		announcements,
		time.Date(2026, 7, 23, 0, 0, 0, 0, time.UTC),
	)
	require.NotNil(t, selected)
	require.Equal(t, 2, selected.ID)
}

func TestAnnouncementPopupStateSelectsLatestPublishedAndPersistsByUser(t *testing.T) {
	setupAnnouncementServiceTest(t)
	setAnnouncementFixture(t, []Announcement{
		{ID: 1, Content: "Old", PublishDate: "2026-07-18T00:00:00Z", Type: "default", Popup: true},
		{ID: 2, Content: "Latest", PublishDate: "2026-07-20T00:00:00Z", Type: "warning", Popup: true},
		{ID: 3, Content: "Future", PublishDate: "2026-07-22T00:00:00Z", Type: "default", Popup: true},
	})
	now := time.Date(2026, 7, 21, 0, 0, 0, 0, time.UTC)

	state, err := GetAnnouncementPopupState(31, now)
	require.NoError(t, err)
	require.NotNil(t, state.Announcement)
	assert.Equal(t, 2, state.Announcement.ID)
	assert.False(t, state.Acknowledged)

	created, err := AcknowledgeAnnouncement(31, state.Signature, 500, now)
	require.NoError(t, err)
	assert.True(t, created)
	created, err = AcknowledgeAnnouncement(31, state.Signature, 600, now)
	require.NoError(t, err)
	assert.False(t, created)

	state, err = GetAnnouncementPopupState(31, now)
	require.NoError(t, err)
	assert.True(t, state.Acknowledged)

	otherUserState, err := GetAnnouncementPopupState(32, now)
	require.NoError(t, err)
	assert.False(t, otherUserState.Acknowledged)

	setAnnouncementFixture(t, []Announcement{{
		ID: 2, Content: "Latest, revised", PublishDate: "2026-07-20T00:00:00Z", Type: "warning", Popup: true,
	}})
	revisedState, err := GetAnnouncementPopupState(31, now)
	require.NoError(t, err)
	assert.NotEqual(t, state.Signature, revisedState.Signature)
	assert.False(t, revisedState.Acknowledged)
}

func TestAnnouncementPopupStateSelectsOnlyExplicitPopupAnnouncements(t *testing.T) {
	setupAnnouncementServiceTest(t)
	console_setting.GetConsoleSetting().Announcements = `[
		{"id":1,"content":"Popup announcement","publishDate":"2026-07-20T00:00:00Z","type":"default","popup":true},
		{"id":2,"content":"Latest non-popup announcement","publishDate":"2026-07-21T00:00:00Z","type":"warning","popup":false},
		{"id":3,"content":"Legacy announcement","publishDate":"2026-07-22T00:00:00Z","type":"default"}
	]`
	now := time.Date(2026, 7, 23, 0, 0, 0, 0, time.UTC)

	state, err := GetAnnouncementPopupState(31, now)
	require.NoError(t, err)
	require.NotNil(t, state.Announcement)
	assert.Equal(t, 1, state.Announcement.ID)
}

func TestAcknowledgeAnnouncementRejectsUnknownSignature(t *testing.T) {
	setupAnnouncementServiceTest(t)
	setAnnouncementFixture(t, []Announcement{{
		ID: 1, Content: "Current", PublishDate: "2026-07-20T00:00:00Z", Type: "default",
	}})
	now := time.Date(2026, 7, 21, 0, 0, 0, 0, time.UTC)

	_, err := AcknowledgeAnnouncement(41, strings.Repeat("f", 64), 100, now)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrAnnouncementSignatureUnknown))
}

func TestAcknowledgeAnnouncementValidatesAgainstAllCurrentlyPublishedAnnouncements(t *testing.T) {
	setupAnnouncementServiceTest(t)
	unpublished := false
	announcements := []Announcement{
		{ID: 1, Content: "Previously displayed", PublishDate: "2026-07-19T00:00:00Z", Type: "default"},
		{ID: 2, Content: "Current latest", PublishDate: "2026-07-20T00:00:00Z", Type: "warning"},
		{ID: 3, Content: "Future", PublishDate: "2026-07-22T00:00:00Z", Type: "default"},
		{ID: 4, Content: "Unpublished", PublishDate: "2026-07-20T00:00:00Z", Type: "default", Published: &unpublished},
	}
	setAnnouncementFixture(t, announcements)
	now := time.Date(2026, 7, 21, 0, 0, 0, 0, time.UTC)

	created, err := AcknowledgeAnnouncement(42, AnnouncementSignature(announcements[0]), 100, now)
	require.NoError(t, err)
	assert.True(t, created)

	_, err = AcknowledgeAnnouncement(42, AnnouncementSignature(announcements[2]), 200, now)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrAnnouncementSignatureUnknown)

	_, err = AcknowledgeAnnouncement(42, AnnouncementSignature(announcements[3]), 300, now)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrAnnouncementSignatureUnknown)
}

func TestAnnouncementPopupStateHonorsDataAndPopupSettings(t *testing.T) {
	setupAnnouncementServiceTest(t)
	setAnnouncementFixture(t, []Announcement{{
		ID: 1, Content: "Current", PublishDate: "2026-07-20T00:00:00Z", Type: "default", Popup: true,
	}})
	now := time.Date(2026, 7, 21, 0, 0, 0, 0, time.UTC)

	console_setting.GetConsoleSetting().AnnouncementsEnabled = false
	state, err := GetAnnouncementPopupState(0, now)
	require.NoError(t, err)
	assert.False(t, state.Enabled)
	assert.Nil(t, state.Announcement)

	console_setting.GetConsoleSetting().AnnouncementsEnabled = true
	common.OptionMapRWMutex.Lock()
	common.OptionMap[announcement_setting.PopupEnabledKey] = "false"
	common.OptionMapRWMutex.Unlock()
	state, err = GetAnnouncementPopupState(0, now)
	require.NoError(t, err)
	assert.True(t, state.Enabled)
	assert.Nil(t, state.Announcement)
}

func TestAnnouncementPopupStateIgnoresLegacyGlobalPopupToggle(t *testing.T) {
	setupAnnouncementServiceTest(t)
	setAnnouncementFixture(t, []Announcement{{
		ID: 1, Content: "Current", PublishDate: "2026-07-20T00:00:00Z", Type: "default", Popup: true,
	}})
	common.OptionMapRWMutex.Lock()
	common.OptionMap[announcement_setting.PopupEnabledKey] = "false"
	common.OptionMapRWMutex.Unlock()

	state, err := GetAnnouncementPopupState(1, time.Date(2026, 7, 21, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	require.NotNil(t, state.Announcement)
	assert.Equal(t, 1, state.Announcement.ID)
	payload, err := common.Marshal(state)
	require.NoError(t, err)
	assert.NotContains(t, string(payload), "popup_enabled")
}
