package service

import (
	"context"
	"database/sql/driver"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	_ "github.com/Wei-Shaw/sub2api/ent/runtime"
	"github.com/Wei-Shaw/sub2api/internal/middleware"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9" //nolint:depguard // 用 miniredis 验证真实 Redis 原子限流及幂等协议。
	"github.com/stretchr/testify/require"
)

type ticketAgentTestSettings struct {
	SettingRepository
	values map[string]string
}

func (r ticketAgentTestSettings) GetMultiple(_ context.Context, _ []string) (map[string]string, error) {
	return r.values, nil
}

type ticketAgentTestUsers struct {
	UserRepository
	u     *User
	err   error
	calls int
}

func (r *ticketAgentTestUsers) GetByID(_ context.Context, _ int64) (*User, error) {
	r.calls++
	return r.u, r.err
}
func (r *ticketAgentTestUsers) GetByIDIncludeDeleted(ctx context.Context, id int64) (*User, error) {
	return r.GetByID(ctx, id)
}

func ticketAgentFixture(t *testing.T) (*TicketAgentService, sqlmock.Sqlmock, *miniredis.Miniredis, *ticketAgentTestUsers) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	t.Cleanup(func() { _ = client.Close() })
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr(), MaxRetries: -1})
	t.Cleanup(func() { _ = rdb.Close() })
	settings := DefaultTicketSettings()
	settings.Enabled = true
	encoded, err := json.Marshal(settings)
	require.NoError(t, err)
	st := &SettingService{settingRepo: ticketAgentTestSettings{values: map[string]string{settingKeyTicketSettings: string(encoded), SettingKeyTicketSystemEnabled: "true"}}}
	users := &ticketAgentTestUsers{u: &User{ID: 42, Role: RoleUser, Username: "owner", Status: StatusActive, Email: "owner@example.test"}}
	svc := &TicketAgentService{entClient: client, userRepo: users, settings: st, redis: rdb, limiter: middleware.NewRateLimiter(rdb)}
	return svc, mock, mr, users
}
func ticketAgentExpectKey(mock sqlmock.Sqlmock, id int64, scope, status string, qps int, expires *time.Time) {
	mock.ExpectQuery(`FROM "ticket_agent_keys"`).WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "name", "key_prefix", "scope", "qps_limit", "status", "expires_at"}).AddRow(id, 42, "test", "lxt_12345678", scope, qps, status, expires))
}
func ticketAgentSecret(t *testing.T) string {
	t.Helper()
	s, _, e := makeSecret()
	require.NoError(t, e)
	return s
}
func TestTicketAgentSecretAndJSONContract(t *testing.T) {
	secret, hash, err := makeSecret()
	require.NoError(t, err)
	require.True(t, parseAgentSecret(secret))
	require.Equal(t, 47, len(secret))
	require.Len(t, hash, 64)
	require.Equal(t, shaHash(secret), hash)
	decoded, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(secret, "lxt_"))
	require.NoError(t, err)
	require.Len(t, decoded, 32)
	other, otherHash, err := makeSecret()
	require.NoError(t, err)
	require.NotEqual(t, secret, other)
	require.NotEqual(t, hash, otherHash)
	for _, s := range []string{"", "lxt_test", secret + "=", "not_" + secret[4:], secret + "A"} {
		require.False(t, parseAgentSecret(s), s)
	}
	out := viewKey(&dbent.TicketAgentKey{ID: 1, Name: "test", KeyHash: hash, KeyPrefix: secret[:12], Scope: "user", Status: "active", QPSLimit: 2})
	raw, err := json.Marshal(out)
	require.NoError(t, err)
	var fields map[string]any
	require.NoError(t, json.Unmarshal(raw, &fields))
	require.Len(t, fields, 10)
	require.Nil(t, fields["last_used_at"])
	require.Nil(t, fields["last_used_ip"])
	require.Nil(t, fields["expires_at"])
	require.NotContains(t, string(raw), hash)
	require.NotContains(t, string(raw), secret)
}
func TestTicketAgentIPFailuresAndFailClosed(t *testing.T) {
	s, mock, mr, _ := ticketAgentFixture(t)
	ctx := context.Background()
	for i := 0; i < 20; i++ {
		_, _, err := s.Authenticate(ctx, "", "192.0.2.1")
		require.Equal(t, "AGENT_KEY_MISSING", infraerrors.Reason(err))
	}
	// 格式错误即使达到封禁阈值也固定 401，并且不查询数据库。
	_, _, err := s.Authenticate(ctx, "bad", "192.0.2.1")
	require.Equal(t, "AGENT_KEY_INVALID", infraerrors.Reason(err))
	count, err := mr.Get("ticket_agent_auth_fail:192.0.2.1")
	require.NoError(t, err)
	require.Equal(t, "21", count)
	mock.ExpectQuery(`FROM "ticket_agent_keys"`).WillReturnRows(sqlmock.NewRows([]string{"id"}))
	_, rate, err := s.Authenticate(ctx, ticketAgentSecret(t), "192.0.2.1")
	require.Equal(t, "AGENT_AUTH_BLOCKED", infraerrors.Reason(err))
	require.Greater(t, rate.RetryAfter, time.Duration(0))
	count, err = mr.Get("ticket_agent_auth_fail:192.0.2.1")
	require.NoError(t, err)
	require.Equal(t, "22", count)
	require.NoError(t, mock.ExpectationsWereMet())
	mr.FastForward(time.Minute)
	_, _, err = s.Authenticate(ctx, "bad", "192.0.2.1")
	require.Equal(t, "AGENT_KEY_INVALID", infraerrors.Reason(err))
	mr.Close()
	_, _, err = s.Authenticate(ctx, "bad", "192.0.2.2")
	require.Equal(t, "AGENT_RATE_LIMIT_UNAVAILABLE", infraerrors.Reason(err))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTicketAgentBlockedIPAllowsValidKey(t *testing.T) {
	s, mock, mr, users := ticketAgentFixture(t)
	ip := "192.0.2.1"
	ctx := context.Background()
	require.NoError(t, mr.Set("ticket_agent_auth_fail:"+ip, "20"))
	mr.SetTTL("ticket_agent_auth_fail:"+ip, time.Minute)
	secret := ticketAgentSecret(t)
	// 封禁计数不影响合法密钥；每个请求仍重新查库并执行两级限流。
	for i := 0; i < 3; i++ {
		ticketAgentExpectKey(mock, 1, "user", "active", 2, nil)
		p, rate, err := s.Authenticate(ctx, secret, ip)
		if i < 2 {
			require.NoError(t, err)
			require.Equal(t, int64(42), p.Owner.ID)
			require.Equal(t, 1-i, rate.Remaining)
		} else {
			require.Equal(t, "AGENT_RATE_LIMITED", infraerrors.Reason(err))
		}
	}
	count, err := mr.Get("ticket_agent_auth_fail:" + ip)
	require.NoError(t, err)
	require.Equal(t, "20", count)
	require.Equal(t, 3, users.calls)
	// 无效密钥仍查库、返回 429，且持续计入当前失败窗口。
	for i := 0; i < 2; i++ {
		mock.ExpectQuery(`FROM "ticket_agent_keys"`).WillReturnRows(sqlmock.NewRows([]string{"id"}))
		_, rate, err := s.Authenticate(ctx, ticketAgentSecret(t), ip)
		require.Equal(t, "AGENT_AUTH_BLOCKED", infraerrors.Reason(err))
		require.Equal(t, 20, rate.Limit)
		require.Greater(t, rate.RetryAfter, time.Duration(0))
	}
	count, err = mr.Get("ticket_agent_auth_fail:" + ip)
	require.NoError(t, err)
	require.Equal(t, "22", count)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTicketAgentValidKeyFailsClosedOnRedisFailure(t *testing.T) {
	s, mock, mr, _ := ticketAgentFixture(t)
	ticketAgentExpectKey(mock, 1, "user", "active", 2, nil)
	mr.SetError("ERR forced Redis failure")
	_, _, err := s.Authenticate(context.Background(), ticketAgentSecret(t), "192.0.2.1")
	require.Equal(t, "AGENT_RATE_LIMIT_UNAVAILABLE", infraerrors.Reason(err))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTicketAgentCreatePersistsHashOnly(t *testing.T) {
	s, mock, _, _ := ticketAgentFixture(t)
	var savedHash string
	mock.ExpectBegin()
	mock.ExpectExec(`SELECT pg_advisory_xact_lock`).WithArgs("ticket-agent-key:42").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`COUNT.*ticket_agent_keys`).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery(`INSERT INTO "ticket_agent_keys"`).WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), int64(42), "my-agent", sqlmock.AnyArg(), ticketAgentHashMatcher{hash: &savedHash}, "user", 2, "active").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(7))
	mock.ExpectCommit()
	created, err := s.CreateKey(context.Background(), 42, TicketAgentKeyCreate{Name: "my-agent"})
	require.NoError(t, err)
	require.Equal(t, int64(7), created.ID)
	require.Equal(t, created.Secret[:12], created.KeyPrefix)
	require.Equal(t, shaHash(created.Secret), savedHash)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTicketAgentKeyIsolation(t *testing.T) {
	s, mock, _, _ := ticketAgentFixture(t)
	// 个人接口的查询必须包含所有者，跨用户访问不泄露密钥是否存在。
	mock.ExpectQuery(`FROM "ticket_agent_keys".*"user_id" = \$2`).WithArgs(int64(7), int64(999)).WillReturnRows(sqlmock.NewRows([]string{"id"}))
	_, err := s.UpdateKey(context.Background(), 999, 7, TicketAgentKeyUpdate{}, false)
	require.Equal(t, "TICKET_NOT_FOUND", infraerrors.Reason(err))
	mock.ExpectQuery(`FROM "ticket_agent_keys".*"user_id" = \$2`).WithArgs(int64(7), int64(999)).WillReturnRows(sqlmock.NewRows([]string{"id"}))
	err = s.DeleteKey(context.Background(), 999, 7, false)
	require.Equal(t, "TICKET_NOT_FOUND", infraerrors.Reason(err))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTicketAgentUsageWorkerLifecycle(t *testing.T) {
	s := NewTicketAgentService(nil, nil, nil, nil)
	s.usedCh <- usedRecord{id: 1}
	done := make(chan struct{})
	go func() { s.Stop(); s.Stop(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("usage worker did not stop")
	}
	s.enqueueLastUsed(&dbent.TicketAgentKey{ID: 1}, "192.0.2.1")
	require.Empty(t, s.usedCh)
}
func TestTicketAgentChecksKeyAndOwnerEveryRequest(t *testing.T) {
	cases := []struct {
		name, scope, status, role, userStatus, want string
		expired                                     bool
	}{
		{name: "disabled key", scope: "user", status: "disabled", role: RoleUser, userStatus: StatusActive, want: "AGENT_KEY_DISABLED"},
		{name: "expired key", scope: "user", status: "active", role: RoleUser, userStatus: StatusActive, want: "AGENT_KEY_EXPIRED", expired: true},
		{name: "disabled owner", scope: "user", status: "active", role: RoleUser, userStatus: "disabled", want: "AGENT_KEY_INVALID"},
		{name: "admin downgraded", scope: "admin", status: "active", role: RoleUser, userStatus: StatusActive, want: "AGENT_SCOPE_FORBIDDEN"},
		{name: "unknown scope", scope: "operator", status: "active", role: RoleAdmin, userStatus: StatusActive, want: "AGENT_KEY_INVALID"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s, mock, _, users := ticketAgentFixture(t)
			users.u.Role = c.role
			users.u.Status = c.userStatus
			var expires *time.Time
			if c.expired {
				v := time.Now().Add(-time.Minute)
				expires = &v
			}
			ticketAgentExpectKey(mock, 1, c.scope, c.status, 2, expires)
			_, _, err := s.Authenticate(context.Background(), ticketAgentSecret(t), "192.0.2.1")
			require.Equal(t, c.want, infraerrors.Reason(err))
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
	t.Run("admin downgrade immediate", func(t *testing.T) {
		s, mock, _, users := ticketAgentFixture(t)
		users.u.Role = RoleAdmin
		secret := ticketAgentSecret(t)
		ticketAgentExpectKey(mock, 1, "admin", "active", 2, nil)
		_, _, err := s.Authenticate(context.Background(), secret, "192.0.2.1")
		require.NoError(t, err)
		users.u.Role = RoleUser
		ticketAgentExpectKey(mock, 1, "admin", "active", 2, nil)
		_, _, err = s.Authenticate(context.Background(), secret, "192.0.2.1")
		require.Equal(t, "AGENT_SCOPE_FORBIDDEN", infraerrors.Reason(err))
		require.Equal(t, 2, users.calls)
		require.NoError(t, mock.ExpectationsWereMet())
	})
	t.Run("revoked immediate", func(t *testing.T) {
		s, mock, _, _ := ticketAgentFixture(t)
		secret := ticketAgentSecret(t)
		ticketAgentExpectKey(mock, 1, "user", "active", 2, nil)
		_, _, err := s.Authenticate(context.Background(), secret, "192.0.2.1")
		require.NoError(t, err)
		mock.ExpectQuery(`FROM "ticket_agent_keys"`).WillReturnRows(sqlmock.NewRows([]string{"id"}))
		_, _, err = s.Authenticate(context.Background(), secret, "192.0.2.1")
		require.Equal(t, "AGENT_KEY_INVALID", infraerrors.Reason(err))
		require.NoError(t, mock.ExpectationsWereMet())
	})
}
func TestTicketAgentQPSAndOwnerAggregate(t *testing.T) {
	s, mock, mr, _ := ticketAgentFixture(t)
	ctx := context.Background()
	secret := ticketAgentSecret(t)
	for i := 0; i < 3; i++ {
		ticketAgentExpectKey(mock, 1, "user", "active", 2, nil)
		_, rate, err := s.Authenticate(ctx, secret, "192.0.2.1")
		if i < 2 {
			require.NoError(t, err)
			require.Equal(t, 1-i, rate.Remaining)
		} else {
			require.Equal(t, "AGENT_RATE_LIMITED", infraerrors.Reason(err))
			require.Equal(t, 0, rate.Remaining)
			require.Greater(t, rate.RetryAfter, time.Duration(0))
		}
	}
	mr.FastForward(time.Second)
	for i := int64(0); i < 6; i++ {
		ticketAgentExpectKey(mock, 10+i, "user", "active", 10, nil)
		_, rate, err := s.Authenticate(ctx, secret, "192.0.2.1")
		if i < 5 {
			require.NoError(t, err)
		} else {
			require.Equal(t, "AGENT_RATE_LIMITED", infraerrors.Reason(err))
			require.Equal(t, 5, rate.Limit)
		}
	}
	require.NoError(t, mock.ExpectationsWereMet())
}
func TestTicketAgentIdempotency(t *testing.T) {
	s, _, mr, _ := ticketAgentFixture(t)
	ctx := context.Background()
	var calls atomic.Int32
	fn := func(context.Context) (int, []byte, error) {
		id := calls.Add(1)
		return 201, []byte(fmt.Sprintf(`{"id":%d}`, id)), nil
	}
	status, data, err := s.RunIdempotent(ctx, 1, "token", "POST /tickets body1", fn)
	require.NoError(t, err)
	require.Equal(t, 201, status)
	require.JSONEq(t, `{"id":1}`, string(data))
	redisKey := fmt.Sprintf("ticket_agent:idempotency:%d:%s", 1, shaHash("token"))
	require.Equal(t, "done", mr.HGet(redisKey, "state"))
	require.Equal(t, ticketAgentIdempotencyDoneTTL, mr.TTL(redisKey))
	_, data, err = s.RunIdempotent(ctx, 1, "token", "POST /tickets body1", fn)
	require.NoError(t, err)
	require.JSONEq(t, `{"id":1}`, string(data))
	require.Equal(t, int32(1), calls.Load())
	_, _, err = s.RunIdempotent(ctx, 1, "token", "POST /tickets body2", fn)
	require.Equal(t, "TICKET_INVALID_INPUT", infraerrors.Reason(err))
	require.Equal(t, int32(1), calls.Load())
	_, _, err = s.RunIdempotent(ctx, 2, "token", "POST /tickets body1", fn)
	require.NoError(t, err)
	require.Equal(t, int32(2), calls.Load())
	mr.FastForward(24 * time.Hour)
	_, _, err = s.RunIdempotent(ctx, 1, "token", "POST /tickets body1", fn)
	require.NoError(t, err)
	require.Equal(t, int32(3), calls.Load())
	mr.Close()
	_, _, err = s.RunIdempotent(ctx, 1, "other", "POST /tickets", fn)
	require.Equal(t, "AGENT_RATE_LIMIT_UNAVAILABLE", infraerrors.Reason(err))
	require.Equal(t, int32(3), calls.Load())
}
func TestTicketAgentIdempotencyConcurrentAndFailure(t *testing.T) {
	s, _, _, _ := ticketAgentFixture(t)
	var calls atomic.Int32
	fn := func(context.Context) (int, []byte, error) {
		calls.Add(1)
		time.Sleep(80 * time.Millisecond)
		return 201, []byte(`{"id":1}`), nil
	}
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			status, body, err := s.RunIdempotent(context.Background(), 1, "same", "create", fn)
			require.NoError(t, err)
			require.Equal(t, 201, status)
			require.JSONEq(t, `{"id":1}`, string(body))
		}()
	}
	wg.Wait()
	require.Equal(t, int32(1), calls.Load())
	_, _, err := s.RunIdempotent(context.Background(), 1, "failure", "create", func(context.Context) (int, []byte, error) { return 0, nil, errors.New("failed before commit") })
	require.Error(t, err)
	_, _, err = s.RunIdempotent(context.Background(), 1, "failure", "create", fn)
	require.NoError(t, err)
	require.Equal(t, int32(2), calls.Load())
}

func TestTicketAgentIdempotencyCanceledContext(t *testing.T) {
	for _, success := range []bool{false, true} {
		t.Run(fmt.Sprintf("success=%t", success), func(t *testing.T) {
			s, _, mr, _ := ticketAgentFixture(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			key := fmt.Sprintf("ticket_agent:idempotency:%d:%s", 1, shaHash("canceled"))
			calls := 0
			fn := func(context.Context) (int, []byte, error) {
				calls++
				require.Equal(t, ticketAgentIdempotencyPendingTTL, mr.TTL(key))
				cancel()
				if !success {
					return 0, nil, context.Canceled
				}
				return 201, []byte(`{"id":1}`), nil
			}
			_, _, err := s.RunIdempotent(ctx, 1, "canceled", "create", fn)
			if success {
				require.NoError(t, err)
				require.Equal(t, "done", mr.HGet(key, "state"))
				require.Equal(t, ticketAgentIdempotencyDoneTTL, mr.TTL(key))
			} else {
				require.ErrorIs(t, err, context.Canceled)
				require.False(t, mr.Exists(key), "canceled request must release its pending lease")
			}
			_, _, err = s.RunIdempotent(context.Background(), 1, "canceled", "create", func(context.Context) (int, []byte, error) {
				calls++
				return 201, []byte(`{"id":1}`), nil
			})
			require.NoError(t, err)
			if success {
				require.Equal(t, 1, calls, "persisted result must survive request cancellation")
			} else {
				require.Equal(t, 2, calls)
			}
		})
	}
}

func TestTicketAgentIdempotencyRedisWriteFailureLeaseRecovery(t *testing.T) {
	for _, success := range []bool{false, true} {
		t.Run(fmt.Sprintf("success=%t", success), func(t *testing.T) {
			s, _, mr, _ := ticketAgentFixture(t)
			key := fmt.Sprintf("ticket_agent:idempotency:%d:%s", 1, shaHash("redis-failure"))
			businessErr := errors.New("business failed")
			_, _, err := s.RunIdempotent(context.Background(), 1, "redis-failure", "create", func(context.Context) (int, []byte, error) {
				require.Equal(t, ticketAgentIdempotencyPendingTTL, mr.TTL(key))
				mr.SetError("ERR forced Redis write failure")
				if !success {
					return 0, nil, businessErr
				}
				return 201, []byte(`{"id":1}`), nil
			})
			if success {
				require.Equal(t, "AGENT_RATE_LIMIT_UNAVAILABLE", infraerrors.Reason(err))
			} else {
				require.ErrorIs(t, err, businessErr)
			}
			mr.SetError("")
			require.Equal(t, "pending", mr.HGet(key, "state"))
			require.Equal(t, ticketAgentIdempotencyPendingTTL, mr.TTL(key))
			mr.FastForward(ticketAgentIdempotencyPendingTTL)
			require.False(t, mr.Exists(key))
			calls := 0
			_, _, err = s.RunIdempotent(context.Background(), 1, "redis-failure", "create", func(context.Context) (int, []byte, error) {
				calls++
				return 201, []byte(`{"id":2}`), nil
			})
			require.NoError(t, err)
			require.Equal(t, 1, calls)
			require.Equal(t, "done", mr.HGet(key, "state"))
			require.Equal(t, ticketAgentIdempotencyDoneTTL, mr.TTL(key))
		})
	}
}

func TestTicketAgentIdempotencyLeaseOwnerFencing(t *testing.T) {
	s, _, mr, _ := ticketAgentFixture(t)
	ctx := context.Background()
	redisKey := fmt.Sprintf("ticket_agent:idempotency:%d:%s", 1, shaHash("owner-fencing"))
	fingerprint := shaHash("create")
	claim := func(owner string) {
		values, err := ticketAgentIdempotencyClaim.Run(ctx, s.redis, []string{redisKey}, fingerprint, owner, int(ticketAgentIdempotencyPendingTTL/time.Second)).StringSlice()
		require.NoError(t, err)
		require.Equal(t, []string{"new", ""}, values)
	}
	claim("old-owner")
	require.Equal(t, ticketAgentIdempotencyPendingTTL, mr.TTL(redisKey))
	mr.FastForward(ticketAgentIdempotencyPendingTTL)
	claim("new-owner")
	result, err := ticketAgentIdempotencyRelease.Run(ctx, s.redis, []string{redisKey}, fingerprint, "old-owner").Int()
	require.NoError(t, err)
	require.Zero(t, result)
	raw := `{"status":201,"data":{"id":1}}`
	result, err = ticketAgentIdempotencyComplete.Run(ctx, s.redis, []string{redisKey}, fingerprint, "old-owner", raw, int(ticketAgentIdempotencyDoneTTL/time.Second)).Int()
	require.NoError(t, err)
	require.Zero(t, result)
	require.Equal(t, "new-owner", mr.HGet(redisKey, "owner"))
	require.Equal(t, "pending", mr.HGet(redisKey, "state"))
	require.Equal(t, ticketAgentIdempotencyPendingTTL, mr.TTL(redisKey))
	result, err = ticketAgentIdempotencyComplete.Run(ctx, s.redis, []string{redisKey}, fingerprint, "new-owner", raw, int(ticketAgentIdempotencyDoneTTL/time.Second)).Int()
	require.NoError(t, err)
	require.Equal(t, 1, result)
	require.Equal(t, "done", mr.HGet(redisKey, "state"))
	require.Equal(t, ticketAgentIdempotencyDoneTTL, mr.TTL(redisKey))
	// 即使是当前 owner，完成后也不能再释放或覆盖结果。
	result, err = ticketAgentIdempotencyRelease.Run(ctx, s.redis, []string{redisKey}, fingerprint, "new-owner").Int()
	require.NoError(t, err)
	require.Zero(t, result)
	result, err = ticketAgentIdempotencyComplete.Run(ctx, s.redis, []string{redisKey}, fingerprint, "new-owner", `{"status":201,"data":{"id":2}}`, int(ticketAgentIdempotencyDoneTTL/time.Second)).Int()
	require.NoError(t, err)
	require.Zero(t, result)
	require.Equal(t, raw, mr.HGet(redisKey, "result"))
}

type ticketAgentHashMatcher struct{ hash *string }

func (m ticketAgentHashMatcher) Match(v driver.Value) bool {
	val, ok := v.(string)
	if !ok || len(val) != 64 {
		return false
	}
	*m.hash = val
	return true
}
func TestTicketAgentCreateValidationAndLimit(t *testing.T) {
	s, mock, _, users := ticketAgentFixture(t)
	ctx := context.Background()
	_, err := s.CreateKey(ctx, 42, TicketAgentKeyCreate{Name: "secret", Scope: "admin"})
	require.Equal(t, "AGENT_SCOPE_FORBIDDEN", infraerrors.Reason(err))
	users.u.Role = RoleAdmin
	_, err = s.CreateKey(ctx, 42, TicketAgentKeyCreate{Name: "invalid\x00name"})
	require.Equal(t, "TICKET_INVALID_INPUT", infraerrors.Reason(err))
	zero := 0
	_, err = s.CreateKey(ctx, 42, TicketAgentKeyCreate{Name: "secret", QPSLimit: &zero})
	require.Equal(t, "AGENT_QPS_INVALID", infraerrors.Reason(err))
	mock.ExpectBegin()
	mock.ExpectExec(`SELECT pg_advisory_xact_lock`).WithArgs("ticket-agent-key:42").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`COUNT.*ticket_agent_keys`).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(5))
	mock.ExpectRollback()
	_, err = s.CreateKey(ctx, 42, TicketAgentKeyCreate{Name: "secret"})
	require.Equal(t, "AGENT_KEY_LIMIT", infraerrors.Reason(err))
	require.NoError(t, mock.ExpectationsWereMet())
}
