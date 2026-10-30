//go:build unit

package service

import (
	"context"
	"errors"
	"math"
	"strconv"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestDegradationSettings_InitializeDefaults(t *testing.T) {
	repo := &forwardedIPMigrationRepoStub{values: map[string]string{}}
	svc := NewSettingService(repo, &config.Config{})

	require.NoError(t, svc.InitializeDefaultSettings(context.Background()))
	require.Equal(t, "true", repo.values[SettingKeyDegradationDetectionEnabled])
	require.Equal(t, "0.8", repo.values[SettingKeyDegradationThreshold])
	require.Equal(t, "1800", repo.values[SettingKeyDegradationDefaultIntervalSeconds])
}

func TestDegradationSettings_ParseAndRuntime(t *testing.T) {
	tests := []struct {
		name     string
		values   map[string]string
		expected DegradationRuntime
	}{
		{name: "missing", expected: defaultDegradationRuntime()},
		{
			name: "lower bounds and disabled",
			values: map[string]string{
				SettingKeyDegradationDetectionEnabled:       "false",
				SettingKeyDegradationThreshold:              "0",
				SettingKeyDegradationDefaultIntervalSeconds: "300",
			},
			expected: DegradationRuntime{Enabled: false, Threshold: 0, DefaultIntervalSeconds: 300},
		},
		{
			name: "upper bounds",
			values: map[string]string{
				SettingKeyDegradationDetectionEnabled:       "true",
				SettingKeyDegradationThreshold:              "1",
				SettingKeyDegradationDefaultIntervalSeconds: "86400",
			},
			expected: DegradationRuntime{Enabled: true, Threshold: 1, DefaultIntervalSeconds: 86400},
		},
		{
			name: "malformed values",
			values: map[string]string{
				SettingKeyDegradationDetectionEnabled:       "invalid",
				SettingKeyDegradationThreshold:              "NaN",
				SettingKeyDegradationDefaultIntervalSeconds: "invalid",
			},
			expected: defaultDegradationRuntime(),
		},
		{
			name: "out of range values",
			values: map[string]string{
				SettingKeyDegradationThreshold:              "1.01",
				SettingKeyDegradationDefaultIntervalSeconds: "299",
			},
			expected: defaultDegradationRuntime(),
		},
		{
			name: "infinite and excessive interval",
			values: map[string]string{
				SettingKeyDegradationThreshold:              "+Inf",
				SettingKeyDegradationDefaultIntervalSeconds: "86401",
			},
			expected: defaultDegradationRuntime(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewSettingService(&settingPublicRepoStub{values: tt.values}, &config.Config{})
			settings := svc.parseSettings(tt.values)
			require.Equal(t, tt.expected.Enabled, settings.DegradationDetectionEnabled)
			require.Equal(t, tt.expected.Threshold, settings.DegradationThreshold)
			require.Equal(t, tt.expected.DefaultIntervalSeconds, settings.DegradationDefaultIntervalSeconds)
			require.Equal(t, tt.expected, svc.GetDegradationRuntime(context.Background()))
		})
	}
	var nilService *SettingService
	require.Equal(t, defaultDegradationRuntime(), nilService.GetDegradationRuntime(context.Background()))
	svc := NewSettingService(&settingPublicRepoStub{err: errors.New("unavailable")}, &config.Config{})
	require.Equal(t, defaultDegradationRuntime(), svc.GetDegradationRuntime(context.Background()))
}

func TestDegradationSettings_BuildUpdates(t *testing.T) {
	svc := NewSettingService(newMockSettingRepo(), &config.Config{})
	for _, threshold := range []float64{0, 0.8, 1} {
		for _, interval := range []int{300, 1800, 86400} {
			settings := &SystemSettings{
				DegradationDetectionEnabled:       true,
				DegradationThreshold:              threshold,
				DegradationDefaultIntervalSeconds: interval,
			}
			updates, err := svc.buildSystemSettingsUpdates(context.Background(), settings)
			require.NoError(t, err)
			require.Equal(t, "true", updates[SettingKeyDegradationDetectionEnabled])
			require.Equal(t, strconv.FormatFloat(threshold, 'f', -1, 64), updates[SettingKeyDegradationThreshold])
			require.Equal(t, strconv.Itoa(interval), updates[SettingKeyDegradationDefaultIntervalSeconds])
		}
	}
	updates, err := svc.buildSystemSettingsUpdates(context.Background(), &SystemSettings{})
	require.NoError(t, err)
	require.Equal(t, "1800", updates[SettingKeyDegradationDefaultIntervalSeconds])
}

func TestDegradationSettings_RejectInvalidUpdates(t *testing.T) {
	svc := NewSettingService(newMockSettingRepo(), &config.Config{})
	for _, threshold := range []float64{-0.01, 1.01, math.NaN(), math.Inf(1), math.Inf(-1)} {
		_, err := svc.buildSystemSettingsUpdates(context.Background(), &SystemSettings{
			DegradationThreshold: threshold,
		})
		require.ErrorContains(t, err, "degradation_threshold")
	}
	for _, interval := range []int{-1, 1, 299, 86401} {
		_, err := svc.buildSystemSettingsUpdates(context.Background(), &SystemSettings{
			DegradationDefaultIntervalSeconds: interval,
		})
		require.ErrorContains(t, err, "degradation_default_interval_seconds")
	}
}
