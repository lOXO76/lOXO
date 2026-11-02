//go:build unit

package service

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestSettingService_CreationEnabledOptOut(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		missing bool
		enabled bool
	}{
		{name: "missing", missing: true, enabled: true},
		{name: "empty", enabled: true},
		{name: "whitespace", value: " \t\n", enabled: true},
		{name: "true", value: "true", enabled: true},
		{name: "one", value: "1", enabled: true},
		{name: "on", value: "on", enabled: true},
		{name: "enabled", value: "enabled", enabled: true},
		{name: "invalid", value: "invalid", enabled: true},
		{name: "false", value: "false"},
		{name: "zero", value: "0"},
		{name: "off", value: "off"},
		{name: "disabled", value: "disabled"},
		{name: "mixed case and whitespace", value: " \tFaLsE\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repo := &panelRateLimitSettingRepo{values: map[string]string{}}
			if !test.missing {
				repo.values[SettingKeyCreationEnabled] = test.value
			}
			svc := NewSettingService(repo, &config.Config{})
			ctx := context.Background()

			require.Equal(t, test.enabled, svc.IsCreationEnabled(ctx))

			settings, err := svc.GetAllSettings(ctx)
			require.NoError(t, err)
			require.Equal(t, test.enabled, settings.CreationEnabled)

			public, err := svc.GetPublicSettings(ctx)
			require.NoError(t, err)
			require.Equal(t, test.enabled, public.CreationEnabled)

			injection, err := svc.GetPublicSettingsForInjection(ctx)
			require.NoError(t, err)
			payload, ok := injection.(*PublicSettingsInjectionPayload)
			require.True(t, ok)
			require.Equal(t, test.enabled, payload.CreationEnabled)
			encoded, err := json.Marshal(payload)
			require.NoError(t, err)
			var fields map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(encoded, &fields))
			require.JSONEq(t, strconv.FormatBool(test.enabled), string(fields[SettingKeyCreationEnabled]))
		})
	}
}

func TestSettingService_IsCreationEnabledDefaultsOnReadFailure(t *testing.T) {
	ctx := context.Background()
	var missingService *SettingService
	require.True(t, missingService.IsCreationEnabled(ctx))
	require.True(t, (&SettingService{}).IsCreationEnabled(ctx))

	svc := NewSettingService(&panelRateLimitSettingRepo{
		values:      map[string]string{SettingKeyCreationEnabled: "false"},
		getValueErr: errors.New("settings unavailable"),
	}, &config.Config{})
	require.True(t, svc.IsCreationEnabled(ctx))
}

func TestSettingService_CreationEnabledDefaultInitialization(t *testing.T) {
	t.Run("new installation", func(t *testing.T) {
		repo := &panelRateLimitSettingRepo{values: map[string]string{}}
		svc := NewSettingService(repo, &config.Config{})
		require.NoError(t, svc.InitializeDefaultSettings(context.Background()))
		require.Equal(t, "true", repo.values[SettingKeyCreationEnabled])
	})

	t.Run("existing disabled setting", func(t *testing.T) {
		repo := &panelRateLimitSettingRepo{values: map[string]string{
			SettingKeyRegistrationEnabled: "true",
			SettingKeyCreationEnabled:     "false",
		}}
		svc := NewSettingService(repo, &config.Config{})
		require.NoError(t, svc.InitializeDefaultSettings(context.Background()))
		require.Equal(t, "false", repo.values[SettingKeyCreationEnabled])
	})

	t.Run("existing installation without creation setting", func(t *testing.T) {
		repo := &panelRateLimitSettingRepo{values: map[string]string{
			SettingKeyRegistrationEnabled: "true",
		}}
		svc := NewSettingService(repo, &config.Config{})
		ctx := context.Background()
		require.NoError(t, svc.InitializeDefaultSettings(ctx))
		require.NotContains(t, repo.values, SettingKeyCreationEnabled)
		require.True(t, svc.IsCreationEnabled(ctx))
	})
}

func TestSettingService_CreationEnabledUpdate(t *testing.T) {
	repo := &panelRateLimitSettingRepo{values: map[string]string{}}
	svc := NewSettingService(repo, &config.Config{})
	updates := 0
	svc.SetOnUpdateCallback(func() { updates++ })
	ctx := context.Background()

	for _, enabled := range []bool{false, true} {
		require.NoError(t, svc.UpdateSettings(ctx, &SystemSettings{CreationEnabled: enabled}))
		require.Equal(t, strconv.FormatBool(enabled), repo.values[SettingKeyCreationEnabled])
		require.Equal(t, enabled, svc.IsCreationEnabled(ctx))
		injection, err := svc.GetPublicSettingsForInjection(ctx)
		require.NoError(t, err)
		require.Equal(t, enabled, injection.(*PublicSettingsInjectionPayload).CreationEnabled)
	}
	require.Equal(t, 2, updates)
}

func TestSettingService_CreationEnabledOmitted(t *testing.T) {
	updateMethods := []struct {
		name   string
		update func(*SettingService, context.Context, *SystemSettings, OmittedSettingKeys) error
	}{
		{name: "system settings", update: (*SettingService).UpdateSettingsOmitting},
		{name: "with auth source defaults", update: func(svc *SettingService, ctx context.Context, settings *SystemSettings, omitted OmittedSettingKeys) error {
			return svc.UpdateSettingsWithAuthSourceDefaultsOmitting(ctx, settings, &AuthSourceDefaultSettings{}, omitted)
		}},
	}
	tests := []struct {
		name    string
		value   string
		missing bool
		enabled bool
	}{
		{name: "missing", missing: true, enabled: true},
		{name: "enabled", value: "true", enabled: true},
		{name: "disabled", value: "false"},
	}
	for _, method := range updateMethods {
		for _, test := range tests {
			t.Run(method.name+"/"+test.name, func(t *testing.T) {
				repo := &panelRateLimitSettingRepo{values: map[string]string{}}
				if !test.missing {
					repo.values[SettingKeyCreationEnabled] = test.value
				}
				svc := NewSettingService(repo, &config.Config{})
				ctx := context.Background()
				require.NoError(t, method.update(svc, ctx, &SystemSettings{CreationEnabled: !test.enabled}, OmittedSettingKeys{
					SettingKeyCreationEnabled: {},
				}))
				if test.missing {
					require.NotContains(t, repo.values, SettingKeyCreationEnabled)
				} else {
					require.Equal(t, test.value, repo.values[SettingKeyCreationEnabled])
				}
				require.Equal(t, test.enabled, svc.IsCreationEnabled(ctx))
				injection, err := svc.GetPublicSettingsForInjection(ctx)
				require.NoError(t, err)
				require.Equal(t, test.enabled, injection.(*PublicSettingsInjectionPayload).CreationEnabled)
			})
		}
	}
}
