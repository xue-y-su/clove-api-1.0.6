package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGetSelfWalletLedgerReturnsOnlyAuthenticatedUserPage(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.WalletLedgerEntry{}))
	require.NoError(t, db.Create(&[]model.WalletLedgerEntry{
		{UserID: 21, OperationID: "wallet:21:visible", Category: "topup", Delta: 20, BalanceAfter: 120, ReferenceType: "topup", ReferenceID: "visible", Context: "private-context", CreatedAt: 100},
		{UserID: 21, OperationID: "wallet:21:visible:settle", Category: "topup", Delta: -5, BalanceAfter: 115, ReferenceType: "topup", ReferenceID: "visible", Context: "private-settle-context", CreatedAt: 110},
		{UserID: 21, OperationID: "wallet:21:older", Category: "usage", Delta: -5, BalanceAfter: 100, ReferenceType: "request", ReferenceID: "older", CreatedAt: 50},
		{UserID: 22, OperationID: "wallet:22:hidden", Category: "topup", Delta: 999, BalanceAfter: 999, ReferenceType: "topup", ReferenceID: "hidden", Context: "other-user-secret", CreatedAt: 25},
	}).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/wallet/ledger/self?p=1&page_size=1&expand_entries=true", nil)
	ctx.Set("id", 21)

	GetSelfWalletLedger(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.JSONEq(t, `{
		"success": true,
		"message": "",
		"data": {
			"items": [{
				"id": 2,
				"category": "topup",
				"delta": 15,
				"balance_after": 115,
				"reference_type": "topup",
				"reference_id": "visible",
				"created_at": 110,
				"entry_count": 2,
				"entries": [{
					"id": 2,
					"category": "topup",
					"delta": -5,
					"balance_after": 115,
					"created_at": 110
				}, {
					"id": 1,
					"category": "topup",
					"delta": 20,
					"balance_after": 120,
					"created_at": 100
				}]
			}],
			"total": 2,
			"page": 1,
			"page_size": 1,
			"activated_at": 50,
			"history_complete": false
		}
	}`, recorder.Body.String())
	require.NotContains(t, recorder.Body.String(), "wallet:21:visible")
	require.NotContains(t, recorder.Body.String(), "private-context")
	require.NotContains(t, recorder.Body.String(), "private-settle-context")
	require.NotContains(t, recorder.Body.String(), "other-user-secret")
}
