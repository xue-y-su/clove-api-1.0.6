package middleware

import (
	"os"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func CORS() gin.HandlerFunc {
	config := cors.DefaultConfig()
	if allowOrigins := parseCORSAllowOrigins(os.Getenv("CORS_ALLOW_ORIGINS")); len(allowOrigins) > 0 {
		config.AllowOrigins = allowOrigins
		config.AllowCredentials = true
		config.AllowHeaders = []string{
			"Authorization", "Content-Type", "Accept", "Origin", "X-Requested-With",
			"New-Api-User", "X-Api-Key", "Anthropic-Version", "Anthropic-Beta",
			"X-Goog-Api-Key", "OpenAI-Beta", "OpenAI-Organization", "OpenAI-Project",
			"X-Request-Id", "Last-Event-ID", "Cache-Control",
		}
	} else {
		config.AllowAllOrigins = true
		// Keep the official relay's broad browser-client compatibility. Wildcard
		// origins never enable credentialed CORS in this mode.
		config.AllowHeaders = []string{"*"}
	}
	config.AllowMethods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}
	return cors.New(config)
}

func parseCORSAllowOrigins(value string) []string {
	if value == "" {
		return nil
	}

	parts := strings.Split(value, ",")
	origins := make([]string, 0, len(parts))
	for _, part := range parts {
		origin := strings.TrimSpace(part)
		if origin != "" {
			origins = append(origins, origin)
		}
	}
	return origins
}

func Version() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-New-Api-Version", common.DisplayVersion)
		c.Next()
	}
}

func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Header("Permissions-Policy", "camera=(self), microphone=(), geolocation=()")
		if c.Writer.Header().Get("Content-Security-Policy") == "" {
			c.Header("Content-Security-Policy", "frame-ancestors 'self'")
		}
		c.Next()
	}
}
