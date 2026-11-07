package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/mail"
	"strconv"
	"strings"
	"sync"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const SettingKeyTicketSystemEnabled = "ticket_system_enabled"
const settingKeyTicketSettings = "ticket_settings"

var ticketSettingsUpdateMu sync.Mutex

type TicketSettings struct {
	Enabled                        bool     `json:"enabled"`
	AdminEmailEnabled              bool     `json:"admin_email_enabled"`
	AdminNotifyEmails              []string `json:"admin_notify_emails"`
	AdminFeishuEnabled             bool     `json:"admin_feishu_enabled"`
	AdminFeishuWebhookConfigured   bool     `json:"admin_feishu_webhook_configured"`
	AdminFeishuWebhookURLMasked    string   `json:"admin_feishu_webhook_url_masked"`
	AdminFeishuSecretConfigured    bool     `json:"admin_feishu_secret_configured"`
	NotifyThrottleSeconds          int      `json:"notify_throttle_seconds"`
	MaxOpenTicketsPerUser          int      `json:"max_open_tickets_per_user"`
	CreatePerHourPerUser           int      `json:"create_per_hour_per_user"`
	MessagesPerMinutePerUser       int      `json:"messages_per_minute_per_user"`
	MaxSubjectLength               int      `json:"max_subject_length"`
	MaxBodyLength                  int      `json:"max_body_length"`
	AgentEnabled                   bool     `json:"agent_enabled"`
	AgentDefaultQPS                int      `json:"agent_default_qps"`
	AgentMaxQPS                    int      `json:"agent_max_qps"`
	AgentUserTotalQPS              int      `json:"agent_user_total_qps"`
	AgentAdminTotalQPS             int      `json:"agent_admin_total_qps"`
	AgentMaxKeysPerUser            int      `json:"agent_max_keys_per_user"`
	AgentFailedAuthPerMinutePerIP  int      `json:"agent_failed_auth_per_minute_per_ip"`
	AgentIPRequestsPerMinute       int      `json:"agent_ip_requests_per_minute"`
	AdminFeishuWebhookURLEncrypted string   `json:"-"`
	AdminFeishuSecretEncrypted     string   `json:"-"`
}

type TicketSettingsUpdate struct {
	Enabled                       *bool     `json:"enabled"`
	AdminEmailEnabled             *bool     `json:"admin_email_enabled"`
	AdminNotifyEmails             *[]string `json:"admin_notify_emails"`
	AdminFeishuEnabled            *bool     `json:"admin_feishu_enabled"`
	AdminFeishuWebhookURL         *string   `json:"admin_feishu_webhook_url"`
	AdminFeishuSecret             *string   `json:"admin_feishu_secret"`
	NotifyThrottleSeconds         *int      `json:"notify_throttle_seconds"`
	MaxOpenTicketsPerUser         *int      `json:"max_open_tickets_per_user"`
	CreatePerHourPerUser          *int      `json:"create_per_hour_per_user"`
	MessagesPerMinutePerUser      *int      `json:"messages_per_minute_per_user"`
	MaxSubjectLength              *int      `json:"max_subject_length"`
	MaxBodyLength                 *int      `json:"max_body_length"`
	AgentEnabled                  *bool     `json:"agent_enabled"`
	AgentDefaultQPS               *int      `json:"agent_default_qps"`
	AgentMaxQPS                   *int      `json:"agent_max_qps"`
	AgentUserTotalQPS             *int      `json:"agent_user_total_qps"`
	AgentAdminTotalQPS            *int      `json:"agent_admin_total_qps"`
	AgentMaxKeysPerUser           *int      `json:"agent_max_keys_per_user"`
	AgentFailedAuthPerMinutePerIP *int      `json:"agent_failed_auth_per_minute_per_ip"`
	AgentIPRequestsPerMinute      *int      `json:"agent_ip_requests_per_minute"`
}

