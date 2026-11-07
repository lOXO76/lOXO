package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/middleware"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9" //nolint:depguard // 测试使用 miniredis 验证真实 Redis 限流协议。
	"github.com/stretchr/testify/require"
)

type ticketTestNonce struct{ client *redis.Client }

func (n ticketTestNonce) ConsumeNonce(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	return n.client.SetNX(ctx, key, "1", ttl).Result()
}

type ticketTestCipher struct{ fail bool }

func (e ticketTestCipher) Encrypt(value string) (string, error) {
	if e.fail {
		return "", errors.New("encryption failed")
	}
	return "encrypted:" + base64.StdEncoding.EncodeToString([]byte(value)), nil
}
func (e ticketTestCipher) Decrypt(value string) (string, error) {
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(value, "encrypted:"))
	return string(decoded), err
}

type ticketTestRoundTripper func(*http.Request) (*http.Response, error)

func (f ticketTestRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type ticketTestEmailSender func(context.Context, NotificationEmailSendInput) error

func (f ticketTestEmailSender) Send(ctx context.Context, input NotificationEmailSendInput) error {
	return f(ctx, input)
}

func TestTicketWebhookValidationMaskAndSignature(t *testing.T) {
	for _, raw := range []string{"https://open.feishu.cn/open-apis/bot/v2/hook/abcdefghijklmno", "https://open.larksuite.com/open-apis/bot/v2/hook/abcdefghijklmno"} {
		require.NoError(t, ValidateTicketWebhookURL(raw))
		require.NotContains(t, MaskTicketWebhookURL(raw), "abcdefghijklmno")
	}
	for _, raw := range []string{"http://open.feishu.cn/open-apis/bot/v2/hook/token", "https://open.feishu.cn.evil.test/open-apis/bot/v2/hook/token", "https://open.feishu.cn@127.0.0.1/open-apis/bot/v2/hook/token", "https://open.feishu.cn:443/open-apis/bot/v2/hook/token", "https://open.feishu.cn/evil", "https://open.feishu.cn/open-apis/bot/v2/hook/", "https://open.feishu.cn/open-apis/bot/v2/hook/token?redirect=localhost", "https://open.feishu.cn/open-apis/bot/v2/hook/%2Ftoken", "https://OPEN.FEISHU.CN/open-apis/bot/v2/hook/token", "https://127.0.0.1/open-apis/bot/v2/hook/token"} {
		t.Run(raw, func(t *testing.T) {
			require.Equal(t, "NOTIFY_WEBHOOK_INVALID", infraerrors.Reason(ValidateTicketWebhookURL(raw)))
		})
	}
	mac := hmac.New(sha256.New, []byte("1600000000\nsecret"))
	require.Equal(t, base64.StdEncoding.EncodeToString(mac.Sum(nil)), ticketFeishuSignature("1600000000", "secret"))
}

func TestTicketFeishuCardTreatsUntrustedContentAsPlainText(t *testing.T) {
	attack := `<at user_id="all">all</at> **bold** [link](https://evil.test)`
	card := buildTicketFeishuCard(TicketNotificationEvent{TicketNo: "TK1", Subject: attack, Excerpt: strings.Repeat("你", 300)}, "https://example.com/tickets/1")
	encoded, err := json.Marshal(card)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), `lark_md`)
	var decoded struct {
		Card struct {
			Elements []struct {
				Text map[string]string `json:"text"`
			} `json:"elements"`
		} `json:"card"`
	}
	require.NoError(t, json.Unmarshal(encoded, &decoded))
	body := decoded.Card.Elements[0].Text
	require.Equal(t, "plain_text", body["tag"])
	require.Contains(t, body["content"], attack)
	require.Equal(t, 200, len([]rune(truncateTicketExcerpt(strings.Repeat("你", 300), 200))))
}

