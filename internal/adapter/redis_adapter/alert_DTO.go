package redisadapter

import (
	"time"
)

type AnomalyAlertResponse struct {
	QueryID       string
	DBUser        string
	CurrentTimeMs float64
	MeanTimeMs    float64
	ZScore        float64
	DetectedAt    time.Time
}
