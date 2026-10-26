package service

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// 管理端签到统计：概览、按日趋势、档位实际分布，以及全部签到明细。
// 所有金额均为「签到额度」（签到奖励发放到余额的部分），与充值无关。

const (
	defaultCheckInStatsDays = 30
	maxCheckInStatsDays     = 365
	maxCheckInRecordsPage   = 100
)

var ErrCheckInInvalidDate = infraerrors.BadRequest("CHECK_IN_INVALID_DATE", "date must be in YYYY-MM-DD format")

type CheckInStatsSummary struct {
	TodayDate           string  `json:"today_date"`
	TodayUsers          int64   `json:"today_users"`
	TodayReward         float64 `json:"today_reward"`
	Last7DaysUsers      int64   `json:"last_7_days_users"`
	TotalCheckIns       int64   `json:"total_check_ins"`
	TotalUsers          int64   `json:"total_users"`
	TotalReward         float64 `json:"total_reward"`
	AverageReward       float64 `json:"average_reward"`
	PeriodCheckIns      int64   `json:"period_check_ins"`
	PeriodReward        float64 `json:"period_reward"`
	PeriodStartDate     string  `json:"period_start_date"`
	CheckInEnabled      bool    `json:"check_in_enabled"`
	CaptchaEnabled      bool    `json:"captcha_enabled"`
	ConfiguredTiers     int     `json:"configured_tiers"`
	ConfiguredRewardMin float64 `json:"configured_reward_min"`
	ConfiguredRewardMax float64 `json:"configured_reward_max"`
}

type CheckInDailyStat struct {
	Date   string  `json:"date"`
	Users  int64   `json:"users"`
	Reward float64 `json:"reward"`
}

// CheckInTierStat 统计周期内每个档位的实际抽中情况；Configured* 取自当前配置，
// 若档位配置在统计周期内被修改过，同一 tier_index 的含义可能不同。
type CheckInTierStat struct {
	TierIndex         int      `json:"tier_index"`
	Count             int64    `json:"count"`
	Reward            float64  `json:"reward"`
	ActualPercent     float64  `json:"actual_percent"`
	ObservedMin       float64  `json:"observed_min"`
	ObservedMax       float64  `json:"observed_max"`
	ConfiguredMin     *float64 `json:"configured_min"`
	ConfiguredMax     *float64 `json:"configured_max"`
	ConfiguredPercent *float64 `json:"configured_percent"`
}

type CheckInStats struct {
	Days    int                 `json:"days"`
	Summary CheckInStatsSummary `json:"summary"`
	Daily   []CheckInDailyStat  `json:"daily"`
	Tiers   []CheckInTierStat   `json:"tiers"`
}

type CheckInRecordFilter struct {
	Search    string
	StartDate string // YYYY-MM-DD，服务器时区签到日，含当天
	EndDate   string // YYYY-MM-DD，含当天
	Page      int
	PageSize  int
	SortBy    string
	SortDesc  bool
}

type AdminCheckInRecord struct {
	ID          int64     `json:"id"`
	UserID      int64     `json:"user_id"`
	UserEmail   string    `json:"user_email"`
	Username    string    `json:"username"`
	CheckInDate string    `json:"check_in_date"`
	Reward      float64   `json:"reward"`
	TierIndex   int       `json:"tier_index"`
	CreatedAt   time.Time `json:"created_at"`
}

func normalizeCheckInStatsDays(days int) int {
	if days <= 0 {
		return defaultCheckInStatsDays
	}
	if days > maxCheckInStatsDays {
		return maxCheckInStatsDays
	}
	return days
}

