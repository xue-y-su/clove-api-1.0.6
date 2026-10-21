package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	TokenRevealSessionKey = "token_reveal_verified_at"
	TokenRevealWindow     = 30 * time.Minute
)

func TokenRevealWindowValid(verifiedAt time.Time, now time.Time) bool {
	if verifiedAt.IsZero() || now.Before(verifiedAt) {
		return false
	}
	return now.Sub(verifiedAt) < TokenRevealWindow
}

func TokenRevealRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetInt("id") <= 0 {
			recordTokenRevealRejection(c, "unauthenticated")
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "未登录", "code": "UNAUTHORIZED"})
			c.Abort()
			return
		}
		session := sessions.Default(c)
		verifiedAtRaw := session.Get(TokenRevealSessionKey)
		verifiedAt, ok := verifiedAtRaw.(int64)
		if !ok || !TokenRevealWindowValid(time.Unix(verifiedAt, 0), time.Now()) {
			if verifiedAtRaw != nil {
				session.Delete(TokenRevealSessionKey)
				_ = session.Save()
			}
			reason := "verification_missing"
			if verifiedAtRaw != nil {
				reason = "verification_expired_or_invalid"
			}
			recordTokenRevealRejection(c, reason)
			c.Header("Cache-Control", "no-store")
			c.JSON(http.StatusForbidden, gin.H{
				"success": false,
				"message": "查看完整 API Key 前需要验证当前账户密码",
				"code":    "TOKEN_REVEAL_VERIFICATION_REQUIRED",
			})
			c.Abort()
			return
		}
		c.Set("token_reveal_verified_at", verifiedAt)
		c.Header("X-Token-Reveal-Verified-Until", strconv.FormatInt(verifiedAt+int64(TokenRevealWindow/time.Second), 10))
		c.Next()
	}
}

func TokenRevealRateLimit() gin.HandlerFunc {
	maxRequests := common.GetEnvOrDefault("TOKEN_REVEAL_RATE_LIMIT", 10)
	windowSeconds := int64(common.GetEnvOrDefault("TOKEN_REVEAL_RATE_LIMIT_SECONDS", 60))
	if maxRequests < 1 {
		maxRequests = 10
	}
	if windowSeconds < 1 {
		windowSeconds = 60
	}
	ipLimiter := rateLimitFactory(maxRequests, windowSeconds, "TR:IP:")
	userLimiter := userRateLimitFactory(maxRequests, windowSeconds, "TR:USER:")
	return func(c *gin.Context) {
		ipLimiter(c)
		if c.IsAborted() {
			recordTokenRevealRejection(c, "rate_limited_ip")
			return
		}
		userLimiter(c)
		if c.IsAborted() {
			recordTokenRevealRejection(c, "rate_limited_user")
			return
		}
		tokenID := strings.TrimSpace(c.Param("id"))
		if tokenID != "" {
			if common.RedisEnabled {
				key := fmt.Sprintf("rateLimit:TR:TOKEN:%s:user:%d", tokenID, c.GetInt("id"))
				redisRateLimiterByKey(c, maxRequests, windowSeconds, key)
			} else {
				inMemoryRateLimiter.Init(common.RateLimitKeyExpirationDuration)
				key := fmt.Sprintf("TR:TOKEN:%s:user:%d", tokenID, c.GetInt("id"))
				if !inMemoryRateLimiter.Request(key, maxRequests, windowSeconds) {
					abortRateLimited(c, windowSeconds)
				}
			}
			if c.IsAborted() {
				recordTokenRevealRejection(c, "rate_limited_token")
				return
			}
		}
		c.Next()
	}
}

func recordTokenRevealRejection(c *gin.Context, reason string) {
	if model.DB == nil {
		return
	}
	requestID := strings.TrimSpace(c.GetString(common.RequestIdKey))
	if requestID == "" {
		requestID = common.NewRequestId()
		c.Set(common.RequestIdKey, requestID)
	}
	target := fmt.Sprintf("user:%d:tokens", c.GetInt("id"))
	params := map[string]any{"reason": reason}
	if tokenID, err := strconv.Atoi(c.Param("id")); err == nil && tokenID > 0 {
		target = fmt.Sprintf("token:%d", tokenID)
		params["token_id"] = tokenID
	}
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		return service.EnqueueWithTx(tx, service.AuditEvent{
			Type:      "token.secret_reveal",
			Result:    "rejected",
			ActorID:   c.GetInt("id"),
			Target:    target,
			IP:        c.ClientIP(),
			RequestID: requestID,
			Params:    params,
		})
	})
	if err != nil {
		logger.LogWarn(context.Background(), "failed to enqueue rejected token reveal audit")
	}
}

func ClearTokenRevealVerification(c *gin.Context) {
	session := sessions.Default(c)
	session.Delete(TokenRevealSessionKey)
	_ = session.Save()
}
