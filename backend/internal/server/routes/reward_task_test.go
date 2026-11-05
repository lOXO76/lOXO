package routes

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestRewardOpenRoutesRateLimitsAndSharedServerBucket(t *testing.T) {
	gin.SetMode(gin.TestMode)
	server := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: server.Addr()})
	defer func() { _ = redisClient.Close() }()
	router := gin.New()
	// 无效参数在业务 service 前失败，用真实路由验证两个限流桶。
	h := &handler.Handlers{RewardTaskOpen: handler.NewRewardTaskOpenHandler(nil)}
	RegisterRewardOpenRoutes(router.Group("/api/v1"), h, redisClient)
	for _, route := range router.Routes() {
		require.Equal(t, http.MethodPost, route.Method)
	}
	send := func(path, ip string) int {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`))
		req.RemoteAddr = ip + ":12345"
		// 无效请求体在业务service前就返回400；限流触发时应返回429。
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, req)
		return recorder.Code
	}
	for i := 0; i < 600; i++ {
		require.Equal(t, http.StatusBadRequest, send("/api/v1/reward-open/sdk/heartbeat", "203.0.113.10"))
	}
	require.Equal(t, http.StatusTooManyRequests, send("/api/v1/reward-open/sdk/heartbeat", "203.0.113.10"))
	require.Equal(t, http.StatusBadRequest, send("/api/v1/reward-open/sdk/heartbeat", "203.0.113.11"))
	// 三个签名接口共享每 IP 600 次窗口；缺签名在访问 service 依赖前返回 401。
	paths := []string{"/api/v1/reward-open/ping", "/api/v1/reward-open/tickets/query", "/api/v1/reward-open/events"}
	for i := 0; i < 600; i++ {
		require.Equal(t, http.StatusUnauthorized, send(paths[i%len(paths)], "203.0.113.12"))
	}
	for _, path := range paths {
		require.Equal(t, http.StatusTooManyRequests, send(path, "203.0.113.12"))
	}
	server.FastForward(time.Minute)
	require.Equal(t, http.StatusBadRequest, send("/api/v1/reward-open/sdk/heartbeat", "203.0.113.10"))
}
