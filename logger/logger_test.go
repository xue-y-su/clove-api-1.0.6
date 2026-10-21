package logger

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/require"
)

func TestQuotaFormattingUsesReadablePrecisionWithoutDuplicatedUnit(t *testing.T) {
	general := operation_setting.GetGeneralSetting()
	previousGeneral := *general
	previousExchangeRate := operation_setting.USDExchangeRate
	t.Cleanup(func() {
		*general = previousGeneral
		operation_setting.USDExchangeRate = previousExchangeRate
	})

	unit := common.QuotaFromFloat(common.QuotaPerUnit)
	general.QuotaDisplayType = operation_setting.QuotaDisplayTypeUSD
	require.Equal(t, "$1.00", FormatQuota(unit))
	require.Equal(t, "$1.00", LogQuota(unit))
	require.Equal(t, "$0.000002", FormatQuota(1))

	general.QuotaDisplayType = operation_setting.QuotaDisplayTypeCNY
	operation_setting.USDExchangeRate = 1
	require.Equal(t, "¥1.00", FormatQuota(unit))
	require.Equal(t, "¥1.00", LogQuota(unit))

	general.QuotaDisplayType = operation_setting.QuotaDisplayTypeTokens
	require.Equal(t, "500000 点", FormatQuota(unit))
	require.Equal(t, "500000 点", LogQuota(unit))
}
