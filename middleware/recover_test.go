package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRelayPanicRecoverDoesNotExposePanicDetail(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RelayPanicRecover())
	router.GET("/relay", func(c *gin.Context) {
		panic("upstream secret token")
	})

	request := httptest.NewRequest(http.MethodGet, "/relay", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusInternalServerError, recorder.Code)
	var body map[string]map[string]string
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	assert.Equal(t, "An internal server error occurred. Please contact the administrator with the request id.", body["error"]["message"])
	assert.Equal(t, "new_api_panic", body["error"]["type"])
	assert.NotContains(t, recorder.Body.String(), "upstream secret token")
}
