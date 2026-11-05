//go:build unit

package handler

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func TestRewardOpenRejectsOversizedBody(t *testing.T) {
	for _, size := range []int{service.RewardOpenMaxBodyBytes, service.RewardOpenMaxBodyBytes + 1} {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest("POST", "/api/v1/reward-open/events", strings.NewReader(strings.Repeat("x", size)))
		body, err := readRewardOpenBody(c)
		if len(body) != size || (err != nil) != (size > service.RewardOpenMaxBodyBytes) {
			t.Fatalf("size=%d read=%d err=%v", size, len(body), err)
		}
	}
}

func TestRewardOpenHandlerValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewRewardTaskOpenHandler(&service.RewardTaskService{})
	for _, tc := range []struct {
		name   string
		method func(*gin.Context)
		body   string
		code   int
		reason string
	}{
		{"unsigned ping", handler.Ping, `{}`, 401, "REWARD_SIGNATURE_MISSING"},
		{"unsigned query", handler.QueryTicket, `{}`, 401, "REWARD_SIGNATURE_MISSING"},
		{"unsigned event", handler.Event, `{}`, 401, "REWARD_SIGNATURE_MISSING"},
		{"event body limit", handler.Event, strings.Repeat("x", service.RewardOpenMaxBodyBytes+1), 400, "REWARD_EVENT_INVALID"},
		{"malformed SDK JSON", handler.Heartbeat, `{`, 400, "REWARD_EVENT_INVALID"},
		{"missing SDK key", handler.Heartbeat, `{"ticket":"rt_x"}`, 400, "REWARD_EVENT_INVALID"},
		{"text plain SDK accepted", handler.Heartbeat, `{"app_key":"rk_x","ticket":"rt_x"}`, 403, "REWARD_TASKS_DISABLED"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest("POST", "/api/v1/reward-open/events", strings.NewReader(tc.body))
			c.Request.Header.Set("Content-Type", "text/plain")
			tc.method(c)
			var result struct {
				Reason string `json:"reason"`
			}
			if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if recorder.Code != tc.code || result.Reason != tc.reason {
				t.Fatalf("HTTP %d reason=%q, want HTTP %d reason=%q", recorder.Code, result.Reason, tc.code, tc.reason)
			}
		})
	}
}
