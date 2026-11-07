package service

// 工单领域服务。这里采用原生 SQL 读写，避免把工单生命周期耦合到 Ent 生成代码的查询构造器；事务边界仍由本服务明确控制。

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base32"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Wei-Shaw/sub2api/internal/middleware"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

var ticketAlphabet = base32.StdEncoding.WithPadding(base32.NoPadding)

func ticketError(reason string) *infraerrors.ApplicationError {
	status := 400
	switch reason {
	case "TICKET_SYSTEM_DISABLED":
		status = 403
	case "TICKET_NOT_FOUND":
		status = 404
	case "TICKET_CLOSED", "TICKET_INVALID_TRANSITION":
		status = 409
	case "TICKET_OPEN_LIMIT", "TICKET_CREATE_RATE_LIMITED", "TICKET_MESSAGE_RATE_LIMITED", "TICKET_NOTIFY_TEST_RATE_LIMITED":
		status = 429
	case "TICKET_NOTIFY_FAILED":
		status = 502
	case "AGENT_API_DISABLED", "AGENT_SCOPE_FORBIDDEN":
		status = 403
	case "AGENT_KEY_MISSING", "AGENT_KEY_INVALID", "AGENT_KEY_DISABLED", "AGENT_KEY_EXPIRED":
		status = 401
	case "AGENT_KEY_LIMIT", "AGENT_INVALID_TRANSITION":
		status = 409
	case "AGENT_QPS_INVALID", "NOTIFY_WEBHOOK_INVALID":
		status = 400
	case "AGENT_RATE_LIMITED", "AGENT_AUTH_BLOCKED":
		status = 429
	case "AGENT_RATE_LIMIT_UNAVAILABLE":
		status = 503
	}
	return infraerrors.New(status, reason, strings.ToLower(strings.ReplaceAll(strings.TrimPrefix(reason, "TICKET_"), "_", " ")))
}

