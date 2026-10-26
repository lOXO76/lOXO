package service

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestFillCheckInDailyStatsFillsMissingDays(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 10, 25, 0, 0, 0, 0, time.UTC)
	got := fillCheckInDailyStats(start, 3, map[string]CheckInDailyStat{
		"2026-10-26": {Date: "2026-10-26", Users: 4, Reward: 1.25},
	})

	require.Equal(t, []CheckInDailyStat{
		{Date: "2026-10-25"},
		{Date: "2026-10-26", Users: 4, Reward: 1.25},
		{Date: "2026-10-27"},
	}, got)
}

func TestMergeCheckInTierStatsComparesActualWithConfigured(t *testing.T) {
	t.Parallel()

	tiers := []CheckInTier{{Min: 0.1, Max: 0.5, Weight: 70}, {Min: 0.5, Max: 1, Weight: 25}, {Min: 1, Max: 5, Weight: 5}}
	actual := []CheckInTierStat{
		{TierIndex: 0, Count: 7, Reward: 2.1, ObservedMin: 0.12, ObservedMax: 0.49},
		{TierIndex: 1, Count: 3, Reward: 2.2, ObservedMin: 0.6, ObservedMax: 0.9},
		// 配置被缩减后遗留的历史档位，仍需展示
		{TierIndex: 4, Count: 1, Reward: 9, ObservedMin: 9, ObservedMax: 9},
	}

	got := mergeCheckInTierStats(actual, tiers, 11)

	require.Len(t, got, 4)
	require.Equal(t, 0, got[0].TierIndex)
	require.InDelta(t, 63.6, got[0].ActualPercent, 1e-9)
	require.InDelta(t, 70, *got[0].ConfiguredPercent, 1e-9)
	require.InDelta(t, 0.1, *got[0].ConfiguredMin, 1e-9)

	// 当前配置里存在但周期内没抽中的档位：次数为 0，仍带配置概率
	require.Equal(t, 2, got[2].TierIndex)
	require.Zero(t, got[2].Count)
	require.InDelta(t, 5, *got[2].ConfiguredPercent, 1e-9)

	// 不在当前配置里的历史档位：没有配置信息
	require.Equal(t, 4, got[3].TierIndex)
	require.Nil(t, got[3].ConfiguredPercent)
	require.InDelta(t, 9.1, got[3].ActualPercent, 1e-9)
}

func TestParseCheckInFilterDate(t *testing.T) {
	t.Parallel()

	got, err := parseCheckInFilterDate(" 2026-10-27 ")
	require.NoError(t, err)
	require.Equal(t, "2026-10-27", got)

	got, err = parseCheckInFilterDate("")
	require.NoError(t, err)
	require.Empty(t, got)

	for _, raw := range []string{"2026/10/27", "2026-13-01", "'; DROP TABLE users; --"} {
		_, err := parseCheckInFilterDate(raw)
		require.True(t, errors.Is(err, ErrCheckInInvalidDate), raw)
	}
}

func TestNormalizeCheckInStatsDays(t *testing.T) {
	t.Parallel()

	require.Equal(t, defaultCheckInStatsDays, normalizeCheckInStatsDays(0))
	require.Equal(t, 7, normalizeCheckInStatsDays(7))
	require.Equal(t, maxCheckInStatsDays, normalizeCheckInStatsDays(10000))
}
