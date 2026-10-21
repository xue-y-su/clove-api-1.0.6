package service

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

var tokenSecretManagerState struct {
	sync.RWMutex
	manager *TokenSecretManager
}

func ConfigureTokenSecretManager(config common.TokenSecretConfig) error {
	manager, err := NewTokenSecretManager(config)
	if err != nil {
		return err
	}
	tokenSecretManagerState.Lock()
	tokenSecretManagerState.manager = manager
	tokenSecretManagerState.Unlock()
	return nil
}

func CurrentTokenSecretManager() (*TokenSecretManager, error) {
	tokenSecretManagerState.RLock()
	manager := tokenSecretManagerState.manager
	tokenSecretManagerState.RUnlock()
	if manager != nil {
		return manager, nil
	}
	config, err := common.LoadTokenSecretConfig()
	if err != nil {
		return nil, err
	}
	if err := ConfigureTokenSecretManager(config); err != nil {
		return nil, err
	}
	tokenSecretManagerState.RLock()
	manager = tokenSecretManagerState.manager
	tokenSecretManagerState.RUnlock()
	return manager, nil
}

func LookupTokenByKey(ctx context.Context, key string) (*model.Token, error) {
	manager, err := CurrentTokenSecretManager()
	if err != nil {
		return nil, err
	}
	return manager.LookupTokenByKey(ctx, key)
}

func InsertProtectedToken(token *model.Token) error {
	manager, err := CurrentTokenSecretManager()
	if err != nil {
		return err
	}
	if err := manager.ProtectToken(token); err != nil {
		return err
	}
	return token.Insert()
}

func (manager *TokenSecretManager) LookupTokenByKey(ctx context.Context, rawKey string) (*model.Token, error) {
	candidates := model.TokenKeyLookupCandidates(rawKey)
	if len(candidates) == 0 {
		return nil, model.ErrTokenNotProvided
	}
	var lastErr error = gorm.ErrRecordNotFound
	for _, candidate := range candidates {
		if manager.HasKeys() {
			fingerprint, err := manager.Fingerprint(candidate)
			if err != nil {
				return nil, err
			}
			token, err := model.GetTokenByFingerprint(ctx, fingerprint)
			if err == nil {
				if token.KeyFingerprint == nil || !manager.MatchesFingerprint(candidate, *token.KeyFingerprint) {
					return nil, ErrTokenSecretInvalid
				}
				token.Key = candidate
				return token, nil
			}
			lastErr = err
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, err
			}
		}
		if manager.Phase() == common.TokenSecretPhaseCipherOnly {
			continue
		}
		token, err := model.GetTokenByKeyNoSQLLog(candidate)
		if err == nil {
			// Once a row has a secure projection it must authenticate through that
			// projection. Falling back here would hide a wrong HMAC configuration.
			if token.KeyFingerprint != nil || token.KeyCiphertext != "" {
				lastErr = gorm.ErrRecordNotFound
				continue
			}
			return token, nil
		}
		lastErr = err
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}
	return nil, lastErr
}

func ValidateUserToken(ctx context.Context, key string) (*model.Token, error) {
	manager, err := CurrentTokenSecretManager()
	if err != nil {
		return nil, err
	}
	return manager.ValidateUserToken(ctx, key)
}

func (manager *TokenSecretManager) ValidateUserToken(ctx context.Context, key string) (*model.Token, error) {
	if key == "" {
		return nil, model.ErrTokenNotProvided
	}
	token, err := manager.LookupTokenByKey(ctx, key)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, model.ErrTokenInvalid
		}
		return nil, fmt.Errorf("%w: %v", model.ErrDatabase, err)
	}
	if token.Status == common.TokenStatusExhausted || token.Status == common.TokenStatusExpired || token.Status != common.TokenStatusEnabled {
		return token, model.ErrTokenInvalid
	}
	if token.ExpiredTime != -1 && token.ExpiredTime < common.GetTimestamp() {
		if !common.RedisEnabled {
			token.Status = common.TokenStatusExpired
			if updateErr := token.SelectUpdate(); updateErr != nil {
				common.SysLog("failed to update token status: " + updateErr.Error())
			}
		}
		return token, model.ErrTokenInvalid
	}
	if !token.UnlimitedQuota && token.RemainQuota <= 0 {
		if !common.RedisEnabled {
			token.Status = common.TokenStatusExhausted
			if updateErr := token.SelectUpdate(); updateErr != nil {
				common.SysLog("failed to update token status: " + updateErr.Error())
			}
		}
		return token, model.ErrTokenInvalid
	}
	return token, nil
}
