package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func resetRateLimiterForTest() {
	inMemoryRateLimiter.Reset()
}

func TestGlobalAPIRateLimitSeparatesReadAndWriteBuckets(t *testing.T) {
	gin.SetMode(gin.TestMode)
	resetRateLimiterForTest()
	previousEnabled := common.GlobalApiRateLimitEnable
	previousRedisEnabled := common.RedisEnabled
	previousLimit := common.GlobalApiRateLimitNum
	previousDuration := common.GlobalApiRateLimitDuration
	common.GlobalApiRateLimitEnable = true
	common.RedisEnabled = false
	common.GlobalApiRateLimitNum = 1
	common.GlobalApiRateLimitDuration = 60
	t.Cleanup(func() {
		common.GlobalApiRateLimitEnable = previousEnabled
		common.RedisEnabled = previousRedisEnabled
		common.GlobalApiRateLimitNum = previousLimit
		common.GlobalApiRateLimitDuration = previousDuration
		resetRateLimiterForTest()
	})

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(common.RequestIdKey, "rate-limit-request")
		c.Next()
	})
	router.Use(GlobalAPIRateLimit())
	router.Any("/api/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	for _, method := range []string{http.MethodGet, http.MethodPost} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(method, "/api/test", nil)
		router.ServeHTTP(recorder, request)
		require.Equal(t, http.StatusOK, recorder.Code, method)
	}
}

func TestGlobalAPIRateLimitSeparatesSignedInUsersBehindOneIP(t *testing.T) {
	gin.SetMode(gin.TestMode)
	resetRateLimiterForTest()
	previousEnabled := common.GlobalApiRateLimitEnable
	previousRedisEnabled := common.RedisEnabled
	previousLimit := common.GlobalApiRateLimitNum
	previousDuration := common.GlobalApiRateLimitDuration
	common.GlobalApiRateLimitEnable = true
	common.RedisEnabled = false
	common.GlobalApiRateLimitNum = 1
	common.GlobalApiRateLimitDuration = 60
	t.Cleanup(func() {
		common.GlobalApiRateLimitEnable = previousEnabled
		common.RedisEnabled = previousRedisEnabled
		common.GlobalApiRateLimitNum = previousLimit
		common.GlobalApiRateLimitDuration = previousDuration
		resetRateLimiterForTest()
	})

	router := gin.New()
	router.Use(sessions.Sessions("session", cookie.NewStore([]byte("rate-limit-test-secret"))))
	router.Use(func(c *gin.Context) {
		userID, err := strconv.Atoi(c.GetHeader("X-Test-User"))
		require.NoError(t, err)
		sessions.Default(c).Set("id", userID)
		c.Next()
	})
	router.Use(GlobalAPIRateLimit())
	router.GET("/api/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	request := func(userID string) int {
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
		req.Header.Set("X-Test-User", userID)
		router.ServeHTTP(recorder, req)
		return recorder.Code
	}

	require.Equal(t, http.StatusOK, request("1"))
	require.Equal(t, http.StatusOK, request("2"))
	require.Equal(t, http.StatusTooManyRequests, request("1"))
}

func TestGlobalAPIRateLimitReturnsStructuredRetryResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	resetRateLimiterForTest()
	previousEnabled := common.GlobalApiRateLimitEnable
	previousRedisEnabled := common.RedisEnabled
	previousLimit := common.GlobalApiRateLimitNum
	previousDuration := common.GlobalApiRateLimitDuration
	common.GlobalApiRateLimitEnable = true
	common.RedisEnabled = false
	common.GlobalApiRateLimitNum = 1
	common.GlobalApiRateLimitDuration = 60
	t.Cleanup(func() {
		common.GlobalApiRateLimitEnable = previousEnabled
		common.RedisEnabled = previousRedisEnabled
		common.GlobalApiRateLimitNum = previousLimit
		common.GlobalApiRateLimitDuration = previousDuration
		resetRateLimiterForTest()
	})

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(common.RequestIdKey, "rate-limit-request")
		c.Next()
	})
	router.Use(GlobalAPIRateLimit())
	router.POST("/api/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	for attempt := 0; attempt < 2; attempt++ {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/test", nil)
		router.ServeHTTP(recorder, request)
		if attempt == 0 {
			require.Equal(t, http.StatusOK, recorder.Code)
			continue
		}

		require.Equal(t, http.StatusTooManyRequests, recorder.Code)
		require.Equal(t, "60", recorder.Header().Get("Retry-After"))
		require.Equal(t, "no-store", recorder.Header().Get("Cache-Control"))
		var body struct {
			Success    bool   `json:"success"`
			Code       string `json:"code"`
			Message    string `json:"message"`
			RetryAfter int64  `json:"retry_after"`
			RequestID  string `json:"request_id"`
		}
		require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
		require.False(t, body.Success)
		require.Equal(t, "RATE_LIMITED", body.Code)
		require.NotEmpty(t, body.Message)
		require.Equal(t, int64(60), body.RetryAfter)
		require.Equal(t, "rate-limit-request", body.RequestID)
	}
}

