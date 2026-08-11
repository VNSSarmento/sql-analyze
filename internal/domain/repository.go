package domain

import "context"

type QueryRepository interface {
	GetByID(ctx context.Context, queryID, dbUser string) (*Query, error)
	Save(ctx context.Context, q *Query) error
	GetTopSlowest(ctx context.Context, limit int) ([]*Query, error)
}

type AlertPublisher interface {
	Publish(ctx context.Context, alert *AnomalyAlert) error
}
