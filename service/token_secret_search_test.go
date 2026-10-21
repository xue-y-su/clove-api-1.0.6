package service

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestSecureTokenSearchSupportsFullHMACAndFourCharacterAffixes(t *testing.T) {
	db := openTokenSecretMigrationTestDB(t)
	manager, err := NewTokenSecretManager(tokenSecretTestConfig(common.TokenSecretPhaseDualRead))
	require.NoError(t, err)
	record := model.Token{UserId: 9, Name: "production", Key: "custom_key-123456", Status: common.TokenStatusEnabled}
	require.NoError(t, manager.ProtectToken(&record))
	require.NoError(t, db.Create(&record).Error)

	tokens, total, err := manager.SearchUserTokens(9, "", "custom_key-123456", 0, 10)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Equal(t, record.Id, tokens[0].Id)

	tokens, total, err = manager.SearchUserTokens(9, "", "cust", 0, 10)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Equal(t, record.Id, tokens[0].Id)

	tokens, total, err = manager.SearchUserTokens(9, "", "3456", 0, 10)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Equal(t, record.Id, tokens[0].Id)
}

func TestSecureTokenSearchRejectsWildcardsAndArbitraryFragments(t *testing.T) {
	manager, err := NewTokenSecretManager(tokenSecretTestConfig(common.TokenSecretPhaseDualRead))
	require.NoError(t, err)

	for _, query := range []string{"cust%", "%3456", "middle", "ab", "" + string([]byte{0x7f, 'a', 'b', 'c'})} {
		_, _, err := manager.SearchUserTokens(1, "", query, 0, 10)
		require.ErrorIs(t, err, ErrTokenSecretSearchInvalid, query)
	}
}

func TestSecureTokenSearchKeepsExactLegacyFallbackOutsideCipherOnly(t *testing.T) {
	db := openTokenSecretMigrationTestDB(t)
	legacy := model.Token{UserId: 9, Name: "legacy", Key: "legacy_key-123456", Status: common.TokenStatusEnabled}
	require.NoError(t, db.Create(&legacy).Error)

	dualRead, err := NewTokenSecretManager(tokenSecretTestConfig(common.TokenSecretPhaseDualRead))
	require.NoError(t, err)
	tokens, total, err := dualRead.SearchUserTokens(9, "", legacy.Key, 0, 10)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Equal(t, legacy.Id, tokens[0].Id)

	cipherOnly, err := NewTokenSecretManager(tokenSecretTestConfig(common.TokenSecretPhaseCipherOnly))
	require.NoError(t, err)
	tokens, total, err = cipherOnly.SearchUserTokens(9, "", legacy.Key, 0, 10)
	require.NoError(t, err)
	require.Zero(t, total)
	require.Empty(t, tokens)
}
