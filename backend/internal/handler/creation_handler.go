package handler

import (
	"context"
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type CreationCatalogService interface {
	GetCatalog(context.Context, int64) (*service.CreationCatalog, error)
}

type CreationHandler struct {
	catalog  CreationCatalogService
	settings *service.SettingService
	costs    CreationCostService
}

type CreationCostService interface {
	GetCosts(context.Context, int64, []string) (*service.CreationCosts, error)
}

func NewCreationHandler(catalog CreationCatalogService, settings *service.SettingService, costs ...CreationCostService) *CreationHandler {
	h := &CreationHandler{catalog: catalog, settings: settings}
	if len(costs) > 0 {
		h.costs = costs[0]
	}
	return h
}

func ProvideCreationHandler(catalog *service.CreationCatalogService, settings *service.SettingService, costs *service.CreationCostService) *CreationHandler {
	return NewCreationHandler(catalog, settings, costs)
}

func (h *CreationHandler) GetCosts(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	ids := make([]string, 0, 20)
	seen := make(map[string]bool)
	for _, raw := range strings.Split(c.Query("request_ids"), ",") {
		id := strings.TrimSpace(raw)
		if id != "" && !seen[id] {
			ids = append(ids, id)
			seen[id] = true
		}
	}
	if len(ids) < 1 || len(ids) > 20 {
		response.BadRequest(c, "request_ids must contain between 1 and 20 unique IDs")
		return
	}
	if h.costs == nil {
		response.Error(c, http.StatusServiceUnavailable, "Creation costs unavailable")
		return
	}
	costs, err := h.costs.GetCosts(c.Request.Context(), subject.UserID, ids)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	response.Success(c, costs)
}

func (h *CreationHandler) GetCatalog(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	if !h.settings.IsCreationEnabled(c.Request.Context()) {
		response.ErrorWithDetails(c, http.StatusNotFound, "Creation is not enabled", "CREATION_DISABLED", nil)
		return
	}
	catalog, err := h.catalog.GetCatalog(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, catalog)
}
