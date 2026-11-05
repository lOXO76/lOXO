package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/rewardtask"
	"github.com/Wei-Shaw/sub2api/ent/rewardtaskpartner"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

type RewardTaskInput struct {
	Title              string     `json:"title"`
	Description        string     `json:"description"`
	TaskType           string     `json:"task_type"`
	VerifyMode         string     `json:"verify_mode"`
	PartnerID          *int64     `json:"partner_id"`
	TargetURL          string     `json:"target_url"`
	IconURL            string     `json:"icon_url"`
	RewardAmount       float64    `json:"reward_amount"`
	Period             string     `json:"period"`
	BrowseSeconds      int        `json:"browse_seconds"`
	IdleTimeoutSeconds int        `json:"idle_timeout_seconds"`
	MinRechargeAmount  float64    `json:"min_recharge_amount"`
	MaxCompletions     int        `json:"max_completions"`
	StartsAt           *time.Time `json:"starts_at"`
	EndsAt             *time.Time `json:"ends_at"`
	Enabled            bool       `json:"enabled"`
	SortOrder          int        `json:"sort_order"`
}

type AdminRewardTask struct {
	RewardTaskInput
	ID              int64     `json:"id"`
	PartnerName     string    `json:"partner_name"`
	CompletedCount  int       `json:"completed_count"`
	InProgressCount int64     `json:"in_progress_count"`
	TotalReward     float64   `json:"total_reward"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type RewardTaskAdminFilter struct {
	Page, PageSize int
	TaskType       string
	Enabled        *bool
	Search         string
}

type RewardTaskPartnerInput struct {
	Name           string   `json:"name"`
	AllowedDomains []string `json:"allowed_domains"`
	Contact        string   `json:"contact"`
	Notes          string   `json:"notes"`
	Enabled        bool     `json:"enabled"`
}

type AdminRewardTaskPartner struct {
	RewardTaskPartnerInput
	ID          int64      `json:"id"`
	AppKey      string     `json:"app_key"`
	SecretHint  string     `json:"secret_hint"`
	TaskCount   int64      `json:"task_count"`
	LastEventAt *time.Time `json:"last_event_at"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	AppSecret   string     `json:"app_secret,omitempty"`
}

type RewardTaskAdminStats struct {
	TotalTasks     int64   `json:"total_tasks"`
	EnabledTasks   int64   `json:"enabled_tasks"`
	CompletedToday int64   `json:"completed_today"`
	RewardToday    float64 `json:"reward_today"`
	CompletedTotal int64   `json:"completed_total"`
	RewardTotal    float64 `json:"reward_total"`
	Partners       int64   `json:"partners"`
}

type RewardTaskRecordAdminFilter struct {
	Page, PageSize int
	TaskID         int64
	Status, Search string
}

type AdminRewardTaskRecord struct {
	ID              int64      `json:"id"`
	TaskID          int64      `json:"task_id"`
	TaskTitle       string     `json:"task_title"`
	TaskType        string     `json:"task_type"`
	UserID          int64      `json:"user_id"`
	UserEmail       string     `json:"user_email"`
	Username        string     `json:"username"`
	PeriodKey       string     `json:"period_key"`
	Status          string     `json:"status"`
	ProgressSeconds int        `json:"progress_seconds"`
	RechargeTotal   float64    `json:"recharge_total"`
	ExternalUserID  string     `json:"external_user_id"`
	StartedAt       time.Time  `json:"started_at"`
	TicketExpiresAt time.Time  `json:"ticket_expires_at"`
	CompletedAt     *time.Time `json:"completed_at"`
	RewardAmount    float64    `json:"reward_amount"`
}

type RewardTaskEventAdminFilter struct {
	Page, PageSize int
	PartnerID      int64
	Result         string
}

type AdminRewardTaskEvent struct {
	ID             int64           `json:"id"`
	PartnerID      int64           `json:"partner_id"`
	PartnerName    string          `json:"partner_name"`
	EventID        string          `json:"event_id"`
	EventType      string          `json:"event_type"`
	Ticket         string          `json:"ticket"`
	TaskID         *int64          `json:"task_id"`
	UserID         *int64          `json:"user_id"`
	ExternalUserID string          `json:"external_user_id"`
	Amount         float64         `json:"amount"`
	OrderID        string          `json:"order_id"`
	Result         string          `json:"result"`
	ErrorCode      string          `json:"error_code"`
	RemoteIP       string          `json:"remote_ip"`
	Payload        json.RawMessage `json:"payload"`
	CreatedAt      time.Time       `json:"created_at"`
}

