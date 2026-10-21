package common

import (
	"errors"
	"testing"

	rootcommon "github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type toolReservationBillingStub struct {
	target int
	calls  int
	err    error
}

func (*toolReservationBillingStub) Settle(int) error           { return nil }
func (*toolReservationBillingStub) Refund(*gin.Context)        {}
func (*toolReservationBillingStub) NeedsRefund() bool          { return false }
func (*toolReservationBillingStub) GetPreConsumedQuota() int   { return 100 }
func (s *toolReservationBillingStub) Reserve(target int) error { return s.ReserveToTarget(target) }
func (s *toolReservationBillingStub) ReserveToTarget(target int) error {
	s.calls++
	s.target = target
	return s.err
}

func newMeterFixture(t *testing.T, setting operation_setting.ToolPriceSetting, groupRatio, otherRatio float64) *ToolBillingMeter {
	t.Helper()
	snapshot, err := operation_setting.NewToolPricingSnapshot(setting)
	require.NoError(t, err)
	return NewToolBillingMeter(ToolBillingContext{
		Pricing:              snapshot,
		PublicModel:          "gpt-4.1-mini",
		GroupRatio:           decimal.NewFromFloat(groupRatio),
		OtherRatioMultiplier: decimal.NewFromFloat(otherRatio),
		QuotaPerUnit:         decimal.NewFromInt(500000),
	})
}

func baseMeterSetting() operation_setting.ToolPriceSetting {
	return operation_setting.ToolPriceSetting{
		Enabled:          true,
		Prices:           map[string]float64{"web_search_preview": 10, "file_search": 2.5},
		MultiplierMode:   operation_setting.ToolMultiplierGroupOnly,
		ImagePricingMode: operation_setting.ImagePricingMatrix,
		ImagePriceMatrix: map[string]map[string]float64{
			"high": {"1024x1024": 0.5},
		},
		ImageModelModeOverrides: map[string]string{},
	}
}

func TestToolBillingMeterCommitsOnlySuccessfulAttempt(t *testing.T) {
	meter := newMeterFixture(t, baseMeterSetting(), 1, 1)
	first := meter.NewAttempt()
	require.NoError(t, first.Confirm(ToolEvent{Name: "web_search_preview", StableID: "call-1"}))
	meter.DiscardAttempt(first)
	second := meter.NewAttempt()
	require.NoError(t, second.Confirm(ToolEvent{Name: "file_search", StableID: "call-2"}))
	require.NoError(t, meter.CommitAttempt(second))

	result, err := meter.TargetToolQuota()
	require.NoError(t, err)
	require.Len(t, result.Items, 1)
	assert.Equal(t, "file_search", result.Items[0].Name)
	assert.Equal(t, 1, result.Items[0].Count)
	assert.Equal(t, 1250, result.ToolQuota)
}

func TestConfirmToolEventReservesCumulativeToolQuotaBeforeDelivery(t *testing.T) {
	billing := &toolReservationBillingStub{}
	info := &RelayInfo{
		Billing:          billing,
		ToolBillingMeter: newMeterFixture(t, baseMeterSetting(), 1, 1),
	}
	info.SetToolBillingBaseQuota(100)
	info.BeginToolBillingAttempt()

	require.NoError(t, info.ConfirmToolEvent(ToolEvent{Name: "file_search", StableID: "call-1"}))
	require.NoError(t, info.ConfirmToolEvent(ToolEvent{Name: "file_search", StableID: "call-1"}))

	require.Equal(t, 1, billing.calls)
	require.Equal(t, 1350, billing.target)
}

func TestConfirmToolEventStopsDeliveryWhenReservationFails(t *testing.T) {
	expected := errors.New("wallet exhausted")
	billing := &toolReservationBillingStub{err: expected}
	info := &RelayInfo{
		Billing:          billing,
		ToolBillingMeter: newMeterFixture(t, baseMeterSetting(), 1, 1),
	}
	info.SetToolBillingBaseQuota(100)
	info.BeginToolBillingAttempt()

	err := info.ConfirmToolEvent(ToolEvent{Name: "file_search", StableID: "call-1"})

	require.ErrorIs(t, err, expected)
	require.Equal(t, 1350, billing.target)
}

