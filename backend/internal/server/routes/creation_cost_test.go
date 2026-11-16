//go:build unit

package routes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type creationCostRouteStub struct {
	userID int64
	ids    []string
}

func (s *creationCostRouteStub) GetCosts(_ context.Context, userID int64, ids []string) (*service.CreationCosts, error) {
	s.userID, s.ids = userID, ids
	return &service.CreationCosts{Items: []service.CreationCost{{RequestID: ids[0], ActualCost: .04}}}, nil
}

func TestCreationCostsUserRouteAuthenticationAndContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, authenticated := range []bool{false, true} {
		t.Run(map[bool]string{true: "authenticated", false: "unauthenticated"}[authenticated], func(t *testing.T) {
			stub := &creationCostRouteStub{}
			settings := newCreationCatalogRouteSettings(map[string]string{})
			handlers := &handler.Handlers{Creation: handler.NewCreationHandler(nil, settings, stub)}
			router := gin.New()
			RegisterUserRoutes(router.Group("/api/v1"), handlers, creationCatalogRouteJWT,
				middleware.AuditLogMiddleware(func(c *gin.Context) { c.Next() }), settings, nil)
			request := httptest.NewRequest(http.MethodGet, "/api/v1/user/creation/costs?request_ids=image-uuid&user_id=999", nil)
			if authenticated {
				request.Header.Set("Authorization", "Bearer creation-test-token")
			}
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)
			if authenticated {
				require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
				require.Equal(t, int64(42), stub.userID)
				require.Equal(t, []string{"image-uuid"}, stub.ids)
				require.JSONEq(t, `{"code":0,"message":"success","data":{"items":[{"request_id":"image-uuid","actual_cost":0.04}]}}`, recorder.Body.String())
			} else {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
				require.Nil(t, stub.ids)
			}
		})
	}
}
