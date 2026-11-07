package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/ticketagentkey"
	"github.com/Wei-Shaw/sub2api/ent/user"
	"github.com/Wei-Shaw/sub2api/internal/middleware"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/redis/go-redis/v9" //nolint:depguard // 工单对外接口需原子 Lua 和 fail-closed；构造器由 wire 注入专用 Redis 客户端。
)

const (
	TicketAgentScopeUser   = "user"
	TicketAgentScopeAdmin  = "admin"
	TicketAgentKeyActive   = "active"
	TicketAgentKeyDisabled = "disabled"
)

type TicketAgentKeyCreate struct {
	Name      string     `json:"name"`
	Scope     string     `json:"scope"`
	QPSLimit  *int       `json:"qps_limit"`
	ExpiresAt *time.Time `json:"expires_at"`
}
type TicketAgentKeyUpdate struct {
	Name     *string `json:"name"`
	QPSLimit *int    `json:"qps_limit"`
	Status   *string `json:"status"`
}
type TicketAgentKeyListFilter struct {
	Page, PageSize         int
	UserID                 int64
	Scope, Status, Keyword string
}

type TicketAgentKeyView struct {
	ID         int64      `json:"id"`
	Name       string     `json:"name"`
	KeyPrefix  string     `json:"key_prefix"`
	Scope      string     `json:"scope"`
	QPSLimit   int        `json:"qps_limit"`
	Status     string     `json:"status"`
	LastUsedAt *time.Time `json:"last_used_at"`
	LastUsedIP *string    `json:"last_used_ip"`
	ExpiresAt  *time.Time `json:"expires_at"`
	CreatedAt  time.Time  `json:"created_at"`
}
type TicketAgentKeyCreated struct {
	TicketAgentKeyView
	Secret string `json:"secret"`
}
type AdminTicketAgentKeyView struct {
	TicketAgentKeyView
	UserID    int64  `json:"user_id"`
	UserEmail string `json:"user_email"`
}

type TicketAgentOwner struct {
	ID       int64  `json:"id"`
	Role     string `json:"role"`
	Username string `json:"username"`
}
type TicketAgentPrincipal struct {
	Key           TicketAgentKeyView `json:"key"`
	Owner         TicketAgentOwner   `json:"owner"`
	OwnerTotalQPS int                `json:"owner_total_qps"`
	MaxBodyLength int                `json:"max_body_length"`
}
type TicketAgentLimits struct {
	QPSLimit      int `json:"qps_limit"`
	OwnerTotalQPS int `json:"owner_total_qps"`
	MaxBodyLength int `json:"max_body_length"`
}
type TicketAgentRate struct {
	Limit      int
	Remaining  int
	RetryAfter time.Duration
}

type TicketAgentService struct {
	entClient *dbent.Client
	userRepo  UserRepository
	settings  *SettingService
	redis     *redis.Client
	limiter   *middleware.RateLimiter
	usedCh    chan usedRecord
	stopCh    chan struct{}
	doneCh    chan struct{}
	stopOnce  sync.Once
}
type usedRecord struct {
	id int64
	at time.Time
	ip string
}

func NewTicketAgentService(client *dbent.Client, users UserRepository, settings *SettingService, redisClient *redis.Client) *TicketAgentService {
	s := &TicketAgentService{entClient: client, userRepo: users, settings: settings, redis: redisClient, usedCh: make(chan usedRecord, 128), stopCh: make(chan struct{}), doneCh: make(chan struct{})}
	if redisClient != nil {
		s.limiter = middleware.NewRateLimiter(redisClient)
	}
	go s.lastUsedWorker()
	return s
}
func (s *TicketAgentService) Stop() { s.stopOnce.Do(func() { close(s.stopCh); <-s.doneCh }) }
func (s *TicketAgentService) lastUsedWorker() {
	defer close(s.doneCh)
	defer func() {
		if recover() != nil {
			slog.Error("ticket agent usage worker panicked")
		}
	}()
	for {
		select {
		case rec := <-s.usedCh:
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			s.writeLastUsed(ctx, rec)
			cancel()
		case <-s.stopCh:
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			for {
				select {
				case rec := <-s.usedCh:
					s.writeLastUsed(ctx, rec)
				case <-ctx.Done():
					return
				default:
					return
				}
			}
		}
	}
}
func (s *TicketAgentService) writeLastUsed(ctx context.Context, rec usedRecord) {
	if s.entClient == nil {
		return
	}
	now := rec.at
	ip := rec.ip
	_, err := s.entClient.TicketAgentKey.Update().Where(ticketagentkey.ID(rec.id), ticketagentkey.DeletedAtIsNil(), ticketagentkey.Or(ticketagentkey.LastUsedAtIsNil(), ticketagentkey.LastUsedAtLTE(now.Add(-60*time.Second)))).SetLastUsedAt(now).SetLastUsedIP(ip).Save(ctx)
	if err != nil {
		slog.Warn("update ticket agent usage failed", "key_id", rec.id, "error", err)
	}
}

