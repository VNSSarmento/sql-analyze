package main

import (
	"fmt"
	"net/http"
	"sql-analyze/internal/config"

	"github.com/gin-gonic/gin"
)

func main() {
	bd := config.NewConn()
	route := gin.Default()

	if bd != nil {
		fmt.Println("Banco conectado com sucesso")
	}

	route.GET("/health", func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{"message": "tá batendo"})
	})

	route.Run()
}
