package controller

import (
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestUpdateOptionRejectsNullValue(t *testing.T) {
	setupComplianceControllerTest(t)
	ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/option/", map[string]any{
		"key":   "SMTPServer",
		"value": nil,
	}, 1)

	UpdateOption(ctx)

	response := decodeAPIResponse(t, recorder)
	require.False(t, response.Success)
	require.Contains(t, response.Message, "不能为空")

	var count int64
	require.NoError(t, model.DB.Model(&model.Option{}).Where("key = ?", "SMTPServer").Count(&count).Error)
	require.Zero(t, count)
}