func ticketAgentError(reason string) *infraerrors.ApplicationError {
	code := 401
	switch reason {
	case "TICKET_NOT_FOUND":
		code = 404
	case "TICKET_INVALID_INPUT":
		code = 400
	case "AGENT_SCOPE_FORBIDDEN", "AGENT_API_DISABLED", "TICKET_SYSTEM_DISABLED":
		code = 403
	case "AGENT_KEY_LIMIT":
		code = 409
	case "AGENT_RATE_LIMITED", "AGENT_AUTH_BLOCKED":
		code = 429
	case "AGENT_RATE_LIMIT_UNAVAILABLE":
		code = 503
	case "AGENT_QPS_INVALID":
		code = 400
	}
	return infraerrors.New(code, reason, strings.ToLower(strings.ReplaceAll(strings.TrimPrefix(reason, "AGENT_"), "_", " ")))
}

func (s *TicketAgentService) getSettings(ctx context.Context) (*TicketSettings, error) {
	if s.settings == nil {
		v := DefaultTicketSettings()
		return &v, nil
	}
	return s.settings.GetTicketSettings(ctx)
}
func (s *TicketAgentService) checkEnabled(ctx context.Context) error {
	st, err := s.getSettings(ctx)
	if err != nil {
		return err
	}
	if !st.Enabled {
		return ticketAgentError("TICKET_SYSTEM_DISABLED")
	}
	if !st.AgentEnabled {
		return ticketAgentError("AGENT_API_DISABLED")
	}
	return nil
}
func makeSecret() (string, string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}
	secret := "lxt_" + base64.RawURLEncoding.EncodeToString(b)
	sum := sha256.Sum256([]byte(secret))
	return secret, hex.EncodeToString(sum[:]), nil
}
func viewKey(k *dbent.TicketAgentKey) TicketAgentKeyView {
	return TicketAgentKeyView{ID: k.ID, Name: k.Name, KeyPrefix: k.KeyPrefix, Scope: k.Scope, QPSLimit: k.QPSLimit, Status: k.Status, LastUsedAt: k.LastUsedAt, LastUsedIP: k.LastUsedIP, ExpiresAt: k.ExpiresAt, CreatedAt: k.CreatedAt}
}

