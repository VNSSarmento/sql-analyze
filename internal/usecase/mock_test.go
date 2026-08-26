package usecase

import (
	"context"
	"sql-analyze/internal/domain"

	"github.com/stretchr/testify/mock"
)

type MockQueryRepository struct {
	mock.Mock
}

func (m *MockQueryRepository) GetByID(ctx context.Context, queryID, dbUser string) (*domain.Query, error) {
	args := m.Called(ctx, queryID, dbUser)

	var query *domain.Query
	if args.Get(0) != nil {
		query = args.Get(0).(*domain.Query)
	}

	return query, args.Error(1)
}

func (m *MockQueryRepository) Save(ctx context.Context, q *domain.Query) error {
	return m.Called(ctx, q).Error(0)
}

func (m *MockQueryRepository) GetTopSlowest(ctx context.Context, limit int) ([]*domain.Query, error) {
	args := m.Called(ctx, limit)

	var query []*domain.Query

	if args.Get(0) != nil {
		query = args.Get(0).([]*domain.Query)
	}

	return query, args.Error(1)
}

type MockAlertPublisher struct {
	mock.Mock
}

func (m *MockAlertPublisher) Publish(ctx context.Context, alert *domain.AnomalyAlert) error {
	return m.Called(ctx, alert).Error(0)
}

type MockCache struct {
	mock.Mock
}

func (m *MockCache) GetSlowest(ctx context.Context, limit int) ([]*domain.Query, error) {
	args := m.Called(ctx, limit)

	var query []*domain.Query

	if args.Get(0) != nil {
		query = args.Get(0).([]*domain.Query)
	}

	return query, args.Error(1)
}

func (m *MockCache) SetSlowest(ctx context.Context, limit int, queries []*domain.Query) error {
	return m.Called(ctx, limit, queries).Error(0)
}
