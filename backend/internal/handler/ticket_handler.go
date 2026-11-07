package handler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	ratemiddleware "github.com/Wei-Shaw/sub2api/internal/middleware"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	ippkg "github.com/Wei-Shaw/sub2api/internal/pkg/ip"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type TicketHandler struct {
	tickets       *service.TicketService
	settings      *service.SettingService
	notifications *service.TicketNotificationService
	agents        *service.TicketAgentService
}

func NewTicketHandler(t *service.TicketService, s *service.SettingService, n *service.TicketNotificationService, a *service.TicketAgentService) *TicketHandler {
	return &TicketHandler{t, s, n, a}
}

func ticketHTTPError(c *gin.Context, err error) {
	if err == nil {
		return
	}
	if infraerrors.Code(err) == 429 {
		if c.Writer.Header().Get("Retry-After") == "" {
			seconds := "60"
			if infraerrors.Reason(err) == "AGENT_RATE_LIMITED" {
				seconds = "1"
			}
			if v := infraerrors.FromError(err).Metadata["retry_after"]; v != "" {
				seconds = v
			}
			c.Header("Retry-After", seconds)
		}
	}
	middleware.SetAuditExtra(c, map[string]any{"result": "failed", "error_code": infraerrors.Reason(err)})
	response.ErrorFrom(c, err)
}
func ticketID(c *gin.Context) (int64, error) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, infraerrors.NotFound("TICKET_NOT_FOUND", "ticket not found")
	}
	return id, nil
}
func ticketBind(c *gin.Context, v any) bool {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 512<<10)
	d := json.NewDecoder(c.Request.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		ticketHTTPError(c, infraerrors.BadRequest("TICKET_INVALID_INPUT", "invalid request body"))
		return false
	}
	if err := d.Decode(new(any)); err != io.EOF {
		ticketHTTPError(c, infraerrors.BadRequest("TICKET_INVALID_INPUT", "request must contain exactly one JSON object"))
		return false
	}
	return true
}
func ticketViewer(c *gin.Context) (int64, bool) {
	a, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return 0, false
	}
	return a.UserID, true
}
func (h *TicketHandler) Enabled(c *gin.Context) {
	if !h.settings.IsTicketSystemEnabled(c.Request.Context()) {
		ticketHTTPError(c, infraerrors.Forbidden("TICKET_SYSTEM_DISABLED", "ticket system disabled"))
		c.Abort()
		return
	}
	c.Next()
}
func ticketFilter(c *gin.Context) service.TicketListFilter {
	p, n := response.ParsePagination(c)
	if n > 100 {
		n = 100
	}
	uid, _ := strconv.ParseInt(c.Query("user_id"), 10, 64)
	aid, _ := strconv.ParseInt(c.Query("assignee_id"), 10, 64)
	return service.TicketListFilter{Page: p, PageSize: n, Status: c.Query("status"), Priority: c.Query("priority"), Category: c.Query("category"), Keyword: c.Query("keyword"), UserID: uid, AssigneeID: aid, UnreadOnly: c.Query("unread_only") == "true"}
}
func (h *TicketHandler) Config(c *gin.Context) {
	s, err := h.settings.GetTicketSettings(c.Request.Context())
	if err != nil {
		ticketHTTPError(c, err)
		return
	}
	response.Success(c, gin.H{"enabled": s.Enabled, "agent_enabled": s.AgentEnabled, "categories": []string{"general", "billing", "technical", "account", "feature", "other"}, "priorities": []string{"low", "normal", "high", "urgent"}, "max_subject_length": s.MaxSubjectLength, "max_body_length": s.MaxBodyLength, "max_open_tickets_per_user": s.MaxOpenTicketsPerUser, "agent_default_qps": s.AgentDefaultQPS, "agent_max_qps": s.AgentMaxQPS, "agent_user_total_qps": s.AgentUserTotalQPS, "agent_max_keys_per_user": s.AgentMaxKeysPerUser})
}
func (h *TicketHandler) List(c *gin.Context) {
	uid, ok := ticketViewer(c)
	if !ok {
		return
	}
	f := ticketFilter(c)
	items, total, err := h.tickets.ListTickets(c.Request.Context(), f, false, uid)
	if err != nil {
		ticketHTTPError(c, err)
		return
	}
	response.Paginated(c, items, total, f.Page, f.PageSize)
}
func (h *TicketHandler) Unread(c *gin.Context) {
	uid, ok := ticketViewer(c)
	if !ok {
		return
	}
	n, err := h.tickets.UnreadCount(c.Request.Context(), uid)
	if err != nil {
		ticketHTTPError(c, err)
		return
	}
	response.Success(c, gin.H{"count": n})
}
func (h *TicketHandler) Create(c *gin.Context) {
	uid, ok := ticketViewer(c)
	if !ok {
		return
	}
	var input service.TicketCreateInput
	if !ticketBind(c, &input) {
		return
	}
	if input.Category == "" {
		input.Category = "general"
	}
	if input.Priority == "" {
		input.Priority = "normal"
	}
	d, err := h.tickets.CreateTicket(c.Request.Context(), uid, "web", input, nil)
	if err != nil {
		ticketHTTPError(c, err)
		return
	}
	response.Created(c, d)
}
func (h *TicketHandler) Get(c *gin.Context) {
	uid, ok := ticketViewer(c)
	if !ok {
		return
	}
	id, err := ticketID(c)
	if err != nil {
		ticketHTTPError(c, err)
		return
	}
	d, err := h.tickets.GetTicket(c.Request.Context(), uid, id, false, 0)
	if err != nil {
		ticketHTTPError(c, err)
		return
	}
	response.Success(c, d)
}
func (h *TicketHandler) Message(c *gin.Context) {
	uid, ok := ticketViewer(c)
	if !ok {
		return
	}
	id, err := ticketID(c)
	if err != nil {
		ticketHTTPError(c, err)
		return
	}
	var input service.TicketMessageInput
	if !ticketBind(c, &input) {
		return
	}
	d, err := h.tickets.AddMessage(c.Request.Context(), uid, id, false, "web", nil, input)
	if err != nil {
		ticketHTTPError(c, err)
		return
	}
	response.Created(c, d)
}
func (h *TicketHandler) Close(c *gin.Context)  { h.closeReopen(c, false) }
func (h *TicketHandler) Reopen(c *gin.Context) { h.closeReopen(c, true) }
func (h *TicketHandler) closeReopen(c *gin.Context, reopen bool) {
	uid, ok := ticketViewer(c)
	if !ok {
		return
	}
	id, err := ticketID(c)
	if err != nil {
		ticketHTTPError(c, err)
		return
	}
	d, err := h.tickets.CloseReopen(c.Request.Context(), uid, id, reopen)
	if err != nil {
		ticketHTTPError(c, err)
		return
	}
	response.Success(c, d)
}
func (h *TicketHandler) Preferences(c *gin.Context) {
	uid, ok := ticketViewer(c)
	if !ok {
		return
	}
	d, err := h.notifications.GetPreferences(c.Request.Context(), uid)
	if err != nil {
		ticketHTTPError(c, err)
		return
	}
	response.Success(c, d)
}
func (h *TicketHandler) UpdatePreferences(c *gin.Context) {
	uid, ok := ticketViewer(c)
	if !ok {
		return
	}
	var input service.TicketNotifyPreferenceUpdate
	if !ticketBind(c, &input) {
		return
	}
	d, err := h.notifications.UpdatePreferences(c.Request.Context(), uid, input)
	if err != nil {
		ticketHTTPError(c, err)
		return
	}
	response.Success(c, d)
}
func (h *TicketHandler) TestPreference(c *gin.Context) {
	uid, ok := ticketViewer(c)
	if !ok {
		return
	}
	var input struct {
		Channel string `json:"channel"`
	}
	if !ticketBind(c, &input) {
		return
	}
	if err := h.notifications.TestUserNotification(c.Request.Context(), uid, input.Channel); err != nil {
		ticketHTTPError(c, err)
		return
	}
	response.Success(c, gin.H{"sent": true})
}
func (h *TicketHandler) Keys(c *gin.Context) {
	uid, ok := ticketViewer(c)
	if !ok {
		return
	}
	d, err := h.agents.ListKeys(c.Request.Context(), uid)
	if err != nil {
		ticketHTTPError(c, err)
		return
	}
	response.Success(c, d)
}
func (h *TicketHandler) CreateKey(c *gin.Context) {
	uid, ok := ticketViewer(c)
	if !ok {
		return
	}
	var input struct {
		Name      string     `json:"name"`
		Scope     string     `json:"scope"`
		QPSLimit  *int       `json:"qps_limit"`
		ExpiresAt *time.Time `json:"expires_at"`
	}
	if !ticketBind(c, &input) {
		return
	}
	d, err := h.agents.CreateKey(c.Request.Context(), uid, service.TicketAgentKeyCreate{Name: input.Name, Scope: input.Scope, QPSLimit: input.QPSLimit, ExpiresAt: input.ExpiresAt})
	if err != nil {
		ticketHTTPError(c, err)
		return
	}
	response.Created(c, d)
}
func (h *TicketHandler) UpdateKey(c *gin.Context)      { h.updateKey(c, false) }
func (h *TicketHandler) AdminUpdateKey(c *gin.Context) { h.updateKey(c, true) }
func (h *TicketHandler) updateKey(c *gin.Context, admin bool) {
	uid, ok := ticketViewer(c)
	if !ok {
		return
	}
	id, err := ticketID(c)
	if err != nil {
		ticketHTTPError(c, err)
		return
	}
	var input struct {
		Name     *string `json:"name"`
		QPSLimit *int    `json:"qps_limit"`
		Status   *string `json:"status"`
	}
	if !ticketBind(c, &input) {
		return
	}
	if admin && input.Name != nil {
		ticketHTTPError(c, infraerrors.BadRequest("TICKET_INVALID_INPUT", "administrator cannot rename another user's key"))
		return
	}
	d, err := h.agents.UpdateKey(c.Request.Context(), uid, id, service.TicketAgentKeyUpdate{Name: input.Name, QPSLimit: input.QPSLimit, Status: input.Status}, admin)
	if err != nil {
		ticketHTTPError(c, err)
		return
	}
	if admin {
		a, err := h.agents.GetAdminKey(c.Request.Context(), id)
		if err != nil {
			ticketHTTPError(c, err)
			return
		}
		response.Success(c, a)
		return
	}
	response.Success(c, d)
}
func (h *TicketHandler) DeleteKey(c *gin.Context)      { h.deleteKey(c, false) }
func (h *TicketHandler) AdminDeleteKey(c *gin.Context) { h.deleteKey(c, true) }
func (h *TicketHandler) deleteKey(c *gin.Context, admin bool) {
	uid, ok := ticketViewer(c)
	if !ok {
		return
	}
	id, err := ticketID(c)
	if err != nil {
		ticketHTTPError(c, err)
		return
	}
	if err = h.agents.DeleteKey(c.Request.Context(), uid, id, admin); err != nil {
		ticketHTTPError(c, err)
		return
	}
	response.Success(c, gin.H{"deleted": true})
}
func (h *TicketHandler) AdminKeys(c *gin.Context) {
	f := ticketFilter(c)
	items, total, err := h.agents.ListAdminKeys(c.Request.Context(), service.TicketAgentKeyListFilter{Page: f.Page, PageSize: f.PageSize, UserID: f.UserID, Scope: c.Query("scope"), Status: f.Status, Keyword: f.Keyword})
	if err != nil {
		ticketHTTPError(c, err)
		return
	}
	response.Paginated(c, items, total, f.Page, f.PageSize)
}
func (h *TicketHandler) AdminList(c *gin.Context) {
	f := ticketFilter(c)
	items, total, err := h.tickets.ListAdmin(c.Request.Context(), f)
	if err != nil {
		ticketHTTPError(c, err)
		return
	}
	response.Paginated(c, items, total, f.Page, f.PageSize)
}
func (h *TicketHandler) AdminGet(c *gin.Context) {
	id, err := ticketID(c)
	if err != nil {
		ticketHTTPError(c, err)
		return
	}
	d, err := h.tickets.GetAdminTicket(c.Request.Context(), id)
	if err != nil {
		ticketHTTPError(c, err)
		return
	}
	response.Success(c, d)
}
func (h *TicketHandler) AdminMessage(c *gin.Context) {
	uid, ok := ticketViewer(c)
	if !ok {
		return
	}
	id, err := ticketID(c)
	if err != nil {
		ticketHTTPError(c, err)
		return
	}
	var input service.TicketMessageInput
	if !ticketBind(c, &input) {
		return
	}
	d, err := h.tickets.AddMessage(c.Request.Context(), uid, id, true, "web", nil, input)
	if err != nil {
		ticketHTTPError(c, err)
		return
	}
	response.Created(c, d)
}
func (h *TicketHandler) AdminUpdate(c *gin.Context) {
	uid, ok := ticketViewer(c)
	if !ok {
		return
	}
	id, err := ticketID(c)
	if err != nil {
		ticketHTTPError(c, err)
		return
	}
	var input service.TicketUpdateInput
	if !ticketBind(c, &input) {
		return
	}
	d, err := h.tickets.UpdateAdmin(c.Request.Context(), id, input, uid)
	if err != nil {
		ticketHTTPError(c, err)
		return
	}
	response.Success(c, d)
}
func (h *TicketHandler) Stats(c *gin.Context) {
	d, err := h.tickets.Stats(c.Request.Context())
	if err != nil {
		ticketHTTPError(c, err)
		return
	}
	response.Success(c, d)
}
func (h *TicketHandler) Assignees(c *gin.Context) {
	d, err := h.tickets.Assignees(c.Request.Context())
	if err != nil {
		ticketHTTPError(c, err)
		return
	}
	response.Success(c, d)
}
func (h *TicketHandler) Settings(c *gin.Context) {
	d, err := h.settings.GetTicketSettings(c.Request.Context())
	if err != nil {
		ticketHTTPError(c, err)
		return
	}
	response.Success(c, d)
}
func (h *TicketHandler) UpdateSettings(c *gin.Context) {
	var input service.TicketSettingsUpdate
	if !ticketBind(c, &input) {
		return
	}
	d, err := h.settings.UpdateTicketSettings(c.Request.Context(), input)
	if err != nil {
		ticketHTTPError(c, err)
		return
	}
	response.Success(c, d)
}
func (h *TicketHandler) TestAdminNotification(c *gin.Context) {
	uid, ok := ticketViewer(c)
	if !ok {
		return
	}
	var input struct {
		Channel string `json:"channel"`
	}
	if !ticketBind(c, &input) {
		return
	}
	if err := h.notifications.TestAdminNotification(c.Request.Context(), uid, input.Channel); err != nil {
		ticketHTTPError(c, err)
		return
	}
	response.Success(c, gin.H{"sent": true})
}

