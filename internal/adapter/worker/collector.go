package worker

import (
	"context"
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

func (c *Collector) collectOnce(ctx context.Context) {

}

func (c *Collector) Start(ctx context.Context) {
	ticker := time.NewTicker(c.interval)

	for {
		select {
		case <-ticker.C:
			c.collectOnce(ctx)
		case <-ctx.Done():
			log.Println("Tempo Finalizado")
		}
	}
}
