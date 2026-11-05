//go:build checkin_verify

package checkinverify

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	_ "github.com/Wei-Shaw/sub2api/ent/runtime"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/Wei-Shaw/sub2api/internal/service"
	_ "github.com/lib/pq"
)

// 真实 Postgres 端到端验证。需要手动准备一个空库并指定连接串：
//
//	CHECKIN_VERIFY_DSN="host=127.0.0.1 port=5432 user=sub2api password=sub2api dbname=checkin_verify sslmode=disable TimeZone=Asia/Shanghai" \
//	  go test -tags=checkin_verify -run TestCheckInEndToEndAgainstPostgres ./internal/checkinverify/
//
// 未设置 CHECKIN_VERIFY_DSN 时跳过，避免在常规测试/CI 中误跑。
const defaultDSN = "host=127.0.0.1 port=55533 user=sub2api password=sub2api dbname=checkin_e2e sslmode=disable TimeZone=Asia/Shanghai"

func verifyDSN(t *testing.T) string {
	t.Helper()
	if dsn := os.Getenv("CHECKIN_VERIFY_DSN"); dsn != "" {
		return dsn
	}
	if os.Getenv("CHECKIN_VERIFY_DEFAULT_DSN") == "" {
		t.Skip("set CHECKIN_VERIFY_DSN to run the Postgres end-to-end check-in test")
	}
	return defaultDSN
}

type settingRepoStub struct{ values map[string]string }

func (s *settingRepoStub) Get(context.Context, string) (*service.Setting, error) { return nil, nil }
func (s *settingRepoStub) GetValue(_ context.Context, key string) (string, error) {
	return s.values[key], nil
}
func (s *settingRepoStub) Set(context.Context, string, string) error { return nil }
func (s *settingRepoStub) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	out := make(map[string]string, len(keys))
	for _, k := range keys {
		out[k] = s.values[k]
	}
	return out, nil
}
func (s *settingRepoStub) SetMultiple(context.Context, map[string]string) error { return nil }
func (s *settingRepoStub) GetAll(context.Context) (map[string]string, error) {
	out := make(map[string]string, len(s.values))
	for k, v := range s.values {
		out[k] = v
	}
	return out, nil
}
func (s *settingRepoStub) Delete(context.Context, string) error { return nil }

