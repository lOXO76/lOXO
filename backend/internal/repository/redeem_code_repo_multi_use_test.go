package repository

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/enttest"
	"github.com/Wei-Shaw/sub2api/ent/redeemcode"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "modernc.org/sqlite"
)

func newMultiUseRedeemRepo(t *testing.T) (*redeemCodeRepository, *dbent.Client) {
	t.Helper()
	name := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := sql.Open("sqlite", "file:"+name+"?mode=memory&cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec("PRAGMA foreign_keys = ON")
	require.NoError(t, err)

	client := enttest.NewClient(t, enttest.WithOptions(dbent.Driver(entsql.OpenDB(dialect.SQLite, db))))
	t.Cleanup(func() { _ = client.Close() })
	return &redeemCodeRepository{client: client}, client
}

func seedMultiUseUser(t *testing.T, client *dbent.Client, name string) int64 {
	t.Helper()
	u, err := client.User.Create().
		SetEmail(name + "@example.com").
		SetPasswordHash("hash").
		Save(context.Background())
	require.NoError(t, err)
	return u.ID
}

func requireMultiUseCounters(t *testing.T, repo *redeemCodeRepository, id int64, usedCount, usedUsers int, status string) {
	t.Helper()
	got, err := repo.GetByID(context.Background(), id)
	require.NoError(t, err)
	require.Equal(t, usedCount, got.UsedCount, "used_count")
	require.Equal(t, usedUsers, got.UsedUserCount, "used_user_count")
	require.Equal(t, status, got.Status, "status")
}

// 2 个用户名额、每人 2 次：逐步走完所有名额，覆盖每人上限、用户名额上限与用完后的状态。
func TestRedeemMultiUseEnforcesUserAndPerUserLimits(t *testing.T) {
	repo, client := newMultiUseRedeemRepo(t)
	ctx := context.Background()
	alice := seedMultiUseUser(t, client, "alice")
	bob := seedMultiUseUser(t, client, "bob")
	carol := seedMultiUseUser(t, client, "carol")

	parent := &service.RedeemCode{
		Code:           "MULTI-LIMITS",
		Type:           service.RedeemTypeMultiUse,
		BenefitType:    service.RedeemTypeBalance,
		Value:          5,
		Status:         service.StatusUnused,
		MaxUsers:       2,
		MaxUsesPerUser: 2,
	}
	require.NoError(t, repo.Create(ctx, parent))

	record, err := repo.RedeemMultiUse(ctx, parent.ID, alice, "REC-A1")
	require.NoError(t, err)
	require.Equal(t, "REC-A1", record.Code)
	require.Equal(t, service.RedeemTypeBalance, record.Type)
	require.Equal(t, 5.0, record.Value)
	require.Equal(t, service.StatusUsed, record.Status)
	require.NotNil(t, record.UsedBy)
	require.Equal(t, alice, *record.UsedBy)
	require.NotNil(t, record.UsedAt)
	require.NotNil(t, record.ParentID)
	require.Equal(t, parent.ID, *record.ParentID)
	require.Equal(t, parent.Code, record.ParentCode)
	requireMultiUseCounters(t, repo, parent.ID, 1, 1, service.StatusUnused)

	_, err = repo.RedeemMultiUse(ctx, parent.ID, alice, "REC-A2")
	require.NoError(t, err)
	requireMultiUseCounters(t, repo, parent.ID, 2, 1, service.StatusUnused)

	// alice 已用完自己的 2 次；被拒绝的兑换不得占用名额。
	_, err = repo.RedeemMultiUse(ctx, parent.ID, alice, "REC-A3")
	require.ErrorIs(t, err, service.ErrRedeemCodeUserLimitReached)
	requireMultiUseCounters(t, repo, parent.ID, 2, 1, service.StatusUnused)

	_, err = repo.RedeemMultiUse(ctx, parent.ID, bob, "REC-B1")
	require.NoError(t, err)
	requireMultiUseCounters(t, repo, parent.ID, 3, 2, service.StatusUnused)

	// 2 个用户名额已被 alice 和 bob 占满，carol 是第 3 个用户。
	_, err = repo.RedeemMultiUse(ctx, parent.ID, carol, "REC-C1")
	require.ErrorIs(t, err, service.ErrRedeemCodeUserQuotaExhausted)
	requireMultiUseCounters(t, repo, parent.ID, 3, 2, service.StatusUnused)

	// bob 的第 2 次用掉最后一个名额，兑换码随之标记为已使用。
	_, err = repo.RedeemMultiUse(ctx, parent.ID, bob, "REC-B2")
	require.NoError(t, err)
	requireMultiUseCounters(t, repo, parent.ID, 4, 2, service.StatusUsed)

	_, err = repo.RedeemMultiUse(ctx, parent.ID, bob, "REC-B3")
	require.ErrorIs(t, err, service.ErrRedeemCodeUsed)
	_, err = repo.RedeemMultiUse(ctx, parent.ID, carol, "REC-C2")
	require.ErrorIs(t, err, service.ErrRedeemCodeUsed)
	requireMultiUseCounters(t, repo, parent.ID, 4, 2, service.StatusUsed)

	records, err := client.RedeemCode.Query().Where(redeemcode.ParentIDEQ(parent.ID)).Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 4, records)

	// 兑换流水与一次性余额兑换码一样计入兑换历史与充值统计。
	history, err := repo.ListByUser(ctx, alice, 10)
	require.NoError(t, err)
	require.Len(t, history, 2)
	for _, item := range history {
		require.Equal(t, service.RedeemTypeBalance, item.Type)
		require.Equal(t, parent.Code, item.ParentCode)
	}
	recharged, err := repo.SumPositiveBalanceByUser(ctx, alice)
	require.NoError(t, err)
	require.InDelta(t, 10.0, recharged, 1e-9)
	carolHistory, err := repo.ListByUser(ctx, carol, 10)
	require.NoError(t, err)
	require.Empty(t, carolHistory)
}

