package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRequestIDAddsAuditMetadataToRequestContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	require.NoError(t, router.SetTrustedProxies(nil))
	router.Use(RequestId())
	router.GET("/", func(c *gin.Context) {
		requestID, _ := c.Request.Context().Value(constant.ContextKey(common.RequestIdKey)).(string)
		clientIP, _ := c.Request.Context().Value(constant.ContextKeyClientIP).(string)
		c.JSON(http.StatusOK, gin.H{"request_id": requestID, "client_ip": clientIP})
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "203.0.113.19:4321"
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.NotEmpty(t, recorder.Header().Get(common.RequestIdKey))
	require.JSONEq(t, `{"request_id":"`+recorder.Header().Get(common.RequestIdKey)+`","client_ip":"203.0.113.19"}`, recorder.Body.String())
}