// AgentIPRateLimit 在密钥查库前按公网单播 IP 计数；反代内部地址跳过，避免误伤共享出口。
func (h *TicketHandler) AgentIPRateLimit(limiter *ratemiddleware.RateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		clientIP := ippkg.GetSecurityClientIP(c, false)
		if clientIP == "" {
			clientIP = c.ClientIP()
		}
		ip := net.ParseIP(clientIP)
		if ip == nil || !ip.IsGlobalUnicast() || ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
			c.Next()
			return
		}
		limit, err := h.settings.GetTicketAgentIPRequestsPerMinuteCached(c.Request.Context())
		unavailable := func() {
			ticketHTTPError(c, infraerrors.New(503, "AGENT_RATE_LIMIT_UNAVAILABLE", "rate limiter unavailable"))
			c.Abort()
		}
		if err != nil {
			unavailable()
			return
		}
		if limit == 0 {
			c.Next()
			return
		}
		c.Header("X-RateLimit-Limit", strconv.Itoa(limit))
		c.Header("X-RateLimit-Remaining", "0")
		if limiter == nil {
			unavailable()
			return
		}
		result, err := limiter.Allow(c.Request.Context(), "ticket_agent:ip:"+ip.String(), limit, time.Minute)
		if err != nil {
			unavailable()
			return
		}
		c.Header("X-RateLimit-Remaining", strconv.Itoa(max(0, limit-int(result.Count))))
		if !result.Allowed {
			c.Header("Retry-After", strconv.Itoa(max(1, int(math.Ceil(result.RetryAfter.Seconds())))))
			ticketHTTPError(c, infraerrors.New(429, "AGENT_RATE_LIMITED", "agent IP request limit exceeded"))
			c.Abort()
			return
		}
		c.Next()
	}
}

