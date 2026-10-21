package service

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

var (
	ErrTokenSecretUnavailable        = errors.New("token secret is unavailable")
	ErrTokenSecretVersionUnsupported = errors.New("token secret version is unsupported")
	ErrTokenSecretInvalid            = errors.New("token secret is invalid")
)

type TokenSecretManager struct {
	phase          common.TokenSecretPhase
	hmacKey        []byte
	currentVersion uint32
	encryptionKeys map[uint32][]byte
	random         io.Reader
}

func (manager *TokenSecretManager) Phase() common.TokenSecretPhase {
	if manager == nil {
		return common.TokenSecretPhaseSchema
	}
	return manager.phase
}

func (manager *TokenSecretManager) HasKeys() bool {
	return manager != nil && len(manager.hmacKey) == sha256.Size && manager.currentVersion != 0
}

func NewTokenSecretManager(config common.TokenSecretConfig) (*TokenSecretManager, error) {
	manager := &TokenSecretManager{
		phase:          config.Phase,
		hmacKey:        append([]byte(nil), config.HMACKey...),
		encryptionKeys: make(map[uint32][]byte, len(config.EncryptionKeys)),
		random:         rand.Reader,
	}
	if config.Phase == common.TokenSecretPhaseSchema && !config.HasKeys() {
		return manager, nil
	}
	if !config.HasKeys() {
		return nil, ErrTokenSecretUnavailable
	}
	for index, key := range config.EncryptionKeys {
		if key.Version == 0 || len(key.Key) != 32 {
			return nil, ErrTokenSecretInvalid
		}
		if _, exists := manager.encryptionKeys[key.Version]; exists {
			return nil, ErrTokenSecretInvalid
		}
		manager.encryptionKeys[key.Version] = append([]byte(nil), key.Key...)
		if index == 0 {
			manager.currentVersion = key.Version
		}
	}
	return manager, nil
}

func (manager *TokenSecretManager) ProtectToken(token *model.Token) error {
	if token == nil || token.Key == "" {
		return ErrTokenSecretInvalid
	}
	if manager.phase == common.TokenSecretPhaseSchema {
		return nil
	}
	fingerprint, err := manager.Fingerprint(token.Key)
	if err != nil {
		return err
	}
	ciphertext, err := manager.encrypt(token.Key, manager.currentVersion)
	if err != nil {
		return err
	}
	token.KeyFingerprint = &fingerprint
	token.KeyCiphertext = ciphertext
	token.KeyVersion = manager.currentVersion
	token.KeyPrefix, token.KeySuffix = tokenSecretAffixes(token.Key)
	if manager.phase == common.TokenSecretPhaseDualRead || manager.phase == common.TokenSecretPhaseCipherOnly {
		token.Key = ""
	}
	return nil
}

func (manager *TokenSecretManager) RevealTokenKey(token *model.Token) (string, error) {
	if token == nil {
		return "", ErrTokenSecretInvalid
	}
	if token.KeyCiphertext != "" {
		plaintext, err := manager.decrypt(token.KeyCiphertext, token.KeyVersion)
		if err != nil {
			return "", err
		}
		if token.KeyFingerprint != nil && !manager.MatchesFingerprint(plaintext, *token.KeyFingerprint) {
			return "", ErrTokenSecretInvalid
		}
		return plaintext, nil
	}
	if manager.phase != common.TokenSecretPhaseCipherOnly && token.Key != "" {
		return token.Key, nil
	}
	return "", ErrTokenSecretUnavailable
}

func (manager *TokenSecretManager) Fingerprint(key string) (string, error) {
	if len(manager.hmacKey) != 32 || key == "" {
		return "", ErrTokenSecretUnavailable
	}
	hash := hmac.New(sha256.New, manager.hmacKey)
	_, _ = hash.Write([]byte(key))
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func (manager *TokenSecretManager) MatchesFingerprint(key string, fingerprint string) bool {
	expected, err := hex.DecodeString(fingerprint)
	if err != nil || len(expected) != sha256.Size || len(manager.hmacKey) != 32 {
		return false
	}
	hash := hmac.New(sha256.New, manager.hmacKey)
	_, _ = hash.Write([]byte(key))
	return hmac.Equal(hash.Sum(nil), expected)
}

func (manager *TokenSecretManager) RevokeTokenKey(tx *gorm.DB, key string, reason string) error {
	if manager == nil || !manager.HasKeys() {
		return ErrTokenSecretUnavailable
	}
	fingerprint, err := manager.Fingerprint(key)
	if err != nil {
		return err
	}
	return model.RecordRevokedTokenFingerprint(tx, fingerprint, reason)
}

func (manager *TokenSecretManager) IsTokenKeyRevoked(tx *gorm.DB, rawKey string) (bool, error) {
	if manager == nil || !manager.HasKeys() {
		return false, ErrTokenSecretUnavailable
	}
	for _, candidate := range model.TokenKeyLookupCandidates(rawKey) {
		fingerprint, err := manager.Fingerprint(candidate)
		if err != nil {
			return false, err
		}
		revoked, err := model.IsTokenFingerprintRevoked(tx, fingerprint)
		if err != nil || revoked {
			return revoked, err
		}
	}
	return false, nil
}

func (manager *TokenSecretManager) encrypt(plaintext string, version uint32) (string, error) {
	key, ok := manager.encryptionKeys[version]
	if !ok {
		return "", ErrTokenSecretVersionUnsupported
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", ErrTokenSecretInvalid
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", ErrTokenSecretInvalid
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(manager.random, nonce); err != nil {
		return "", fmt.Errorf("generate token secret nonce: %w", err)
	}
	payload := gcm.Seal(nonce, nonce, []byte(plaintext), tokenSecretAAD(version))
	return base64.RawStdEncoding.EncodeToString(payload), nil
}

func (manager *TokenSecretManager) decrypt(encoded string, version uint32) (string, error) {
	key, ok := manager.encryptionKeys[version]
	if !ok {
		return "", ErrTokenSecretVersionUnsupported
	}
	payload, err := base64.RawStdEncoding.DecodeString(encoded)
	if err != nil {
		return "", ErrTokenSecretInvalid
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", ErrTokenSecretInvalid
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil || len(payload) < gcm.NonceSize()+gcm.Overhead() {
		return "", ErrTokenSecretInvalid
	}
	nonce := payload[:gcm.NonceSize()]
	ciphertext := payload[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, tokenSecretAAD(version))
	if err != nil {
		return "", ErrTokenSecretInvalid
	}
	return string(plaintext), nil
}

func tokenSecretAAD(version uint32) []byte {
	return []byte(fmt.Sprintf("clove-token-secret:v%d", version))
}

func tokenSecretAffixes(key string) (string, string) {
	prefixLength := min(4, len(key))
	suffixStart := max(0, len(key)-4)
	return key[:prefixLength], key[suffixStart:]
}
