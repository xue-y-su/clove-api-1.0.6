package controller

import (
	"errors"
	"net/http"
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestTokenKeyRevealEnqueuesCriticalAuditBeforeReturningKey(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.AuditOutboxEvent{}))
	token := seedToken(t, db, 1, "owned-token", "owner1234token5678")
	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/token/"+strconv.Itoa(token.Id)+"/key", nil, 1)
	ctx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(token.Id)}}

	GetTokenKey(ctx)

	require.True(t, decodeAPIResponse(t, recorder).Success)
	var event model.AuditOutboxEvent
	require.NoError(t, db.First(&event, "event_type = ?", "token.secret_reveal").Error)
	require.Equal(t, model.AuditOutboxStatusPending, event.Status)
	require.NotContains(t, event.Payload, token.GetFullKey())
}

func TestTokenKeyRevealReturnsNoKeyWhenAuditEnqueueFails(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.AuditOutboxEvent{}))
	token := seedToken(t, db, 1, "owned-token", "owner1234token5678")
	failure := errors.New("outbox unavailable")
	require.NoError(t, db.Callback().Create().Before("gorm:create").Register("token_key_outbox_test:fail", func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Schema != nil && tx.Statement.Schema.Table == "audit_outbox_events" {
			tx.AddError(failure)
		}
	}))
	t.Cleanup(func() { _ = db.Callback().Create().Remove("token_key_outbox_test:fail") })
	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/token/"+strconv.Itoa(token.Id)+"/key", nil, 1)
	ctx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(token.Id)}}

	GetTokenKey(ctx)

	require.False(t, decodeAPIResponse(t, recorder).Success)
	require.NotContains(t, recorder.Body.String(), token.GetFullKey())
}
