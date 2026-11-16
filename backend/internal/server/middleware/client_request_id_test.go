package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestClientRequestIDGeneratesAndExposesID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(ClientRequestID())
	router.GET("/", func(c *gin.Context) {
		value, _ := c.Request.Context().Value(ctxkey.ClientRequestID).(string)
		c.String(http.StatusOK, value)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.NotEmpty(t, w.Body.String())
	require.Equal(t, w.Body.String(), w.Header().Get(clientRequestIDHeader))
}

func TestClientRequestIDBoundsExistingContextID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(ClientRequestID())
	router.GET("/", func(c *gin.Context) {
		value, _ := c.Request.Context().Value(ctxkey.ClientRequestID).(string)
		c.String(http.StatusOK, value)
	})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(context.WithValue(req.Context(), ctxkey.ClientRequestID, strings.Repeat("x", 200)))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Len(t, w.Body.String(), 36)
	require.NotEqual(t, strings.Repeat("x", maxPersistentRequestIDBytes), w.Body.String())
	require.Equal(t, w.Body.String(), w.Header().Get(clientRequestIDHeader))
}

func TestClientRequestIDPreservesExistingContextID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(ClientRequestID())
	router.GET("/", func(c *gin.Context) {
		value, _ := c.Request.Context().Value(ctxkey.ClientRequestID).(string)
		c.String(http.StatusOK, value)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(context.WithValue(req.Context(), ctxkey.ClientRequestID, "existing-client-request-id"))
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "existing-client-request-id", w.Body.String())
	require.Equal(t, "existing-client-request-id", w.Header().Get(clientRequestIDHeader))
}

func TestClientRequestIDIsReadableFromAllowedBrowserOrigin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(CORS(config.CORSConfig{AllowedOrigins: []string{"https://panel.example"}}))
	router.Use(ClientRequestID())
	router.POST("/v1/images/generations", func(c *gin.Context) {
		id, _ := c.Request.Context().Value(ctxkey.ClientRequestID).(string)
		c.JSON(http.StatusOK, gin.H{"request_id": id})
	})
	request := httptest.NewRequest(http.MethodPost, "/v1/images/generations", nil)
	request.Header.Set("Origin", "https://panel.example")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "https://panel.example", recorder.Header().Get("Access-Control-Allow-Origin"))
	require.NotEmpty(t, recorder.Header().Get(clientRequestIDHeader))
	require.Contains(t, recorder.Header().Get("Access-Control-Expose-Headers"), clientRequestIDHeader)
	require.Contains(t, recorder.Body.String(), recorder.Header().Get(clientRequestIDHeader))
}
