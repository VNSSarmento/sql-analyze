package usecase

import (
	"context"
	"errors"
	"log"
	"sql-analyze/internal/domain"
	"time"
)

type AnalyzeQueryUseCase struct {
	repository domain.QueryRepository
	pubAlert   domain.AlertPublisher
}

func NewAnalyzeQueryUseCase(repository domain.QueryRepository, publisher domain.AlertPublisher) *AnalyzeQueryUseCase {
	return &AnalyzeQueryUseCase{
		repository: repository,
		pubAlert:   publisher,
	}
}
func (a *AnalyzeQueryUseCase) Execute(ctx context.Context, queryID string, dbUser string, normalizedQuery string, executionTimeMs float64) error {
	query, err := a.repository.GetByID(ctx, queryID, dbUser)

	if errors.Is(err, domain.ErrQueryNotFound) {
		query = &domain.Query{
			QueryID:         queryID,
			DBUser:          dbUser,
			NormalizedQuery: normalizedQuery,
		}
	} else if err != nil {
		return err
	}

	now := time.Now()

	result := query.RegisterExecution(executionTimeMs)

	if result.IsAnomaly {
		alert := &domain.AnomalyAlert{
			QueryID:       query.QueryID,
			DBUser:        query.DBUser,
			CurrentTimeMs: executionTimeMs,
			MeanTimeMs:    query.MeanTimeMs,
			ZScore:        result.ZScore,
			DetectedAt:    now,
		}
		if err := a.pubAlert.Publish(ctx, alert); err != nil {
			log.Println(err)
		}
	}

	err = a.repository.Save(ctx, query)
	if err != nil {
		return err
	}

	return nil
}