type TicketSummary struct {
	ID            int64      `json:"id"`
	TicketNo      string     `json:"ticket_no"`
	Subject       string     `json:"subject"`
	Category      string     `json:"category"`
	Priority      string     `json:"priority"`
	Status        string     `json:"status"`
	UserID        int64      `json:"user_id"`
	AssigneeID    *int64     `json:"assignee_id"`
	Unread        bool       `json:"unread"`
	MessageCount  int        `json:"message_count"`
	LastMessageAt time.Time  `json:"last_message_at"`
	LastMessageBy string     `json:"last_message_by"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	ClosedAt      *time.Time `json:"closed_at"`
}
type AdminTicketSummary struct {
	TicketSummary
	UserEmail    string  `json:"user_email"`
	UserUsername string  `json:"user_username"`
	AssigneeName *string `json:"assignee_name"`
	UserUnread   bool    `json:"user_unread"`
	AdminUnread  bool    `json:"admin_unread"`
}
type TicketMessage struct {
	ID         int64     `json:"id"`
	TicketID   int64     `json:"ticket_id"`
	AuthorType string    `json:"author_type"`
	AuthorID   *int64    `json:"author_id"`
	AuthorName string    `json:"author_name"`
	Via        string    `json:"via"`
	Internal   bool      `json:"internal"`
	Body       string    `json:"body"`
	CreatedAt  time.Time `json:"created_at"`
}
type TicketDetail struct {
	Ticket   TicketSummary   `json:"ticket"`
	Messages []TicketMessage `json:"messages"`
}
type AdminTicketDetail struct {
	Ticket   AdminTicketSummary `json:"ticket"`
	Messages []TicketMessage    `json:"messages"`
}
type TicketStats struct {
	PendingAdmin int64 `json:"pending_admin"`
	PendingUser  int64 `json:"pending_user"`
	Resolved     int64 `json:"resolved"`
	Closed       int64 `json:"closed"`
	Unread       int64 `json:"unread"`
}
type TicketCreateInput struct {
	Subject  string `json:"subject"`
	Category string `json:"category"`
	Priority string `json:"priority"`
	Body     string `json:"body"`
}
type TicketMessageInput struct {
	Body     string `json:"body"`
	Internal bool   `json:"internal"`
}
type TicketUpdateInput struct {
	Status     *string `json:"status"`
	Priority   *string `json:"priority"`
	AssigneeID *int64  `json:"assignee_id"`
}
type TicketListFilter struct {
	Page, PageSize                      int
	Status, Priority, Category, Keyword string
	UserID, AssigneeID                  int64
	UnreadOnly                          bool
}

type TicketService struct {
	db       *sql.DB
	settings *SettingService
	limiter  *middleware.RateLimiter
	notify   *TicketNotificationService
}

func NewTicketService(db *sql.DB, settings *SettingService, limiter *middleware.RateLimiter, notify *TicketNotificationService) *TicketService {
	return &TicketService{db: db, settings: settings, limiter: limiter, notify: notify}
}

func (s *TicketService) checkEnabled(ctx context.Context) error {
	if s.settings == nil || !s.settings.IsTicketSystemEnabled(ctx) {
		return ticketError("TICKET_SYSTEM_DISABLED")
	}
	return nil
}
func validTicketCategory(v string) bool {
	switch v {
	case "general", "billing", "technical", "account", "feature", "other":
		return true
	}
	return false
}
func validTicketPriority(v string) bool {
	switch v {
	case "low", "normal", "high", "urgent":
		return true
	}
	return false
}
func validTicketStatus(v string) bool {
	switch v {
	case "pending_admin", "pending_user", "resolved", "closed":
		return true
	}
	return false
}
func validateText(v string, max int) bool {
	return utf8.ValidString(v) && !strings.ContainsRune(v, 0) && utf8.RuneCountInString(v) > 0 && utf8.RuneCountInString(v) <= max
}

func (s *TicketService) businessLimit(ctx context.Context, key string, limit int, window time.Duration, reason string) error {
	if s.limiter == nil {
		return ticketError("AGENT_RATE_LIMIT_UNAVAILABLE")
	}
	result, err := s.limiter.Allow(ctx, "ticket:business:"+key, limit, window)
	if err != nil {
		return ticketError("AGENT_RATE_LIMIT_UNAVAILABLE").WithCause(err)
	}
	if !result.Allowed {
		seconds := int64((result.RetryAfter + time.Second - 1) / time.Second)
		if seconds < 1 {
			seconds = 1
		}
		return ticketError(reason).WithMetadata(map[string]string{"retry_after": fmt.Sprint(seconds)})
	}
	return nil
}

func (s *TicketService) ticketNo() (string, error) {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "TK" + time.Now().Format("20060102") + ticketAlphabet.EncodeToString(b)[:6], nil
}

type ticketSQL interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

type ticketRowScanner interface{ Scan(...any) error }

func scanTicketSummary(row ticketRowScanner) (TicketSummary, error) {
	var t TicketSummary
	err := row.Scan(&t.ID, &t.TicketNo, &t.Subject, &t.Category, &t.Priority, &t.Status, &t.UserID, &t.AssigneeID, &t.Unread, &t.MessageCount, &t.LastMessageAt, &t.LastMessageBy, &t.CreatedAt, &t.UpdatedAt, &t.ClosedAt)
	return t, err
}

// 公共汇总只使用公开消息；内部备注不改变其数量、时间与排序。
func ticketSummaryColumns(admin bool) string {
	unread := "t.user_unread"
	if admin {
		unread = "t.admin_unread"
	}
	return "t.id,t.ticket_no,t.subject,t.category,t.priority,t.status,t.user_id,t.assignee_id," + unread + ",t.message_count,t.last_message_at,t.last_message_by,t.created_at,t.updated_at,t.closed_at"
}

func summaryQuery(ctx context.Context, q ticketSQL, id int64, admin bool) (TicketSummary, error) {
	t, err := scanTicketSummary(q.QueryRowContext(ctx, "SELECT "+ticketSummaryColumns(admin)+" FROM tickets t WHERE t.id=$1", id))
	if errors.Is(err, sql.ErrNoRows) {
		return t, ticketError("TICKET_NOT_FOUND")
	}
	return t, err
}

func lockTicketOwner(ctx context.Context, tx *sql.Tx, userID int64) error {
	_, err := tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", fmt.Sprintf("ticket-owner:%d", userID))
	return err
}

func checkOpenTicketLimit(ctx context.Context, tx *sql.Tx, userID int64, max int) error {
	var open int
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM tickets WHERE user_id=$1 AND status IN ('pending_admin','pending_user')", userID).Scan(&open); err != nil {
		return err
	}
	if open >= max {
		return ticketError("TICKET_OPEN_LIMIT")
	}
	return nil
}

func ticketLookupError(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return ticketError("TICKET_NOT_FOUND")
	}
	return err
}

// 全站消息分配锁确保提交顺序与递增游标一致，避免增量轮询跳过晚提交的小 ID。
func lockTicketMessageOrder(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(hashtextextended('ticket-message-order',0))")
	return err
}

func (s *TicketService) CreateTicket(ctx context.Context, userID int64, via string, input TicketCreateInput, agentKeyID *int64) (*TicketDetail, error) {
	if err := s.checkEnabled(ctx); err != nil {
		return nil, err
	}
	cfg, err := s.settings.GetTicketSettings(ctx)
	if err != nil {
		return nil, err
	}
	if input.Category == "" {
		input.Category = "general"
	}
	if input.Priority == "" {
		input.Priority = "normal"
	}
	input.Subject = strings.TrimSpace(input.Subject)
	if !validateText(input.Subject, cfg.MaxSubjectLength) || !validateText(input.Body, cfg.MaxBodyLength) || strings.TrimSpace(input.Body) == "" || !validTicketCategory(input.Category) || !validTicketPriority(input.Priority) || (via != "web" && via != "agent") {
		return nil, ticketError("TICKET_INVALID_INPUT")
	}
	if err = s.businessLimit(ctx, fmt.Sprintf("create:%d", userID), cfg.CreatePerHourPerUser, time.Hour, "TICKET_CREATE_RATE_LIMITED"); err != nil {
		return nil, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	if err = lockTicketOwner(ctx, tx, userID); err != nil {
		return nil, err
	}
	if err = checkOpenTicketLimit(ctx, tx, userID, cfg.MaxOpenTicketsPerUser); err != nil {
		return nil, err
	}
	if err = lockTicketMessageOrder(ctx, tx); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	var id int64
	// 随机工单号允许碰撞时重试，不让唯一索引冲突中止整笔事务。
	for attempt := 0; attempt < 8; attempt++ {
		no, randomErr := s.ticketNo()
		if randomErr != nil {
			return nil, randomErr
		}
		err = tx.QueryRowContext(ctx, `INSERT INTO tickets(ticket_no,user_id,subject,category,priority,status,user_unread,admin_unread,message_count,last_message_at,last_message_by,created_at,updated_at) VALUES($1,$2,$3,$4,$5,'pending_admin',false,true,1,$6,'user',$6,$6) ON CONFLICT(ticket_no) DO NOTHING RETURNING id`, no, userID, input.Subject, input.Category, input.Priority, now).Scan(&id)
		if err == nil {
			break
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
	}
	if err != nil {
		return nil, fmt.Errorf("allocate unique ticket number: %w", err)
	}

	var mid int64
	err = tx.QueryRowContext(ctx, `INSERT INTO ticket_messages(ticket_id,author_type,author_id,via,agent_key_id,body,created_at) VALUES($1,'user',$2,$3,$4,$5,$6) RETURNING id`, id, userID, via, agentKeyID, input.Body, now).Scan(&mid)
	if err != nil {
		return nil, err
	}
	ticket, err := summaryQuery(ctx, tx, id, false)
	if err != nil {
		return nil, err
	}
	messages, err := queryTicketMessages(ctx, tx, id, false, 0)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	if s.notify != nil {
		s.notify.Enqueue(TicketNotificationEvent{TicketID: id, ActorID: userID, ToAdmin: true, Created: true, Excerpt: input.Body})
	}
	return &TicketDetail{Ticket: ticket, Messages: messages}, nil
}

func queryTicketMessages(ctx context.Context, q ticketSQL, id int64, admin bool, after int64) ([]TicketMessage, error) {
	where := "m.ticket_id=$1 AND m.id>$2"
	if !admin {
		where += " AND NOT m.internal"
	}
	rows, err := q.QueryContext(ctx, `SELECT m.id,m.ticket_id,m.author_type,m.author_id,m.via,m.internal,m.body,m.created_at,COALESCE(NULLIF(u.username,''),u.email,'') FROM ticket_messages m LEFT JOIN users u ON u.id=m.author_id WHERE `+where+" ORDER BY m.id", id, after)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	messages := []TicketMessage{}
	for rows.Next() {
		var m TicketMessage
		if err = rows.Scan(&m.ID, &m.TicketID, &m.AuthorType, &m.AuthorID, &m.Via, &m.Internal, &m.Body, &m.CreatedAt, &m.AuthorName); err != nil {
			return nil, err
		}
		m = ticketMessageVisibility(m, admin)
		messages = append(messages, m)
	}
	return messages, rows.Err()
}

func ticketMessageVisibility(m TicketMessage, admin bool) TicketMessage {
	if m.AuthorType == "system" {
		m.AuthorName = "System"
		if !admin {
			m.AuthorID = nil
		}
	}
	if !admin && m.AuthorType == "admin" {
		m.AuthorID = nil
		m.AuthorName = "Support"
	}
	return m
}

func (s *TicketService) GetTicket(ctx context.Context, viewerID, id int64, admin bool, after int64) (*TicketDetail, error) {
	if err := s.checkEnabled(ctx); err != nil {
		return nil, err
	}
	if after < 0 {
		return nil, ticketError("TICKET_INVALID_INPUT")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	var owner int64
	if err = tx.QueryRowContext(ctx, "SELECT user_id FROM tickets WHERE id=$1 FOR UPDATE", id).Scan(&owner); err != nil {
		return nil, ticketLookupError(err)
	}
	if !admin && owner != viewerID {
		return nil, ticketError("TICKET_NOT_FOUND")
	}
	messages, err := queryTicketMessages(ctx, tx, id, admin, after)
	if err != nil {
		return nil, err
	}
	column := "user_unread"
	if admin {
		column = "admin_unread"
	}
	if _, err = tx.ExecContext(ctx, "UPDATE tickets SET "+column+"=false WHERE id=$1", id); err != nil {
		return nil, err
	}
	ticket, err := summaryQuery(ctx, tx, id, admin)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return &TicketDetail{Ticket: ticket, Messages: messages}, nil
}

func (s *TicketService) AddMessage(ctx context.Context, viewerID, id int64, admin bool, via string, agentKeyID *int64, input TicketMessageInput) (*TicketMessage, error) {
	if err := s.checkEnabled(ctx); err != nil {
		return nil, err
	}
	cfg, err := s.settings.GetTicketSettings(ctx)
	if err != nil {
		return nil, err
	}
	if !validateText(input.Body, cfg.MaxBodyLength) || strings.TrimSpace(input.Body) == "" || (input.Internal && !admin) || (via != "web" && via != "agent") {
		return nil, ticketError("TICKET_INVALID_INPUT")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	// 所有会增加开放工单数的路径先获取同一 owner 锁，保持锁序一致。
	if !admin {
		if err = lockTicketOwner(ctx, tx, viewerID); err != nil {
			return nil, err
		}
	}
	var status string
	var owner int64
	if err = tx.QueryRowContext(ctx, "SELECT user_id,status FROM tickets WHERE id=$1 FOR UPDATE", id).Scan(&owner, &status); err != nil {
		return nil, ticketLookupError(err)
	}
	if !admin && owner != viewerID {
		return nil, ticketError("TICKET_NOT_FOUND")
	}
	if !admin && status == "closed" {
		return nil, ticketError("TICKET_CLOSED")
	}
	if !admin && status == "resolved" {
		if err = checkOpenTicketLimit(ctx, tx, owner, cfg.MaxOpenTicketsPerUser); err != nil {
			return nil, err
		}
	}
	if !admin {
		if err = s.businessLimit(ctx, fmt.Sprintf("message:%d", viewerID), cfg.MessagesPerMinutePerUser, time.Minute, "TICKET_MESSAGE_RATE_LIMITED"); err != nil {
			return nil, err
		}
	}
	if err = lockTicketMessageOrder(ctx, tx); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	authorType := "user"
	if admin {
		authorType = "admin"
	}
	var mid int64
	err = tx.QueryRowContext(ctx, `INSERT INTO ticket_messages(ticket_id,author_type,author_id,via,agent_key_id,internal,body,created_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id`, id, authorType, viewerID, via, agentKeyID, input.Internal, input.Body, now).Scan(&mid)
	if err != nil {
		return nil, err
	}
	if !input.Internal {
		status = "pending_admin"
		if admin {
			status = "pending_user"
		}
		// 发件不清除本方未读；只有 GET 详情确认已读。
		if admin {
			_, err = tx.ExecContext(ctx, `UPDATE tickets SET status=$2,closed_at=NULL,message_count=message_count+1,last_message_at=$3,last_message_by=$4,updated_at=$3,user_unread=true WHERE id=$1`, id, status, now, authorType)
		} else {
			_, err = tx.ExecContext(ctx, `UPDATE tickets SET status=$2,closed_at=NULL,message_count=message_count+1,last_message_at=$3,last_message_by=$4,updated_at=$3,admin_unread=true WHERE id=$1`, id, status, now, authorType)
		}
		if err != nil {
			return nil, err
		}
	}
	var name string
	if err = tx.QueryRowContext(ctx, "SELECT COALESCE(NULLIF(username,''),email) FROM users WHERE id=$1", viewerID).Scan(&name); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	message := ticketMessageVisibility(TicketMessage{ID: mid, TicketID: id, AuthorType: authorType, AuthorID: &viewerID, AuthorName: name, Via: via, Internal: input.Internal, Body: input.Body, CreatedAt: now}, admin)
	if !input.Internal && s.notify != nil {
		s.notify.Enqueue(TicketNotificationEvent{TicketID: id, ActorID: viewerID, ToAdmin: !admin, Excerpt: input.Body})
	}
	return &message, nil
}

func (s *TicketService) CloseReopen(ctx context.Context, userID, id int64, reopen bool) (*TicketSummary, error) {
	if err := s.checkEnabled(ctx); err != nil {
		return nil, err
	}
	cfg, err := s.settings.GetTicketSettings(ctx)
	if err != nil {
		return nil, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	if err = lockTicketOwner(ctx, tx, userID); err != nil {
		return nil, err
	}
	var status string
	var owner int64
	if err = tx.QueryRowContext(ctx, "SELECT user_id,status FROM tickets WHERE id=$1 FOR UPDATE", id).Scan(&owner, &status); err != nil {
		return nil, ticketLookupError(err)
	}
	if owner != userID {
		return nil, ticketError("TICKET_NOT_FOUND")
	}
	if reopen && (status != "resolved" && status != "closed") {
		return nil, ticketError("TICKET_INVALID_TRANSITION")
	}
	if !reopen && status == "closed" {
		return nil, ticketError("TICKET_CLOSED")
	}
	newStatus := "closed"
	var closed any = time.Now().UTC()
	if reopen {
		if err = checkOpenTicketLimit(ctx, tx, userID, cfg.MaxOpenTicketsPerUser); err != nil {
			return nil, err
		}
		newStatus = "pending_admin"
		closed = nil
	}
	_, err = tx.ExecContext(ctx, "UPDATE tickets SET status=$2,closed_at=$3,updated_at=NOW(),admin_unread=true WHERE id=$1", id, newStatus, closed)
	if err != nil {
		return nil, err
	}
	ticket, err := summaryQuery(ctx, tx, id, false)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return &ticket, nil
}

func ticketListWhere(filter TicketListFilter, admin bool, viewerID int64) (string, []any, error) {
	if (filter.Status != "" && !validTicketStatus(filter.Status)) || (filter.Priority != "" && !validTicketPriority(filter.Priority)) || (filter.Category != "" && !validTicketCategory(filter.Category)) || filter.UserID < 0 || filter.AssigneeID < 0 {
		return "", nil, ticketError("TICKET_INVALID_INPUT")
	}
	where := []string{"1=1"}
	args := []any{}
	add := func(column string, value any) {
		args = append(args, value)
		where = append(where, column+"=$"+fmt.Sprint(len(args)))
	}
	if !admin {
		add("t.user_id", viewerID)
	} else if filter.UserID > 0 {
		add("t.user_id", filter.UserID)
	}
	for _, f := range []struct{ value, column string }{{filter.Status, "t.status"}, {filter.Priority, "t.priority"}, {filter.Category, "t.category"}} {
		if f.value != "" {
			add(f.column, f.value)
		}
	}
	if filter.AssigneeID > 0 {
		add("t.assignee_id", filter.AssigneeID)
	}
	if filter.UnreadOnly {
		if admin {
			where = append(where, "t.admin_unread")
		} else {
			where = append(where, "t.user_unread")
		}
	}
	if filter.Keyword != "" {
		args = append(args, "%"+strings.NewReplacer(`\`, `\\`, "%", `\%`, "_", `\_`).Replace(filter.Keyword)+"%")
		p := "$" + fmt.Sprint(len(args))
		clause := "(t.subject ILIKE " + p + " OR t.ticket_no ILIKE " + p
		if admin {
			clause += " OR EXISTS(SELECT 1 FROM users search_user WHERE search_user.id=t.user_id AND search_user.email ILIKE " + p + ")"
		}
		where = append(where, clause+")")
	}
	return " FROM tickets t WHERE " + strings.Join(where, " AND "), args, nil
}

func (s *TicketService) ListTickets(ctx context.Context, filter TicketListFilter, admin bool, viewerID int64) ([]TicketSummary, int64, error) {
	if err := s.checkEnabled(ctx); err != nil {
		return nil, 0, err
	}
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 {
		filter.PageSize = 20
	}
	if filter.PageSize > 100 {
		filter.PageSize = 100
	}
	base, args, err := ticketListWhere(filter, admin, viewerID)
	if err != nil {
		return nil, 0, err
	}
	var total int64
	if err = s.db.QueryRowContext(ctx, "SELECT COUNT(*)"+base, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, filter.PageSize, int64(filter.Page-1)*int64(filter.PageSize))
	rows, err := s.db.QueryContext(ctx, "SELECT "+ticketSummaryColumns(admin)+base+" ORDER BY t.updated_at DESC,t.id DESC LIMIT $"+fmt.Sprint(len(args)-1)+" OFFSET $"+fmt.Sprint(len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()
	out := []TicketSummary{}
	for rows.Next() {
		ticket, e := scanTicketSummary(rows)
		if e != nil {
			return nil, 0, e
		}
		out = append(out, ticket)
	}
	return out, total, rows.Err()
}

func adminTicketSummary(ctx context.Context, q ticketSQL, t TicketSummary) (AdminTicketSummary, error) {
	a := AdminTicketSummary{TicketSummary: t}
	err := q.QueryRowContext(ctx, `SELECT u.email,u.username,COALESCE(NULLIF(au.username,''),au.email),t.user_unread,t.admin_unread FROM tickets t JOIN users u ON u.id=t.user_id LEFT JOIN users au ON au.id=t.assignee_id WHERE t.id=$1`, t.ID).Scan(&a.UserEmail, &a.UserUsername, &a.AssigneeName, &a.UserUnread, &a.AdminUnread)
	return a, err
}

func (s *TicketService) ListAdmin(ctx context.Context, f TicketListFilter) ([]AdminTicketSummary, int64, error) {
	items, total, err := s.ListTickets(ctx, f, true, 0)
	if err != nil {
		return nil, 0, err
	}
	out := make([]AdminTicketSummary, 0, len(items))
	for _, ticket := range items {
		a, e := adminTicketSummary(ctx, s.db, ticket)
		if e != nil {
			return nil, 0, e
		}
		out = append(out, a)
	}
	return out, total, nil
}

func (s *TicketService) GetAdminTicket(ctx context.Context, id int64) (*AdminTicketDetail, error) {
	d, err := s.GetTicket(ctx, 0, id, true, 0)
	if err != nil {
		return nil, err
	}
	a, err := adminTicketSummary(ctx, s.db, d.Ticket)
	if err != nil {
		return nil, err
	}
	return &AdminTicketDetail{Ticket: a, Messages: d.Messages}, nil
}

func (s *TicketService) UpdateAdmin(ctx context.Context, id int64, input TicketUpdateInput, actorID int64) (*AdminTicketSummary, error) {
	if input.Status == nil && input.Priority == nil && input.AssigneeID == nil {
		return nil, ticketError("TICKET_INVALID_INPUT")
	}
	if err := s.checkEnabled(ctx); err != nil {
		return nil, err
	}
	if (input.Status != nil && !validTicketStatus(*input.Status)) || (input.Priority != nil && !validTicketPriority(*input.Priority)) || (input.AssigneeID != nil && *input.AssigneeID < 0) {
		return nil, ticketError("TICKET_INVALID_INPUT")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	var status string
	if err = tx.QueryRowContext(ctx, "SELECT status FROM tickets WHERE id=$1 FOR UPDATE", id).Scan(&status); err != nil {
		return nil, ticketLookupError(err)
	}
	if input.AssigneeID != nil && *input.AssigneeID != 0 {
		var valid bool
		if err = tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM users WHERE id=$1 AND role='admin' AND status='active' AND deleted_at IS NULL)", *input.AssigneeID).Scan(&valid); err != nil {
			return nil, err
		}
		if !valid {
			return nil, ticketError("TICKET_INVALID_INPUT")
		}
	}
	now := time.Now().UTC()
	newStatus := status
	if input.Status != nil {
		newStatus = *input.Status
	}
	assign := any(nil)
	if input.AssigneeID != nil && *input.AssigneeID != 0 {
		assign = *input.AssigneeID
	}
	_, err = tx.ExecContext(ctx, `UPDATE tickets SET status=$2::varchar(20),priority=COALESCE($3::varchar(20),priority),assignee_id=CASE WHEN $4::boolean THEN $5::bigint ELSE assignee_id END,closed_at=CASE WHEN $2::varchar(20)='closed' THEN COALESCE(closed_at,$6::timestamptz) ELSE NULL END,updated_at=$6::timestamptz WHERE id=$1`, id, newStatus, input.Priority, input.AssigneeID != nil, assign, now)
	if err != nil {
		return nil, err
	}
	notify := newStatus != status && (newStatus == "resolved" || newStatus == "closed")
	if notify {
		if err = lockTicketMessageOrder(ctx, tx); err != nil {
			return nil, err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO ticket_messages(ticket_id,author_type,author_id,via,internal,body,created_at) VALUES($1,'system',$2,'system',false,$3,$4)`, id, actorID, "status_changed:"+newStatus, now)
		if err != nil {
			return nil, err
		}
		_, err = tx.ExecContext(ctx, `UPDATE tickets SET message_count=message_count+1,last_message_at=$2,last_message_by='system',user_unread=true WHERE id=$1`, id, now)
		if err != nil {
			return nil, err
		}
	}
	t, err := summaryQuery(ctx, tx, id, true)
	if err != nil {
		return nil, err
	}
	a, err := adminTicketSummary(ctx, tx, t)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	if notify && s.notify != nil {
		s.notify.Enqueue(TicketNotificationEvent{TicketID: id, ActorID: actorID, ToAdmin: false, Excerpt: "status_changed:" + newStatus})
	}
	return &a, nil
}

