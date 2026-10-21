package controller

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	appi18n "github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func withTokenBalanceBuilder(t *testing.T, builder func(string) (*service.TokenBalanceSnapshot, error)) {
	t.Helper()
	original := buildTokenBalanceSnapshot
	buildTokenBalanceSnapshot = builder
	t.Cleanup(func() { buildTokenBalanceSnapshot = original })
}

func TestGetTokenBalanceReturnsRedactedNoStoreResponse(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	user := &model.User{Id: 11, Username: "balance-owner", Role: 1, Status: 1}
	require.NoError(t, db.Create(user).Error)
	gin.SetMode(gin.TestMode)
	const rawKey = "sk-secret-key-value"
	var received string
	withTokenBalanceBuilder(t, func(key string) (*service.TokenBalanceSnapshot, error) {
		received = key
		return &service.TokenBalanceSnapshot{
			UserID:    user.Id,
			MaskedKey: "sk-secr**********alue",
			Status:    service.TokenBalanceStatusActive,
		}, nil
	})

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/token-balance", strings.NewReader(`{"key":"`+rawKey+`"}`))
	ctx.Request.Header.Set("Content-Type", "application/json")

	GetTokenBalance(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "no-store, no-cache, must-revalidate, private, max-age=0", recorder.Header().Get("Cache-Control"))
	require.Equal(t, rawKey, received)
	require.NotContains(t, recorder.Body.String(), rawKey)
	require.Contains(t, recorder.Body.String(), `"success":true`)

	var log model.Log
	require.NoError(t, db.First(&log, "type = ?", model.LogTypeManage).Error)
	require.Equal(t, user.Id, log.UserId)
	require.NotContains(t, log.Other, rawKey)
	var other map[string]any
	require.NoError(t, json.Unmarshal([]byte(log.Other), &other))
	op := other["op"].(map[string]any)
	require.Equal(t, "sensitive.token_balance_query", op["action"])
	params := op["params"].(map[string]any)
	require.Equal(t, "sk-secr**********alue", params["masked_key"])
	require.Equal(t, "active", params["status"])
	require.Equal(t, "success", params["result"])
	require.Equal(t, "/api/token-balance", params["route"])
	require.Equal(t, float64(user.Id), params["target_user_id"])
}

func TestGetTokenBalanceReturnsGenericInvalidKeyError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	require.NoError(t, appi18n.Init())
	const rawKey = "sk-unknown-secret"
	withTokenBalanceBuilder(t, func(string) (*service.TokenBalanceSnapshot, error) {
		return nil, service.ErrTokenBalanceKeyInvalid
	})

	testCases := []struct {
		language string
		message  string
	}{
		{language: "en", message: "Invalid API Key"},
		{language: "zh-CN", message: "API Key 无效"},
		{language: "zh-TW", message: "API Key 無效"},
	}
	for _, testCase := range testCases {
		t.Run(testCase.language, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest(http.MethodPost, "/api/token-balance", strings.NewReader(`{"key":"`+rawKey+`"}`))
			ctx.Request.Header.Set("Content-Type", "application/json")
			ctx.Request.Header.Set("Accept-Language", testCase.language)

			GetTokenBalance(ctx)

			require.Equal(t, http.StatusUnauthorized, recorder.Code)
			require.NotContains(t, recorder.Body.String(), rawKey)
			require.Contains(t, recorder.Body.String(), `"message":"`+testCase.message+`"`)
			require.Contains(t, recorder.Body.String(), `"code":"invalid_api_key"`)
		})
	}
}

func TestGetTokenBalanceHidesInternalErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	withTokenBalanceBuilder(t, func(string) (*service.TokenBalanceSnapshot, error) {
		return nil, errors.New("database password leaked")
	})

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/token-balance", strings.NewReader(`{"key":"sk-valid-shape"}`))
	ctx.Request.Header.Set("Content-Type", "application/json")

	GetTokenBalance(ctx)

	require.Equal(t, http.StatusInternalServerError, recorder.Code)
	require.NotContains(t, recorder.Body.String(), "database password leaked")
	require.Contains(t, recorder.Body.String(), `"code":"query_failed"`)
}
