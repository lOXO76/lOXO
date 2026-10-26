package service

import (
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

func TestCheckInBalanceHistoryCodeMarksCheckInCredit(t *testing.T) {
	t.Parallel()

	createdAt := time.Date(2026, 10, 27, 0, 31, 5, 0, time.UTC)
	// lib/pq 读回的 DATE 列是当天零点，日期分量即签到自然日。
	checkInDate := time.Date(2026, 10, 27, 0, 0, 0, 0, time.UTC)

	got := checkInBalanceHistoryCode(10, 42, 0.2783, checkInDate, createdAt)

	require.Equal(t, int64(-42), got.ID)
	require.Equal(t, RedeemTypeCheckIn, got.Type)
	require.Equal(t, "CHECKIN-2026-10-27", got.Code)
	require.InDelta(t, 0.2783, got.Value, 1e-9)
	require.Equal(t, StatusUsed, got.Status)
	require.NotNil(t, got.UsedBy)
	require.Equal(t, int64(10), *got.UsedBy)
	require.NotNil(t, got.UsedAt)
	require.True(t, got.UsedAt.Equal(createdAt))
}

func TestMergeBalanceHistoryCodesInterleavesCheckInCredits(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	usedBy := int64(10)
	at := func(offset time.Duration) *time.Time {
		value := base.Add(offset)
		return &value
	}
	redeemCodes := []RedeemCode{
		{ID: 1, Type: RedeemTypeBalance, Value: 8, Status: StatusUsed, UsedBy: &usedBy, UsedAt: at(-time.Hour), CreatedAt: base.Add(-time.Hour)},
	}
	affiliateCodes := []RedeemCode{
		{ID: -5, Type: RedeemTypeAffiliateBalance, Value: 3, Status: StatusUsed, UsedBy: &usedBy, UsedAt: at(-3 * time.Hour), CreatedAt: base.Add(-3 * time.Hour)},
	}
	checkInCodes := []RedeemCode{
		checkInBalanceHistoryCode(usedBy, 5, 0.31, base, base),
		checkInBalanceHistoryCode(usedBy, 4, 0.42, base.Add(-24*time.Hour), base.Add(-2*time.Hour)),
	}

	got := mergeBalanceHistoryCodes(redeemCodes, append(affiliateCodes, checkInCodes...), pagination.PaginationParams{Page: 1, PageSize: 10})

	types := make([]string, 0, len(got))
	for _, item := range got {
		types = append(types, item.Type)
	}
	require.Equal(t, []string{RedeemTypeCheckIn, RedeemTypeBalance, RedeemTypeCheckIn, RedeemTypeAffiliateBalance}, types)
	// 返利与签到都使用负数 ID（-5），需靠类型区分，合并时不能互相覆盖。
	require.Equal(t, int64(-5), got[0].ID)
	require.Equal(t, int64(-5), got[3].ID)
}
