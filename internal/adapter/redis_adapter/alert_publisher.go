package redisadapter

import (
	"context"
	"sql-analyze/internal/domain"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const streamName = "queries:alertas"
const groupName = "workers-alertas"

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
		Stream: streamName,
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

func (p *StreamAlertPublisher) EnsureConsumerGroup(ctx context.Context) error {
	result := p.ClientAlert.XGroupCreateMkStream(ctx, streamName, groupName, "$")
	err := result.Err()

	if err == nil {
		return nil
	}

	if strings.Contains(err.Error(), "BUSYGROUP") {
		return nil
	}

	return err
}