func (s *CheckInService) GetAdminStats(ctx context.Context, days int) (*CheckInStats, error) {
	if s == nil || s.entClient == nil {
		return nil, infraerrors.InternalServer("CHECK_IN_UNAVAILABLE", "check-in service is unavailable")
	}
	days = normalizeCheckInStatsDays(days)
	today, todayDate := serverToday(time.Now())
	start := today.AddDate(0, 0, -(days - 1))
	startDate := start.Format("2006-01-02")
	last7StartDate := today.AddDate(0, 0, -6).Format("2006-01-02")

	stats := &CheckInStats{Days: days, Daily: []CheckInDailyStat{}, Tiers: []CheckInTierStat{}}
	summary := &stats.Summary
	summary.TodayDate = todayDate
	summary.PeriodStartDate = startDate

	if err := s.queryRow(ctx, `
SELECT COUNT(*),
       COUNT(DISTINCT user_id),
       COALESCE(SUM(reward), 0)::double precision,
       COUNT(*) FILTER (WHERE check_in_date = $1::date),
       COALESCE(SUM(reward) FILTER (WHERE check_in_date = $1::date), 0)::double precision,
       COUNT(DISTINCT user_id) FILTER (WHERE check_in_date >= $2::date),
       COUNT(*) FILTER (WHERE check_in_date >= $3::date),
       COALESCE(SUM(reward) FILTER (WHERE check_in_date >= $3::date), 0)::double precision
FROM user_check_ins`, []any{todayDate, last7StartDate, startDate},
		&summary.TotalCheckIns, &summary.TotalUsers, &summary.TotalReward,
		&summary.TodayUsers, &summary.TodayReward, &summary.Last7DaysUsers,
		&summary.PeriodCheckIns, &summary.PeriodReward,
	); err != nil {
		return nil, fmt.Errorf("query check-in summary: %w", err)
	}
	if summary.TotalCheckIns > 0 {
		summary.AverageReward = summary.TotalReward / float64(summary.TotalCheckIns)
	}

	daily, err := s.queryDailyStats(ctx, startDate)
	if err != nil {
		return nil, err
	}
	stats.Daily = fillCheckInDailyStats(start, days, daily)

	var tiers []CheckInTier
	if s.settingService != nil {
		settings, err := s.settingService.GetAllSettings(ctx)
		if err != nil {
			return nil, err
		}
		tiers = settings.CheckInTiers
		summary.CheckInEnabled = settings.CheckInEnabled
		summary.CaptchaEnabled = settings.CheckInCaptchaEnabled
		summary.ConfiguredTiers = len(tiers)
		summary.ConfiguredRewardMin, summary.ConfiguredRewardMax = checkInRewardRange(tiers)
	}
	tierStats, err := s.queryTierStats(ctx, startDate)
	if err != nil {
		return nil, err
	}
	stats.Tiers = mergeCheckInTierStats(tierStats, tiers, summary.PeriodCheckIns)
	return stats, nil
}

