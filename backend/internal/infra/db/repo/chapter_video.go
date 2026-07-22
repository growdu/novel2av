package repo

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/novel2av/backend/internal/domain"
)

type ChapterVideoRepo struct{ db *pgxpool.Pool }

func NewChapterVideoRepo(db *pgxpool.Pool) *ChapterVideoRepo { return &ChapterVideoRepo{db: db} }

type ChapterVideo struct {
	ChapterID   string    `json:"chapter_id"`
	VideoKey    string    `json:"video_key"`
	DurationSec float64   `json:"duration_sec"`
	Status      string    `json:"status"`
	Error       string    `json:"error"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Upsert creates or updates a chapter_videos row keyed by chapter_id.
func (r *ChapterVideoRepo) Upsert(ctx context.Context, v ChapterVideo) (ChapterVideo, error) {
	row := r.db.QueryRow(ctx, `
		INSERT INTO chapter_videos (chapter_id, video_key, duration_sec, status, error, updated_at)
		VALUES ($1,$2,$3,$4,$5, now())
		ON CONFLICT (chapter_id) DO UPDATE
		SET video_key    = COALESCE(NULLIF(EXCLUDED.video_key, ''), chapter_videos.video_key),
		    duration_sec = EXCLUDED.duration_sec,
		    status       = EXCLUDED.status,
		    error        = EXCLUDED.error,
		    updated_at   = now()
		RETURNING chapter_id, video_key, duration_sec, status, error, created_at, updated_at`,
		v.ChapterID, v.VideoKey, v.DurationSec, v.Status, v.Error,
	)
	return scanChapterVideo(row)
}

func (r *ChapterVideoRepo) Get(ctx context.Context, chapterID string) (ChapterVideo, error) {
	row := r.db.QueryRow(ctx, `
		SELECT chapter_id, video_key, duration_sec, status, error, created_at, updated_at
		FROM chapter_videos WHERE chapter_id = $1`, chapterID)
	return scanChapterVideo(row)
}

func (r *ChapterVideoRepo) ListByProject(ctx context.Context, projectID string) ([]ChapterVideo, error) {
	rows, err := r.db.Query(ctx, `
		SELECT v.chapter_id, v.video_key, v.duration_sec, v.status, v.error, v.created_at, v.updated_at
		FROM chapter_videos v
		JOIN chapters c ON c.id = v.chapter_id
		WHERE c.project_id = $1
		ORDER BY c."index" ASC`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ChapterVideo
	for rows.Next() {
		v, err := scanChapterVideo(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func scanChapterVideo(s scanner) (ChapterVideo, error) {
	var v ChapterVideo
	err := s.Scan(&v.ChapterID, &v.VideoKey, &v.DurationSec, &v.Status, &v.Error,
		&v.CreatedAt, &v.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ChapterVideo{}, domain.ErrNotFound
	}
	return v, err
}
