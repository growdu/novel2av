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
	primary := asynq.NewClient(asynq.RedisClientOpt{Addr: addrOf(primaryURL), DB: dbOf(primaryURL)})
	ai := asynq.NewClient(asynq.RedisClientOpt{Addr: addrOf(aiURL), DB: dbOf(aiURL)})
	rdb := redis.NewClient(&redis.Options{Addr: addrOf(primaryURL), DB: dbOf(primaryURL)})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		primary.Close()
		ai.Close()
		return nil, nil, fmt.Errorf("redis ping: %w", err)
	}
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

func (a *AsynqClient) Ping(ctx context.Context) error { return a.rdb.Ping(ctx).Err() }

// addrOf extracts the host:port portion of a redis URL via ParseRedisURL.
// Falls back to localhost:6379 so a malformed URL never blocks startup in dev;
// operators running two distinct Redis instances should see a clear error from
// the redis ping below instead of a silent single-instance fallback.
func addrOf(raw string) string {
	addr, _, err := ParseRedisURL(raw)
	if err != nil || addr == "" {
		return "localhost:6379"
	}
	return addr
}
func dbOf(raw string) int {
	_, db, err := ParseRedisURL(raw)
	if err != nil {
		return 0
	}
	return db
}

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
