package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestCORSRewardSDKReflectsOriginWithoutCredentials(t *testing.T) {
	for _, method := range []string{http.MethodPost, http.MethodOptions} {
		t.Run(method, func(t *testing.T) {
			router := gin.New()
			router.Use(CORS(config.CORSConfig{AllowedOrigins: []string{"https://panel.example"}, AllowCredentials: true}))
			router.POST("/api/v1/reward-open/sdk/heartbeat", func(c *gin.Context) { c.Status(http.StatusOK) })
			request := httptest.NewRequest(method, "/api/v1/reward-open/sdk/heartbeat", nil)
			request.Header.Set("Origin", "https://partner.example")
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)
			if method == http.MethodOptions {
				require.Equal(t, http.StatusNoContent, recorder.Code)
			} else {
				require.Equal(t, http.StatusOK, recorder.Code)
			}
			require.Equal(t, "https://partner.example", recorder.Header().Get("Access-Control-Allow-Origin"))
			require.Contains(t, recorder.Header().Values("Vary"), "Origin")
			require.Equal(t, "POST, OPTIONS", recorder.Header().Get("Access-Control-Allow-Methods"))
			require.Equal(t, "Content-Type", recorder.Header().Get("Access-Control-Allow-Headers"))
			require.Empty(t, recorder.Header().Get("Access-Control-Allow-Credentials"))
		})
	}
}

func TestCORSRewardSDKExceptionDoesNotApplyToSignedEndpoints(t *testing.T) {
	for _, path := range []string{"/api/v1/reward-open/events", "/api/v1/reward-open/sdk-other/heartbeat"} {
		request := httptest.NewRequest(http.MethodOptions, path, nil)
		request.Header.Set("Origin", "https://partner.example")
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		ctx.Request = request
		CORS(config.CORSConfig{AllowedOrigins: []string{"https://panel.example"}})(ctx)
		require.Equal(t, http.StatusForbidden, recorder.Code)
		require.Empty(t, recorder.Header().Get("Access-Control-Allow-Origin"))
	}
}
