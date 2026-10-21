package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type TokenSecretMigrationResult struct {
	Scanned   int
	Migrated  int
	LastID    int
	FailedIDs []int
	Error     error
}

type TokenSecretMigrator struct {
	manager *TokenSecretManager
}

type TokenSecretMigrationWorker struct {
	cancel   context.CancelFunc
	done     chan struct{}
	stopOnce sync.Once
}

func NewTokenSecretMigrator(manager *TokenSecretManager) *TokenSecretMigrator {
	return &TokenSecretMigrator{manager: manager}
}

func (migrator *TokenSecretMigrator) BackfillBatch(ctx context.Context, db *gorm.DB, afterID int, limit int) TokenSecretMigrationResult {
	result := TokenSecretMigrationResult{LastID: afterID}
	if migrator == nil || migrator.manager == nil {
		result.Error = ErrTokenSecretUnavailable
		return result
	}
	tokens, err := model.ListLegacyTokenSecrets(ctx, db, afterID, limit)
	if err != nil {
		result.Error = err
		return result
	}
	clearPlaintext := migrator.manager.Phase() == common.TokenSecretPhaseCipherOnly
	var failures []error
	for index := range tokens {
		if err := ctx.Err(); err != nil {
			failures = append(failures, err)
			break
		}
		token := &tokens[index]
		result.Scanned++
		result.LastID = token.Id
		if err := migrator.manager.ProtectToken(token); err != nil {
			result.FailedIDs = append(result.FailedIDs, token.Id)
			failures = append(failures, fmt.Errorf("token %d: %w", token.Id, err))
			continue
		}
		if token.KeyFingerprint == nil {
			result.FailedIDs = append(result.FailedIDs, token.Id)
			failures = append(failures, fmt.Errorf("token %d: %w", token.Id, ErrTokenSecretUnavailable))
			continue
		}
		projection := model.TokenSecretProjection{
			Fingerprint: *token.KeyFingerprint,
			Ciphertext:  token.KeyCiphertext,
			Version:     token.KeyVersion,
			Prefix:      token.KeyPrefix,
			Suffix:      token.KeySuffix,
		}
		if err := model.UpdateTokenSecretProjection(ctx, db, token.Id, projection, clearPlaintext); err != nil {
			result.FailedIDs = append(result.FailedIDs, token.Id)
			failures = append(failures, fmt.Errorf("token %d: %w", token.Id, err))
			continue
		}
		result.Migrated++
	}
	result.Error = errors.Join(failures...)
	return result
}

func (migrator *TokenSecretMigrator) BackfillAll(ctx context.Context, db *gorm.DB, batchSize int) TokenSecretMigrationResult {
	total := TokenSecretMigrationResult{}
	afterID := 0
	for {
		batch := migrator.BackfillBatch(ctx, db, afterID, batchSize)
		total.Scanned += batch.Scanned
		total.Migrated += batch.Migrated
		total.FailedIDs = append(total.FailedIDs, batch.FailedIDs...)
		total.LastID = batch.LastID
		if batch.Error != nil {
			total.Error = errors.Join(total.Error, batch.Error)
		}
		if batch.Scanned == 0 || batch.LastID <= afterID {
			break
		}
		afterID = batch.LastID
		if batch.Scanned < normalizedTokenSecretBatchSize(batchSize) {
			break
		}
	}
	return total
}

func PrepareCipherOnlyTokenSecrets(ctx context.Context, db *gorm.DB, manager *TokenSecretManager, batchSize int) error {
	if db == nil || manager == nil || manager.Phase() != common.TokenSecretPhaseCipherOnly || !manager.HasKeys() {
		return ErrTokenSecretUnavailable
	}
	batchSize = normalizedTokenSecretBatchSize(batchSize)
	if err := ValidateCipherOnlyTokenSecrets(ctx, db, manager, batchSize); err != nil {
		return err
	}

	for {
		var ids []int
		if err := db.WithContext(ctx).Model(&model.Token{}).
			Where(clause.Neq{Column: clause.Column{Name: "key"}, Value: nil}).
			Where(clause.Neq{Column: clause.Column{Name: "key"}, Value: ""}).
			Order("id ASC").
			Limit(batchSize).
			Pluck("id", &ids).Error; err != nil {
			return err
		}
		if len(ids) == 0 {
			return nil
		}
		result := db.WithContext(ctx).Model(&model.Token{}).Where("id IN ?", ids).Update("key", nil)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != int64(len(ids)) {
			return errors.New("not all verified token plaintext rows were cleared")
		}
	}
}

