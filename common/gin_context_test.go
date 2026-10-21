package common

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGetContextKeyInt64PreservesLargeWalletQuota(t *testing.T) {
	ctx, _ := gin.CreateTestContext(nil)
	SetContextKey(ctx, constant.ContextKeyUserQuota, int64(2_500_000_000))
	require.Equal(t, int64(2_500_000_000), GetContextKeyInt64(ctx, constant.ContextKeyUserQuota))
}
