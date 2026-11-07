package routes

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

type ticketRouteSettingRepo struct {
	service.SettingRepository
	limitReads int
}

func (r *ticketRouteSettingRepo) GetValue(_ context.Context, _ string) (string, error) {
	r.limitReads++
	return `{"agent_ip_requests_per_minute":2}`, nil
}

func (r *ticketRouteSettingRepo) GetMultiple(_ context.Context, _ []string) (map[string]string, error) {
	return map[string]string{service.SettingKeyTicketSystemEnabled: "true"}, nil
}

func TestTicketAgentIPRateLimitRunsBeforeAuthenticationAcrossRoutes(t *testing.T) {
	mini := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mini.Addr(), MaxRetries: -1})
	t.Cleanup(func() { _ = client.Close() })
	repo := &ticketRouteSettingRepo{}
	settings := service.NewSettingService(repo, nil)
	agents := service.NewTicketAgentService(nil, nil, settings, client)
	t.Cleanup(agents.Stop)
	router := gin.New()
	require.NoError(t, router.SetTrustedProxies(nil))
	cfg := &config.Config{}
	cfg.SetTrustForwardedIPForAPIKeyACL(true)
	router.Use(servermiddleware.SessionBindingContext(cfg))
	RegisterTicketAgentRoutes(router.Group("/api/v1"), handler.NewTicketHandler(nil, settings, nil, agents), client)
	request := func(path, ip string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.RemoteAddr = "127.0.0.1:4000"
		req.Header.Set("CF-Connecting-IP", ip)
		router.ServeHTTP(w, req)
		return w
	}
	ip := "8.8.8.8"
	for _, path := range []string{"/api/v1/agent/me", "/api/v1/agent/updates"} {
		w := request(path, ip)
		require.Equal(t, http.StatusUnauthorized, w.Code)
		require.Contains(t, w.Body.String(), `"reason":"AGENT_KEY_MISSING"`)
	}
	w := request("/api/v1/agent/tickets", ip)
	require.Equal(t, http.StatusTooManyRequests, w.Code)
	var envelope struct {
		Code   int    `json:"code"`
		Reason string `json:"reason"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &envelope))
	require.Equal(t, 429, envelope.Code)
	require.Equal(t, "AGENT_RATE_LIMITED", envelope.Reason)
	require.Equal(t, "2", w.Header().Get("X-RateLimit-Limit"))
	require.Equal(t, "0", w.Header().Get("X-RateLimit-Remaining"))
	require.NotEmpty(t, w.Header().Get("Retry-After"))
	failures, err := mini.Get("ticket_agent_auth_fail:" + ip)
	require.NoError(t, err)
	require.Equal(t, "2", failures, "IP quota must reject the request before authentication increments failures")
	require.Equal(t, 1, repo.limitReads, "over-limit requests must not re-read settings")
	require.Equal(t, http.StatusUnauthorized, request("/api/v1/agent/me", "1.1.1.1").Code, "public IP buckets must be independent")
	mini.FastForward(time.Minute)
	require.Equal(t, http.StatusUnauthorized, request("/api/v1/agent/me", ip).Code, "a new minute must reset the bucket")
	// 公开文档保留独立的 60/min 上限，不消耗鉴权请求配额。
	require.Equal(t, http.StatusOK, request("/api/v1/agent/docs.md", ip).Code)
}

func TestTicketAgentDocsPublicLimitUsesStandardEnvelope(t *testing.T) {
	mini := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mini.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	router := gin.New()
	RegisterTicketAgentRoutes(router.Group("/api/v1"), handler.NewTicketHandler(nil, nil, nil, nil), client)
	for i := 0; i < 61; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/api/v1/agent/docs.md", nil)
		req.RemoteAddr = "192.0.2.40:4000"
		router.ServeHTTP(w, req)
		if i < 60 {
			require.Equal(t, 200, w.Code)
		} else {
			require.Equal(t, 429, w.Code)
			require.Contains(t, w.Body.String(), `"reason":"AGENT_RATE_LIMITED"`)
			require.NotEmpty(t, w.Header().Get("Retry-After"))
		}
		require.Equal(t, "60", w.Header().Get("X-RateLimit-Limit"))
	}
}

func TestTicketRouteContract(t *testing.T) {
	r := gin.New()
	h := handler.NewTicketHandler(nil, nil, nil, nil)
	v1 := r.Group("/api/v1")
	registerUserTicketRoutes(v1.Group("/user"), h)
	registerAdminTicketRoutes(v1.Group("/admin"), h)
	RegisterTicketAgentRoutes(v1, h, nil)
	routes := map[string]bool{}
	for _, item := range r.Routes() {
		routes[item.Method+" "+item.Path] = true
	}
	for _, expected := range []string{
		"GET /api/v1/user/ticket-config", "GET /api/v1/user/tickets", "GET /api/v1/user/tickets/unread-count", "POST /api/v1/user/tickets", "GET /api/v1/user/tickets/:id", "POST /api/v1/user/tickets/:id/messages", "POST /api/v1/user/tickets/:id/close", "POST /api/v1/user/tickets/:id/reopen", "GET /api/v1/user/ticket-notify-preferences", "PUT /api/v1/user/ticket-notify-preferences", "POST /api/v1/user/ticket-notify-preferences/test", "GET /api/v1/user/ticket-agent-keys", "POST /api/v1/user/ticket-agent-keys", "PATCH /api/v1/user/ticket-agent-keys/:id", "DELETE /api/v1/user/ticket-agent-keys/:id",
		"GET /api/v1/admin/tickets", "GET /api/v1/admin/tickets/stats", "GET /api/v1/admin/tickets/:id", "POST /api/v1/admin/tickets/:id/messages", "PATCH /api/v1/admin/tickets/:id", "GET /api/v1/admin/ticket-settings", "PUT /api/v1/admin/ticket-settings", "POST /api/v1/admin/ticket-settings/test-notification", "GET /api/v1/admin/ticket-agent-keys", "PATCH /api/v1/admin/ticket-agent-keys/:id", "DELETE /api/v1/admin/ticket-agent-keys/:id", "GET /api/v1/admin/ticket-assignees",
		"GET /api/v1/agent/me", "GET /api/v1/agent/tickets", "POST /api/v1/agent/tickets", "GET /api/v1/agent/tickets/:id", "POST /api/v1/agent/tickets/:id/messages", "POST /api/v1/agent/tickets/:id/close", "POST /api/v1/agent/tickets/:id/reopen", "PATCH /api/v1/agent/tickets/:id", "GET /api/v1/agent/updates", "GET /api/v1/agent/docs.md", "GET /api/v1/agent/openapi.json",
	} {
		require.True(t, routes[expected], expected)
	}
	require.Len(t, routes, 38)
}