func ValidateCipherOnlyTokenSecrets(ctx context.Context, db *gorm.DB, manager *TokenSecretManager, batchSize int) error {
	if db == nil || manager == nil || manager.Phase() != common.TokenSecretPhaseCipherOnly || !manager.HasKeys() {
		return ErrTokenSecretUnavailable
	}
	batchSize = normalizedTokenSecretBatchSize(batchSize)
	afterID := 0
	for {
		var tokens []model.Token
		if err := db.WithContext(ctx).
			Select("id", "key", "key_fingerprint", "key_ciphertext", "key_version").
			Where("id > ?", afterID).
			Order("id ASC").
			Limit(batchSize).
			Find(&tokens).Error; err != nil {
			return err
		}
		if len(tokens) == 0 {
			break
		}
		for index := range tokens {
			token := &tokens[index]
			afterID = token.Id
			if token.KeyFingerprint == nil || token.KeyCiphertext == "" || token.KeyVersion == 0 {
				return fmt.Errorf("token %d is not ready for cipher-only phase", token.Id)
			}
			plaintext, err := manager.RevealTokenKey(token)
			if err != nil || !manager.MatchesFingerprint(plaintext, *token.KeyFingerprint) {
				return fmt.Errorf("token %d cannot be verified for cipher-only phase", token.Id)
			}
		}
		if len(tokens) < batchSize {
			break
		}
	}
	return nil
}

func StartTokenSecretMigrationWorker() *TokenSecretMigrationWorker {
	intervalSeconds := common.GetEnvOrDefault("TOKEN_SECRET_BACKFILL_INTERVAL_SECONDS", 300)
	if intervalSeconds < 1 {
		intervalSeconds = 1
	}
	batchSize := common.GetEnvOrDefault("TOKEN_SECRET_BACKFILL_BATCH_SIZE", 100)
	return startTokenSecretMigrationWorker(context.Background(), time.Duration(intervalSeconds)*time.Second, batchSize)
}

func startTokenSecretMigrationWorker(parent context.Context, interval time.Duration, batchSize int) *TokenSecretMigrationWorker {
	ctx, cancel := context.WithCancel(parent) // #nosec G118 -- cancel is retained and called by worker.Stop.
	worker := &TokenSecretMigrationWorker{cancel: cancel, done: make(chan struct{})}
	go func() {
		defer close(worker.done)
		process := func() {
			if !common.IsMasterNode {
				return
			}
			manager, err := CurrentTokenSecretManager()
			if err != nil {
				logger.LogError(context.Background(), "token secret migration configuration is unavailable")
				return
			}
			if manager.Phase() == common.TokenSecretPhaseSchema || manager.Phase() == common.TokenSecretPhaseCipherOnly || !manager.HasKeys() {
				return
			}
			result := NewTokenSecretMigrator(manager).BackfillAll(ctx, model.DB, batchSize)
			if result.Error != nil {
				logger.LogWarn(context.Background(), fmt.Sprintf("token secret backfill completed with failures: scanned=%d migrated=%d failed_ids=%v", result.Scanned, result.Migrated, result.FailedIDs))
			} else if result.Migrated > 0 {
				logger.LogInfo(context.Background(), fmt.Sprintf("token secret backfill completed: scanned=%d migrated=%d", result.Scanned, result.Migrated))
			}
		}
		process()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				process()
			}
		}
	}()
	return worker
}

func (worker *TokenSecretMigrationWorker) Stop(ctx context.Context) error {
	if worker == nil {
		return nil
	}
	worker.stopOnce.Do(worker.cancel)
	select {
	case <-worker.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func normalizedTokenSecretBatchSize(batchSize int) int {
	if batchSize <= 0 {
		return 100
	}
	if batchSize > 1000 {
		return 1000
	}
	return batchSize
}
