package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

var timeFormat = "2006-01-02T15:04:05.000Z"

var inMemoryRateLimiter common.InMemoryRateLimiter

var defNext = func(c *gin.Context) {
	c.Next()
}

func abortRateLimited(c *gin.Context, retryAfter int64) {
	if retryAfter < 1 {
		retryAfter = 1
	}
	c.Header("Retry-After", strconv.FormatInt(retryAfter, 10))
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusTooManyRequests, gin.H{
		"success":     false,
		"code":        "RATE_LIMITED",
		"message":     common.TranslateMessage(c, i18n.MsgRateLimitGlobal),
		"retry_after": retryAfter,
		"request_id":  c.GetString(common.RequestIdKey),
	})
	c.Abort()
}

func redisRateLimiter(c *gin.Context, maxRequestNum int, duration int64, mark string) {
	ctx := context.Background()
	rdb := common.RDB
	key := "rateLimit:" + mark + c.ClientIP()
	listLength, err := rdb.LLen(ctx, key).Result()
	if err != nil {
		fmt.Println(err.Error())
		c.Status(http.StatusInternalServerError)
		c.Abort()
		return
	}
	if listLength < int64(maxRequestNum) {
		rdb.LPush(ctx, key, time.Now().Format(timeFormat))
		rdb.Expire(ctx, key, common.RateLimitKeyExpirationDuration)
	} else {
		oldTimeStr, _ := rdb.LIndex(ctx, key, -1).Result()
		oldTime, err := time.Parse(timeFormat, oldTimeStr)
		if err != nil {
			fmt.Println(err)
			c.Status(http.StatusInternalServerError)
			c.Abort()
			return
		}
		nowTimeStr := time.Now().Format(timeFormat)
		nowTime, err := time.Parse(timeFormat, nowTimeStr)
		if err != nil {
			fmt.Println(err)
			c.Status(http.StatusInternalServerError)
			c.Abort()
			return
		}
		// time.Since will return negative number!
		// See: https://stackoverflow.com/questions/50970900/why-is-time-since-returning-negative-durations-on-windows
		if int64(nowTime.Sub(oldTime).Seconds()) < duration {
			rdb.Expire(ctx, key, common.RateLimitKeyExpirationDuration)
			elapsed := int64(nowTime.Sub(oldTime).Seconds())
			abortRateLimited(c, duration-elapsed)
			return
		} else {
			rdb.LPush(ctx, key, time.Now().Format(timeFormat))
			rdb.LTrim(ctx, key, 0, int64(maxRequestNum-1))
			rdb.Expire(ctx, key, common.RateLimitKeyExpirationDuration)
		}
	}
}

func memoryRateLimiter(c *gin.Context, maxRequestNum int, duration int64, mark string) {
	key := mark + c.ClientIP()
	if !inMemoryRateLimiter.Request(key, maxRequestNum, duration) {
		abortRateLimited(c, duration)
		return
	}
}

func rateLimitFactory(maxRequestNum int, duration int64, mark string) func(c *gin.Context) {
	if common.RedisEnabled {
		return func(c *gin.Context) {
			redisRateLimiter(c, maxRequestNum, duration, mark)
		}
	} else {
		// It's safe to call multi times.
		inMemoryRateLimiter.Init(common.RateLimitKeyExpirationDuration)
		return func(c *gin.Context) {
			memoryRateLimiter(c, maxRequestNum, duration, mark)
		}
	}
}

func GlobalWebRateLimit() func(c *gin.Context) {
	if common.GlobalWebRateLimitEnable {
		limiter := rateLimitFactory(common.GlobalWebRateLimitNum, common.GlobalWebRateLimitDuration, "GW")
		return func(c *gin.Context) {
			if !isWebDocumentRequest(c.Request) {
				c.Next()
				return
			}
			limiter(c)
		}
	}
	return defNext
}

func isWebDocumentRequest(request *http.Request) bool {
	if request.Method != http.MethodGet && request.Method != http.MethodHead {
		return true
	}

	switch strings.ToLower(strings.TrimSpace(request.Header.Get("Sec-Fetch-Dest"))) {
	case "document", "iframe":
		return true
	case "audio", "embed", "empty", "font", "image", "manifest", "object", "script", "serviceworker", "sharedworker", "style", "track", "video", "worker":
		return false
	}

	accept := strings.ToLower(request.Header.Get("Accept"))
	if strings.Contains(accept, "text/html") || strings.Contains(accept, "application/xhtml+xml") {
		return true
	}

	path := strings.TrimSuffix(request.URL.Path, "/")
	if path == "" {
		return true
	}
	lastSlash := strings.LastIndex(path, "/")
	return !strings.Contains(path[lastSlash+1:], ".")
}

func GlobalAPIRateLimit() func(c *gin.Context) {
	if common.GlobalApiRateLimitEnable {
		readLimiter := globalAPIRateLimitFactory(common.GlobalApiRateLimitNum, common.GlobalApiRateLimitDuration, "GA:R:")
		writeLimiter := globalAPIRateLimitFactory(common.GlobalApiRateLimitNum, common.GlobalApiRateLimitDuration, "GA:W:")
		return func(c *gin.Context) {
			switch c.Request.Method {
			case http.MethodGet, http.MethodHead, http.MethodOptions:
				readLimiter(c)
			default:
				writeLimiter(c)
			}
		}
	}
	return defNext
}

