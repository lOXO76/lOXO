//go:build unit

package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strings"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	_ "github.com/Wei-Shaw/sub2api/ent/runtime"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

func validRewardTaskAdminInput() RewardTaskInput {
	return RewardTaskInput{Title: "Browse", TaskType: "browse", VerifyMode: "timer", TargetURL: "https://example.com/?ticket={ticket}#details", RewardAmount: 0.12345678, Period: "daily", BrowseSeconds: 30, Enabled: true}
}

func TestValidateRewardTaskInputCombinations(t *testing.T) {
	partnerID := int64(1)
	for _, taskType := range []string{"browse", "register", "recharge", "other"} {
		for _, mode := range []string{"timer", "sdk", "callback"} {
			for _, period := range []string{"once", "daily", "weekly", "monthly", "other"} {
				t.Run(taskType+"/"+mode+"/"+period, func(t *testing.T) {
					in := validRewardTaskAdminInput()
					in.TaskType, in.VerifyMode, in.Period = taskType, mode, period
					in.MinRechargeAmount = 1
					if mode != "timer" {
						in.PartnerID = &partnerID
					}
					valid := period != "other" && ((taskType == "browse" && (mode == "timer" || mode == "sdk")) || (taskType == "register" && mode == "callback" && period == "once") || (taskType == "recharge" && mode == "callback"))
					err := ValidateRewardTaskInput(in)
					if (err == nil) != valid {
						t.Fatalf("valid=%v, error=%v", valid, err)
					}
					if err != nil && infraerrors.Reason(err) != "REWARD_TASK_INVALID" {
						t.Fatalf("unexpected reason: %v", err)
					}
				})
			}
		}
	}
}

func TestValidateRewardTaskInputFields(t *testing.T) {
	now := time.Now()
	partnerID := int64(1)
	tests := []struct {
		name string
		edit func(*RewardTaskInput)
	}{
		{"empty title", func(in *RewardTaskInput) { in.Title = "  " }},
		{"long title", func(in *RewardTaskInput) { in.Title = strings.Repeat("x", 101) }},
		{"long description", func(in *RewardTaskInput) { in.Description = strings.Repeat("x", 2001) }},
		{"unsafe url", func(in *RewardTaskInput) { in.TargetURL = "javascript:alert(1)" }},
		{"relative url", func(in *RewardTaskInput) { in.TargetURL = "/target" }},
		{"long url", func(in *RewardTaskInput) { in.TargetURL = "https://example.com/" + strings.Repeat("x", 2048) }},
		{"unsafe icon", func(in *RewardTaskInput) { in.IconURL = "data:image/png;base64,aaaa" }},
		{"external relative icon", func(in *RewardTaskInput) { in.IconURL = "//example.com/icon.png" }},
		{"zero reward", func(in *RewardTaskInput) { in.RewardAmount = 0 }},
		{"large reward", func(in *RewardTaskInput) { in.RewardAmount = 10000.01 }},
		{"nan reward", func(in *RewardTaskInput) { in.RewardAmount = math.NaN() }},
		{"infinite reward", func(in *RewardTaskInput) { in.RewardAmount = math.Inf(1) }},
		{"reward precision", func(in *RewardTaskInput) { in.RewardAmount = 0.123456789 }},
		{"timer partner", func(in *RewardTaskInput) { in.PartnerID = &partnerID }},
		{"missing sdk partner", func(in *RewardTaskInput) { in.VerifyMode = "sdk" }},
		{"short browsing", func(in *RewardTaskInput) { in.BrowseSeconds = 4 }},
		{"long browsing", func(in *RewardTaskInput) { in.BrowseSeconds = 86401 }},
		{"negative quota", func(in *RewardTaskInput) { in.MaxCompletions = -1 }},
		{"same window", func(in *RewardTaskInput) { in.StartsAt, in.EndsAt = &now, &now }},
		{"small recharge", func(in *RewardTaskInput) {
			in.TaskType, in.VerifyMode, in.PartnerID, in.MinRechargeAmount = "recharge", "callback", &partnerID, 0.001
		}},
		{"recharge precision", func(in *RewardTaskInput) {
			in.TaskType, in.VerifyMode, in.PartnerID, in.MinRechargeAmount = "recharge", "callback", &partnerID, 1.001
		}},
		{"large recharge", func(in *RewardTaskInput) {
			in.TaskType, in.VerifyMode, in.PartnerID, in.MinRechargeAmount = "recharge", "callback", &partnerID, 100000000.01
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := validRewardTaskAdminInput()
			tt.edit(&in)
			if err := ValidateRewardTaskInput(in); err == nil || infraerrors.Reason(err) != "REWARD_TASK_INVALID" {
				t.Fatalf("expected REWARD_TASK_INVALID, got %v", err)
			}
		})
	}
	for _, reward := range []float64{0.00000001, 0.000001, 0.1, 0.12345678, 9999.99999999, 10000} {
		in := validRewardTaskAdminInput()
		in.RewardAmount, in.IconURL = reward, "/icons/reward.svg"
		if err := ValidateRewardTaskInput(in); err != nil {
			t.Fatalf("valid reward %v rejected: %v", reward, err)
		}
	}
}

