package routes

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type creationCatalogRouteServiceStub struct {
	catalog *service.CreationCatalog
	err     error
	userIDs []int64
	ctx     context.Context
}

func (stub *creationCatalogRouteServiceStub) GetCatalog(ctx context.Context, userID int64) (*service.CreationCatalog, error) {
	stub.userIDs = append(stub.userIDs, userID)
	stub.ctx = ctx
	return stub.catalog, stub.err
}

func newCreationCatalogRouteSettings(values map[string]string) *service.SettingService {
	return service.NewSettingService(&channelMonitorRouteSettingRepoStub{values: values}, &config.Config{})
}

func creationCatalogRouteJWT(c *gin.Context) {
	if c.GetHeader("Authorization") != "Bearer creation-test-token" {
		middleware.AbortWithError(c, http.StatusUnauthorized, "UNAUTHORIZED", "Authorization required")
		return
	}
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42})
	c.Set(string(middleware.ContextKeyUserRole), service.RoleUser)
	c.Next()
}

func newCreationCatalogRouteRouter(catalog handler.CreationCatalogService, settings *service.SettingService, jwtAuth middleware.JWTAuthMiddleware) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handlers := &handler.Handlers{Creation: handler.NewCreationHandler(catalog, settings)}
	auditLog := middleware.AuditLogMiddleware(func(c *gin.Context) { c.Next() })
	RegisterUserRoutes(router.Group("/api/v1"), handlers, jwtAuth, auditLog, settings, nil)
	return router
}

func newCreationCatalogRouteRequest(ctx context.Context) *http.Request {
	request := httptest.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/user/creation/catalog", nil)
	request.Header.Set("Authorization", "Bearer creation-test-token")
	return request
}

func TestCreationCatalogRequiresAuthentication(t *testing.T) {
	tests := []struct {
		name         string
		enabled      string
		withoutToken bool
		jwtAuth      middleware.JWTAuthMiddleware
	}{
		{
			name:         "missing bearer token",
			enabled:      "true",
			withoutToken: true,
			jwtAuth:      creationCatalogRouteJWT,
		},
		{
			name:         "missing bearer token while disabled",
			enabled:      "false",
			withoutToken: true,
			jwtAuth:      creationCatalogRouteJWT,
		},
		{
			name:    "missing auth subject while disabled",
			enabled: "false",
			jwtAuth: func(c *gin.Context) {
				c.Set(string(middleware.ContextKeyUserRole), service.RoleUser)
				c.Next()
			},
		},
		{
			name:    "wrong auth subject type",
			enabled: "true",
			jwtAuth: func(c *gin.Context) {
				c.Set(string(middleware.ContextKeyUser), &service.User{ID: 42})
				c.Set(string(middleware.ContextKeyUserRole), service.RoleUser)
				c.Next()
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			catalog := &creationCatalogRouteServiceStub{}
			settings := newCreationCatalogRouteSettings(map[string]string{service.SettingKeyCreationEnabled: test.enabled})
			router := newCreationCatalogRouteRouter(catalog, settings, test.jwtAuth)
			request := newCreationCatalogRouteRequest(context.Background())
			if test.withoutToken {
				request.Header.Del("Authorization")
			}
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)

			require.Equal(t, http.StatusUnauthorized, recorder.Code, recorder.Body.String())
			require.Empty(t, catalog.userIDs)
		})
	}
}

func TestCreationCatalogDisabledDoesNotCallService(t *testing.T) {
	catalog := &creationCatalogRouteServiceStub{err: errors.New("catalog must not be requested while disabled")}
	settings := newCreationCatalogRouteSettings(map[string]string{service.SettingKeyCreationEnabled: "false"})
	router := newCreationCatalogRouteRouter(catalog, settings, creationCatalogRouteJWT)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, newCreationCatalogRouteRequest(context.Background()))

	require.Equal(t, http.StatusNotFound, recorder.Code, recorder.Body.String())
	require.Empty(t, catalog.userIDs)
	var body struct {
		Code    int             `json:"code"`
		Message string          `json:"message"`
		Reason  string          `json:"reason"`
		Data    json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.Equal(t, http.StatusNotFound, body.Code)
	require.Equal(t, "CREATION_DISABLED", body.Reason)
	require.NotEmpty(t, body.Message)
	require.Empty(t, body.Data)
}

func TestCreationCatalogAllowsOrdinaryUsers(t *testing.T) {
	tests := []struct {
		name     string
		settings *service.SettingService
	}{
		{
			name:     "explicitly enabled",
			settings: newCreationCatalogRouteSettings(map[string]string{service.SettingKeyCreationEnabled: "true"}),
		},
		{
			name:     "setting absent defaults to enabled",
			settings: newCreationCatalogRouteSettings(map[string]string{}),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			catalog := &creationCatalogRouteServiceStub{catalog: &service.CreationCatalog{Groups: []service.CreationGroup{}}}
			router := newCreationCatalogRouteRouter(catalog, test.settings, creationCatalogRouteJWT)
			requestContext, cancel := context.WithCancel(context.Background())
			defer cancel()
			request := newCreationCatalogRouteRequest(requestContext)
			request.URL.RawQuery = "user_id=999"
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)

			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
			require.Equal(t, []int64{42}, catalog.userIDs)
			require.Same(t, requestContext, catalog.ctx)
			require.JSONEq(t, `{"code":0,"message":"success","data":{"groups":[]}}`, recorder.Body.String())
		})
	}
}

