package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/novel2av/backend/internal/domain"
	"github.com/novel2av/backend/internal/infra/db/repo"
	"github.com/novel2av/backend/internal/infra/queue"
	"github.com/novel2av/backend/internal/infra/storage"
)

type ChapterService struct {
	db        *pgxpool.Pool
	storage   *storage.MinIOClient
	chapters  *repo.ChapterRepo
	projects  *repo.ProjectRepo
	queue     *queue.AsynqClient
}

func NewChapterService(db *pgxpool.Pool, st *storage.MinIOClient, q *queue.AsynqClient) *ChapterService {
	return &ChapterService{
		db:       db,
		storage:  st,
		chapters: repo.NewChapterRepo(db),
		projects: repo.NewProjectRepo(db),
		queue:    q,
	}
}

func (s *ChapterService) List(ctx context.Context, projectID string) ([]domain.Chapter, error) {
	return s.chapters.ListByProject(ctx, projectID)
}

func (s *ChapterService) Get(ctx context.Context, id string) (domain.Chapter, error) {
	return s.chapters.Get(ctx, id)
}

// Patch supports rename and manual status update.
func (s *ChapterService) Patch(ctx context.Context, id string, title *string, status *string) (domain.Chapter, error) {
	if title != nil && *title == "" {
		return domain.Chapter{}, fmt.Errorf("%w: title cannot be empty", domain.ErrInvalidInput)
	}
	return s.chapters.Patch(ctx, id, title, status)
}

// TriggerSplit enqueues `ai:split_chapters` for a project.
func (s *ChapterService) TriggerSplit(ctx context.Context, projectID string) (string, error) {
	p, err := s.projects.Get(ctx, projectID)
	if err != nil {
		return "", err
	}
	body, _ := json.Marshal(SplitChaptersBackendPayload{
		ProjectID: projectID,
		SourceKey: p.SourceKey,
	})
	return s.queue.Enqueue(ctx, "ai:split_chapters", body, 3)
}

// IngestSplitResult is called after the ai-engine worker finishes. It downloads
// the JSON manifest and upserts every chapter row.
//
// In production this is invoked by a callback worker that listens on
// Redis channel `events:job:<job_id>`; for now it can be triggered by CLI
// (`novel2av chapter ingest <project_id>`) for easy testing.
func (s *ChapterService) IngestSplitResult(ctx context.Context, projectID string) (int, error) {
	key := fmt.Sprintf("results/%s/split_chapters.json", projectID)
	body, err := s.downloadResult(ctx, key)
	if err != nil {
		return 0, fmt.Errorf("download %s: %w", key, err)
	}
	var manifest struct {
		Chapters []struct {
			Index       int    `json:"index"`
			Title       string `json:"title"`
			StartOffset int    `json:"start_offset"`
			EndOffset   int    `json:"end_offset"`
			Key         string `json:"key"`
		} `json:"chapters"`
	}
	if err := json.Unmarshal(body, &manifest); err != nil {
		return 0, fmt.Errorf("parse manifest: %w", err)
	}
	// Stable order by index.
	sort.Slice(manifest.Chapters, func(i, j int) bool {
		return manifest.Chapters[i].Index < manifest.Chapters[j].Index
	})

	count := 0
	for _, ch := range manifest.Chapters {
		_, err := s.chapters.Upsert(ctx, domain.Chapter{
			ProjectID:  projectID,
			Index:      ch.Index,
			Title:      ch.Title,
			ContentKey: ch.Key,
			WordCount:  0, // populated on demand; ai-engine already wrote content in the JSON
			Status:     "READY",
		})
		if err != nil {
			return count, fmt.Errorf("upsert chapter %d: %w", ch.Index, err)
		}
		count++
	}

	// Update project status.
	if _, err := s.db.Exec(ctx,
		`UPDATE projects SET status = $2, updated_at = $3 WHERE id = $1`,
		projectID, domain.ProjectSplit, time.Now().UTC()); err != nil {
		return count, fmt.Errorf("update project status: %w", err)
	}
	return count, nil
}

// --- helpers ---------------------------------------------------------------

func (s *ChapterService) downloadResult(ctx context.Context, key string) ([]byte, error) {
	url, err := s.storage.PresignGet(ctx, key, 5*time.Minute)
	if err != nil {
		return nil, err
	}
	return fetchURL(ctx, url.String())
}

type SplitChaptersBackendPayload struct {
	ProjectID string `json:"project_id"`
	SourceKey string `json:"source_key"`
}