func TestConfirmToolEventWithdrawsEvidenceWhenReservationFails(t *testing.T) {
	expected := errors.New("wallet exhausted")
	billing := &toolReservationBillingStub{err: expected}
	info := &RelayInfo{
		Billing:          billing,
		ToolBillingMeter: newMeterFixture(t, baseMeterSetting(), 1, 1),
	}
	info.SetToolBillingBaseQuota(100)
	info.BeginToolBillingAttempt()

	err := info.ConfirmToolEvent(ToolEvent{Name: "file_search", StableID: "call-failed"})
	require.ErrorIs(t, err, expected)

	billing.err = nil
	require.NoError(t, info.CommitToolBillingAttempt())
	result, resultErr := info.ToolBillingMeter.TargetToolQuota()
	require.NoError(t, resultErr)
	require.Zero(t, result.ToolQuota,
		"evidence is billable only after its cumulative reservation succeeds")
	require.Empty(t, result.Items)
}

func TestToolBillingMeterDeduplicatesAliasesAndNormalizedPayloads(t *testing.T) {
	meter := newMeterFixture(t, baseMeterSetting(), 1, 1)
	attempt := meter.NewAttempt()
	require.NoError(t, attempt.Confirm(ToolEvent{
		Name: "web_search_preview", StableID: "item-1", Aliases: []string{"call-1"},
	}))
	require.NoError(t, attempt.Confirm(ToolEvent{
		Name: "web_search_preview", StableID: "call-1",
	}))
	require.NoError(t, attempt.Confirm(ToolEvent{
		Name: "file_search", Payload: []byte(`{"status":"completed","id":"anonymous"}`),
	}))
	require.NoError(t, attempt.Confirm(ToolEvent{
		Name: "file_search", Payload: []byte(`{ "id": "anonymous", "status": "completed" }`),
	}))
	require.NoError(t, meter.CommitAttempt(attempt))

	result, err := meter.TargetToolQuota()
	require.NoError(t, err)
	require.Len(t, result.Items, 2)
	assert.Equal(t, 1, result.Items[0].Count)
	assert.Equal(t, 1, result.Items[1].Count)
}

func TestToolBillingMeterRoundsOnceAndAllocatesLargestRemainders(t *testing.T) {
	setting := baseMeterSetting()
	setting.Prices = map[string]float64{"alpha": 0.0008, "beta": 0.0008}
	meter := newMeterFixture(t, setting, 1, 1)
	attempt := meter.NewAttempt()
	require.NoError(t, attempt.Confirm(ToolEvent{Name: "alpha", StableID: "a"}))
	require.NoError(t, attempt.Confirm(ToolEvent{Name: "beta", StableID: "b"}))
	require.NoError(t, meter.CommitAttempt(attempt))

	result, err := meter.TargetToolQuota()
	require.NoError(t, err)
	assert.Equal(t, 1, result.ToolQuota)
	require.Len(t, result.Items, 2)
	assert.Equal(t, 1, result.Items[0].SubtotalQuota)
	assert.Equal(t, 0, result.Items[1].SubtotalQuota)
	assert.Equal(t, result.ToolQuota, result.Items[0].SubtotalQuota+result.Items[1].SubtotalQuota)
}

