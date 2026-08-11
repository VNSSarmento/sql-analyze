package domain

import (
	"math"
	"time"
)

type Query struct {
	QueryID         string
	DBUser          string
	NormalizedQuery string
	ExecutionsCount int64
	MeanTimeMs      float64
	M2              float64
	LastExecutionAt *time.Time
	LastAnomalyAt   *time.Time
	CreatedAt       time.Time
}

type AnomalyAlert struct {
	QueryID       string
	DBUser        string
	CurrentTimeMs float64
	MeanTimeMs    float64
	ZScore        float64
	DetectedAt    time.Time
}

type ExecutionResult struct {
	IsAnomaly bool
	ZScore    float64
}

func (q *Query) RegisterExecution(execMs float64) ExecutionResult {
	now := time.Now()
	result := ExecutionResult{}

	if q.ExecutionsCount >= 8 {
		variance := q.M2 / float64(q.ExecutionsCount-1)
		desvP := math.Sqrt(variance)

		if desvP == 0 {
			if execMs != q.MeanTimeMs {
				result.IsAnomaly = true
			}
		} else {
			zScore := (execMs - q.MeanTimeMs) / desvP
			if math.Abs(zScore) > 3.0 {
				result.IsAnomaly = true
				result.ZScore = zScore
			}
		}

		if result.IsAnomaly {
			q.LastAnomalyAt = &now
		}
	}

	q.ExecutionsCount++
	delta1 := execMs - q.MeanTimeMs
	q.MeanTimeMs += delta1 / float64(q.ExecutionsCount)
	delta2 := execMs - q.MeanTimeMs
	q.M2 += delta1 * delta2

	q.LastExecutionAt = &now

	return result
}
