//go:build unit

package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSettingHandler_GetPublicSettings_CreationEnabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
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
			repo := &settingHandlerPublicRepoStub{values: test.values}
			handler := NewSettingHandler(service.NewSettingService(repo, &config.Config{}), "test")
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/settings/public", nil)
			handler.GetPublicSettings(ctx)
			require.Equal(t, http.StatusOK, recorder.Code)
			var payload struct {
				Data map[string]any `json:"data"`
			}
			require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
			require.Equal(t, test.enabled, payload.Data[service.SettingKeyCreationEnabled])
		})
	}
}
