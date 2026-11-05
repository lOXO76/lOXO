//go:build unit

package service

import (
	"strings"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
)

func TestRewardPeriodBoundaries(t *testing.T) {
	tests := []struct{ at, period, key, reset string }{
		{"2021-01-01T12:00:00Z", "weekly", "2020-W53", "2021-01-04T00:00:00Z"},
		{"2024-12-30T12:00:00Z", "weekly", "2025-W01", "2025-01-06T00:00:00Z"},
		{"2026-12-31T23:59:00Z", "daily", "2026-12-31", "2027-01-01T00:00:00Z"},
		{"2026-12-31T12:00:00Z", "monthly", "2026-12", "2027-01-01T00:00:00Z"},
		{"2026-11-05T12:00:00Z", "once", "once", ""},
	}
	old := time.Local
	time.Local = time.UTC
	defer func() { time.Local = old }()
	for _, tt := range tests {
		t.Run(tt.period+tt.at, func(t *testing.T) {
			now, _ := time.Parse(time.RFC3339, tt.at)
			key, reset := rewardPeriod(tt.period, now)
			if key != tt.key {
				t.Fatalf("key %s want %s", key, tt.key)
			}
			if tt.reset == "" {
				if reset != nil {
					t.Fatal(reset)
				}
			} else if reset == nil || reset.Format(time.RFC3339) != tt.reset {
				t.Fatalf("reset %v want %s", reset, tt.reset)
			}
		})
	}
}
func TestRewardJumpURL(t *testing.T) {
	for _, tt := range []struct{ target, mode, want string }{
		{"https://example.com/path?x=1#frag", "sdk", "https://example.com/path?reward_ticket=rt_123&x=1#frag"},
		{"https://example.com/{ticket}?t={ticket}#end", "callback", "https://example.com/rt_123?t=rt_123#end"},
		{"https://example.com/{ticket}?x=1#frag", "timer", "https://example.com/?x=1#frag"},
	} {
		got, err := rewardJumpURL(tt.target, tt.mode, "rt_123")
		if err != nil || got != tt.want {
			t.Fatalf("got %s %v want %s", got, err, tt.want)
		}
	}
}
func TestRewardTicketAndExpiration(t *testing.T) {
	a, err := rewardRandomToken("rt_", 20)
	if err != nil || len(a) != 43 || !strings.HasPrefix(a, "rt_") {
		t.Fatalf("ticket %q %v", a, err)
	}
	b, _ := rewardRandomToken("rt_", 20)
	if a == b {
		t.Fatal("tickets collide")
	}
	now := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	reset := now.Add(time.Hour)
	if got := rewardTicketExpiry("browse", now, &reset); !got.Equal(reset) {
		t.Fatal(got)
	}
	if got := rewardTicketExpiry("register", now, nil); !got.Equal(now.Add(30 * 24 * time.Hour)) {
		t.Fatal(got)
	}
}

func TestRewardProgressSecondsRoundsAndCaps(t *testing.T) {
	for _, tc := range []struct {
		ms        int64
		browse    int
		completed bool
		want      int
	}{
		{0, 20, false, 0},
		{-5, 20, false, 0},
		{499, 20, false, 0},
		{500, 20, false, 1},
		// 固定 1 秒节拍下累计值贴着整秒边界抖动：14999 与 15003 必须显示成同一秒，不能「停一拍、跳两秒」。
		{14999, 20, false, 15},
		{15003, 20, false, 15},
		{15996, 20, false, 16},
		// 未完成时封顶 browse-1：19600ms 四舍五入是 20，但还没发奖，不能显示 20/20。
		{19600, 20, false, 19},
		{19999, 20, false, 19},
		{20004, 20, true, 20},
		{23000, 20, true, 20},
		// 非浏览任务（browse_seconds=0）不封顶。
		{1999, 0, true, 2},
		{86400123, 0, false, 86400},
	} {
		if got := rewardProgressSeconds(tc.ms, tc.browse, tc.completed); got != tc.want {
			t.Errorf("rewardProgressSeconds(%d, %d, %v) = %d, want %d", tc.ms, tc.browse, tc.completed, got, tc.want)
		}
	}
	record := &dbent.RewardTaskRecord{ProgressMs: 14999, Status: "in_progress"}
	if current := rewardCurrent(record, &dbent.RewardTask{VerifyMode: "sdk", BrowseSeconds: 20}); current.ProgressSeconds != 15 {
		t.Fatalf("rewardCurrent progress = %d, want 15", current.ProgressSeconds)
	}
}
