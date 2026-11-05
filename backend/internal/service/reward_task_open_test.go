package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

func TestRewardOpenSignature(t *testing.T) {
	now := time.Unix(1793850000, 0)
	body := []byte(`{"ticket":"rt_abc"}`)
	headers := RewardOpenSignature{AppKey: "rk_example", Timestamp: fmt.Sprint(now.Unix()), Nonce: "0123456789abcdef"}
	mac := hmac.New(sha256.New, []byte("secret"))
	_, _ = mac.Write([]byte(headers.Timestamp + "\n" + headers.Nonce + "\n" + string(body)))
	headers.Signature = hex.EncodeToString(mac.Sum(nil))
	for _, tc := range []struct {
		name    string
		headers RewardOpenSignature
		body    []byte
		now     time.Time
		reason  string
	}{
		{"valid", headers, body, now, ""},
		{"boundary", headers, body, now.Add(300 * time.Second), ""},
		{"tampered", headers, []byte(`{}`), now, "REWARD_SIGNATURE_INVALID"},
		{"expired", headers, body, now.Add(301 * time.Second), "REWARD_TIMESTAMP_EXPIRED"},
		{"future", headers, body, now.Add(-301 * time.Second), "REWARD_TIMESTAMP_EXPIRED"},
		{"missing", RewardOpenSignature{}, body, now, "REWARD_SIGNATURE_MISSING"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := infraerrors.Reason(verifyRewardOpenSignature(tc.headers, "secret", tc.body, tc.now)); got != tc.reason {
				t.Fatalf("reason=%q, want %q", got, tc.reason)
			}
		})
	}
}

func TestRewardOpenNonceReplayAndExpiry(t *testing.T) {
	store := &rewardNonceTestStore{now: time.Unix(1793850000, 0), keys: make(map[string]time.Time)}
	svc := &RewardTaskService{nonceStore: store}
	ctx := context.Background()
	if err := svc.consumeRewardOpenNonce(ctx, "rk_a", "0123456789abcdef"); err != nil {
		t.Fatal(err)
	}
	if got := infraerrors.Reason(svc.consumeRewardOpenNonce(ctx, "rk_a", "0123456789abcdef")); got != "REWARD_NONCE_REPLAYED" {
		t.Fatalf("reason=%q", got)
	}
	if err := svc.consumeRewardOpenNonce(ctx, "rk_b", "0123456789abcdef"); err != nil {
		t.Fatal(err)
	}
	store.now = store.now.Add(600 * time.Second)
	if err := svc.consumeRewardOpenNonce(ctx, "rk_a", "0123456789abcdef"); err != nil {
		t.Fatal(err)
	}
	if err := (&RewardTaskService{}).consumeRewardOpenNonce(ctx, "rk_a", "0123456789abcdef"); err != nil {
		t.Fatalf("nil Redis must fail open: %v", err)
	}
	store.err = errors.New("redis unavailable")
	if err := svc.consumeRewardOpenNonce(ctx, "rk_a", "0123456789abcdef"); err != nil {
		t.Fatalf("Redis error must fail open: %v", err)
	}
}

type rewardNonceTestStore struct {
	now  time.Time
	keys map[string]time.Time
	err  error
}

func (s *rewardNonceTestStore) ConsumeNonce(_ context.Context, key string, ttl time.Duration) (bool, error) {
	if s.err != nil {
		return false, s.err
	}
	if expires, exists := s.keys[key]; exists && expires.After(s.now) {
		return false, nil
	}
	s.keys[key] = s.now.Add(ttl)
	return true, nil
}

func TestRewardSDKAllowedDomains(t *testing.T) {
	for _, tc := range []struct {
		domains         []string
		origin, referer string
		want            bool
	}{
		{nil, "", "", true},
		{[]string{"example.com"}, "https://example.com:8443", "", true},
		{[]string{"example.com"}, "https://EXAMPLE.COM", "", true},
		{[]string{"example.com"}, "https://evil-example.com", "", false},
		{[]string{"*.example.com"}, "https://example.com", "", false},
		{[]string{"*.example.com"}, "https://a.b.example.com", "", true},
		{[]string{"*.example.com"}, "https://example.com.evil.test", "", false},
		{[]string{"example.com"}, "null", "https://example.com/", false},
		{[]string{"example.com"}, "", "https://example.com/path?q=1", true},
		{[]string{"example.com"}, "", "example.com/path", false},
		{[]string{"::1"}, "http://[::1]:8000", "", true},
	} {
		if got := rewardSDKOriginAllowed(tc.domains, tc.origin, tc.referer); got != tc.want {
			t.Errorf("domains=%v origin=%q referer=%q got=%v", tc.domains, tc.origin, tc.referer, got)
		}
	}
}

func TestRewardSDKHeartbeatDelta(t *testing.T) {
	now := time.Unix(1793850000, 0)
	for _, tc := range []struct {
		delta  time.Duration
		millis int64
		update bool
	}{
		{-time.Second, 0, false}, {0, 0, false},
		{799 * time.Millisecond, 0, false}, {800 * time.Millisecond, 800, true},
		{999 * time.Millisecond, 999, true}, {1500 * time.Millisecond, 1500, true},
		{3000 * time.Millisecond, 3000, true}, {3001 * time.Millisecond, 0, true},
		{time.Hour, 0, true},
	} {
		last := now.Add(-tc.delta)
		millis, update := rewardSDKHeartbeatDelta(&last, now)
		if int64(millis) != tc.millis || update != tc.update {
			t.Errorf("delta=%v got=(%d,%v), want=(%d,%v)", tc.delta, millis, update, tc.millis, tc.update)
		}
	}
	if millis, update := rewardSDKHeartbeatDelta(nil, now); millis != 0 || !update {
		t.Fatalf("first heartbeat=(%d,%v)", millis, update)
	}
}

