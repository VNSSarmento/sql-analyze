package main

import (
	"fmt"
	"log"
	"net/http"
	"sql-analyze/internal/adapter/redisAdapter"
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
	alertPublisher := redisAdapter.NewStreamAlertPublisher(redisConn.Client)

	if redisConn != nil {
		fmt.Println("Redis conectado com sucesso")
	}

	route := gin.Default()

	route.GET("/health", func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{"message": "tá batendo"})
	})

	route.Run()
}
