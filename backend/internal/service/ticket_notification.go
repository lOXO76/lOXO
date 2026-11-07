package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/middleware"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const ticketNotificationShutdownTimeout = 10 * time.Second

type ticketNotificationEmailSender interface {
	Send(context.Context, NotificationEmailSendInput) error
}

type TicketNotifyPreference struct {
	EmailEnabled            bool   `json:"email_enabled"`
	Email                   string `json:"email"`
	FeishuEnabled           bool   `json:"feishu_enabled"`
	FeishuWebhookConfigured bool   `json:"feishu_webhook_configured"`
	FeishuWebhookURLMasked  string `json:"feishu_webhook_url_masked"`
	FeishuSecretConfigured  bool   `json:"feishu_secret_configured"`
}

type TicketNotifyPreferenceUpdate struct {
	EmailEnabled     *bool   `json:"email_enabled"`
	FeishuEnabled    *bool   `json:"feishu_enabled"`
	FeishuWebhookURL *string `json:"feishu_webhook_url"`
	FeishuSecret     *string `json:"feishu_secret"`
}

type TicketNotificationEvent struct {
	TicketID   int64
	ActorID    int64
	ToAdmin    bool
	Created    bool
	Excerpt    string
	TicketNo   string
	Subject    string
	Status     string
	ActorLabel string
	ownerID    int64
	assigneeID sql.NullInt64
	actorEmail string
}

type ticketNotificationRecipient struct {
	id       int64
	email    string
	name     string
	emailOn  bool
	feishuOn bool
	webhook  string
	secret   string
}

type TicketNotificationService struct {
	db              *sql.DB
	settings        *SettingService
	emails          ticketNotificationEmailSender
	nonce           RewardNonceStore
	limiter         *middleware.RateLimiter
	encryptor       SecretEncryptor
	queue           chan TicketNotificationEvent
	wg              sync.WaitGroup
	stopOnce        sync.Once
	stop            chan struct{}
	done            chan struct{}
	workerCtx       context.Context
	cancel          context.CancelFunc
	shutdownTimeout time.Duration
	mu              sync.RWMutex
	stopped         bool
	http            *http.Client
}

func NewTicketNotificationService(db *sql.DB, settings *SettingService, emails *NotificationEmailService, nonce RewardNonceStore, limiter *middleware.RateLimiter, encryptor SecretEncryptor) *TicketNotificationService {
	transport := &http.Transport{ForceAttemptHTTP2: true, TLSHandshakeTimeout: 5 * time.Second, ResponseHeaderTimeout: 5 * time.Second, IdleConnTimeout: 30 * time.Second, MaxIdleConns: 10}
	workerCtx, cancel := context.WithCancel(context.Background())
	s := &TicketNotificationService{db: db, settings: settings, nonce: nonce, limiter: limiter, encryptor: encryptor, queue: make(chan TicketNotificationEvent, 128), stop: make(chan struct{}), done: make(chan struct{}), workerCtx: workerCtx, cancel: cancel, shutdownTimeout: ticketNotificationShutdownTimeout, http: &http.Client{Timeout: 5 * time.Second, Transport: transport, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}}
	if emails != nil {
		s.emails = emails
	}
	if settings != nil {
		settings.ticketSettingsEncryptor = encryptor
	}
	for i := 0; i < 2; i++ {
		s.wg.Add(1)
		go s.worker()
	}
	go func() { s.wg.Wait(); close(s.done) }()
	return s
}

func (s *TicketNotificationService) worker() {
	defer s.wg.Done()
	for {
		if s.workerCtx.Err() != nil {
			return
		}
		select {
		case <-s.stop:
			s.drain()
			return
		default:
		}
		select {
		case <-s.workerCtx.Done():
			return
		case event := <-s.queue:
			s.processEvent(s.workerCtx, event)
		case <-s.stop:
			s.drain()
			return
		}
	}
}

func (s *TicketNotificationService) drain() {
	for {
		if s.workerCtx.Err() != nil {
			return
		}
		select {
		case <-s.workerCtx.Done():
			return
		case event := <-s.queue:
			s.processEvent(s.workerCtx, event)
		default:
			return
		}
	}
}

