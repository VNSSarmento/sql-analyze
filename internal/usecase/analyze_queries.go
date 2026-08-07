package usecase

import (
	"context"
	"math"
	"sql-analyze/internal/domain"
)

type AnalyzeQueryUseCase struct {
	repository domain.QueryRepository
	pubAlert   domain.AlertPublisher
}

func NewAnalyzeQueryUseCase(repository domain.QueryRepository, publishe domain.AlertPublisher) *AnalyzeQueryUseCase {
	return &AnalyzeQueryUseCase{
		repository: repository,
		pubAlert:   publishe,
	}
}

func (a *AnalyzeQueryUseCase) Execute(ctx context.Context, queryID string, dbUser string, normalizedQuery string, executionTimeMs float64) error {
	query, err := a.repository.GetByID(ctx, queryID, dbUser)

	if err == domain.ErrQueryNotFound {
		query.QueryID = queryID
		query.DBUser = dbUser
		query.NormalizedQuery = normalizedQuery
		query.MeanTimeMs = executionTimeMs
	}

	if query.ExecutionsCount >= 8 {
		var zScore float64

		variance := query.M2 / float64(query.ExecutionsCount)
		desvP := math.Sqrt(variance)

		if desvP == 0 {
			zScore = 0
		} else {
			zScore = (executionTimeMs - query.MeanTimeMs) / desvP
		}

		if math.Abs(zScore) > 3.0 {

		}
	}
}