func TestTicketWebhookSendSanitizesErrorsAndRejectsBadResponses(t *testing.T) {
	cipher := ticketTestCipher{}
	webhook, _ := cipher.Encrypt("https://open.feishu.cn/open-apis/bot/v2/hook/secret-token")
	secret, _ := cipher.Encrypt("sign-secret")
	s := NewTicketNotificationService(nil, nil, nil, nil, nil, cipher)
	defer s.Stop()
	require.Equal(t, 5*time.Second, s.http.Timeout)
	require.ErrorIs(t, s.http.CheckRedirect(nil, nil), http.ErrUseLastResponse)
	s.http.Transport = ticketTestRoundTripper(func(r *http.Request) (*http.Response, error) {
		var body struct {
			MsgType   string `json:"msg_type"`
			Timestamp string `json:"timestamp"`
			Sign      string `json:"sign"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		require.Equal(t, "interactive", body.MsgType)
		require.Equal(t, ticketFeishuSignature(body.Timestamp, "sign-secret"), body.Sign)
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"code":0}`))}, nil
	})
	require.NoError(t, s.sendFeishu(context.Background(), webhook, secret, TicketNotificationEvent{}, false))
	for _, body := range []string{`{"code":1}`, `garbage`, `{}`, strings.Repeat("x", 70000)} {
		s.http.Transport = ticketTestRoundTripper(func(r *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}, nil
		})
		require.Error(t, s.sendFeishu(context.Background(), webhook, "", TicketNotificationEvent{}, false))
	}
	s.http.Transport = ticketTestRoundTripper(func(r *http.Request) (*http.Response, error) {
		return nil, &url.Error{Op: "Post", URL: r.URL.String(), Err: errors.New("network failed")}
	})
	err := s.sendFeishu(context.Background(), webhook, "", TicketNotificationEvent{}, false)
	require.Error(t, err)
	require.NotContains(t, err.Error(), "secret-token")
}

func TestTicketNotificationThrottleAndTestLimit(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	s := &TicketNotificationService{nonce: ticketTestNonce{client}, limiter: middleware.NewRateLimiter(client)}
	ctx := context.Background()
	event := TicketNotificationEvent{TicketID: 9, ToAdmin: true}
	allowed, err := s.allowNotification(ctx, event, 60)
	require.NoError(t, err)
	require.True(t, allowed)
	allowed, err = s.allowNotification(ctx, event, 60)
	require.NoError(t, err)
	require.False(t, allowed)
	event.ToAdmin = false
	allowed, err = s.allowNotification(ctx, event, 60)
	require.NoError(t, err)
	require.True(t, allowed)
	server.FastForward(time.Minute)
	allowed, err = s.allowNotification(ctx, event, 60)
	require.NoError(t, err)
	require.True(t, allowed)
	for i := 0; i < 3; i++ {
		require.NoError(t, s.allowTest(ctx, 42))
	}
	require.Equal(t, "TICKET_NOTIFY_TEST_RATE_LIMITED", infraerrors.Reason(s.allowTest(ctx, 42)))
	require.NoError(t, s.allowTest(ctx, 43))
	server.Close()
	require.Equal(t, "TICKET_NOTIFY_FAILED", infraerrors.Reason(s.allowTest(ctx, 42)))
}

func TestTicketNotificationDoesNotNotifyActor(t *testing.T) {
	s := &TicketNotificationService{}
	for _, r := range []*ticketNotificationRecipient{{id: 7, emailOn: true, email: "admin@example.com", feishuOn: true, webhook: "should-not-decrypt"}, {id: 9, emailOn: true, email: "ADMIN@example.com"}} {
		require.NoError(t, s.sendRecipient(context.Background(), r, TicketNotificationEvent{ActorID: 7, actorEmail: "admin@example.com"}, true, map[string]bool{}))
	}
}

func TestTicketNotificationQueueDrainsAndStops(t *testing.T) {
	s := NewTicketNotificationService(nil, nil, nil, nil, nil, ticketTestCipher{})
	require.Equal(t, 128, cap(s.queue))
	for id := int64(1); id <= 30; id++ {
		s.Enqueue(TicketNotificationEvent{TicketID: id})
	}
	s.Stop()
	s.Stop()
	require.Empty(t, s.queue)
	s.Enqueue(TicketNotificationEvent{TicketID: 100})
	require.Empty(t, s.queue)
}

