package service

import (
	"context"
	cryptorand "crypto/rand"
	"errors"
	"fmt"
	"math"
	"math/big"
	"strings"
	"time"

	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/usercheckin"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

var (
	ErrCheckInDisabled        = infraerrors.BadRequest("CHECK_IN_DISABLED", "check-in is disabled")
	ErrCheckInAlreadyDone     = infraerrors.Conflict("CHECK_IN_ALREADY_DONE", "check-in already completed today")
	ErrCheckInCaptchaRequired = infraerrors.BadRequest("CHECK_IN_CAPTCHA_REQUIRED", "captcha verification is required")
	ErrCheckInInvalidTiers    = infraerrors.BadRequest("CHECK_IN_INVALID_TIERS", "check-in tiers are invalid or unavailable")
)

// checkInHistoryLimit 限制状态查询拉取的记录条数：连续天数最多统计一年，更早的明细用不上。
const checkInHistoryLimit = 366

// CaptchaVerifier is implemented by AuthService and keeps check-in independent
// from the concrete Turnstile/Tencent/Aliyun implementations.
type CaptchaVerifier interface {
	VerifyCaptcha(ctx context.Context, proof CaptchaProof, remoteIP string) error
}

type CheckInService struct {
	entClient            *dbent.Client
	userRepo             UserRepository
	settingService       *SettingService
	captchaVerifier      CaptchaVerifier
	enterpriseMembership EnterpriseMembershipReader
	billingCacheService  *BillingCacheService
	authCacheInvalidator APIKeyAuthCacheInvalidator
	randomFloat          func() (float64, error)
}

func NewCheckInService(
	entClient *dbent.Client,
	userRepo UserRepository,
	settingService *SettingService,
	captchaVerifier CaptchaVerifier,
	enterpriseMembership EnterpriseMembershipReader,
	billingCacheService *BillingCacheService,
	authCacheInvalidator APIKeyAuthCacheInvalidator,
) *CheckInService {
	return &CheckInService{
		entClient:            entClient,
		userRepo:             userRepo,
		settingService:       settingService,
		captchaVerifier:      captchaVerifier,
		enterpriseMembership: enterpriseMembership,
		billingCacheService:  billingCacheService,
		authCacheInvalidator: authCacheInvalidator,
		randomFloat:          cryptoRandomFloat,
	}
}

type CheckInRecordView struct {
	Date   string  `json:"date"`
	Reward float64 `json:"reward"`
}

// CheckInCalendarDay 是月历中的一天：reward 非 nil 表示当天已签到。
type CheckInCalendarDay struct {
	Date   string   `json:"date"`
	Reward *float64 `json:"reward"`
}

// CheckInCalendar 是某一自然月的签到记录，供前端渲染日历。
type CheckInCalendar struct {
	Month       string               `json:"month"` // YYYY-MM
	Today       string               `json:"today"` // 服务器时区的今天
	Days        []CheckInCalendarDay `json:"days"`
	CheckedDays int                  `json:"checked_days"`
	TotalReward float64              `json:"total_reward"`
}

type CheckInStatus struct {
	Enabled         bool    `json:"enabled"`
	CaptchaRequired bool    `json:"captcha_required"`
	CheckedInToday  bool    `json:"checked_in_today"`
	TodayReward     float64 `json:"today_reward"`
	TodayDate       string  `json:"today_date"`
	StreakDays      int     `json:"streak_days"`
	NextResetAt     string  `json:"next_reset_at"`
	// 只下发所有档位聚合后的奖励区间，不暴露各档权重与概率。
	RewardMin     float64             `json:"reward_min"`
	RewardMax     float64             `json:"reward_max"`
	RecentRecords []CheckInRecordView `json:"recent_records"`
}

type CheckInResult struct {
	Reward      float64 `json:"reward"`
	Balance     float64 `json:"balance"`
	CheckInDate string  `json:"check_in_date"`
	StreakDays  int     `json:"streak_days"`
}

// serverToday 返回服务器时区（timezone.Init 设置的 time.Local）的当天零点与日期字符串。
// 「每天」的边界完全由服务器时区决定，与前端浏览器时区无关。
func serverToday(now time.Time) (time.Time, string) {
	local := now.In(time.Local)
	date := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.Local)
	return date, date.Format("2006-01-02")
}

