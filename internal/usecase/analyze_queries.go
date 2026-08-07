package usecase

import "sql-analyze/internal/domain"

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
