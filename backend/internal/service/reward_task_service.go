package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math"
	"net/url"
	"strings"
	"time"

	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/rewardtask"
	"github.com/Wei-Shaw/sub2api/ent/rewardtaskrecord"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

type RewardNonceStore interface {
	ConsumeNonce(context.Context, string, time.Duration) (bool, error)
}

type RewardTaskService struct {
	entClient            *dbent.Client
	userRepo             UserRepository
	settingService       *SettingService
	enterpriseMembership EnterpriseMembershipReader
	billingCacheService  *BillingCacheService
	authCacheInvalidator APIKeyAuthCacheInvalidator
	encryptor            SecretEncryptor
	nonceStore           RewardNonceStore
}

func NewRewardTaskService(client *dbent.Client, users UserRepository, settings *SettingService, membership EnterpriseMembershipReader, billing *BillingCacheService, auth APIKeyAuthCacheInvalidator, encryptor SecretEncryptor, nonceStore RewardNonceStore) *RewardTaskService {
	return &RewardTaskService{entClient: client, userRepo: users, settingService: settings, enterpriseMembership: membership, billingCacheService: billing, authCacheInvalidator: auth, encryptor: encryptor, nonceStore: nonceStore}
}

func rewardTaskError(reason string) *infraerrors.ApplicationError {
	message := strings.ToLower(strings.ReplaceAll(strings.TrimPrefix(reason, "REWARD_"), "_", " "))
	switch reason {
	case "REWARD_TASK_NOT_FOUND", "REWARD_PARTNER_NOT_FOUND", "REWARD_TICKET_NOT_FOUND":
		return infraerrors.NotFound(reason, message)
	case "REWARD_TASK_ALREADY_COMPLETED", "REWARD_TASK_QUOTA_EXHAUSTED", "REWARD_PARTNER_IN_USE", "REWARD_EXTERNAL_USER_USED", "REWARD_EXTERNAL_USER_MISMATCH":
		return infraerrors.Conflict(reason, message)
	case "REWARD_PARTNER_INVALID", "REWARD_SIGNATURE_MISSING", "REWARD_SIGNATURE_INVALID", "REWARD_TIMESTAMP_EXPIRED", "REWARD_NONCE_REPLAYED":
		return infraerrors.Unauthorized(reason, message)
	case "REWARD_TICKET_PARTNER_MISMATCH", "REWARD_ORIGIN_NOT_ALLOWED":
		return infraerrors.Forbidden(reason, message)
	default:
		return infraerrors.BadRequest(reason, message)
	}
}

func (s *RewardTaskService) enabled(ctx context.Context) (bool, error) {
	if s.settingService == nil {
		return false, nil
	}
	settings, err := s.settingService.GetRewardTaskSettings(ctx)
	if err != nil {
		return false, err
	}
	return settings.Enabled, nil
}

func rewardPeriod(period string, now time.Time) (string, *time.Time) {
	local := now.In(time.Local)
	day := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.Local)
	var key string
	var end time.Time
	switch period {
	case "daily":
		key = day.Format("2006-01-02")
		end = day.AddDate(0, 0, 1)
	case "weekly":
		year, week := day.ISOWeek()
		key = fmt.Sprintf("%04d-W%02d", year, week)
		offset := (int(day.Weekday()) + 6) % 7
		end = day.AddDate(0, 0, 7-offset)
	case "monthly":
		key = day.Format("2006-01")
		end = time.Date(day.Year(), day.Month()+1, 1, 0, 0, 0, 0, time.Local)
	default:
		return "once", nil
	}
	return key, &end
}

func rewardRandomToken(prefix string, bytes int) (string, error) {
	data := make([]byte, bytes)
	if _, err := rand.Read(data); err != nil {
		return "", err
	}
	return prefix + hex.EncodeToString(data), nil
}

func rewardTicketExpiry(taskType string, now time.Time, reset *time.Time) time.Time {
	ttl := 30 * 24 * time.Hour
	if taskType == "browse" {
		ttl = 24 * time.Hour
	}
	expires := now.Add(ttl)
	if reset != nil && reset.Before(expires) {
		return *reset
	}
	return expires
}

