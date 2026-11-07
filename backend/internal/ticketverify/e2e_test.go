//go:build ticket_verify

package ticketverify

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	_ "github.com/Wei-Shaw/sub2api/ent/runtime"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/middleware"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/repository"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

// 显式启动独立临时容器，测试完成后仅删除本次创建的 loxo-ticket-* 容器。
// 运行：TICKET_VERIFY_DOCKER=1 go test -tags=ticket_verify -v ./internal/ticketverify
// 也可提供 TICKET_VERIFY_DSN，连接调用者指定的独立测试库。
func ticketVerifyDSN(t *testing.T) string {
	t.Helper()
	if dsn := os.Getenv("TICKET_VERIFY_DSN"); dsn != "" {
		return dsn
	}
	if os.Getenv("TICKET_VERIFY_DOCKER") != "1" {
		t.Skip("set TICKET_VERIFY_DOCKER=1 or TICKET_VERIFY_DSN to run isolated PostgreSQL tests")
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := listener.Addr().(*net.TCPAddr).Port
	require.NoError(t, listener.Close())
	require.NotContains(t, []int{5432, 6379}, port)
	name := fmt.Sprintf("loxo-ticket-%d", time.Now().UnixNano())
	image := os.Getenv("TICKET_VERIFY_POSTGRES_IMAGE")
	if image == "" {
		image = "postgres:16-alpine"
	}
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, "docker", "run", "--rm", "-d", "--name", name,
		"-e", "POSTGRES_USER=ticket_verify", "-e", "POSTGRES_PASSWORD=ticket_verify", "-e", "POSTGRES_DB=ticket_verify",
		"-p", "127.0.0.1:"+strconv.Itoa(port)+":5432", image).CombinedOutput()
	require.NoError(t, err, "docker run: %s", output)
	t.Cleanup(func() {
		cleanup, stop := context.WithTimeout(context.Background(), 15*time.Second)
		defer stop()
		out, err := exec.CommandContext(cleanup, "docker", "stop", "--time", "1", name).CombinedOutput()
		if err != nil {
			t.Logf("stop temporary container %s: %v %s", name, err, out)
		}
	})
	dsn := fmt.Sprintf("host=127.0.0.1 port=%d user=ticket_verify password=ticket_verify dbname=ticket_verify sslmode=disable", port)
	probe, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer func() { _ = probe.Close() }()
	deadline := time.Now().Add(45 * time.Second)
	for {
		attempt, done := context.WithTimeout(t.Context(), time.Second)
		err = probe.PingContext(attempt)
		done()
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("PostgreSQL did not become ready: %v", err)
		}
		time.Sleep(250 * time.Millisecond)
	}
	t.Logf("isolated PostgreSQL ready: container=%s port=%d", name, port)
	return dsn
}

// 记录真实密钥表查询次数；系统开关仍需从 settings 读取。
type ticketCountingDriver struct {
	dialect.Driver
	queries atomic.Int64
}

func (d *ticketCountingDriver) Query(ctx context.Context, query string, args, value any) error {
	if strings.Contains(query, `"ticket_agent_keys"`) {
		d.queries.Add(1)
	}
	return d.Driver.Query(ctx, query, args, value)
}