func TestCriticalUserRateLimitSeparatesPoliciesAndUsers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	resetRateLimiterForTest()
	previousEnabled := common.CriticalRateLimitEnable
	previousRedisEnabled := common.RedisEnabled
	previousLimit := common.CriticalRateLimitNum
	previousDuration := common.CriticalRateLimitDuration
	common.CriticalRateLimitEnable = true
	common.RedisEnabled = false
	common.CriticalRateLimitNum = 1
	common.CriticalRateLimitDuration = 60
	t.Cleanup(func() {
		common.CriticalRateLimitEnable = previousEnabled
		common.RedisEnabled = previousRedisEnabled
		common.CriticalRateLimitNum = previousLimit
		common.CriticalRateLimitDuration = previousDuration
		resetRateLimiterForTest()
	})

	router := gin.New()
	router.Use(func(c *gin.Context) {
		userID, err := strconv.Atoi(c.GetHeader("X-Test-User"))
		require.NoError(t, err)
		c.Set("id", userID)
		c.Set(common.RequestIdKey, "rate-limit-request")
		c.Next()
	})
	router.POST("/profile", CriticalUserRateLimit("profile.write"), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	router.POST("/invitation", CriticalUserRateLimit("invitation.write"), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	request := func(path string, userID string) int {
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, path, nil)
		req.Header.Set("X-Test-User", userID)
		router.ServeHTTP(recorder, req)
		return recorder.Code
	}

	require.Equal(t, http.StatusOK, request("/profile", "1"))
	require.Equal(t, http.StatusOK, request("/invitation", "1"))
	require.Equal(t, http.StatusOK, request("/profile", "2"))
	require.Equal(t, http.StatusTooManyRequests, request("/profile", "1"))
}

func TestGlobalWebRateLimitDoesNotChargeStaticAssetsAsPageViews(t *testing.T) {
	gin.SetMode(gin.TestMode)
	resetRateLimiterForTest()
	previousEnabled := common.GlobalWebRateLimitEnable
	previousRedisEnabled := common.RedisEnabled
	previousLimit := common.GlobalWebRateLimitNum
	previousDuration := common.GlobalWebRateLimitDuration
	common.GlobalWebRateLimitEnable = true
	common.RedisEnabled = false
	common.GlobalWebRateLimitNum = 1
	common.GlobalWebRateLimitDuration = 60
	t.Cleanup(func() {
		common.GlobalWebRateLimitEnable = previousEnabled
		common.RedisEnabled = previousRedisEnabled
		common.GlobalWebRateLimitNum = previousLimit
		common.GlobalWebRateLimitDuration = previousDuration
		resetRateLimiterForTest()
	})

	router := gin.New()
	router.Use(GlobalWebRateLimit())
	router.GET("/*path", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	staticRequest := httptest.NewRequest(http.MethodGet, "/static/js/app.js", nil)
	staticRequest.Header.Set("Accept", "*/*")
	staticRequest.Header.Set("Sec-Fetch-Dest", "script")
	staticRecorder := httptest.NewRecorder()
	router.ServeHTTP(staticRecorder, staticRequest)
	require.Equal(t, http.StatusOK, staticRecorder.Code)

	pageRequest := httptest.NewRequest(http.MethodGet, "/playground", nil)
	pageRequest.Header.Set("Accept", "text/html,application/xhtml+xml")
	pageRequest.Header.Set("Sec-Fetch-Dest", "document")
	pageRecorder := httptest.NewRecorder()
	router.ServeHTTP(pageRecorder, pageRequest)
	require.Equal(t, http.StatusOK, pageRecorder.Code)

	secondPageRequest := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	secondPageRequest.Header.Set("Accept", "text/html")
	secondPageRequest.Header.Set("Sec-Fetch-Dest", "document")
	secondPageRecorder := httptest.NewRecorder()
	router.ServeHTTP(secondPageRecorder, secondPageRequest)
	require.Equal(t, http.StatusTooManyRequests, secondPageRecorder.Code)
}

func TestTokenBalanceRateLimitUsesDedicatedIPBucket(t *testing.T) {
	gin.SetMode(gin.TestMode)
	resetRateLimiterForTest()
	previousRedisEnabled := common.RedisEnabled
	common.RedisEnabled = false
	t.Cleanup(func() {
		common.RedisEnabled = previousRedisEnabled
		resetRateLimiterForTest()
	})

	router := gin.New()
	router.Use(TokenBalanceRateLimit())
	router.POST("/api/token-balance", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	for attempt := 1; attempt <= 11; attempt++ {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/token-balance", nil)
		router.ServeHTTP(recorder, request)
		if attempt <= 10 {
			require.Equal(t, http.StatusOK, recorder.Code)
			continue
		}
		require.Equal(t, http.StatusTooManyRequests, recorder.Code)
		require.Equal(t, "60", recorder.Header().Get("Retry-After"))
	}
}
