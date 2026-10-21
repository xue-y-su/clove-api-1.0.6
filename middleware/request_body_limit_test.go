package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPIRequestBodyLimitRejectsOversizedBodyBeforeHandler(t *testing.T) {
	oldLimit := constant.APIRequestBodyLimitKB
	constant.APIRequestBodyLimitKB = 1
	t.Cleanup(func() {
		constant.APIRequestBodyLimitKB = oldLimit
	})

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(APIRequestBodyLimit())
	handlerCalled := false
	router.POST("/api/protected", func(c *gin.Context) {
		handlerCalled = true
		c.Status(http.StatusNoContent)
	})

	request := httptest.NewRequest(http.MethodPost, "/api/protected", strings.NewReader(strings.Repeat("x", 1025)))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusRequestEntityTooLarge, recorder.Code)
	assert.False(t, handlerCalled)
}
