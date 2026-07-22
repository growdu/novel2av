package repo

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/novel2av/backend/internal/domain"
)

type ShotRepo struct{ db *pgxpool.Pool }

func NewShotRepo(db *pgxpool.Pool) *ShotRepo { return &ShotRepo{db: db} }

// Upsert inserts or updates a shot keyed by (chapter_id, scene_idx, shot_idx).
func (r *ShotRepo) Upsert(ctx context.Context, s domain.Shot) (domain.Shot, error) {
	row := r.db.QueryRow(ctx, `
		INSERT INTO shots (chapter_id, scene_idx, shot_idx, type, description, narration,
		                    mood, duration_sec, status, image_key, tts_key, bgm_key, subtitle_key, meta)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9, COALESCE(NULLIF($10,''),''),
		        COALESCE(NULLIF($11,''),''), COALESCE(NULLIF($12,''),''),
		        COALESCE(NULLIF($13,''),''), $14)
		ON CONFLICT (chapter_id, scene_idx, shot_idx) DO UPDATE
		SET type        = EXCLUDED.type,
		    description = EXCLUDED.description,
		    narration   = EXCLUDED.narration,
		    mood        = EXCLUDED.mood,
		    duration_sec = EXCLUDED.duration_sec,
		    status      = EXCLUDED.status,
		    image_key   = COALESCE(NULLIF(EXCLUDED.image_key, ''), shots.image_key),
		    tts_key     = COALESCE(NULLIF(EXCLUDED.tts_key, ''), shots.tts_key),
		    bgm_key     = COALESCE(NULLIF(EXCLUDED.bgm_key, ''), shots.bgm_key),
		    subtitle_key= COALESCE(NULLIF(EXCLUDED.subtitle_key, ''), shots.subtitle_key),
		    meta        = EXCLUDED.meta
		RETURNING id, created_at`,
		s.ChapterID, s.SceneIdx, s.ShotIdx, s.Type, s.Description, s.Narration,
		s.Mood, s.DurationSec, s.Status, s.ImageKey, s.TTSKey, s.BGMKey, s.SubtitleKey, []byte("{}"))
	if err := row.Scan(&s.ID, &s.CreatedAt); err != nil {
		return domain.Shot{}, err
	}
	return s, nil
}

func (r *ShotRepo) ListByProject(ctx context.Context, projectID string) ([]domain.Shot, error) {
	rows, err := r.db.Query(ctx, `
		SELECT s.id, s.chapter_id, s.scene_idx, s.shot_idx, s.type, s.description,
		       s.narration, s.mood, s.duration_sec, s.status, s.image_key, s.tts_key,
		       s.bgm_key, s.subtitle_key, s.meta, s.created_at
		FROM shots s
		JOIN chapters c ON c.id = s.chapter_id
		WHERE c.project_id = $1
		ORDER BY c."index" ASC, s.scene_idx ASC, s.shot_idx ASC`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Shot
	for rows.Next() {
		s, err := scanShot(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *ShotRepo) Get(ctx context.Context, id string) (domain.Shot, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, chapter_id, scene_idx, shot_idx, type, description, narration, mood,
		       duration_sec, status, image_key, tts_key, bgm_key, subtitle_key, meta, created_at
		FROM shots WHERE id = $1`, id)
	return scanShot(row)
}

func (r *ShotRepo) PatchAssets(ctx context.Context, id string, imageKey, ttsKey, bgmKey, subtitleKey *string) (domain.Shot, error) {
	sets := []string{}
	args := []any{id}
	idx := 2
	for _, kv := range []struct {
		k string
		v *string
	}{{"image_key", imageKey}, {"tts_key", ttsKey}, {"bgm_key", bgmKey}, {"subtitle_key", subtitleKey}} {
		if kv.v != nil {
			sets = append(sets, kv.k+" = $"+itoa(idx))
			args = append(args, *kv.v)
			idx++
		}
	}
	if len(sets) == 0 {
		return r.Get(ctx, id)
	}
	q := "UPDATE shots SET " + strings.Join(sets, ", ") + " WHERE id = $1 " +
		`RETURNING id, chapter_id, scene_idx, shot_idx, type, description, narration, mood,
		         duration_sec, status, image_key, tts_key, bgm_key, subtitle_key, meta, created_at`
	row := r.db.QueryRow(ctx, q, args...)
	return scanShot(row)
}

func scanShot(s scanner) (domain.Shot, error) {
	var (
		sh      domain.Shot
		metaRaw []byte
	)
	err := s.Scan(&sh.ID, &sh.ChapterID, &sh.SceneIdx, &sh.ShotIdx, &sh.Type,
		&sh.Description, &sh.Narration, &sh.Mood, &sh.DurationSec, &sh.Status,
		&sh.ImageKey, &sh.TTSKey, &sh.BGMKey, &sh.SubtitleKey, &metaRaw, &sh.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Shot{}, domain.ErrNotFound
	}
	if len(metaRaw) > 0 {
		_ = jsonUnmarshal(metaRaw, &sh.Meta)
	}
	return sh, err
}

// jsonUnmarshal delegates to encoding/json (kept as a hook for tests).
var jsonUnmarshal = json.Unmarshal

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	pos := len(b)
	for n > 0 {
		pos--
		b[pos] = byte('0' + n%10)
		n /= 10
	}
	return string(b[pos:])
}