// formatCheckInDate 取 time.Time 自身的年月日分量，不做时区换算。
// check_in_date 是 Postgres DATE 列，lib/pq 读回的是该日期的零点；而 DSN 的
// TimeZone 与会话时区同源于 cfg.Timezone（见 repository.InitEnt），因此写入与
// 读回的自然日一致。切勿在此处改成 In(time.Local) 换算，否则在负时区会回退一天。
func formatCheckInDate(value time.Time) string {
	year, month, day := value.Date()
	return fmt.Sprintf("%04d-%02d-%02d", year, month, day)
}

func (s *CheckInService) loadConfig(ctx context.Context) (*SystemSettings, bool, error) {
	if s == nil || s.settingService == nil {
		return nil, false, ErrCheckInInvalidTiers
	}
	settings, err := s.settingService.GetAllSettings(ctx)
	if err != nil {
		return nil, false, err
	}
	if settings.CheckInEnabled && len(settings.CheckInTiers) == 0 {
		return settings, false, ErrCheckInInvalidTiers
	}
	captchaRequired := false
	if settings.CheckInCaptchaEnabled {
		providerConfig, err := s.settingService.GetCaptchaProviderConfig(ctx)
		if err != nil {
			return nil, false, err
		}
		captchaRequired = checkInHasEnabledProvider(providerConfig)
	}
	return settings, captchaRequired, nil
}

func (s *CheckInService) GetStatus(ctx context.Context, userID int64) (*CheckInStatus, error) {
	settings, captchaRequired, err := s.loadConfig(ctx)
	if err != nil {
		// Disabled check-in deliberately keeps returning an empty status payload.
		if errors.Is(err, ErrCheckInInvalidTiers) && settings != nil && !settings.CheckInEnabled {
			captchaRequired = false
		} else {
			return nil, err
		}
	}
	today, todayDate := serverToday(time.Now())
	nextReset := today.AddDate(0, 0, 1)
	status := &CheckInStatus{
		Enabled:         settings != nil && settings.CheckInEnabled,
		CaptchaRequired: captchaRequired,
		TodayDate:       todayDate,
		NextResetAt:     nextReset.Format(time.RFC3339),
		RecentRecords:   []CheckInRecordView{},
	}
	if settings != nil && settings.CheckInEnabled {
		status.RewardMin, status.RewardMax = checkInRewardRange(settings.CheckInTiers)
	}
	if s.entClient == nil {
		return status, nil
	}
	// 只取最近一年的记录：满足连续天数与最近 7 条展示所需，避免历史越长查询越慢。
	records, err := s.entClient.UserCheckIn.Query().Where(usercheckin.UserID(userID)).Order(usercheckin.ByCheckInDate(entsql.OrderDesc())).Limit(checkInHistoryLimit).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("query check-in records: %w", err)
	}
	if status.Enabled {
		status.StreakDays = checkInStreak(records, today)
	}
	for i, record := range records {
		date := formatCheckInDate(record.CheckInDate)
		if status.Enabled && date == todayDate {
			status.CheckedInToday = true
			status.TodayReward = record.Reward
		}
		if i < 7 {
			status.RecentRecords = append(status.RecentRecords, CheckInRecordView{Date: date, Reward: record.Reward})
		}
	}
	return status, nil
}

