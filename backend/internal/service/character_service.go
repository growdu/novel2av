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

type CharacterService struct {
	db         *pgxpool.Pool
	storage    *storage.MinIOClient
	characters *repo.CharacterRepo
	chapters   *repo.ChapterRepo
	projects   *repo.ProjectRepo
	queue      *queue.AsynqClient
}

func NewCharacterService(db *pgxpool.Pool, st *storage.MinIOClient, q *queue.AsynqClient) *CharacterService {
	return &CharacterService{
		db:         db,
		storage:    st,
		characters: repo.NewCharacterRepo(db),
		chapters:   repo.NewChapterRepo(db),
		projects:   repo.NewProjectRepo(db),
		queue:      q,
	}
}

func (s *CharacterService) List(ctx context.Context, projectID string) ([]domain.Character, error) {
	return s.characters.ListByProject(ctx, projectID)
}

func (s *CharacterService) Get(ctx context.Context, id string) (domain.Character, error) {
	return s.characters.Get(ctx, id)
}

func (s *CharacterService) Patch(ctx context.Context, id string, p domain.CharacterPatch) (domain.Character, error) {
	if p.Name != nil && *p.Name == "" {
		return domain.Character{}, fmt.Errorf("%w: name cannot be empty", domain.ErrInvalidInput)
	}
	return s.characters.Patch(ctx, id, p)
}

func (s *CharacterService) Delete(ctx context.Context, id string) error {
	return s.characters.Delete(ctx, id)
}

// PresignRefImage returns a time-limited URL for a character's ref image.
func (s *CharacterService) PresignRefImage(ctx context.Context, characterID string, ttl time.Duration) (string, error) {
	c, err := s.characters.Get(ctx, characterID)
	if err != nil {
		return "", err
	}
	if c.RefImageKey == "" {
		return "", nil
	}
	u, err := s.storage.PresignGet(ctx, c.RefImageKey, ttl)
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

// TriggerExtract enqueues ai:extract_characters for a project.
// It passes every chapter's `content_key` so ai-engine can pull them.
func (s *CharacterService) TriggerExtract(ctx context.Context, projectID string) (string, error) {
	if _, err := s.projects.Get(ctx, projectID); err != nil {
		return "", err
	}
	chs, err := s.chapters.ListByProject(ctx, projectID)
	if err != nil {
		return "", err
	}
	keys := make([]string, 0, len(chs))
	for _, c := range chs {
		if c.Status == "MERGED" {
			continue
		}
		if c.ContentKey != "" {
			keys = append(keys, c.ContentKey)
		}
	}
	if len(keys) == 0 {
		return "", fmt.Errorf("%w: no chapters to extract from", domain.ErrConflict)
	}

	// Flip project status.
	if _, err := s.db.Exec(ctx,
		`UPDATE projects SET status=$2, updated_at=$3 WHERE id=$1`,
		projectID, domain.ProjectExtracting, time.Now().UTC()); err != nil {
		return "", err
	}

	body, _ := json.Marshal(ExtractCharactersBackendPayload{
		ProjectID:    projectID,
		ChapterKeys:  keys,
		OnlyChapters: nil,
	})
	return s.queue.Enqueue(ctx, "ai:extract_characters", body, 3)
}

// TriggerRegenImage enqueues ai:character_image with the requested variant count.
func (s *CharacterService) TriggerRegenImage(ctx context.Context, characterID string, variants int) (string, error) {
	c, err := s.characters.Get(ctx, characterID)
	if err != nil {
		return "", err
	}
	if variants <= 0 {
		variants = 4
	}
	body, _ := json.Marshal(RegenImageBackendPayload{
		ProjectID:   c.ProjectID,
		CharacterID: c.ID,
		Variants:    variants,
	})
	return s.queue.Enqueue(ctx, "ai:character_image", body, 2)
}

// IngestExtractResult reads results/<project_id>/characters.json and upserts
// characters, then marks the project READY (for downstream pipeline steps).
func (s *CharacterService) IngestExtractResult(ctx context.Context, projectID string) (int, error) {
	key := fmt.Sprintf("results/%s/characters.json", projectID)
	body, err := s.downloadResult(ctx, key)
	if err != nil {
		return 0, fmt.Errorf("download %s: %w", key, err)
	}
	var manifest struct {
		Characters []struct {
			Name        string   `json:"name"`
			Aliases     []string `json:"aliases"`
			Role        string   `json:"role"`
			Appearance  string   `json:"appearance"`
			Personality string   `json:"personality"`
			Voice       string   `json:"voice"`
		} `json:"characters"`
	}
	if err := json.Unmarshal(body, &manifest); err != nil {
		return 0, fmt.Errorf("parse manifest: %w", err)
	}
	sort.Slice(manifest.Characters, func(i, j int) bool {
		return manifest.Characters[i].Name < manifest.Characters[j].Name
	})

	count := 0
	for _, ch := range manifest.Characters {
		name := ch.Name
		if name == "" {
			continue
		}
		_, err := s.characters.UpsertByName(ctx, domain.Character{
			ProjectID:   projectID,
			Name:        name,
			Aliases:     ch.Aliases,
			Role:        ch.Role,
			Appearance:  ch.Appearance,
			Personality: ch.Personality,
			Voice:       ch.Voice,
		})
		if err != nil {
			return count, fmt.Errorf("upsert %s: %w", name, err)
		}
		count++
	}
	if _, err := s.db.Exec(ctx,
		`UPDATE projects SET status=$2, updated_at=$3 WHERE id=$1`,
		projectID, domain.ProjectReady, time.Now().UTC()); err != nil {
		return count, fmt.Errorf("update project status: %w", err)
	}
	return count, nil
}

// IngestCharacterImage writes the ref_image_key back to the row.
func (s *CharacterService) IngestCharacterImage(ctx context.Context, characterID, refKey string) error {
	return s.characters.SetRefImage(ctx, characterID, refKey)
}

// --- helpers ---------------------------------------------------------------

func (s *CharacterService) downloadResult(ctx context.Context, key string) ([]byte, error) {
	url, err := s.storage.PresignGet(ctx, key, 5*time.Minute)
	if err != nil {
		return nil, err
	}
	return fetchURL(ctx, url.String())
}

type ExtractCharactersBackendPayload struct {
	ProjectID    string   `json:"project_id"`
	ChapterKeys  []string `json:"chapter_keys"`
	OnlyChapters []int    `json:"only_chapters,omitempty"`
}

type RegenImageBackendPayload struct {
	ProjectID   string `json:"project_id"`
	CharacterID string `json:"character_id"`
	Variants    int    `json:"variants"`
}
