//go:build unit

package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

type creationCostRepoStub struct {
	userID   int64
	ids      []string
	requests map[int64]map[string]float64
	tasks    map[string]float64
	pages    [][]AsyncTaskRecord
	err      error
}

func (r *creationCostRepoStub) SumUserRequestCosts(_ context.Context, userID int64, ids []string) (map[string]float64, error) {
	r.userID, r.ids = userID, ids
	return r.requests[userID], r.err
}

func (r *creationCostRepoStub) SumUserTaskCosts(_ context.Context, userID int64, records []AsyncTaskRecord) (map[string]float64, error) {
	r.userID = userID
	r.pages = append(r.pages, append([]AsyncTaskRecord(nil), records...))
	return r.tasks, r.err
}

type creationCostTaskStore struct {
	imageTaskMemoryStore
	records []AsyncTaskRecord
}

func (s *creationCostTaskStore) SaveAsyncTask(_ context.Context, _ *AsyncTaskRecord) error {
	return nil
}
func (s *creationCostTaskStore) ListAsyncTasks(_ context.Context, _ int64) ([]AsyncTaskRecord, error) {
	return s.records, nil
}
func (s *creationCostTaskStore) GetAsyncTask(_ context.Context, _ int64, id string) (*AsyncTaskRecord, error) {
	for _, task := range s.records {
		if task.ID == id {
			return &task, nil
		}
	}
	return nil, ErrImageTaskNotFound
}

func TestCreationCostsMapsClientIDsAndScopesUser(t *testing.T) {
	repo := &creationCostRepoStub{requests: map[int64]map[string]float64{
		7: {"client:openai-image": 0.04, "client:gemini-image": 0},
		8: {"client:foreign": 99},
	}}
	svc := NewCreationCostService(repo)
	got, err := svc.GetCosts(context.Background(), 7, []string{"openai-image", "foreign", "gemini-image", "missing"})
	require.NoError(t, err)
	require.Equal(t, int64(7), repo.userID)
	require.Equal(t, []string{"client:openai-image", "client:foreign", "client:gemini-image", "client:missing"}, repo.ids)
	require.Equal(t, []CreationCost{{RequestID: "openai-image", ActualCost: .04}, {RequestID: "gemini-image", ActualCost: 0}}, got.Items)
	got, err = svc.GetCosts(context.Background(), 9, []string{"missing"})
	require.NoError(t, err)
	data, err := json.Marshal(got)
	require.NoError(t, err)
	require.JSONEq(t, `{"items":[]}`, string(data))
}

func TestAsyncTaskCostsBatchOnlyCurrentPageAndOmitMissing(t *testing.T) {
	now := time.Now()
	record := func(id string, userID, created int64) AsyncTaskRecord {
		return AsyncTaskRecord{AsyncTask: AsyncTask{ID: id, Kind: "video", APIKeyID: 2, CreatedAt: created, ExpiresAt: now.Add(time.Hour).Unix()}, UserID: userID, RequestID: id}
	}
	store := &creationCostTaskStore{records: []AsyncTaskRecord{record("older", 7, 1), record("missing", 7, 2), record("charged", 7, 3), record("foreign", 8, 4)}}
	repo := &creationCostRepoStub{tasks: map[string]float64{"charged": .15, "foreign": 99}}
	svc := NewImageTaskService(store, NewCreationCostService(repo))
	page, err := svc.ListUserTasks(context.Background(), 7, 1, 2, "", "", "")
	require.NoError(t, err)
	require.Equal(t, 3, page.Total)
	require.Len(t, repo.pages, 1)
	require.Len(t, repo.pages[0], 2)
	require.Equal(t, int64(7), repo.userID)
	require.Equal(t, "charged", repo.pages[0][0].ID)
	require.Equal(t, .15, *page.Items[0].ActualCost)
	require.Nil(t, page.Items[1].ActualCost)
	data, err := json.Marshal(page.Items[1])
	require.NoError(t, err)
	require.NotContains(t, string(data), "actual_cost")
	task, err := svc.GetUserTask(context.Background(), 7, "charged")
	require.NoError(t, err)
	require.Nil(t, task.ActualCost)
	require.Len(t, repo.pages, 1, "routing reads must not query costs")
	svc.PopulateUserTaskCost(context.Background(), 7, task)
	require.Equal(t, .15, *task.ActualCost)
	_, err = svc.GetUserTask(context.Background(), 7, "foreign")
	require.ErrorIs(t, err, ErrImageTaskNotFound)
	require.Len(t, repo.pages, 2, "foreign ownership must be rejected before querying cost")
	require.Nil(t, store.records[2].ActualCost, "response enrichment must not mutate stored records")
}