func (s *CheckInService) GetMonthCalendar(ctx context.Context, userID int64, month string) (*CheckInCalendar, error) {
	_, todayDate := serverToday(time.Now())

	// 未指定月份时默认当前月；格式必须为 YYYY-MM，避免拼进 SQL 之外的歧义。
	month = strings.TrimSpace(month)
	if month == "" {
		month = todayDate[:7]
	}
	first, err := time.ParseInLocation("2006-01", month, time.Local)
	if err != nil {
		return nil, infraerrors.BadRequest("CHECK_IN_INVALID_MONTH", "month must be in YYYY-MM format")
	}
	next := first.AddDate(0, 1, 0)

	calendar := &CheckInCalendar{
		Month: first.Format("2006-01"),
		Today: todayDate,
		Days:  []CheckInCalendarDay{},
	}
	if s.entClient == nil {
		return calendar, nil
	}
	records, err := s.entClient.UserCheckIn.Query().
		Where(
			usercheckin.UserID(userID),
			usercheckin.CheckInDateGTE(first),
			usercheckin.CheckInDateLT(next),
		).
		Order(usercheckin.ByCheckInDate()).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("query check-in calendar: %w", err)
	}
	for _, record := range records {
		reward := record.Reward
		calendar.Days = append(calendar.Days, CheckInCalendarDay{
			Date:   formatCheckInDate(record.CheckInDate),
			Reward: &reward,
		})
		calendar.TotalReward += reward
	}
	calendar.CheckedDays = len(calendar.Days)
	return calendar, nil
}

func checkInStreak(records []*dbent.UserCheckIn, today time.Time) int {
	if len(records) == 0 {
		return 0
	}
	dateSet := make(map[string]struct{}, len(records))
	for _, record := range records {
		dateSet[formatCheckInDate(record.CheckInDate)] = struct{}{}
	}
	start := today
	if _, ok := dateSet[start.Format("2006-01-02")]; !ok {
		start = start.AddDate(0, 0, -1)
	}
	streak := 0
	for {
		if _, ok := dateSet[start.Format("2006-01-02")]; !ok {
			break
		}
		streak++
		start = start.AddDate(0, 0, -1)
	}
	return streak
}

func (s *CheckInService) verifyCaptchaIfRequired(ctx context.Context, captchaRequired bool, proof CaptchaProof, remoteIP string) error {
	// 未要求验证（含「开关打开但没有任何服务商可用」）时直接放行。
	if !captchaRequired {
		return nil
	}
	if strings.TrimSpace(proof.TurnstileToken) == "" && strings.TrimSpace(proof.TencentTicket) == "" && strings.TrimSpace(proof.TencentRandstr) == "" {
		return ErrCheckInCaptchaRequired
	}
	if s.captchaVerifier == nil {
		return infraerrors.InternalServer("CHECK_IN_CAPTCHA_UNAVAILABLE", "captcha verifier is unavailable")
	}
	return s.captchaVerifier.VerifyCaptcha(ctx, proof, remoteIP)
}

