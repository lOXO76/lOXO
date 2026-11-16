//go:build unit

package handler

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type creationCostHandlerStub struct {
	ids    []string
	userID int64
	result *service.CreationCosts
	err    error
}

func (s *creationCostHandlerStub) GetCosts(_ context.Context, userID int64, ids []string) (*service.CreationCosts, error) {
	s.userID, s.ids = userID, ids
	return s.result, s.err
}

func TestCreationCostHandlerParametersAndEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	maxIDs := make([]string, 20)
	for i := range maxIDs {
		maxIDs[i] = strconv.Itoa(i)
	}
	for _, test := range []struct {
		name, query string
		status      int
		want        []string
	}{
		{"absent", "", 400, nil},
		{"empty", " , , ", 400, nil},
		{"deduplicates and trims", " , one ,one,two, ", 200, []string{"one", "two"}},
		{"twenty", strings.Join(maxIDs, ","), 200, maxIDs},
		{"twenty one", strings.Join(maxIDs, ",") + ",extra", 400, nil},
		{"duplicates do not count toward limit", strings.Repeat("one,", 30), 200, []string{"one"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			stub := &creationCostHandlerStub{result: &service.CreationCosts{Items: []service.CreationCost{}}}
			h := NewCreationHandler(nil, nil, stub)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodGet, "/?user_id=999&request_ids="+url.QueryEscape(test.query), nil)
			c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42})
			h.GetCosts(c)
			require.Equal(t, test.status, w.Code, w.Body.String())
			require.Equal(t, test.want, stub.ids)
			if test.status == http.StatusOK {
				require.Equal(t, int64(42), stub.userID)
				require.Equal(t, "no-store", w.Header().Get("Cache-Control"))
				require.JSONEq(t, `{"code":0,"message":"success","data":{"items":[]}}`, w.Body.String())
			}
		})
	}
}

func TestCreationCostHandlerRequiresSubject(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/?request_ids=one", nil)
	stub := &creationCostHandlerStub{}
	NewCreationHandler(nil, nil, stub).GetCosts(c)
	require.Equal(t, http.StatusUnauthorized, w.Code)
	require.Nil(t, stub.ids)
}

type creationCostHandlerTaskStore struct {
	asyncImageMemoryStore
	records []service.AsyncTaskRecord
}

func (s *creationCostHandlerTaskStore) SaveAsyncTask(_ context.Context, _ *service.AsyncTaskRecord) error {
	return nil
}

func (s *creationCostHandlerTaskStore) ListAsyncTasks(_ context.Context, _ int64) ([]service.AsyncTaskRecord, error) {
	return s.records, nil
}

func (s *creationCostHandlerTaskStore) GetAsyncTask(_ context.Context, _ int64, id string) (*service.AsyncTaskRecord, error) {
	for _, record := range s.records {
		if record.ID == id {
			return &record, nil
		}
	}
	return nil, service.ErrImageTaskNotFound
}

type creationCostHandlerRepo struct {
	taskCalls int
	err       error
}

func (*creationCostHandlerRepo) SumUserRequestCosts(context.Context, int64, []string) (map[string]float64, error) {
	return nil, nil
}

func (r *creationCostHandlerRepo) SumUserTaskCosts(_ context.Context, userID int64, _ []service.AsyncTaskRecord) (map[string]float64, error) {
	r.taskCalls++
	if r.err != nil {
		return nil, r.err
	}
	if userID == 42 {
		return map[string]float64{"charged": .04}, nil
	}
	return nil, nil
}

func TestAsyncTaskHandlersExposeActualCostsOnListAndRefresh(t *testing.T) {
	gin.SetMode(gin.TestMode)
	record := func(id string, createdAt int64) service.AsyncTaskRecord {
		return service.AsyncTaskRecord{AsyncTask: service.AsyncTask{
			ID: id, TaskID: id, Kind: "image", APIKeyID: 2, Status: "completed", CreatedAt: createdAt, ExpiresAt: time.Now().Add(time.Hour).Unix(),
		}, UserID: 42, BillingRequestID: "client:" + id}
	}
	store := &creationCostHandlerTaskStore{records: []service.AsyncTaskRecord{record("charged", 2), record("missing", 1)}}
	repo := &creationCostHandlerRepo{}
	tasks := service.NewImageTaskService(store, service.NewCreationCostService(repo))
	h := NewAsyncImageHandler(tasks, nil)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42})
		c.Next()
	})
	router.GET("/tasks", h.ListUserTasks)
	router.GET("/tasks/:id", h.GetUserTask)
	router.POST("/tasks/:id/refresh", h.RefreshUserTask)
	for _, test := range []struct{ method, path, want string }{
		{http.MethodGet, "/tasks", `"actual_cost":0.04`},
		{http.MethodGet, "/tasks/charged", `"actual_cost":0.04`},
		{http.MethodPost, "/tasks/charged/refresh", `"actual_cost":0.04`},
		{http.MethodPost, "/tasks/missing/refresh", `"id":"missing"`},
	} {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(test.method, test.path, nil))
		require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
		require.Contains(t, recorder.Body.String(), test.want)
		if strings.Contains(test.path, "missing") {
			require.NotContains(t, recorder.Body.String(), "actual_cost")
		}
	}
	require.Equal(t, 4, repo.taskCalls, "each JSON response performs one optional cost query")
}

