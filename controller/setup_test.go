package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGetSetupIncludesDatabaseTypeWhenInitialized(t *testing.T) {
	previousSetup := constant.Setup
	previousMainDatabaseType := common.MainDatabaseType()
	previousLogDatabaseType := common.LogDatabaseType()
	constant.Setup = true
	common.SetDatabaseTypes(common.DatabaseTypeMySQL, previousLogDatabaseType)
	t.Cleanup(func() {
		constant.Setup = previousSetup
		common.SetDatabaseTypes(previousMainDatabaseType, previousLogDatabaseType)
	})

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/setup", nil)

	GetSetup(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	require.Contains(t, string(response.Data), `"database_type":"mysql"`)
}
