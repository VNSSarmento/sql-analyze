package postgres

import (
	"context"
	"errors"
	"fmt"
	"sql-analyze/internal/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresContactRepository struct {
	pool *pgxpool.Pool
}

func NewContactRepository(pool *pgxpool.Pool) *PostgresContactRepository {
	return &PostgresContactRepository{pool: pool}
}

func (p *PostgresContactRepository) GetByDBUser(ctx context.Context, dbUser string) (*domain.UserContact, error) {
	sql := "SELECT db_user,email,display_name,created_at FROM user_contacts WHERE db_user = $1"

	userContact := &domain.UserContact{}

	result := p.pool.QueryRow(ctx, sql, dbUser)

	err := result.Scan(
		&userContact.DBUser,
		&userContact.Email,
		&userContact.DisplayName,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrContactNotFound
	}

	if err != nil {
		err := fmt.Errorf("buscando query: %w", err)

		return nil, err
	}

	return userContact, nil

}