func (s *TicketService) Stats(ctx context.Context) (*TicketStats, error) {
	if err := s.checkEnabled(ctx); err != nil {
		return nil, err
	}
	var x TicketStats
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FILTER(WHERE status='pending_admin'),COUNT(*) FILTER(WHERE status='pending_user'),COUNT(*) FILTER(WHERE status='resolved'),COUNT(*) FILTER(WHERE status='closed'),COUNT(*) FILTER(WHERE admin_unread) FROM tickets`).Scan(&x.PendingAdmin, &x.PendingUser, &x.Resolved, &x.Closed, &x.Unread)
	return &x, err
}

func (s *TicketService) UnreadCount(ctx context.Context, userID int64) (int64, error) {
	if err := s.checkEnabled(ctx); err != nil {
		return 0, err
	}
	var count int64
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM tickets WHERE user_id=$1 AND user_unread", userID).Scan(&count)
	return count, err
}

type TicketAssignee struct {
	ID       int64  `json:"id"`
	Email    string `json:"email"`
	Username string `json:"username"`
}

func (s *TicketService) Assignees(ctx context.Context) ([]TicketAssignee, error) {
	if err := s.checkEnabled(ctx); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, "SELECT id,email,username FROM users WHERE role='admin' AND status='active' AND deleted_at IS NULL ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []TicketAssignee{}
	for rows.Next() {
		var a TicketAssignee
		if err = rows.Scan(&a.ID, &a.Email, &a.Username); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

type TicketAgentUpdate struct {
	Cursor  int64         `json:"cursor"`
	Ticket  TicketSummary `json:"ticket"`
	Message TicketMessage `json:"message"`
}
type TicketAgentUpdates struct {
	Items      []TicketAgentUpdate `json:"items"`
	NextCursor int64               `json:"next_cursor"`
	HasMore    bool                `json:"has_more"`
}

func (s *TicketService) Updates(ctx context.Context, viewerID int64, admin bool, cursor int64, limit int) (*TicketAgentUpdates, error) {
	if err := s.checkEnabled(ctx); err != nil {
		return nil, err
	}
	if cursor < 0 || limit < 0 {
		return nil, ticketError("TICKET_INVALID_INPUT")
	}
	if limit == 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	where := "m.id>$1"
	args := []any{cursor, limit + 1}
	if !admin {
		where += " AND t.user_id=$3 AND NOT m.internal"
		args = append(args, viewerID)
	}
	rows, err := s.db.QueryContext(ctx, `SELECT m.id,m.ticket_id,m.author_type,m.author_id,m.via,m.internal,m.body,m.created_at,COALESCE(NULLIF(u.username,''),u.email,'') FROM ticket_messages m JOIN tickets t ON t.id=m.ticket_id LEFT JOIN users u ON u.id=m.author_id WHERE `+where+" ORDER BY m.id LIMIT $2", args...)
	if err != nil {
		return nil, err
	}
	messages := []TicketMessage{}
	for rows.Next() {
		var m TicketMessage
		if err = rows.Scan(&m.ID, &m.TicketID, &m.AuthorType, &m.AuthorID, &m.Via, &m.Internal, &m.Body, &m.CreatedAt, &m.AuthorName); err != nil {
			_ = rows.Close()
			return nil, err
		}
		messages = append(messages, ticketMessageVisibility(m, admin))
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return nil, err
	}
	out := &TicketAgentUpdates{Items: []TicketAgentUpdate{}, NextCursor: cursor, HasMore: len(messages) > limit}
	if out.HasMore {
		messages = messages[:limit]
	}
	for _, m := range messages {
		ticket, e := summaryQuery(ctx, s.db, m.TicketID, admin)
		if e != nil {
			return nil, e
		}
		out.Items = append(out.Items, TicketAgentUpdate{Cursor: m.ID, Ticket: ticket, Message: m})
		out.NextCursor = m.ID
	}
	return out, nil
}
