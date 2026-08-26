package usecase

import (
	"context"
	"errors"
	"sql-analyze/internal/domain"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestAnalyzeQueryUseCase(t *testing.T) {
	casos := []struct {
		nome       string
		execMs     float64
		setupMocks func(repo *MockQueryRepository, pub *MockAlertPublisher)
		wantErr    bool
	}{
		{
			nome:   "execução anômala publica alerta",
			execMs: 90,
			setupMocks: func(repo *MockQueryRepository, pub *MockAlertPublisher) {
				repo.On("GetByID", mock.Anything, "query-1", "admin").
					Return(&domain.Query{ExecutionsCount: 8, MeanTimeMs: 50, M2: 175}, nil)
				repo.On("Save", mock.Anything, mock.Anything).Return(nil)
				pub.On("Publish", mock.Anything, mock.Anything).Return(nil)
			},
			wantErr: false,
		},
		{
			nome:   "log de erro ao publica alerta",
			execMs: 90,
			setupMocks: func(repo *MockQueryRepository, pub *MockAlertPublisher) {
				repo.On("GetByID", mock.Anything, "query-1", "admin").
					Return(&domain.Query{ExecutionsCount: 8, MeanTimeMs: 50, M2: 175}, nil)
				repo.On("Save", mock.Anything, mock.Anything).Return(nil)
				pub.On("Publish", mock.Anything, mock.Anything).Return(errors.New("erro ao publicar alerta"))
			},
			wantErr: false,
		},
		{
			nome:   "execução normal",
			execMs: 52,
			setupMocks: func(repo *MockQueryRepository, pub *MockAlertPublisher) {
				repo.On("GetByID", mock.Anything, "query-1", "admin").
					Return(&domain.Query{ExecutionsCount: 8, MeanTimeMs: 50, M2: 175}, nil)
				repo.On("Save", mock.Anything, mock.Anything).Return(nil)
			},
			wantErr: false,
		},
		{
			nome:   "erro query não encontrada",
			execMs: 0,
			setupMocks: func(repo *MockQueryRepository, pub *MockAlertPublisher) {
				repo.On("GetByID", mock.Anything, "query-1", "admin").
					Return(&domain.Query{}, domain.ErrQueryNotFound)
				repo.On("Save", mock.Anything, mock.Anything).Return(nil)
			},
			wantErr: false,
		},
		{
			nome:   "erro no salvamento",
			execMs: 8,
			setupMocks: func(repo *MockQueryRepository, pub *MockAlertPublisher) {
				repo.On("GetByID", mock.Anything, "query-1", "admin").
					Return(&domain.Query{ExecutionsCount: 8, MeanTimeMs: 50, M2: 175}, nil)
				repo.On("Save", mock.Anything, mock.Anything).Return(errors.New("Erro ao tentar salvar"))
				pub.On("Publish", mock.Anything, mock.Anything).Return(nil)
			},
			wantErr: true,
		},
		{
			nome:   "erro ao buscar a query",
			execMs: 8,
			setupMocks: func(repo *MockQueryRepository, pub *MockAlertPublisher) {
				repo.On("GetByID", mock.Anything, "query-1", "admin").
					Return(nil, errors.New("Erro ao buscar no banco"))

			},
			wantErr: true,
		},
	}

	for _, teste := range casos {
		t.Run(teste.nome, func(t *testing.T) {
			repoMock := new(MockQueryRepository)
			pubMock := new(MockAlertPublisher)

			teste.setupMocks(repoMock, pubMock)

			uc := NewAnalyzeQueryUseCase(repoMock, pubMock)

			err := uc.Execute(context.Background(), "query-1", "admin", "select * from teste", teste.execMs)

			if teste.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			repoMock.AssertExpectations(t)
			pubMock.AssertExpectations(t)
		})
	}

}
