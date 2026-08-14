package usecase

import (
	"context"
	"errors"
	"log"
	"sql-analyze/internal/domain"
)

type ListSlowestQueriesUseCase struct {
	repository domain.QueryRepository
	cache      domain.QueryCache
}

func NewListSlowestQueriesUseCase(r domain.QueryRepository, cache domain.QueryCache) *ListSlowestQueriesUseCase {
	return &ListSlowestQueriesUseCase{
		repository: r,
		cache:      cache,
	}
}

func (c *ListSlowestQueriesUseCase) Execute(ctx context.Context, limit int) ([]*domain.Query, error) {
	queries, err := c.cache.GetSlowest(ctx, limit)

	if err == nil {
		return queries, nil
	}

	if !errors.Is(err, domain.ErrCacheMiss) {
		log.Println("cache indisponível, encaminhando para banco de dados:", err)
	}

	queries, err = c.repository.GetTopSlowest(ctx, limit)
	if err != nil {
		return nil, err
	}

	err = c.cache.SetSlowest(ctx, limit, queries)

	if err != nil {
		log.Println("falha ao adicionar no cache:", err)
	}

	return queries, nil
}