func globalAPIRateLimitFactory(maxRequestNum int, duration int64, mark string) func(c *gin.Context) {
	if !common.RedisEnabled {
		inMemoryRateLimiter.Init(common.RateLimitKeyExpirationDuration)
	}
	return func(c *gin.Context) {
		identity := "ip:" + c.ClientIP()
		if value, exists := c.Get(sessions.DefaultKey); exists {
			if session, ok := value.(sessions.Session); ok {
				switch userID := session.Get("id").(type) {
				case int:
					if userID > 0 {
						identity = "user:" + strconv.Itoa(userID)
					}
				case int64:
					if userID > 0 {
						identity = "user:" + strconv.FormatInt(userID, 10)
					}
				}
			}
		}

		key := mark + identity
		if common.RedisEnabled {
			redisRateLimiterByKey(c, maxRequestNum, duration, "rateLimit:"+key)
			return
		}
		if !inMemoryRateLimiter.Request(key, maxRequestNum, duration) {
			abortRateLimited(c, duration)
		}
	}
}

func CriticalRateLimit() func(c *gin.Context) {
	if common.CriticalRateLimitEnable {
		return rateLimitFactory(common.CriticalRateLimitNum, common.CriticalRateLimitDuration, "CT:legacy:ip:")
	}
	return defNext
}

func CriticalIPRateLimit(policy string) func(c *gin.Context) {
	if !common.CriticalRateLimitEnable {
		return defNext
	}
	return rateLimitFactory(
		common.CriticalRateLimitNum,
		common.CriticalRateLimitDuration,
		"CT:"+policy+":ip:",
	)
}

func CriticalUserRateLimit(policy string) func(c *gin.Context) {
	if !common.CriticalRateLimitEnable {
		return defNext
	}
	return userRateLimitFactory(
		common.CriticalRateLimitNum,
		common.CriticalRateLimitDuration,
		"CT:"+policy,
	)
}

func DownloadRateLimit() func(c *gin.Context) {
	return rateLimitFactory(common.DownloadRateLimitNum, common.DownloadRateLimitDuration, "DW")
}

func UploadRateLimit() func(c *gin.Context) {
	return rateLimitFactory(common.UploadRateLimitNum, common.UploadRateLimitDuration, "UP")
}

// userRateLimitFactory creates a rate limiter keyed by authenticated user ID
// instead of client IP, making it resistant to proxy rotation attacks.
// Must be used AFTER authentication middleware (UserAuth).
func userRateLimitFactory(maxRequestNum int, duration int64, mark string) func(c *gin.Context) {
	if common.RedisEnabled {
		return func(c *gin.Context) {
			userId := c.GetInt("id")
			if userId == 0 {
				c.Status(http.StatusUnauthorized)
				c.Abort()
				return
			}
			key := fmt.Sprintf("rateLimit:%s:user:%d", mark, userId)
			redisRateLimiterByKey(c, maxRequestNum, duration, key)
		}
	}
	// It's safe to call multi times.
	inMemoryRateLimiter.Init(common.RateLimitKeyExpirationDuration)
	return func(c *gin.Context) {
		userId := c.GetInt("id")
		if userId == 0 {
			c.Status(http.StatusUnauthorized)
			c.Abort()
			return
		}
		key := fmt.Sprintf("%s:user:%d", mark, userId)
		if !inMemoryRateLimiter.Request(key, maxRequestNum, duration) {
			abortRateLimited(c, duration)
			return
		}
	}
}

func redisRateLimiterByKey(c *gin.Context, maxRequestNum int, duration int64, key string) {
	ctx := context.Background()
	rdb := common.RDB
	listLength, err := rdb.LLen(ctx, key).Result()
	if err != nil {
		fmt.Println(err.Error())
		c.Status(http.StatusInternalServerError)
		c.Abort()
		return
	}
	if listLength < int64(maxRequestNum) {
		rdb.LPush(ctx, key, time.Now().Format(timeFormat))
		rdb.Expire(ctx, key, common.RateLimitKeyExpirationDuration)
	} else {
		oldTimeStr, _ := rdb.LIndex(ctx, key, -1).Result()
		oldTime, err := time.Parse(timeFormat, oldTimeStr)
		if err != nil {
			fmt.Println(err)
			c.Status(http.StatusInternalServerError)
			c.Abort()
			return
		}
		nowTimeStr := time.Now().Format(timeFormat)
		nowTime, err := time.Parse(timeFormat, nowTimeStr)
		if err != nil {
			fmt.Println(err)
			c.Status(http.StatusInternalServerError)
			c.Abort()
			return
		}
		if int64(nowTime.Sub(oldTime).Seconds()) < duration {
			rdb.Expire(ctx, key, common.RateLimitKeyExpirationDuration)
			elapsed := int64(nowTime.Sub(oldTime).Seconds())
			abortRateLimited(c, duration-elapsed)
			return
		} else {
			rdb.LPush(ctx, key, time.Now().Format(timeFormat))
			rdb.LTrim(ctx, key, 0, int64(maxRequestNum-1))
			rdb.Expire(ctx, key, common.RateLimitKeyExpirationDuration)
		}
	}
}

// SearchRateLimit returns a per-user rate limiter for search endpoints.
// Configurable via SEARCH_RATE_LIMIT_ENABLE / SEARCH_RATE_LIMIT / SEARCH_RATE_LIMIT_DURATION.
func SearchRateLimit() func(c *gin.Context) {
	if !common.SearchRateLimitEnable {
		return defNext
	}
	return userRateLimitFactory(common.SearchRateLimitNum, common.SearchRateLimitDuration, "SR")
}
