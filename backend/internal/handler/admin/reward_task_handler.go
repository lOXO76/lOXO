package admin

import (
	"strconv"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type RewardTaskHandler struct {
	svc *service.RewardTaskService
}

func NewRewardTaskHandler(svc *service.RewardTaskService) *RewardTaskHandler {
	return &RewardTaskHandler{svc: svc}
}

func rewardAdminInvalid(c *gin.Context, message string) {
	response.ErrorFrom(c, infraerrors.BadRequest("REWARD_TASK_INVALID", message))
}

func rewardAdminID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		rewardAdminInvalid(c, "id must be a positive integer")
		return 0, false
	}
	return id, true
}

func rewardAdminOptionalID(c *gin.Context, key string) (int64, bool) {
	raw := c.Query(key)
	if raw == "" {
		return 0, true
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		rewardAdminInvalid(c, key+" must be a positive integer")
		return 0, false
	}
	return id, true
}

func rewardAdminPagination(c *gin.Context) (int, int) {
	page, size := response.ParsePagination(c)
	if size > 100 {
		size = 100
	}
	return page, size
}

func (h *RewardTaskHandler) ListTasks(c *gin.Context) {
	page, size := rewardAdminPagination(c)
	filter := service.RewardTaskAdminFilter{Page: page, PageSize: size, TaskType: c.Query("task_type"), Search: c.Query("search")}
	if filter.TaskType != "" && filter.TaskType != "browse" && filter.TaskType != "register" && filter.TaskType != "recharge" {
		rewardAdminInvalid(c, "task_type must be browse, register or recharge")
		return
	}
	if raw := c.Query("enabled"); raw != "" {
		if raw != "true" && raw != "false" {
			rewardAdminInvalid(c, "enabled must be true or false")
			return
		}
		enabled := raw == "true"
		filter.Enabled = &enabled
	}
	items, total, err := h.svc.ListAdminTasks(c.Request.Context(), filter)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, total, page, size)
}

func (h *RewardTaskHandler) CreateTask(c *gin.Context) {
	in := service.RewardTaskInput{IdleTimeoutSeconds: 15}
	if err := c.ShouldBindJSON(&in); err != nil {
		rewardAdminInvalid(c, err.Error())
		return
	}
	item, err := h.svc.CreateAdminTask(c.Request.Context(), in)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, item)
}

func (h *RewardTaskHandler) UpdateTask(c *gin.Context) {
	id, ok := rewardAdminID(c)
	if !ok {
		return
	}
	in := service.RewardTaskInput{IdleTimeoutSeconds: 15}
	if err := c.ShouldBindJSON(&in); err != nil {
		rewardAdminInvalid(c, err.Error())
		return
	}
	item, err := h.svc.UpdateAdminTask(c.Request.Context(), id, in)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, item)
}

func (h *RewardTaskHandler) DeleteTask(c *gin.Context) {
	id, ok := rewardAdminID(c)
	if !ok {
		return
	}
	if err := h.svc.DeleteAdminTask(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

func (h *RewardTaskHandler) GetStats(c *gin.Context) {
	stats, err := h.svc.GetAdminStats(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, stats)
}

func (h *RewardTaskHandler) ListPartners(c *gin.Context) {
	items, err := h.svc.ListAdminPartners(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, items)
}

func (h *RewardTaskHandler) CreatePartner(c *gin.Context) {
	var in service.RewardTaskPartnerInput
	if err := c.ShouldBindJSON(&in); err != nil {
		rewardAdminInvalid(c, err.Error())
		return
	}
	item, err := h.svc.CreateAdminPartner(c.Request.Context(), in)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, item)
}

func (h *RewardTaskHandler) UpdatePartner(c *gin.Context) {
	id, ok := rewardAdminID(c)
	if !ok {
		return
	}
	var in service.RewardTaskPartnerInput
	if err := c.ShouldBindJSON(&in); err != nil {
		rewardAdminInvalid(c, err.Error())
		return
	}
	item, err := h.svc.UpdateAdminPartner(c.Request.Context(), id, in)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, item)
}

func (h *RewardTaskHandler) ResetPartnerSecret(c *gin.Context) {
	id, ok := rewardAdminID(c)
	if !ok {
		return
	}
	item, err := h.svc.ResetAdminPartnerSecret(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, item)
}

func (h *RewardTaskHandler) DeletePartner(c *gin.Context) {
	id, ok := rewardAdminID(c)
	if !ok {
		return
	}
	if err := h.svc.DeleteAdminPartner(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

func (h *RewardTaskHandler) ListRecords(c *gin.Context) {
	taskID, ok := rewardAdminOptionalID(c, "task_id")
	if !ok {
		return
	}
	page, size := rewardAdminPagination(c)
	items, total, err := h.svc.ListAdminRecords(c.Request.Context(), service.RewardTaskRecordAdminFilter{Page: page, PageSize: size, TaskID: taskID, Status: c.Query("status"), Search: c.Query("search")})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, total, page, size)
}

func (h *RewardTaskHandler) ListEvents(c *gin.Context) {
	partnerID, ok := rewardAdminOptionalID(c, "partner_id")
	if !ok {
		return
	}
	page, size := rewardAdminPagination(c)
	items, total, err := h.svc.ListAdminEvents(c.Request.Context(), service.RewardTaskEventAdminFilter{Page: page, PageSize: size, PartnerID: partnerID, Result: c.Query("result")})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, total, page, size)
}
