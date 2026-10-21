package service

import (
	"context"
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

var ErrTokenSecretSearchInvalid = errors.New("token key search must be a complete key or exactly four prefix/suffix characters")

func SearchUserTokens(ctx context.Context, userID int, keyword string, keyQuery string, offset int, limit int) ([]*model.Token, int64, error) {
	manager, err := CurrentTokenSecretManager()
	if err != nil {
		return nil, 0, err
	}
	return manager.searchUserTokens(ctx, userID, keyword, keyQuery, offset, limit)
}

func (manager *TokenSecretManager) SearchUserTokens(userID int, keyword string, keyQuery string, offset int, limit int) ([]*model.Token, int64, error) {
	return manager.searchUserTokens(context.Background(), userID, keyword, keyQuery, offset, limit)
}

func (manager *TokenSecretManager) searchUserTokens(ctx context.Context, userID int, keyword string, rawQuery string, offset int, limit int) ([]*model.Token, int64, error) {
	criteria, err := manager.tokenSearchCriteria(rawQuery)
	if err != nil {
		return nil, 0, err
	}
	return model.SearchUserTokensBySecret(ctx, userID, keyword, criteria, offset, limit)
}

func (manager *TokenSecretManager) tokenSearchCriteria(rawQuery string) (model.TokenSecretSearchCriteria, error) {
	query := strings.TrimSpace(rawQuery)
	if query == "" {
		return model.TokenSecretSearchCriteria{}, nil
	}
	if strings.Contains(query, "%") || !validTokenKeyCharacters(query) {
		return model.TokenSecretSearchCriteria{}, ErrTokenSecretSearchInvalid
	}
	if len(query) == 4 {
		return model.TokenSecretSearchCriteria{Affix: query}, nil
	}

	candidates := model.TokenKeyLookupCandidates(query)
	validCandidates := make([]string, 0, len(candidates))
	seen := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		if len(candidate) < 16 || len(candidate) > 128 || !validTokenKeyCharacters(candidate) {
			continue
		}
		if _, exists := seen[candidate]; exists {
			continue
		}
		seen[candidate] = struct{}{}
		validCandidates = append(validCandidates, candidate)
	}
	if len(validCandidates) == 0 {
		return model.TokenSecretSearchCriteria{}, ErrTokenSecretSearchInvalid
	}

	criteria := model.TokenSecretSearchCriteria{}
	if manager.HasKeys() {
		criteria.Fingerprints = make([]string, 0, len(validCandidates))
		for _, candidate := range validCandidates {
			fingerprint, err := manager.Fingerprint(candidate)
			if err != nil {
				return model.TokenSecretSearchCriteria{}, err
			}
			criteria.Fingerprints = append(criteria.Fingerprints, fingerprint)
		}
	}
	if manager.Phase() != common.TokenSecretPhaseCipherOnly {
		criteria.LegacyKeys = validCandidates
	}
	return criteria, nil
}

func validTokenKeyCharacters(key string) bool {
	for index := 0; index < len(key); index++ {
		character := key[index]
		if (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') || (character >= '0' && character <= '9') || character == '_' || character == '-' {
			continue
		}
		return false
	}
	return key != ""
}
