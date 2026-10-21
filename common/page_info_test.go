package common

import (
	"math"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGetPageQueryNormalizesPaginationBounds(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name         string
		target       string
		wantPage     int
		wantPageSize int
	}{
		{
			name:         "negative page and page size use defaults",
			target:       "/items?p=-2&page_size=-1",
			wantPage:     1,
			wantPageSize: ItemsPerPage,
		},
		{
			name:         "negative legacy ps uses default",
			target:       "/items?ps=-1",
			wantPage:     1,
			wantPageSize: ItemsPerPage,
		},
		{
			name:         "negative legacy size uses default",
			target:       "/items?size=-1",
			wantPage:     1,
			wantPageSize: ItemsPerPage,
		},
		{
			name:         "positive legacy ps remains compatible",
			target:       "/items?page_size=0&ps=25",
			wantPage:     1,
			wantPageSize: 25,
		},
		{
			name:         "page size keeps existing maximum",
			target:       "/items?p=2&page_size=101",
			wantPage:     2,
			wantPageSize: 100,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest("GET", test.target, nil)

			pageInfo := GetPageQuery(ctx)

			require.Equal(t, test.wantPage, pageInfo.GetPage())
			require.Equal(t, test.wantPageSize, pageInfo.GetPageSize())
		})
	}
}

func TestPageInfoIndexesDoNotOverflow(t *testing.T) {
	pageInfo := PageInfo{Page: math.MaxInt, PageSize: 100}

	require.Equal(t, math.MaxInt, pageInfo.GetStartIdx())
	require.Equal(t, math.MaxInt, pageInfo.GetEndIdx())
}
