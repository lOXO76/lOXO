//go:build unit

package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestSettingServiceHeaderLinksDefaults(t *testing.T) {
	ctx := context.Background()
	for _, stored := range []map[string]string{
		{},
		{SettingKeyHeaderLinks: ""},
		{SettingKeyRegistrationEnabled: "true"},
	} {
		repo := &panelRateLimitSettingRepo{values: stored}
		svc := NewSettingService(repo, &config.Config{})
		system, err := svc.GetAllSettings(ctx)
		require.NoError(t, err)
		require.Equal(t, "[]", system.HeaderLinks)
		public, err := svc.GetPublicSettings(ctx)
		require.NoError(t, err)
		require.Equal(t, "[]", public.HeaderLinks)
		injection, err := svc.GetPublicSettingsForInjection(ctx)
		require.NoError(t, err)
		require.JSONEq(t, "[]", string(injection.(*PublicSettingsInjectionPayload).HeaderLinks))
	}
	for _, test := range []struct {
		name   string
		values map[string]string
		want   string
	}{
		{name: "new installation", values: map[string]string{}, want: "[]"},
		{name: "keep existing", values: map[string]string{SettingKeyRegistrationEnabled: "true", SettingKeyHeaderLinks: `[{"id":"help"}]`}, want: `[{"id":"help"}]`},
	} {
		t.Run(test.name, func(t *testing.T) {
			repo := &panelRateLimitSettingRepo{values: test.values}
			svc := NewSettingService(repo, &config.Config{})
			require.NoError(t, svc.InitializeDefaultSettings(ctx))
			require.Equal(t, test.want, repo.values[SettingKeyHeaderLinks])
		})
	}
}

func TestSettingServiceHeaderLinksPersistAndInject(t *testing.T) {
	ctx := context.Background()
	raw := `[{"id":"public","label":"Help","url":"/help","icon_svg":"<svg/>","visibility":"user","open_in_new_tab":true,"sort_order":2},{"id":"private","label":"Admin secret","url":"https://private.example","visibility":"admin","open_in_new_tab":true,"sort_order":1}]`
	publicOnly := `[{"id":"public","label":"Help","url":"/help","icon_svg":"<svg/>","visibility":"user","open_in_new_tab":true,"sort_order":2}]`
	menu := `[{"id":"menu","url":"https://menu.example","visibility":"user","open_in_new_tab":true},{"id":"admin-menu","url":"https://admin.example","visibility":"admin","open_in_new_tab":true}]`
	repo := &panelRateLimitSettingRepo{values: map[string]string{}}
	svc := NewSettingService(repo, &config.Config{})
	updates := 0
	svc.SetOnUpdateCallback(func() { updates++ })
	require.NoError(t, svc.UpdateSettings(ctx, &SystemSettings{HeaderLinks: raw, CustomMenuItems: menu}))
	require.Equal(t, 1, updates)
	require.Equal(t, raw, repo.values[SettingKeyHeaderLinks])
	system, err := svc.GetAllSettings(ctx)
	require.NoError(t, err)
	require.Equal(t, raw, system.HeaderLinks)
	public, err := svc.GetPublicSettings(ctx)
	require.NoError(t, err)
	require.Equal(t, raw, public.HeaderLinks)
	injection, err := svc.GetPublicSettingsForInjection(ctx)
	require.NoError(t, err)
	payload := injection.(*PublicSettingsInjectionPayload)
	require.JSONEq(t, publicOnly, string(payload.HeaderLinks))
	require.JSONEq(t, `[{"id":"menu","url":"https://menu.example","visibility":"user","open_in_new_tab":true}]`, string(payload.CustomMenuItems))
	encoded, err := json.Marshal(payload)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "private.example")
	require.NotContains(t, string(encoded), "Admin secret")
	var fields map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(encoded, &fields))
	require.JSONEq(t, publicOnly, string(fields[SettingKeyHeaderLinks]))
	require.NoError(t, svc.UpdateSettingsOmitting(ctx, &SystemSettings{}, OmittedSettingKeys{SettingKeyHeaderLinks: {}}))
	require.Equal(t, raw, repo.values[SettingKeyHeaderLinks])
	require.NoError(t, svc.UpdateSettings(ctx, &SystemSettings{HeaderLinks: "[]"}))
	require.Equal(t, "[]", repo.values[SettingKeyHeaderLinks])
}

func TestSettingServiceHeaderLinksInjectionEmptyFallback(t *testing.T) {
	for _, raw := range []string{"", "[]", "null", "invalid", "{}", `[1]`, `[{"visibility":"admin"}]`} {
		t.Run(raw, func(t *testing.T) {
			repo := &panelRateLimitSettingRepo{values: map[string]string{SettingKeyHeaderLinks: raw}}
			svc := NewSettingService(repo, &config.Config{})
			injection, err := svc.GetPublicSettingsForInjection(context.Background())
			require.NoError(t, err)
			require.JSONEq(t, "[]", string(injection.(*PublicSettingsInjectionPayload).HeaderLinks))
		})
	}
}

func TestSettingServiceHeaderLinksDoNotExpandFrameSrc(t *testing.T) {
	repo := &panelRateLimitSettingRepo{values: map[string]string{
		SettingKeyHomeContent:                 "https://home.example/page",
		SettingKeyPurchaseSubscriptionEnabled: "true",
		SettingKeyPurchaseSubscriptionURL:     "https://purchase.example/page",
		SettingKeyCustomMenuItems:             `[{"url":"https://menu.example/page","visibility":"user"}]`,
		SettingKeyHeaderLinks:                 `[{"url":"https://header.example/help","visibility":"user","open_in_new_tab":true},{"url":"https://private-header.example","visibility":"admin","open_in_new_tab":false}]`,
	}}
	svc := NewSettingService(repo, &config.Config{})
	origins, err := svc.GetFrameSrcOrigins(context.Background())
	require.NoError(t, err)
	require.Equal(t, []string{"https://home.example", "https://purchase.example", "https://menu.example"}, origins)
}
