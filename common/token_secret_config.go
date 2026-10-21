package common

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type TokenSecretPhase string

const (
	TokenSecretPhaseSchema     TokenSecretPhase = "schema"
	TokenSecretPhaseDualWrite  TokenSecretPhase = "dual-write"
	TokenSecretPhaseDualRead   TokenSecretPhase = "dual-read"
	TokenSecretPhaseCipherOnly TokenSecretPhase = "cipher-only"
	tokenSecretKeyBytes                         = 32
)

type TokenEncryptionKey struct {
	Version uint32
	Key     []byte
}

type TokenSecretConfig struct {
	Phase          TokenSecretPhase
	HMACKey        []byte
	EncryptionKeys []TokenEncryptionKey
}

func (config TokenSecretConfig) HasKeys() bool {
	return len(config.HMACKey) == tokenSecretKeyBytes && len(config.EncryptionKeys) > 0
}

func LoadTokenSecretConfig() (TokenSecretConfig, error) {
	config := TokenSecretConfig{Phase: TokenSecretPhase(strings.ToLower(strings.TrimSpace(os.Getenv("TOKEN_SECRET_PHASE"))))}
	if config.Phase == "" {
		config.Phase = TokenSecretPhaseSchema
	}
	if !validTokenSecretPhase(config.Phase) {
		return TokenSecretConfig{}, errors.New("TOKEN_SECRET_PHASE is invalid")
	}

	hmacRaw := strings.TrimSpace(os.Getenv("TOKEN_HMAC_KEY"))
	cipherRaw := strings.TrimSpace(os.Getenv("TOKEN_ENCRYPTION_KEYS"))
	if hmacRaw == "" && cipherRaw == "" {
		if config.Phase != TokenSecretPhaseSchema {
			return TokenSecretConfig{}, errors.New("token secret keys are required for the configured phase")
		}
		return config, nil
	}
	if hmacRaw == "" || cipherRaw == "" {
		return TokenSecretConfig{}, errors.New("TOKEN_HMAC_KEY and TOKEN_ENCRYPTION_KEYS must be configured together")
	}

	hmacKey, err := base64.StdEncoding.DecodeString(hmacRaw)
	if err != nil || len(hmacKey) != tokenSecretKeyBytes {
		return TokenSecretConfig{}, fmt.Errorf("TOKEN_HMAC_KEY must be base64-encoded %d-byte key material", tokenSecretKeyBytes)
	}
	config.HMACKey = hmacKey

	seenVersions := make(map[uint32]struct{})
	for _, entry := range strings.Split(cipherRaw, ",") {
		parts := strings.SplitN(strings.TrimSpace(entry), ":", 2)
		if len(parts) != 2 {
			return TokenSecretConfig{}, errors.New("TOKEN_ENCRYPTION_KEYS entry is invalid")
		}
		version, err := strconv.ParseUint(strings.TrimSpace(parts[0]), 10, 32)
		if err != nil || version == 0 {
			return TokenSecretConfig{}, errors.New("TOKEN_ENCRYPTION_KEYS version is invalid")
		}
		versionNumber := uint32(version)
		if _, exists := seenVersions[versionNumber]; exists {
			return TokenSecretConfig{}, errors.New("TOKEN_ENCRYPTION_KEYS contains a duplicate version")
		}
		key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(parts[1]))
		if err != nil || len(key) != tokenSecretKeyBytes {
			return TokenSecretConfig{}, fmt.Errorf("TOKEN_ENCRYPTION_KEYS version %d must contain base64-encoded %d-byte key material", versionNumber, tokenSecretKeyBytes)
		}
		seenVersions[versionNumber] = struct{}{}
		config.EncryptionKeys = append(config.EncryptionKeys, TokenEncryptionKey{Version: versionNumber, Key: key})
	}
	if len(config.EncryptionKeys) == 0 {
		return TokenSecretConfig{}, errors.New("TOKEN_ENCRYPTION_KEYS must include at least one version")
	}
	return config, nil
}

func validTokenSecretPhase(phase TokenSecretPhase) bool {
	switch phase {
	case TokenSecretPhaseSchema, TokenSecretPhaseDualWrite, TokenSecretPhaseDualRead, TokenSecretPhaseCipherOnly:
		return true
	default:
		return false
	}
}