func TestNormalizeRewardPartnerDomains(t *testing.T) {
	got, err := NormalizeRewardPartnerDomains([]string{" Example.COM ", "*.Example.com", "example.com", "127.0.0.1", "::1"})
	if err != nil || strings.Join(got, ",") != "example.com,*.example.com,127.0.0.1,::1" {
		t.Fatalf("unexpected domains: %v (%v)", got, err)
	}
	for _, input := range [][]string{{""}, {"https://example.com"}, {"example.com:443"}, {"example.com/path"}, {"user@example.com"}, {"example..com"}, {"-example.com"}, {"example-.com"}, {"*.127.0.0.1"}, {"*.*.example.com"}, {strings.Repeat("a", 64) + ".com"}} {
		if _, err := NormalizeRewardPartnerDomains(input); err == nil || infraerrors.Reason(err) != "REWARD_PARTNER_INVALID_DOMAINS" {
			t.Fatalf("expected invalid domains for %v, got %v", input, err)
		}
	}
	input := make([]string, 21)
	for i := range input {
		input[i] = strings.Repeat("a", i+1) + ".com"
	}
	if _, err := NormalizeRewardPartnerDomains(input); err == nil {
		t.Fatal("accepted more than 20 domains")
	}
	if got, err := NormalizeRewardPartnerDomains(nil); err != nil || got == nil || len(got) != 0 {
		t.Fatalf("nil must normalize to JSON array: %v (%v)", got, err)
	}
}

func TestNormalizeRewardTaskInputUnusedProgress(t *testing.T) {
	for _, taskType := range []string{"browse", "register", "recharge"} {
		in := RewardTaskInput{Title: "  Title  ", TaskType: taskType, BrowseSeconds: 30, MinRechargeAmount: 12.34}
		got := normalizeRewardTaskInput(in)
		if got.Title != "Title" || (taskType != "browse" && got.BrowseSeconds != 0) || (taskType != "recharge" && got.MinRechargeAmount != 0) {
			t.Fatalf("incorrect normalization for %s: %+v", taskType, got)
		}
	}
}

type rewardAdminTestEncryptor struct {
	plaintext string
	err       error
}

func (e *rewardAdminTestEncryptor) Encrypt(plaintext string) (string, error) {
	e.plaintext = plaintext
	return "encrypted:" + plaintext, e.err
}

func (e *rewardAdminTestEncryptor) Decrypt(ciphertext string) (string, error) {
	return strings.TrimPrefix(ciphertext, "encrypted:"), e.err
}

func TestRewardAdminPartnerSecrets(t *testing.T) {
	encryptor := &rewardAdminTestEncryptor{}
	svc := &RewardTaskService{encryptor: encryptor}
	secret, encrypted, err := svc.newRewardPartnerSecret()
	if err != nil || !regexp.MustCompile(`^rs_[0-9a-f]{48}$`).MatchString(secret) || encrypted != "encrypted:"+secret || encryptor.plaintext != secret {
		t.Fatalf("incorrect generated secret or encryption: secret=%q, encrypted=%q, error=%v", secret, encrypted, err)
	}
	next, _, err := svc.newRewardPartnerSecret()
	if err != nil || next == secret {
		t.Fatal("secret must be newly randomized for each reset")
	}
	encryptor.err = errors.New("encryption failed")
	if secret, ciphertext, err := svc.newRewardPartnerSecret(); err == nil || secret != "" || ciphertext != "" {
		t.Fatal("encryption failures must not return secret material")
	}
	if _, _, err := (&RewardTaskService{}).newRewardPartnerSecret(); err == nil {
		t.Fatal("missing encryptor must fail closed")
	}
}

