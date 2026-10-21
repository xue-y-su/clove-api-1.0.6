package model

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckinRewardExpirationDefaultsDisabled(t *testing.T) {
	require.False(t, operation_setting.GetCheckinSetting().ExpireUnusedQuota)
}

func TestNextLocalMidnight(t *testing.T) {
	location := time.FixedZone("UTC+8", 8*60*60)
	now := time.Date(2026, 7, 29, 13, 5, 0, 0, location)

	require.Equal(
		t,
		time.Date(2026, 7, 30, 0, 0, 0, 0, location).Unix(),
		nextLocalMidnight(now),
	)
}

func TestNewCheckinRewardStaysPermanentWhenExpirationDisabled(t *testing.T) {
	now := time.Date(2026, 7, 29, 13, 5, 0, 0, time.FixedZone("UTC+8", 8*60*60))

	checkin := newCheckinReward(7, "2026-07-29", 1200, now, false)

	require.Equal(t, 7, checkin.UserId)
	require.Equal(t, "2026-07-29", checkin.CheckinDate)
	require.Equal(t, 1200, checkin.QuotaAwarded)
	require.Zero(t, checkin.QuotaRemaining)
	require.Zero(t, checkin.ExpiresAt)
	require.Zero(t, checkin.ReclaimedQuota)
	require.Zero(t, checkin.SettledAt)
	require.Equal(t, now.Unix(), checkin.CreatedAt)
}

func TestNewCheckinRewardCapturesExpirationWhenEnabled(t *testing.T) {
	location := time.FixedZone("UTC+8", 8*60*60)
	now := time.Date(2026, 7, 29, 13, 5, 0, 0, location)

	checkin := newCheckinReward(7, "2026-07-29", 1200, now, true)

	require.EqualValues(t, 1200, checkin.QuotaRemaining)
	require.Equal(t, time.Date(2026, 7, 30, 0, 0, 0, 0, location).Unix(), checkin.ExpiresAt)
	require.Zero(t, checkin.ReclaimedQuota)
	require.Zero(t, checkin.SettledAt)
}

func TestRandomCheckinQuotaValidatesRange(t *testing.T) {
	_, err := randomCheckinQuota(-1, 10)
	require.Error(t, err)

	_, err = randomCheckinQuota(11, 10)
	require.Error(t, err)

	_, err = randomCheckinQuota(0, common.MaxQuota+1)
	require.Error(t, err)
}

func TestRandomCheckinQuotaReturnsConfiguredValueForFixedRange(t *testing.T) {
	quota, err := randomCheckinQuota(1234, 1234)

	require.NoError(t, err)
	assert.Equal(t, 1234, quota)
}

func TestRandomCheckinQuotaStaysWithinConfiguredRange(t *testing.T) {
	quota, err := randomCheckinQuota(1000, 1001)

	require.NoError(t, err)
	assert.GreaterOrEqual(t, quota, 1000)
	assert.LessOrEqual(t, quota, 1001)
}

func TestGetUserCheckinStatsProjectsRewardLifecycle(t *testing.T) {
	setupTopupWalletTestDB(t)
	require.NoError(t, DB.Create(&User{
		Id: 61, Username: "checkin-stats", AffCode: "checkin-stats-aff",
		Status: common.UserStatusEnabled,
	}).Error)
	require.NoError(t, DB.Create(&Checkin{
		UserId: 61, CheckinDate: "2026-07-15", QuotaAwarded: 1200,
		QuotaRemaining: 450, ExpiresAt: 1784131200, ReclaimedQuota: 750,
		SettledAt: 1784131201, CreatedAt: 1784044800,
	}).Error)

	stats, err := GetUserCheckinStats(61, "2026-07")

	require.NoError(t, err)
	records, ok := stats["records"].([]CheckinRecord)
	require.True(t, ok)
	require.Len(t, records, 1)
	require.EqualValues(t, 450, records[0].QuotaRemaining)
	require.EqualValues(t, 1784131200, records[0].ExpiresAt)
	require.EqualValues(t, 750, records[0].ReclaimedQuota)
}
