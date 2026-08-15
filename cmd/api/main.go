package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
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

	err := godotenv.Load()

	if err != nil {
		log.Println("Aviso: .env não encontrado, seguindo com variáveis do ambiente")
	}

	ctx := context.Background()
	collectInterval := time.Minute * 2

	bd := config.NewPostgresConn()

	if bd != nil {
		fmt.Println("Banco conectado com sucesso")
	}

	redisConn := config.NewRedisConn()
	alertPublisher := redisadapter.NewStreamAlertPublisher(redisConn.Client)

	if err := alertPublisher.EnsureConsumerGroup(ctx); err != nil {
		log.Fatalf("Erro ao garantir consumer group: %v", err)
	}

	repository := postgres.NewPostgresQueryRepository(bd.ConnPool)

	usecase := usecase.NewAnalyzeQueryUseCase(repository, alertPublisher)

	collectorWorker := worker.NewCollector(bd.ConnPool, usecase, collectInterval)

	go collectorWorker.Start(ctx)

	route := gin.Default()

	route.GET("/health", func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{"message": "tá batendo"})
	})

	route.Run()
}
