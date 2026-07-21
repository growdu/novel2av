// Package service is the cross-entity use case layer.
//
// It depends on infrastructure (DB, queue, storage) but never calls LLM/Image/TTS
// directly — those always go through the ai-engine via the queue.
package service

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/novel2av/backend/internal/infra/queue"
	"github.com/novel2av/backend/internal/infra/storage"
)

// Services bundles the use cases that handlers and CLI subcommands both call.
type Services struct {
	DB       *pgxpool.Pool
	Storage  *storage.MinIOClient
	Queue    *queue.AsynqClient
	Pipeline *PipelineService
	Asset    *AssetService
}

// New wires the service graph.
func New(db *pgxpool.Pool, st *storage.MinIOClient, q *queue.AsynqClient) *Services {
	hub := newEventHub()
	return &Services{
		DB:       db,
		Storage:  st,
		Queue:    q,
		Pipeline: NewPipelineService(db, q, hub),
		Asset:    NewAssetService(st),
	}
}

// Ping checks liveness of dependencies for /readyz.
func (s *Services) Ping(ctx context.Context) error {
	if err := s.DB.Ping(ctx); err != nil {
		return err
	}
	if err := s.Queue.Ping(ctx); err != nil {
		return err
	}
	return nil
}
