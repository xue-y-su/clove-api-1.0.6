package model

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/compliance_setting"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ComplianceStatementSnapshot struct {
	Id                 int    `json:"id"`
	ItemKey            string `json:"item_key" gorm:"type:varchar(128);index;uniqueIndex:idx_compliance_snapshot_key_version_hash,priority:1"`
	Title              string `json:"title" gorm:"type:varchar(255);default:''"`
	SourceType         string `json:"source_type" gorm:"type:varchar(64);index;default:''"`
	Version            string `json:"version" gorm:"type:varchar(128);index;uniqueIndex:idx_compliance_snapshot_key_version_hash,priority:2"`
	Content            string `json:"content" gorm:"type:text"`
	ContentHash        string `json:"content_hash" gorm:"type:char(64);index;uniqueIndex:idx_compliance_snapshot_key_version_hash,priority:3"`
	ContentLength      int    `json:"content_length" gorm:"index;default:0"`
	SortOrder          int    `json:"sort_order" gorm:"index;default:0"`
	ConfirmationMode   string `json:"confirmation_mode" gorm:"type:varchar(64);default:''"`
	ConfirmationPhrase string `json:"confirmation_phrase" gorm:"type:varchar(255);default:''"`
	Segments           string `json:"segments" gorm:"type:text"`
	CreatedAt          int64  `json:"created_at" gorm:"bigint;index"`
}

type ComplianceConfirmation struct {
	Id          int    `json:"id"`
	UserId      int    `json:"user_id" gorm:"index;uniqueIndex:idx_compliance_confirmation_snapshot,priority:1"`
	Username    string `json:"username" gorm:"index;default:''"`
	SnapshotId  int    `json:"snapshot_id" gorm:"index;default:0;uniqueIndex:idx_compliance_confirmation_snapshot,priority:2"`
	Version     string `json:"version" gorm:"type:varchar(128);index;uniqueIndex:idx_compliance_confirmation_snapshot,priority:3"`
	ContentHash string `json:"content_hash" gorm:"type:char(64);index;uniqueIndex:idx_compliance_confirmation_snapshot,priority:4"`
	Ip          string `json:"ip" gorm:"type:varchar(64);index;default:''"`
	UserAgent   string `json:"user_agent" gorm:"type:text"`
	ConfirmedAt int64  `json:"confirmed_at" gorm:"bigint;index"`
}

type ComplianceConfirmationFilter struct {
	UserId         int
	SnapshotId     int
	Username       string
	Version        string
	ContentHash    string
	Ip             string
	StartTimestamp int64
	EndTimestamp   int64
}

func EnsureComplianceStatementSnapshots(settings compliance_setting.Settings) ([]ComplianceStatementSnapshot, error) {
	settings = compliance_setting.NormalizeSettings(settings)
	if err := compliance_setting.ValidateSettings(settings); err != nil {
		return nil, err
	}
	if !settings.StatementEnabled {
		return nil, nil
	}
	items := compliance_setting.EffectiveStatementItems(settings)
	snapshots := make([]ComplianceStatementSnapshot, 0, len(items))
	for _, item := range items {
		snapshot, err := ensureComplianceStatementSnapshot(item)
		if err != nil {
			return nil, err
		}
		snapshots = append(snapshots, snapshot)
	}
	return snapshots, nil
}

func ensureComplianceStatementSnapshot(item compliance_setting.StatementItem) (ComplianceStatementSnapshot, error) {
	hash := compliance_setting.ContentHash(item.Content)
	var snapshot ComplianceStatementSnapshot
	err := DB.Where("item_key = ? AND version = ? AND content_hash = ?", item.ItemKey, item.Version, hash).First(&snapshot).Error
	if err == nil {
		return snapshot, nil
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return snapshot, err
	}
	segmentsBytes, err := common.Marshal(item.Segments)
	if err != nil {
		return snapshot, err
	}
	title := item.Title
	if title == "" {
		title = item.ItemKey
	}
	snapshot = ComplianceStatementSnapshot{
		ItemKey:            item.ItemKey,
		Title:              title,
		SourceType:         item.SourceType,
		Version:            item.Version,
		Content:            item.Content,
		ContentHash:        hash,
		ContentLength:      len(item.Content),
		SortOrder:          item.SortOrder,
		ConfirmationMode:   item.ConfirmationMode,
		ConfirmationPhrase: item.ConfirmationPhrase,
		Segments:           string(segmentsBytes),
		CreatedAt:          common.GetTimestamp(),
	}
	return snapshot, DB.Create(&snapshot).Error
}

