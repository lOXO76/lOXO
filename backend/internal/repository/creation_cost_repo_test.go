//go:build unit

package repository

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

func newCreationCostTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec(`CREATE TABLE usage_logs (user_id BIGINT, api_key_id BIGINT, request_id TEXT, actual_cost REAL)`)
	require.NoError(t, err)
	_, err = db.Exec(`CREATE TABLE video_billing_reservations (id INTEGER PRIMARY KEY, created_at BIGINT, user_id BIGINT, api_key_id BIGINT, protocol TEXT, task_id TEXT, file_id TEXT)`)
	require.NoError(t, err)
	return db
}

func addCreationUsage(t *testing.T, db *sql.DB, userID, keyID int64, requestID string, cost float64) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO usage_logs VALUES ($1,$2,$3,$4)`, userID, keyID, requestID, cost)
	require.NoError(t, err)
}

func TestCreationCostRepositorySumsRequestsAndIsolatesUsers(t *testing.T) {
	db := newCreationCostTestDB(t)
	addCreationUsage(t, db, 7, 1, "client:image-id", .04)
	addCreationUsage(t, db, 7, 2, "client:image-id", .06)
	addCreationUsage(t, db, 8, 3, "client:image-id", 90)
	addCreationUsage(t, db, 8, 3, "client:foreign", 100)
	addCreationUsage(t, db, 7, 1, "client:zero", 0)
	addCreationUsage(t, db, 7, 1, "image-id", 123)
	repo := NewCreationCostRepository(db)
	costs, err := repo.SumUserRequestCosts(context.Background(), 7, []string{"client:image-id", "client:foreign", "client:zero", "client:missing"})
	require.NoError(t, err)
	require.Len(t, costs, 2)
	require.InDelta(t, .1, costs["client:image-id"], 1e-10)
	require.Equal(t, 0., costs["client:zero"])
	result, err := service.NewCreationCostService(repo).GetCosts(context.Background(), 8, []string{"image-id", "foreign", "zero"})
	require.NoError(t, err)
	require.Equal(t, []service.CreationCost{{RequestID: "image-id", ActualCost: 90}, {RequestID: "foreign", ActualCost: 100}}, result.Items)
}

func TestCreationCostRepositoryBatchesEveryTaskProtocol(t *testing.T) {
	db := newCreationCostTestDB(t)
	repo := NewCreationCostRepository(db)
	task := func(id, requestID, protocol string) service.AsyncTaskRecord {
		return service.AsyncTaskRecord{AsyncTask: service.AsyncTask{ID: id, Kind: "video", APIKeyID: 2, Protocol: protocol}, UserID: 7, RequestID: requestID}
	}
	records := []service.AsyncTaskRecord{
		task("seedance", "seedance:seed-task", service.VideoProtocolSeedance),
		task("minimax-v1", "minimax:mm-v1-task", service.VideoProtocolMiniMax),
		task("minimax-v2", "minimax:mm-v2-task", service.VideoProtocolMiniMax),
		task("gemini", "gemini:models/veo/operations/one", service.VideoProtocolGemini),
		task("grok", "grok-task", "grok"),
		task("file", "minimax-file:file-2", service.VideoProtocolMiniMax),
		task("foreign-file", "minimax-file:file-foreign", service.VideoProtocolMiniMax),
		task("foreign", "grok-task", "grok"),
		task("legacy-image", "imgtask_old", "openai"),
		task("image", "imgtask_new", "openai"),
	}
	records[7].UserID = 8
	records[8].Kind = "image"
	records[9].Kind, records[9].BillingRequestID = "image", "client:image-submit"
	for i := 0; i < 5; i++ {
		billingID := service.StableGrokVideoBillingRequestID(records[i].RequestID)
		addCreationUsage(t, db, 7, 2, billingID, float64(i+1)/10)
		addCreationUsage(t, db, 8, 2, billingID, 100)
		addCreationUsage(t, db, 7, 99, billingID, 200)
	}
	addCreationUsage(t, db, 7, 2, "grok-video:grok-task", .25)
	addCreationUsage(t, db, 7, 2, "client:image-submit", .04)
	addCreationUsage(t, db, 7, 2, "imgtask_old", 300)
	_, err := db.Exec(`INSERT INTO video_billing_reservations VALUES
 (1,1,7,2,'minimax','minimax:mm-v1-task','file-2'),
 (2,2,7,2,'minimax','minimax:mm-v2-task','file-2'),
 (3,3,8,2,'minimax','minimax:mm-v2-task','file-foreign'),
 (4,4,7,99,'minimax','minimax:mm-v2-task','file-foreign'),
 (5,5,7,2,'seedance','minimax:mm-v2-task','file-foreign')`)
	require.NoError(t, err)
	costs, err := repo.SumUserTaskCosts(context.Background(), 7, records)
	require.NoError(t, err)
	require.Len(t, costs, 7)
	require.InDelta(t, .1, costs["seedance"], 1e-10)
	require.InDelta(t, .2, costs["minimax-v1"], 1e-10)
	require.InDelta(t, .3, costs["minimax-v2"], 1e-10)
	require.InDelta(t, .3, costs["file"], 1e-10, "file aliases use the latest matching owned reservation")
	require.InDelta(t, .4, costs["gemini"], 1e-10)
	require.InDelta(t, .75, costs["grok"], 1e-10)
	require.InDelta(t, .04, costs["image"], 1e-10)
	require.NotContains(t, costs, "legacy-image")
	require.NotContains(t, costs, "foreign-file")
	require.NotContains(t, costs, "foreign")
}

func TestImageTaskStoreRetainsBillingCorrelationWithoutPersistingCosts(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	store := NewImageTaskStore(rdb)
	svc := service.NewImageTaskService(store)
	ctx := context.WithValue(context.Background(), ctxkey.ClientRequestID, "submit-uuid")
	image, err := svc.CreateWithMetadata(ctx, service.ImageTaskOwner{UserID: 7, APIKeyID: 2}, "gpt-image", "openai", nil)
	require.NoError(t, err)
	catalog, ok := store.(service.AsyncTaskStore)
	require.True(t, ok)
	task, err := catalog.GetAsyncTask(ctx, 7, image.ID)
	require.NoError(t, err)
	require.Equal(t, "client:submit-uuid", task.BillingRequestID)
	cost := .04
	task.ActualCost = &cost
	task.ExpiresAt = time.Now().Add(time.Hour).Unix()
	require.NoError(t, catalog.SaveAsyncTask(ctx, task))
	raw, err := rdb.Get(ctx, asyncTaskKey(7, image.ID)).Result()
	require.NoError(t, err)
	require.NotContains(t, raw, "actual_cost")
	require.Contains(t, raw, "client:submit-uuid")
	require.Equal(t, &cost, task.ActualCost, "serialization must not mutate the response")
}