func (s *TicketNotificationService) processEvent(parent context.Context, event TicketNotificationEvent) {
	defer func() {
		if recovered := recover(); recovered != nil {
			slog.Error("ticket notification worker panic recovered", "ticket_id", event.TicketID)
		}
	}()
	ctx, cancel := context.WithTimeout(parent, 15*time.Second)
	defer cancel()
	if err := s.deliver(ctx, event); err != nil {
		slog.Warn("ticket notification delivery failed", "ticket_id", event.TicketID, "error", err)
	}
}

func (s *TicketNotificationService) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() {
		s.mu.Lock()
		s.stopped = true
		close(s.stop)
		s.mu.Unlock()
		timeout := s.shutdownTimeout
		if timeout <= 0 {
			timeout = ticketNotificationShutdownTimeout
		}
		timer := time.NewTimer(timeout)
		defer timer.Stop()
		defer s.cancel()
		// SMTP 实现可能忽略 ctx；关闭流程只在总期限内尽力排空。
		select {
		case <-s.done:
		case <-timer.C:
			s.cancel()
			slog.Warn("ticket notification shutdown timed out", "timeout", timeout)
		}
		if s.http != nil {
			s.http.CloseIdleConnections()
		}
	})
}

func (s *TicketNotificationService) Enqueue(event TicketNotificationEvent) {
	if s == nil {
		return
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.stopped {
		return
	}
	select {
	case s.queue <- event:
	default:
		slog.Warn("ticket notification queue full", "ticket_id", event.TicketID)
	}
}

func (s *TicketNotificationService) recipient(ctx context.Context, userID int64) (*ticketNotificationRecipient, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("ticket notification database is not configured")
	}
	r := &ticketNotificationRecipient{id: userID}
	err := s.db.QueryRowContext(ctx, `SELECT u.email,u.username,COALESCE(p.email_enabled,true),COALESCE(p.feishu_enabled,false),COALESCE(p.feishu_webhook_url_encrypted,''),COALESCE(p.feishu_secret_encrypted,'') FROM users u LEFT JOIN ticket_notify_preferences p ON p.user_id=u.id WHERE u.id=$1 AND u.status='active' AND u.deleted_at IS NULL`, userID).Scan(&r.email, &r.name, &r.emailOn, &r.feishuOn, &r.webhook, &r.secret)
	return r, err
}

func (s *TicketNotificationService) GetPreferences(ctx context.Context, userID int64) (*TicketNotifyPreference, error) {
	r, err := s.recipient(ctx, userID)
	if err != nil {
		return nil, err
	}
	result := &TicketNotifyPreference{EmailEnabled: r.emailOn, Email: r.email, FeishuEnabled: r.feishuOn, FeishuWebhookConfigured: r.webhook != "", FeishuSecretConfigured: r.secret != ""}
	if r.webhook != "" {
		if s.encryptor == nil {
			return nil, errors.New("ticket notification encryption is not configured")
		}
		value, err := s.encryptor.Decrypt(r.webhook)
		if err != nil {
			return nil, fmt.Errorf("decrypt ticket notification configuration: %w", err)
		}
		result.FeishuWebhookURLMasked = MaskTicketWebhookURL(value)
	}
	return result, nil
}

