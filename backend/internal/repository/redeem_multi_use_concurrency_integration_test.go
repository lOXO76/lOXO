//go:build integration

package repository

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/ent/redeemcode"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

// 多个用户并发兑换同一个多次兑换码：名额不得超发，余额只按成功的兑换入账。
// 没有 Redis 锁（cache 为 nil），并发安全完全依赖数据库行锁。
func TestRedeemMultiUseConcurrentRedemptionsRespectLimits(t *testing.T) {
	const (
		maxUsers        = 3
		maxUsesPerUser  = 2
		userCount       = 5
		attemptsPerUser = 4
		value           = 2.5
	)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	client := testEntClient(t)
	seed := time.Now().UnixNano()

	userIDs := make([]int64, 0, userCount)
	initialBalance := make(map[int64]float64, userCount)
	for i := range userCount {
		u, err := client.User.Create().
			SetEmail(fmt.Sprintf("multi-use-%d-%d@example.com", seed, i)).
			SetPasswordHash("test").
			Save(ctx)
		require.NoError(t, err)
		userIDs = append(userIDs, u.ID)
		initialBalance[u.ID] = u.Balance
	}

	codes := NewRedeemCodeRepository(client)
	parent := &service.RedeemCode{
		Code:           fmt.Sprintf("MU-%d", seed),
		Type:           service.RedeemTypeMultiUse,
		BenefitType:    service.RedeemTypeBalance,
		Value:          value,
		Status:         service.StatusUnused,
		MaxUsers:       maxUsers,
		MaxUsesPerUser: maxUsesPerUser,
	}
	require.NoError(t, codes.Create(ctx, parent))

	// 兑换会真实提交事务，显式清理，避免影响同包其它集成测试的列表与计数。
	t.Cleanup(func() {
		cleanupCtx := context.Background()
		_, err := integrationDB.ExecContext(cleanupCtx, `DELETE FROM redeem_codes WHERE parent_id = $1 OR id = $1`, parent.ID)
		require.NoError(t, err)
		for _, id := range userIDs {
			_, err := integrationDB.ExecContext(cleanupCtx, `DELETE FROM users WHERE id = $1`, id)
			require.NoError(t, err)
		}
	})

	redeem := service.NewRedeemService(codes, NewUserRepository(client, integrationDB), nil, nil, nil, client, nil, nil)

	type outcome struct {
		userID int64
		err    error
	}
	start := make(chan struct{})
	results := make(chan outcome, userCount*attemptsPerUser)
	var wg sync.WaitGroup
	for _, userID := range userIDs {
		for range attemptsPerUser {
			wg.Add(1)
			go func(userID int64) {
				defer wg.Done()
				<-start
				_, err := redeem.Redeem(ctx, userID, parent.Code)
				results <- outcome{userID: userID, err: err}
			}(userID)
		}
	}
	close(start)
	wg.Wait()
	close(results)

	successes := make(map[int64]int, userCount)
	for result := range results {
		switch {
		case result.err == nil:
			successes[result.userID]++
		case errors.Is(result.err, service.ErrRedeemCodeUserLimitReached),
			errors.Is(result.err, service.ErrRedeemCodeUserQuotaExhausted),
			errors.Is(result.err, service.ErrRedeemCodeUsed):
		default:
			t.Fatalf("unexpected redeem error for user %d: %v", result.userID, result.err)
		}
	}

	// 恰好 maxUsers 个用户兑换成功，且每人都用满了自己的次数。
	require.Len(t, successes, maxUsers)
	for userID, count := range successes {
		require.Equal(t, maxUsesPerUser, count, "user %d", userID)
	}

	got, err := codes.GetByID(ctx, parent.ID)
	require.NoError(t, err)
	require.Equal(t, maxUsers*maxUsesPerUser, got.UsedCount)
	require.Equal(t, maxUsers, got.UsedUserCount)
	require.Equal(t, service.StatusUsed, got.Status)

	for _, userID := range userIDs {
		u, err := client.User.Get(ctx, userID)
		require.NoError(t, err)
		require.InDelta(t, initialBalance[userID]+float64(successes[userID])*value, u.Balance, 1e-9, "balance of user %d", userID)

		records, err := client.RedeemCode.Query().
			Where(redeemcode.ParentIDEQ(parent.ID), redeemcode.UsedByEQ(userID)).
			Count(ctx)
		require.NoError(t, err)
		require.Equal(t, successes[userID], records, "redemption records of user %d", userID)
	}
}
