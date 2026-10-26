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
