package relay

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRelayTaskFetchRejectsInvalidModeWithoutPanic(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)

	var taskErr interface{}
	require.NotPanics(t, func() {
		taskErr = RelayTaskFetch(c, -999)
	})

	require.NotNil(t, taskErr)
	require.Equal(t, http.StatusBadRequest, taskErr.(*dto.TaskError).StatusCode)
}