// AgentAuth 使用独立密钥链路；每次请求重新确认所有者身份与密钥状态。
func (h *TicketHandler) AgentAuth(c *gin.Context) {
	secret := ""
	if auth := c.GetHeader("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		secret = strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
	} else {
		secret = c.GetHeader("X-Agent-Key")
	}
	ip := ippkg.GetSecurityClientIP(c, false)
	if ip == "" {
		ip = c.ClientIP()
	}
	p, rate, err := h.agents.Authenticate(c.Request.Context(), secret, ip)
	c.Header("X-RateLimit-Limit", strconv.Itoa(rate.Limit))
	c.Header("X-RateLimit-Remaining", strconv.Itoa(max(rate.Remaining, 0)))
	if rate.RetryAfter > 0 {
		c.Header("Retry-After", strconv.Itoa(max(1, int(math.Ceil(rate.RetryAfter.Seconds())))))
	}
	if err != nil {
		ticketHTTPError(c, err)
		c.Abort()
		return
	}
	c.Set("ticket_agent", p)
	c.Next()
}
func ticketPrincipal(c *gin.Context) *service.TicketAgentPrincipal {
	v, _ := c.Get("ticket_agent")
	p, _ := v.(*service.TicketAgentPrincipal)
	return p
}
func (h *TicketHandler) AgentMe(c *gin.Context) {
	p := ticketPrincipal(c)
	response.Success(c, gin.H{"key": p.Key, "owner": p.Owner, "limits": service.TicketAgentLimits{QPSLimit: p.Key.QPSLimit, OwnerTotalQPS: p.OwnerTotalQPS, MaxBodyLength: p.MaxBodyLength}})
}
func (h *TicketHandler) AgentList(c *gin.Context) {
	p := ticketPrincipal(c)
	f := ticketFilter(c)
	if p.Key.Scope == "admin" {
		items, total, err := h.tickets.ListAdmin(c.Request.Context(), f)
		if err != nil {
			ticketHTTPError(c, err)
			return
		}
		response.Paginated(c, items, total, f.Page, f.PageSize)
		return
	}
	items, total, err := h.tickets.ListTickets(c.Request.Context(), f, false, p.Owner.ID)
	if err != nil {
		ticketHTTPError(c, err)
		return
	}
	response.Paginated(c, items, total, f.Page, f.PageSize)
}
func (h *TicketHandler) AgentGet(c *gin.Context) {
	p := ticketPrincipal(c)
	id, err := ticketID(c)
	if err != nil {
		ticketHTTPError(c, err)
		return
	}
	after, err := strconv.ParseInt(c.DefaultQuery("after_message_id", "0"), 10, 64)
	if err != nil || after < 0 {
		ticketHTTPError(c, infraerrors.BadRequest("TICKET_INVALID_INPUT", "invalid after_message_id"))
		return
	}
	if p.Key.Scope == "admin" {
		d, err := h.tickets.GetAdminTicket(c.Request.Context(), id)
		if err != nil {
			ticketHTTPError(c, err)
			return
		}
		if after > 0 {
			msgs := d.Messages[:0]
			for _, m := range d.Messages {
				if m.ID > after {
					msgs = append(msgs, m)
				}
			}
			d.Messages = msgs
		}
		response.Success(c, d)
		return
	}
	d, err := h.tickets.GetTicket(c.Request.Context(), p.Owner.ID, id, false, after)
	if err != nil {
		ticketHTTPError(c, err)
		return
	}
	response.Success(c, d)
}
func (h *TicketHandler) AgentCreate(c *gin.Context) {
	p := ticketPrincipal(c)
	if p.Key.Scope != "user" {
		ticketHTTPError(c, infraerrors.Forbidden("AGENT_SCOPE_FORBIDDEN", "user scope required"))
		return
	}
	var input service.TicketCreateInput
	if !ticketBind(c, &input) {
		return
	}
	if input.Category == "" {
		input.Category = "general"
	}
	if input.Priority == "" {
		input.Priority = "normal"
	}
	h.idempotent(c, input, func(ctx context.Context) (any, error) {
		return h.tickets.CreateTicket(ctx, p.Owner.ID, "agent", input, &p.Key.ID)
	})
}
func (h *TicketHandler) AgentMessage(c *gin.Context) {
	p := ticketPrincipal(c)
	id, err := ticketID(c)
	if err != nil {
		ticketHTTPError(c, err)
		return
	}
	var input service.TicketMessageInput
	if !ticketBind(c, &input) {
		return
	}
	h.idempotent(c, input, func(ctx context.Context) (any, error) {
		return h.tickets.AddMessage(ctx, p.Owner.ID, id, p.Key.Scope == "admin", "agent", &p.Key.ID, input)
	})
}
func (h *TicketHandler) idempotent(c *gin.Context, input any, fn func(context.Context) (any, error)) {
	p := ticketPrincipal(c)
	encoded, _ := json.Marshal(input)
	sum := sha256.Sum256(encoded)
	operation := c.Request.Method + ":" + c.Request.URL.Path + ":" + hex.EncodeToString(sum[:])
	status, data, err := h.agents.RunIdempotent(c.Request.Context(), p.Key.ID, c.GetHeader("Idempotency-Key"), operation, func(ctx context.Context) (int, []byte, error) {
		d, err := fn(ctx)
		if err != nil {
			return 0, nil, err
		}
		b, err := json.Marshal(d)
		return 201, b, err
	})
	if err != nil {
		ticketHTTPError(c, err)
		return
	}
	c.JSON(status, response.Response{Code: 0, Message: "success", Data: json.RawMessage(data)})
}
func (h *TicketHandler) AgentClose(c *gin.Context)  { h.agentCloseReopen(c, false) }
func (h *TicketHandler) AgentReopen(c *gin.Context) { h.agentCloseReopen(c, true) }
func (h *TicketHandler) agentCloseReopen(c *gin.Context, reopen bool) {
	p := ticketPrincipal(c)
	if p.Key.Scope != "user" {
		ticketHTTPError(c, infraerrors.Forbidden("AGENT_SCOPE_FORBIDDEN", "user scope required"))
		return
	}
	id, err := ticketID(c)
	if err != nil {
		ticketHTTPError(c, err)
		return
	}
	d, err := h.tickets.CloseReopen(c.Request.Context(), p.Owner.ID, id, reopen)
	if err != nil {
		ticketHTTPError(c, err)
		return
	}
	response.Success(c, d)
}
func (h *TicketHandler) AgentUpdate(c *gin.Context) {
	p := ticketPrincipal(c)
	if p.Key.Scope != "admin" {
		ticketHTTPError(c, infraerrors.Forbidden("AGENT_SCOPE_FORBIDDEN", "admin scope required"))
		return
	}
	id, err := ticketID(c)
	if err != nil {
		ticketHTTPError(c, err)
		return
	}
	var input service.TicketUpdateInput
	if !ticketBind(c, &input) {
		return
	}
	d, err := h.tickets.UpdateAdmin(c.Request.Context(), id, input, p.Owner.ID)
	if err != nil {
		ticketHTTPError(c, err)
		return
	}
	response.Success(c, d)
}
func (h *TicketHandler) AgentUpdates(c *gin.Context) {
	p := ticketPrincipal(c)
	cursor, err := strconv.ParseInt(c.DefaultQuery("cursor", "0"), 10, 64)
	if err != nil || cursor < 0 {
		ticketHTTPError(c, infraerrors.BadRequest("TICKET_INVALID_INPUT", "invalid cursor"))
		return
	}
	limit, err := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if err != nil || limit < 1 {
		ticketHTTPError(c, infraerrors.BadRequest("TICKET_INVALID_INPUT", "invalid limit"))
		return
	}
	if limit > 100 {
		limit = 100
	}
	d, err := h.tickets.Updates(c.Request.Context(), p.Owner.ID, p.Key.Scope == "admin", cursor, limit)
	if err != nil {
		ticketHTTPError(c, err)
		return
	}
	response.Success(c, d)
}