func TestRedeemMultiUseCopiesSubscriptionBenefitToRecord(t *testing.T) {
	repo, client := newMultiUseRedeemRepo(t)
	ctx := context.Background()
	user := seedMultiUseUser(t, client, "subscriber")
	group, err := client.Group.Create().SetName("multi-use-group").Save(ctx)
	require.NoError(t, err)
	expiresAt := time.Now().Add(48 * time.Hour)

	parent := &service.RedeemCode{
		Code:           "MULTI-SUB",
		Type:           service.RedeemTypeMultiUse,
		BenefitType:    service.RedeemTypeSubscription,
		Status:         service.StatusUnused,
		GroupID:        &group.ID,
		ValidityDays:   7,
		ExpiresAt:      &expiresAt,
		MaxUsers:       1,
		MaxUsesPerUser: 1,
	}
	require.NoError(t, repo.Create(ctx, parent))

	record, err := repo.RedeemMultiUse(ctx, parent.ID, user, "REC-SUB")
	require.NoError(t, err)
	require.Equal(t, service.RedeemTypeSubscription, record.Type)
	require.NotNil(t, record.GroupID)
	require.Equal(t, group.ID, *record.GroupID)
	require.Equal(t, 7, record.ValidityDays)
	// 流水行是已完成的兑换，不继承兑换码自身的过期时间。
	require.Nil(t, record.ExpiresAt)
	// 1 个用户、每人 1 次：一次兑换即用完。
	requireMultiUseCounters(t, repo, parent.ID, 1, 1, service.StatusUsed)

	stored, err := repo.GetByID(ctx, record.ID)
	require.NoError(t, err)
	require.Equal(t, parent.Code, stored.ParentCode)
}