func TestCheckInEndToEndAgainstPostgres(t *testing.T) {
	dsn := verifyDSN(t)
	if err := timezone.Init("Asia/Shanghai"); err != nil {
		t.Fatalf("timezone init: %v", err)
	}
	ctx := context.Background()
	sqlDB, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer sqlDB.Close()
	if err := repository.ApplyMigrations(ctx, sqlDB); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, sqlDB)))
	defer client.Close()

	// 每次运行使用独立邮箱，配合唯一索引与清理逻辑保持可重复。
	email := fmt.Sprintf("checkin-e2e-%d@example.com", time.Now().UnixNano())
	user, err := client.User.Create().SetEmail(email).SetPasswordHash("x").Save(ctx)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	settings := &settingRepoStub{values: map[string]string{
		service.SettingKeyCheckInEnabled:        "true",
		service.SettingKeyCheckInCaptchaEnabled: "true", // 打开验证但没有任何 Provider → R1 必须放行
		service.SettingKeyCheckInTiers:          `[{"min":1,"max":1,"weight":1}]`,
	}}
	settingService := service.NewSettingService(settings, &config.Config{})
	svc := service.NewCheckInService(client, repository.NewUserRepository(client, sqlDB), settingService, nil, nil, nil, nil)

	status, err := svc.GetStatus(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetStatus: %v", err)
	}
	if !status.Enabled || status.CaptchaRequired || status.CheckedInToday {
		t.Fatalf("unexpected status: %+v", status)
	}

	first, err := svc.PerformCheckIn(ctx, user.ID, service.CaptchaProof{}, "127.0.0.1")
	if err != nil {
		t.Fatalf("first check-in (R1 should allow without provider): %v", err)
	}
	if first.Reward != 1 || first.StreakDays != 1 {
		t.Fatalf("unexpected first result: %+v", first)
	}

	second, err := svc.PerformCheckIn(ctx, user.ID, service.CaptchaProof{}, "127.0.0.1")
	if err == nil {
		t.Fatalf("second check-in should fail, got %+v", second)
	}

	reloaded, err := client.User.Get(ctx, user.ID)
	if err != nil {
		t.Fatalf("reload user: %v", err)
	}
	if reloaded.Balance < 0.999999 || reloaded.Balance > 1.000001 {
		t.Fatalf("balance = %v, want exactly one reward (1.0)", reloaded.Balance)
	}
	// 签到额度不是充值：不能计入累计充值
	if reloaded.TotalRecharged != 0 {
		t.Fatalf("total_recharged = %v, check-in credit must not count as recharge", reloaded.TotalRecharged)
	}

	status, err = svc.GetStatus(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetStatus after check-in: %v", err)
	}
	if !status.CheckedInToday || status.TodayReward != 1 || status.StreakDays != 1 || len(status.RecentRecords) != 1 {
		t.Fatalf("unexpected status after check-in: %+v", status)
	}
	// 月历：当前月应恰好包含今天这一条；上个月为空；非法月份被拒绝。
	calendar, err := svc.GetMonthCalendar(ctx, user.ID, "")
	if err != nil {
		t.Fatalf("GetMonthCalendar: %v", err)
	}
	if calendar.Month != status.TodayDate[:7] || calendar.CheckedDays != 1 || len(calendar.Days) != 1 || calendar.Days[0].Date != status.TodayDate {
		t.Fatalf("unexpected calendar: %+v", calendar)
	}
	prevMonth := time.Now().AddDate(0, -1, 0).Format("2006-01")
	prev, err := svc.GetMonthCalendar(ctx, user.ID, prevMonth)
	if err != nil {
		t.Fatalf("GetMonthCalendar prev: %v", err)
	}
	if prev.CheckedDays != 0 || len(prev.Days) != 0 {
		t.Fatalf("previous month should be empty: %+v", prev)
	}
	if _, err := svc.GetMonthCalendar(ctx, user.ID, "2026/10"); err == nil {
		t.Fatal("invalid month should be rejected")
	}

	// 管理端统计
	stats, err := svc.GetAdminStats(ctx, 30)
	if err != nil {
		t.Fatalf("GetAdminStats: %v", err)
	}
	if stats.Summary.TodayUsers < 1 || stats.Summary.TodayReward < 1 || len(stats.Daily) != 30 || stats.Daily[29].Date != status.TodayDate || stats.Daily[29].Users < 1 {
		t.Fatalf("unexpected stats summary/daily: %+v last=%+v", stats.Summary, stats.Daily[len(stats.Daily)-1])
	}
	if len(stats.Tiers) != 1 || stats.Tiers[0].Count < 1 || stats.Tiers[0].ConfiguredPercent == nil || *stats.Tiers[0].ConfiguredPercent != 100 {
		t.Fatalf("unexpected tier stats: %+v", stats.Tiers)
	}

	// 管理端明细：按邮箱搜索、日期筛选、非法日期、排序字段白名单
	records, total, err := svc.ListAdminRecords(ctx, service.CheckInRecordFilter{Search: email, Page: 1, PageSize: 20, SortBy: "reward", SortDesc: true})
	if err != nil || total != 1 || len(records) != 1 || records[0].UserEmail != email || records[0].CheckInDate != status.TodayDate || records[0].Reward != 1 {
		t.Fatalf("ListAdminRecords by email = %+v total=%d err=%v", records, total, err)
	}
	if _, total, err := svc.ListAdminRecords(ctx, service.CheckInRecordFilter{Search: email, StartDate: "2999-01-01", Page: 1, PageSize: 20}); err != nil || total != 0 {
		t.Fatalf("future start_date should match nothing: total=%d err=%v", total, err)
	}
	if _, _, err := svc.ListAdminRecords(ctx, service.CheckInRecordFilter{StartDate: "2026/10/27"}); err == nil {
		t.Fatal("invalid start_date should be rejected")
	}
	if _, _, err := svc.ListAdminRecords(ctx, service.CheckInRecordFilter{Search: "100%_", SortBy: "u.id; DROP TABLE users", Page: 1, PageSize: 5}); err != nil {
		t.Fatalf("hostile search/sort should be neutralised, got %v", err)
	}

	// 管理端充值记录：签到额度以 check_in 类型出现，且不计入累计充值
	adminSvc := service.NewAdminService(&config.Config{}, repository.NewUserRepository(client, sqlDB), nil, nil, nil, nil,
		repository.NewRedeemCodeRepository(client), nil, nil, nil, nil, nil, nil, client, settingService, nil, nil, nil, nil, nil, nil, nil, nil)
	for _, codeType := range []string{"", service.RedeemTypeCheckIn} {
		history, historyTotal, totalRecharged, err := adminSvc.GetUserBalanceHistory(ctx, user.ID, 1, 20, codeType)
		if err != nil || historyTotal != 1 || len(history) != 1 || history[0].Type != service.RedeemTypeCheckIn ||
			history[0].Code != "CHECKIN-"+status.TodayDate || history[0].Value != 1 || totalRecharged != 0 {
			t.Fatalf("balance history (type=%q) = %+v total=%d recharged=%v err=%v", codeType, history, historyTotal, totalRecharged, err)
		}
	}

	t.Logf("OK reward=%v balance=%v streak=%d today=%s next=%s calendar=%s days=%d", first.Reward, reloaded.Balance, status.StreakDays, status.TodayDate, status.NextResetAt, calendar.Month, calendar.CheckedDays)
}
