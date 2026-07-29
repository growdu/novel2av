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

type ShotService struct {
	db        *pgxpool.Pool
	storage   *storage.MinIOClient
	shots     *repo.ShotRepo
	chapters  *repo.ChapterRepo
	characters *repo.CharacterRepo
	queue     *queue.AsynqClient
}

func NewShotService(db *pgxpool.Pool, st *storage.MinIOClient, q *queue.AsynqClient) *ShotService {
	return &ShotService{
		db:         db,
		storage:    st,
		shots:      repo.NewShotRepo(db),
		chapters:   repo.NewChapterRepo(db),
		characters: repo.NewCharacterRepo(db),
		queue:      q,
	}
}

func (s *ShotService) ListByProject(ctx context.Context, projectID string) ([]domain.Shot, error) {
	return s.shots.ListByProject(ctx, projectID)
}

func (s *ShotService) Get(ctx context.Context, id string) (domain.Shot, error) {
	return s.shots.Get(ctx, id)
}

type SceneBreakdownBackendPayload struct {
	ProjectID  string   `json:"project_id"`
	ChapterID  string   `json:"chapter_id"`
	CharacterRefs []string `json:"character_refs,omitempty"`
}

// TriggerBreakdown enqueues ai:scene_breakdown for one chapter.
func (s *ShotService) TriggerBreakdown(ctx context.Context, projectID, chapterID string) (string, error) {
	if _, err := s.chapters.Get(ctx, chapterID); err != nil {
		return "", err
	}
	// Pull all character ref_image_keys for consistency hints.
	chars, err := s.characters.ListByProject(ctx, projectID)
	if err != nil {
		return "", err
	}
	refs := make([]string, 0, len(chars))
	for _, c := range chars {
		if c.RefImageKey != "" {
			refs = append(refs, c.RefImageKey)
		}
	}
	body, _ := json.Marshal(SceneBreakdownBackendPayload{
		ProjectID: projectID, ChapterID: chapterID, CharacterRefs: refs,
	})
	return s.queue.Enqueue(ctx, "ai:scene_breakdown", body, 3)
}

// TriggerGenerateShot enqueues ai:generate_shot.
func (s *ShotService) TriggerGenerateShot(ctx context.Context, shotID string, aspect string) (string, error) {
	sh, err := s.shots.Get(ctx, shotID)
	if err != nil {
		return "", err
	}
	// Look up the project's default aspect from the project's config (M5 might extend).
	chars, _ := s.characters.ListByProject(ctx, projectIDOfChapter(ctx, s, sh.ChapterID))
	refs := make([]string, 0, len(chars))
	for _, c := range chars {
		if c.RefImageKey != "" {
			refs = append(refs, c.RefImageKey)
		}
	}
	if aspect == "" {
		aspect = "9:16"
	}
	body, _ := json.Marshal(GenerateShotBackendPayload{
		ProjectID:     projectIDOfChapter(ctx, s, sh.ChapterID),
		ChapterID:     sh.ChapterID,
		ShotID:        sh.ID,
		Description:   sh.Description,
		Narration:     sh.Narration,
		Mood:          sh.Mood,
		CharacterRefs: refs,
		Style:         "cinematic",
		Aspect:        aspect,
	})
	return s.queue.Enqueue(ctx, "ai:generate_shot", body, 3)
}

func projectIDOfChapter(ctx context.Context, s *ShotService, chapterID string) string {
	if ch, err := s.chapters.Get(ctx, chapterID); err == nil {
		return ch.ProjectID
	}
	return ""
}

// IngestBreakdown reads results/<project_id>/chapters/<chapter_id>/breakdown.json
// from MinIO and upserts shot rows.
func (s *ShotService) IngestBreakdown(ctx context.Context, projectID, chapterID string) (int, error) {
	key := fmt.Sprintf("results/%s/chapters/%s/breakdown.json", projectID, chapterID)
	body, err := s.downloadResult(ctx, key)
	if err != nil {
		return 0, fmt.Errorf("download %s: %w", key, err)
	}
	var manifest struct {
		Shots []struct {
			SceneIdx    int     `json:"scene_idx"`
			ShotIdx     int     `json:"shot_idx"`
			Type        string  `json:"type"`
			Description string  `json:"description"`
			Narration   string  `json:"narration"`
			Mood        string  `json:"mood"`
			DurationSec float64 `json:"duration_sec"`
		} `json:"shots"`
	}
	if err := json.Unmarshal(body, &manifest); err != nil {
		return 0, fmt.Errorf("parse: %w", err)
	}
	sort.Slice(manifest.Shots, func(i, j int) bool {
		if manifest.Shots[i].SceneIdx != manifest.Shots[j].SceneIdx {
			return manifest.Shots[i].SceneIdx < manifest.Shots[j].SceneIdx
		}
		return manifest.Shots[i].ShotIdx < manifest.Shots[j].ShotIdx
	})

	n := 0
	for _, sh := range manifest.Shots {
		_, err := s.shots.Upsert(ctx, domain.Shot{
			ChapterID:   chapterID,
			SceneIdx:    sh.SceneIdx,
			ShotIdx:     sh.ShotIdx,
			Type:        sh.Type,
			Description: sh.Description,
			Narration:   sh.Narration,
			Mood:        sh.Mood,
			DurationSec: sh.DurationSec,
			Status:      "PENDING",
		})
		if err != nil {
			return n, fmt.Errorf("upsert shot: %w", err)
		}
		n++
	}
	return n, nil
}

// IngestShotAssets writes the per-shot asset keys back to the row.
func (s *ShotService) IngestShotAssets(ctx context.Context, shotID string, p ShotAssetPatch) (domain.Shot, error) {
	// Defensive: tests (and any future partial-init path) should not panic on
	// a nil repo; surface a typed error matching the test contract.
	if s.shots == nil {
		return domain.Shot{}, fmt.Errorf("shot repo not configured")
	}
	return s.shots.PatchAssets(ctx, shotID, p.ImageKey, p.TTSKey, p.BGMKey, p.SubtitleKey)
}

// TriggerProjectBreakdown enqueues a breakdown per chapter in the project.
func (s *ShotService) TriggerProjectBreakdown(ctx context.Context, projectID string) ([]string, error) {
	chs, err := s.chapters.ListByProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(chs))
	for _, c := range chs {
		jobID, err := s.TriggerBreakdown(ctx, projectID, c.ID)
		if err != nil {
			return ids, err
		}
		ids = append(ids, jobID)
	}
	return ids, nil
}

// --- helpers ---------------------------------------------------------------

func (s *ShotService) downloadResult(ctx context.Context, key string) ([]byte, error) {
	url, err := s.storage.PresignGet(ctx, key, 5*time.Minute)
	if err != nil {
		return nil, err
	}
	return fetchURL(ctx, url.String())
}

type GenerateShotBackendPayload struct {
	ProjectID     string   `json:"project_id"`
	ChapterID     string   `json:"chapter_id"`
	ShotID        string   `json:"shot_id"`
	Description   string   `json:"description"`
	Narration     string   `json:"narration"`
	Mood          string   `json:"mood"`
	CharacterRefs []string `json:"character_refs,omitempty"`
	Style         string   `json:"style"`
	Aspect        string   `json:"aspect"`
}

// ShotAssetPatch captures which subset of asset keys the worker produced.
type ShotAssetPatch struct {
	ImageKey    *string `json:"image_key,omitempty"`
	TTSKey      *string `json:"tts_key,omitempty"`
	BGMKey      *string `json:"bgm_key,omitempty"`
	SubtitleKey *string `json:"subtitle_key,omitempty"`
}