func RecordComplianceConfirmation(userId int, username string, version string, contentHash string, ip string, userAgent string, confirmedAt int64) error {
	if confirmedAt == 0 {
		confirmedAt = common.GetTimestamp()
	}
	confirmation := ComplianceConfirmation{
		UserId:      userId,
		Username:    strings.TrimSpace(username),
		SnapshotId:  0,
		Version:     strings.TrimSpace(version),
		ContentHash: strings.TrimSpace(contentHash),
		Ip:          NormalizeAuditIP(ip),
		UserAgent:   userAgent,
		ConfirmedAt: confirmedAt,
	}
	return upsertComplianceConfirmation(DB, &confirmation)
}

func RecordComplianceSnapshotConfirmation(userId int, username string, snapshot ComplianceStatementSnapshot, ip string, userAgent string, confirmedAt int64) error {
	if confirmedAt == 0 {
		confirmedAt = common.GetTimestamp()
	}
	confirmation := ComplianceConfirmation{
		UserId:      userId,
		Username:    strings.TrimSpace(username),
		SnapshotId:  snapshot.Id,
		Version:     strings.TrimSpace(snapshot.Version),
		ContentHash: strings.TrimSpace(snapshot.ContentHash),
		Ip:          NormalizeAuditIP(ip),
		UserAgent:   userAgent,
		ConfirmedAt: confirmedAt,
	}
	return upsertComplianceConfirmation(DB, &confirmation)
}

func upsertComplianceConfirmation(tx *gorm.DB, confirmation *ComplianceConfirmation) error {
	return tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "user_id"},
			{Name: "snapshot_id"},
			{Name: "version"},
			{Name: "content_hash"},
		},
		DoUpdates: clause.AssignmentColumns([]string{
			"username",
			"ip",
			"user_agent",
			"confirmed_at",
		}),
	}).Create(confirmation).Error
}

func HasComplianceConfirmation(userId int, version string, contentHash string) (bool, error) {
	var count int64
	err := DB.Model(&ComplianceConfirmation{}).
		Where("user_id = ? AND snapshot_id = 0 AND version = ? AND content_hash = ?", userId, strings.TrimSpace(version), strings.TrimSpace(contentHash)).
		Count(&count).Error
	return count > 0, err
}

func HasAllComplianceSnapshotConfirmations(userId int, snapshots []ComplianceStatementSnapshot) (bool, error) {
	if userId <= 0 {
		return false, nil
	}
	if len(snapshots) == 0 {
		return true, nil
	}
	ids := make([]int, 0, len(snapshots))
	for _, snapshot := range snapshots {
		if snapshot.Id <= 0 {
			return false, nil
		}
		ids = append(ids, snapshot.Id)
	}
	var count int64
	err := DB.Model(&ComplianceConfirmation{}).
		Where("user_id = ? AND snapshot_id IN ?", userId, ids).
		Distinct("snapshot_id").
		Count(&count).Error
	return int(count) == len(ids), err
}

func dedupeComplianceConfirmations() error {
	if DB == nil || !DB.Migrator().HasTable(&ComplianceConfirmation{}) {
		return nil
	}
	return DB.Exec(`
DELETE FROM compliance_confirmations
WHERE id NOT IN (
  SELECT id FROM (
    SELECT MAX(id) AS id
    FROM compliance_confirmations
    GROUP BY user_id, snapshot_id, version, content_hash
  ) AS keep_ids
)`).Error
}

func GetComplianceConfirmations(filter ComplianceConfirmationFilter, startIdx int, num int) ([]*ComplianceConfirmation, int64, error) {
	if num <= 0 {
		num = common.ItemsPerPage
	}
	if num > 100 {
		num = 100
	}
	tx := DB.Model(&ComplianceConfirmation{})
	if filter.UserId > 0 {
		tx = tx.Where("user_id = ?", filter.UserId)
	}
	if filter.SnapshotId > 0 {
		tx = tx.Where("snapshot_id = ?", filter.SnapshotId)
	}
	var err error
	if tx, err = applyExplicitLogTextFilter(tx, "username", strings.TrimSpace(filter.Username)); err != nil {
		return nil, 0, err
	}
	if tx, err = applyExplicitLogTextFilter(tx, "version", strings.TrimSpace(filter.Version)); err != nil {
		return nil, 0, err
	}
	if tx, err = applyExplicitLogTextFilter(tx, "content_hash", strings.TrimSpace(filter.ContentHash)); err != nil {
		return nil, 0, err
	}
	if tx, err = applyLogIPFilter(tx, "ip", filter.Ip); err != nil {
		return nil, 0, err
	}
	if filter.StartTimestamp != 0 {
		tx = tx.Where("confirmed_at >= ?", filter.StartTimestamp)
	}
	if filter.EndTimestamp != 0 {
		tx = tx.Where("confirmed_at <= ?", filter.EndTimestamp)
	}

	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var confirmations []*ComplianceConfirmation
	err = tx.Order("confirmed_at desc, id desc").Offset(startIdx).Limit(num).Find(&confirmations).Error
	for _, confirmation := range confirmations {
		confirmation.Ip = NormalizeAuditIP(confirmation.Ip)
	}
	return confirmations, total, err
}
