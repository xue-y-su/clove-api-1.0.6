package model

import (
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ChannelModelPublication struct {
	ChannelID   int    `json:"channel_id" gorm:"primaryKey;autoIncrement:false"`
	PublicModel string `json:"public_model" gorm:"primaryKey;type:varchar(255);autoIncrement:false"`
	Approved    bool   `json:"approved" gorm:"not null;default:false"`
	CreatedAt   int64  `json:"created_at" gorm:"bigint;not null"`
	UpdatedAt   int64  `json:"updated_at" gorm:"bigint;not null"`
}

type channelModelPublicationKey struct {
	channelID int
	model     string
}

type channelModelPublicationCacheEntry struct {
	approved   bool
	generation uint64
	expiresAt  time.Time
}

const channelModelPublicationCacheTTL = 30 * time.Second

var channelModelPublicationCacheNow = time.Now

var channelModelPublicationCache = struct {
	sync.RWMutex
	entries    map[channelModelPublicationKey]channelModelPublicationCacheEntry
	generation uint64
}{entries: make(map[channelModelPublicationKey]channelModelPublicationCacheEntry)}

func normalizePublicationModels(models []string) []string {
	seen := make(map[string]struct{}, len(models))
	result := make([]string, 0, len(models))
	for _, modelName := range models {
		modelName = strings.TrimSpace(modelName)
		if modelName == "" {
			continue
		}
		if _, exists := seen[modelName]; exists {
			continue
		}
		seen[modelName] = struct{}{}
		result = append(result, modelName)
	}
	return result
}

func resetChannelModelPublicationCache() {
	channelModelPublicationCache.Lock()
	channelModelPublicationCache.entries = make(map[channelModelPublicationKey]channelModelPublicationCacheEntry)
	channelModelPublicationCache.generation++
	channelModelPublicationCache.Unlock()
}

func invalidateChannelModelPublicationCache(channelIDs []int, models []string) {
	idSet := make(map[int]struct{}, len(channelIDs))
	for _, channelID := range channelIDs {
		idSet[channelID] = struct{}{}
	}
	modelSet := make(map[string]struct{}, len(models))
	for _, modelName := range normalizePublicationModels(models) {
		modelSet[modelName] = struct{}{}
	}

	channelModelPublicationCache.Lock()
	channelModelPublicationCache.generation++
	for key := range channelModelPublicationCache.entries {
		if _, matchesChannel := idSet[key.channelID]; !matchesChannel {
			continue
		}
		if len(modelSet) > 0 {
			if _, matchesModel := modelSet[key.model]; !matchesModel {
				continue
			}
		}
		delete(channelModelPublicationCache.entries, key)
	}
	channelModelPublicationCache.Unlock()
}

func cacheChannelModelPublicationResult(key channelModelPublicationKey, approved bool, queryGeneration uint64) bool {
	channelModelPublicationCache.Lock()
	defer channelModelPublicationCache.Unlock()
	if channelModelPublicationCache.generation != queryGeneration {
		return false
	}
	channelModelPublicationCache.entries[key] = channelModelPublicationCacheEntry{
		approved:   approved,
		generation: queryGeneration,
		expiresAt:  channelModelPublicationCacheNow().Add(channelModelPublicationCacheTTL),
	}
	return true
}

// InvalidateChannelModelPublicationState must be called after a transaction
// that changes publication rows commits, so readers cannot retain a value read
// while those rows were still uncommitted.
func InvalidateChannelModelPublicationState(channelIDs []int, models []string) {
	invalidateChannelModelPublicationCache(channelIDs, models)
	InvalidateCatalogPricing()
}

func IsChannelModelPublicationApproved(channelID int, publicModel string) bool {
	publicModel = strings.TrimSpace(publicModel)
	if DB == nil || channelID <= 0 || publicModel == "" {
		return false
	}
	key := channelModelPublicationKey{channelID: channelID, model: publicModel}
	channelModelPublicationCache.RLock()
	entry, cached := channelModelPublicationCache.entries[key]
	queryGeneration := channelModelPublicationCache.generation
	channelModelPublicationCache.RUnlock()
	if cached && entry.generation == queryGeneration && channelModelPublicationCacheNow().Before(entry.expiresAt) {
		return entry.approved
	}

	var publication ChannelModelPublication
	result := DB.Where("channel_id = ? AND public_model = ?", channelID, publicModel).Limit(1).Find(&publication)
	if result.Error != nil {
		return false
	}
	approved := result.Error == nil && result.RowsAffected == 1 && publication.Approved
	cacheChannelModelPublicationResult(key, approved, queryGeneration)
	return approved
}

func ApproveChannelModelPublications(tx *gorm.DB, channelID int, models []string) error {
	models = normalizePublicationModels(models)
	if len(models) == 0 {
		return nil
	}
	if tx == nil {
		tx = DB
	}
	now := common.GetTimestamp()
	publications := make([]ChannelModelPublication, 0, len(models))
	for _, modelName := range models {
		publications = append(publications, ChannelModelPublication{
			ChannelID:   channelID,
			PublicModel: modelName,
			Approved:    true,
			CreatedAt:   now,
			UpdatedAt:   now,
		})
	}
	err := tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "channel_id"}, {Name: "public_model"}},
		DoUpdates: clause.Assignments(map[string]any{
			"approved":   true,
			"updated_at": now,
		}),
	}).Create(&publications).Error
	if err != nil {
		return err
	}
	InvalidateChannelModelPublicationState([]int{channelID}, models)
	return nil
}

func RevokeChannelModelPublications(tx *gorm.DB, channelID int, models []string) error {
	models = normalizePublicationModels(models)
	if len(models) == 0 {
		return nil
	}
	if tx == nil {
		tx = DB
	}
	if err := tx.Where("channel_id = ? AND public_model IN ?", channelID, models).Delete(&ChannelModelPublication{}).Error; err != nil {
		return err
	}
	InvalidateChannelModelPublicationState([]int{channelID}, models)
	return nil
}

func DeleteChannelModelPublications(tx *gorm.DB, channelIDs []int) error {
	if len(channelIDs) == 0 {
		return nil
	}
	if tx == nil {
		tx = DB
	}
	if !tx.Migrator().HasTable(&ChannelModelPublication{}) {
		InvalidateChannelModelPublicationState(channelIDs, nil)
		return nil
	}
	if err := tx.Where("channel_id IN ?", channelIDs).Delete(&ChannelModelPublication{}).Error; err != nil {
		return err
	}
	InvalidateChannelModelPublicationState(channelIDs, nil)
	return nil
}