type ticketStoredSettings struct {
	TicketSettings
	WebhookEncrypted string `json:"webhook_encrypted"`
	SecretEncrypted  string `json:"secret_encrypted"`
}

func DefaultTicketSettings() TicketSettings {
	return TicketSettings{
		AdminEmailEnabled: true, AdminNotifyEmails: []string{}, NotifyThrottleSeconds: 60,
		MaxOpenTicketsPerUser: 5, CreatePerHourPerUser: 10, MessagesPerMinutePerUser: 20,
		MaxSubjectLength: 200, MaxBodyLength: 5000, AgentEnabled: true, AgentDefaultQPS: 2,
		AgentMaxQPS: 10, AgentUserTotalQPS: 5, AgentAdminTotalQPS: 20, AgentMaxKeysPerUser: 5,
		AgentFailedAuthPerMinutePerIP: 20, AgentIPRequestsPerMinute: 600,
	}
}

type cachedTicketAgentIPRate struct {
	limit     int
	expiresAt time.Time
}

// GetTicketAgentIPRequestsPerMinuteCached 避免鉴权前被限流的请求反复查询设置表。
// 缓存一分钟；本实例保存工单设置后立即更新，其他实例最多一分钟同步。
func (s *SettingService) GetTicketAgentIPRequestsPerMinuteCached(ctx context.Context) (int, error) {
	if s == nil || s.settingRepo == nil {
		return DefaultTicketSettings().AgentIPRequestsPerMinute, nil
	}
	load := func() *cachedTicketAgentIPRate {
		cached := s.ticketAgentIPRateCache.Load()
		if cached != nil && time.Now().Before(cached.expiresAt) {
			return cached
		}
		return nil
	}
	if cached := load(); cached != nil {
		return cached.limit, nil
	}
	result, err, _ := s.ticketAgentIPRateSF.Do("ticket_agent_ip_rate", func() (any, error) {
		ticketSettingsUpdateMu.Lock()
		defer ticketSettingsUpdateMu.Unlock()
		if cached := load(); cached != nil {
			return cached.limit, nil
		}
		dbCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		raw, err := s.settingRepo.GetValue(dbCtx, settingKeyTicketSettings)
		if err != nil && !errors.Is(err, ErrSettingNotFound) {
			return 0, fmt.Errorf("read ticket agent IP rate setting: %w", err)
		}
		settings := struct {
			Limit int `json:"agent_ip_requests_per_minute"`
		}{Limit: DefaultTicketSettings().AgentIPRequestsPerMinute}
		if strings.TrimSpace(raw) != "" {
			if err := json.Unmarshal([]byte(raw), &settings); err != nil {
				return 0, fmt.Errorf("decode ticket agent IP rate setting: %w", err)
			}
		}
		if settings.Limit < 0 || settings.Limit > 100000 {
			return 0, errors.New("ticket agent IP rate setting is outside its allowed range")
		}
		s.ticketAgentIPRateCache.Store(&cachedTicketAgentIPRate{limit: settings.Limit, expiresAt: time.Now().Add(time.Minute)})
		return settings.Limit, nil
	})
	if err != nil {
		return 0, err
	}
	limit, ok := result.(int)
	if !ok {
		return 0, errors.New("unexpected ticket agent IP rate setting")
	}
	return limit, nil
}

