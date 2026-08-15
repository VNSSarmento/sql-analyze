package worker

import (
	"context"
	"fmt"
	"log"
	"sql-analyze/internal/usecase"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type statSnapshot struct {
	calls           int64
	totalExecTimeMs float64
}

type Collector struct {
	pool           *pgxpool.Pool
	analyzeUseCase *usecase.AnalyzeQueryUseCase
	interval       time.Duration
	snapshots      map[string]statSnapshot
}

func NewCollector(pool *pgxpool.Pool, analyzeUseCase *usecase.AnalyzeQueryUseCase, interval time.Duration) *Collector {
	return &Collector{
		pool:           pool,
		analyzeUseCase: analyzeUseCase,
		interval:       interval,
		snapshots:      make(map[string]statSnapshot),
	}
}

func (c *Collector) collectOnce(ctx context.Context) {

	sql := "SELECT pss.queryid::text, r.rolname AS db_user, pss.query AS normalized_query, pss.calls, pss.total_exec_time AS total_exec_time_ms FROM pg_stat_statements pss JOIN pg_roles r ON pss.userid = r.oid WHERE pss.calls > 0;"

	rows, err := c.pool.Query(ctx, sql)

	if err != nil {
		log.Printf("erro ao executar query no pg_stat_statements: %v", err)
		return
	}

	err = rows.Err()

	if err != nil {
		log.Printf("erro ao iterar resultados do pg_stat_statements: %v", err)
	}

	defer rows.Close()

	for rows.Next() {

		var queryID, dbUser, normalizedQuery string
		var currentCalls int64
		var currentTotalTime float64

		err := rows.Scan(
			&queryID, &dbUser,
			&normalizedQuery,
			&currentCalls,
			&currentTotalTime)

		if err != nil {
			log.Printf("erro no scan: %v", err)
			continue
		}

		key := fmt.Sprintf("%s:%s", queryID, dbUser)

		prevSnapshot, exist := c.snapshots[key]

		callDelta := currentCalls - prevSnapshot.calls
		timeDelta := currentTotalTime - prevSnapshot.totalExecTimeMs

		if !exist || callDelta <= 0 {
			c.snapshots[key] = statSnapshot{
				calls:           currentCalls,
				totalExecTimeMs: currentTotalTime,
			}
			continue
		}

		avgTimeMs := timeDelta / float64(callDelta)

		err = c.analyzeUseCase.Execute(ctx, queryID, dbUser, normalizedQuery, avgTimeMs)
		if err != nil {
			log.Printf("work error: %v", err)
		}

		c.snapshots[key] = statSnapshot{calls: callDelta, totalExecTimeMs: currentTotalTime}

	}
}

func (c *Collector) Start(ctx context.Context) {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.collectOnce(ctx)
		}
	}
}
