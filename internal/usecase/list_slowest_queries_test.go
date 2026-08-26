package usecase

import (
	"context"
	"errors"
	"sql-analyze/internal/domain"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

var queriesMock []*domain.Query = []*domain.Query{
	{ExecutionsCount: 8, MeanTimeMs: 50, M2: 175},
	{ExecutionsCount: 10, MeanTimeMs: 9999, M2: 175},
	{ExecutionsCount: 10, MeanTimeMs: 50, M2: 0},
}

func TestListSlowstQueries(t *testing.T) {
	casos := []struct {
		nome       string
		limit      int
		setupMocks func(repo *MockQueryRepository, cache *MockCache)
		wantErr    bool
	}{
		{
			nome:  "Cache hit",
			limit: 10,
			setupMocks: func(repo *MockQueryRepository, cache *MockCache) {
				cache.On("GetSlowest", mock.Anything, 10).Return(queriesMock, nil)
			},
			wantErr: false,
		},
		{
			nome:  "Cache miss",
			limit: 10,
			setupMocks: func(repo *MockQueryRepository, cache *MockCache) {
				cache.On("GetSlowest", mock.Anything, 10).Return(nil, domain.ErrCacheMiss)
				repo.On("GetTopSlowest", mock.Anything, 10).Return(queriesMock, nil)
				cache.On("SetSlowest", mock.Anything, 10, mock.Anything).Return(nil)
			},
			wantErr: false,
		},
		{
			nome:  "Erro repositorio",
			limit: 10,
			setupMocks: func(repo *MockQueryRepository, cache *MockCache) {
				cache.On("GetSlowest", mock.Anything, 10).Return(nil, errors.New("redis indisponivel"))
				repo.On("GetTopSlowest", mock.Anything, 10).Return(nil, errors.New("error ao acessar o banco de dados"))
			},
			wantErr: true,
		},
		{
			nome:  "Erro genérico do cache (Redis fora do ar)",
			limit: 10,
			setupMocks: func(repo *MockQueryRepository, cache *MockCache) {
				cache.On("GetSlowest", mock.Anything, 10).Return(nil, errors.New("redis indisponivel"))
				repo.On("GetTopSlowest", mock.Anything, 10).Return(queriesMock, nil)
				cache.On("SetSlowest", mock.Anything, 10, mock.Anything).Return(errors.New("falha ao adicionar no cache"))
			},
		},
	}

	for _, caso := range casos {
		t.Run(caso.nome, func(t *testing.T) {
			repoMock := new(MockQueryRepository)
			cacheMock := new(MockCache)

			caso.setupMocks(repoMock, cacheMock)

			uc := NewListSlowestQueriesUseCase(repoMock, cacheMock)

			_, err := uc.Execute(context.Background(), caso.limit)

			if caso.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			repoMock.AssertExpectations(t)
			cacheMock.AssertExpectations(t)
		})
	}
}