func (s *TicketAgentService) CreateKey(ctx context.Context, ownerID int64, in TicketAgentKeyCreate) (*TicketAgentKeyCreated, error) {
	st, err := s.getSettings(ctx)
	if err != nil {
		return nil, err
	}
	if !st.Enabled {
		return nil, ticketAgentError("TICKET_SYSTEM_DISABLED")
	}
	name := strings.TrimSpace(in.Name)
	if !validateText(name, 100) {
		return nil, ticketAgentError("TICKET_INVALID_INPUT")
	}
	scope := in.Scope
	if scope == "" {
		scope = TicketAgentScopeUser
	}
	if scope != TicketAgentScopeUser && scope != TicketAgentScopeAdmin {
		return nil, ticketAgentError("AGENT_SCOPE_FORBIDDEN")
	}
	if scope == TicketAgentScopeAdmin {
		if s.userRepo == nil {
			return nil, ticketAgentError("AGENT_SCOPE_FORBIDDEN")
		}
		owner, err := s.userRepo.GetByID(ctx, ownerID)
		if err != nil {
			return nil, err
		}
		if owner == nil || !owner.IsActive() || !owner.IsAdmin() || owner.DeletedAt != nil {
			return nil, ticketAgentError("AGENT_SCOPE_FORBIDDEN")
		}
	}
	qps := st.AgentDefaultQPS
	if in.QPSLimit != nil {
		qps = *in.QPSLimit
	}
	if qps < 1 || qps > st.AgentMaxQPS {
		return nil, ticketAgentError("AGENT_QPS_INVALID")
	}
	if in.ExpiresAt != nil && !in.ExpiresAt.After(time.Now()) {
		return nil, ticketAgentError("TICKET_INVALID_INPUT")
	}
	if s.entClient == nil {
		return nil, errors.New("ticket agent database unavailable")
	}
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	// 在用户维度串行创建，防止并发请求绕过密钥数量上限。
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, fmt.Sprintf("ticket-agent-key:%d", ownerID)); err != nil {
		return nil, err
	}
	n, err := tx.TicketAgentKey.Query().Where(ticketagentkey.UserID(ownerID), ticketagentkey.DeletedAtIsNil()).Count(ctx)
	if err != nil {
		return nil, err
	}
	if n >= st.AgentMaxKeysPerUser {
		return nil, ticketAgentError("AGENT_KEY_LIMIT")
	}
	secret, hash, err := makeSecret()
	if err != nil {
		return nil, err
	}
	create := tx.TicketAgentKey.Create().SetUserID(ownerID).SetName(name).SetKeyPrefix(secret[:12]).SetKeyHash(hash).SetScope(scope).SetQPSLimit(qps).SetStatus(TicketAgentKeyActive)
	if in.ExpiresAt != nil {
		create.SetExpiresAt(*in.ExpiresAt)
	}
	k, err := create.Save(ctx)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &TicketAgentKeyCreated{TicketAgentKeyView: viewKey(k), Secret: secret}, nil
}
func (s *TicketAgentService) ListKeys(ctx context.Context, ownerID int64) ([]TicketAgentKeyView, error) {
	if s.entClient == nil {
		return []TicketAgentKeyView{}, nil
	}
	keys, err := s.entClient.TicketAgentKey.Query().Where(ticketagentkey.UserID(ownerID), ticketagentkey.DeletedAtIsNil()).Order(ticketagentkey.ByCreatedAt()).All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]TicketAgentKeyView, 0, len(keys))
	for _, k := range keys {
		out = append(out, viewKey(k))
	}
	return out, nil
}
func (s *TicketAgentService) UpdateKey(ctx context.Context, ownerID, keyID int64, in TicketAgentKeyUpdate, admin bool) (*TicketAgentKeyView, error) {
	if s.entClient == nil {
		return nil, ticketAgentError("TICKET_NOT_FOUND")
	}
	q := s.entClient.TicketAgentKey.Query().Where(ticketagentkey.ID(keyID), ticketagentkey.DeletedAtIsNil())
	if !admin {
		q = q.Where(ticketagentkey.UserID(ownerID))
	}
	k, err := q.Only(ctx)
	if dbent.IsNotFound(err) {
		return nil, ticketAgentError("TICKET_NOT_FOUND")
	}
	if err != nil {
		return nil, err
	}
	u := k.Update()
	if in.Name != nil {
		n := strings.TrimSpace(*in.Name)
		if !validateText(n, 100) {
			return nil, ticketAgentError("TICKET_INVALID_INPUT")
		}
		u.SetName(n)
	}
	if in.QPSLimit != nil {
		st, e := s.getSettings(ctx)
		if e != nil {
			return nil, e
		}
		if *in.QPSLimit < 1 || *in.QPSLimit > st.AgentMaxQPS {
			return nil, ticketAgentError("AGENT_QPS_INVALID")
		}
		u.SetQPSLimit(*in.QPSLimit)
	}
	if in.Status != nil {
		if *in.Status != TicketAgentKeyActive && *in.Status != TicketAgentKeyDisabled {
			return nil, ticketAgentError("TICKET_INVALID_INPUT")
		}
		u.SetStatus(*in.Status)
	}
	k, err = u.Save(ctx)
	if err != nil {
		return nil, err
	}
	v := viewKey(k)
	return &v, nil
}
func (s *TicketAgentService) DeleteKey(ctx context.Context, ownerID, keyID int64, admin bool) error {
	if s.entClient == nil {
		return ticketAgentError("TICKET_NOT_FOUND")
	}
	q := s.entClient.TicketAgentKey.Query().Where(ticketagentkey.ID(keyID), ticketagentkey.DeletedAtIsNil())
	if !admin {
		q = q.Where(ticketagentkey.UserID(ownerID))
	}
	k, e := q.Only(ctx)
	if dbent.IsNotFound(e) {
		return ticketAgentError("TICKET_NOT_FOUND")
	}
	if e != nil {
		return e
	}
	now := time.Now()
	_, e = k.Update().SetStatus(TicketAgentKeyDisabled).SetDeletedAt(now).Save(ctx)
	return e
}

