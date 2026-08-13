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

func (p *PostgresQueryRepository) GetByID(ctx context.Context, queryID, dbUser string) (*domain.Query, error) {
	sql := "SELECT query_id, db_user, normalized_query, executions_count, mean_time_ms, m2, last_execution_at, last_anomaly_at, created_at FROM queries WHERE query_id = $1 AND db_user = $2"

	query := &domain.Query{}

	err := p.pool.QueryRow(ctx, sql, queryID, dbUser).Scan(
		&query.QueryID,
		&query.DBUser,
		&query.NormalizedQuery,
		&query.ExecutionsCount,
		&query.MeanTimeMs,
		&query.M2,
		&query.LastExecutionAt,
		&query.LastAnomalyAt,
		&query.CreatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrQueryNotFound
	}

	if err != nil {
		err := fmt.Errorf("buscando query: %w", err)

		return nil, err
	}

	return query, nil
}

func (p *PostgresQueryRepository) Save(ctx context.Context, q *domain.Query) error {
	sql := "INSERT INTO queries(query_id, db_user, normalized_query, executions_count, mean_time_ms, m2, last_execution_at, last_anomaly_at, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9) ON CONFLICT (query_id, db_user) DO UPDATE SET normalized_query=$3, executions_count=$4, mean_time_ms=$5, m2=$6, last_execution_at=$7, last_anomaly_at=$8"

	_, err := p.pool.Exec(ctx, sql, q.QueryID, q.DBUser, q.NormalizedQuery, q.ExecutionsCount, q.MeanTimeMs, q.M2, q.LastExecutionAt, q.LastAnomalyAt, q.CreatedAt)

	return err
}

func (p *PostgresQueryRepository) GetTopSlowest(ctx context.Context, limit int) ([]*domain.Query, error) {

	sql := "SELECT query_id, db_user, normalized_query, executions_count, mean_time_ms, m2, last_execution_at, last_anomaly_at, created_at FROM queries ORDER BY mean_time_ms DESC LIMIT $1"
	rows, err := p.pool.Query(ctx, sql, limit)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var queries []*domain.Query

	for rows.Next() {
		query := &domain.Query{}
		err := rows.Scan(
			&query.QueryID,
			&query.DBUser,
			&query.NormalizedQuery,
			&query.ExecutionsCount,
			&query.MeanTimeMs,
			&query.M2,
			&query.LastExecutionAt,
			&query.LastAnomalyAt,
			&query.CreatedAt,
		)

		if err != nil {
			return nil, err
		}

		queries = append(queries, query)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return queries, nil
}
