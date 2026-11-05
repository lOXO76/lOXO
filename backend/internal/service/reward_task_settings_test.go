package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestRewardTasksSettingsOptIn(t *testing.T) {
	for _, value := range []string{"", "false", "invalid", "TRUE", "true"} {
		t.Run(value, func(t *testing.T) {
			svc := NewSettingService(&panelRateLimitSettingRepo{values: map[string]string{
				SettingKeyRewardTasksEnabled: value,
			}}, &config.Config{})
			ctx := context.Background()
			want := value == "true"
			require.Equal(t, want, svc.IsRewardTasksEnabled(ctx))
			settings, err := svc.GetRewardTaskSettings(ctx)
			require.NoError(t, err)
			require.Equal(t, want, settings.Enabled)
			system, err := svc.GetAllSettings(ctx)
			require.NoError(t, err)
			require.Equal(t, want, system.RewardTasksEnabled)
			public, err := svc.GetPublicSettings(ctx)
			require.NoError(t, err)
			require.Equal(t, want, public.RewardTasksEnabled)
			injected, err := svc.GetPublicSettingsForInjection(ctx)
			require.NoError(t, err)
			encoded, err := json.Marshal(injected)
			require.NoError(t, err)
			var fields map[string]any
			require.NoError(t, json.Unmarshal(encoded, &fields))
			require.Equal(t, want, fields[SettingKeyRewardTasksEnabled])
		})
	}
}

func TestRewardTasksSettingsWritePreservesOtherSettingsAndRefreshesCaches(t *testing.T) {
	repo := &panelRateLimitSettingRepo{values: map[string]string{
		SettingKeySiteName: "Keep site", "unrecognized_setting": "keep value",
	}}
	svc := NewSettingService(repo, &config.Config{})
	updates := 0
	svc.SetOnUpdateCallback(func() { updates++ })
	ctx := context.Background()
	for _, enabled := range []bool{true, false} {
		require.NoError(t, svc.SetRewardTasksEnabled(ctx, enabled))
		require.Equal(t, enabled, svc.IsRewardTasksEnabled(ctx))
		require.Equal(t, "Keep site", repo.values[SettingKeySiteName])
		require.Equal(t, "keep value", repo.values["unrecognized_setting"])
		require.Len(t, repo.values, 3, "dedicated update must write only its own key")
	}
	require.Equal(t, 2, updates)
	full, err := svc.GetAllSettings(ctx)
	require.NoError(t, err)
	full.RewardTasksEnabled = true
	require.NoError(t, svc.UpdateSettings(ctx, full))
	require.True(t, svc.IsRewardTasksEnabled(ctx))
	require.NoError(t, svc.UpdateSettingsOmitting(ctx, &SystemSettings{}, OmittedSettingKeys{SettingKeyRewardTasksEnabled: {}}))
	require.True(t, svc.IsRewardTasksEnabled(ctx))
}

func TestRewardTasksSettingsDefaultsAndReadFailure(t *testing.T) {
	repo := &panelRateLimitSettingRepo{values: map[string]string{}}
	svc := NewSettingService(repo, &config.Config{})
	require.False(t, svc.IsRewardTasksEnabled(context.Background()))
	require.NoError(t, svc.InitializeDefaultSettings(context.Background()))
	require.Equal(t, "false", repo.values[SettingKeyRewardTasksEnabled])
	repo.getValueErr = errors.New("settings unavailable")
	_, err := svc.GetRewardTaskSettings(context.Background())
	require.Error(t, err)
	require.False(t, svc.IsRewardTasksEnabled(context.Background()))
	var missing *SettingService
	require.False(t, missing.IsRewardTasksEnabled(context.Background()))
}
