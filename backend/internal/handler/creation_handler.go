package handler

import (
	"context"
	"net/http"

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
}

func NewCreationHandler(catalog CreationCatalogService, settings *service.SettingService) *CreationHandler {
	return &CreationHandler{catalog: catalog, settings: settings}
}

func ProvideCreationHandler(catalog *service.CreationCatalogService, settings *service.SettingService) *CreationHandler {
	return NewCreationHandler(catalog, settings)
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
