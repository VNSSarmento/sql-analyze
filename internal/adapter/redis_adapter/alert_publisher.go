package redisadapter

import (
	"context"
	"sql-analyze/internal/domain"
	"time"

	"github.com/redis/go-redis/v9"
)

const stremName = "queries:alertas"

type StreamAlertPublisher struct {
	ClientAlert *redis.Client
}

func NewStreamAlertPublisher(client *redis.Client) *StreamAlertPublisher {
	return &StreamAlertPublisher{
		ClientAlert: client,
	}
}

func (p *StreamAlertPublisher) Publish(ctx context.Context, alert *domain.AnomalyAlert) error {
	args := redis.XAddArgs{
		Stream: stremName,
		Values: map[string]any{
			"query_id":     alert.QueryID,
			"db_user":      alert.DBUser,
			"current_time": alert.CurrentTimeMs,
			"mean_time_ms": alert.MeanTimeMs,
			"z_score":      alert.ZScore,
			"detected_at":  alert.DetectedAt.Format(time.RFC3339),
		},
	}

	return p.ClientAlert.XAdd(ctx, &args).Err()
}
