package middleware

import "github.com/gin-gonic/gin"

const (
	tokenBalanceRateLimitRequests = 10
	tokenBalanceRateLimitSeconds  = 60
)

// TokenBalanceRateLimit protects the anonymous balance lookup endpoint with a
// dedicated per-IP bucket, independent from general API traffic.
func TokenBalanceRateLimit() gin.HandlerFunc {
	return rateLimitFactory(tokenBalanceRateLimitRequests, tokenBalanceRateLimitSeconds, "TB:")
}
