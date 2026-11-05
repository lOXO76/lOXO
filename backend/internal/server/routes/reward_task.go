package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/middleware"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"time"
)

func RegisterRewardOpenRoutes(v1 *gin.RouterGroup, h *handler.Handlers, redisClient *redis.Client) {
	limiter := middleware.NewRateLimiter(redisClient)
	open := v1.Group("/reward-open")
	signed := open.Group("")
	signed.Use(limiter.Limit("reward-open-server", 600, time.Minute))
	signed.POST("/ping", h.RewardTaskOpen.Ping)
	signed.POST("/tickets/query", h.RewardTaskOpen.QueryTicket)
	signed.POST("/events", h.RewardTaskOpen.Event)
	open.POST("/sdk/heartbeat", limiter.Limit("reward-open-sdk", 600, time.Minute), h.RewardTaskOpen.Heartbeat)
}
