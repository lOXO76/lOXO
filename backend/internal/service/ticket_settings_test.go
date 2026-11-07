package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestTicketSettingsDefaultAndJSONContract(t *testing.T) {
	s := NewSettingService(newNotificationEmailMemorySettingRepo(), nil)
	settings, err := s.GetTicketSettings(context.Background())
	require.NoError(t, err)
	require.False(t, settings.Enabled)
	require.True(t, settings.AgentEnabled)
	require.Equal(t, 60, settings.NotifyThrottleSeconds)
	require.Equal(t, 2, settings.AgentDefaultQPS)
	require.Equal(t, 600, settings.AgentIPRequestsPerMinute)
	encoded, err := json.Marshal(settings)
	require.NoError(t, err)
	var fields map[string]any
	require.NoError(t, json.Unmarshal(encoded, &fields))
	require.Len(t, fields, 21)
	require.Equal(t, float64(600), fields["agent_ip_requests_per_minute"])
	require.Equal(t, []any{}, fields["admin_notify_emails"])
	require.NotContains(t, string(encoded), "encrypted")
}

func TestTicketSettingsSecretsKeepClearAndEncrypted(t *testing.T) {
	ctx := context.Background()
	repo := newNotificationEmailMemorySettingRepo()
	s := NewSettingService(repo, nil)
	s.ticketSettingsEncryptor = ticketTestCipher{}
	webhook := "https://open.feishu.cn/open-apis/bot/v2/hook/very-secret-token"
	secret := "signature-secret"
	enabled := true
	settings, err := s.UpdateTicketSettings(ctx, TicketSettingsUpdate{Enabled: &enabled, AdminFeishuWebhookURL: &webhook, AdminFeishuSecret: &secret})
	require.NoError(t, err)
	require.True(t, settings.AdminFeishuWebhookConfigured)
	require.True(t, settings.AdminFeishuSecretConfigured)
	require.Contains(t, settings.AdminFeishuWebhookURLMasked, "****")
	stored, err := repo.GetValue(ctx, settingKeyTicketSettings)
	require.NoError(t, err)
	require.NotContains(t, stored, "very-secret-token")
	require.NotContains(t, stored, secret)
	encoded, err := json.Marshal(settings)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "very-secret-token")
	require.NotContains(t, string(encoded), secret)
	var absent TicketSettingsUpdate
	require.NoError(t, json.Unmarshal([]byte(`{"admin_feishu_webhook_url":null,"admin_feishu_secret":null}`), &absent))
	settings, err = s.UpdateTicketSettings(ctx, absent)
	require.NoError(t, err)
	require.True(t, settings.AdminFeishuWebhookConfigured)
	clear := ""
	settings, err = s.UpdateTicketSettings(ctx, TicketSettingsUpdate{AdminFeishuWebhookURL: &clear, AdminFeishuSecret: &clear})
	require.NoError(t, err)
	require.False(t, settings.AdminFeishuWebhookConfigured)
	require.False(t, settings.AdminFeishuSecretConfigured)
	require.True(t, s.IsTicketSystemEnabled(ctx))
}

func TestTicketSettingsValidateRangesAndEmails(t *testing.T) {
	for _, update := range []string{`{"notify_throttle_seconds":-1}`, `{"notify_throttle_seconds":3601}`, `{"agent_default_qps":0}`, `{"agent_max_qps":1001}`, `{"agent_default_qps":11}`, `{"max_subject_length":0}`, `{"max_body_length":100001}`, `{"max_open_tickets_per_user":0}`, `{"create_per_hour_per_user":0}`, `{"messages_per_minute_per_user":0}`, `{"agent_max_keys_per_user":0}`, `{"agent_failed_auth_per_minute_per_ip":0}`, `{"admin_notify_emails":["Alice <alice@example.com>"]}`, `{"admin_notify_emails":["bad"]}`} {
		t.Run(update, func(t *testing.T) {
			s := NewSettingService(newNotificationEmailMemorySettingRepo(), nil)
			var input TicketSettingsUpdate
			require.NoError(t, json.Unmarshal([]byte(update), &input))
			_, err := s.UpdateTicketSettings(context.Background(), input)
			require.Equal(t, "TICKET_INVALID_INPUT", infraerrors.Reason(err))
		})
	}
	s := NewSettingService(newNotificationEmailMemorySettingRepo(), nil)
	emails := []string{"admin@example.com", "ADMIN@example.com"}
	settings, err := s.UpdateTicketSettings(context.Background(), TicketSettingsUpdate{AdminNotifyEmails: &emails})
	require.NoError(t, err)
	require.Len(t, settings.AdminNotifyEmails, 1)
}

func TestTicketAgentIPRateSettingRoundTripAndBounds(t *testing.T) {
	s := NewSettingService(newNotificationEmailMemorySettingRepo(), nil)
	ctx := context.Background()
	for _, limit := range []int{0, 1, 600, 100000} {
		settings, err := s.UpdateTicketSettings(ctx, TicketSettingsUpdate{AgentIPRequestsPerMinute: &limit})
		require.NoError(t, err)
		require.Equal(t, limit, settings.AgentIPRequestsPerMinute)
		cached, err := s.GetTicketAgentIPRequestsPerMinuteCached(ctx)
		require.NoError(t, err)
		require.Equal(t, limit, cached, "settings writes must immediately refresh the cache")
		settings, err = s.UpdateTicketSettings(ctx, TicketSettingsUpdate{})
		require.NoError(t, err)
		require.Equal(t, limit, settings.AgentIPRequestsPerMinute, "omitted field must retain its value")
	}
	for _, limit := range []int{-1, 100001} {
		_, err := s.UpdateTicketSettings(ctx, TicketSettingsUpdate{AgentIPRequestsPerMinute: &limit})
		require.Equal(t, "TICKET_INVALID_INPUT", infraerrors.Reason(err))
	}
}

