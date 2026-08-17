package http

import (
	"sql-analyze/internal/domain"
	"time"
)

type SlowQueryResponse struct {
	QueryID         string     `json:"query_id"`
	DBUser          string     `json:"db_user"`
	NormalizedQuery string     `json:"normalized_query"`
	ExecutionsCount int64      `json:"executions_count"`
	MeanTimeMs      float64    `json:"mean_time_ms"`
	M2              float64    `json:"m2"`
	LastExecutionAt *time.Time `json:"last_executions_at"`
	LastAnomalyAt   *time.Time `json:"last_anomaly_at"`
	CreatedAt       time.Time  `json:"created_at"`
}

func mapToSlowQueryResponseList(domainQueries []*domain.Query) []SlowQueryResponse {

	queriesJson := make([]SlowQueryResponse, len(domainQueries))

	for index, query := range domainQueries {
		queriesJson[index].QueryID = query.QueryID
		queriesJson[index].DBUser = query.DBUser
		queriesJson[index].NormalizedQuery = query.NormalizedQuery
		queriesJson[index].ExecutionsCount = query.ExecutionsCount
		queriesJson[index].MeanTimeMs = query.MeanTimeMs
		queriesJson[index].M2 = query.M2
		queriesJson[index].LastExecutionAt = query.LastExecutionAt
		queriesJson[index].LastAnomalyAt = query.LastAnomalyAt
		queriesJson[index].CreatedAt = query.CreatedAt
	}

	return queriesJson

}
