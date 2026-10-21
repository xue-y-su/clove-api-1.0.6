package controller

import (
	"context"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

var fetchInvitationPosterLogo = service.FetchInvitationPosterLogo

func GetInvitationPosterLogo(c *gin.Context) {
	common.OptionMapRWMutex.RLock()
	logoURL := common.Logo
	common.OptionMapRWMutex.RUnlock()

	logo, err := fetchInvitationPosterLogo(requestContext(c), logoURL)
	if err != nil {
		c.Header("Cache-Control", "no-store")
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "poster logo unavailable",
		})
		return
	}

	c.Header("Cache-Control", "public, max-age=300")
	c.Header("ETag", logo.ETag)
	c.Header("X-Content-Type-Options", "nosniff")
	if c.GetHeader("If-None-Match") == logo.ETag {
		c.Status(http.StatusNotModified)
		return
	}
	c.Data(http.StatusOK, logo.ContentType, logo.Body)
}

func requestContext(c *gin.Context) context.Context {
	if c.Request != nil {
		return c.Request.Context()
	}
	return context.Background()
}
