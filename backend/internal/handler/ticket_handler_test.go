//go:build unit

package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	ratemiddleware "github.com/Wei-Shaw/sub2api/internal/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9" //nolint:depguard // 测试使用 miniredis 验证真实 Redis 限流协议。
	"github.com/stretchr/testify/require"
)

type ticketIPRateSettingRepo struct {
	settingHandlerPublicRepoStub
}

func (r *ticketIPRateSettingRepo) GetValue(_ context.Context, key string) (string, error) {
	return r.values[key], nil
}

func TestTicketHandlerConfigContractAndDisabledGate(t *testing.T) {
	repo := &settingHandlerPublicRepoStub{values: map[string]string{}}
	settings := service.NewSettingService(repo, &config.Config{})
	h := NewTicketHandler(nil, settings, nil, nil)
	r := gin.New()
	r.GET("/config", h.Config)
	r.GET("/disabled", h.Enabled, func(c *gin.Context) { t.Fatal("disabled guard allowed handler") })
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/config", nil))
	require.Equal(t, 200, w.Code)
	var env struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &env))
	require.Len(t, env.Data, 11)
	require.Equal(t, false, env.Data["enabled"])
	require.Equal(t, true, env.Data["agent_enabled"])
	require.Equal(t, float64(200), env.Data["max_subject_length"])
	require.NotContains(t, env.Data, "agent_ip_requests_per_minute")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/disabled", nil))
	require.Equal(t, 403, w.Code)
	require.Contains(t, w.Body.String(), "TICKET_SYSTEM_DISABLED")
}
func TestTicketHandlerPublicSettingsFlag(t *testing.T) {
	for _, v := range []string{"", "false", "true"} {
		repo := &settingHandlerPublicRepoStub{values: map[string]string{service.SettingKeyTicketSystemEnabled: v}}
		h := NewSettingHandler(service.NewSettingService(repo, &config.Config{}), "test")
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/settings/public", nil)
		h.GetPublicSettings(c)
		var env struct {
			Data map[string]any `json:"data"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &env))
		require.Equal(t, v == "true", env.Data["ticket_system_enabled"])
		require.NotContains(t, env.Data, "agent_ip_requests_per_minute")
	}
}

func TestTicketAgentIPRateLimitSkipsNonPublicAddresses(t *testing.T) {
	for _, remote := range []string{"127.0.0.1:4000", "10.0.0.1:4000", "172.18.0.3:4000", "192.168.1.1:4000", "169.254.1.1:4000", "0.0.0.0:4000", "[::1]:4000", "[fd00::1]:4000", "[fe80::1]:4000", "[ff02::1]:4000", "[::]:4000"} {
		t.Run(remote, func(t *testing.T) {
			h := NewTicketHandler(nil, nil, nil, nil)
			router := gin.New()
			require.NoError(t, router.SetTrustedProxies(nil))
			// 缺少设置与 Redis 时仍跳过，确保反代地址不进入任何共享桶。
			router.GET("/test", h.AgentIPRateLimit(nil), func(c *gin.Context) { c.Status(http.StatusNoContent) })
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.RemoteAddr = remote
			router.ServeHTTP(w, req)
			require.Equal(t, http.StatusNoContent, w.Code)
			require.Empty(t, w.Header().Get("X-RateLimit-Limit"))
		})
	}
}

func TestTicketAgentIPRateLimitHeadersAndRedisFailure(t *testing.T) {
	mini := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mini.Addr(), MaxRetries: -1})
	t.Cleanup(func() { _ = client.Close() })
	repo := &ticketIPRateSettingRepo{settingHandlerPublicRepoStub{values: map[string]string{"ticket_settings": `{"agent_ip_requests_per_minute":1}`}}}
	h := NewTicketHandler(nil, service.NewSettingService(repo, nil), nil, nil)
	router := gin.New()
	router.GET("/test", h.AgentIPRateLimit(ratemiddleware.NewRateLimiter(client)), func(c *gin.Context) { c.Status(http.StatusNoContent) })
	request := func() *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = "8.8.8.8:4000"
		router.ServeHTTP(w, req)
		return w
	}
	w := request()
	require.Equal(t, http.StatusNoContent, w.Code)
	require.Equal(t, "1", w.Header().Get("X-RateLimit-Limit"))
	require.Equal(t, "0", w.Header().Get("X-RateLimit-Remaining"))
	mini.FastForward(time.Minute - 1500*time.Millisecond)
	w = request()
	require.Equal(t, http.StatusTooManyRequests, w.Code)
	require.Equal(t, "2", w.Header().Get("Retry-After"), "remaining TTL must round up")
	require.Contains(t, w.Body.String(), `"reason":"AGENT_RATE_LIMITED"`)
	mini.FastForward(1500 * time.Millisecond)
	require.Equal(t, http.StatusNoContent, request().Code)
	mini.SetError("ERR unavailable")
	w = request()
	require.Equal(t, http.StatusServiceUnavailable, w.Code)
	require.Contains(t, w.Body.String(), `"reason":"AGENT_RATE_LIMIT_UNAVAILABLE"`)
	require.Empty(t, w.Header().Get("Retry-After"))
}

func TestTicketAgentIPRateLimitDisabledAndMissingRedis(t *testing.T) {
	for _, limit := range []string{"0", "600"} {
		repo := &ticketIPRateSettingRepo{settingHandlerPublicRepoStub{values: map[string]string{"ticket_settings": `{"agent_ip_requests_per_minute":` + limit + `}`}}}
		h := NewTicketHandler(nil, service.NewSettingService(repo, nil), nil, nil)
		router := gin.New()
		router.GET("/test", h.AgentIPRateLimit(nil), func(c *gin.Context) { c.Status(http.StatusNoContent) })
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = "8.8.8.8:4000"
		router.ServeHTTP(w, req)
		if limit == "0" {
			require.Equal(t, http.StatusNoContent, w.Code)
		} else {
			require.Equal(t, http.StatusServiceUnavailable, w.Code)
			require.Contains(t, w.Body.String(), `"reason":"AGENT_RATE_LIMIT_UNAVAILABLE"`)
		}
	}
}
func TestTicketHandlerBindingAndErrorEnvelope(t *testing.T) {
	for _, body := range []string{`{"unknown":true}`, `{"body":"ok"} {}`, `not json`} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/test", strings.NewReader(body))
		var input service.TicketMessageInput
		require.False(t, ticketBind(c, &input))
		require.Equal(t, 400, w.Code)
		require.Contains(t, w.Body.String(), "TICKET_INVALID_INPUT")
	}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/test/abc", nil)
	c.Params = gin.Params{{Key: "id", Value: "abc"}}
	_, err := ticketID(c)
	require.Error(t, err)
	ticketHTTPError(c, err)
	require.Equal(t, 404, w.Code)
	require.Contains(t, w.Body.String(), "TICKET_NOT_FOUND")
}
func TestTicketJSONContracts(t *testing.T) {
	now := time.Now().UTC()
	ticket := service.TicketSummary{ID: 1, TicketNo: "TK20261106ABCDEF", Subject: "test", Category: "general", Priority: "normal", Status: "pending_admin", UserID: 2, CreatedAt: now, UpdatedAt: now, LastMessageAt: now}
	data, err := json.Marshal(service.AdminTicketSummary{TicketSummary: ticket, UserEmail: "a@example.com", UserUsername: "alice"})
	require.NoError(t, err)
	var fields map[string]any
	require.NoError(t, json.Unmarshal(data, &fields))
	require.Len(t, fields, 20)
	require.Equal(t, "a@example.com", fields["user_email"])
	require.Equal(t, nil, fields["closed_at"])
	require.Equal(t, nil, fields["assignee_name"])
	require.Equal(t, nil, fields["assignee_id"])
	data, err = json.Marshal(service.TicketMessage{ID: 1, TicketID: 1, AuthorType: "admin", AuthorName: "Support", CreatedAt: now})
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(data, &fields))
	require.Contains(t, string(data), `"author_id":null`)
	require.NotContains(t, string(data), "agent_key_id")
}

func TestTicketAgentHTTPHeadersAndIdentityContract(t *testing.T) {
	rdb := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: rdb.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	repo := &settingHandlerPublicRepoStub{values: map[string]string{service.SettingKeyTicketSystemEnabled: "true", "ticket_settings": `{"agent_failed_auth_per_minute_per_ip":1}`}}
	settings := service.NewSettingService(repo, &config.Config{})
	agents := service.NewTicketAgentService(nil, nil, settings, client)
	t.Cleanup(agents.Stop)
	h := NewTicketHandler(nil, settings, nil, agents)
	router := gin.New()
	router.GET("/protected", h.AgentAuth, func(c *gin.Context) { t.Fatal("missing agent key authenticated") })
	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/protected", nil)
		req.RemoteAddr = "192.0.2.1:1234"
		if i == 1 {
			// 格式合法但查不到的密钥才使用 IP 封禁响应。
			req.Header.Set("X-Agent-Key", "lxt_"+strings.Repeat("A", 43))
		}
		router.ServeHTTP(w, req)
		if i == 0 {
			require.Equal(t, 401, w.Code)
			require.Contains(t, w.Body.String(), "AGENT_KEY_MISSING")
		} else {
			require.Equal(t, 429, w.Code)
			require.Contains(t, w.Body.String(), "AGENT_AUTH_BLOCKED")
			require.NotEmpty(t, w.Header().Get("Retry-After"))
		}
		require.NotEmpty(t, w.Header().Get("X-RateLimit-Limit"))
		require.NotEmpty(t, w.Header().Get("X-RateLimit-Remaining"))
	}
	wMalformed := httptest.NewRecorder()
	malformed := httptest.NewRequest("GET", "/protected", nil)
	malformed.RemoteAddr = "192.0.2.1:1234"
	malformed.Header.Set("X-Agent-Key", "lxt_bad")
	router.ServeHTTP(wMalformed, malformed)
	require.Equal(t, http.StatusUnauthorized, wMalformed.Code)
	require.Contains(t, wMalformed.Body.String(), "AGENT_KEY_INVALID")
	require.Empty(t, wMalformed.Header().Get("Retry-After"))
	router.GET("/me", func(c *gin.Context) {
		c.Set("ticket_agent", &service.TicketAgentPrincipal{Key: service.TicketAgentKeyView{ID: 1, Scope: "user", QPSLimit: 2}, Owner: service.TicketAgentOwner{ID: 2, Role: "user", Username: "alice"}, OwnerTotalQPS: 5, MaxBodyLength: 5000})
	}, h.AgentMe)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", "/me", nil))
	require.Equal(t, 200, w.Code)
	var envelope struct {
		Data map[string]json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &envelope))
	require.Len(t, envelope.Data, 3)
	require.Contains(t, string(envelope.Data["limits"]), `"owner_total_qps":5`)
	require.Contains(t, string(envelope.Data["key"]), `"last_used_ip":null`)
}
