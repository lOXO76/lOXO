package handler

import (
	"context"
	"errors"
	"io"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

type CheckInHandler struct {
	service CheckInServicePort
}

type CheckInServicePort interface {
	GetStatus(ctx context.Context, userID int64) (*service.CheckInStatus, error)
	GetMonthCalendar(ctx context.Context, userID int64, month string) (*service.CheckInCalendar, error)
	PerformCheckIn(ctx context.Context, userID int64, proof service.CaptchaProof, remoteIP string) (*service.CheckInResult, error)
}

func NewCheckInHandler(checkInService CheckInServicePort) *CheckInHandler {
	return &CheckInHandler{service: checkInService}
}

func (h *CheckInHandler) GetStatus(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	status, err := h.service.GetStatus(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, status)
}

// GetCalendar GET /user/check-in/calendar?month=YYYY-MM
// 返回指定月份的签到记录；month 省略时取服务器时区的当前月。
func (h *CheckInHandler) GetCalendar(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	calendar, err := h.service.GetMonthCalendar(c.Request.Context(), subject.UserID, c.Query("month"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, calendar)
}

type CheckInRequest struct {
	TurnstileToken string `json:"turnstile_token"`
	TencentTicket  string `json:"tencent_ticket"`
	TencentRandstr string `json:"tencent_randstr"`
}

func (h *CheckInHandler) PerformCheckIn(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req CheckInRequest
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	result, err := h.service.PerformCheckIn(c.Request.Context(), subject.UserID, service.CaptchaProof{
		TurnstileToken: req.TurnstileToken,
		TencentTicket:  req.TencentTicket,
		TencentRandstr: req.TencentRandstr,
	}, c.ClientIP())
	if err != nil {
		middleware2.SetAuditExtra(c, map[string]any{"result": "failed", "error_code": infraerrors.Reason(err)})
		response.ErrorFrom(c, err)
		return
	}
	// 操作日志留存本次签到额度的入账结果，便于与余额流水中的「签到额度」记录核对。
	middleware2.SetAuditExtra(c, map[string]any{
		"result":          "success",
		"check_in_reward": result.Reward,
		"check_in_date":   result.CheckInDate,
		"streak_days":     result.StreakDays,
	})
	response.Success(c, result)
}
