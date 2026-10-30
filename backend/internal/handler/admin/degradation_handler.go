package admin

import (
	"strconv"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type DegradationHandler struct {
	svc    *service.DegradationService
	runner *service.DegradationRunner
	tokens *service.DegradationVisualTokenService
}

func NewDegradationHandler(svc *service.DegradationService, runner *service.DegradationRunner, tokens *service.DegradationVisualTokenService) *DegradationHandler {
	return &DegradationHandler{svc: svc, runner: runner, tokens: tokens}
}

func (h *DegradationHandler) ListProbes(c *gin.Context) {
	items, err := h.svc.ListProbes(c.Request.Context(), true)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"items": items})
}

func (h *DegradationHandler) CreateProbe(c *gin.Context) {
	var req service.DegradationProbeCreateParams
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	item, err := h.svc.CreateProbe(c.Request.Context(), req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, item)
}

func (h *DegradationHandler) UpdateProbe(c *gin.Context) {
	id, ok := degradationID(c)
	if !ok {
		return
	}
	var req service.DegradationProbeCreateParams
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	item, err := h.svc.UpdateProbe(c.Request.Context(), id, req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, item)
}

func (h *DegradationHandler) DeleteProbe(c *gin.Context) {
	id, ok := degradationID(c)
	if !ok {
		return
	}
	if err := h.svc.DeleteProbe(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

func (h *DegradationHandler) TestRegex(c *gin.Context) {
	var req struct {
		Regex      string `json:"regex"`
		Flags      string `json:"flags"`
		Negate     bool   `json:"negate"`
		SampleText string `json:"sample_text"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	matched, passed, err := h.svc.TestRegex(c.Request.Context(), req.Regex, req.Flags, req.Negate, req.SampleText)
	if err != nil {
		// The trial endpoint returns compile failures in its result so the
		// editor can display them inline; saving the same rule returns 400.
		response.Success(c, gin.H{"matched": false, "passed": false, "error": infraerrors.Message(err)})
		return
	}
	response.Success(c, gin.H{"matched": matched, "passed": passed, "error": ""})
}

func (h *DegradationHandler) ListTargets(c *gin.Context) {
	var groupID *int64
	if raw := c.Query("group_id"); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || id <= 0 {
			response.BadRequest(c, "group_id must be a positive integer")
			return
		}
		groupID = &id
	}
	items, err := h.svc.ListTargets(c.Request.Context(), groupID, true)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"items": items})
}

func (h *DegradationHandler) CreateTarget(c *gin.Context) {
	var req service.DegradationTargetParams
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	item, err := h.svc.CreateTarget(c.Request.Context(), req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, item)
}

func (h *DegradationHandler) UpdateTarget(c *gin.Context) {
	id, ok := degradationID(c)
	if !ok {
		return
	}
	var req service.DegradationTargetParams
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	item, err := h.svc.UpdateTarget(c.Request.Context(), id, req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, item)
}

func (h *DegradationHandler) DeleteTarget(c *gin.Context) {
	id, ok := degradationID(c)
	if !ok {
		return
	}
	if err := h.svc.DeleteTarget(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

func (h *DegradationHandler) RunTarget(c *gin.Context) {
	id, ok := degradationID(c)
	if !ok {
		return
	}
	result, err := h.runner.Trigger(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *DegradationHandler) CreateRunViewToken(c *gin.Context) {
	id, ok := degradationID(c)
	if !ok {
		return
	}
	if _, err := h.svc.GetVisualRunForToken(c.Request.Context(), id, nil, true); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	result, err := h.tokens.Issue(id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *DegradationHandler) TargetRuns(c *gin.Context) {
	id, ok := degradationID(c)
	if !ok {
		return
	}
	limit, _ := strconv.Atoi(c.Query("limit"))
	items, err := h.svc.GetTargetRuns(c.Request.Context(), id, limit)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"items": items})
}

func (h *DegradationHandler) Overview(c *gin.Context) {
	windowHours := 24
	if raw := c.Query("window_hours"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 || value > 168 {
			response.ErrorFrom(c, service.ErrDegradationInvalidWindow)
			return
		}
		windowHours = value
	}
	result, err := h.svc.GetOverview(c.Request.Context(), nil, windowHours, true)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

// PreviewAccount 返回该「分组 × 模型」会被调度到的账号，用于配置时自检。
// 契约规定入参走 POST body {group_id, model}；同时兼容 query，便于手工 curl
// 或早期调用方（body 优先，query 兜底）。
func (h *DegradationHandler) PreviewAccount(c *gin.Context) {
	var req struct {
		GroupID int64  `json:"group_id"`
		Model   string `json:"model"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		// 空 body / 非 JSON：退回到 query 解析。
		groupID, parseErr := strconv.ParseInt(c.Query("group_id"), 10, 64)
		req.GroupID, req.Model = groupID, c.Query("model")
		if parseErr != nil {
			req.GroupID = 0
		}
	}
	model := strings.TrimSpace(req.Model)
	if req.GroupID <= 0 || model == "" {
		response.BadRequest(c, "group_id and model are required")
		return
	}
	result, err := h.svc.PreviewAccount(c.Request.Context(), req.GroupID, model)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func degradationID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "id must be a positive integer")
		return 0, false
	}
	return id, true
}
