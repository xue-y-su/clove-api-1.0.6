package controller

import (
	"crypto/sha256"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func enqueueCriticalAuditWithTx(c *gin.Context, tx *gorm.DB, actorUserID int, target string, action string, params map[string]interface{}) error {
	return enqueueCriticalAuditResultWithTx(c, tx, actorUserID, target, action, "success", params)
}

func enqueueCriticalAuditResultWithTx(c *gin.Context, tx *gorm.DB, actorUserID int, target string, action string, result string, params map[string]interface{}) error {
	requestID := strings.TrimSpace(c.GetString(common.RequestIdKey))
	if requestID == "" {
		requestID = common.NewRequestId()
		c.Set(common.RequestIdKey, requestID)
	}
	digest := sha256.Sum256([]byte(requestID + "\x00" + action + "\x00" + target))
	return service.EnqueueWithTx(tx, service.AuditEvent{
		EventID:   fmt.Sprintf("audit:%x", digest[:]),
		Type:      action,
		Result:    result,
		ActorID:   actorUserID,
		Target:    target,
		IP:        c.ClientIP(),
		RequestID: requestID,
		Params:    params,
	})
}

func enqueueCriticalAudit(c *gin.Context, actorUserID int, target string, action string, result string, params map[string]interface{}) error {
	return model.DB.Transaction(func(tx *gorm.DB) error {
		return enqueueCriticalAuditResultWithTx(c, tx, actorUserID, target, action, result, params)
	})
}
