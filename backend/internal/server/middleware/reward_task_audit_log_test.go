package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRewardTaskAuditActionsAndPartnerResponseSecrets(t *testing.T) {
	repository := &auditCaptureRepository{}
	auditService := service.NewAuditLogService(repository, nil)
	auditService.Start()
	router := gin.New()
	router.Use(gin.HandlerFunc(NewAuditLogMiddleware(auditService)))
	for _, route := range []string{
		"/api/v1/user/reward-tasks/:id/start",
		"/api/v1/user/reward-tasks/:id/claim",
		"/api/v1/admin/reward-task-partners",
		"/api/v1/admin/reward-task-partners/:id/reset-secret",
	} {
		router.POST(route, func(c *gin.Context) {
			SetAuditExtra(c, map[string]any{"app_secret": "rs_response-secret-canary"})
			c.JSON(http.StatusOK, gin.H{"app_secret": "rs_response-secret-canary"})
		})
	}
	for _, path := range []string{
		"/api/v1/user/reward-tasks/1/start", "/api/v1/user/reward-tasks/1/claim",
		"/api/v1/admin/reward-task-partners", "/api/v1/admin/reward-task-partners/1/reset-secret",
	} {
		request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`))
		request.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		require.Equal(t, http.StatusOK, recorder.Code)
		require.Contains(t, recorder.Body.String(), "rs_response-secret-canary")
	}
	auditService.Stop()
	repository.mu.Lock()
	logs := append([]*service.AuditLog(nil), repository.logs...)
	repository.mu.Unlock()
	require.Len(t, logs, 4)
	actions := make([]string, 0, len(logs))
	for _, entry := range logs {
		encoded, err := json.Marshal(entry)
		require.NoError(t, err)
		require.NotContains(t, string(encoded), "rs_response-secret-canary", "response bodies must never enter audit logs")
		actions = append(actions, entry.Action)
	}
	require.Contains(t, actions, "user.reward_task.start")
	require.Contains(t, actions, "user.reward_task.claim")
}