func (s *CheckInService) queryRow(ctx context.Context, query string, args []any, dest ...any) error {
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

func (s *CheckInService) queryDailyStats(ctx context.Context, startDate string) (map[string]CheckInDailyStat, error) {
	rows, err := s.entClient.QueryContext(ctx, `
SELECT check_in_date, COUNT(*), COALESCE(SUM(reward), 0)::double precision
FROM user_check_ins
WHERE check_in_date >= $1::date
GROUP BY check_in_date`, startDate)
	if err != nil {
		return nil, fmt.Errorf("query check-in daily stats: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := map[string]CheckInDailyStat{}
	for rows.Next() {
		var date time.Time
		var item CheckInDailyStat
		if err := rows.Scan(&date, &item.Users, &item.Reward); err != nil {
			return nil, err
		}
		item.Date = formatCheckInDate(date)
		out[item.Date] = item
	}
	return out, rows.Err()
}

// fillCheckInDailyStats 按自然日铺满统计区间，没有签到的日子补 0，图表不会出现断档。
func fillCheckInDailyStats(start time.Time, days int, byDate map[string]CheckInDailyStat) []CheckInDailyStat {
	out := make([]CheckInDailyStat, 0, days)
	for i := 0; i < days; i++ {
		date := start.AddDate(0, 0, i).Format("2006-01-02")
		item, ok := byDate[date]
		if !ok {
			item = CheckInDailyStat{Date: date}
		}
		out = append(out, item)
	}
	return out
}

func (s *CheckInService) queryTierStats(ctx context.Context, startDate string) ([]CheckInTierStat, error) {
	rows, err := s.entClient.QueryContext(ctx, `
SELECT tier_index,
       COUNT(*),
       COALESCE(SUM(reward), 0)::double precision,
       COALESCE(MIN(reward), 0)::double precision,
       COALESCE(MAX(reward), 0)::double precision
FROM user_check_ins
WHERE check_in_date >= $1::date
GROUP BY tier_index
ORDER BY tier_index`, startDate)
	if err != nil {
		return nil, fmt.Errorf("query check-in tier stats: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := []CheckInTierStat{}
	for rows.Next() {
		var item CheckInTierStat
		if err := rows.Scan(&item.TierIndex, &item.Count, &item.Reward, &item.ObservedMin, &item.ObservedMax); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

// mergeCheckInTierStats 把实际抽中情况与当前档位配置对齐：当前配置里存在但周期内没被抽中的档位
// 也会出现（次数为 0），便于对比「配置概率」与「实际占比」。
func mergeCheckInTierStats(actual []CheckInTierStat, tiers []CheckInTier, periodTotal int64) []CheckInTierStat {
	byIndex := make(map[int]CheckInTierStat, len(actual))
	maxIndex := len(tiers) - 1
	for _, item := range actual {
		byIndex[item.TierIndex] = item
		if item.TierIndex > maxIndex {
			maxIndex = item.TierIndex
		}
	}
	percentages := checkInTierPercentages(tiers)
	out := make([]CheckInTierStat, 0, maxIndex+1)
	for index := 0; index <= maxIndex; index++ {
		item, seen := byIndex[index]
		item.TierIndex = index
		if index < len(tiers) {
			minReward, maxReward, percent := tiers[index].Min, tiers[index].Max, math.Round(percentages[index]*10)/10
			item.ConfiguredMin, item.ConfiguredMax, item.ConfiguredPercent = &minReward, &maxReward, &percent
		} else if !seen {
			continue
		}
		if periodTotal > 0 {
			item.ActualPercent = math.Round(float64(item.Count)/float64(periodTotal)*1000) / 10
		}
		out = append(out, item)
	}
	return out
}

func checkInTierPercentages(tiers []CheckInTier) []float64 {
	percentages := make([]float64, len(tiers))
	var total float64
	for _, tier := range tiers {
		total += tier.Weight
	}
	if total <= 0 || math.IsNaN(total) || math.IsInf(total, 0) {
		return percentages
	}
	for i, tier := range tiers {
		percentages[i] = tier.Weight / total * 100
	}
	return percentages
}

// parseCheckInFilterDate 校验 YYYY-MM-DD；返回规范化后的日期字符串，空串表示不过滤。
func parseCheckInFilterDate(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	parsed, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return "", ErrCheckInInvalidDate
	}
	return parsed.Format("2006-01-02"), nil
}

var checkInRecordSortColumns = map[string]string{
	"created_at":    "c.created_at",
	"check_in_date": "c.check_in_date",
	"reward":        "c.reward",
	"tier_index":    "c.tier_index",
	"user":          "u.email",
}

func (s *CheckInService) ListAdminRecords(ctx context.Context, filter CheckInRecordFilter) ([]AdminCheckInRecord, int64, error) {
	if s == nil || s.entClient == nil {
		return nil, 0, infraerrors.InternalServer("CHECK_IN_UNAVAILABLE", "check-in service is unavailable")
	}
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 {
		filter.PageSize = 20
	}
	if filter.PageSize > maxCheckInRecordsPage {
		filter.PageSize = maxCheckInRecordsPage
	}

	conditions := make([]string, 0, 3)
	args := make([]any, 0, 6)
	if search := strings.TrimSpace(filter.Search); search != "" {
		args = append(args, "%"+strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(search)+"%")
		conditions = append(conditions, fmt.Sprintf("(u.email ILIKE $%[1]d OR u.username ILIKE $%[1]d OR c.user_id::text ILIKE $%[1]d)", len(args)))
	}
	startDate, err := parseCheckInFilterDate(filter.StartDate)
	if err != nil {
		return nil, 0, err
	}
	if startDate != "" {
		args = append(args, startDate)
		conditions = append(conditions, fmt.Sprintf("c.check_in_date >= $%d::date", len(args)))
	}
	endDate, err := parseCheckInFilterDate(filter.EndDate)
	if err != nil {
		return nil, 0, err
	}
	if endDate != "" {
		args = append(args, endDate)
		conditions = append(conditions, fmt.Sprintf("c.check_in_date <= $%d::date", len(args)))
	}
	where := ""
	if len(conditions) > 0 {
		where = " WHERE " + strings.Join(conditions, " AND ")
	}
	// 用户被删除后签到记录仍需可查，因此 LEFT JOIN。
	from := " FROM user_check_ins c LEFT JOIN users u ON u.id = c.user_id"

	var total int64
	if err := s.queryRow(ctx, "SELECT COUNT(*)"+from+where, args, &total); err != nil {
		return nil, 0, fmt.Errorf("count check-in records: %w", err)
	}

	sortColumn, ok := checkInRecordSortColumns[filter.SortBy]
	if !ok {
		sortColumn = "c.created_at"
	}
	direction := "DESC"
	if !filter.SortDesc {
		direction = "ASC"
	}
	args = append(args, filter.PageSize, (filter.Page-1)*filter.PageSize)
	rows, err := s.entClient.QueryContext(ctx, `
SELECT c.id, c.user_id, COALESCE(u.email, ''), COALESCE(u.username, ''),
       c.check_in_date, c.reward::double precision, c.tier_index, c.created_at`+
		from+where+
		fmt.Sprintf(" ORDER BY %s %s, c.id %s LIMIT $%d OFFSET $%d", sortColumn, direction, direction, len(args)-1, len(args)),
		args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query check-in records: %w", err)
	}
	defer func() { _ = rows.Close() }()

	items := make([]AdminCheckInRecord, 0, filter.PageSize)
	for rows.Next() {
		var item AdminCheckInRecord
		var date time.Time
		if err := rows.Scan(&item.ID, &item.UserID, &item.UserEmail, &item.Username, &date, &item.Reward, &item.TierIndex, &item.CreatedAt); err != nil {
			return nil, 0, err
		}
		item.CheckInDate = formatCheckInDate(date)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}
