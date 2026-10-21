package model

import (
	"context"
	"errors"

	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

type TokenSecretProjection struct {
	Fingerprint string
	Ciphertext  string
	Version     uint32
	Prefix      string
	Suffix      string
}

type TokenSecretSearchCriteria struct {
	Fingerprints []string
	Affix        string
	LegacyKeys   []string
}

func EnsureTokenSecretIndexes(db *gorm.DB) error {
	if db == nil {
		return ErrDatabase
	}
	const indexName = "idx_tokens_key_fingerprint"
	if db.Migrator().HasIndex(&Token{}, indexName) {
		return nil
	}
	return db.Exec("CREATE UNIQUE INDEX " + indexName + " ON tokens (key_fingerprint)").Error
}

func GetTokenByFingerprint(ctx context.Context, fingerprint string) (*Token, error) {
	if fingerprint == "" {
		return nil, ErrTokenInvalid
	}
	var token Token
	err := DB.WithContext(ctx).
		Where("key_fingerprint = ?", fingerprint).
		First(&token).Error
	if err != nil {
		return nil, err
	}
	return &token, nil
}

func ListLegacyTokenSecrets(ctx context.Context, db *gorm.DB, afterID int, limit int) ([]Token, error) {
	if db == nil {
		return nil, ErrDatabase
	}
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}
	var tokens []Token
	err := db.WithContext(ctx).
		Where("id > ? AND "+commonKeyCol+" IS NOT NULL AND "+commonKeyCol+" <> '' AND (key_fingerprint IS NULL OR key_ciphertext = '')", afterID).
		Order("id ASC").
		Limit(limit).
		Find(&tokens).Error
	return tokens, err
}

func UpdateTokenSecretProjection(ctx context.Context, db *gorm.DB, tokenID int, projection TokenSecretProjection, clearPlaintext bool) error {
	if db == nil || tokenID <= 0 || projection.Fingerprint == "" || projection.Ciphertext == "" || projection.Version == 0 {
		return ErrTokenInvalid
	}
	updates := map[string]any{
		"key_fingerprint": projection.Fingerprint,
		"key_ciphertext":  projection.Ciphertext,
		"key_version":     projection.Version,
		"key_prefix":      projection.Prefix,
		"key_suffix":      projection.Suffix,
	}
	if clearPlaintext {
		updates["key"] = nil
	}
	result := db.WithContext(ctx).Model(&Token{}).Where("id = ?", tokenID).Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return errors.New("token secret record was not updated")
	}
	return nil
}

func SearchUserTokensBySecret(ctx context.Context, userID int, keyword string, criteria TokenSecretSearchCriteria, offset int, limit int) ([]*Token, int64, error) {
	if limit <= 0 || limit > searchHardLimit {
		limit = searchHardLimit
	}
	if offset < 0 {
		offset = 0
	}
	// Secret searches deliberately discard interpolated SQL logging because a
	// legacy exact-match value can still be present during the migration.
	query := DB.WithContext(ctx).Session(&gorm.Session{Logger: gormlogger.Discard}).
		Model(&Token{}).
		Where("user_id = ?", userID)
	if keyword != "" {
		pattern, err := sanitizeLikePattern(keyword)
		if err != nil {
			return nil, 0, err
		}
		query = query.Where("name LIKE ? ESCAPE '!'", pattern)
	}
	if criteria.Affix != "" {
		query = query.Where("(key_prefix = ? OR key_suffix = ?)", criteria.Affix, criteria.Affix)
	} else if len(criteria.Fingerprints) > 0 || len(criteria.LegacyKeys) > 0 {
		var secretQuery *gorm.DB
		if len(criteria.Fingerprints) > 0 {
			secretQuery = DB.Where("key_fingerprint IN ?", criteria.Fingerprints)
		}
		if len(criteria.LegacyKeys) > 0 {
			if secretQuery == nil {
				secretQuery = DB.Where(commonKeyCol+" IN ?", criteria.LegacyKeys)
			} else {
				secretQuery = secretQuery.Or(commonKeyCol+" IN ?", criteria.LegacyKeys)
			}
		}
		query = query.Where(secretQuery)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var tokens []*Token
	if err := query.Order("id DESC").Offset(offset).Limit(limit).Find(&tokens).Error; err != nil {
		return nil, 0, err
	}
	return tokens, total, nil
}
