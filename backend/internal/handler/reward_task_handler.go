package handler

import (
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"strconv"
)

type RewardTaskHandler struct{ service *service.RewardTaskService }

func NewRewardTaskHandler(s *service.RewardTaskService) *RewardTaskHandler {
	return &RewardTaskHandler{service: s}
}
func (h *RewardTaskHandler) List(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	result, err := h.service.ListUserTasks(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}
func (h *RewardTaskHandler) Start(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.ErrorFrom(c, infraerrors.NotFound("REWARD_TASK_NOT_FOUND", "task not found"))
		return
	}
	result, err := h.service.StartTask(c.Request.Context(), subject.UserID, id)
	if err != nil {
		middleware.SetAuditExtra(c, map[string]any{"result": "failed", "error_code": infraerrors.Reason(err)})
		response.ErrorFrom(c, err)
		return
	}
	middleware.SetAuditExtra(c, map[string]any{"result": "success", "task_id": id})
	response.Success(c, result)
}
func (h *RewardTaskHandler) Claim(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.ErrorFrom(c, infraerrors.NotFound("REWARD_TASK_NOT_FOUND", "task not found"))
		return
	}
	result, err := h.service.ClaimTimerTask(c.Request.Context(), subject.UserID, id)
	if err != nil {
		middleware.SetAuditExtra(c, map[string]any{"result": "failed", "error_code": infraerrors.Reason(err)})
		response.ErrorFrom(c, err)
		return
	}
	middleware.SetAuditExtra(c, map[string]any{"result": "success", "task_id": id, "reward_amount": result.RewardAmount})
	response.Success(c, result)
}
func (h *RewardTaskHandler) Records(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	page, size := response.ParsePagination(c)
	if size > 100 {
		size = 100
	}
	items, total, err := h.service.ListUserRecords(c.Request.Context(), subject.UserID, page, size)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, total, page, size)
}
