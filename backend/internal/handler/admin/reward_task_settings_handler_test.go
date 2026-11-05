package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRewardTaskSettingsHandlerReadWrite(t *testing.T) {
	handler, repo := newStepUpSwitchTestHandler(t, map[string]string{service.SettingKeySiteName: "Keep site"})
	rewardHandler := NewRewardTaskSettingsHandler(handler.settingService)
	updates := 0
	handler.settingService.SetOnUpdateCallback(func() { updates++ })
	for _, body := range []string{`{"enabled":true}`, `{"enabled":false}`} {
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		ctx.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/reward-task-settings", strings.NewReader(body))
		ctx.Request.Header.Set("Content-Type", "application/json")
		rewardHandler.UpdateSettings(ctx)
		require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
		require.Equal(t, "Keep site", repo.values[service.SettingKeySiteName])
		require.Len(t, repo.values, 2)
		var payload struct {
			Data service.RewardTaskSettings `json:"data"`
		}
		require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
		require.Equal(t, strings.Contains(body, "true"), payload.Data.Enabled)
		getRecorder := httptest.NewRecorder()
		getCtx, _ := gin.CreateTestContext(getRecorder)
		getCtx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/reward-task-settings", nil)
		rewardHandler.GetSettings(getCtx)
		require.Equal(t, recorder.Body.String(), getRecorder.Body.String())
	}
	require.Equal(t, 2, updates)
}

func TestRewardTaskSettingsHandlerRejectsMissingOrInvalidEnabled(t *testing.T) {
	for _, body := range []string{`{}`, `{"enabled":null}`, `{"enabled":"true"}`, `{"enabled":1}`} {
		t.Run(body, func(t *testing.T) {
			handler, repo := newStepUpSwitchTestHandler(t, map[string]string{service.SettingKeyRewardTasksEnabled: "true"})
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/reward-task-settings", strings.NewReader(body))
			NewRewardTaskSettingsHandler(handler.settingService).UpdateSettings(ctx)
			require.Equal(t, http.StatusBadRequest, recorder.Code)
			require.Equal(t, "true", repo.values[service.SettingKeyRewardTasksEnabled])
		})
	}
}

func TestSystemSettingsRewardTaskFlagAndAudit(t *testing.T) {
	handler, repo := newStepUpSwitchTestHandler(t, map[string]string{})
	before, err := handler.settingService.GetAllSettings(context.Background())
	require.NoError(t, err)
	recorder := doUpdateSettings(t, handler, map[string]any{service.SettingKeyRewardTasksEnabled: true}, nil)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, "true", repo.values[service.SettingKeyRewardTasksEnabled])
	var payload struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	require.Equal(t, true, payload.Data[service.SettingKeyRewardTasksEnabled])
	after, err := handler.settingService.GetAllSettings(context.Background())
	require.NoError(t, err)
	require.Contains(t, diffSettings(before, after, nil, nil, UpdateSettingsRequest{}), service.SettingKeyRewardTasksEnabled)
	require.NotContains(t, diffSettings(after, after, nil, nil, UpdateSettingsRequest{}), service.SettingKeyRewardTasksEnabled)
	recorder = doUpdateSettings(t, handler, map[string]any{service.SettingKeySiteName: "Updated site"}, nil)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, "true", repo.values[service.SettingKeyRewardTasksEnabled])
}
