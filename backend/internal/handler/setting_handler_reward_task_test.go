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

func TestSettingHandlerGetPublicSettingsRewardTasksOptIn(t *testing.T) {
	for _, value := range []string{"", "false", "true"} {
		t.Run(value, func(t *testing.T) {
			repo := &settingHandlerPublicRepoStub{values: map[string]string{service.SettingKeyRewardTasksEnabled: value}}
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
			require.Equal(t, value == "true", payload.Data[service.SettingKeyRewardTasksEnabled])
		})
	}
}
