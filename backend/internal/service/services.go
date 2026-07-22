package service

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/novel2av/backend/internal/infra/queue"
	"github.com/novel2av/backend/internal/infra/storage"
)

type Services struct {
	DB          *pgxpool.Pool
	Storage     *storage.MinIOClient
	Queue       *queue.AsynqClient
	Events      *queue.EventBus
	Pipeline    *PipelineService
	Asset       *AssetService
	Project     *ProjectService
	Chapter     *ChapterService
	Character   *CharacterService
	Shot        *ShotService
	Composition *CompositionService
}

func New(db *pgxpool.Pool, st *storage.MinIOClient, q *queue.AsynqClient, eb *queue.EventBus) *Services {
	hub := newEventHub()
	return &Services{
		DB:          db,
		Storage:     st,
		Queue:       q,
		Events:      eb,
		Pipeline:    NewPipelineService(db, q, hub),
		Asset:       NewAssetService(st),
		Project:     NewProjectService(db, st),
		Chapter:     NewChapterService(db, st, q),
		Character:   NewCharacterService(db, st, q),
		Shot:        NewShotService(db, st, q),
		Composition: NewCompositionService(db, st, q, eb),
	}
}

func (s *Services) Ping(ctx context.Context) error {
	if err := s.DB.Ping(ctx); err != nil {
		return err
	}
	if err := s.Queue.Ping(ctx); err != nil {
		return err
	}
	return nil
}
