package worker

import (
	"context"
	"errors"
	"fmt"
	"log"
	redisadapter "sql-analyze/internal/adapter/redis_adapter"
	"sql-analyze/internal/domain"
	"strconv"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

type AlertConsumer struct {
	Client       *redis.Client
	ConsumerName string
	MinIdleTime  time.Duration
	ContactRepo  domain.ContactRepository
}

func NewAlertConsumer(client *redis.Client, consumerName string, contactRepo domain.ContactRepository) *AlertConsumer {
	return &AlertConsumer{
		Client:       client,
		ConsumerName: consumerName,
		MinIdleTime:  time.Minute * 3,
		ContactRepo:  contactRepo,
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

	detectedAt, err := time.Parse(time.RFC3339, fmt.Sprintf("%v", values["detected_at"]))
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
		DetectedAt:    detectedAt,
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
			a.processMessage(ctx, message.ID, message.Values)
		}
	}
}

func (a *AlertConsumer) processMessage(ctx context.Context, messageID string, values map[string]any) {
	alert := parseMapToAlert(values)
	user, err := a.ContactRepo.GetByDBUser(ctx, alert.DBUser)

	switch {
	case errors.Is(err, domain.ErrContactNotFound):
		log.Printf("Alerta da query %s (usuário %s): sem contato cadastrado", alert.QueryID, alert.DBUser)
	case err != nil:
		log.Printf("erro ao buscar contato do usuário %s: %v", alert.DBUser, err)
	default:
		log.Printf("Alerta para %s (%s): query %s rodou %.2fms (média histórica %.2fms), %.2f desvios acima da média",
			user.DisplayName, user.Email, alert.QueryID, alert.CurrentTimeMs, alert.MeanTimeMs, alert.ZScore)
	}

	err = a.Client.XAck(ctx, redisadapter.StreamName, redisadapter.GroupName, messageID).Err()

	if err != nil {
		log.Printf("erro ao confirmar mensagem %s: %v", messageID, err)
	}
}

func (a *AlertConsumer) Start(ctx context.Context, wg *sync.WaitGroup) {
	wg.Add(2)

	go func() {
		defer wg.Done()
		a.runConsumeLoop(ctx)
	}()

	go func() {
		defer wg.Done()
		a.runCleanupLoop(ctx)
	}()
}

func (a *AlertConsumer) runConsumeLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			a.ConsumeNew(ctx)
		}
	}
}

func (a *AlertConsumer) runCleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(time.Minute * 1)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.ReclaimStale(ctx)
		}
	}
}

func (a *AlertConsumer) ReclaimStale(ctx context.Context) {
	args := redis.XAutoClaimArgs{
		Stream:   redisadapter.StreamName,
		Group:    redisadapter.GroupName,
		Consumer: a.ConsumerName,
		MinIdle:  a.MinIdleTime,
		Start:    "0-0",
		Count:    50,
	}

	messages, _, err := a.Client.XAutoClaim(ctx, &args).Result()

	if err != nil {
		log.Printf("erro ao verificar mensagem pendentes: %v", err)
		return
	}

	for _, message := range messages {
		a.processMessage(ctx, message.ID, message.Values)
	}
}
