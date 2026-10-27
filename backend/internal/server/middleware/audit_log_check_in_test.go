package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestCheckInAuditRecordsCreditAndOmitsCaptchaBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repository := &auditCaptureRepository{}
	auditService := service.NewAuditLogService(repository, nil)
	auditService.Start()

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(ContextKeyUser), AuthSubject{UserID: 7})
		c.Set(string(ContextKeyUserRole), "user")
		c.Next()
	})
	router.Use(gin.HandlerFunc(NewAuditLogMiddleware(auditService)))
	router.POST("/api/v1/user/check-in", func(c *gin.Context) {
		SetAuditExtra(c, map[string]any{
			"result":          "success",
			"check_in_reward": 0.2783,
			"check_in_date":   "2026-10-27",
			"streak_days":     6,
		})
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	// tencent_ticket / tencent_randstr 不在键名脱敏清单内，必须靠整体不入库来保护。
	request := httptest.NewRequest(http.MethodPost, "/api/v1/user/check-in",
		bytes.NewBufferString(`{"tencent_ticket":"captcha-canary-ticket","tencent_randstr":"captcha-canary-rand"}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(httptest.NewRecorder(), request)
	auditService.Stop()

	repository.mu.Lock()
	logs := append([]*service.AuditLog(nil), repository.logs...)
	repository.mu.Unlock()
	require.Len(t, logs, 1)

	entry := logs[0]
	require.Equal(t, service.AuditActionCheckIn, entry.Action)
	require.Equal(t, "<credential-bearing body omitted>", entry.RequestBody)
	require.NotContains(t, entry.RequestBody, "captcha-canary")
	require.NotNil(t, entry.ActorUserID)
	require.Equal(t, int64(7), *entry.ActorUserID)
	require.Equal(t, "success", entry.Extra["result"])
	require.InDelta(t, 0.2783, entry.Extra["check_in_reward"], 1e-9)
	require.Equal(t, "2026-10-27", entry.Extra["check_in_date"])
	require.EqualValues(t, 6, entry.Extra["streak_days"])
}
