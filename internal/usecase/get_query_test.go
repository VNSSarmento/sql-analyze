package usecase

import (
	"context"
	"errors"
	"sql-analyze/internal/domain"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestGetQuery(t *testing.T) {
	casos := []struct {
		nome      string
		queryID   string
		dbUser    string
		setupMock func(repo *MockQueryRepository)
		wantErr   bool
	}{
		{
			nome:    "execucao normal",
			queryID: "query-01",
			dbUser:  "admin",
			setupMock: func(repo *MockQueryRepository) {
				repo.On("GetByID", mock.Anything, "query-01", "admin").
					Return(&domain.Query{ExecutionsCount: 8, MeanTimeMs: 50, M2: 175}, nil)
			},
			wantErr: false,
		},
		{
			nome:    "erro repositorio",
			queryID: "query-01",
			dbUser:  "admin",
			setupMock: func(repo *MockQueryRepository) {
				repo.On("GetByID", mock.Anything, "query-01", "admin").Return(nil, errors.New("error ao acessar o banco"))
			},
			wantErr: true,
		},
	}

	for _, caso := range casos {
		t.Run(caso.nome, func(t *testing.T) {
			repo := new(MockQueryRepository)

			caso.setupMock(repo)

			uc := NewGetQueryUseCase(repo)

			_, err := uc.Execute(context.Background(), caso.queryID, caso.dbUser)

			if caso.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			repo.AssertExpectations(t)
		})
	}
}