func (s *TicketAgentService) ListAdminKeys(ctx context.Context, f TicketAgentKeyListFilter) ([]AdminTicketAgentKeyView, int64, error) {
	if s.entClient == nil {
		return []AdminTicketAgentKeyView{}, 0, nil
	}
	q := s.entClient.TicketAgentKey.Query().Where(ticketagentkey.DeletedAtIsNil())
	if f.UserID > 0 {
		q = q.Where(ticketagentkey.UserID(f.UserID))
	}
	if f.Scope != "" {
		if f.Scope != TicketAgentScopeUser && f.Scope != TicketAgentScopeAdmin {
			return nil, 0, ticketAgentError("TICKET_INVALID_INPUT")
		}
		q = q.Where(ticketagentkey.Scope(f.Scope))
	}
	if f.Status != "" {
		if f.Status != TicketAgentKeyActive && f.Status != TicketAgentKeyDisabled {
			return nil, 0, ticketAgentError("TICKET_INVALID_INPUT")
		}
		q = q.Where(ticketagentkey.Status(f.Status))
	}
	if f.Keyword != "" {
		userIDs, err := s.entClient.User.Query().Where(user.EmailContainsFold(f.Keyword)).IDs(ctx)
		if err != nil {
			return nil, 0, err
		}
		q = q.Where(ticketagentkey.Or(ticketagentkey.NameContainsFold(f.Keyword), ticketagentkey.KeyPrefixContainsFold(f.Keyword), ticketagentkey.UserIDIn(userIDs...)))
	}
	total, e := q.Count(ctx)
	if e != nil {
		return nil, 0, e
	}
	page := f.Page
	if page < 1 {
		page = 1
	}
	size := f.PageSize
	if size < 1 {
		size = 20
	}
	if size > 100 {
		size = 100
	}
	keys, e := q.Order(ticketagentkey.ByCreatedAt()).Offset((page - 1) * size).Limit(size).All(ctx)
	if e != nil {
		return nil, 0, e
	}
	out := make([]AdminTicketAgentKeyView, 0, len(keys))
	for _, k := range keys {
		v := AdminTicketAgentKeyView{TicketAgentKeyView: viewKey(k), UserID: k.UserID}
		if s.userRepo != nil {
			if u, ue := s.userRepo.GetByID(ctx, k.UserID); ue == nil && u != nil {
				v.UserEmail = u.Email
			}
		}
		out = append(out, v)
	}
	return out, int64(total), nil
}

func (s *TicketAgentService) GetAdminKey(ctx context.Context, keyID int64) (*AdminTicketAgentKeyView, error) {
	if s.entClient == nil {
		return nil, ticketAgentError("TICKET_NOT_FOUND")
	}
	k, err := s.entClient.TicketAgentKey.Query().Where(ticketagentkey.ID(keyID), ticketagentkey.DeletedAtIsNil()).Only(ctx)
	if dbent.IsNotFound(err) {
		return nil, ticketAgentError("TICKET_NOT_FOUND")
	}
	if err != nil {
		return nil, err
	}
	out := &AdminTicketAgentKeyView{TicketAgentKeyView: viewKey(k), UserID: k.UserID}
	if s.userRepo != nil {
		owner, err := s.userRepo.GetByIDIncludeDeleted(ctx, k.UserID)
		if err != nil {
			return nil, err
		}
		if owner != nil {
			out.UserEmail = owner.Email
		}
	}
	return out, nil
}