func TestCreationCatalogResponseContract(t *testing.T) {
	minDuration, maxDuration := 3, 15
	requestPrice := 0.5
	tests := []struct {
		name     string
		group    service.CreationGroup
		wantJSON string
	}{
		{
			name: "image only with empty model list",
			group: service.CreationGroup{
				ID: 11, Name: "Images", Description: "Image generation", Platform: "openai",
				SubscriptionType: "standard", RateMultiplier: 1.25,
				Image: &service.CreationImageCapability{
					Protocols: []string{"openai"}, Models: []service.CreationModel{},
					AsyncSupported: false, EditSupported: true,
				},
			},
			wantJSON: `{
				"id":11,"name":"Images","description":"Image generation","platform":"openai",
				"subscription_type":"standard","rate_multiplier":1.25,
				"image":{"protocols":["openai"],"models":[],"async_supported":false,"edit_supported":true},
				"video":null
			}`,
		},
		{
			name: "video only with unset capabilities",
			group: service.CreationGroup{
				ID: 12, Name: "Videos", Description: "", Platform: "gemini",
				SubscriptionType: "subscription", RateMultiplier: 0.8,
				Video: &service.CreationVideoCapability{
					Protocols: []string{"gemini"}, Models: []service.CreationModel{}, Resolutions: []string{},
				},
			},
			wantJSON: `{
				"id":12,"name":"Videos","description":"","platform":"gemini",
				"subscription_type":"subscription","rate_multiplier":0.8,"image":null,
				"video":{"protocols":["gemini"],"models":[],"resolutions":[],"min_duration":null,"max_duration":null}
			}`,
		},
		{
			name: "image and video with model protocols and duration bounds",
			group: service.CreationGroup{
				ID: 13, Name: "Media", Description: "Image and video generation", Platform: "grok",
				SubscriptionType: "standard", RateMultiplier: 2,
				Image: &service.CreationImageCapability{
					Protocols: []string{"openai"},
					Models: []service.CreationModel{
						{ID: "grok-imagine-image", Protocols: []string{}, Pricing: &service.CreationModelPricing{
							Unit: "image", Tiers: []service.CreationPriceTier{{Label: "1K", Price: 0.04}, {Label: "2K", Price: 0.08}},
						}},
					},
					AsyncSupported: true, EditSupported: true,
				},
				Video: &service.CreationVideoCapability{
					Protocols: []string{"grok"},
					Models: []service.CreationModel{
						{ID: "grok-imagine-video", Protocols: []string{"grok"}, Pricing: &service.CreationModelPricing{
							Unit: "request", Price: &requestPrice, Tiers: []service.CreationPriceTier{},
						}},
						{ID: "grok-imagine-video-1.5", Protocols: []string{"grok"}},
					},
					Resolutions: []string{"720p", "1080p"}, MinDuration: &minDuration, MaxDuration: &maxDuration,
				},
			},
			wantJSON: `{
				"id":13,"name":"Media","description":"Image and video generation","platform":"grok",
				"subscription_type":"standard","rate_multiplier":2,
				"image":{"protocols":["openai"],"models":[{"id":"grok-imagine-image","protocols":[],
					"pricing":{"unit":"image","tiers":[{"label":"1K","price":0.04},{"label":"2K","price":0.08}]}}],
					"async_supported":true,"edit_supported":true},
				"video":{"protocols":["grok"],"models":[
					{"id":"grok-imagine-video","protocols":["grok"],"pricing":{"unit":"request","price":0.5,"tiers":[]}},
					{"id":"grok-imagine-video-1.5","protocols":["grok"],"pricing":null}],
					"resolutions":["720p","1080p"],"min_duration":3,"max_duration":15}
			}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			catalog := &creationCatalogRouteServiceStub{catalog: &service.CreationCatalog{Groups: []service.CreationGroup{test.group}}}
			settings := newCreationCatalogRouteSettings(map[string]string{service.SettingKeyCreationEnabled: "true"})
			router := newCreationCatalogRouteRouter(catalog, settings, creationCatalogRouteJWT)
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, newCreationCatalogRouteRequest(context.Background()))

			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
			require.Contains(t, recorder.Header().Get("Content-Type"), "application/json")
			require.Equal(t, []int64{42}, catalog.userIDs)
			require.JSONEq(t, `{"code":0,"message":"success","data":{"groups":[`+test.wantJSON+`]}}`, recorder.Body.String())
		})
	}
}

func TestCreationCatalogServiceError(t *testing.T) {
	catalog := &creationCatalogRouteServiceStub{
		catalog: &service.CreationCatalog{Groups: []service.CreationGroup{{ID: 999, Name: "partial catalog must not be exposed"}}},
		err:     errors.New("private catalog error: account_id=123 token=secret upstream=https://internal.example"),
	}
	settings := newCreationCatalogRouteSettings(map[string]string{service.SettingKeyCreationEnabled: "true"})
	router := newCreationCatalogRouteRouter(catalog, settings, creationCatalogRouteJWT)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, newCreationCatalogRouteRequest(context.Background()))

	require.Equal(t, http.StatusInternalServerError, recorder.Code, recorder.Body.String())
	require.Equal(t, []int64{42}, catalog.userIDs)
	var body struct {
		Code    int             `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.Equal(t, http.StatusInternalServerError, body.Code)
	require.NotEmpty(t, body.Message)
	require.Empty(t, body.Data)
	require.NotContains(t, recorder.Body.String(), "partial catalog")
	require.NotContains(t, recorder.Body.String(), "private catalog error")
	require.NotContains(t, recorder.Body.String(), "account_id")
	require.NotContains(t, recorder.Body.String(), "secret")
	require.NotContains(t, recorder.Body.String(), "internal.example")
}
