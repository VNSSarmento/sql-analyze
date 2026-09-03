package redisadapter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sql-analyze/internal/domain"
	"time"

	"github.com/redis/go-redis/v9"
)

type QueryCacheAdapter struct {
	client *redis.Client
}

func NewQueryCacheAdapter(client *redis.Client) *QueryCacheAdapter {
	return &QueryCacheAdapter{
		client: client,
	}
}

func (c *QueryCacheAdapter) SetSlowest(ctx context.Context, limit int, queries []*domain.Query) error {
	key := fmt.Sprintf("queries:slowest:%d", limit)

	jsonBytes, err := json.Marshal(queries)

	if err != nil {
		return fmt.Errorf("failed to marshal queries for cache: %w", err)
	}

	return c.client.Set(ctx, key, jsonBytes, time.Minute*1).Err()

}

func (c *QueryCacheAdapter) GetSlowest(ctx context.Context, limit int) ([]*domain.Query, error) {
	key := fmt.Sprintf("queries:slowest:%d", limit)

	stringCmd := c.client.Get(ctx, key)
	result, err := stringCmd.Result()

	if errors.Is(err, redis.Nil) {
		return nil, domain.ErrCacheMiss
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get key from redis: %w", err)
	}

	var queries []*domain.Query

	err = json.Unmarshal([]byte(result), &queries)

	if err != nil {

		return nil, fmt.Errorf("failed to unmarshal cached queries: %w", err)
	}

	return queries, nil

}
