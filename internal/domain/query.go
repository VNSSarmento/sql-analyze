package domain

import "time"

type Query struct {
	QueryId         string
	DbUser          string
	NormalizedQuery string
	ExecutionsCount int64
	MeanTimeMs      float64
	M2              float64
	LastExecutionAt time.Time
	LastAnomalyAt   time.Time
	CreatedAt       time.Time
}
