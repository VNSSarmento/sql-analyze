package redisadapter

import (
	"sql-analyze/internal/domain"
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

func mapAnomalyAlertResponse(domainAlert *domain.AnomalyAlert) AnomalyAlertResponse {
	alertJson := AnomalyAlertResponse{}

	alertJson.QueryID = domainAlert.QueryID
	alertJson.DBUser = domainAlert.DBUser
	alertJson.CurrentTimeMs = domainAlert.CurrentTimeMs
	alertJson.MeanTimeMs = domainAlert.MeanTimeMs
	alertJson.ZScore = domainAlert.ZScore
	alertJson.DetectedAt = domainAlert.DetectedAt

	return alertJson
}