func rewardTaskInputError(message string) error {
	return infraerrors.BadRequest("REWARD_TASK_INVALID", message)
}

func rewardDecimalValid(value float64, precision int, min, max float64) bool {
	if math.IsNaN(value) || math.IsInf(value, 0) || value < min || value > max {
		return false
	}
	decimal := strconv.FormatFloat(value, 'f', -1, 64)
	if dot := strings.IndexByte(decimal, '.'); dot >= 0 {
		return len(decimal)-dot-1 <= precision
	}
	return true
}

func rewardHTTPURL(raw string) bool {
	parsed, err := url.Parse(strings.ReplaceAll(raw, "{ticket}", "reward_ticket"))
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Hostname() != "" && parsed.User == nil
}

// ValidateRewardTaskInput 校验组合与金额精度，非对应类型的进度字段在保存时归零。
func ValidateRewardTaskInput(in RewardTaskInput) error {
	if n := utf8.RuneCountInString(strings.TrimSpace(in.Title)); n < 1 || n > 100 {
		return rewardTaskInputError("title must contain 1 to 100 characters")
	}
	if utf8.RuneCountInString(in.Description) > 2000 {
		return rewardTaskInputError("description must not exceed 2000 characters")
	}
	if len(in.TargetURL) > 2048 || !rewardHTTPURL(in.TargetURL) {
		return rewardTaskInputError("target_url must be an http or https URL of at most 2048 characters")
	}
	localIcon := strings.HasPrefix(in.IconURL, "/") && !strings.HasPrefix(in.IconURL, "//") && !strings.ContainsAny(in.IconURL, "\\\r\n")
	if in.IconURL != "" && !rewardHTTPURL(in.IconURL) && !localIcon {
		return rewardTaskInputError("icon_url must be an http/https URL or a local absolute path")
	}
	if !rewardDecimalValid(in.RewardAmount, 8, 0.00000001, 10000) {
		return rewardTaskInputError("reward_amount must be greater than 0, at most 10000, with up to 8 decimal places")
	}
	switch in.Period {
	case "once", "daily", "weekly", "monthly":
	default:
		return rewardTaskInputError("period must be once, daily, weekly or monthly")
	}
	switch in.TaskType {
	case "browse":
		if in.VerifyMode != "timer" && in.VerifyMode != "sdk" {
			return rewardTaskInputError("browse tasks require timer or sdk verification")
		}
		if in.BrowseSeconds < 5 || in.BrowseSeconds > 86400 {
			return rewardTaskInputError("browse_seconds must be between 5 and 86400")
		}
	case "register":
		if in.VerifyMode != "callback" || in.Period != "once" {
			return rewardTaskInputError("register tasks require callback verification and once period")
		}
	case "recharge":
		if in.VerifyMode != "callback" {
			return rewardTaskInputError("recharge tasks require callback verification")
		}
		if !rewardDecimalValid(in.MinRechargeAmount, 2, 0.01, 100000000) {
			return rewardTaskInputError("min_recharge_amount must be between 0.01 and 100000000 with up to 2 decimal places")
		}
	default:
		return rewardTaskInputError("task_type must be browse, register or recharge")
	}
	if in.VerifyMode == "timer" && in.PartnerID != nil {
		return rewardTaskInputError("partner_id must be null for timer tasks")
	}
	if in.VerifyMode != "timer" && (in.PartnerID == nil || *in.PartnerID <= 0) {
		return rewardTaskInputError("partner_id is required for sdk and callback tasks")
	}
	if in.IdleTimeoutSeconds != 0 && (in.IdleTimeoutSeconds < 5 || in.IdleTimeoutSeconds > 3600) {
		return rewardTaskInputError("idle_timeout_seconds must be 0 or an integer between 5 and 3600")
	}
	if in.MaxCompletions < 0 || in.MaxCompletions > math.MaxInt32 {
		return rewardTaskInputError("max_completions must be between 0 and 2147483647")
	}
	if in.SortOrder < math.MinInt32 || in.SortOrder > math.MaxInt32 {
		return rewardTaskInputError("sort_order is outside the supported integer range")
	}
	if in.StartsAt != nil && in.EndsAt != nil && !in.StartsAt.Before(*in.EndsAt) {
		return rewardTaskInputError("starts_at must be earlier than ends_at")
	}
	return nil
}

