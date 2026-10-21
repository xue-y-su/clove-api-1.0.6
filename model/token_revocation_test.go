package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRevokedTokenFingerprintPersistsOnlyOneRevocationPerFingerprint(t *testing.T) {
	db := openTokenModelTestDB(t)
	require.NoError(t, db.AutoMigrate(&RevokedTokenFingerprint{}))

	const fingerprint = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	require.NoError(t, RecordRevokedTokenFingerprint(db, fingerprint, "compromised"))

	revoked, err := IsTokenFingerprintRevoked(db, fingerprint)
	require.NoError(t, err)
	require.True(t, revoked)

	var stored RevokedTokenFingerprint
	require.NoError(t, db.First(&stored, "fingerprint = ?", fingerprint).Error)
	require.Equal(t, fingerprint, stored.Fingerprint)
	require.Equal(t, "compromised", stored.Reason)
	require.Positive(t, stored.CreatedAt)
	require.Error(t, RecordRevokedTokenFingerprint(db, fingerprint, "compromised"))
}