func TestTicketAgentIPRateCachedSettings(t *testing.T) {
	for _, raw := range []string{"", `{}`, `{"agent_default_qps":2}`} {
		repo := &panelRateLimitSettingRepo{values: map[string]string{settingKeyTicketSettings: raw}}
		s := NewSettingService(repo, nil)
		var wg sync.WaitGroup
		limits, errs := make([]int, 10), make([]error, 10)
		for i := range 10 {
			wg.Go(func() {
				limits[i], errs[i] = s.GetTicketAgentIPRequestsPerMinuteCached(context.Background())
			})
		}
		wg.Wait()
		for i := range limits {
			require.NoError(t, errs[i])
			require.Equal(t, 600, limits[i])
		}
		require.Equal(t, 1, repo.getValueCalls, "parallel requests must share one cached settings read")
		require.NoError(t, repo.Set(context.Background(), settingKeyTicketSettings, `{"agent_ip_requests_per_minute":5}`))
		s.ticketAgentIPRateCache.Store(&cachedTicketAgentIPRate{limit: 600, expiresAt: time.Now().Add(-time.Second)})
		limit, err := s.GetTicketAgentIPRequestsPerMinuteCached(context.Background())
		require.NoError(t, err)
		require.Equal(t, 5, limit)
		require.Equal(t, 2, repo.getValueCalls)
	}
	for _, raw := range []string{`not json`, `{"agent_ip_requests_per_minute":-1}`, `{"agent_ip_requests_per_minute":100001}`} {
		s := NewSettingService(&panelRateLimitSettingRepo{values: map[string]string{settingKeyTicketSettings: raw}}, nil)
		_, err := s.GetTicketAgentIPRequestsPerMinuteCached(context.Background())
		require.Error(t, err)
	}
	s := NewSettingService(&panelRateLimitSettingRepo{getValueErr: errors.New("database unavailable")}, nil)
	_, err := s.GetTicketAgentIPRequestsPerMinuteCached(context.Background())
	require.ErrorContains(t, err, "database unavailable")
}

func TestTicketEmailOfficialTemplatesAndHTMLEscaping(t *testing.T) {
	s := NewNotificationEmailService(newNotificationEmailMemorySettingRepo(), nil)
	for _, event := range []string{NotificationEmailEventTicketCreated, NotificationEmailEventTicketNewMessage} {
		for _, locale := range []string{"zh", "en"} {
			tmpl, err := s.GetTemplate(context.Background(), event, locale)
			require.NoError(t, err)
			preview, err := s.PreviewTemplate(context.Background(), NotificationEmailPreviewInput{Event: event, Locale: locale, Subject: tmpl.Subject, HTML: tmpl.HTML, Variables: map[string]string{"ticket_subject": "<script>alert(1)</script>", "message_excerpt": "<at user_id=all>ping</at>"}})
			require.NoError(t, err)
			require.NotContains(t, preview.HTML, "<script>")
			require.Contains(t, preview.HTML, "&lt;at user_id=all&gt;")
			require.False(t, strings.Contains(preview.HTML, "<at user_id="))
		}
	}
}

func TestTicketEmailTemplatesOmitUnconfiguredLinks(t *testing.T) {
	emails := NewNotificationEmailService(newNotificationEmailMemorySettingRepo(), nil)
	for _, event := range []string{NotificationEmailEventTicketCreated, NotificationEmailEventTicketNewMessage} {
		for _, locale := range []string{"zh", "en"} {
			for _, ticketURL := range []string{"", "/tickets/1", "javascript:alert(1)"} {
				preview, err := emails.PreviewTemplate(context.Background(), NotificationEmailPreviewInput{Event: event, Locale: locale, Variables: map[string]string{"ticket_url": ticketURL}})
				require.NoError(t, err)
				require.NotContains(t, preview.HTML, "<a ")
				require.NotContains(t, preview.HTML, `href=""`)
			}
			preview, err := emails.PreviewTemplate(context.Background(), NotificationEmailPreviewInput{Event: event, Locale: locale, Variables: map[string]string{"ticket_url": "https://panel.example.com/tickets/1"}})
			require.NoError(t, err)
			require.Contains(t, preview.HTML, `href="https://panel.example.com/tickets/1"`)
		}
	}
}

func TestTicketEmailLinkRemovalOnlyTargetsTicketURLAnchors(t *testing.T) {
	body := `<p>Message</p><a class="button" href="{{ ticket_url }}">View ticket</a><a href='{{ticket_url}}'>Ticket</a><a href="https://help.example.com">Help</a>`
	preview, err := renderNotificationEmail(NotificationEmailEventTicketNewMessage, "Ticket", body, map[string]string{"ticket_url": ""}, nil)
	require.NoError(t, err)
	require.NotContains(t, preview.HTML, "View ticket")
	require.NotContains(t, preview.HTML, ">Ticket</a>")
	require.Contains(t, preview.HTML, `<a href="https://help.example.com">Help</a>`)
	preview, err = renderNotificationEmail(NotificationEmailEventAuthPasswordReset, "Reset", `<a href="{{reset_url}}">Reset</a>`, map[string]string{"reset_url": ""}, nil)
	require.NoError(t, err)
	require.Contains(t, preview.HTML, `<a href="">Reset</a>`, "non-ticket email events retain existing rendering behavior")
}
