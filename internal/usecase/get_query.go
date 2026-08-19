package usecase

import (
	"context"
	"sql-analyze/internal/domain"
)

type GetQueryUseCase struct {
	repository domain.QueryRepository
}

func NewGetQueryUseCase(repository domain.QueryRepository) *GetQueryUseCase {
	return &GetQueryUseCase{
		repository: repository,
	}
}

func (g *GetQueryUseCase) Execute(ctx context.Context, queryID string, dbUser string) (*domain.Query, error) {
	return g.repository.GetByID(ctx, queryID, dbUser)
}
