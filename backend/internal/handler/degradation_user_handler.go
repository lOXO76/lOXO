package handler

import (
	"strconv"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type DegradationUserHandler struct {
	svc      *service.DegradationService
	settings *service.SettingService
	keys     *service.APIKeyService
	groups   service.GroupRepository
	tokens   *service.DegradationVisualTokenService
}

func NewDegradationUserHandler(svc *service.DegradationService, settings *service.SettingService, keys *service.APIKeyService, groups service.GroupRepository, tokens *service.DegradationVisualTokenService) *DegradationUserHandler {
	return &DegradationUserHandler{svc: svc, settings: settings, keys: keys, groups: groups, tokens: tokens}
}

func (h *DegradationUserHandler) viewerGroups(c *gin.Context) ([]int64, bool) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "Authentication required")
		return nil, false
	}
	allowed, restrictPublic, err := h.keys.GetUserGroupVisibility(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return nil, false
	}
	groups, err := h.groups.ListActive(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return nil, false
	}
	ids := make([]int64, 0, len(groups))
	for _, group := range groups {
		_, authorized := allowed[group.ID]
		if authorized || (!group.IsExclusive && !restrictPublic) {
			ids = append(ids, group.ID)
		}
	}
	return ids, true
}

func (h *DegradationUserHandler) Overview(c *gin.Context) {
	windowHours, err := parseDegradationWindow(c.Query("window_hours"))
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	runtime := h.settings.GetDegradationRuntime(c.Request.Context())
	if !runtime.Enabled {
		response.Success(c, service.DegradationOverview{Enabled: false, WindowHours: windowHours, Threshold: runtime.Threshold, GeneratedAt: time.Now().UTC(), Groups: []service.DegradationGroupOverview{}})
		return
	}
	ids, ok := h.viewerGroups(c)
	if !ok {
		return
	}
	result, err := h.svc.GetOverview(c.Request.Context(), ids, windowHours)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *DegradationUserHandler) Runs(c *gin.Context) {
	out := make([]service.DegradationRunListItem, 0)
	if !h.settings.GetDegradationRuntime(c.Request.Context()).Enabled {
		response.Success(c, gin.H{"items": out})
		return
	}
	ids, ok := h.viewerGroups(c)
	if !ok {
		return
	}
	if rawGroupID := c.Query("group_id"); rawGroupID != "" {
		groupID, err := strconv.ParseInt(rawGroupID, 10, 64)
		if err != nil || groupID <= 0 {
			response.BadRequest(c, "group_id must be a positive integer")
			return
		}
		visible := false
		for _, id := range ids {
			if id == groupID {
				visible = true
				break
			}
		}
		if !visible {
			response.Success(c, gin.H{"items": out})
			return
		}
		ids = []int64{groupID}
	}
	if len(ids) == 0 {
		response.Success(c, gin.H{"items": out})
		return
	}
	limit, _ := strconv.Atoi(c.Query("limit"))
	items, err := h.svc.GetRuns(c.Request.Context(), ids, c.Query("model"), limit, false)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	for _, run := range items {
		out = append(out, service.DegradationRunListItem{ID: run.ID, GroupID: run.GroupID, Model: run.Model, ProbeName: run.ProbeName, Status: run.Status, ProbeType: run.ProbeType, ArtifactKind: run.ArtifactKind, LatencyMs: run.LatencyMs, CreatedAt: run.CreatedAt})
	}
	response.Success(c, gin.H{"items": out})
}

func (h *DegradationUserHandler) VisualRuns(c *gin.Context) {
	items := []service.DegradationShowcase{}
	if !h.settings.GetDegradationRuntime(c.Request.Context()).Enabled {
		response.Success(c, gin.H{"items": items})
		return
	}
	ids, ok := h.viewerGroups(c)
	if !ok {
		return
	}
	groupID, _ := strconv.ParseInt(c.Query("group_id"), 10, 64)
	probeID, _ := strconv.ParseInt(c.Query("probe_id"), 10, 64)
	limit, _ := strconv.Atoi(c.Query("limit"))
	items, err := h.svc.ListVisualRuns(c.Request.Context(), ids, groupID, c.Query("model"), probeID, limit)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"items": items})
}

func (h *DegradationUserHandler) CreateVisualViewToken(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 || !h.settings.GetDegradationRuntime(c.Request.Context()).Enabled {
		response.ErrorFrom(c, service.ErrDegradationRunNotFound)
		return
	}
	ids, ok := h.viewerGroups(c)
	if !ok {
		return
	}
	if _, err := h.svc.GetVisualRunForToken(c.Request.Context(), id, ids, false); err != nil {
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

func parseDegradationWindow(raw string) (int, error) {
	if raw == "" {
		return 24, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 || value > 168 {
		return 0, service.ErrDegradationInvalidWindow
	}
	return value, nil
}
