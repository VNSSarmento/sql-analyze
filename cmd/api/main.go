package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sql-analyze/internal/adapter/http/handler"
	"sql-analyze/internal/adapter/postgres"
	redisadapter "sql-analyze/internal/adapter/redis_adapter"
	"sql-analyze/internal/adapter/worker"
	"sql-analyze/internal/config"
	"sql-analyze/internal/usecase"
	"sync"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

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
	alertConsumer := worker.NewAlertConsumer(redisConn.Client, "alert-consumer-1", contactRepository)

	var wg sync.WaitGroup

	go collector.Start(ctx, &wg)
	go alertConsumer.Start(ctx, &wg)

	route := gin.Default()

	route.GET("/health", func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{"message": "tá batendo"})
	})

	route.GET("/queries/slowest", h.GetSlowestQueries)
	route.GET("/queries/:queryID/:dbUser", h.GetQueryById)

	srv := &http.Server{Addr: ":8080", Handler: route}

	go func() {
		err := srv.ListenAndServe()

		if err != nil && err != http.ErrServerClosed {
			log.Fatalf("erro no servidor HTTP: %v", err)
		}
	}()

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("erro ao encerrar servidor HTTP: %v", err)
	}

	wg.Wait()

	bd.ConnPool.Close()

	if err := redisConn.Client.Close(); err != nil {
		log.Printf("erro ao fechar client Redis: %v", err)
	}
}
