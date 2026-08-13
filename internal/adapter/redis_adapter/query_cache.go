package redisadapter

import (
	"context"
	"encoding/json"
	"fmt"
	"sql-analyze/internal/domain"
	"time"

	"github.com/redis/go-redis/v9"
)

type QueryCacheAdpter struct {
	client *redis.Client
}

func NewCacheQuery(client *redis.Client) *QueryCacheAdpter {
	return &QueryCacheAdpter{
		client: client,
	}
}

func (c *QueryCacheAdpter) SetSlowest(ctx context.Context, limit int, queries []*domain.Query) error {
	key := fmt.Sprintf("queries:slowest:%d", limit)

	jsonBytes, _ := json.Marshal(queries)

	return c.client.Set(ctx, key, jsonBytes, time.Minute*1).Err()

}