func TestToolBillingMeterAppliesFullMultiplierOnlyWhenConfigured(t *testing.T) {
	groupOnly := baseMeterSetting()
	groupOnly.Prices = map[string]float64{"custom": 1}
	meter := newMeterFixture(t, groupOnly, 2, 3)
	attempt := meter.NewAttempt()
	require.NoError(t, attempt.Confirm(ToolEvent{Name: "custom", StableID: "a"}))
	require.NoError(t, meter.CommitAttempt(attempt))
	result, err := meter.TargetToolQuota()
	require.NoError(t, err)
	assert.Equal(t, 1000, result.ToolQuota)

	full := groupOnly
	full.MultiplierMode = operation_setting.ToolMultiplierFull
	meter = newMeterFixture(t, full, 2, 3)
	attempt = meter.NewAttempt()
	require.NoError(t, attempt.Confirm(ToolEvent{Name: "custom", StableID: "a"}))
	require.NoError(t, meter.CommitAttempt(attempt))
	result, err = meter.TargetToolQuota()
	require.NoError(t, err)
	assert.Equal(t, 3000, result.ToolQuota)
}

func TestToolBillingMeterImageMatrixUsesActualOutputCount(t *testing.T) {
	meter := newMeterFixture(t, baseMeterSetting(), 2, 1)
	attempt := meter.NewAttempt()
	require.NoError(t, attempt.Confirm(ToolEvent{
		Name: "image_generation", StableID: "image-batch", Count: 2,
		ImageQuality: "high", ImageSize: "1024x1024",
	}))
	require.NoError(t, meter.CommitAttempt(attempt))

	result, err := meter.TargetToolQuota()
	require.NoError(t, err)
	require.Len(t, result.Items, 1)
	assert.Equal(t, "usd_per_call", result.Items[0].Unit)
	assert.Equal(t, 2, result.Items[0].Count)
	assert.Equal(t, 1000000, result.ToolQuota)
}

func TestToolBillingMeterFreeDisabledAndZeroGroupProduceNoCharge(t *testing.T) {
	for name, fixture := range map[string]struct {
		mutate func(*operation_setting.ToolPriceSetting)
		group  float64
	}{
		"disabled":   {func(s *operation_setting.ToolPriceSetting) { s.Enabled = false }, 1},
		"zero price": {func(s *operation_setting.ToolPriceSetting) { s.Prices["custom"] = 0 }, 1},
		"zero group": {func(s *operation_setting.ToolPriceSetting) { s.Prices["custom"] = 10 }, 0},
	} {
		t.Run(name, func(t *testing.T) {
			setting := baseMeterSetting()
			fixture.mutate(&setting)
			meter := newMeterFixture(t, setting, fixture.group, 1)
			attempt := meter.NewAttempt()
			require.NoError(t, attempt.Confirm(ToolEvent{Name: "custom", StableID: "call"}))
			require.NoError(t, meter.CommitAttempt(attempt))
			result, err := meter.TargetToolQuota()
			require.NoError(t, err)
			assert.Zero(t, result.ToolQuota)
			assert.Empty(t, result.Items)
		})
	}
}

func TestToolBillingMeterRetainsRequestSnapshotAfterGlobalUpdate(t *testing.T) {
	setting := baseMeterSetting()
	setting.Prices = map[string]float64{"custom": 1}
	meter := newMeterFixture(t, setting, 1, 1)
	setting.Prices["custom"] = 100

	attempt := meter.NewAttempt()
	require.NoError(t, attempt.Confirm(ToolEvent{Name: "custom", StableID: "call"}))
	require.NoError(t, meter.CommitAttempt(attempt))
	result, err := meter.TargetToolQuota()
	require.NoError(t, err)
	assert.Equal(t, 500, result.ToolQuota)
}

func TestToolBillingMeterRejectsQuotaOverflow(t *testing.T) {
	setting := baseMeterSetting()
	setting.Prices = map[string]float64{"custom": 1e20}
	meter := newMeterFixture(t, setting, 1, 1)
	attempt := meter.NewAttempt()
	require.NoError(t, attempt.Confirm(ToolEvent{Name: "custom", StableID: "call"}))
	require.NoError(t, meter.CommitAttempt(attempt))

	_, err := meter.TargetToolQuota()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "quota")
	assert.Equal(t, rootcommon.MaxQuota, rootcommon.QuotaFromDecimal(decimal.NewFromFloat(1e30)))
}
