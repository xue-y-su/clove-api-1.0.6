package service

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/announcement_setting"
	"github.com/QuantumNous/new-api/setting/console_setting"
)

var ErrAnnouncementSignatureUnknown = errors.New("announcement signature is unknown")

type Announcement struct {
	ID          int    `json:"id"`
	Title       string `json:"title,omitempty"`
	Content     string `json:"content"`
	Extra       string `json:"extra"`
	ImageURL    string `json:"imageUrl"`
	PublishDate string `json:"publishDate"`
	Type        string `json:"type"`
	Popup       bool   `json:"popup"`
	Published   *bool  `json:"published,omitempty"`
}

type AnnouncementPopupState struct {
	Enabled      bool          `json:"enabled"`
	Announcement *Announcement `json:"announcement"`
	Signature    string        `json:"signature"`
	Acknowledged bool          `json:"acknowledged"`
}

type announcementSignaturePayload struct {
	ID          int    `json:"id"`
	Title       string `json:"title,omitempty"`
	Content     string `json:"content"`
	Extra       string `json:"extra"`
	Image       string `json:"image"`
	PublishDate string `json:"publishDate"`
	Type        string `json:"type"`
	Popup       bool   `json:"popup"`
	Published   *bool  `json:"published,omitempty"`
}

func AnnouncementSignature(announcement Announcement) string {
	if announcement.Type == "" {
		announcement.Type = "default"
	}
	payload, err := common.Marshal(announcementSignaturePayload{
		ID:          announcement.ID,
		Title:       strings.TrimSpace(announcement.Title),
		Content:     announcement.Content,
		Extra:       announcement.Extra,
		Image:       announcement.ImageURL,
		PublishDate: announcement.PublishDate,
		Type:        announcement.Type,
		Popup:       announcement.Popup,
		Published:   canonicalAnnouncementPublishingOverride(announcement.Published),
	})
	if err != nil {
		return ""
	}
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:])
}

func GetAnnouncementPopupState(userID int, now time.Time) (AnnouncementPopupState, error) {
	settings := announcement_setting.GetSettings()
	state := AnnouncementPopupState{
		Enabled: settings.Enabled,
	}
	if !settings.Enabled || userID <= 0 {
		return state, nil
	}

	announcements, err := currentAnnouncements()
	if err != nil {
		return state, err
	}
	announcement := selectLatestPublishedAnnouncement(announcements, now)
	if announcement == nil {
		return state, nil
	}
	state.Announcement = announcement
	state.Signature = AnnouncementSignature(*announcement)
	state.Acknowledged, err = model.HasAnnouncementAcknowledgement(userID, state.Signature)
	return state, err
}

func AcknowledgeAnnouncement(userID int, signature string, acknowledgedAt int64, now time.Time) (bool, error) {
	if userID <= 0 {
		return false, model.ErrInvalidAnnouncementAcknowledgement
	}
	signature = strings.TrimSpace(signature)
	announcements, err := currentAnnouncements()
	if err != nil {
		return false, err
	}
	if now.IsZero() {
		now = time.Now()
	}
	for index := range announcements {
		if !isAnnouncementPublished(announcements[index]) {
			continue
		}
		publishedAt, parseErr := time.Parse(time.RFC3339, strings.TrimSpace(announcements[index].PublishDate))
		if parseErr != nil || publishedAt.After(now) {
			continue
		}
		if signature == AnnouncementSignature(announcements[index]) {
			return model.RecordAnnouncementAcknowledgement(userID, signature, acknowledgedAt)
		}
	}
	return false, ErrAnnouncementSignatureUnknown
}

func currentAnnouncements() ([]Announcement, error) {
	raw := strings.TrimSpace(console_setting.GetConsoleSetting().Announcements)
	if raw == "" {
		return []Announcement{}, nil
	}
	normalized, err := console_setting.NormalizeAnnouncementsJSON(raw)
	if err != nil {
		return nil, err
	}
	var announcements []Announcement
	if err := common.UnmarshalJsonStr(normalized, &announcements); err != nil {
		return nil, err
	}
	for index := range announcements {
		if announcements[index].Type == "" {
			announcements[index].Type = "default"
		}
	}
	return announcements, nil
}

func selectLatestPublishedAnnouncement(announcements []Announcement, now time.Time) *Announcement {
	if now.IsZero() {
		now = time.Now()
	}
	var selected *Announcement
	var selectedTime time.Time
	for index := range announcements {
		if !announcements[index].Popup || !isAnnouncementPublished(announcements[index]) {
			continue
		}
		publishedAt, err := time.Parse(time.RFC3339, strings.TrimSpace(announcements[index].PublishDate))
		if err != nil || publishedAt.After(now) {
			continue
		}
		if selected == nil || publishedAt.After(selectedTime) ||
			(publishedAt.Equal(selectedTime) && announcements[index].ID > selected.ID) {
			candidate := announcements[index]
			selected = &candidate
			selectedTime = publishedAt
		}
	}
	return selected
}

func isAnnouncementPublished(announcement Announcement) bool {
	return announcement.Published == nil || *announcement.Published
}

func canonicalAnnouncementPublishingOverride(published *bool) *bool {
	if published == nil || *published {
		return nil
	}
	value := false
	return &value
}
