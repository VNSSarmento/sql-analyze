package config

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Bd struct {
	ConnPool *pgxpool.Pool
}

func NewPostgresConn() *Bd {
	ctx := context.Background()

	host := os.Getenv("POSTGRES_HOST")
	port := os.Getenv("POSTGRES_PORT")
	user := os.Getenv("POSTGRES_USER")
	pass := os.Getenv("POSTGRES_PASSWORD")
	db := os.Getenv("POSTGRES_DB")
	sslmode := os.Getenv("POSTGRES_SSLMODE")
	timezone := os.Getenv("POSTGRES_TIMEZONE")

	connString := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=%s TimeZone=%s", host, user, pass, db, port, sslmode, timezone)

	pool, err := pgxpool.New(ctx, connString)

	if err != nil {
		log.Fatalf("Não foi possivel conectar com o banco de dados: %v", err)
	}

	if err := pool.Ping(ctx); err != nil {
		log.Fatalf("Banco de dados inacessível: %v", err)
	}

	return &Bd{ConnPool: pool}
}