func (s *CheckInService) PerformCheckIn(ctx context.Context, userID int64, proof CaptchaProof, remoteIP string) (*CheckInResult, error) {
	settings, captchaRequired, err := s.loadConfig(ctx)
	if err != nil {
		return nil, err
	}
	if !settings.CheckInEnabled {
		return nil, ErrCheckInDisabled
	}
	// R1：只有真正要求验证时才校验。开关打开但一家验证码服务商都没启用时
	// captchaRequired 为 false，直接放行签到（配置缺失不应阻断用户领取奖励）。
	if err := s.verifyCaptchaIfRequired(ctx, captchaRequired, proof, remoteIP); err != nil {
		return nil, err
	}
	if err := checkEnterprisePersonalWallet(ctx, s.enterpriseMembership, userID); err != nil {
		return nil, err
	}

	today, todayDate := serverToday(time.Now())
	tierIndex, reward, err := s.drawReward(settings.CheckInTiers)
	if err != nil {
		return nil, ErrCheckInInvalidTiers.WithCause(err)
	}
	if s.entClient == nil || s.userRepo == nil {
		return nil, infraerrors.InternalServer("CHECK_IN_UNAVAILABLE", "check-in service is unavailable")
	}
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin check-in transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	txCtx := dbent.NewTxContext(ctx, tx)
	if _, err := tx.UserCheckIn.Create().SetUserID(userID).SetCheckInDate(today).SetReward(reward).SetTierIndex(tierIndex).Save(txCtx); err != nil {
		if dbent.IsConstraintError(err) {
			return nil, ErrCheckInAlreadyDone
		}
		return nil, fmt.Errorf("insert check-in record: %w", err)
	}
	// 签到额度只增加余额，不计入 total_recharged（累计充值）：它是签到奖励而不是充值，
	// 计入会抬高按累计充值百分比计算的余额不足提醒阈值。也不做非负校验，透支用户同样可以签到。
	if err := tx.User.UpdateOneID(userID).AddBalance(reward).Exec(txCtx); err != nil {
		if dbent.IsNotFound(err) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("credit check-in balance: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit check-in transaction: %w", err)
	}
	s.invalidateBalanceCaches(ctx, userID)
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("reload check-in balance: %w", err)
	}
	status, err := s.GetStatus(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &CheckInResult{Reward: reward, Balance: user.Balance, CheckInDate: todayDate, StreakDays: status.StreakDays}, nil
}

// invalidateBalanceCaches 与兑换码充值一致：余额变更后清理鉴权缓存与计费余额缓存，
// 否则网关在缓存过期前仍按旧余额校验，刚领到的签到额度无法立即使用。
func (s *CheckInService) invalidateBalanceCaches(ctx context.Context, userID int64) {
	if s.authCacheInvalidator != nil {
		s.authCacheInvalidator.InvalidateAuthCacheByUserID(ctx, userID)
	}
	if s.billingCacheService == nil {
		return
	}
	go func() {
		cacheCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = s.billingCacheService.InvalidateUserBalance(cacheCtx, userID)
	}()
}

func (s *CheckInService) drawReward(tiers []CheckInTier) (int, float64, error) {
	valid, err := NormalizeCheckInTiers(tiers)
	if err != nil || len(valid) == 0 {
		if err == nil {
			err = errors.New("no usable check-in tiers")
		}
		return 0, 0, err
	}
	var total float64
	for _, tier := range valid {
		total += tier.Weight
	}
	if !checkInNumberValid(total) || total <= 0 {
		return 0, 0, errors.New("total check-in tier weight must be greater than zero")
	}
	randFn := s.randomFloat
	if randFn == nil {
		randFn = cryptoRandomFloat
	}
	r, err := randFn()
	if err != nil {
		return 0, 0, fmt.Errorf("generate tier random value: %w", err)
	}
	if r < 0 || r >= 1 || math.IsNaN(r) {
		return 0, 0, errors.New("random value must be in [0,1)")
	}
	target := r * total
	cumulative := 0.0
	selected := len(valid) - 1
	for i, tier := range valid {
		cumulative += tier.Weight
		if cumulative > target {
			selected = i
			break
		}
	}
	tier := valid[selected]
	if tier.Min == tier.Max {
		return selected, tier.Min, nil
	}
	r, err = randFn()
	if err != nil {
		return 0, 0, fmt.Errorf("generate reward random value: %w", err)
	}
	if r < 0 || r >= 1 || math.IsNaN(r) {
		return 0, 0, errors.New("random value must be in [0,1)")
	}
	reward := tier.Min + r*(tier.Max-tier.Min)
	reward = math.Round(reward*1e4) / 1e4
	if reward < tier.Min {
		reward = tier.Min
	}
	if reward > tier.Max {
		reward = tier.Max
	}
	return selected, reward, nil
}

func cryptoRandomFloat() (float64, error) {
	const precision = uint64(1 << 53)
	n, err := cryptorand.Int(cryptorand.Reader, new(big.Int).SetUint64(precision))
	if err != nil {
		return 0, err
	}
	return float64(n.Uint64()) / float64(precision), nil
}
