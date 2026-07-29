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

type CompositionService struct {
	db        *pgxpool.Pool
	storage   *storage.MinIOClient
	chapters  *repo.ChapterRepo
	shots     *repo.ShotRepo
	videos    *repo.ChapterVideoRepo
	projects  *repo.ProjectRepo
	queue     *queue.AsynqClient
	events    *queue.EventBus
}

func NewCompositionService(db *pgxpool.Pool, st *storage.MinIOClient, q *queue.AsynqClient, eb *queue.EventBus) *CompositionService {
	return &CompositionService{
		db:        db,
		storage:   st,
		chapters:  repo.NewChapterRepo(db),
		shots:     repo.NewShotRepo(db),
		videos:    repo.NewChapterVideoRepo(db),
		projects:  repo.NewProjectRepo(db),
		queue:     q,
		events:    eb,
	}
}

func (s *CompositionService) Get(ctx context.Context, chapterID string) (repo.ChapterVideo, error) {
	return s.videos.Get(ctx, chapterID)
}

func (s *CompositionService) ListByProject(ctx context.Context, projectID string) ([]repo.ChapterVideo, error) {
	return s.videos.ListByProject(ctx, projectID)
}

// TriggerCompose enqueues ai:compose_chapter for one chapter and uploads the
// per-shot manifest the worker needs (results/<id>/chapters/<cid>/shots.json).
func (s *CompositionService) TriggerCompose(ctx context.Context, chapterID, aspect string) (string, error) {
	// Defensive: tests (and any future partial-init path) should not panic on
	// a nil repo; surface a typed error so TriggerCompose still satisfies error-mapping tests.
	if s.chapters == nil {
		return "", fmt.Errorf("compose chapters repo not configured")
	}
	ch, err := s.chapters.Get(ctx, chapterID)
	if err != nil {
		return "", err
	}
	shots, err := s.shots.ListByProject(ctx, ch.ProjectID)
	if err != nil {
		return "", err
	}
	shot_payloads := make([]map[string]any, 0, len(shots))
	for _, sh := range shots {
		if sh.ChapterID != chapterID {
			continue
		}
		if sh.ImageKey == "" || sh.TTSKey == "" {
			return "", fmt.Errorf("%w: shot %s missing assets", domain.ErrConflict, sh.ID)
		}
		shot_payloads = append(shot_payloads, map[string]any{
			"shot_id":      sh.ID,
			"image_key":    sh.ImageKey,
			"tts_key":      sh.TTSKey,
			"bgm_key":      sh.BGMKey,
			"duration_sec": sh.DurationSec,
			"narration":    sh.Narration,
			"mood":         sh.Mood,
		})
	}
	if len(shot_payloads) == 0 {
		return "", fmt.Errorf("%w: no shots ready for chapter", domain.ErrConflict)
	}
	sort.Slice(shot_payloads, func(i, j int) bool {
		return shot_payloads[i]["shot_id"].(string) < shot_payloads[j]["shot_id"].(string)
	})
	manifest := map[string]any{"shots": shot_payloads}
	body, _ := json.Marshal(manifest)
	if err := s.uploadManifest(ctx, ch.ProjectID, ch.ID, body); err != nil {
		return "", fmt.Errorf("upload manifest: %w", err)
	}

	if aspect == "" {
		aspect = "9:16"
	}
	_, _ = s.videos.Upsert(ctx, repo.ChapterVideo{
		ChapterID: ch.ID, Status: "COMPOSING",
	})

	payload, _ := json.Marshal(ComposeChapterBackendPayload{
		ProjectID: ch.ProjectID, ChapterID: ch.ID, Aspect: aspect,
	})
	jobID, err := s.queue.Enqueue(ctx, "ai:compose_chapter", payload, 2)
	if err != nil {
		return "", err
	}
	s.events.Publish(ctx, ch.ProjectID, queue.ProgressEvent{
		Type: "chapter.compose.start", ProjectID: ch.ProjectID,
		ChapterID: ch.ID, JobID: jobID, Status: "queued",
	})
	return jobID, nil
}

// IngestComposeResult reads the worker's output key and updates chapter_videos.
func (s *CompositionService) IngestComposeResult(ctx context.Context, chapterID, videoKey string, durationSec float64, status string, errMsg string) error {
	_, err := s.videos.Upsert(ctx, repo.ChapterVideo{
		ChapterID: chapterID, VideoKey: videoKey, DurationSec: durationSec,
		Status: status, Error: errMsg,
	})
	if err != nil {
		return err
	}
	if status == "READY" {
		ch, err := s.chapters.Get(ctx, chapterID)
		if err == nil {
			_, _ = s.db.Exec(ctx,
				`UPDATE projects SET status=$2, updated_at=$3 WHERE id=$1`,
				ch.ProjectID, domain.ProjectRunning, time.Now().UTC())
		}
		s.events.Publish(ctx, ch.ProjectID, queue.ProgressEvent{
			Type: "chapter.ready", ProjectID: ch.ProjectID, ChapterID: chapterID,
		})
	}
	return nil
}

// TriggerProjectCompose enqueues compose for every chapter in a project.
func (s *CompositionService) TriggerProjectCompose(ctx context.Context, projectID, aspect string) ([]string, error) {
	chs, err := s.chapters.ListByProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(chs))
	for _, c := range chs {
		jobID, err := s.TriggerCompose(ctx, c.ID, aspect)
		if err != nil {
			return ids, err
		}
		ids = append(ids, jobID)
	}
	return ids, nil
}

// --- helpers ---------------------------------------------------------------

func (s *CompositionService) uploadManifest(ctx context.Context, projectID, chapterID string, body []byte) error {
	key := fmt.Sprintf("results/%s/chapters/%s/shots.json", projectID, chapterID)
	return s.storage.PutObject(ctx, key, bytes.NewReader(body), int64(len(body)), "application/json")
}

// ComposeChapterBackendPayload is the queue payload we send to ai-engine for
// the `ai:compose_chapter` task. Mirrors the Python side (ai-engine/app/
// schemas/payloads.py:ComposeChapterPayload).
type ComposeChapterBackendPayload struct {
	ProjectID string `json:"project_id"`
	ChapterID string `json:"chapter_id"`
	Aspect    string `json:"aspect"`
}