func TestRewardSDKHeartbeatMillisecondAccumulationDoesNotLoseSeconds(t *testing.T) {
	now := time.Unix(1793850000, 0)
	last := now
	var total int64
	for i := 0; i < 10; i++ {
		now = now.Add(999 * time.Millisecond)
		millis, update := rewardSDKHeartbeatDelta(&last, now)
		if !update {
			t.Fatal("one-second heartbeat was ignored")
		}
		total += int64(millis)
		last = now
	}
	if total != 9990 || total/1000 != 9 {
		t.Fatalf("progress=%dms, want 9990ms/9s", total)
	}
}

func TestRewardSDKHeartbeatInterleavedTabsCannotExceedElapsedTime(t *testing.T) {
	start := time.Unix(1793850000, 0)
	for _, intervals := range [][]time.Duration{
		{100 * time.Millisecond},
		{799 * time.Millisecond, time.Millisecond},
		{time.Second, 10 * time.Millisecond, 100 * time.Millisecond, 20 * time.Millisecond},
		{900 * time.Millisecond, 200 * time.Millisecond, 4000 * time.Millisecond, 100 * time.Millisecond},
		{1 * time.Nanosecond, 3000 * time.Millisecond, 3001 * time.Millisecond, 800 * time.Millisecond},
	} {
		now := start
		var last *time.Time
		var total int64
		for i := 0; i < 10000; i++ {
			now = now.Add(intervals[i%len(intervals)])
			millis, update := rewardSDKHeartbeatDelta(last, now)
			total += int64(millis)
			if update {
				accepted := now
				last = &accepted
			}
			if total > now.Sub(start).Milliseconds() {
				t.Fatalf("multiple tabs accumulated %dms in %v", total, now.Sub(start))
			}
		}
	}
}

func TestRewardOpenEventAmountValidation(t *testing.T) {
	for _, amount := range []string{`12.34`, `"12.34"`, `0.01`, `"100"`, `1000000000000.00`, `"999999999999999999.99"`} {
		if _, err := parseRewardOpenEvent([]byte(`{"event_id":"a","event_type":"recharge","ticket":"rt_a","external_user_id":"u","amount":` + amount + `}`)); err != nil {
			t.Errorf("valid amount %s: %v", amount, err)
		}
	}
	for _, amount := range []string{`0`, `-1`, `0.001`, `"NaN"`, `"1e3"`, `null`, `true`, `"99999999999999999999"`} {
		if _, err := parseRewardOpenEvent([]byte(`{"event_id":"a","event_type":"recharge","ticket":"rt_a","external_user_id":"u","amount":` + amount + `}`)); infraerrors.Reason(err) != "REWARD_EVENT_INVALID" {
			t.Errorf("invalid amount %s: %v", amount, err)
		}
	}
	if _, err := parseRewardOpenEvent([]byte(`{"event_id":"a","event_type":"register","ticket":"rt_a","external_user_id":"u","amount":"ignored"}`)); err != nil {
		t.Fatalf("register ignores amount: %v", err)
	}
}

func TestRewardOpenDuplicateEventDoesNotMutateBalanceOrRecharge(t *testing.T) {
	for _, originalResult := range []string{"accepted", "completed", "ignored"} {
		t.Run(originalResult, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
			t.Cleanup(func() { _ = client.Close() })
			mock.ExpectBegin()
			mock.ExpectQuery(`SELECT pg_advisory_xact_lock`).WithArgs(sqlmock.AnyArg()).WillReturnRows(sqlmock.NewRows([]string{"pg_advisory_xact_lock"}).AddRow(nil))
			mock.ExpectQuery(`FROM "reward_task_events"`).WillReturnRows(sqlmock.NewRows([]string{"id", "partner_id", "event_id", "event_type", "record_id", "task_id", "result"}).AddRow(1, 7, "evt_1", "recharge", 21, 31, originalResult))
			mock.ExpectQuery(`FROM "reward_task_records"`).WillReturnRows(sqlmock.NewRows([]string{"id", "task_id", "user_id", "status", "recharge_total", "progress_ms"}).AddRow(21, 31, 41, "completed", 100.0, 1999))
			mock.ExpectQuery(`FROM "reward_tasks"`).WillReturnRows(sqlmock.NewRows([]string{"id", "title", "task_type", "min_recharge_amount"}).AddRow(31, "Recharge", "recharge", 100.0))
			mock.ExpectCommit()
			tx, err := client.Tx(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = tx.Rollback() }()
			// 重复事件即使携带不同票据/金额，也应返回原事件而不触碰新票据。
			input := &rewardOpenEventInput{EventID: "evt_1", EventType: "recharge", Ticket: "different_ticket", amountText: "999"}
			result, rewardedUserID, err := (&RewardTaskService{entClient: client}).processRewardOpenEventTx(context.Background(), tx, 7, nil, "127.0.0.1", input)
			if err != nil {
				t.Fatal(err)
			}
			if result.rejection != nil || !result.response.Duplicate || result.response.Result != originalResult || result.response.TaskID != 31 || result.response.Progress.RechargeTotal != 100 || result.response.Progress.ProgressSeconds != 2 || rewardedUserID != 0 {
				t.Fatalf("duplicate result=%+v rewardUserID=%d", result, rewardedUserID)
			}
			if err := tx.Commit(); err != nil {
				t.Fatal(err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatalf("idempotent request must execute no writes: %v", err)
			}
		})
	}
}