// NormalizeRewardPartnerDomains 仅存 host，不接受协议、端口、路径或非前缀通配符。
func NormalizeRewardPartnerDomains(input []string) ([]string, error) {
	out := make([]string, 0, len(input))
	seen := make(map[string]bool, len(input))
	for _, value := range input {
		value = strings.ToLower(strings.TrimSpace(value))
		host := strings.TrimPrefix(value, "*.")
		wildcard := host != value
		valid := rewardDomainHostValid(host)
		if wildcard && net.ParseIP(host) != nil {
			valid = false
		}
		if !valid {
			return nil, infraerrors.BadRequest("REWARD_PARTNER_INVALID_DOMAINS", "allowed_domains must contain valid hosts or *.host without a scheme, port or path")
		}
		if !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	if len(out) > 20 {
		return nil, infraerrors.BadRequest("REWARD_PARTNER_INVALID_DOMAINS", "allowed_domains must not exceed 20 unique hosts")
	}
	return out, nil
}

func rewardDomainHostValid(host string) bool {
	if net.ParseIP(host) != nil {
		return true
	}
	if len(host) == 0 || len(host) > 253 {
		return false
	}
	for _, label := range strings.Split(host, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, c := range label {
			if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '-' {
				return false
			}
		}
	}
	return true
}

func normalizeRewardTaskInput(in RewardTaskInput) RewardTaskInput {
	in.Title = strings.TrimSpace(in.Title)
	if in.TaskType != "browse" {
		in.BrowseSeconds = 0
	}
	if in.TaskType != "recharge" {
		in.MinRechargeAmount = 0
	}
	return in
}

func normalizeRewardPartnerInput(in RewardTaskPartnerInput) (RewardTaskPartnerInput, error) {
	in.Name, in.Contact = strings.TrimSpace(in.Name), strings.TrimSpace(in.Contact)
	if n := utf8.RuneCountInString(in.Name); n < 1 || n > 100 {
		return in, rewardTaskInputError("name must contain 1 to 100 characters")
	}
	if utf8.RuneCountInString(in.Contact) > 200 {
		return in, rewardTaskInputError("contact must not exceed 200 characters")
	}
	var err error
	in.AllowedDomains, err = NormalizeRewardPartnerDomains(in.AllowedDomains)
	return in, err
}

func rewardAdminPagination(page, size int) (int, int) {
	if page <= 0 {
		page = 1
	}
	if size <= 0 {
		size = 20
	}
	if size > 100 {
		size = 100
	}
	return page, size
}

func (s *RewardTaskService) rewardAdminReady() error {
	if s == nil || s.entClient == nil {
		return infraerrors.InternalServer("REWARD_TASK_UNAVAILABLE", "reward task service is unavailable")
	}
	return nil
}

func (s *RewardTaskService) rewardAdminQueryRow(ctx context.Context, query string, args []any, dest ...any) error {
	rows, err := s.entClient.QueryContext(ctx, query, args...)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return err
		}
		return sql.ErrNoRows
	}
	if err := rows.Scan(dest...); err != nil {
		return err
	}
	return rows.Err()
}

func (s *RewardTaskService) ListAdminTasks(ctx context.Context, filter RewardTaskAdminFilter) ([]AdminRewardTask, int64, error) {
	if err := s.rewardAdminReady(); err != nil {
		return nil, 0, err
	}
	filter.Page, filter.PageSize = rewardAdminPagination(filter.Page, filter.PageSize)
	query := s.entClient.RewardTask.Query().Where(rewardtask.DeletedAtIsNil())
	if filter.TaskType != "" {
		query.Where(rewardtask.TaskTypeEQ(filter.TaskType))
	}
	if filter.Enabled != nil {
		query.Where(rewardtask.EnabledEQ(*filter.Enabled))
	}
	if search := strings.TrimSpace(filter.Search); search != "" {
		query.Where(rewardtask.TitleContainsFold(search))
	}
	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	tasks, err := query.Order(dbent.Asc(rewardtask.FieldSortOrder), dbent.Desc(rewardtask.FieldID)).Offset((filter.Page - 1) * filter.PageSize).Limit(filter.PageSize).All(ctx)
	if err != nil {
		return nil, 0, err
	}
	items, err := s.rewardAdminTaskViews(ctx, tasks)
	return items, int64(total), err
}