func shaHash(secret string) string {
	h := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(h[:])
}
func parseAgentSecret(secret string) bool {
	if len(secret) != 47 || !strings.HasPrefix(secret, "lxt_") {
		return false
	}
	b, e := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(secret, "lxt_"))
	return e == nil && len(b) == 32 && "lxt_"+base64.RawURLEncoding.EncodeToString(b) == secret
}
func (s *TicketAgentService) recordAuthFailure(ctx context.Context, ip string) (int64, time.Duration, error) {
	if s.redis == nil {
		return 0, 0, errors.New("redis unavailable")
	}
	key := "ticket_agent_auth_fail:" + ip
	values, err := ticketAgentAuthFailureScript.Run(ctx, s.redis, []string{key}).Int64Slice()
	if err != nil {
		return 0, 0, err
	}
	if len(values) != 2 {
		return 0, 0, errors.New("unexpected ticket agent authentication counter result")
	}
	return values[0], time.Duration(values[1]) * time.Millisecond, nil
}

var ticketAgentAuthFailureScript = redis.NewScript(`
local n = redis.call('INCR', KEYS[1])
if n == 1 or redis.call('TTL', KEYS[1]) < 0 then redis.call('EXPIRE', KEYS[1], 60) end
return {n, redis.call('PTTL', KEYS[1])}
`)

func (s *TicketAgentService) enqueueLastUsed(k *dbent.TicketAgentKey, ip string) {
	rec := usedRecord{id: k.ID, at: time.Now(), ip: ip}
	select {
	case <-s.stopCh:
		return
	default:
	}
	select {
	case s.usedCh <- rec:
	default:
	}
}

