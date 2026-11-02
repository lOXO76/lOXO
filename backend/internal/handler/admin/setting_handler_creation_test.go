package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSettingHandler_GetSettings_CreationEnabled(t *testing.T) {
	tests := []struct {
		name    string
		values  map[string]string
		enabled bool
	}{
		{name: "default", values: map[string]string{}, enabled: true},
		{name: "enabled", values: map[string]string{service.SettingKeyCreationEnabled: "true"}, enabled: true},
		{name: "disabled", values: map[string]string{service.SettingKeyCreationEnabled: "false"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler, _ := newStepUpSwitchTestHandler(t, test.values)
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/settings", nil)
			handler.GetSettings(ctx)
			require.Equal(t, http.StatusOK, recorder.Code)
			var payload struct {
				Data map[string]any `json:"data"`
			}
			require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
			require.Equal(t, test.enabled, payload.Data[service.SettingKeyCreationEnabled])
		})
	}
}

func TestSettingHandler_UpdateSettings_CreationEnabled(t *testing.T) {
	handler, repo := newStepUpSwitchTestHandler(t, map[string]string{})
	for _, enabled := range []bool{false, true} {
		t.Run(strconv.FormatBool(enabled), func(t *testing.T) {
			recorder := doUpdateSettings(t, handler, map[string]any{
				service.SettingKeyCreationEnabled: enabled,
			}, nil)
			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
			require.Equal(t, strconv.FormatBool(enabled), repo.values[service.SettingKeyCreationEnabled])
			require.Equal(t, enabled, handler.settingService.IsCreationEnabled(context.Background()))
			var payload struct {
				Data map[string]any `json:"data"`
			}
			require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
			require.Equal(t, enabled, payload.Data[service.SettingKeyCreationEnabled])
		})
	}
}

func TestSettingHandler_UpdateSettings_CreationEnabledPreserved(t *testing.T) {
	tests := []struct {
		name    string
		values  map[string]string
		body    map[string]any
		enabled bool
	}{
		{name: "default omitted", values: map[string]string{}, body: map[string]any{"site_name": "Example Gateway"}, enabled: true},
		{name: "enabled omitted", values: map[string]string{service.SettingKeyCreationEnabled: "true"}, body: map[string]any{"site_name": "Example Gateway"}, enabled: true},
		{name: "disabled omitted", values: map[string]string{service.SettingKeyCreationEnabled: "false"}, body: map[string]any{"site_name": "Example Gateway"}},
		{name: "default null", values: map[string]string{}, body: map[string]any{service.SettingKeyCreationEnabled: nil}, enabled: true},
		{name: "enabled null", values: map[string]string{service.SettingKeyCreationEnabled: "true"}, body: map[string]any{service.SettingKeyCreationEnabled: nil}, enabled: true},
		{name: "disabled null", values: map[string]string{service.SettingKeyCreationEnabled: "false"}, body: map[string]any{service.SettingKeyCreationEnabled: nil}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler, repo := newStepUpSwitchTestHandler(t, test.values)
			before, err := handler.settingService.GetAllSettings(context.Background())
			require.NoError(t, err)
			recorder := doUpdateSettings(t, handler, test.body, nil)
			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
			require.Equal(t, strconv.FormatBool(test.enabled), repo.values[service.SettingKeyCreationEnabled])
			var payload struct {
				Data map[string]any `json:"data"`
			}
			require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
			require.Equal(t, test.enabled, payload.Data[service.SettingKeyCreationEnabled])
			after, err := handler.settingService.GetAllSettings(context.Background())
			require.NoError(t, err)
			require.NotContains(t, diffSettings(before, after, nil, nil, UpdateSettingsRequest{}), service.SettingKeyCreationEnabled)
		})
	}
}

func TestDiffSettings_CreationEnabled(t *testing.T) {
	for _, before := range []bool{false, true} {
		for _, after := range []bool{false, true} {
			t.Run(strconv.FormatBool(before)+" to "+strconv.FormatBool(after), func(t *testing.T) {
				changed := diffSettings(
					&service.SystemSettings{CreationEnabled: before},
					&service.SystemSettings{CreationEnabled: after},
					nil, nil, UpdateSettingsRequest{CreationEnabled: &after},
				)
				if before == after {
					require.Empty(t, changed)
				} else {
					require.Equal(t, []string{service.SettingKeyCreationEnabled}, changed)
				}
			})
		}
	}
}
