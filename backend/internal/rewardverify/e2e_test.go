//go:build reward_verify

package rewardverify

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/rewardtaskrecord"
	_ "github.com/Wei-Shaw/sub2api/ent/runtime"
	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/Wei-Shaw/sub2api/internal/service"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

// 只在显式指定临时库时执行：REWARD_VERIFY_DSN=... go test -tags=reward_verify ./internal/rewardverify
func TestRewardTasksAgainstPostgres(t *testing.T) {
	dsn := os.Getenv("REWARD_VERIFY_DSN")
	if dsn == "" {
		t.Skip("set REWARD_VERIFY_DSN to an isolated PostgreSQL database")
	}
	ctx := context.Background()
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	require.NoError(t, repository.ApplyMigrations(ctx, db))
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	defer func() { _ = client.Close() }()
	settings := service.NewSettingService(repository.NewSettingRepository(client), &config.Config{})
	require.NoError(t, settings.SetRewardTasksEnabled(ctx, true))
	cfg := &config.Config{}
	cfg.Totp.EncryptionKey = strings.Repeat("ab", 32)
	encryptor, err := repository.NewAESEncryptor(cfg)
	require.NoError(t, err)
	svc := service.NewRewardTaskService(client, repository.NewUserRepository(client, db), settings, nil, nil, nil, encryptor, nil)
	suffix := fmt.Sprint(time.Now().UnixNano())
	user, err := client.User.Create().SetEmail("reward-" + suffix + "@example.com").SetPasswordHash("x").Save(ctx)
	require.NoError(t, err)
	second, err := client.User.Create().SetEmail("reward-second-" + suffix + "@example.com").SetPasswordHash("x").Save(ctx)
	require.NoError(t, err)
	partner, err := svc.CreateAdminPartner(ctx, service.RewardTaskPartnerInput{Name: "Verify " + suffix, Enabled: true, AllowedDomains: []string{"example.com"}})
	require.NoError(t, err)
	stored, err := client.RewardTaskPartner.Get(ctx, partner.ID)
	require.NoError(t, err)
	require.NotEqual(t, partner.AppSecret, stored.AppSecretEncrypted)
	plain, err := encryptor.Decrypt(stored.AppSecretEncrypted)
	require.NoError(t, err)
	require.Equal(t, partner.AppSecret, plain)
	sign := func(body []byte) {
		headers := service.RewardOpenSignature{AppKey: partner.AppKey, Timestamp: fmt.Sprint(time.Now().Unix()), Nonce: "nonce_" + fmt.Sprint(time.Now().UnixNano())}
		mac := hmac.New(sha256.New, []byte(partner.AppSecret))
		_, _ = mac.Write([]byte(headers.Timestamp + "\n" + headers.Nonce + "\n"))
		_, _ = mac.Write(body)
		headers.Signature = hex.EncodeToString(mac.Sum(nil))
		authenticated, err := svc.AuthenticateRewardOpen(ctx, headers, body)
		require.NoError(t, err)
		require.Equal(t, partner.ID, authenticated.ID)
	}
	taskInput := func(taskType, mode string, reward float64) service.RewardTaskInput {
		in := service.RewardTaskInput{Title: "Verify " + taskType + suffix, TaskType: taskType, VerifyMode: mode, PartnerID: &partner.ID, TargetURL: "https://example.com/?source=test#section", RewardAmount: reward, IdleTimeoutSeconds: 15, Period: "once", Enabled: true}
		if taskType == "browse" {
			in.BrowseSeconds = 5
		}
		if taskType == "recharge" {
			in.MinRechargeAmount = 100
		}
		if mode == "timer" {
			in.PartnerID = nil
		}
		return in
	}
	start := func(taskID, userID int64) string {
		result, err := svc.StartTask(ctx, userID, taskID)
		require.NoError(t, err)
		record, err := client.RewardTaskRecord.Query().Where(rewardtaskrecord.TaskID(taskID), rewardtaskrecord.UserID(userID)).Only(ctx)
		require.NoError(t, err)
		if strings.Contains(result.JumpURL, "reward_ticket") {
			require.Contains(t, result.JumpURL, record.Ticket)
		}
		return record.Ticket
	}
	eventBody := func(id, kind, ticket, external, amount string) []byte {
		body, err := json.Marshal(map[string]any{"event_id": id, "event_type": kind, "ticket": ticket, "external_user_id": external, "amount": amount, "occurred_at": time.Now().Unix()})
		require.NoError(t, err)
		return body
	}
	t.Run("register callback and concurrent event idempotency", func(t *testing.T) {
		task, err := svc.CreateAdminTask(ctx, taskInput("register", "callback", 0.5))
		require.NoError(t, err)
		ticket := start(task.ID, user.ID)
		body := eventBody("register-"+suffix, "register", ticket, "external-a", "")
		sign(body)
		errs := make(chan error, 8)
		var wg sync.WaitGroup
		for i := 0; i < 8; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := svc.ProcessRewardOpenEvent(ctx, partner.ID, body, "127.0.0.1")
				errs <- err
			}()
		}
		wg.Wait()
		close(errs)
		for err := range errs {
			require.NoError(t, err)
		}
		duplicate, err := svc.ProcessRewardOpenEvent(ctx, partner.ID, body, "127.0.0.1")
		require.NoError(t, err)
		require.True(t, duplicate.Duplicate)
		otherTicket := start(task.ID, second.ID)
		_, err = svc.ProcessRewardOpenEvent(ctx, partner.ID, eventBody("external-used-"+suffix, "register", otherTicket, "external-a", ""), "127.0.0.1")
		require.Equal(t, "REWARD_EXTERNAL_USER_USED", infraerrors.Reason(err))
		// rejected event 允许修正后用同 event_id 重试。
		_, err = svc.ProcessRewardOpenEvent(ctx, partner.ID, eventBody("external-used-"+suffix, "register", otherTicket, "external-b", ""), "127.0.0.1")
		require.NoError(t, err)
	})
	t.Run("recharge two instalments and duplicate", func(t *testing.T) {
		task, err := svc.CreateAdminTask(ctx, taskInput("recharge", "callback", 0.75))
		require.NoError(t, err)
		ticket := start(task.ID, user.ID)
		first := eventBody("recharge-first-"+suffix, "recharge", ticket, "payer-a", "30.00")
		result, err := svc.ProcessRewardOpenEvent(ctx, partner.ID, first, "127.0.0.1")
		require.NoError(t, err)
		require.Equal(t, "accepted", result.Result)
		require.Equal(t, 30.0, result.Progress.RechargeTotal)
		result, err = svc.ProcessRewardOpenEvent(ctx, partner.ID, first, "127.0.0.1")
		require.NoError(t, err)
		require.True(t, result.Duplicate)
		require.Equal(t, 30.0, result.Progress.RechargeTotal)
		result, err = svc.ProcessRewardOpenEvent(ctx, partner.ID, eventBody("recharge-second-"+suffix, "recharge", ticket, "payer-a", "70.00"), "127.0.0.1")
		require.NoError(t, err)
		require.True(t, result.Rewarded)
		require.Equal(t, 100.0, result.Progress.RechargeTotal)
	})
	t.Run("SDK one-second heartbeats exclude pauses", func(t *testing.T) {
		in := taskInput("browse", "sdk", 0.25)
		task, err := svc.CreateAdminTask(ctx, in)
		require.NoError(t, err)
		require.Equal(t, 15, task.IdleTimeoutSeconds)
		ticket := start(task.ID, user.ID)
		require.NoError(t, client.RewardTaskRecord.Update().Where(rewardtaskrecord.Ticket(ticket)).SetProgressMs(1999).SetLastHeartbeatAt(time.Now()).SetTicketExpiresAt(time.Now().Add(-time.Second)).Exec(ctx))
		renewed, err := svc.StartTask(ctx, user.ID, task.ID)
		require.NoError(t, err)
		require.Zero(t, renewed.Current.ProgressSeconds)
		refreshed, err := client.RewardTaskRecord.Query().Where(rewardtaskrecord.TaskID(task.ID), rewardtaskrecord.UserID(user.ID)).Only(ctx)
		require.NoError(t, err)
		require.NotEqual(t, ticket, refreshed.Ticket)
		require.Zero(t, refreshed.ProgressMs)
		require.Nil(t, refreshed.LastHeartbeatAt)
		ticket = refreshed.Ticket
		heartbeat := func() *service.RewardSDKHeartbeatResult {
			result, err := svc.RewardSDKHeartbeat(ctx, partner.AppKey, ticket, "https://example.com", "")
			require.NoError(t, err)
			require.Equal(t, 1, result.HeartbeatInterval)
			require.Equal(t, 15, result.IdleTimeoutSeconds)
			return result
		}
		require.Equal(t, 0, heartbeat().ProgressSeconds)
		record, err := client.RewardTaskRecord.Query().Where(rewardtaskrecord.Ticket(ticket)).Only(ctx)
		require.NoError(t, err)
		require.Zero(t, record.ProgressMs)
		heartbeat()
		ignored, err := client.RewardTaskRecord.Get(ctx, record.ID)
		require.NoError(t, err)
		require.True(t, ignored.LastHeartbeatAt.Equal(*record.LastHeartbeatAt))
		time.Sleep(1020 * time.Millisecond)
		active := heartbeat()
		require.GreaterOrEqual(t, active.ProgressSeconds, 1)
		beforePause, err := client.RewardTaskRecord.Get(ctx, record.ID)
		require.NoError(t, err)
		require.GreaterOrEqual(t, beforePause.ProgressMs, int64(1000))
		time.Sleep(3100 * time.Millisecond)
		resumed := heartbeat()
		require.Equal(t, active.ProgressSeconds, resumed.ProgressSeconds)
		afterPause, err := client.RewardTaskRecord.Get(ctx, record.ID)
		require.NoError(t, err)
		require.Equal(t, beforePause.ProgressMs, afterPause.ProgressMs)
		require.True(t, afterPause.LastHeartbeatAt.After(*beforePause.LastHeartbeatAt))
		require.False(t, resumed.Completed)
		deadline := time.Now().Add(10 * time.Second)
		latest := resumed
		for !latest.Completed && time.Now().Before(deadline) {
			time.Sleep(1020 * time.Millisecond)
			latest = heartbeat()
		}
		require.True(t, latest.Completed)
		require.GreaterOrEqual(t, latest.ProgressSeconds, in.BrowseSeconds)
		require.True(t, heartbeat().Completed)
		record, err = client.RewardTaskRecord.Get(ctx, record.ID)
		require.NoError(t, err)
		// 完成后展示秒数固定为 browse_seconds；真实毫秒进度不少于要求时长。
		require.GreaterOrEqual(t, record.ProgressMs, int64(in.BrowseSeconds)*1000)
		require.Equal(t, in.BrowseSeconds, latest.ProgressSeconds)
		queried, err := svc.QueryRewardOpenTicket(ctx, partner.ID, ticket)
		require.NoError(t, err)
		require.Equal(t, latest.ProgressSeconds, queried.ProgressSeconds)
		tasks, err := svc.ListUserTasks(ctx, user.ID)
		require.NoError(t, err)
		found := false
		for _, item := range tasks.Tasks {
			if item.ID == task.ID {
				found = true
				require.Equal(t, 15, item.IdleTimeoutSeconds)
				require.Equal(t, latest.ProgressSeconds, item.Current.ProgressSeconds)
			}
		}
		require.True(t, found)
		adminRecords, _, err := svc.ListAdminRecords(ctx, service.RewardTaskRecordAdminFilter{TaskID: task.ID, Page: 1, PageSize: 20})
		require.NoError(t, err)
		require.Len(t, adminRecords, 1)
		require.Equal(t, latest.ProgressSeconds, adminRecords[0].ProgressSeconds)
		in.IdleTimeoutSeconds = 0
		updated, err := svc.UpdateAdminTask(ctx, task.ID, in)
		require.NoError(t, err)
		require.Zero(t, updated.IdleTimeoutSeconds)
		heartbeatWithNoIdle, err := svc.RewardSDKHeartbeat(ctx, partner.AppKey, ticket, "https://example.com", "")
		require.NoError(t, err)
		require.Zero(t, heartbeatWithNoIdle.IdleTimeoutSeconds)
	})
	t.Run("timer concurrent claims award once", func(t *testing.T) {
		task, err := svc.CreateAdminTask(ctx, taskInput("browse", "timer", 0.125))
		require.NoError(t, err)
		ticket := start(task.ID, user.ID)
		_, err = svc.ClaimTimerTask(ctx, user.ID, task.ID)
		require.Equal(t, "REWARD_TASK_TIMER_NOT_ELAPSED", infraerrors.Reason(err))
		require.NoError(t, client.RewardTaskRecord.Update().Where(rewardtaskrecord.Ticket(ticket)).SetStartedAt(time.Now().Add(-6*time.Second)).Exec(ctx))
		results := make(chan error, 8)
		var wg sync.WaitGroup
		for i := 0; i < 8; i++ {
			wg.Add(1)
			go func() { defer wg.Done(); _, err := svc.ClaimTimerTask(ctx, user.ID, task.ID); results <- err }()
		}
		wg.Wait()
		close(results)
		successes := 0
		for err := range results {
			if err == nil {
				successes++
			} else {
				require.Equal(t, "REWARD_TASK_ALREADY_COMPLETED", infraerrors.Reason(err))
			}
		}
		require.Equal(t, 1, successes)
	})
	t.Run("atomic quota and retained recharge retry", func(t *testing.T) {
		in := taskInput("recharge", "callback", 0.1)
		in.MaxCompletions = 1
		task, err := svc.CreateAdminTask(ctx, in)
		require.NoError(t, err)
		a := start(task.ID, user.ID)
		b := start(task.ID, second.ID)
		_, err = svc.ProcessRewardOpenEvent(ctx, partner.ID, eventBody("quota-a-"+suffix, "recharge", a, "quota-a", "100"), "127.0.0.1")
		require.NoError(t, err)
		secondBody := eventBody("quota-b-"+suffix, "recharge", b, "quota-b", "100")
		for i := 0; i < 2; i++ {
			_, err = svc.ProcessRewardOpenEvent(ctx, partner.ID, secondBody, "127.0.0.1")
			require.Equal(t, "REWARD_TASK_QUOTA_EXHAUSTED", infraerrors.Reason(err))
		}
		record, err := client.RewardTaskRecord.Query().Where(rewardtaskrecord.Ticket(b)).Only(ctx)
		require.NoError(t, err)
		require.Equal(t, 100.0, record.RechargeTotal)
		require.Equal(t, "in_progress", record.Status)
		in.MaxCompletions = 2
		_, err = svc.UpdateAdminTask(ctx, task.ID, in)
		require.NoError(t, err)
		result, err := svc.ProcessRewardOpenEvent(ctx, partner.ID, secondBody, "127.0.0.1")
		require.NoError(t, err)
		require.True(t, result.Rewarded)
		require.Equal(t, 100.0, result.Progress.RechargeTotal)
	})
	reloaded, err := client.User.Get(ctx, user.ID)
	require.NoError(t, err)
	require.InDelta(t, 1.725, reloaded.Balance, 1e-8)
	require.Zero(t, reloaded.TotalRecharged)
	listing, err := svc.ListUserTasks(ctx, user.ID)
	require.NoError(t, err)
	require.EqualValues(t, 5, listing.Summary.CompletedCount)
	require.InDelta(t, 1.725, listing.Summary.TotalReward, 1e-8)
	records, total, err := svc.ListUserRecords(ctx, user.ID, 1, 20)
	require.NoError(t, err)
	require.EqualValues(t, 5, total)
	require.Len(t, records, 5)
	stats, err := svc.GetAdminStats(ctx)
	require.NoError(t, err)
	require.GreaterOrEqual(t, stats.CompletedTotal, int64(7))
	adminService := service.NewAdminService(nil, nil, nil, nil, nil, nil, repository.NewRedeemCodeRepository(client), nil, nil, nil, nil, nil, nil, client, settings, nil, nil, nil, nil, nil, nil, nil, nil)
	for _, kind := range []string{service.RedeemTypeTaskReward, ""} {
		history, total, recharged, err := adminService.GetUserBalanceHistory(ctx, user.ID, 1, 20, kind)
		require.NoError(t, err)
		require.EqualValues(t, 5, total)
		require.Len(t, history, 5)
		require.Zero(t, recharged)
		for _, entry := range history {
			require.Equal(t, service.RedeemTypeTaskReward, entry.Type)
			require.True(t, strings.HasPrefix(entry.Code, "TASK-"))
			require.NotEmpty(t, entry.Notes)
		}
	}
	adminTasks, taskCount, err := svc.ListAdminTasks(ctx, service.RewardTaskAdminFilter{Page: 1, PageSize: 20, Search: suffix})
	require.NoError(t, err)
	require.EqualValues(t, 5, taskCount)
	require.Len(t, adminTasks, 5)
	adminRecords, recordCount, err := svc.ListAdminRecords(ctx, service.RewardTaskRecordAdminFilter{Page: 1, PageSize: 20, Search: user.Email})
	require.NoError(t, err)
	require.EqualValues(t, 5, recordCount)
	require.Len(t, adminRecords, 5)
	events, eventCount, err := svc.ListAdminEvents(ctx, service.RewardTaskEventAdminFilter{Page: 1, PageSize: 20, PartnerID: partner.ID})
	require.NoError(t, err)
	require.GreaterOrEqual(t, eventCount, int64(7))
	require.NotEmpty(t, events)
	require.Equal(t, "REWARD_PARTNER_IN_USE", infraerrors.Reason(svc.DeleteAdminPartner(ctx, partner.ID)))
	t.Run("different users racing last quota", func(t *testing.T) {
		a, err := client.User.Create().SetEmail("quota-race-a-" + suffix + "@example.com").SetPasswordHash("x").Save(ctx)
		require.NoError(t, err)
		b, err := client.User.Create().SetEmail("quota-race-b-" + suffix + "@example.com").SetPasswordHash("x").Save(ctx)
		require.NoError(t, err)
		in := taskInput("register", "callback", 2)
		in.MaxCompletions = 1
		task, err := svc.CreateAdminTask(ctx, in)
		require.NoError(t, err)
		tickets := []string{start(task.ID, a.ID), start(task.ID, b.ID)}
		gate := make(chan struct{})
		errs := make(chan error, 2)
		var wg sync.WaitGroup
		for i := range tickets {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				<-gate
				_, err := svc.ProcessRewardOpenEvent(ctx, partner.ID, eventBody(fmt.Sprintf("quota-race-%d-%s", i, suffix), "register", tickets[i], fmt.Sprint(i), ""), "127.0.0.1")
				errs <- err
			}(i)
		}
		close(gate)
		wg.Wait()
		close(errs)
		successes := 0
		for err := range errs {
			if err == nil {
				successes++
			} else {
				require.Equal(t, "REWARD_TASK_QUOTA_EXHAUSTED", infraerrors.Reason(err))
			}
		}
		require.Equal(t, 1, successes)
		a, err = client.User.Get(ctx, a.ID)
		require.NoError(t, err)
		b, err = client.User.Get(ctx, b.ID)
		require.NoError(t, err)
		require.Equal(t, 2.0, a.Balance+b.Balance)
		stored, err := client.RewardTask.Get(ctx, task.ID)
		require.NoError(t, err)
		require.Equal(t, 1, stored.CompletedCount)
	})
	require.NoError(t, settings.SetRewardTasksEnabled(ctx, false))
	disabled, err := svc.ListUserTasks(ctx, user.ID)
	require.NoError(t, err)
	require.False(t, disabled.Enabled)
	require.Empty(t, disabled.Tasks)
	require.EqualValues(t, 5, disabled.Summary.CompletedCount)
}