func (s *TicketNotificationService) UpdatePreferences(ctx context.Context, userID int64, input TicketNotifyPreferenceUpdate) (*TicketNotifyPreference, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("ticket notification database is not configured")
	}
	var webhook, secret *string
	if input.FeishuWebhookURL != nil {
		value := ""
		if err := updateTicketEncryptedValue(s.encryptor, input.FeishuWebhookURL, &value, true); err != nil {
			return nil, err
		}
		webhook = &value
	}
	if input.FeishuSecret != nil {
		value := ""
		if err := updateTicketEncryptedValue(s.encryptor, input.FeishuSecret, &value, false); err != nil {
			return nil, err
		}
		secret = &value
	}
	// 在一条 upsert 中合并可选字段，避免并发 PATCH 覆盖彼此未修改的凭证。
	_, err := s.db.ExecContext(ctx, `INSERT INTO ticket_notify_preferences(user_id,email_enabled,feishu_enabled,feishu_webhook_url_encrypted,feishu_secret_encrypted,updated_at) VALUES($1,COALESCE($2::bool,true),COALESCE($3::bool,false),COALESCE($4::text,''),COALESCE($5::text,''),NOW()) ON CONFLICT(user_id) DO UPDATE SET email_enabled=COALESCE($2::bool,ticket_notify_preferences.email_enabled),feishu_enabled=COALESCE($3::bool,ticket_notify_preferences.feishu_enabled),feishu_webhook_url_encrypted=COALESCE($4::text,ticket_notify_preferences.feishu_webhook_url_encrypted),feishu_secret_encrypted=COALESCE($5::text,ticket_notify_preferences.feishu_secret_encrypted),updated_at=NOW()`, userID, input.EmailEnabled, input.FeishuEnabled, webhook, secret)
	if err != nil {
		return nil, err
	}
	return s.GetPreferences(ctx, userID)
}

func (s *TicketNotificationService) allowNotification(ctx context.Context, event TicketNotificationEvent, seconds int) (bool, error) {
	if seconds == 0 {
		return true, nil
	}
	if s.nonce == nil {
		return false, errors.New("ticket notification throttle is unavailable")
	}
	direction := "to_user"
	if event.ToAdmin {
		direction = "to_admin"
	}
	return s.nonce.ConsumeNonce(ctx, fmt.Sprintf("ticket_notify:%d:%s", event.TicketID, direction), time.Duration(seconds)*time.Second)
}

func (s *TicketNotificationService) deliver(ctx context.Context, event TicketNotificationEvent) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	settings, err := s.settings.GetTicketSettings(ctx)
	if err != nil {
		return err
	}
	if !settings.Enabled {
		return nil
	}
	err = s.db.QueryRowContext(ctx, `SELECT t.ticket_no,t.subject,t.status,t.user_id,t.assignee_id,COALESCE(a.email,''),COALESCE(NULLIF(a.username,''),a.email,'') FROM tickets t LEFT JOIN users a ON a.id=$2 WHERE t.id=$1`, event.TicketID, event.ActorID).Scan(&event.TicketNo, &event.Subject, &event.Status, &event.ownerID, &event.assigneeID, &event.actorEmail, &event.ActorLabel)
	if err != nil {
		return err
	}
	if !event.ToAdmin && event.ownerID == event.ActorID {
		return nil
	}
	allowed, err := s.allowNotification(ctx, event, settings.NotifyThrottleSeconds)
	if err != nil || !allowed {
		return err
	}
	if event.ToAdmin {
		return s.deliverAdmins(ctx, event, settings)
	}
	r, err := s.recipient(ctx, event.ownerID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	return s.sendRecipient(ctx, r, event, false, map[string]bool{})
}