func TestRedeemMultiUseRejectsUnavailableCodes(t *testing.T) {
	repo, client := newMultiUseRedeemRepo(t)
	ctx := context.Background()
	user := seedMultiUseUser(t, client, "unavailable")
	past := time.Now().Add(-48 * time.Hour)

	multiUse := func(code, status string, expiresAt *time.Time) *service.RedeemCode {
		return &service.RedeemCode{
			Code:           code,
			Type:           service.RedeemTypeMultiUse,
			BenefitType:    service.RedeemTypeBalance,
			Value:          5,
			Status:         status,
			ExpiresAt:      expiresAt,
			MaxUsers:       10,
			MaxUsesPerUser: 10,
		}
	}
	tests := []struct {
		name    string
		code    *service.RedeemCode
		wantErr error
	}{
		{"past expiry", multiUse("MULTI-EXPIRED", service.StatusUnused, &past), service.ErrRedeemCodeExpired},
		{"expired status", multiUse("MULTI-EXPIRED-STATUS", service.StatusExpired, nil), service.ErrRedeemCodeExpired},
		{"disabled", multiUse("MULTI-DISABLED", service.StatusDisabled, nil), service.ErrRedeemCodeUsed},
		{
			"single-use code",
			&service.RedeemCode{Code: "SINGLE-USE", Type: service.RedeemTypeBalance, Value: 5, Status: service.StatusUnused},
			service.ErrRedeemCodeNotFound,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.NoError(t, repo.Create(ctx, tt.code))

			_, err := repo.RedeemMultiUse(ctx, tt.code.ID, user, "REC-"+tt.code.Code)
			require.ErrorIs(t, err, tt.wantErr)

			got, err := repo.GetByID(ctx, tt.code.ID)
			require.NoError(t, err)
			require.Zero(t, got.UsedCount)
			require.Zero(t, got.UsedUserCount)
			require.Equal(t, tt.code.Status, got.Status)
		})
	}

	t.Run("missing code", func(t *testing.T) {
		_, err := repo.RedeemMultiUse(ctx, 999999, user, "REC-MISSING")
		require.ErrorIs(t, err, service.ErrRedeemCodeNotFound)
	})

	records, err := client.RedeemCode.Query().Where(redeemcode.ParentIDNotNil()).Count(ctx)
	require.NoError(t, err)
	require.Zero(t, records)
}

// 一次性兑换码不指定限制时按「1 个用户、每人 1 次」落库，与迁移给存量数据的默认值一致。
func TestRedeemCodeCreateDefaultsSingleUseLimits(t *testing.T) {
	repo, _ := newMultiUseRedeemRepo(t)
	ctx := context.Background()

	single := &service.RedeemCode{Code: "SINGLE-DEFAULTS", Type: service.RedeemTypeBalance, Value: 5, Status: service.StatusUnused}
	require.NoError(t, repo.Create(ctx, single))
	batch := []service.RedeemCode{
		{Code: "BATCH-DEFAULTS", Type: service.RedeemTypeBalance, Value: 5, Status: service.StatusUnused},
	}
	require.NoError(t, repo.CreateBatch(ctx, batch))

	for _, code := range []string{"SINGLE-DEFAULTS", "BATCH-DEFAULTS"} {
		got, err := repo.GetByCode(ctx, code)
		require.NoError(t, err)
		require.Equal(t, 1, got.MaxUsers)
		require.Equal(t, 1, got.MaxUsesPerUser)
		require.Zero(t, got.UsedCount)
		require.Zero(t, got.UsedUserCount)
		require.Empty(t, got.BenefitType)
		require.Nil(t, got.ParentID)
		require.Empty(t, got.ParentCode)
	}
}

// 兑换在调用方事务内执行：权益发放失败导致回滚时，占用的名额与流水行必须一并撤销。
func TestRedeemMultiUseJoinsCallerTransaction(t *testing.T) {
	repo, client := newMultiUseRedeemRepo(t)
	ctx := context.Background()
	user := seedMultiUseUser(t, client, "rollback")

	parent := &service.RedeemCode{
		Code:           "MULTI-ROLLBACK",
		Type:           service.RedeemTypeMultiUse,
		BenefitType:    service.RedeemTypeBalance,
		Value:          5,
		Status:         service.StatusUnused,
		MaxUsers:       1,
		MaxUsesPerUser: 1,
	}
	require.NoError(t, repo.Create(ctx, parent))

	tx, err := client.Tx(ctx)
	require.NoError(t, err)
	_, err = repo.RedeemMultiUse(dbent.NewTxContext(ctx, tx), parent.ID, user, "REC-ROLLBACK")
	require.NoError(t, err)
	require.NoError(t, tx.Rollback())

	requireMultiUseCounters(t, repo, parent.ID, 0, 0, service.StatusUnused)
	records, err := client.RedeemCode.Query().Where(redeemcode.ParentIDEQ(parent.ID)).Count(ctx)
	require.NoError(t, err)
	require.Zero(t, records)

	// 回滚后名额仍可用。
	_, err = repo.RedeemMultiUse(ctx, parent.ID, user, "REC-RETRY")
	require.NoError(t, err)
	requireMultiUseCounters(t, repo, parent.ID, 1, 1, service.StatusUsed)
}