func TestTicketNotificationStopDeadlineWithBlockedSender(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	repo := newNotificationEmailMemorySettingRepo()
	enabled, throttle := true, 0
	settings := NewSettingService(repo, nil)
	_, err = settings.UpdateTicketSettings(context.Background(), TicketSettingsUpdate{Enabled: &enabled, NotifyThrottleSeconds: &throttle})
	require.NoError(t, err)
	mock.MatchExpectationsInOrder(false)
	for id := int64(1); id <= 2; id++ {
		mock.ExpectQuery("SELECT t.ticket_no").WithArgs(id, int64(8)).WillReturnRows(sqlmock.NewRows([]string{"ticket_no", "subject", "status", "user_id", "assignee_id", "email", "username"}).AddRow("TK1", "subject", "pending_user", 7, nil, "admin@example.com", "admin"))
		mock.ExpectQuery("SELECT u.email").WithArgs(int64(7)).WillReturnRows(sqlmock.NewRows([]string{"email", "username", "email_enabled", "feishu_enabled", "webhook", "secret"}).AddRow("user@example.com", "user", true, false, "", ""))
	}
	s := NewTicketNotificationService(db, settings, nil, nil, nil, ticketTestCipher{})
	s.shutdownTimeout = 25 * time.Millisecond
	started, release := make(chan struct{}, 2), make(chan struct{})
	var calls atomic.Int64
	s.emails = ticketTestEmailSender(func(_ context.Context, _ NotificationEmailSendInput) error {
		calls.Add(1)
		started <- struct{}{}
		<-release // 模拟无视 ctx 的 SMTP 实现。
		return nil
	})
	var releaseOnce sync.Once
	t.Cleanup(func() {
		releaseOnce.Do(func() { close(release) })
		s.Stop()
		select {
		case <-s.done:
		case <-time.After(time.Second):
			t.Error("notification workers did not exit after the blocked sender was released")
		}
		_ = db.Close()
	})
	for id := int64(1); id <= 2; id++ {
		s.Enqueue(TicketNotificationEvent{TicketID: id, ActorID: 8})
	}
	for i := 0; i < 2; i++ {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("notification sender did not start")
		}
	}
	s.Enqueue(TicketNotificationEvent{TicketID: 3, ActorID: 8})
	before := time.Now()
	s.Stop()
	require.Less(t, time.Since(before), 500*time.Millisecond)
	require.ErrorIs(t, s.workerCtx.Err(), context.Canceled)
	require.Len(t, s.queue, 1)
	before = time.Now()
	s.Stop()
	require.Less(t, time.Since(before), 100*time.Millisecond)
	releaseOnce.Do(func() { close(release) })
	select {
	case <-s.done:
	case <-time.After(time.Second):
		t.Fatal("notification workers did not finish after release")
	}
	require.Equal(t, int64(2), calls.Load(), "queued messages must not be delivered after shutdown expires")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTicketNotificationURLPrefersFrontendAndRequiresAbsoluteURL(t *testing.T) {
	for _, test := range []struct {
		name, frontend, api, want string
	}{
		{"frontend preferred", "https://panel.example.com/", "https://api.example.com", "https://panel.example.com"},
		{"HTTP frontend", "http://panel.example.com/app/", "https://api.example.com", "http://panel.example.com/app"},
		{"API fallback", "", "https://api.example.com/", "https://api.example.com"},
		{"invalid frontend fallback", "/panel", "https://api.example.com", "https://api.example.com"},
		{"invalid scheme fallback", "javascript:alert(1)", "https://api.example.com", "https://api.example.com"},
		{"empty", "", "", ""},
		{"relative", "/panel", "/api", ""},
		{"protocol relative", "//panel.example.com", "//api.example.com", ""},
		{"credentials", "https://user:pass@panel.example.com", "", ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			repo := newNotificationEmailMemorySettingRepo()
			require.NoError(t, repo.SetMultiple(context.Background(), map[string]string{SettingKeyFrontendURL: test.frontend, SettingKeyAPIBaseURL: test.api}))
			s := &TicketNotificationService{settings: NewSettingService(repo, nil)}
			if test.want == "" {
				require.Empty(t, s.ticketURL(context.Background(), 9, false))
				require.Empty(t, s.ticketURL(context.Background(), 9, true))
				require.Empty(t, s.ticketURL(context.Background(), 0, false))
			} else {
				require.Equal(t, test.want+"/tickets/9", s.ticketURL(context.Background(), 9, false))
				require.Equal(t, test.want+"/admin/tickets/9", s.ticketURL(context.Background(), 9, true))
				require.Equal(t, test.want+"/tickets", s.ticketURL(context.Background(), 0, false))
			}
		})
	}
}

func TestTicketFeishuCardOmitsActionWithoutAbsoluteURL(t *testing.T) {
	for _, ticketURL := range []string{"", "/tickets/9", "//example.com/tickets/9", "javascript:alert(1)"} {
		encoded, err := json.Marshal(buildTicketFeishuCard(TicketNotificationEvent{Subject: "subject", Excerpt: "message"}, ticketURL))
		require.NoError(t, err)
		require.NotContains(t, string(encoded), `"tag":"action"`)
		require.NotContains(t, string(encoded), `"tag":"button"`)
		require.NotContains(t, string(encoded), `"url":`)
		require.Contains(t, string(encoded), "message")
	}
	encoded, err := json.Marshal(buildTicketFeishuCard(TicketNotificationEvent{}, "https://panel.example.com/tickets/9"))
	require.NoError(t, err)
	require.Contains(t, string(encoded), `"tag":"action"`)
	require.Contains(t, string(encoded), `"url":"https://panel.example.com/tickets/9"`)
}

