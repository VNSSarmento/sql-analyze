package main

import (
	"fmt"
	"sql-analyze/internal/config"
)

func main() {
	bd := config.NewConn()

	if bd != nil {
		fmt.Println("Banco conectado com sucesso")
	}
}
