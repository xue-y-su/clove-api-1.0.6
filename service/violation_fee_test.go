package service

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
)

func TestCalcViolationFeeQuotaRejectsOverflow(t *testing.T) {
	previousQuotaPerUnit := common.QuotaPerUnit
	common.QuotaPerUnit = 1
	t.Cleanup(func() {
		common.QuotaPerUnit = previousQuotaPerUnit
	})

	assert.Zero(t, calcViolationFeeQuota(float64(common.MaxQuota)+1, 1))
}