func (s *SettingService) GetTicketSettings(ctx context.Context) (*TicketSettings, error) {
	stored := ticketStoredSettings{TicketSettings: DefaultTicketSettings()}
	if s == nil || s.settingRepo == nil {
		return &stored.TicketSettings, nil
	}
	values, err := s.settingRepo.GetMultiple(ctx, []string{settingKeyTicketSettings, SettingKeyTicketSystemEnabled})
	if err != nil && !errors.Is(err, ErrSettingNotFound) {
		return nil, err
	}
	if raw := values[settingKeyTicketSettings]; raw != "" {
		if err := json.Unmarshal([]byte(raw), &stored); err != nil {
			return nil, fmt.Errorf("decode ticket settings: %w", err)
		}
	}
	if value, ok := values[SettingKeyTicketSystemEnabled]; ok {
		stored.Enabled = value == "true"
	}
	if stored.AdminNotifyEmails == nil {
		stored.AdminNotifyEmails = []string{}
	}
	stored.AdminFeishuWebhookURLEncrypted = stored.WebhookEncrypted
	stored.AdminFeishuSecretEncrypted = stored.SecretEncrypted
	stored.AdminFeishuWebhookConfigured = stored.WebhookEncrypted != ""
	stored.AdminFeishuSecretConfigured = stored.SecretEncrypted != ""
	stored.AdminFeishuWebhookURLMasked = ""
	if stored.WebhookEncrypted != "" && s.ticketSettingsEncryptor != nil {
		plaintext, err := s.ticketSettingsEncryptor.Decrypt(stored.WebhookEncrypted)
		if err != nil {
			return nil, fmt.Errorf("decrypt ticket notification configuration: %w", err)
		}
		stored.AdminFeishuWebhookURLMasked = MaskTicketWebhookURL(plaintext)
	}
	return &stored.TicketSettings, nil
}

func (s *SettingService) IsTicketSystemEnabled(ctx context.Context) bool {
	settings, err := s.GetTicketSettings(ctx)
	return err == nil && settings.Enabled
}

func (s *SettingService) UpdateTicketSettings(ctx context.Context, input TicketSettingsUpdate) (*TicketSettings, error) {
	ticketSettingsUpdateMu.Lock()
	defer ticketSettingsUpdateMu.Unlock()
	settings, err := s.GetTicketSettings(ctx)
	if err != nil {
		return nil, err
	}
	for _, field := range []struct {
		input  *bool
		target *bool
	}{
		{input.Enabled, &settings.Enabled}, {input.AdminEmailEnabled, &settings.AdminEmailEnabled},
		{input.AdminFeishuEnabled, &settings.AdminFeishuEnabled}, {input.AgentEnabled, &settings.AgentEnabled},
	} {
		if field.input != nil {
			*field.target = *field.input
		}
	}
	for _, field := range []struct {
		input  *int
		target *int
	}{
		{input.NotifyThrottleSeconds, &settings.NotifyThrottleSeconds}, {input.MaxOpenTicketsPerUser, &settings.MaxOpenTicketsPerUser},
		{input.CreatePerHourPerUser, &settings.CreatePerHourPerUser}, {input.MessagesPerMinutePerUser, &settings.MessagesPerMinutePerUser},
		{input.MaxSubjectLength, &settings.MaxSubjectLength}, {input.MaxBodyLength, &settings.MaxBodyLength},
		{input.AgentDefaultQPS, &settings.AgentDefaultQPS}, {input.AgentMaxQPS, &settings.AgentMaxQPS},
		{input.AgentUserTotalQPS, &settings.AgentUserTotalQPS}, {input.AgentAdminTotalQPS, &settings.AgentAdminTotalQPS},
		{input.AgentMaxKeysPerUser, &settings.AgentMaxKeysPerUser}, {input.AgentFailedAuthPerMinutePerIP, &settings.AgentFailedAuthPerMinutePerIP},
		{input.AgentIPRequestsPerMinute, &settings.AgentIPRequestsPerMinute},
	} {
		if field.input != nil {
			*field.target = *field.input
		}
	}
	if input.AdminNotifyEmails != nil {
		settings.AdminNotifyEmails = make([]string, 0, len(*input.AdminNotifyEmails))
		seen := map[string]bool{}
		for _, value := range *input.AdminNotifyEmails {
			value = strings.TrimSpace(value)
			address, err := mail.ParseAddress(value)
			if err != nil || address.Address != value || len(value) > 254 {
				return nil, infraerrors.BadRequest("TICKET_INVALID_INPUT", "invalid administrator notification email")
			}
			if !seen[strings.ToLower(value)] {
				settings.AdminNotifyEmails = append(settings.AdminNotifyEmails, value)
				seen[strings.ToLower(value)] = true
			}
		}
	}
	if err := validateTicketSettings(settings); err != nil {
		return nil, err
	}
	if err := updateTicketEncryptedValue(s.ticketSettingsEncryptor, input.AdminFeishuWebhookURL, &settings.AdminFeishuWebhookURLEncrypted, true); err != nil {
		return nil, err
	}
	if err := updateTicketEncryptedValue(s.ticketSettingsEncryptor, input.AdminFeishuSecret, &settings.AdminFeishuSecretEncrypted, false); err != nil {
		return nil, err
	}
	stored := ticketStoredSettings{TicketSettings: *settings, WebhookEncrypted: settings.AdminFeishuWebhookURLEncrypted, SecretEncrypted: settings.AdminFeishuSecretEncrypted}
	data, err := json.Marshal(stored)
	if err != nil {
		return nil, err
	}
	if err := s.settingRepo.SetMultiple(ctx, map[string]string{settingKeyTicketSettings: string(data), SettingKeyTicketSystemEnabled: strconv.FormatBool(settings.Enabled)}); err != nil {
		return nil, err
	}
	s.ticketAgentIPRateCache.Store(&cachedTicketAgentIPRate{limit: settings.AgentIPRequestsPerMinute, expiresAt: time.Now().Add(time.Minute)})
	if s.onUpdate != nil {
		s.onUpdate()
	}
	return s.GetTicketSettings(ctx)
}