func TestAsyncTaskCostQueryFailureKeepsTasksAvailable(t *testing.T) {
	staleCost := 99.
	store := &creationCostTaskStore{records: []AsyncTaskRecord{{
		AsyncTask: AsyncTask{ID: "charged", Kind: "video", ActualCost: &staleCost, ExpiresAt: time.Now().Add(time.Hour).Unix()},
		UserID:    7, RequestID: "task",
	}}}
	repo := &creationCostRepoStub{tasks: map[string]float64{"charged": .15}, err: errors.New("usage_logs unavailable")}
	svc := NewImageTaskService(store, NewCreationCostService(repo))
	core, logs := observer.New(zap.WarnLevel)
	ctx := logger.IntoContext(context.Background(), zap.New(core))
	page, err := svc.ListUserTasks(ctx, 7, 1, 20, "", "", "")
	require.NoError(t, err)
	require.Equal(t, 1, page.Total)
	require.Len(t, page.Items, 1)
	require.Nil(t, page.Items[0].ActualCost)
	data, err := json.Marshal(page)
	require.NoError(t, err)
	require.NotContains(t, string(data), "actual_cost")
	require.Len(t, repo.pages, 1)
	task, err := svc.GetUserTask(ctx, 7, "charged")
	require.NoError(t, err)
	require.Len(t, repo.pages, 1, "plain GetUserTask must avoid the failing cost repository")
	svc.PopulateUserTaskCost(ctx, 7, task)
	require.Nil(t, task.ActualCost)
	require.Len(t, repo.pages, 2)
	data, err = json.Marshal(task.AsyncTask)
	require.NoError(t, err)
	require.NotContains(t, string(data), "actual_cost")
	warnings := logs.FilterMessage("async_task.cost_query_failed").All()
	require.Len(t, warnings, 2)
	for _, warning := range warnings {
		require.Equal(t, zap.WarnLevel, warning.Level)
		require.Equal(t, int64(7), warning.ContextMap()["user_id"])
		require.Equal(t, int64(1), warning.ContextMap()["task_count"])
		require.Equal(t, "usage_logs unavailable", warning.ContextMap()["error"])
	}
}

func TestAsyncImageRetainsSubmissionBillingRequestID(t *testing.T) {
	store := &imageTaskMemoryStore{}
	svc := NewImageTaskService(store)
	ctx := context.WithValue(context.Background(), ctxkey.ClientRequestID, "image-submit-uuid")
	_, err := svc.CreateWithMetadata(ctx, ImageTaskOwner{UserID: 7, APIKeyID: 2}, "gpt-image-1", "openai", nil)
	require.NoError(t, err)
	require.Equal(t, "client:image-submit-uuid", store.task.BillingRequestID)
	ctx = context.WithoutCancel(ctx)
	require.Equal(t, store.task.BillingRequestID, resolveUsageBillingRequestID(ctx, "upstream-image-request"))
}

func TestAsyncTaskCostsPreserveRecordedZeroAndRejectForeignEnrichment(t *testing.T) {
	repo := &creationCostRepoStub{tasks: map[string]float64{"zero": 0, "foreign": 99}}
	records := []AsyncTaskRecord{
		{AsyncTask: AsyncTask{ID: "zero"}, UserID: 7},
		{AsyncTask: AsyncTask{ID: "foreign"}, UserID: 8},
	}
	require.NoError(t, NewCreationCostService(repo).populateTasks(context.Background(), 7, records))
	require.NotNil(t, records[0].ActualCost)
	require.Zero(t, *records[0].ActualCost)
	require.Nil(t, records[1].ActualCost)
	data, err := json.Marshal(records[0].AsyncTask)
	require.NoError(t, err)
	require.Contains(t, string(data), `"actual_cost":0`)
}

func TestSynchronousImageBillingUsesClientRequestIDForBothGateways(t *testing.T) {
	for _, protocol := range []string{"openai", "gemini"} {
		t.Run(protocol, func(t *testing.T) {
			usageRepo := &openAIRecordUsageLogRepoStub{inserted: true}
			userRepo := &openAIRecordUsageUserRepoStub{}
			subRepo := &openAIRecordUsageSubRepoStub{}
			price, groupID := .04, int64(9)
			key := &APIKey{ID: 2, UserID: 7, GroupID: &groupID,
				Group: &Group{ID: groupID, RateMultiplier: 1, ImagePrice2K: &price}}
			user, account := &User{ID: 7}, &Account{ID: 3}
			ctx := context.WithValue(context.Background(), ctxkey.ClientRequestID, protocol+"-image-uuid")
			var err error
			if protocol == "openai" {
				svc := newOpenAIRecordUsageServiceForTest(usageRepo, userRepo, subRepo, nil)
				err = svc.RecordUsage(ctx, &OpenAIRecordUsageInput{
					Result: &OpenAIForwardResult{RequestID: "upstream-openai-image", Model: "gpt-image-1", ImageCount: 1, ImageSize: ImageBillingSize2K},
					APIKey: key, User: user, Account: account,
				})
			} else {
				svc := newGatewayRecordUsageServiceForTest(usageRepo, userRepo, subRepo)
				err = svc.RecordUsage(ctx, &RecordUsageInput{
					Result: &ForwardResult{RequestID: "upstream-gemini-image", Model: "gemini-image", ImageCount: 1, ImageSize: ImageBillingSize2K},
					APIKey: key, User: user, Account: account,
				})
			}
			require.NoError(t, err)
			require.NotNil(t, usageRepo.lastLog)
			require.Equal(t, "client:"+protocol+"-image-uuid", usageRepo.lastLog.RequestID)
			require.InDelta(t, price, usageRepo.lastLog.ActualCost, 1e-10)
		})
	}
}