func (s *RewardTaskService) rewardAdminTaskViews(ctx context.Context, tasks []*dbent.RewardTask) ([]AdminRewardTask, error) {
	items := make([]AdminRewardTask, 0, len(tasks))
	if len(tasks) == 0 {
		return items, nil
	}
	ids, args := make([]string, 0, len(tasks)), make([]any, 0, len(tasks)+1)
	for _, task := range tasks {
		args = append(args, task.ID)
		ids = append(ids, fmt.Sprintf("$%d", len(args)))
		items = append(items, AdminRewardTask{RewardTaskInput: RewardTaskInput{Title: task.Title, Description: task.Description, TaskType: task.TaskType, VerifyMode: task.VerifyMode, PartnerID: task.PartnerID, TargetURL: task.TargetURL, IconURL: task.IconURL, RewardAmount: task.RewardAmount, Period: task.Period, BrowseSeconds: task.BrowseSeconds, IdleTimeoutSeconds: task.IdleTimeoutSeconds, MinRechargeAmount: task.MinRechargeAmount, MaxCompletions: task.MaxCompletions, StartsAt: task.StartsAt, EndsAt: task.EndsAt, Enabled: task.Enabled, SortOrder: task.SortOrder}, ID: task.ID, CompletedCount: task.CompletedCount, CreatedAt: task.CreatedAt, UpdatedAt: task.UpdatedAt})
	}
	args = append(args, time.Now())
	rows, err := s.entClient.QueryContext(ctx, `SELECT t.id, COALESCE(p.name, ''),
 (SELECT COUNT(*) FROM reward_task_records r WHERE r.task_id=t.id AND r.status='in_progress' AND r.ticket_expires_at > `+fmt.Sprintf("$%d", len(args))+`),
 (SELECT COALESCE(SUM(r.reward_amount),0)::double precision FROM reward_task_records r WHERE r.task_id=t.id AND r.status='completed')
 FROM reward_tasks t LEFT JOIN reward_task_partners p ON p.id=t.partner_id WHERE t.id IN (`+strings.Join(ids, ",")+`)`, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	byID := make(map[int64]int, len(items))
	for i := range items {
		byID[items[i].ID] = i
	}
	for rows.Next() {
		var id, inProgress int64
		var name string
		var reward float64
		if err := rows.Scan(&id, &name, &inProgress, &reward); err != nil {
			return nil, err
		}
		i := byID[id]
		items[i].PartnerName, items[i].InProgressCount, items[i].TotalReward = name, inProgress, reward
	}
	return items, rows.Err()
}

func rewardLockPartner(ctx context.Context, tx *dbent.Tx, id *int64) error {
	if id == nil {
		return nil
	}
	_, err := tx.RewardTaskPartner.Query().Where(rewardtaskpartner.IDEQ(*id), rewardtaskpartner.DeletedAtIsNil()).ForUpdate().Only(ctx)
	if dbent.IsNotFound(err) {
		return rewardTaskError("REWARD_PARTNER_NOT_FOUND")
	}
	return err
}

func (s *RewardTaskService) CreateAdminTask(ctx context.Context, in RewardTaskInput) (*AdminRewardTask, error) {
	if err := ValidateRewardTaskInput(in); err != nil {
		return nil, err
	}
	if err := s.rewardAdminReady(); err != nil {
		return nil, err
	}
	in = normalizeRewardTaskInput(in)
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	if err := rewardLockPartner(ctx, tx, in.PartnerID); err != nil {
		return nil, err
	}
	task, err := tx.RewardTask.Create().SetTitle(in.Title).SetDescription(in.Description).SetTaskType(in.TaskType).SetVerifyMode(in.VerifyMode).
		SetNillablePartnerID(in.PartnerID).SetTargetURL(in.TargetURL).SetIconURL(in.IconURL).SetRewardAmount(in.RewardAmount).SetPeriod(in.Period).
		SetBrowseSeconds(in.BrowseSeconds).SetIdleTimeoutSeconds(in.IdleTimeoutSeconds).SetMinRechargeAmount(in.MinRechargeAmount).SetMaxCompletions(in.MaxCompletions).
		SetNillableStartsAt(in.StartsAt).SetNillableEndsAt(in.EndsAt).SetEnabled(in.Enabled).SetSortOrder(in.SortOrder).Save(ctx)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	items, err := s.rewardAdminTaskViews(ctx, []*dbent.RewardTask{task})
	if err != nil {
		return nil, err
	}
	return &items[0], nil
}

func (s *RewardTaskService) UpdateAdminTask(ctx context.Context, id int64, in RewardTaskInput) (*AdminRewardTask, error) {
	if err := s.rewardAdminReady(); err != nil {
		return nil, err
	}
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	existing, err := tx.RewardTask.Query().Where(rewardtask.IDEQ(id), rewardtask.DeletedAtIsNil()).ForUpdate().Only(ctx)
	if dbent.IsNotFound(err) {
		return nil, rewardTaskError("REWARD_TASK_NOT_FOUND")
	}
	if err != nil {
		return nil, err
	}
	if existing.TaskType != in.TaskType {
		return nil, rewardTaskError("REWARD_TASK_TYPE_IMMUTABLE")
	}
	if err := ValidateRewardTaskInput(in); err != nil {
		return nil, err
	}
	if err := rewardLockPartner(ctx, tx, in.PartnerID); err != nil {
		return nil, err
	}
	in = normalizeRewardTaskInput(in)
	update := tx.RewardTask.UpdateOneID(id).SetTitle(in.Title).SetDescription(in.Description).SetVerifyMode(in.VerifyMode).
		SetTargetURL(in.TargetURL).SetIconURL(in.IconURL).SetRewardAmount(in.RewardAmount).SetPeriod(in.Period).SetBrowseSeconds(in.BrowseSeconds).SetIdleTimeoutSeconds(in.IdleTimeoutSeconds).
		SetMinRechargeAmount(in.MinRechargeAmount).SetMaxCompletions(in.MaxCompletions).SetEnabled(in.Enabled).SetSortOrder(in.SortOrder)
	if in.PartnerID == nil {
		update.ClearPartnerID()
	} else {
		update.SetPartnerID(*in.PartnerID)
	}
	if in.StartsAt == nil {
		update.ClearStartsAt()
	} else {
		update.SetStartsAt(*in.StartsAt)
	}
	if in.EndsAt == nil {
		update.ClearEndsAt()
	} else {
		update.SetEndsAt(*in.EndsAt)
	}
	task, err := update.Save(ctx)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	items, err := s.rewardAdminTaskViews(ctx, []*dbent.RewardTask{task})
	if err != nil {
		return nil, err
	}
	return &items[0], nil
}

func (s *RewardTaskService) DeleteAdminTask(ctx context.Context, id int64) error {
	if err := s.rewardAdminReady(); err != nil {
		return err
	}
	now := time.Now()
	count, err := s.entClient.RewardTask.Update().Where(rewardtask.IDEQ(id), rewardtask.DeletedAtIsNil()).SetDeletedAt(now).SetUpdatedAt(now).Save(ctx)
	if err == nil && count == 0 {
		return rewardTaskError("REWARD_TASK_NOT_FOUND")
	}
	return err
}

func (s *RewardTaskService) GetAdminStats(ctx context.Context) (*RewardTaskAdminStats, error) {
	if err := s.rewardAdminReady(); err != nil {
		return nil, err
	}
	now := time.Now().In(time.Local)
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	end := start.AddDate(0, 0, 1)
	stats := new(RewardTaskAdminStats)
	if err := s.rewardAdminQueryRow(ctx, `SELECT
 (SELECT COUNT(*) FROM reward_tasks WHERE deleted_at IS NULL),
 (SELECT COUNT(*) FROM reward_tasks WHERE deleted_at IS NULL AND enabled),
 COUNT(*) FILTER (WHERE completed_at >= $1 AND completed_at < $2),
 COALESCE(SUM(reward_amount) FILTER (WHERE completed_at >= $1 AND completed_at < $2),0)::double precision,
 COUNT(*), COALESCE(SUM(reward_amount),0)::double precision,
 (SELECT COUNT(*) FROM reward_task_partners WHERE deleted_at IS NULL)
 FROM reward_task_records WHERE status='completed'`, []any{start, end}, &stats.TotalTasks, &stats.EnabledTasks, &stats.CompletedToday, &stats.RewardToday, &stats.CompletedTotal, &stats.RewardTotal, &stats.Partners); err != nil {
		return nil, err
	}
	return stats, nil
}

func (s *RewardTaskService) rewardAdminPartnerViews(ctx context.Context, partners []*dbent.RewardTaskPartner) ([]AdminRewardTaskPartner, error) {
	items := make([]AdminRewardTaskPartner, 0, len(partners))
	if len(partners) == 0 {
		return items, nil
	}
	ids, args := make([]string, 0, len(partners)), make([]any, 0, len(partners))
	byID := make(map[int64]int, len(partners))
	for _, partner := range partners {
		byID[partner.ID] = len(items)
		domains := partner.AllowedDomains
		if domains == nil {
			domains = []string{}
		}
		items = append(items, AdminRewardTaskPartner{RewardTaskPartnerInput: RewardTaskPartnerInput{Name: partner.Name, AllowedDomains: domains, Contact: partner.Contact, Notes: partner.Notes, Enabled: partner.Enabled}, ID: partner.ID, AppKey: partner.AppKey, SecretHint: partner.SecretHint, LastEventAt: partner.LastEventAt, CreatedAt: partner.CreatedAt, UpdatedAt: partner.UpdatedAt})
		args = append(args, partner.ID)
		ids = append(ids, fmt.Sprintf("$%d", len(args)))
	}
	rows, err := s.entClient.QueryContext(ctx, `SELECT partner_id, COUNT(*) FROM reward_tasks WHERE deleted_at IS NULL AND partner_id IN (`+strings.Join(ids, ",")+`) GROUP BY partner_id`, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var id, count int64
		if err := rows.Scan(&id, &count); err != nil {
			return nil, err
		}
		items[byID[id]].TaskCount = count
	}
	return items, rows.Err()
}

func (s *RewardTaskService) ListAdminPartners(ctx context.Context) ([]AdminRewardTaskPartner, error) {
	if err := s.rewardAdminReady(); err != nil {
		return nil, err
	}
	partners, err := s.entClient.RewardTaskPartner.Query().Where(rewardtaskpartner.DeletedAtIsNil()).Order(dbent.Desc(rewardtaskpartner.FieldID)).All(ctx)
	if err != nil {
		return nil, err
	}
	return s.rewardAdminPartnerViews(ctx, partners)
}

func (s *RewardTaskService) newRewardPartnerSecret() (string, string, error) {
	if s.encryptor == nil {
		return "", "", infraerrors.InternalServer("REWARD_TASK_UNAVAILABLE", "partner secret encryption is unavailable")
	}
	secret, err := rewardRandomToken("rs_", 24)
	if err != nil {
		return "", "", err
	}
	encrypted, err := s.encryptor.Encrypt(secret)
	if err != nil {
		return "", "", err
	}
	return secret, encrypted, nil
}

func (s *RewardTaskService) CreateAdminPartner(ctx context.Context, in RewardTaskPartnerInput) (*AdminRewardTaskPartner, error) {
	var err error
	in, err = normalizeRewardPartnerInput(in)
	if err != nil {
		return nil, err
	}
	if err := s.rewardAdminReady(); err != nil {
		return nil, err
	}
	appKey, err := rewardRandomToken("rk_", 12)
	if err != nil {
		return nil, err
	}
	secret, encrypted, err := s.newRewardPartnerSecret()
	if err != nil {
		return nil, err
	}
	partner, err := s.entClient.RewardTaskPartner.Create().SetName(in.Name).SetAppKey(appKey).SetAppSecretEncrypted(encrypted).
		SetSecretHint(secret[len(secret)-4:]).SetAllowedDomains(in.AllowedDomains).SetContact(in.Contact).SetNotes(in.Notes).SetEnabled(in.Enabled).Save(ctx)
	if err != nil {
		return nil, err
	}
	items, err := s.rewardAdminPartnerViews(ctx, []*dbent.RewardTaskPartner{partner})
	if err != nil {
		return nil, err
	}
	items[0].AppSecret = secret
	return &items[0], nil
}

func (s *RewardTaskService) UpdateAdminPartner(ctx context.Context, id int64, in RewardTaskPartnerInput) (*AdminRewardTaskPartner, error) {
	var err error
	in, err = normalizeRewardPartnerInput(in)
	if err != nil {
		return nil, err
	}
	if err := s.rewardAdminReady(); err != nil {
		return nil, err
	}
	partner, err := s.entClient.RewardTaskPartner.UpdateOneID(id).Where(rewardtaskpartner.DeletedAtIsNil()).
		SetName(in.Name).SetAllowedDomains(in.AllowedDomains).SetContact(in.Contact).SetNotes(in.Notes).SetEnabled(in.Enabled).Save(ctx)
	if dbent.IsNotFound(err) {
		return nil, rewardTaskError("REWARD_PARTNER_NOT_FOUND")
	}
	if err != nil {
		return nil, err
	}
	items, err := s.rewardAdminPartnerViews(ctx, []*dbent.RewardTaskPartner{partner})
	if err != nil {
		return nil, err
	}
	return &items[0], nil
}

func (s *RewardTaskService) ResetAdminPartnerSecret(ctx context.Context, id int64) (*AdminRewardTaskPartner, error) {
	if err := s.rewardAdminReady(); err != nil {
		return nil, err
	}
	secret, encrypted, err := s.newRewardPartnerSecret()
	if err != nil {
		return nil, err
	}
	partner, err := s.entClient.RewardTaskPartner.UpdateOneID(id).Where(rewardtaskpartner.DeletedAtIsNil()).SetAppSecretEncrypted(encrypted).SetSecretHint(secret[len(secret)-4:]).Save(ctx)
	if dbent.IsNotFound(err) {
		return nil, rewardTaskError("REWARD_PARTNER_NOT_FOUND")
	}
	if err != nil {
		return nil, err
	}
	items, err := s.rewardAdminPartnerViews(ctx, []*dbent.RewardTaskPartner{partner})
	if err != nil {
		return nil, err
	}
	items[0].AppSecret = secret
	return &items[0], nil
}

func (s *RewardTaskService) DeleteAdminPartner(ctx context.Context, id int64) error {
	if err := s.rewardAdminReady(); err != nil {
		return err
	}
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := rewardLockPartner(ctx, tx, &id); err != nil {
		return err
	}
	inUse, err := tx.RewardTask.Query().Where(rewardtask.PartnerIDEQ(id), rewardtask.DeletedAtIsNil()).Exist(ctx)
	if err != nil {
		return err
	}
	if inUse {
		return rewardTaskError("REWARD_PARTNER_IN_USE")
	}
	now := time.Now()
	if _, err := tx.RewardTaskPartner.UpdateOneID(id).SetDeletedAt(now).SetUpdatedAt(now).Save(ctx); err != nil {
		return err
	}
	return tx.Commit()
}

func rewardAdminWhere(conditions []string) string {
	if len(conditions) == 0 {
		return ""
	}
	return " WHERE " + strings.Join(conditions, " AND ")
}

func (s *RewardTaskService) ListAdminRecords(ctx context.Context, filter RewardTaskRecordAdminFilter) ([]AdminRewardTaskRecord, int64, error) {
	if err := s.rewardAdminReady(); err != nil {
		return nil, 0, err
	}
	if filter.Status != "" && filter.Status != "in_progress" && filter.Status != "completed" {
		return nil, 0, rewardTaskInputError("status must be in_progress or completed")
	}
	filter.Page, filter.PageSize = rewardAdminPagination(filter.Page, filter.PageSize)
	args, conditions := make([]any, 0, 5), make([]string, 0, 3)
	if filter.TaskID > 0 {
		args = append(args, filter.TaskID)
		conditions = append(conditions, fmt.Sprintf("r.task_id=$%d", len(args)))
	}
	if filter.Status != "" {
		args = append(args, filter.Status)
		conditions = append(conditions, fmt.Sprintf("r.status=$%d", len(args)))
	}
	if search := strings.TrimSpace(filter.Search); search != "" {
		args = append(args, "%"+strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(search)+"%")
		conditions = append(conditions, fmt.Sprintf("(u.email ILIKE $%[1]d OR u.username ILIKE $%[1]d OR r.user_id::text ILIKE $%[1]d)", len(args)))
	}
	// 历史记录保留软删任务及用户的信息，不能应用默认软删过滤。
	from := " FROM reward_task_records r LEFT JOIN reward_tasks t ON t.id=r.task_id LEFT JOIN users u ON u.id=r.user_id"
	where := rewardAdminWhere(conditions)
	var total int64
	if err := s.rewardAdminQueryRow(ctx, "SELECT COUNT(*)"+from+where, args, &total); err != nil {
		return nil, 0, err
	}
	args = append(args, filter.PageSize, (filter.Page-1)*filter.PageSize)
	rows, err := s.entClient.QueryContext(ctx, `SELECT r.id, r.task_id, COALESCE(NULLIF(r.task_title,''),t.title,''), COALESCE(t.task_type,''),
 r.user_id, COALESCE(u.email,''), COALESCE(u.username,''), r.period_key, r.status, r.progress_ms, COALESCE(t.browse_seconds,0),
 r.recharge_total::double precision, r.external_user_id, r.started_at, r.ticket_expires_at, r.completed_at, r.reward_amount::double precision`+
		from+where+fmt.Sprintf(" ORDER BY r.id DESC LIMIT $%d OFFSET $%d", len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]AdminRewardTaskRecord, 0, filter.PageSize)
	for rows.Next() {
		var item AdminRewardTaskRecord
		var progressMs int64
		var browseSeconds int
		if err := rows.Scan(&item.ID, &item.TaskID, &item.TaskTitle, &item.TaskType, &item.UserID, &item.UserEmail, &item.Username, &item.PeriodKey, &item.Status, &progressMs, &browseSeconds, &item.RechargeTotal, &item.ExternalUserID, &item.StartedAt, &item.TicketExpiresAt, &item.CompletedAt, &item.RewardAmount); err != nil {
			return nil, 0, err
		}
		item.ProgressSeconds = rewardProgressSeconds(progressMs, browseSeconds, item.Status == "completed")
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func (s *RewardTaskService) ListAdminEvents(ctx context.Context, filter RewardTaskEventAdminFilter) ([]AdminRewardTaskEvent, int64, error) {
	if err := s.rewardAdminReady(); err != nil {
		return nil, 0, err
	}
	if filter.Result != "" && filter.Result != "accepted" && filter.Result != "completed" && filter.Result != "ignored" && filter.Result != "rejected" {
		return nil, 0, rewardTaskInputError("result must be accepted, completed, ignored or rejected")
	}
	filter.Page, filter.PageSize = rewardAdminPagination(filter.Page, filter.PageSize)
	args, conditions := make([]any, 0, 4), make([]string, 0, 2)
	if filter.PartnerID > 0 {
		args = append(args, filter.PartnerID)
		conditions = append(conditions, fmt.Sprintf("e.partner_id=$%d", len(args)))
	}
	if filter.Result != "" {
		args = append(args, filter.Result)
		conditions = append(conditions, fmt.Sprintf("e.result=$%d", len(args)))
	}
	from := " FROM reward_task_events e LEFT JOIN reward_task_partners p ON p.id=e.partner_id"
	where := rewardAdminWhere(conditions)
	var total int64
	if err := s.rewardAdminQueryRow(ctx, "SELECT COUNT(*)"+from+where, args, &total); err != nil {
		return nil, 0, err
	}
	args = append(args, filter.PageSize, (filter.Page-1)*filter.PageSize)
	rows, err := s.entClient.QueryContext(ctx, `SELECT e.id, e.partner_id, COALESCE(p.name,''), e.event_id, e.event_type, e.ticket,
 e.task_id, e.user_id, e.external_user_id, e.amount::double precision, e.order_id, e.result, e.error_code, e.remote_ip, e.payload, e.created_at`+
		from+where+fmt.Sprintf(" ORDER BY e.id DESC LIMIT $%d OFFSET $%d", len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]AdminRewardTaskEvent, 0, filter.PageSize)
	for rows.Next() {
		var item AdminRewardTaskEvent
		if err := rows.Scan(&item.ID, &item.PartnerID, &item.PartnerName, &item.EventID, &item.EventType, &item.Ticket, &item.TaskID, &item.UserID, &item.ExternalUserID, &item.Amount, &item.OrderID, &item.Result, &item.ErrorCode, &item.RemoteIP, &item.Payload, &item.CreatedAt); err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}