func TestTicketsAgainstPostgres(t *testing.T) {
	dsn := ticketVerifyDSN(t)
	ctx := t.Context()
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, repository.ApplyMigrations(ctx, db))
	countingDriver := &ticketCountingDriver{Driver: entsql.OpenDB(dialect.Postgres, db)}
	client := dbent.NewClient(dbent.Driver(countingDriver))
	t.Cleanup(func() { _ = client.Close() })
	settings := service.NewSettingService(repository.NewSettingRepository(client), &config.Config{})
	enabled := true
	_, err = settings.UpdateTicketSettings(ctx, service.TicketSettingsUpdate{Enabled: &enabled})
	require.NoError(t, err)
	mini := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: mini.Addr()})
	t.Cleanup(func() { _ = redisClient.Close() })
	svc := service.NewTicketService(db, settings, middleware.NewRateLimiter(redisClient), nil)
	users := repository.NewUserRepository(client, db)
	agent := service.NewTicketAgentService(client, users, settings, redisClient)
	t.Cleanup(agent.Stop)
	suffix := fmt.Sprint(time.Now().UnixNano())
	user, err := client.User.Create().SetEmail("ticket-" + suffix + "@example.com").SetUsername("ticket-user").SetPasswordHash("x").Save(ctx)
	require.NoError(t, err)
	other, err := client.User.Create().SetEmail("ticket-other-" + suffix + "@example.com").SetPasswordHash("x").Save(ctx)
	require.NoError(t, err)
	admin, err := client.User.Create().SetEmail("ticket-admin-" + suffix + "@example.com").SetUsername("agent-admin").SetPasswordHash("x").SetRole("admin").Save(ctx)
	require.NoError(t, err)
	created, err := svc.CreateTicket(ctx, user.ID, "web", service.TicketCreateInput{Subject: "真实工单", Category: "billing", Priority: "normal", Body: "第一条消息"}, nil)
	require.NoError(t, err)
	id := created.Ticket.ID
	require.Regexp(t, `^TK[0-9]{8}[A-Z0-9]{6}$`, created.Ticket.TicketNo)
	require.Equal(t, "pending_admin", created.Ticket.Status)
	require.Len(t, created.Messages, 1)
	_, err = svc.GetTicket(ctx, other.ID, id, false, 0)
	require.Equal(t, "TICKET_NOT_FOUND", infraerrors.Reason(err))

	adminView, err := svc.GetAdminTicket(ctx, id)
	require.NoError(t, err)
	require.False(t, adminView.Ticket.AdminUnread)
	internal, err := svc.AddMessage(ctx, admin.ID, id, true, "web", nil, service.TicketMessageInput{Body: "内部备注", Internal: true})
	require.NoError(t, err)
	require.True(t, internal.Internal)
	privateView, err := svc.GetTicket(ctx, user.ID, id, false, 0)
	require.NoError(t, err)
	require.Len(t, privateView.Messages, 1)
	require.Equal(t, "pending_admin", privateView.Ticket.Status)
	require.False(t, privateView.Ticket.Unread)
	public, err := svc.AddMessage(ctx, admin.ID, id, true, "web", nil, service.TicketMessageInput{Body: "公开回复"})
	require.NoError(t, err)
	var unread bool
	require.NoError(t, db.QueryRowContext(ctx, "SELECT user_unread FROM tickets WHERE id=$1", id).Scan(&unread))
	require.True(t, unread)
	privateView, err = svc.GetTicket(ctx, user.ID, id, false, 0)
	require.NoError(t, err)
	require.Equal(t, "pending_user", privateView.Ticket.Status)
	require.Len(t, privateView.Messages, 2)
	require.Nil(t, privateView.Messages[1].AuthorID)
	require.Equal(t, "Support", privateView.Messages[1].AuthorName)
	incremental, err := svc.GetTicket(ctx, user.ID, id, false, public.ID-1)
	require.NoError(t, err)
	require.Len(t, incremental.Messages, 1)
	require.Equal(t, public.ID, incremental.Messages[0].ID)

	closed, err := svc.CloseReopen(ctx, user.ID, id, false)
	require.NoError(t, err)
	require.Equal(t, "closed", closed.Status)
	require.NotNil(t, closed.ClosedAt)
	_, err = svc.AddMessage(ctx, user.ID, id, false, "web", nil, service.TicketMessageInput{Body: "关闭后回复"})
	require.Equal(t, "TICKET_CLOSED", infraerrors.Reason(err))
	reopened, err := svc.CloseReopen(ctx, user.ID, id, true)
	require.NoError(t, err)
	require.Equal(t, "pending_admin", reopened.Status)
	require.Nil(t, reopened.ClosedAt)
	_, err = svc.CloseReopen(ctx, user.ID, id, true)
	require.Equal(t, "TICKET_INVALID_TRANSITION", infraerrors.Reason(err))
	resolved := "resolved"
	updated, err := svc.UpdateAdmin(ctx, id, service.TicketUpdateInput{Status: &resolved, AssigneeID: &admin.ID}, admin.ID)
	require.NoError(t, err)
	require.Equal(t, "resolved", updated.Status)
	privateView, err = svc.GetTicket(ctx, user.ID, id, false, 0)
	require.NoError(t, err)
	last := privateView.Messages[len(privateView.Messages)-1]
	require.Equal(t, "system", last.AuthorType)
	require.Equal(t, "status_changed:resolved", last.Body)
	_, err = svc.AddMessage(ctx, user.ID, id, false, "web", nil, service.TicketMessageInput{Body: "问题未解决"})
	require.NoError(t, err)
	privateView, err = svc.GetTicket(ctx, user.ID, id, false, 0)
	require.NoError(t, err)
	require.Equal(t, "pending_admin", privateView.Ticket.Status)
	adminView, err = svc.GetAdminTicket(ctx, id)
	require.NoError(t, err)
	encoded, err := json.Marshal(adminView)
	require.NoError(t, err)
	require.Contains(t, string(encoded), `"user_email"`)
	require.Contains(t, string(encoded), `"assignee_name"`)
	require.Contains(t, string(encoded), `"admin_unread"`)

	key, err := agent.CreateKey(ctx, user.ID, service.TicketAgentKeyCreate{Name: "verify", Scope: "user"})
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(key.Secret, "lxt_"))
	principal, _, err := agent.Authenticate(ctx, key.Secret, "192.0.2.1")
	require.NoError(t, err)
	require.Equal(t, user.ID, principal.Owner.ID)
	stored, err := client.TicketAgentKey.Get(ctx, key.ID)
	require.NoError(t, err)
	require.NotEqual(t, key.Secret, stored.KeyHash)
	require.Len(t, stored.KeyHash, 64)
	_, err = agent.CreateKey(ctx, user.ID, service.TicketAgentKeyCreate{Name: "invalid-admin", Scope: "admin"})
	require.Equal(t, "AGENT_SCOPE_FORBIDDEN", infraerrors.Reason(err))
	adminKey, err := agent.CreateKey(ctx, admin.ID, service.TicketAgentKeyCreate{Name: "verify-admin", Scope: "admin"})
	require.NoError(t, err)
	_, _, err = agent.Authenticate(ctx, adminKey.Secret, "192.0.2.2")
	require.NoError(t, err)
	require.NoError(t, client.User.UpdateOneID(admin.ID).SetRole("user").Exec(ctx))
	_, _, err = agent.Authenticate(ctx, adminKey.Secret, "192.0.2.2")
	require.Equal(t, "AGENT_SCOPE_FORBIDDEN", infraerrors.Reason(err))
	require.NoError(t, agent.DeleteKey(ctx, user.ID, key.ID, false))
	_, _, err = agent.Authenticate(ctx, key.Secret, "192.0.2.1")
	require.Error(t, err)
	t.Run("authentication failures never deny valid shared IP keys", func(t *testing.T) {
		failedLimit := 2
		_, err := settings.UpdateTicketSettings(ctx, service.TicketSettingsUpdate{AgentFailedAuthPerMinutePerIP: &failedLimit})
		require.NoError(t, err)
		active, err := agent.CreateKey(ctx, user.ID, service.TicketAgentKeyCreate{Name: "failure-regression", Scope: "user"})
		require.NoError(t, err)
		sourceIP := "198.51.100.42"
		for _, failure := range []struct{ secret, reason string }{{"", "AGENT_KEY_MISSING"}, {"wrong-format", "AGENT_KEY_INVALID"}} {
			queriesBefore := countingDriver.queries.Load()
			_, _, err = agent.Authenticate(ctx, failure.secret, sourceIP)
			require.Equal(t, queriesBefore, countingDriver.queries.Load(), "malformed credentials must not query the key table")
			require.Equal(t, failure.reason, infraerrors.Reason(err))
			require.Equal(t, 401, infraerrors.Code(err))
		}
		before, err := redisClient.Get(ctx, "ticket_agent_auth_fail:"+sourceIP).Int64()
		require.NoError(t, err)
		require.Equal(t, int64(failedLimit), before)
		nonexistent := "lxt_" + base64.RawURLEncoding.EncodeToString(make([]byte, 32))
		for i := range 2 {
			_, rate, authErr := agent.Authenticate(ctx, nonexistent, sourceIP)
			require.Equal(t, "AGENT_AUTH_BLOCKED", infraerrors.Reason(authErr))
			require.Equal(t, 429, infraerrors.Code(authErr))
			require.Positive(t, rate.RetryAfter)
			count, readErr := redisClient.Get(ctx, "ticket_agent_auth_fail:"+sourceIP).Int64()
			require.NoError(t, readErr)
			require.Equal(t, before+int64(i)+1, count)
		}
		for _, secret := range []string{"", "still-malformed"} {
			queriesBefore := countingDriver.queries.Load()
			_, _, err = agent.Authenticate(ctx, secret, sourceIP)
			require.Equal(t, queriesBefore, countingDriver.queries.Load(), "malformed credentials must not query the key table even after ban")
			require.Equal(t, 401, infraerrors.Code(err))
		}
		mini.FastForward(time.Second)
		live, _, err := agent.Authenticate(ctx, active.Secret, sourceIP)
		require.NoError(t, err)
		require.Equal(t, user.ID, live.Owner.ID)

		// 兼容转发头开关开启时，恶意头可污染同 IP 失败计数，但不能阻断有效凭据。
		cfg := &config.Config{}
		cfg.SetTrustForwardedIPForAPIKeyACL(true)
		router := gin.New()
		require.NoError(t, router.SetTrustedProxies([]string{"127.0.0.1", "172.18.0.0/16"}))
		router.Use(servermiddleware.SessionBindingContext(cfg))
		h := handler.NewTicketHandler(nil, settings, nil, agent)
		router.GET("/api/v1/agent/me", h.AgentAuth, h.AgentMe)
		request := func(remote, secret string, headers map[string]string) *httptest.ResponseRecorder {
			mini.FastForward(time.Second)
			req := httptest.NewRequest(http.MethodGet, "/api/v1/agent/me", nil)
			req.RemoteAddr = remote
			if secret != "" {
				req.Header.Set("Authorization", "Bearer "+secret)
			}
			for name, value := range headers {
				req.Header.Set(name, value)
			}
			result := httptest.NewRecorder()
			router.ServeHTTP(result, req)
			return result
		}
		victimIP := "203.0.113.77"
		for _, header := range []string{"X-Forwarded-For", "CF-Connecting-IP"} {
			for range failedLimit + 1 {
				response := request("127.0.0.1:53000", nonexistent, map[string]string{header: victimIP})
				require.Contains(t, []int{401, 429}, response.Code)
			}
		}
		poisoned, err := redisClient.Get(ctx, "ticket_agent_auth_fail:"+victimIP).Int64()
		require.NoError(t, err)
		require.Greater(t, poisoned, int64(failedLimit))
		healthy := request(victimIP+":53001", active.Secret, nil)
		require.Equal(t, http.StatusOK, healthy.Code, healthy.Body.String())
		require.NotEmpty(t, healthy.Header().Get("X-RateLimit-Limit"))
		require.Contains(t, healthy.Body.String(), `"owner"`)
		for _, remote := range []string{"127.0.0.1:53002", "172.18.0.3:53003"} {
			for range failedLimit + 1 {
				_ = request(remote, nonexistent, nil)
			}
			healthy = request(remote, active.Secret, nil)
			require.Equal(t, http.StatusOK, healthy.Code, healthy.Body.String())
		}
	})

	t.Run("idempotent canceled failure retries and committed result survives cancellation", func(t *testing.T) {
		active, err := agent.CreateKey(ctx, user.ID, service.TicketAgentKeyCreate{Name: "idempotency-regression", Scope: "user"})
		require.NoError(t, err)
		var calls atomic.Int32
		failedCtx, cancelFailed := context.WithCancel(ctx)
		defer cancelFailed()
		_, _, err = agent.RunIdempotent(failedCtx, active.ID, "cancel-before-commit", "POST /tickets cancellation", func(workCtx context.Context) (int, []byte, error) {
			calls.Add(1)
			cancelFailed()
			return 0, nil, workCtx.Err()
		})
		require.ErrorIs(t, err, context.Canceled)
		status, result, err := agent.RunIdempotent(ctx, active.ID, "cancel-before-commit", "POST /tickets cancellation", func(context.Context) (int, []byte, error) {
			calls.Add(1)
			return http.StatusCreated, []byte(`{"retried":true}`), nil
		})
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, status)
		require.JSONEq(t, `{"retried":true}`, string(result))
		require.Equal(t, int32(2), calls.Load())

		// 实际写入 PostgreSQL 后取消请求，done 记录仍需写入 Redis 并完整保留 24 小时。
		completedCtx, cancelCompleted := context.WithCancel(ctx)
		defer cancelCompleted()
		createToken := "create-after-client-disconnect"
		createOperation := "POST /tickets committed body"
		create := func(workCtx context.Context) (int, []byte, error) {
			calls.Add(1)
			detail, createErr := svc.CreateTicket(workCtx, user.ID, "agent", service.TicketCreateInput{Subject: "idempotent committed", Body: "created only once"}, &active.ID)
			if createErr != nil {
				return 0, nil, createErr
			}
			body, marshalErr := json.Marshal(detail)
			cancelCompleted()
			return http.StatusCreated, body, marshalErr
		}
		status, result, err = agent.RunIdempotent(completedCtx, active.ID, createToken, createOperation, create)
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, status)
		require.ErrorIs(t, completedCtx.Err(), context.Canceled)
		var detail service.TicketDetail
		require.NoError(t, json.Unmarshal(result, &detail))
		require.Positive(t, detail.Ticket.ID)
		never := func(context.Context) (int, []byte, error) {
			calls.Add(1)
			return 0, nil, errors.New("idempotent result unexpectedly executed twice")
		}
		mini.FastForward(24*time.Hour - time.Second)
		replayStatus, replay, err := agent.RunIdempotent(ctx, active.ID, createToken, createOperation, never)
		require.NoError(t, err)
		require.Equal(t, status, replayStatus)
		require.JSONEq(t, string(result), string(replay))
		require.Equal(t, int32(3), calls.Load())
		var count int
		require.NoError(t, db.QueryRowContext(ctx, "SELECT COUNT(*) FROM tickets WHERE subject='idempotent committed' AND user_id=$1", user.ID).Scan(&count))
		require.Equal(t, 1, count)

		messageToken := "message-once"
		messageOperation := fmt.Sprintf("POST /tickets/%d/messages body", detail.Ticket.ID)
		var messageCalls int
		addMessage := func(workCtx context.Context) (int, []byte, error) {
			messageCalls++
			message, messageErr := svc.AddMessage(workCtx, user.ID, detail.Ticket.ID, false, "agent", &active.ID, service.TicketMessageInput{Body: "idempotent message"})
			if messageErr != nil {
				return 0, nil, messageErr
			}
			body, marshalErr := json.Marshal(message)
			return http.StatusCreated, body, marshalErr
		}
		_, firstMessage, err := agent.RunIdempotent(ctx, active.ID, messageToken, messageOperation, addMessage)
		require.NoError(t, err)
		_, secondMessage, err := agent.RunIdempotent(ctx, active.ID, messageToken, messageOperation, addMessage)
		require.NoError(t, err)
		require.JSONEq(t, string(firstMessage), string(secondMessage))
		require.Equal(t, 1, messageCalls)
		require.NoError(t, db.QueryRowContext(ctx, "SELECT COUNT(*) FROM ticket_messages WHERE ticket_id=$1 AND body='idempotent message'", detail.Ticket.ID).Scan(&count))
		require.Equal(t, 1, count)
		mini.FastForward(time.Second)
		expiredCalls := 0
		_, _, err = agent.RunIdempotent(ctx, active.ID, createToken, createOperation, func(context.Context) (int, []byte, error) {
			expiredCalls++
			return http.StatusCreated, []byte(`{"expired":true}`), nil
		})
		require.NoError(t, err)
		require.Equal(t, 1, expiredCalls)
	})

	t.Run("shared business limits and concurrent open cap", func(t *testing.T) {
		concurrentUser, err := client.User.Create().SetEmail("ticket-concurrent-" + suffix + "@example.com").SetPasswordHash("x").Save(ctx)
		require.NoError(t, err)
		maxOpen, createLimit, messageLimit := 1, 100, 2
		_, err = settings.UpdateTicketSettings(ctx, service.TicketSettingsUpdate{MaxOpenTicketsPerUser: &maxOpen, CreatePerHourPerUser: &createLimit, MessagesPerMinutePerUser: &messageLimit})
		require.NoError(t, err)
		type outcome struct {
			detail *service.TicketDetail
			err    error
		}
		results := make(chan outcome, 8)
		var wait sync.WaitGroup
		for i := range 8 {
			wait.Add(1)
			go func() {
				defer wait.Done()
				via := "web"
				if i%2 != 0 {
					via = "agent"
				}
				d, e := svc.CreateTicket(ctx, concurrentUser.ID, via, service.TicketCreateInput{Subject: fmt.Sprint("concurrent", i), Body: "body"}, nil)
				results <- outcome{d, e}
			}()
		}
		wait.Wait()
		close(results)
		successes := 0
		var winner *service.TicketDetail
		for result := range results {
			if result.err == nil {
				successes++
				winner = result.detail
			} else {
				require.Equal(t, "TICKET_OPEN_LIMIT", infraerrors.Reason(result.err))
			}
		}
		require.Equal(t, 1, successes)
		_, err = svc.AddMessage(ctx, concurrentUser.ID, winner.Ticket.ID, false, "web", nil, service.TicketMessageInput{Body: "web"})
		require.NoError(t, err)
		_, err = svc.AddMessage(ctx, concurrentUser.ID, winner.Ticket.ID, false, "agent", nil, service.TicketMessageInput{Body: "agent"})
		require.NoError(t, err)
		_, err = svc.AddMessage(ctx, concurrentUser.ID, winner.Ticket.ID, false, "web", nil, service.TicketMessageInput{Body: "over"})
		require.Equal(t, "TICKET_MESSAGE_RATE_LIMITED", infraerrors.Reason(err))
		_, err = svc.CloseReopen(ctx, concurrentUser.ID, winner.Ticket.ID, false)
		require.NoError(t, err)
		second, err := svc.CreateTicket(ctx, concurrentUser.ID, "agent", service.TicketCreateInput{Subject: "second", Body: "body"}, nil)
		require.NoError(t, err)
		_, err = svc.CloseReopen(ctx, concurrentUser.ID, winner.Ticket.ID, true)
		require.Equal(t, "TICKET_OPEN_LIMIT", infraerrors.Reason(err))
		_, err = svc.CloseReopen(ctx, concurrentUser.ID, second.Ticket.ID, false)
		require.NoError(t, err)
		createLimit = 2
		_, err = settings.UpdateTicketSettings(ctx, service.TicketSettingsUpdate{CreatePerHourPerUser: &createLimit})
		require.NoError(t, err)
		rateUser, err := client.User.Create().SetEmail("ticket-rate-" + suffix + "@example.com").SetPasswordHash("x").Save(ctx)
		require.NoError(t, err)
		first, err := svc.CreateTicket(ctx, rateUser.ID, "web", service.TicketCreateInput{Subject: "first", Body: "body"}, nil)
		require.NoError(t, err)
		_, err = svc.CloseReopen(ctx, rateUser.ID, first.Ticket.ID, false)
		require.NoError(t, err)
		_, err = svc.CreateTicket(ctx, rateUser.ID, "agent", service.TicketCreateInput{Subject: "second", Body: "body"}, nil)
		require.NoError(t, err)
		_, err = svc.CreateTicket(ctx, rateUser.ID, "web", service.TicketCreateInput{Subject: "third", Body: "body"}, nil)
		require.Equal(t, "TICKET_CREATE_RATE_LIMITED", infraerrors.Reason(err))
	})

	t.Logf("OK ticket=%s id=%d public/internal visibility, transitions, JSON shape, key ownership, downgrade and revocation verified", created.Ticket.TicketNo, id)
}
