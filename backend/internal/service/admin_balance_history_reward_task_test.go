package service

import (
	"context"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

func TestRewardTaskBalanceHistoryCodeUsesTitleSnapshot(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	got := rewardTaskBalanceHistoryCode(10, 42, 0.12345678, "Task snapshot", at)
	require.Equal(t, int64(-42), got.ID)
	require.Equal(t, "TASK-42", got.Code)
	require.Equal(t, RedeemTypeTaskReward, got.Type)
	require.Equal(t, "Task snapshot", got.Notes)
	require.Equal(t, 0.12345678, got.Value)
	require.Equal(t, StatusUsed, got.Status)
	require.Equal(t, int64(10), *got.UsedBy)
	require.Equal(t, at, *got.UsedAt)
	require.Equal(t, at, got.CreatedAt)
}

func TestMergeBalanceHistoryIncludesTaskRewardsWithoutIDCollision(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	reward := rewardTaskBalanceHistoryCode(10, 42, 0.5, "Task", at)
	checkIn := checkInBalanceHistoryCode(10, 42, 0.3, at, at.Add(-time.Hour))
	got := mergeBalanceHistoryCodes(nil, []RedeemCode{checkIn, reward}, pagination.PaginationParams{Page: 1, PageSize: 20})
	require.Len(t, got, 2)
	require.Equal(t, RedeemTypeTaskReward, got[0].Type)
	require.Equal(t, RedeemTypeCheckIn, got[1].Type)
}

type taskBalanceHistoryRedeemRepo struct {
	RedeemCodeRepository
	codes []RedeemCode
}

func (r *taskBalanceHistoryRedeemRepo) SumPositiveBalanceByUser(context.Context, int64) (float64, error) {
	return 30, nil
}

func (r *taskBalanceHistoryRedeemRepo) ListByUserPaginated(_ context.Context, _ int64, _ pagination.PaginationParams, _ string) ([]RedeemCode, *pagination.PaginationResult, error) {
	return r.codes, &pagination.PaginationResult{Total: int64(len(r.codes))}, nil
}

func TestGetUserBalanceHistoryTaskRewardFilterAndAll(t *testing.T) {
	for _, codeType := range []string{RedeemTypeTaskReward, ""} {
		t.Run(codeType, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
			defer func() {
				require.NoError(t, client.Close())
				require.NoError(t, mock.ExpectationsWereMet())
			}()
			at := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
			redeemRepo := &taskBalanceHistoryRedeemRepo{codes: []RedeemCode{{ID: 1, Type: RedeemTypeBalance, Value: 30, CreatedAt: at.Add(-time.Hour)}}}
			svc := &adminServiceImpl{entClient: client, redeemCodeRepo: redeemRepo}
			limit := 20
			if codeType == "" {
				limit = 1000
				mock.ExpectQuery(`SELECT id, amount::double precision, created_at FROM user_affiliate_ledger`).
					WithArgs(int64(10), 0, limit).WillReturnRows(sqlmock.NewRows([]string{"id", "amount", "created_at"}))
				mock.ExpectQuery(`SELECT COUNT\(\*\) FROM user_affiliate_ledger`).WithArgs(int64(10)).
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
				mock.ExpectQuery(`SELECT id, reward::double precision, check_in_date, created_at FROM user_check_ins`).
					WithArgs(int64(10), 0, limit).WillReturnRows(sqlmock.NewRows([]string{"id", "reward", "check_in_date", "created_at"}))
				mock.ExpectQuery(`SELECT COUNT\(\*\) FROM user_check_ins`).WithArgs(int64(10)).
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
			}
			mock.ExpectQuery(`SELECT id, reward_amount::double precision, task_title, completed_at FROM reward_task_records WHERE user_id = \$1 AND status = 'completed'`).
				WithArgs(int64(10), 0, limit).WillReturnRows(sqlmock.NewRows([]string{"id", "reward_amount", "task_title", "completed_at"}).
				AddRow(42, 0.12345678, "Task snapshot", at))
			mock.ExpectQuery(`SELECT COUNT\(\*\) FROM reward_task_records WHERE user_id = \$1 AND status = 'completed'`).WithArgs(int64(10)).
				WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
			codes, total, recharged, err := svc.GetUserBalanceHistory(context.Background(), 10, 1, 20, codeType)
			require.NoError(t, err)
			wantTotal := int64(1)
			if codeType == "" {
				wantTotal++
			}
			require.Equal(t, wantTotal, total)
			require.Len(t, codes, int(wantTotal))
			require.Equal(t, "TASK-42", codes[0].Code)
			require.Equal(t, "Task snapshot", codes[0].Notes)
			require.Equal(t, 0.12345678, codes[0].Value)
			require.Equal(t, float64(30), recharged, "task rewards must not contribute to total recharged")
			mock.ExpectClose()
		})
	}
}
