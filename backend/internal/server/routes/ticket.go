package routes

import (
	"math"
	"strconv"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	ratemiddleware "github.com/Wei-Shaw/sub2api/internal/middleware"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	ippkg "github.com/Wei-Shaw/sub2api/internal/pkg/ip"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

func registerUserTicketRoutes(user *gin.RouterGroup, h *handler.TicketHandler) {
	if h == nil {
		return
	}
	user.GET("/ticket-config", h.Config)
	enabled := user.Group("")
	enabled.Use(h.Enabled)
	enabled.GET("/tickets", h.List)
	enabled.GET("/tickets/unread-count", h.Unread)
	enabled.POST("/tickets", h.Create)
	enabled.GET("/tickets/:id", h.Get)
	enabled.POST("/tickets/:id/messages", h.Message)
	enabled.POST("/tickets/:id/close", h.Close)
	enabled.POST("/tickets/:id/reopen", h.Reopen)
	enabled.GET("/ticket-notify-preferences", h.Preferences)
	enabled.PUT("/ticket-notify-preferences", h.UpdatePreferences)
	enabled.POST("/ticket-notify-preferences/test", h.TestPreference)
	enabled.GET("/ticket-agent-keys", h.Keys)
	enabled.POST("/ticket-agent-keys", h.CreateKey)
	enabled.PATCH("/ticket-agent-keys/:id", h.UpdateKey)
	enabled.DELETE("/ticket-agent-keys/:id", h.DeleteKey)
}
func registerAdminTicketRoutes(admin *gin.RouterGroup, h *handler.TicketHandler) {
	if h == nil {
		return
	}
	admin.GET("/tickets", h.AdminList)
	admin.GET("/tickets/stats", h.Stats)
	admin.GET("/tickets/:id", h.AdminGet)
	admin.POST("/tickets/:id/messages", h.AdminMessage)
	admin.PATCH("/tickets/:id", h.AdminUpdate)
	admin.GET("/ticket-settings", h.Settings)
	admin.PUT("/ticket-settings", h.UpdateSettings)
	admin.POST("/ticket-settings/test-notification", h.TestAdminNotification)
	admin.GET("/ticket-agent-keys", h.AdminKeys)
	admin.PATCH("/ticket-agent-keys/:id", h.AdminUpdateKey)
	admin.DELETE("/ticket-agent-keys/:id", h.AdminDeleteKey)
	admin.GET("/ticket-assignees", h.Assignees)
}
func RegisterTicketAgentRoutes(v1 *gin.RouterGroup, h *handler.TicketHandler, r *redis.Client) {
	if h == nil {
		return
	}
	agent := v1.Group("/agent")
	limiter := ratemiddleware.NewRateLimiter(r)
	publicLimit := func(c *gin.Context) {
		ip := ippkg.GetSecurityClientIP(c, false)
		if ip == "" {
			ip = c.ClientIP()
		}
		c.Header("X-RateLimit-Limit", "60")
		c.Header("X-RateLimit-Remaining", "0")
		if r == nil {
			response.ErrorFrom(c, infraerrors.New(503, "AGENT_RATE_LIMIT_UNAVAILABLE", "rate limiter unavailable"))
			c.Abort()
			return
		}
		result, err := limiter.Allow(c.Request.Context(), "ticket-agent-docs:"+ip, 60, time.Minute)
		if err != nil {
			response.ErrorFrom(c, infraerrors.New(503, "AGENT_RATE_LIMIT_UNAVAILABLE", "rate limiter unavailable"))
			c.Abort()
			return
		}
		c.Header("X-RateLimit-Remaining", strconv.Itoa(max(0, 60-int(result.Count))))
		if !result.Allowed {
			c.Header("Retry-After", strconv.Itoa(max(1, int(math.Ceil(result.RetryAfter.Seconds())))))
			response.ErrorFrom(c, infraerrors.New(429, "AGENT_RATE_LIMITED", "public documentation rate limited"))
			c.Abort()
			return
		}
		c.Next()
	}
	agent.GET("/docs.md", publicLimit, handler.TicketAgentDocs)
	agent.GET("/openapi.json", publicLimit, handler.TicketAgentOpenAPI)
	protected := agent.Group("")
	var agentLimiter *ratemiddleware.RateLimiter
	if r != nil {
		agentLimiter = limiter
	}
	protected.Use(h.AgentIPRateLimit(agentLimiter), h.AgentAuth)
	protected.GET("/me", h.AgentMe)
	protected.GET("/tickets", h.AgentList)
	protected.POST("/tickets", h.AgentCreate)
	protected.GET("/tickets/:id", h.AgentGet)
	protected.POST("/tickets/:id/messages", h.AgentMessage)
	protected.POST("/tickets/:id/close", h.AgentClose)
	protected.POST("/tickets/:id/reopen", h.AgentReopen)
	protected.PATCH("/tickets/:id", h.AgentUpdate)
	protected.GET("/updates", h.AgentUpdates)
}