func validateTicketSettings(settings *TicketSettings) error {
	for _, field := range []struct{ value, min, max int }{
		{settings.NotifyThrottleSeconds, 0, 3600}, {settings.MaxOpenTicketsPerUser, 1, 1000},
		{settings.CreatePerHourPerUser, 1, 10000}, {settings.MessagesPerMinutePerUser, 1, 10000},
		{settings.MaxSubjectLength, 1, 1000}, {settings.MaxBodyLength, 1, 100000},
		{settings.AgentDefaultQPS, 1, 1000}, {settings.AgentMaxQPS, 1, 1000},
		{settings.AgentUserTotalQPS, 1, 1000}, {settings.AgentAdminTotalQPS, 1, 1000},
		{settings.AgentMaxKeysPerUser, 1, 1000}, {settings.AgentFailedAuthPerMinutePerIP, 1, 10000},
		{settings.AgentIPRequestsPerMinute, 0, 100000},
	} {
		if field.value < field.min || field.value > field.max {
			return infraerrors.BadRequest("TICKET_INVALID_INPUT", "ticket setting is outside its allowed range")
		}
	}
	if settings.AgentDefaultQPS > settings.AgentMaxQPS || len(settings.AdminNotifyEmails) > 100 {
		return infraerrors.BadRequest("TICKET_INVALID_INPUT", "invalid ticket settings")
	}
	return nil
}

func updateTicketEncryptedValue(encryptor SecretEncryptor, input *string, target *string, webhook bool) error {
	if input == nil {
		return nil
	}
	value := strings.TrimSpace(*input)
	if value == "" {
		*target = ""
		return nil
	}
	if webhook {
		if err := ValidateTicketWebhookURL(value); err != nil {
			return err
		}
	}
	if len(value) > 4096 {
		return infraerrors.BadRequest("TICKET_INVALID_INPUT", "notification credential is too long")
	}
	if encryptor == nil {
		return errors.New("ticket notification encryption is not configured")
	}
	ciphertext, err := encryptor.Encrypt(value)
	if err != nil {
		return fmt.Errorf("encrypt ticket notification configuration: %w", err)
	}
	*target = ciphertext
	return nil
}