func rewardJumpURL(target, mode, ticket string) (string, error) {
	if mode == "timer" {
		return strings.ReplaceAll(target, "{ticket}", ""), nil
	}
	if strings.Contains(target, "{ticket}") {
		return strings.ReplaceAll(target, "{ticket}", ticket), nil
	}
	parsed, err := url.Parse(target)
	if err != nil {
		return "", err
	}
	query := parsed.Query()
	query.Set("reward_ticket", ticket)
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func (s *RewardTaskService) taskAvailable(ctx context.Context, task *dbent.RewardTask, now time.Time) error {
	if task.DeletedAt != nil || !task.Enabled || (task.StartsAt != nil && now.Before(*task.StartsAt)) || (task.EndsAt != nil && !now.Before(*task.EndsAt)) {
		return rewardTaskError("REWARD_TASK_UNAVAILABLE")
	}
	if task.PartnerID != nil {
		partner, err := s.entClient.RewardTaskPartner.Get(ctx, *task.PartnerID)
		if dbent.IsNotFound(err) {
			return rewardTaskError("REWARD_TASK_UNAVAILABLE")
		}
		if err != nil {
			return err
		}
		if !partner.Enabled || partner.DeletedAt != nil {
			return rewardTaskError("REWARD_TASK_UNAVAILABLE")
		}
	}
	return nil
}

func (s *RewardTaskService) loadTask(ctx context.Context, id int64) (*dbent.RewardTask, error) {
	task, err := s.entClient.RewardTask.Get(ctx, id)
	if dbent.IsNotFound(err) {
		return nil, rewardTaskError("REWARD_TASK_NOT_FOUND")
	}
	return task, err
}

type RewardTaskCurrent struct {
	Status          string     `json:"status"`
	PeriodKey       string     `json:"period_key"`
	StartedAt       time.Time  `json:"started_at"`
	TicketExpiresAt time.Time  `json:"ticket_expires_at"`
	ClaimableAt     *time.Time `json:"claimable_at"`
	ProgressSeconds int        `json:"progress_seconds"`
	RechargeTotal   float64    `json:"recharge_total"`
	CompletedAt     *time.Time `json:"completed_at"`
	RewardAmount    float64    `json:"reward_amount"`
}

func rewardCurrent(record *dbent.RewardTaskRecord, task *dbent.RewardTask) *RewardTaskCurrent {
	if record == nil {
		return nil
	}
	out := &RewardTaskCurrent{Status: record.Status, PeriodKey: record.PeriodKey, StartedAt: record.StartedAt, TicketExpiresAt: record.TicketExpiresAt, ProgressSeconds: rewardProgressSeconds(record.ProgressMs, task.BrowseSeconds, record.Status == "completed"), RechargeTotal: record.RechargeTotal, CompletedAt: record.CompletedAt, RewardAmount: record.RewardAmount}
	if task.VerifyMode == "timer" {
		at := record.StartedAt.Add(time.Duration(task.BrowseSeconds) * time.Second)
		out.ClaimableAt = &at
	}
	return out
}

type UserRewardTask struct {
	ID                 int64              `json:"id"`
	Title              string             `json:"title"`
	Description        string             `json:"description"`
	TaskType           string             `json:"task_type"`
	VerifyMode         string             `json:"verify_mode"`
	TargetURL          string             `json:"target_url"`
	IconURL            string             `json:"icon_url"`
	RewardAmount       float64            `json:"reward_amount"`
	Period             string             `json:"period"`
	BrowseSeconds      int                `json:"browse_seconds"`
	IdleTimeoutSeconds int                `json:"idle_timeout_seconds"`
	MinRechargeAmount  float64            `json:"min_recharge_amount"`
	EndsAt             *time.Time         `json:"ends_at"`
	QuotaExhausted     bool               `json:"quota_exhausted"`
	NextResetAt        *time.Time         `json:"next_reset_at"`
	Current            *RewardTaskCurrent `json:"current"`
}
type RewardTaskSummary struct {
	CompletedCount int64   `json:"completed_count"`
	TotalReward    float64 `json:"total_reward"`
}
type UserRewardTaskList struct {
	Enabled bool              `json:"enabled"`
	Tasks   []UserRewardTask  `json:"tasks"`
	Summary RewardTaskSummary `json:"summary"`
}

func (s *RewardTaskService) ListUserTasks(ctx context.Context, userID int64) (*UserRewardTaskList, error) {
	enabled, err := s.enabled(ctx)
	if err != nil {
		return nil, err
	}
	out := &UserRewardTaskList{Enabled: enabled, Tasks: []UserRewardTask{}}
	rows, err := s.entClient.QueryContext(ctx, `SELECT COUNT(*),COALESCE(SUM(reward_amount),0)::double precision FROM reward_task_records WHERE user_id=$1 AND status='completed'`, userID)
	if err != nil {
		return nil, err
	}
	if rows.Next() {
		err = rows.Scan(&out.Summary.CompletedCount, &out.Summary.TotalReward)
	}
	rowsErr := rows.Err()
	_ = rows.Close()
	if err != nil {
		return nil, err
	}
	if rowsErr != nil {
		return nil, rowsErr
	}
	if !enabled {
		return out, nil
	}
	now := time.Now()
	tasks, err := s.entClient.RewardTask.Query().Where(rewardtask.Enabled(true), rewardtask.Or(rewardtask.StartsAtIsNil(), rewardtask.StartsAtLTE(now)), rewardtask.Or(rewardtask.EndsAtIsNil(), rewardtask.EndsAtGT(now))).Order(rewardtask.BySortOrder(), rewardtask.ByID(entsql.OrderDesc())).All(ctx)
	if err != nil {
		return nil, err
	}
	if len(tasks) == 0 {
		return out, nil
	}
	// 只拉取可见任务的当前周期，避免历史积累造成无界查询。
	taskIDs := make([]int64, 0, len(tasks))
	for _, task := range tasks {
		taskIDs = append(taskIDs, task.ID)
	}
	keys := make([]string, 0, 4)
	for _, period := range []string{"once", "daily", "weekly", "monthly"} {
		key, _ := rewardPeriod(period, now)
		keys = append(keys, key)
	}
	records, err := s.entClient.RewardTaskRecord.Query().Where(rewardtaskrecord.UserID(userID), rewardtaskrecord.TaskIDIn(taskIDs...), rewardtaskrecord.PeriodKeyIn(keys...)).All(ctx)
	if err != nil {
		return nil, err
	}
	byTask := map[string]*dbent.RewardTaskRecord{}
	for _, r := range records {
		byTask[fmt.Sprintf("%d:%s", r.TaskID, r.PeriodKey)] = r
	}
	for _, task := range tasks {
		if err := s.taskAvailable(ctx, task, now); err != nil {
			if infraerrors.Reason(err) == "REWARD_TASK_UNAVAILABLE" {
				continue
			}
			return nil, err
		}
		key, reset := rewardPeriod(task.Period, now)
		out.Tasks = append(out.Tasks, UserRewardTask{ID: task.ID, Title: task.Title, Description: task.Description, TaskType: task.TaskType, VerifyMode: task.VerifyMode, TargetURL: strings.ReplaceAll(task.TargetURL, "{ticket}", ""), IconURL: task.IconURL, RewardAmount: task.RewardAmount, Period: task.Period, BrowseSeconds: task.BrowseSeconds, IdleTimeoutSeconds: task.IdleTimeoutSeconds, MinRechargeAmount: task.MinRechargeAmount, EndsAt: task.EndsAt, QuotaExhausted: task.MaxCompletions > 0 && task.CompletedCount >= task.MaxCompletions, NextResetAt: reset, Current: rewardCurrent(byTask[fmt.Sprintf("%d:%s", task.ID, key)], task)})
	}
	return out, nil
}

type StartRewardTaskResult struct {
	JumpURL string             `json:"jump_url"`
	Current *RewardTaskCurrent `json:"current"`
}

func (s *RewardTaskService) StartTask(ctx context.Context, userID, taskID int64) (*StartRewardTaskResult, error) {
	enabled, err := s.enabled(ctx)
	if err != nil {
		return nil, err
	}
	if !enabled {
		return nil, rewardTaskError("REWARD_TASKS_DISABLED")
	}
	task, err := s.loadTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	if err := s.taskAvailable(ctx, task, now); err != nil {
		return nil, err
	}
	if err := checkEnterprisePersonalWallet(ctx, s.enterpriseMembership, userID); err != nil {
		return nil, err
	}
	key, reset := rewardPeriod(task.Period, now)
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	// 对相同用户/任务/周期的并发 start 串行化，避免无记录时唯一冲突导致事务中断。
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, fmt.Sprintf("reward-start:%d:%d:%s", userID, taskID, key)); err != nil {
		return nil, err
	}
	record, err := tx.RewardTaskRecord.Query().Where(rewardtaskrecord.UserID(userID), rewardtaskrecord.TaskID(taskID), rewardtaskrecord.PeriodKey(key)).ForUpdate().Only(ctx)
	if err != nil && !dbent.IsNotFound(err) {
		return nil, err
	}
	if record != nil && record.Status == "completed" {
		return nil, rewardTaskError("REWARD_TASK_ALREADY_COMPLETED")
	}
	if record == nil || !record.TicketExpiresAt.After(now) {
		// 只有新记录检查名额；过期记录重发票据不消耗名额，实际入账仍有原子门禁。
		if record == nil && task.MaxCompletions > 0 && task.CompletedCount >= task.MaxCompletions {
			return nil, rewardTaskError("REWARD_TASK_QUOTA_EXHAUSTED")
		}
		ticket, err := rewardRandomToken("rt_", 20)
		if err != nil {
			return nil, err
		}
		expiry := rewardTicketExpiry(task.TaskType, now, reset)
		if record == nil {
			record, err = tx.RewardTaskRecord.Create().SetUserID(userID).SetTaskID(taskID).SetPeriodKey(key).SetTicket(ticket).SetStatus("in_progress").SetStartedAt(now).SetTicketExpiresAt(expiry).Save(ctx)
		} else {
			record, err = tx.RewardTaskRecord.UpdateOneID(record.ID).SetTicket(ticket).SetProgressMs(0).SetRechargeTotal(0).SetExternalUserID("").ClearLastHeartbeatAt().SetStartedAt(now).SetTicketExpiresAt(expiry).Save(ctx)
		}
		if err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	jump, err := rewardJumpURL(task.TargetURL, task.VerifyMode, record.Ticket)
	if err != nil {
		return nil, err
	}
	return &StartRewardTaskResult{JumpURL: jump, Current: rewardCurrent(record, task)}, nil
}

// 调用方须持有 record 的 FOR UPDATE 锁；名额、余额、记录在同一事务提交。
func (s *RewardTaskService) completeRewardTask(ctx context.Context, tx *dbent.Tx, record *dbent.RewardTaskRecord, task *dbent.RewardTask, now time.Time) (bool, error) {
	if record.Status == "completed" {
		return false, nil
	}
	result, err := tx.ExecContext(ctx, `UPDATE reward_tasks SET completed_count=completed_count+1,updated_at=$2 WHERE id=$1 AND (max_completions=0 OR completed_count<max_completions)`, task.ID, now)
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	if count == 0 {
		return false, rewardTaskError("REWARD_TASK_QUOTA_EXHAUSTED")
	}
	if err := tx.RewardTaskRecord.UpdateOneID(record.ID).SetStatus("completed").SetCompletedAt(now).SetRewardAmount(task.RewardAmount).SetTaskTitle(task.Title).Exec(ctx); err != nil {
		return false, err
	}
	if err := tx.User.UpdateOneID(record.UserID).AddBalance(task.RewardAmount).Exec(ctx); err != nil {
		return false, err
	}
	record.Status = "completed"
	record.CompletedAt = &now
	record.RewardAmount = task.RewardAmount
	record.TaskTitle = task.Title
	return true, nil
}
func (s *RewardTaskService) invalidateBalanceCaches(ctx context.Context, userID int64) {
	if s.authCacheInvalidator != nil {
		s.authCacheInvalidator.InvalidateAuthCacheByUserID(ctx, userID)
	}
	if s.billingCacheService != nil {
		go func() {
			cacheCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = s.billingCacheService.InvalidateUserBalance(cacheCtx, userID)
		}()
	}
}

type ClaimRewardTaskResult struct {
	RewardAmount float64            `json:"reward_amount"`
	Balance      float64            `json:"balance"`
	Current      *RewardTaskCurrent `json:"current"`
}

func (s *RewardTaskService) ClaimTimerTask(ctx context.Context, userID, taskID int64) (*ClaimRewardTaskResult, error) {
	enabled, err := s.enabled(ctx)
	if err != nil {
		return nil, err
	}
	if !enabled {
		return nil, rewardTaskError("REWARD_TASKS_DISABLED")
	}
	task, err := s.loadTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if task.VerifyMode != "timer" {
		return nil, rewardTaskError("REWARD_TASK_CLAIM_NOT_SUPPORTED")
	}
	now := time.Now()
	if err := s.taskAvailable(ctx, task, now); err != nil {
		return nil, err
	}
	if err := checkEnterprisePersonalWallet(ctx, s.enterpriseMembership, userID); err != nil {
		return nil, err
	}
	key, _ := rewardPeriod(task.Period, now)
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	record, err := tx.RewardTaskRecord.Query().Where(rewardtaskrecord.UserID(userID), rewardtaskrecord.TaskID(taskID), rewardtaskrecord.PeriodKey(key)).ForUpdate().Only(ctx)
	if dbent.IsNotFound(err) {
		return nil, rewardTaskError("REWARD_TASK_NOT_STARTED")
	}
	if err != nil {
		return nil, err
	}
	if record.Status == "completed" {
		return nil, rewardTaskError("REWARD_TASK_ALREADY_COMPLETED")
	}
	if !record.TicketExpiresAt.After(now) {
		return nil, rewardTaskError("REWARD_TICKET_EXPIRED")
	}
	claimAt := record.StartedAt.Add(time.Duration(task.BrowseSeconds) * time.Second)
	if now.Before(claimAt) {
		return nil, rewardTaskError("REWARD_TASK_TIMER_NOT_ELAPSED").WithMetadata(map[string]string{"remaining_seconds": fmt.Sprintf("%.0f", math.Ceil(claimAt.Sub(now).Seconds()))})
	}
	if _, err := s.completeRewardTask(ctx, tx, record, task, now); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	s.invalidateBalanceCaches(ctx, userID)
	user, err := s.entClient.User.Get(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &ClaimRewardTaskResult{RewardAmount: record.RewardAmount, Balance: user.Balance, Current: rewardCurrent(record, task)}, nil
}

type UserRewardTaskRecord struct {
	ID           int64      `json:"id"`
	TaskID       int64      `json:"task_id"`
	TaskTitle    string     `json:"task_title"`
	TaskType     string     `json:"task_type"`
	PeriodKey    string     `json:"period_key"`
	RewardAmount float64    `json:"reward_amount"`
	CompletedAt  *time.Time `json:"completed_at"`
}

func (s *RewardTaskService) ListUserRecords(ctx context.Context, userID int64, page, pageSize int) ([]UserRewardTaskRecord, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	q := s.entClient.RewardTaskRecord.Query().Where(rewardtaskrecord.UserID(userID), rewardtaskrecord.Status("completed"))
	total, err := q.Clone().Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	records, err := q.Order(rewardtaskrecord.ByCompletedAt(entsql.OrderDesc()), rewardtaskrecord.ByID(entsql.OrderDesc())).Limit(pageSize).Offset((page - 1) * pageSize).All(ctx)
	if err != nil {
		return nil, 0, err
	}
	items := make([]UserRewardTaskRecord, 0, len(records))
	for _, r := range records {
		// 历史完成记录保留已软删任务的类型，奖励与标题取完成快照。
		var taskType string
		rows, err := s.entClient.QueryContext(ctx, `SELECT task_type FROM reward_tasks WHERE id=$1`, r.TaskID)
		if err != nil {
			return nil, 0, err
		}
		if rows.Next() {
			err = rows.Scan(&taskType)
		}
		rowErr := rows.Err()
		_ = rows.Close()
		if err != nil {
			return nil, 0, err
		}
		if rowErr != nil {
			return nil, 0, rowErr
		}
		items = append(items, UserRewardTaskRecord{ID: r.ID, TaskID: r.TaskID, TaskTitle: r.TaskTitle, TaskType: taskType, PeriodKey: r.PeriodKey, RewardAmount: r.RewardAmount, CompletedAt: r.CompletedAt})
	}
	return items, int64(total), nil
}
