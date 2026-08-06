package config

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/redis/go-redis/v9"
)

type ClientRedis struct {
	Client *redis.Client
}

func NewRedisConn() *ClientRedis {
	ctx := context.Background()

	host := os.Getenv("REDIS_HOST")
	port := os.Getenv("REDIS_PORT")
	pass := os.Getenv("REDIS_PASSWORD")

	addr := fmt.Sprintf("%s:%s", host, port)

	options := &redis.Options{
		Addr:     addr,
		Password: pass,
		DB:       0,
	}

	client := redis.NewClient(options)

	err := client.Ping(ctx).Err()

	if err != nil {
		log.Fatalf("error ao tentar conectar o redis: %v", err)
	}

	return &ClientRedis{
		Client: client,
	}
}
