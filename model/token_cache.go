package model

import (
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
)

func getTokenCacheKey(key string) string {
	return fmt.Sprintf("token:%s", common.GenerateHMAC(key))
}

func cacheSetToken(token Token) error {
	// Encrypted token records use the dedicated fingerprint lookup path. The
	// legacy cache is keyed from plaintext and must never collapse records with
	// a NULL legacy key into the same empty-key slot.
	if token.KeyFingerprint != nil || token.KeyCiphertext != "" {
		return nil
	}
	if token.Key == "" {
		return nil
	}
	key := getTokenCacheKey(token.Key)
	token.Clean()
	err := common.RedisHSetObj(key, &token, time.Duration(common.RedisKeyCacheSeconds())*time.Second)
	if err != nil {
		return err
	}
	return nil
}

func cacheDeleteToken(key string) error {
	if key == "" {
		return nil
	}
	err := common.RedisDelKey(getTokenCacheKey(key))
	if err != nil {
		return err
	}
	return nil
}

func cacheIncrTokenQuota(key string, increment int64) error {
	err := common.RedisHIncrBy(getTokenCacheKey(key), constant.TokenFiledRemainQuota, increment)
	if err != nil {
		return err
	}
	return nil
}

func cacheDecrTokenQuota(key string, decrement int64) error {
	return cacheIncrTokenQuota(key, -decrement)
}

func cacheSetTokenField(key string, field string, value string) error {
	err := common.RedisHSetField(getTokenCacheKey(key), field, value)
	if err != nil {
		return err
	}
	return nil
}

// CacheGetTokenByKey 从缓存中获取 token，如果缓存中不存在，则从数据库中获取
func cacheGetTokenByKey(key string) (*Token, error) {
	if !common.RedisEnabled {
		return nil, fmt.Errorf("redis is not enabled")
	}
	var token Token
	err := common.RedisHGetObj(getTokenCacheKey(key), &token)
	if err != nil {
		return nil, err
	}
	token.Key = key
	return &token, nil
}