func (s *TicketAgentService) Authenticate(ctx context.Context, secret, ip string) (*TicketAgentPrincipal, TicketAgentRate, error) {
	var zero TicketAgentRate
	if err := s.checkEnabled(ctx); err != nil {
		return nil, zero, err
	}
	st, err := s.getSettings(ctx)
	if err != nil {
		return nil, zero, err
	}
	fail := func(reason string, applyBan bool) (*TicketAgentPrincipal, TicketAgentRate, error) {
		failed, ttl, err := s.recordAuthFailure(ctx, ip)
		if err != nil {
			return nil, zero, ticketAgentError("AGENT_RATE_LIMIT_UNAVAILABLE")
		}
		if applyBan && failed > int64(st.AgentFailedAuthPerMinutePerIP) {
			if ttl <= 0 {
				ttl = time.Minute
			}
			return nil, TicketAgentRate{Limit: st.AgentFailedAuthPerMinutePerIP, RetryAfter: ttl}, ticketAgentError("AGENT_AUTH_BLOCKED")
		}
		return nil, zero, ticketAgentError(reason)
	}
	// 格式错误不查库；共享出口 IP 的历史失败不影响有效密钥。
	if secret == "" {
		return fail("AGENT_KEY_MISSING", false)
	}
	if !parseAgentSecret(secret) {
		return fail("AGENT_KEY_INVALID", false)
	}
	if s.entClient == nil {
		return fail("AGENT_KEY_INVALID", true)
	}
	k, e := s.entClient.TicketAgentKey.Query().Where(ticketagentkey.KeyHash(shaHash(secret)), ticketagentkey.DeletedAtIsNil()).Only(ctx)
	if dbent.IsNotFound(e) {
		return fail("AGENT_KEY_INVALID", true)
	}
	if e != nil {
		return nil, zero, e
	}
	if k.Status != TicketAgentKeyActive {
		return fail("AGENT_KEY_DISABLED", true)
	}
	if k.ExpiresAt != nil && !k.ExpiresAt.After(time.Now()) {
		return fail("AGENT_KEY_EXPIRED", true)
	}
	if s.userRepo == nil {
		return fail("AGENT_KEY_INVALID", true)
	}
	u, e := s.userRepo.GetByID(ctx, k.UserID)
	if e != nil || u == nil || !u.IsActive() || u.DeletedAt != nil {
		return fail("AGENT_KEY_INVALID", true)
	}
	if k.Scope == TicketAgentScopeAdmin && !u.IsAdmin() {
		return fail("AGENT_SCOPE_FORBIDDEN", true)
	}
	if k.Scope != TicketAgentScopeUser && k.Scope != TicketAgentScopeAdmin {
		return fail("AGENT_KEY_INVALID", true)
	}
	keyLimit := k.QPSLimit
	if keyLimit > st.AgentMaxQPS {
		keyLimit = st.AgentMaxQPS
	}
	if keyLimit < 1 {
		return fail("AGENT_KEY_INVALID", true)
	}
	ownerLimit := st.AgentUserTotalQPS
	if k.Scope == TicketAgentScopeAdmin {
		ownerLimit = st.AgentAdminTotalQPS
	}
	if ownerLimit < 1 {
		ownerLimit = 1
	}
	if s.limiter == nil {
		return nil, zero, ticketAgentError("AGENT_RATE_LIMIT_UNAVAILABLE")
	}
	kr, e := s.limiter.Allow(ctx, fmt.Sprintf("ticket_agent:key:%d", k.ID), keyLimit, time.Second)
	if e != nil {
		return nil, zero, ticketAgentError("AGENT_RATE_LIMIT_UNAVAILABLE")
	}
	if !kr.Allowed {
		return nil, TicketAgentRate{Limit: keyLimit, Remaining: 0, RetryAfter: kr.RetryAfter}, ticketAgentError("AGENT_RATE_LIMITED")
	}
	or, e := s.limiter.Allow(ctx, fmt.Sprintf("ticket_agent:owner:%d:%s", k.UserID, k.Scope), ownerLimit, time.Second)
	if e != nil {
		return nil, zero, ticketAgentError("AGENT_RATE_LIMIT_UNAVAILABLE")
	}
	if !or.Allowed {
		return nil, TicketAgentRate{Limit: ownerLimit, Remaining: 0, RetryAfter: or.RetryAfter}, ticketAgentError("AGENT_RATE_LIMITED")
	}
	rem := keyLimit - int(kr.Count)
	if ownerRemaining := ownerLimit - int(or.Count); ownerRemaining < rem {
		rem = ownerRemaining
	}
	if rem < 0 {
		rem = 0
	}
	// Redis 在跨进程维度节流，数据库条件同时兜底，避免频繁写入。
	used, err := s.redis.SetNX(ctx, fmt.Sprintf("ticket_agent:last_used:%d", k.ID), "1", time.Minute).Result()
	if err != nil {
		return nil, zero, ticketAgentError("AGENT_RATE_LIMIT_UNAVAILABLE")
	}
	if used {
		s.enqueueLastUsed(k, ip)
	}
	return &TicketAgentPrincipal{Key: viewKey(k), Owner: TicketAgentOwner{ID: u.ID, Role: u.Role, Username: u.Username}, OwnerTotalQPS: ownerLimit, MaxBodyLength: st.MaxBodyLength}, TicketAgentRate{Limit: keyLimit, Remaining: rem}, nil
}

type idempotencyEnvelope struct {
	Status int             `json:"status"`
	Data   json.RawMessage `json:"data"`
}

const (
	ticketAgentIdempotencyPendingTTL   = 2 * time.Minute
	ticketAgentIdempotencyDoneTTL      = 24 * time.Hour
	ticketAgentIdempotencyWriteTimeout = 2 * time.Second
)

var ticketAgentIdempotencyClaim = redis.NewScript(`
if redis.call('EXISTS', KEYS[1]) == 0 then
  redis.call('HSET', KEYS[1], 'operation', ARGV[1], 'owner', ARGV[2], 'state', 'pending')
  redis.call('EXPIRE', KEYS[1], ARGV[3])
  return {'new', ''}
end
if redis.call('HGET', KEYS[1], 'operation') ~= ARGV[1] then return {'mismatch', ''} end
if redis.call('HGET', KEYS[1], 'state') == 'done' then
  return {'done', redis.call('HGET', KEYS[1], 'result')}
end
return {'pending', ''}
`)

var ticketAgentIdempotencyComplete = redis.NewScript(`
if redis.call('HGET', KEYS[1], 'operation') ~= ARGV[1]
  or redis.call('HGET', KEYS[1], 'owner') ~= ARGV[2]
  or redis.call('HGET', KEYS[1], 'state') ~= 'pending' then return 0 end
redis.call('HSET', KEYS[1], 'state', 'done', 'result', ARGV[3])
redis.call('EXPIRE', KEYS[1], ARGV[4])
return 1
`)

