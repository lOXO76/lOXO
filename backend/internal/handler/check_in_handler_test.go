//go:build unit

package handler

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

type checkInHandlerServiceStub struct{}

func (checkInHandlerServiceStub) GetStatus(context.Context, int64) (*service.CheckInStatus, error) {
	return nil, service.ErrCheckInAlreadyDone
}

func (checkInHandlerServiceStub) GetMonthCalendar(context.Context, int64, string) (*service.CheckInCalendar, error) {
	return nil, service.ErrCheckInAlreadyDone
}

func (checkInHandlerServiceStub) PerformCheckIn(context.Context, int64, service.CaptchaProof, string) (*service.CheckInResult, error) {
	return nil, service.ErrCheckInAlreadyDone
}

func TestCheckInHandlerMapsAlreadyDoneError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest("GET", "/api/v1/user/check-in", nil)
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 7})
	NewCheckInHandler(checkInHandlerServiceStub{}).GetStatus(c)
	if recorder.Code != 409 {
		t.Fatalf("status = %d, want 409", recorder.Code)
	}
	var body struct {
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Reason != "CHECK_IN_ALREADY_DONE" {
		t.Fatalf("reason = %q", body.Reason)
	}
}

type checkInHandlerSuccessStub struct{ checkInHandlerServiceStub }

func (checkInHandlerSuccessStub) PerformCheckIn(context.Context, int64, service.CaptchaProof, string) (*service.CheckInResult, error) {
	return &service.CheckInResult{Reward: 0.2783, Balance: 5.2783, CheckInDate: "2026-10-27", StreakDays: 6}, nil
}

func performCheckInForAudit(t *testing.T, svc CheckInServicePort) (int, map[string]any) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest("POST", "/api/v1/user/check-in", nil)
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 7})
	NewCheckInHandler(svc).PerformCheckIn(c)
	value, ok := c.Get("audit_extra")
	if !ok {
		t.Fatal("audit extra was not recorded")
	}
	return recorder.Code, value.(map[string]any)
}

func TestCheckInHandlerRecordsCheckInCreditForAudit(t *testing.T) {
	code, extra := performCheckInForAudit(t, checkInHandlerSuccessStub{})
	if code != 200 {
		t.Fatalf("status = %d, want 200", code)
	}
	if extra["result"] != "success" || extra["check_in_reward"] != 0.2783 || extra["check_in_date"] != "2026-10-27" || extra["streak_days"] != 6 {
		t.Fatalf("unexpected audit extra: %#v", extra)
	}
}

func TestCheckInHandlerRecordsFailureReasonForAudit(t *testing.T) {
	code, extra := performCheckInForAudit(t, checkInHandlerServiceStub{})
	if code != 409 {
		t.Fatalf("status = %d, want 409", code)
	}
	if extra["result"] != "failed" || extra["error_code"] != "CHECK_IN_ALREADY_DONE" {
		t.Fatalf("unexpected audit extra: %#v", extra)
	}
	if _, leaked := extra["check_in_reward"]; leaked {
		t.Fatalf("failed check-in must not record a credit: %#v", extra)
	}
}
