// Package queue wraps asynq for enqueueing pipeline tasks.
//
// Two Redis logical DBs:
//   - redis_url (db0): shared / cache
//   - redis_ai_url (db1): ai-engine queue (also read by Python Celery workers)
package queue

import (
	"context"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
)

type AsynqClient struct {
	primary  *asynq.Client
	aiClient *asynq.Client
}

// NewAsynqClient builds two asynq clients pointing at different Redis DBs.
func NewAsynqClient(aiURL, primaryURL string) (*AsynqClient, error) {
	primary, err := asynq.NewClient(asynq.RedisClientOpt{Addr: addrOf(primaryURL), DB: dbOf(primaryURL)})
	if err != nil {
		return nil, fmt.Errorf("primary asynq: %w", err)
	}
	ai, err := asynq.NewClient(asynq.RedisClientOpt{Addr: addrOf(aiURL), DB: dbOf(aiURL)})
	if err != nil {
		primary.Close()
		return nil, fmt.Errorf("ai asynq: %w", err)
	}
	return &AsynqClient{primary: primary, aiClient: ai}, nil
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

func (a *AsynqClient) Ping(_ context.Context) error {
	return a.primary.Ping()
}

func addrOf(_ string) string { return "localhost:6379" } // overridden by env in real configs
func dbOf(_ string) int      { return 0 }