var ticketAgentIdempotencyRelease = redis.NewScript(`
if redis.call('HGET', KEYS[1], 'operation') ~= ARGV[1]
  or redis.call('HGET', KEYS[1], 'owner') ~= ARGV[2]
  or redis.call('HGET', KEYS[1], 'state') ~= 'pending' then return 0 end
redis.call('DEL', KEYS[1])
return 1
`)

func (s *TicketAgentService) releaseIdempotency(ctx context.Context, keyID int64, redisKey, fingerprint, owner string) {
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), ticketAgentIdempotencyWriteTimeout)
	defer cancel()
	if err := ticketAgentIdempotencyRelease.Run(cleanupCtx, s.redis, []string{redisKey}, fingerprint, owner).Err(); err != nil {
		slog.Warn("release ticket agent idempotency lease failed", "key_id", keyID, "error", err)
	}
}

func (s *TicketAgentService) RunIdempotent(ctx context.Context, keyID int64, token, operation string, fn func(context.Context) (int, []byte, error)) (int, []byte, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return fn(ctx)
	}
	if len(token) > 200 {
		return 0, nil, ticketAgentError("TICKET_INVALID_INPUT")
	}
	if s.redis == nil {
		return 0, nil, ticketAgentError("AGENT_RATE_LIMIT_UNAVAILABLE")
	}
	redisKey := fmt.Sprintf("ticket_agent:idempotency:%d:%s", keyID, shaHash(token))
	fingerprint := shaHash(operation)
	ownerBytes := make([]byte, 32)
	if _, err := rand.Read(ownerBytes); err != nil {
		return 0, nil, ticketAgentError("AGENT_RATE_LIMIT_UNAVAILABLE").WithCause(err)
	}
	owner := base64.RawURLEncoding.EncodeToString(ownerBytes)
	var claimed bool
	for i := 0; i < 21; i++ {
		values, err := ticketAgentIdempotencyClaim.Run(ctx, s.redis, []string{redisKey}, fingerprint, owner, int(ticketAgentIdempotencyPendingTTL/time.Second)).StringSlice()
		if err != nil || len(values) != 2 {
			return 0, nil, ticketAgentError("AGENT_RATE_LIMIT_UNAVAILABLE")
		}
		switch values[0] {
		case "new":
			claimed = true
		case "mismatch":
			return 0, nil, ticketAgentError("TICKET_INVALID_INPUT")
		case "done":
			var env idempotencyEnvelope
			if err := json.Unmarshal([]byte(values[1]), &env); err != nil {
				return 0, nil, ticketAgentError("AGENT_RATE_LIMIT_UNAVAILABLE")
			}
			return env.Status, env.Data, nil
		}
		if claimed {
			break
		}
		timer := time.NewTimer(50 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return 0, nil, ctx.Err()
		case <-timer.C:
		}
	}
	if !claimed {
		return 0, nil, ticketAgentError("AGENT_RATE_LIMITED")
	}
	status, data, err := fn(ctx)
	if err != nil {
		s.releaseIdempotency(ctx, keyID, redisKey, fingerprint, owner)
		return status, data, err
	}
	raw, err := json.Marshal(idempotencyEnvelope{Status: status, Data: data})
	if err != nil {
		return 0, nil, fmt.Errorf("encode ticket idempotent response: %w", err)
	}
	// 客户端断开后仍短暂保存已提交结果；失败时保留两分钟租约，避免立即重复执行。
	persistCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), ticketAgentIdempotencyWriteTimeout)
	defer cancel()
	result, err := ticketAgentIdempotencyComplete.Run(persistCtx, s.redis, []string{redisKey}, fingerprint, owner, raw, int(ticketAgentIdempotencyDoneTTL/time.Second)).Int()
	if err != nil || result != 1 {
		slog.Warn("complete ticket agent idempotency lease failed", "key_id", keyID, "error", err)
		return 0, nil, ticketAgentError("AGENT_RATE_LIMIT_UNAVAILABLE")
	}
	return status, data, nil
}
