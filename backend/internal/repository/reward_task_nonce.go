package repository

import (
	"context"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
	"time"
)

type rewardTaskNonceStore struct{ client *redis.Client }

func NewRewardTaskNonceStore(client *redis.Client) service.RewardNonceStore {
	return &rewardTaskNonceStore{client: client}
}
func (s *rewardTaskNonceStore) ConsumeNonce(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	if s.client == nil {
		return true, nil
	}
	return s.client.SetNX(ctx, key, "1", ttl).Result()
}
