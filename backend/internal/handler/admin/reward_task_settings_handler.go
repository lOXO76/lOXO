package admin

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type RewardTaskSettingsHandler struct {
	settingService *service.SettingService
}

func NewRewardTaskSettingsHandler(settingService *service.SettingService) *RewardTaskSettingsHandler {
	return &RewardTaskSettingsHandler{settingService: settingService}
}

func (h *RewardTaskSettingsHandler) GetSettings(c *gin.Context) {
	settings, err := h.settingService.GetRewardTaskSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, settings)
}

func (h *RewardTaskSettingsHandler) UpdateSettings(c *gin.Context) {
	var req struct {
		Enabled *bool `json:"enabled" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "enabled must be a boolean")
		return
	}
	previous, err := h.settingService.GetRewardTaskSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if err := h.settingService.SetRewardTasksEnabled(c.Request.Context(), *req.Enabled); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	(&SettingHandler{}).auditSettingsUpdate(c,
		&service.SystemSettings{RewardTasksEnabled: previous.Enabled},
		&service.SystemSettings{RewardTasksEnabled: *req.Enabled},
		nil, nil, UpdateSettingsRequest{},
	)
	middleware.SetAuditExtra(c, map[string]any{"enabled": *req.Enabled})
	response.Success(c, &service.RewardTaskSettings{Enabled: *req.Enabled})
}
