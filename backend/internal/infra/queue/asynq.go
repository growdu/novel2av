// Package queue wraps asynq for enqueueing pipeline tasks.
package queue

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
)

type AsynqClient struct {
	primary  *asynq.Client
	aiClient *asynq.Client
	rdb      *redis.Client
}

func NewAsynqClient(aiURL, primaryURL string) (*AsynqClient, *EventBus, error) {
	primary, err := asynq.NewClient(asynq.RedisClientOpt{Addr: addrOf(primaryURL), DB: dbOf(primaryURL)})
	if err != nil {
		return nil, nil, fmt.Errorf("primary asynq: %w", err)
	}
	ai, err := asynq.NewClient(asynq.RedisClientOpt{Addr: addrOf(aiURL), DB: dbOf(aiURL)})
	if err != nil {
		primary.Close()
		return nil, nil, fmt.Errorf("ai asynq: %w", err)
	}
	rdb := redis.NewClient(&redis.Options{Addr: addrOf(primaryURL), DB: dbOf(primaryURL)})
	bus := NewEventBus(rdb)
	return &AsynqClient{primary: primary, aiClient: ai, rdb: rdb}, bus, nil
}

// Enqueue pushes a task to the AI queue with sane defaults.
func (a *AsynqClient) Enqueue(_ context.Context, taskType string, payload []byte, maxRetry int) (string, error) {
	t := asynq.NewTask(taskType, payload,
		asynq.MaxRetry(maxRetry),
		asynq.Timeout(30*time.Minute),
		asynq.Queue("ai"),
	)
	info, err := a.aiClient.Enqueue(t)
	if err != nil {
		return "", err
	}
	return info.ID, nil
}

func (a *AsynqClient) Ping(_ context.Context) error { return a.primary.Ping() }

func addrOf(_ string) string { return "localhost:6379" }
func dbOf(_ string) int      { return 0 }

// ParseRedisURL is a small helper exported for future use (e.g. tests).
func ParseRedisURL(s string) (addr string, db int, err error) {
	u, err := url.Parse(s)
	if err != nil {
		return "", 0, err
	}
	addr = u.Host
	if u.Path != "" && len(u.Path) > 1 {
		_, _ = fmt.Sscanf(u.Path[1:], "%d", &db)
	}
	return addr, db, nil
}
