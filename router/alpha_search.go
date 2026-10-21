package router

import (
	"errors"
	"net/http"

	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

func alphaSearchEnabledGuard() gin.HandlerFunc {
	return func(c *gin.Context) {
		if operation_setting.IsAlphaSearchEnabled() {
			c.Next()
			return
		}

		apiErr := types.NewErrorWithStatusCode(
			errors.New("alpha search is disabled"),
			types.ErrorCodeAlphaSearchDisabled,
			http.StatusServiceUnavailable,
			types.ErrOptionWithSkipRetry(),
			types.ErrOptionWithNoRecordErrorLog(),
		)
		c.AbortWithStatusJSON(apiErr.StatusCode, gin.H{"error": apiErr.ToOpenAIError()})
	}
}

func registerAlphaSearchRoute(group *gin.RouterGroup, distribute gin.HandlerFunc, handler gin.HandlerFunc) {
	alphaSearchRouter := group.Group("")
	alphaSearchRouter.Use(alphaSearchEnabledGuard(), distribute)
	alphaSearchRouter.POST("/alpha/search", handler)
}
