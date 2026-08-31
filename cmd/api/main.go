package main

import (
	"context"
	"log"
	"net/http"
	"sql-analyze/internal/adapter/http/handler"
	"sql-analyze/internal/adapter/postgres"
	redisadapter "sql-analyze/internal/adapter/redis_adapter"
	"sql-analyze/internal/adapter/worker"
	"sql-analyze/internal/config"
	"sql-analyze/internal/usecase"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	ctx := context.Background()
	collectInterval := time.Minute * 2

	if err := godotenv.Load(); err != nil {
		log.Println("Aviso: .env não encontrado, seguindo com variáveis do ambiente")
	}

	bd := config.NewPostgresConn()
	redisConn := config.NewRedisConn()

	alertPublisher := redisadapter.NewStreamAlertPublisher(redisConn.Client)

	if err := alertPublisher.EnsureConsumerGroup(ctx); err != nil {
		log.Fatalf("Erro ao garantir consumer group: %v", err)
	}

	repository := postgres.NewPostgresQueryRepository(bd.ConnPool)
	contactRepository := postgres.NewContactRepository(bd.ConnPool)

	analyzeUseCase := usecase.NewAnalyzeQueryUseCase(repository, alertPublisher)
	queryCache := redisadapter.NewQueryCacheAdapter(redisConn.Client)
	listSlowerUseCase := usecase.NewListSlowestQueriesUseCase(repository, queryCache)
	getQueryUseCase := usecase.NewGetQueryUseCase(repository)

	h := handler.NewHandler(analyzeUseCase, listSlowerUseCase, getQueryUseCase)

	collector := worker.NewCollector(bd.ConnPool, analyzeUseCase, collectInterval)

	go collector.Start(ctx)

	alertConsumer := worker.NewAlertConsumer(redisConn.Client, "alert-consumer-1", contactRepository)

	go alertConsumer.Start(ctx)

	route := gin.Default()

	route.GET("/health", func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{"message": "tá batendo"})
	})

	route.GET("/queries/slowest", h.GetSlowestQueries)
	route.GET("/queries/:queryID/:dbUser", h.GetQueryById)

	route.Run()
}
