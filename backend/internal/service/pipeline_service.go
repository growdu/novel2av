package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/novel2av/backend/internal/infra/queue"
)

// PipelineJobPayload is what backend writes into the queue for ai-engine to consume.
type PipelineJobPayload struct {
	ProjectID string   `json:"project_id"`
	Step      string   `json:"step"` // ai:split_chapters | ai:extract_characters | ...
	RefID     string   `json:"ref_id,omitempty"`
	Options   PipelineOptions `json:"options,omitempty"`
}

type PipelineOptions struct {
	ImageProvider string `json:"image_provider,omitempty"`
	TTSProvider   string `json:"tts_provider,omitempty"`
	Aspect        string `json:"aspect,omitempty"`
}

// PipelineService orchestrates the long-running pipeline by enqueueing tasks.
// All actual work happens in ai-engine workers.
type PipelineService struct {
	db    *pgxpool.Pool
	queue *queue.AsynqClient
	hub   *EventHub
}

func NewPipelineService(db *pgxpool.Pool, q *queue.AsynqClient, hub *EventHub) *PipelineService {
	return &PipelineService{db: db, queue: q, hub: hub}
}

// EnqueueStep pushes one pipeline step to ai-engine. Idempotency is the
// caller's responsibility (use deterministic Step + RefID).
func (s *PipelineService) EnqueueStep(ctx context.Context, p PipelineJobPayload) (string, error) {
	if p.ProjectID == "" || p.Step == "" {
		return "", fmt.Errorf("project_id and step are required")
	}
	body, err := json.Marshal(p)
	if err != nil {
		return "", err
	}
	id, err := s.queue.Enqueue(ctx, p.Step, body, 3)
	if err != nil {
		return "", err
	}
	s.hub.Publish(ctx, p.ProjectID, Event{Type: "job.queued", JobID: id, Step: p.Step})
	return id, nil
}
