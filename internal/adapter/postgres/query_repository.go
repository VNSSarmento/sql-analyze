package postgres

import (
	"context"
	"errors"
	"fmt"
	"sql-analyze/internal/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresQueryRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresQueryRepository(pool *pgxpool.Pool) *PostgresQueryRepository {
	return &PostgresQueryRepository{pool: pool}
}

func (p *PostgresQueryRepository) GetById(ctx context.Context, queryID, dbUser string) (*domain.Query, error) {
	sql := "SELECT id, query_id, db_user, normalized_query, executions_count, mean_time_ms, m2, last_execution_at, last_anomaly_at, created_at FROM queries WHERE query_id = $1 AND db_user = $2"

	query := &domain.Query{}

	result := p.pool.QueryRow(ctx, sql, queryID, dbUser).Scan(
		query.QueryID,
		query.DBUser,
		query.NormalizedQuery,
		query.ExecutionsCount,
		query.MeanTimeMs,
		query.M2,
		query.LastExecutionAt,
		query.LastAnomalyAt,
		query.CreatedAt,
	)

	if errors.Is(result, pgx.ErrNoRows) {
		return nil, domain.ErrQueryNotFound
	}

	if result != nil {
		err := fmt.Errorf("Buscando query: %w", result)

		return nil, err
	}

	return query, nil
}