func TestTicketNotificationEmailWithoutURLPreservesEmptyValue(t *testing.T) {
	repo := newNotificationEmailMemorySettingRepo()
	s := &TicketNotificationService{settings: NewSettingService(repo, nil)}
	var captured NotificationEmailSendInput
	s.emails = ticketTestEmailSender(func(_ context.Context, input NotificationEmailSendInput) error { captured = input; return nil })
	require.NoError(t, s.sendRecipient(context.Background(), &ticketNotificationRecipient{id: 7, email: "user@example.com", emailOn: true}, TicketNotificationEvent{TicketID: 9, ActorID: 8, TicketNo: "TK9"}, false, map[string]bool{}))
	value, present := captured.Variables["ticket_url"]
	require.True(t, present)
	require.Empty(t, value)
	emails := NewNotificationEmailService(repo, nil)
	variables := emails.runtimeVariables(context.Background(), captured.Event, "en", captured)
	require.Empty(t, variables["ticket_url"], "preview defaults must not fill a missing live ticket URL")
	tmpl, err := emails.GetTemplate(context.Background(), captured.Event, "en")
	require.NoError(t, err)
	rendered, err := renderNotificationEmail(captured.Event, tmpl.Subject, tmpl.HTML, variables, nil)
	require.NoError(t, err)
	require.NotContains(t, rendered.HTML, "<a ")
	require.NotContains(t, rendered.HTML, `href=""`)
}

func TestTicketNotifyPreferencesDefaultsAndEncryption(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	s := &TicketNotificationService{db: db, encryptor: ticketTestCipher{}}
	mock.ExpectQuery("SELECT u.email").WithArgs(int64(4)).WillReturnRows(sqlmock.NewRows([]string{"email", "username", "email_enabled", "feishu_enabled", "webhook", "secret"}).AddRow("user@example.com", "user", true, false, "", ""))
	preference, err := s.GetPreferences(context.Background(), 4)
	require.NoError(t, err)
	require.Equal(t, "user@example.com", preference.Email)
	require.True(t, preference.EmailEnabled)
	require.False(t, preference.FeishuWebhookConfigured)
	s.encryptor = ticketTestCipher{fail: true}
	webhook := "https://open.feishu.cn/open-apis/bot/v2/hook/secret-token"
	_, err = s.UpdatePreferences(context.Background(), 4, TicketNotifyPreferenceUpdate{FeishuWebhookURL: &webhook})
	require.Error(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTicketNotificationUserOwnerSelfSuppression(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	repo := newNotificationEmailMemorySettingRepo()
	require.NoError(t, repo.Set(context.Background(), SettingKeyTicketSystemEnabled, "true"))
	s := &TicketNotificationService{db: db, settings: NewSettingService(repo, nil)}
	mock.ExpectQuery("SELECT t.ticket_no").WithArgs(int64(1), int64(7)).WillReturnRows(sqlmock.NewRows([]string{"ticket_no", "subject", "status", "user_id", "assignee_id", "email", "username"}).AddRow("TK1", "subject", "pending_user", 7, nil, "owner@example.com", "owner"))
	require.NoError(t, s.deliver(context.Background(), TicketNotificationEvent{TicketID: 1, ActorID: 7}))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTicketNotificationUserReceivesDefaultPreference(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	repo := newNotificationEmailMemorySettingRepo()
	enabled := true
	zero := 0
	settings := NewSettingService(repo, nil)
	_, err = settings.UpdateTicketSettings(context.Background(), TicketSettingsUpdate{Enabled: &enabled, NotifyThrottleSeconds: &zero})
	require.NoError(t, err)
	smtp := startNotificationEmailTestSMTPServer(t)
	require.NoError(t, repo.SetMultiple(context.Background(), smtp.settings()))
	s := &TicketNotificationService{db: db, settings: settings, emails: NewNotificationEmailService(repo, NewEmailService(repo, nil))}
	mock.ExpectQuery("SELECT t.ticket_no").WithArgs(int64(1), int64(8)).WillReturnRows(sqlmock.NewRows([]string{"ticket_no", "subject", "status", "user_id", "assignee_id", "email", "username"}).AddRow("TK1", "subject", "pending_user", 7, nil, "admin@example.com", "private-agent-name"))
	mock.ExpectQuery("SELECT u.email").WithArgs(int64(7)).WillReturnRows(sqlmock.NewRows([]string{"email", "username", "email_enabled", "feishu_enabled", "webhook", "secret"}).AddRow("user@example.com", "user", true, false, "", ""))
	require.NoError(t, s.deliver(context.Background(), TicketNotificationEvent{TicketID: 1, ActorID: 8, Excerpt: "message"}))
	require.Equal(t, int64(1), smtp.messageCount())
	require.NoError(t, mock.ExpectationsWereMet())
}