func rewardAdminTestService(t *testing.T) (*RewardTaskService, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	t.Cleanup(func() { _ = client.Close() })
	return &RewardTaskService{entClient: client}, mock
}

func TestRewardAdminStats(t *testing.T) {
	svc, mock := rewardAdminTestService(t)
	mock.ExpectQuery(`(?s)SELECT.*FROM reward_task_records WHERE status='completed'`).WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"total_tasks", "enabled_tasks", "completed_today", "reward_today", "completed_total", "reward_total", "partners"}).AddRow(5, 3, 12, 6, 100, 50, 2))
	stats, err := svc.GetAdminStats(context.Background())
	if err != nil || stats.TotalTasks != 5 || stats.EnabledTasks != 3 || stats.CompletedToday != 12 || stats.RewardToday != 6 || stats.CompletedTotal != 100 || stats.RewardTotal != 50 || stats.Partners != 2 {
		t.Fatalf("incorrect stats: %+v (%v)", stats, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRewardAdminPartnerViewNeverExposesEncryptedSecret(t *testing.T) {
	svc, mock := rewardAdminTestService(t)
	mock.ExpectQuery(`SELECT partner_id, COUNT\(\*\) FROM reward_tasks WHERE deleted_at IS NULL`).WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"partner_id", "count"}).AddRow(7, 2))
	items, err := svc.rewardAdminPartnerViews(context.Background(), []*dbent.RewardTaskPartner{{ID: 7, Name: "Partner", AppKey: "rk_public", AppSecretEncrypted: "DO_NOT_EXPOSE", SecretHint: "1234"}})
	if err != nil || len(items) != 1 || items[0].TaskCount != 2 {
		t.Fatalf("incorrect partner view: %+v (%v)", items, err)
	}
	body, err := json.Marshal(items[0])
	if err != nil || strings.Contains(string(body), "DO_NOT_EXPOSE") || strings.Contains(string(body), "app_secret") || !strings.Contains(string(body), `"allowed_domains":[]`) || !strings.Contains(string(body), `"last_event_at":null`) {
		t.Fatalf("unsafe or incomplete partner JSON: %s (%v)", body, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRewardAdminRecordsReadModel(t *testing.T) {
	svc, mock := rewardAdminTestService(t)
	filter := RewardTaskRecordAdminFilter{Page: 2, PageSize: 10, TaskID: 5, Status: "completed", Search: "user_1"}
	mock.ExpectQuery(`(?s)SELECT COUNT\(\*\) FROM reward_task_records r LEFT JOIN reward_tasks.*r.task_id=\$1 AND r.status=\$2`).
		WithArgs(int64(5), "completed", `%user\_1%`).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(11))
	now := time.Now()
	mock.ExpectQuery(`(?s)SELECT r.id, r.task_id, COALESCE\(NULLIF\(r.task_title,''\),t.title,''\).*r.status, r.progress_ms, COALESCE\(t.browse_seconds,0\).*ORDER BY r.id DESC LIMIT \$4 OFFSET \$5`).
		WithArgs(int64(5), "completed", `%user\_1%`, 10, 10).
		WillReturnRows(sqlmock.NewRows([]string{"id", "task_id", "task_title", "task_type", "user_id", "user_email", "username", "period_key", "status", "progress_ms", "browse_seconds", "recharge_total", "external_user_id", "started_at", "ticket_expires_at", "completed_at", "reward_amount"}).
			AddRow(2, 5, "Snapshot title", "register", 1, "user@example.com", "user_1", "once", "completed", 0, 0, 0, "external", now, now.Add(time.Hour), now, 0.5))
	items, total, err := svc.ListAdminRecords(context.Background(), filter)
	if err != nil || total != 11 || len(items) != 1 || items[0].TaskTitle != "Snapshot title" || items[0].CompletedAt == nil || items[0].RewardAmount != 0.5 {
		t.Fatalf("incorrect records: %+v, total=%d, error=%v", items, total, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRewardAdminEventsReadModelNullableAndPayload(t *testing.T) {
	svc, mock := rewardAdminTestService(t)
	filter := RewardTaskEventAdminFilter{Page: 1, PageSize: 20, PartnerID: 7, Result: "rejected"}
	mock.ExpectQuery(`(?s)SELECT COUNT\(\*\) FROM reward_task_events.*e.partner_id=\$1 AND e.result=\$2`).
		WithArgs(int64(7), "rejected").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(`(?s)SELECT e.id, e.partner_id.*ORDER BY e.id DESC LIMIT \$3 OFFSET \$4`).
		WithArgs(int64(7), "rejected", 20, 0).
		WillReturnRows(sqlmock.NewRows([]string{"id", "partner_id", "partner_name", "event_id", "event_type", "ticket", "task_id", "user_id", "external_user_id", "amount", "order_id", "result", "error_code", "remote_ip", "payload", "created_at"}).
			AddRow(8, 7, "Partner", "event-1", "register", "missing", nil, nil, "external", 0, "", "rejected", "REWARD_TICKET_NOT_FOUND", "127.0.0.1", []byte(`{"raw":"invalid"}`), time.Now()))
	items, total, err := svc.ListAdminEvents(context.Background(), filter)
	if err != nil || total != 1 || len(items) != 1 || items[0].TaskID != nil || items[0].UserID != nil || string(items[0].Payload) != `{"raw":"invalid"}` {
		t.Fatalf("incorrect events: %+v, total=%d, error=%v", items, total, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRewardAdminPartnerDeleteLocksAndChecksBindings(t *testing.T) {
	for _, tc := range []struct {
		name   string
		found  bool
		reason string
	}{
		{name: "missing", reason: "REWARD_PARTNER_NOT_FOUND"},
		{name: "bound", found: true, reason: "REWARD_PARTNER_IN_USE"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, mock := rewardAdminTestService(t)
			mock.ExpectBegin()
			rows := sqlmock.NewRows([]string{"id", "name"})
			if tc.found {
				rows.AddRow(7, "Partner")
			}
			mock.ExpectQuery(`(?s)SELECT.*FROM "reward_task_partners".*"deleted_at" IS NULL.*FOR UPDATE`).WithArgs(int64(7)).WillReturnRows(rows)
			if tc.found {
				mock.ExpectQuery(`(?s)SELECT.*FROM "reward_tasks".*"partner_id" = \$1.*"deleted_at" IS NULL`).WithArgs(int64(7)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(5))
			}
			mock.ExpectRollback()
			err := svc.DeleteAdminPartner(context.Background(), 7)
			if got := infraerrors.Reason(err); got != tc.reason {
				t.Fatalf("reason=%q, want %q, error=%v", got, tc.reason, err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestRewardAdminTaskTypeImmutableBeforeOtherValidation(t *testing.T) {
	svc, mock := rewardAdminTestService(t)
	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)SELECT.*FROM "reward_tasks".*"deleted_at" IS NULL.*FOR UPDATE`).WithArgs(int64(5)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "task_type"}).AddRow(5, "register"))
	mock.ExpectRollback()
	_, err := svc.UpdateAdminTask(context.Background(), 5, RewardTaskInput{TaskType: "recharge"})
	if got := infraerrors.Reason(err); got != "REWARD_TASK_TYPE_IMMUTABLE" {
		t.Fatalf("reason=%q, want REWARD_TASK_TYPE_IMMUTABLE", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestValidateRewardTaskIdleTimeout(t *testing.T) {
	for _, seconds := range []int{-1, 1, 4, 3601} {
		in := validRewardTaskAdminInput()
		encoded := []byte(`{"idle_timeout_seconds":` + fmt.Sprint(seconds) + `}`)
		if err := json.Unmarshal(encoded, &in); err != nil {
			t.Fatal(err)
		}
		if err := ValidateRewardTaskInput(in); infraerrors.Reason(err) != "REWARD_TASK_INVALID" {
			t.Errorf("idle timeout %d accepted: %v", seconds, err)
		}
	}
	for _, seconds := range []int{0, 5, 15, 3600} {
		for _, taskType := range []string{"browse", "register", "recharge"} {
			in := validRewardTaskAdminInput()
			in.IdleTimeoutSeconds = seconds
			if taskType != "browse" {
				id := int64(1)
				in.TaskType = taskType
				in.VerifyMode = "callback"
				in.Period = "once"
				in.PartnerID = &id
				in.MinRechargeAmount = 1
			}
			if err := ValidateRewardTaskInput(in); err != nil {
				t.Errorf("idle timeout %d rejected for %s: %v", seconds, taskType, err)
			}
			if got := normalizeRewardTaskInput(in).IdleTimeoutSeconds; got != seconds {
				t.Errorf("idle timeout lost for %s: %d", taskType, got)
			}
		}
	}
	in := validRewardTaskAdminInput()
	if err := json.Unmarshal([]byte(`{"idle_timeout_seconds":15.5}`), &in); err == nil {
		t.Fatal("fractional idle timeout accepted")
	}
}
