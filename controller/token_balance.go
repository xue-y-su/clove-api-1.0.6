package controller

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

var buildTokenBalanceSnapshot = service.BuildTokenBalanceSnapshot

type tokenBalanceRequest struct {
	Key string `json:"key"`
}

func tokenBalanceError(c *gin.Context, status int, code, message string) {
	c.JSON(status, gin.H{
		"success": false,
		"code":    code,
		"message": message,
	})
}

func recordTokenBalanceQueryAudit(c *gin.Context, snapshot *service.TokenBalanceSnapshot, result string) {
	if snapshot == nil || snapshot.UserID <= 0 {
		return
	}
	route := c.FullPath()
	if route == "" && c.Request != nil && c.Request.URL != nil {
		route = c.Request.URL.Path
	}
	params := map[string]interface{}{
		"masked_key":     snapshot.MaskedKey,
		"status":         snapshot.Status,
		"result":         result,
		"route":          route,
		"target_user_id": snapshot.UserID,
	}
	if snapshot.TokenID > 0 {
		params["token_id"] = snapshot.TokenID
	}
	model.RecordOperationAuditLog(snapshot.UserID, auditContentEN("sensitive.token_balance_query", params), c.ClientIP(), "sensitive.token_balance_query", params, nil, nil)
}

// GetTokenBalance returns a privacy-minimized balance snapshot for a supplied
// API key. The raw key is never included in responses or log messages.
func GetTokenBalance(c *gin.Context) {
	c.Header("Cache-Control", "no-store, no-cache, must-revalidate, private, max-age=0")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")

	var request tokenBalanceRequest
	if err := c.ShouldBindJSON(&request); err != nil || strings.TrimSpace(request.Key) == "" {
		tokenBalanceError(c, http.StatusBadRequest, "invalid_request", common.TranslateMessage(c, i18n.MsgTokenBalanceInvalidRequest))
		return
	}

	snapshot, err := buildTokenBalanceSnapshot(request.Key)
	if err != nil {
		if errors.Is(err, service.ErrTokenBalanceKeyInvalid) {
			tokenBalanceError(c, http.StatusUnauthorized, "invalid_api_key", common.TranslateMessage(c, i18n.MsgTokenBalanceInvalidKey))
			return
		}
		common.SysLog(fmt.Sprintf("token balance query failed (%T)", err))
		tokenBalanceError(c, http.StatusInternalServerError, "query_failed", common.TranslateMessage(c, i18n.MsgTokenBalanceQueryFailed))
		return
	}
	recordTokenBalanceQueryAudit(c, snapshot, "success")
	common.ApiSuccess(c, snapshot)
}
