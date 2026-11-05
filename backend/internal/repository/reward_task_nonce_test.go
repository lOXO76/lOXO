package repository

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestRewardNonceStoreUsesAtomicSETNXAndTTL(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	defer func() { _ = client.Close() }()
	store := NewRewardTaskNonceStore(client)
	ctx := context.Background()
	key := "reward_open:nonce:rk_key:nonce_0123456789"
	first, err := store.ConsumeNonce(ctx, key, 600*time.Second)
	require.NoError(t, err)
	require.True(t, first)
	repeated, err := store.ConsumeNonce(ctx, key, 600*time.Second)
	require.NoError(t, err)
	require.False(t, repeated)
	require.Equal(t, 600*time.Second, server.TTL(key))
	server.FastForward(600 * time.Second)
	again, err := store.ConsumeNonce(ctx, key, 600*time.Second)
	require.NoError(t, err)
	require.True(t, again)
}
