package main

import (
	"fmt"
	"sql-analyze/internal/config"
)

func main() {
	bd := config.NewConn()
	fmt.Println("Banco conectado com sucesso")
}
