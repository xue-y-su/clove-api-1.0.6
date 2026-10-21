package controller

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestGetAllRedemptionsNegativePageSizeDoesNotReturnEveryRow(t *testing.T) {
	db := openTokenControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Redemption{}))

	redemptions := make([]model.Redemption, common.ItemsPerPage+2)
	for i := range redemptions {
		redemptions[i] = model.Redemption{
			Name:   fmt.Sprintf("pagination-%d", i),
			Key:    fmt.Sprintf("%032d", i+1),
			Status: common.RedemptionCodeStatusEnabled,
		}
	}
	require.NoError(t, db.Create(&redemptions).Error)

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/redemption/?p=1&page_size=-1", nil, 1)
	GetAllRedemptions(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	var page struct {
		Page     int                `json:"page"`
		PageSize int                `json:"page_size"`
		Total    int                `json:"total"`
		Items    []model.Redemption `json:"items"`
	}
	require.NoError(t, common.Unmarshal(response.Data, &page))
	require.Equal(t, 1, page.Page)
	require.Len(t, page.Items, common.ItemsPerPage)
	require.Equal(t, common.ItemsPerPage, page.PageSize)
	require.Equal(t, len(redemptions), page.Total)
}
