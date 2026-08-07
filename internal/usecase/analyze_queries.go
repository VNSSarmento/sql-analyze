package usecase

import (
	"context"
	"math"
	"sql-analyze/internal/domain"
	"time"
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
		query = &domain.Query{
			QueryID:         queryID,
			DBUser:          dbUser,
			NormalizedQuery: normalizedQuery,
		}
	} else if err != nil {
		return err
	}

	now := time.Now()

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
			pubAlert := &domain.AnomalyAlert{
				QueryID:       query.QueryID,
				DBUser:        query.DBUser,
				CurrentTimeMs: executionTimeMs,
				ZScore:        zScore,
				DetectedAt:    now,
			}

			_ = a.pubAlert.Publish(ctx, pubAlert)

			query.LastAnomalyAt = &now
		}
	}

	query.ExecutionsCount++

	delta1 := executionTimeMs - query.MeanTimeMs

	query.MeanTimeMs += delta1 / float64(query.ExecutionsCount)

	delta2 := executionTimeMs - query.MeanTimeMs

	query.M2 += delta1 * delta2

	query.LastExecutionAt = &now

	err = a.repository.Save(ctx, query)
	if err != nil {
		return err
	}

	return nil
}
