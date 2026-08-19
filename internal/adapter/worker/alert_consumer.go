package worker

import (
	"context"
	"errors"
	"fmt"
	"log"
	redisadapter "sql-analyze/internal/adapter/redis_adapter"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

type AlertConsumer struct {
	Client       *redis.Client
	ConsumerName string
	MinIdleTime  time.Duration
}

func NewAlertConsumer(client *redis.Client, consumerName string) *AlertConsumer {
	return &AlertConsumer{
		Client:       client,
		ConsumerName: consumerName,
		MinIdleTime:  time.Minute * 3,
	}
}

func parseMapToAlert(values map[string]any) *redisadapter.AnomalyAlertResponse {

	zscore, err := strconv.ParseFloat(fmt.Sprintf("%v", values["z_score"]), 64)
	if err != nil {
		log.Printf("error na conversão: %v", err)
	}

	currentTimeMs, err := strconv.ParseFloat(fmt.Sprintf("%v", values["current_time"]), 64)
	if err != nil {
		log.Printf("error na conversão: %v", err)
	}

	meanTimeMs, err := strconv.ParseFloat(fmt.Sprintf("%v", values["mean_time_ms"]), 64)
	if err != nil {
		log.Printf("error na conversão: %v", err)
	}

	deletedtAt, err := time.Parse(time.RFC3339, fmt.Sprintf("%v", values["detected_at"]))
	if err != nil {
		log.Println("error na conversão:", err)
	}

	queryID := fmt.Sprintf("%v", values["query_id"])
	dbUser := fmt.Sprintf("%v", values["db_user"])

	data := redisadapter.AnomalyAlertResponse{
		QueryID:       queryID,
		DBUser:        dbUser,
		ZScore:        zscore,
		CurrentTimeMs: currentTimeMs,
		MeanTimeMs:    meanTimeMs,
		DetectedAt:    deletedtAt,
	}

	return &data

}

func (a *AlertConsumer) ConsumeNew(ctx context.Context) {
	args := redis.XReadGroupArgs{
		Group:    redisadapter.GroupName,
		Consumer: a.ConsumerName,
		Streams:  []string{redisadapter.StreamName, ">"},
		Count:    10,
		Block:    time.Second * 2,
	}

	streamsResult, err := a.Client.XReadGroup(ctx, &args).Result()

	if errors.Is(err, redis.Nil) {
		log.Println("consumer alert: fila vazia")
		return
	}

	if err != nil {
		log.Printf("erro ao ler grupo do redis: %v", err)
		return
	}

	for _, stream := range streamsResult {
		for _, message := range stream.Messages {
			alert := parseMapToAlert(message.Values)

			log.Printf("Alerta recebido da query: %s", alert.QueryID)

			a.Client.XAck(ctx, redisadapter.StreamName, redisadapter.GroupName, message.ID).Err()
		}
	}
}

func (a *AlertConsumer) Start(ctx context.Context) {
	ticker := time.NewTicker(time.Minute * 1)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			log.Println("Rodando limpeza de alertas")
		default:
			a.ConsumeNew(ctx)
		}
	}
}
