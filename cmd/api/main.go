package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	redisadapter "sql-analyze/internal/adapter/redis_adapter"
	"sql-analyze/internal/config"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {

	err := godotenv.Load()

	if err != nil {
		log.Println("Aviso: .env não encontrado, seguindo com variáveis do ambiente")
	}

	bd := config.NewPostgresConn()

	if bd != nil {
		fmt.Println("Banco conectado com sucesso")
	}

	redisConn := config.NewRedisConn()
	alertPublisher := redisadapter.NewStreamAlertPublisher(redisConn.Client)

	if err := alertPublisher.EnsureConsumerGroup(context.Background()); err != nil {
		log.Fatalf("Erro ao garantir consumer group: %v", err)
	}

	route := gin.Default()

	route.GET("/health", func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{"message": "tá batendo"})
	})

	route.Run()
}