func (s *TicketNotificationService) deliverAdmins(ctx context.Context, event TicketNotificationEvent, settings *TicketSettings) error {
	seen := map[string]bool{}
	var deliveryErrors []error
	if settings.AdminEmailEnabled {
		emails := append([]string{}, settings.AdminNotifyEmails...)
		if len(emails) == 0 {
			rows, err := s.db.QueryContext(ctx, `SELECT email FROM users WHERE role='admin' AND status='active' AND deleted_at IS NULL`)
			if err != nil {
				return err
			}
			for rows.Next() {
				var email string
				if err := rows.Scan(&email); err != nil {
					_ = rows.Close()
					return err
				}
				emails = append(emails, email)
			}
			if err := rows.Err(); err != nil {
				_ = rows.Close()
				return err
			}
			_ = rows.Close()
		}
		for _, email := range emails {
			if err := s.sendRecipient(ctx, &ticketNotificationRecipient{email: email, emailOn: true}, event, true, seen); err != nil {
				deliveryErrors = append(deliveryErrors, err)
			}
		}
	}
	if settings.AdminFeishuEnabled && settings.AdminFeishuWebhookURLEncrypted != "" {
		if err := s.sendFeishu(ctx, settings.AdminFeishuWebhookURLEncrypted, settings.AdminFeishuSecretEncrypted, event, true); err != nil {
			deliveryErrors = append(deliveryErrors, err)
		}
	}
	if event.assigneeID.Valid && event.assigneeID.Int64 != event.ActorID {
		var eligible bool
		if err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id=$1 AND role='admin' AND status='active' AND deleted_at IS NULL)`, event.assigneeID.Int64).Scan(&eligible); err != nil {
			deliveryErrors = append(deliveryErrors, err)
		} else if eligible {
			r, err := s.recipient(ctx, event.assigneeID.Int64)
			if err != nil {
				deliveryErrors = append(deliveryErrors, err)
			} else if err := s.sendRecipient(ctx, r, event, true, seen); err != nil {
				deliveryErrors = append(deliveryErrors, err)
			}
		}
	}
	return errors.Join(deliveryErrors...)
}

func (s *TicketNotificationService) ticketURL(ctx context.Context, id int64, admin bool) string {
	base := ""
	if s.settings != nil && s.settings.settingRepo != nil {
		for _, key := range []string{SettingKeyFrontendURL, SettingKeyAPIBaseURL} {
			value, err := s.settings.settingRepo.GetValue(ctx, key)
			if err == nil && isAbsoluteTicketURL(strings.TrimSpace(value)) {
				base = strings.TrimRight(strings.TrimSpace(value), "/")
				break
			}
		}
	}
	if base == "" {
		return ""
	}
	path := "/tickets/"
	if admin {
		path = "/admin/tickets/"
	}
	if id == 0 {
		return base + strings.TrimRight(path, "/")
	}
	return base + path + strconv.FormatInt(id, 10)
}

func isAbsoluteTicketURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Hostname() != "" && u.User == nil && u.Opaque == "" && u.RawQuery == "" && u.Fragment == ""
}

func (s *TicketNotificationService) sendRecipient(ctx context.Context, r *ticketNotificationRecipient, event TicketNotificationEvent, admin bool, seen map[string]bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if (r.id != 0 && r.id == event.ActorID) || (r.email != "" && event.actorEmail != "" && strings.EqualFold(r.email, event.actorEmail)) {
		return nil
	}
	var deliveryErrors []error
	if !admin {
		event.ActorLabel = "Support"
	}
	if r.emailOn && r.email != "" && !seen[strings.ToLower(r.email)] {
		seen[strings.ToLower(r.email)] = true
		if s.emails == nil {
			deliveryErrors = append(deliveryErrors, errors.New("ticket notification email is not configured"))
		} else {
			eventName := NotificationEmailEventTicketNewMessage
			if event.Created {
				eventName = NotificationEmailEventTicketCreated
			}
			name := r.name
			if name == "" {
				name = r.email
			}
			if err := s.emails.Send(ctx, NotificationEmailSendInput{Event: eventName, UserID: r.id, RecipientEmail: r.email, RecipientName: name, Variables: map[string]string{"ticket_no": event.TicketNo, "ticket_subject": event.Subject, "ticket_status": event.Status, "message_excerpt": truncateTicketExcerpt(event.Excerpt, 200), "ticket_url": s.ticketURL(ctx, event.TicketID, admin), "actor_label": event.ActorLabel}}); err != nil {
				deliveryErrors = append(deliveryErrors, err)
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return errors.Join(append(deliveryErrors, err)...)
	}
	if r.feishuOn && r.webhook != "" {
		if err := s.sendFeishu(ctx, r.webhook, r.secret, event, admin); err != nil {
			deliveryErrors = append(deliveryErrors, err)
		}
	}
	return errors.Join(deliveryErrors...)
}

type ticketFeishuCard struct {
	MsgType   string         `json:"msg_type"`
	Timestamp string         `json:"timestamp,omitempty"`
	Sign      string         `json:"sign,omitempty"`
	Card      map[string]any `json:"card"`
}

func ticketFeishuSignature(timestamp, secret string) string {
	mac := hmac.New(sha256.New, []byte(timestamp+"\n"+secret))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func (s *TicketNotificationService) sendFeishu(ctx context.Context, encryptedURL, encryptedSecret string, event TicketNotificationEvent, admin bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s.encryptor == nil {
		return errors.New("ticket notification encryption is not configured")
	}
	webhook, err := s.encryptor.Decrypt(encryptedURL)
	if err != nil {
		return errors.New("could not decrypt ticket webhook configuration")
	}
	if err := ValidateTicketWebhookURL(webhook); err != nil {
		return err
	}
	secret := ""
	if encryptedSecret != "" {
		secret, err = s.encryptor.Decrypt(encryptedSecret)
		if err != nil {
			return errors.New("could not decrypt ticket webhook signature")
		}
	}
	card := buildTicketFeishuCard(event, s.ticketURL(ctx, event.TicketID, admin))
	if secret != "" {
		card.Timestamp = strconv.FormatInt(time.Now().Unix(), 10)
		card.Sign = ticketFeishuSignature(card.Timestamp, secret)
	}
	payload, err := json.Marshal(card)
	if err != nil {
		return errors.New("could not encode ticket webhook message")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhook, strings.NewReader(string(payload)))
	if err != nil {
		return errors.New("could not construct ticket webhook request")
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.http.Do(req)
	// net/http 的 url.Error 含完整 webhook token，禁止把它放入日志错误链。
	if err != nil {
		return errors.New("ticket webhook request failed")
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return errors.New("could not read ticket webhook response")
	}
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("ticket webhook HTTP status %d", resp.StatusCode)
	}
	var result struct {
		Code *int `json:"code"`
	}
	if err := json.Unmarshal(data, &result); err != nil || result.Code == nil {
		return errors.New("invalid ticket webhook response")
	}
	if *result.Code != 0 {
		return fmt.Errorf("ticket webhook returned code %d", *result.Code)
	}
	return nil
}

func buildTicketFeishuCard(event TicketNotificationEvent, ticketURL string) ticketFeishuCard {
	title := "工单有新消息 / New ticket message"
	if event.Created {
		title = "收到新工单 / New support ticket"
	}
	elements := []any{
		map[string]any{"tag": "div", "text": map[string]string{"tag": "plain_text", "content": fmt.Sprintf("[%s] %s\n%s", event.TicketNo, event.Subject, truncateTicketExcerpt(event.Excerpt, 200))}},
	}
	if isAbsoluteTicketURL(ticketURL) {
		elements = append(elements, map[string]any{"tag": "action", "actions": []any{map[string]any{"tag": "button", "text": map[string]string{"tag": "plain_text", "content": "查看工单 / View ticket"}, "url": ticketURL, "type": "primary"}}})
	}
	return ticketFeishuCard{MsgType: "interactive", Card: map[string]any{
		"header":   map[string]any{"title": map[string]string{"tag": "plain_text", "content": title}},
		"elements": elements,
	}}
}

func ValidateTicketWebhookURL(raw string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme != "https" || u.User != nil || (u.Host != "open.feishu.cn" && u.Host != "open.larksuite.com") || !strings.HasPrefix(u.Path, "/open-apis/bot/v2/hook/") || strings.TrimPrefix(u.Path, "/open-apis/bot/v2/hook/") == "" || u.RawPath != "" || u.RawQuery != "" || u.Fragment != "" {
		return infraerrors.BadRequest("NOTIFY_WEBHOOK_INVALID", "invalid Feishu webhook URL")
	}
	return nil
}

func MaskTicketWebhookURL(raw string) string {
	if err := ValidateTicketWebhookURL(raw); err != nil {
		return ""
	}
	u, _ := url.Parse(raw)
	token := strings.TrimPrefix(u.Path, "/open-apis/bot/v2/hook/")
	masked := "****"
	if len(token) > 12 {
		masked = token[:4] + "****" + token[len(token)-4:]
	}
	return "https://" + u.Host + "/open-apis/bot/v2/hook/" + masked
}

func truncateTicketExcerpt(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

func (s *TicketNotificationService) allowTest(ctx context.Context, userID int64) error {
	if s.limiter == nil {
		return infraerrors.New(http.StatusBadGateway, "TICKET_NOTIFY_FAILED", "notification rate limiter is unavailable")
	}
	result, err := s.limiter.Allow(ctx, fmt.Sprintf("ticket_notify_test:%d", userID), 3, time.Minute)
	if err != nil {
		return infraerrors.New(http.StatusBadGateway, "TICKET_NOTIFY_FAILED", "notification rate limiter is unavailable")
	}
	if !result.Allowed {
		return infraerrors.TooManyRequests("TICKET_NOTIFY_TEST_RATE_LIMITED", "notification tests are limited to three per minute").WithMetadata(map[string]string{"retry_after": strconv.FormatInt(int64((result.RetryAfter+time.Second-1)/time.Second), 10)})
	}
	return nil
}

func (s *TicketNotificationService) TestUserNotification(ctx context.Context, userID int64, channel string) error {
	if err := s.allowTest(ctx, userID); err != nil {
		return err
	}
	r, err := s.recipient(ctx, userID)
	if err != nil {
		return err
	}
	return s.testRecipient(ctx, r, channel)
}

func (s *TicketNotificationService) TestAdminNotification(ctx context.Context, actorID int64, channel string) error {
	if err := s.allowTest(ctx, actorID); err != nil {
		return err
	}
	settings, err := s.settings.GetTicketSettings(ctx)
	if err != nil {
		return err
	}
	if channel == "feishu" {
		return s.testRecipient(ctx, &ticketNotificationRecipient{webhook: settings.AdminFeishuWebhookURLEncrypted, secret: settings.AdminFeishuSecretEncrypted}, channel)
	}
	if channel != "email" {
		return infraerrors.BadRequest("TICKET_INVALID_INPUT", "invalid notification channel")
	}
	emails := append([]string{}, settings.AdminNotifyEmails...)
	if len(emails) == 0 {
		rows, err := s.db.QueryContext(ctx, `SELECT email FROM users WHERE role='admin' AND status='active' AND deleted_at IS NULL`)
		if err != nil {
			return err
		}
		for rows.Next() {
			var email string
			if err := rows.Scan(&email); err != nil {
				_ = rows.Close()
				return err
			}
			emails = append(emails, email)
		}
		err = rows.Err()
		_ = rows.Close()
		if err != nil {
			return err
		}
	}
	if len(emails) == 0 {
		return infraerrors.New(http.StatusBadGateway, "TICKET_NOTIFY_FAILED", "no administrator email is configured")
	}
	for _, email := range emails {
		if err := s.testRecipient(ctx, &ticketNotificationRecipient{email: email}, channel); err != nil {
			return err
		}
	}
	return nil
}

func (s *TicketNotificationService) testRecipient(ctx context.Context, r *ticketNotificationRecipient, channel string) error {
	event := TicketNotificationEvent{TicketNo: "TEST", Subject: "工单通知测试 / Ticket notification test", Status: "pending_admin", Excerpt: "这是一条测试通知。 / This is a test notification.", ActorLabel: "System"}
	var err error
	switch channel {
	case "email":
		if r.email == "" || s.emails == nil {
			err = errors.New("notification email is not configured")
		} else {
			err = s.emails.Send(ctx, NotificationEmailSendInput{Event: NotificationEmailEventTicketNewMessage, UserID: r.id, RecipientEmail: r.email, RecipientName: r.name, Variables: map[string]string{"ticket_no": event.TicketNo, "ticket_subject": event.Subject, "ticket_status": event.Status, "message_excerpt": event.Excerpt, "ticket_url": s.ticketURL(ctx, 0, false), "actor_label": event.ActorLabel}})
		}
	case "feishu":
		if r.webhook == "" {
			err = errors.New("notification webhook is not configured")
		} else {
			err = s.sendFeishu(ctx, r.webhook, r.secret, event, false)
		}
	default:
		return infraerrors.BadRequest("TICKET_INVALID_INPUT", "invalid notification channel")
	}
	if err != nil {
		return infraerrors.New(http.StatusBadGateway, "TICKET_NOTIFY_FAILED", "could not send test notification").WithCause(err)
	}
	return nil
}
