package service

import (
	"bytes"
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

type ProjectVideoService struct {
	db        *pgxpool.Pool
	storage   *storage.MinIOClient
	projects  *repo.ProjectRepo
	chapters  *repo.ChapterRepo
	chapVids  *repo.ChapterVideoRepo
	fullVids  *repo.ProjectVideoRepo
	queue     *queue.AsynqClient
	events    *queue.EventBus
}

func NewProjectVideoService(db *pgxpool.Pool, st *storage.MinIOClient, q *queue.AsynqClient, eb *queue.EventBus) *ProjectVideoService {
	return &ProjectVideoService{
		db:        db,
		storage:   st,
		projects:  repo.NewProjectRepo(db),
		chapters:  repo.NewChapterRepo(db),
		chapVids:  repo.NewChapterVideoRepo(db),
		fullVids:  repo.NewProjectVideoRepo(db),
		queue:     q,
		events:    eb,
	}
}

func (s *ProjectVideoService) Get(ctx context.Context, projectID string) (repo.ProjectVideo, error) {
	return s.fullVids.Get(ctx, projectID)
}

// TriggerCompose enqueues ai:compose_full after publishing a manifest to
// MinIO that lists every chapter video in order.
func (s *ProjectVideoService) TriggerCompose(ctx context.Context, projectID string) (string, error) {
	// Defensive: tests (and any future partial-init path) should not panic on
	// a nil repo; surface a typed error matching the test contract.
	if s.projects == nil {
		return "", fmt.Errorf("project repo not configured")
	}
	p, err := s.projects.Get(ctx, projectID)
	if err != nil {
		return "", err
	}
	chs, err := s.chapters.ListByProject(ctx, projectID)
	if err != nil {
		return "", err
	}
	vids, err := s.chapVids.ListByProject(ctx, projectID)
	if err != nil {
		return "", err
	}
	chapByID := make(map[string]domain.Chapter, len(chs))
	for _, c := range chs {
		chapByID[c.ID] = c
	}
	// Stable order: chapter.index.
	type entry struct {
		ChapterID string  `json:"chapter_id"`
		Index     int     `json:"index"`
		Title     string  `json:"title"`
		VideoKey  string  `json:"video_key"`
		Aspect    string  `json:"aspect"`
	}
	items := make([]entry, 0, len(vids))
	for _, v := range vids {
		ch, ok := chapByID[v.ChapterID]
		if !ok || v.VideoKey == "" {
			continue
		}
		items = append(items, entry{
			ChapterID: v.ChapterID, Index: ch.Index, Title: ch.Title,
			VideoKey: v.VideoKey, Aspect: p.Config.Aspect,
		})
	}
	if len(items) == 0 {
		return "", fmt.Errorf("%w: no chapter videos ready", domain.ErrConflict)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Index < items[j].Index })

	manifest := map[string]any{"chapters": items}
	projectMeta := map[string]any{"title": p.Title}
	body1, _ := json.Marshal(manifest)
	if err := s.upload(ctx, fmt.Sprintf("results/%s/full.json", projectID), body1); err != nil {
		return "", fmt.Errorf("upload manifest: %w", err)
	}
	body2, _ := json.Marshal(projectMeta)
	if err := s.upload(ctx, fmt.Sprintf("results/%s/project.json", projectID), body2); err != nil {
		return "", fmt.Errorf("upload project meta: %w", err)
	}

	// Mark project_videos row as COMPOSING + flip project status.
	_, _ = s.fullVids.Upsert(ctx, repo.ProjectVideo{ProjectID: projectID, Status: "COMPOSING"})
	if _, err := s.db.Exec(ctx,
		`UPDATE projects SET status=$2, updated_at=$3 WHERE id=$1`,
		projectID, domain.ProjectRunning, time.Now().UTC()); err != nil {
		return "", err
	}

	payload, _ := json.Marshal(ComposeFullBackendPayload{ProjectID: projectID})
	jobID, err := s.queue.Enqueue(ctx, "ai:compose_full", payload, 2)
	if err != nil {
		return "", err
	}
	s.events.Publish(ctx, projectID, queue.ProgressEvent{
		Type: "project.compose.start", ProjectID: projectID,
		JobID: jobID, Status: "queued",
	})
	return jobID, nil
}

// IngestComposeResult updates the row + flips project to DONE on READY.
func (s *ProjectVideoService) IngestComposeResult(ctx context.Context, projectID, videoKey string, durationSec float64, status, errMsg string) error {
	_, err := s.fullVids.Upsert(ctx, repo.ProjectVideo{
		ProjectID: projectID, VideoKey: videoKey, DurationSec: durationSec,
		Status: status, Error: errMsg,
	})
	if err != nil {
		return err
	}
	if status == "READY" {
		if _, err := s.db.Exec(ctx,
			`UPDATE projects SET status=$2, updated_at=$3 WHERE id=$1`,
			projectID, domain.ProjectDone, time.Now().UTC()); err != nil {
			return err
		}
		s.events.Publish(ctx, projectID, queue.ProgressEvent{
			Type: "project.ready", ProjectID: projectID,
		})
	}
	return nil
}

func (s *ProjectVideoService) upload(ctx context.Context, key string, body []byte) error {
	return s.storage.PutObject(ctx, key, bytes.NewReader(body), int64(len(body)), "application/json")
}

type ComposeFullBackendPayload struct {
	ProjectID string `json:"project_id"`
}
