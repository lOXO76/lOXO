//go:build unit

package admin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestDegradationSettingsHandler_Get(t *testing.T) {
	tests := []struct {
		name      string
		stored    map[string]string
		enabled   bool
		threshold float64
		interval  int
	}{
		{name: "defaults", stored: map[string]string{}, enabled: true, threshold: 0.8, interval: 1800},
		{
			name: "stored values",
			stored: map[string]string{
				service.SettingKeyDegradationDetectionEnabled:       "false",
				service.SettingKeyDegradationThreshold:              "0",
				service.SettingKeyDegradationDefaultIntervalSeconds: "300",
			},
			enabled: false, threshold: 0, interval: 300,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, _ := newStepUpSwitchTestHandler(t, tt.stored)
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/settings", nil)
			h.GetSettings(c)
			require.Equal(t, http.StatusOK, rec.Code)
			assertDegradationSettingsResponse(t, rec, tt.enabled, tt.threshold, tt.interval)
		})
	}
}

func TestDegradationSettingsHandler_PutAndPreserve(t *testing.T) {
	h, repo := newStepUpSwitchTestHandler(t, map[string]string{
		service.SettingKeyDegradationDetectionEnabled:       "true",
		service.SettingKeyDegradationThreshold:              "0.9",
		service.SettingKeyDegradationDefaultIntervalSeconds: "3600",
	})
	rec := doUpdateSettings(t, h, map[string]any{
		"degradation_detection_enabled":        false,
		"degradation_threshold":                0,
		"degradation_default_interval_seconds": 300,
	}, nil)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "false", repo.values[service.SettingKeyDegradationDetectionEnabled])
	require.Equal(t, "0", repo.values[service.SettingKeyDegradationThreshold])
	require.Equal(t, "300", repo.values[service.SettingKeyDegradationDefaultIntervalSeconds])
	assertDegradationSettingsResponse(t, rec, false, 0, 300)

	for _, payload := range []map[string]any{
		{"site_name": "Preserve settings"},
		{"degradation_detection_enabled": nil, "degradation_threshold": nil, "degradation_default_interval_seconds": nil},
	} {
		rec = doUpdateSettings(t, h, payload, nil)
		require.Equal(t, http.StatusOK, rec.Code)
		require.Equal(t, "false", repo.values[service.SettingKeyDegradationDetectionEnabled])
		require.Equal(t, "0", repo.values[service.SettingKeyDegradationThreshold])
		require.Equal(t, "300", repo.values[service.SettingKeyDegradationDefaultIntervalSeconds])
		assertDegradationSettingsResponse(t, rec, false, 0, 300)
	}

	rec = doUpdateSettings(t, h, map[string]any{
		"degradation_detection_enabled":        true,
		"degradation_threshold":                1,
		"degradation_default_interval_seconds": 86400,
	}, nil)
	require.Equal(t, http.StatusOK, rec.Code)
	assertDegradationSettingsResponse(t, rec, true, 1, 86400)
}

func TestDegradationSettingsHandler_RejectInvalidPut(t *testing.T) {
	for _, payload := range []map[string]any{
		{"degradation_threshold": -0.01},
		{"degradation_threshold": 1.01},
		{"degradation_default_interval_seconds": 0},
		{"degradation_default_interval_seconds": 299},
		{"degradation_default_interval_seconds": 86401},
	} {
		h, repo := newStepUpSwitchTestHandler(t, map[string]string{
			service.SettingKeyDegradationThreshold:              "0.9",
			service.SettingKeyDegradationDefaultIntervalSeconds: "3600",
		})
		rec := doUpdateSettings(t, h, payload, nil)
		require.Equal(t, http.StatusBadRequest, rec.Code, "payload=%v body=%s", payload, rec.Body.String())
		require.Nil(t, repo.lastUpdates)
	}
}

func assertDegradationSettingsResponse(t *testing.T, rec *httptest.ResponseRecorder, enabled bool, threshold float64, interval int) {
	t.Helper()
	var body struct {
		Data struct {
			Enabled   *bool    `json:"degradation_detection_enabled"`
			Threshold *float64 `json:"degradation_threshold"`
			Interval  *int     `json:"degradation_default_interval_seconds"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.NotNil(t, body.Data.Enabled)
	require.NotNil(t, body.Data.Threshold)
	require.NotNil(t, body.Data.Interval)
	require.Equal(t, enabled, *body.Data.Enabled)
	require.Equal(t, threshold, *body.Data.Threshold)
	require.Equal(t, interval, *body.Data.Interval)
}
