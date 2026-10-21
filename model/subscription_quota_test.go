package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCalcSubscriptionBalanceQuotaRejectsOverflow(t *testing.T) {
	previousQuotaPerUnit := common.QuotaPerUnit
	common.QuotaPerUnit = 1
	t.Cleanup(func() {
		common.QuotaPerUnit = previousQuotaPerUnit
	})

	quota, err := calcSubscriptionBalanceQuota(float64(common.MaxQuota) + 1)

	require.Error(t, err)
	assert.Zero(t, quota)
}