func TestAsyncTaskHandlersCostQueryFailureStillReturnsJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &creationCostHandlerTaskStore{records: []service.AsyncTaskRecord{{
		AsyncTask: service.AsyncTask{ID: "charged", Kind: "image", Status: "completed", ExpiresAt: time.Now().Add(time.Hour).Unix()},
		UserID:    42,
	}}}
	repo := &creationCostHandlerRepo{err: errors.New("usage_logs unavailable")}
	h := NewAsyncImageHandler(service.NewImageTaskService(store, service.NewCreationCostService(repo)), nil)
	for _, action := range []func(*gin.Context){h.ListUserTasks, h.GetUserTask, h.RefreshUserTask} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/user/async-tasks/charged", nil)
		c.Params = gin.Params{{Key: "id", Value: "charged"}}
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42})
		action(c)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		require.Contains(t, w.Body.String(), `"id":"charged"`)
		require.NotContains(t, w.Body.String(), "actual_cost")
		require.NotContains(t, w.Body.String(), "usage_logs unavailable")
	}
	require.Equal(t, 3, repo.taskCalls)
}

func TestAsyncTaskContentDownloadSkipsCostQueries(t *testing.T) {
	openAI, slots, _, upstream := newGrokMediaSlotHandler(t, false, false,
		service.PlatformVideo, service.VideoProtocolMiniMax, "https://metaso.cn/api/minimax/v2", "true")
	c, _ := grokMediaSlotContext(context.Background(), false)
	key, ok := middleware.GetAPIKeyFromContext(c)
	require.True(t, ok)
	openAI.apiKeyService = service.NewAPIKeyService(&dailyUsageAPIKeyRepoStub{keys: map[int64]*service.APIKey{key.ID: key}}, nil, nil, nil, nil, nil, nil)
	requestID := "minimax:download-task"
	require.NoError(t, openAI.gatewayService.BindGrokMediaVideoRequestAccount(context.Background(), key.GroupID, requestID, key.UserID, key.ID, 1))
	store := &creationCostHandlerTaskStore{records: []service.AsyncTaskRecord{{
		AsyncTask: service.AsyncTask{ID: "download", Kind: "video", Protocol: service.VideoProtocolMiniMax,
			Status: "completed", APIKeyID: key.ID, GroupID: key.GroupID, ExpiresAt: time.Now().Add(time.Hour).Unix()},
		UserID: key.UserID, RequestID: requestID,
	}}}
	repo := &creationCostHandlerRepo{err: errors.New("cost queries must not run for content downloads")}
	h := NewAsyncImageHandler(service.NewImageTaskService(store, service.NewCreationCostService(repo)), openAI)
	upstream.call = func(req *http.Request, _ int64) (*http.Response, error) {
		if req.URL.Host == "files.metaso.cn" {
			return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"video/mp4"}},
				Body: io.NopCloser(strings.NewReader("video-content")), ContentLength: 13}, nil
		}
		require.Equal(t, "/api/minimax/v2/query/video_generation/download-task", req.URL.Path)
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/json"}},
			Body: io.NopCloser(strings.NewReader(`{"task":{"id":"download-task","status":"succeeded","content":{"url":"https://files.metaso.cn/video.mp4"}}}`))}, nil
	}
	for _, action := range []func(*gin.Context){h.DownloadUserTask, h.DownloadAdminTask} {
		c, w := grokMediaSlotContext(context.Background(), false)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/user/async-tasks/download/content", nil)
		c.Params = gin.Params{{Key: "id", Value: "download"}, {Key: "user_id", Value: strconv.FormatInt(key.UserID, 10)}}
		action(c)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		require.Equal(t, "video-content", w.Body.String())
		require.Zero(t, repo.taskCalls)
	}
	require.Equal(t, 4, upstream.calls, "both downloads must reach status lookup and content streaming")
	slots.assertReleased(t)
}

func TestAdminTaskReadAndRefreshSkipCostQueries(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &creationCostHandlerTaskStore{records: []service.AsyncTaskRecord{{
		AsyncTask: service.AsyncTask{ID: "charged", Kind: "image", Status: "completed", ExpiresAt: time.Now().Add(time.Hour).Unix()},
		UserID:    42,
	}}}
	repo := &creationCostHandlerRepo{err: errors.New("cost queries must not run for admin task responses")}
	h := NewAsyncImageHandler(service.NewImageTaskService(store, service.NewCreationCostService(repo)), nil)
	for _, action := range []func(*gin.Context){h.GetAdminTask, h.RefreshAdminTask} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/async-tasks/42/charged", nil)
		c.Params = gin.Params{{Key: "id", Value: "charged"}, {Key: "user_id", Value: "42"}}
		action(c)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		require.Contains(t, w.Body.String(), `"id":"charged"`)
		require.NotContains(t, w.Body.String(), "actual_cost")
		require.Zero(t, repo.taskCalls)
	}
}
