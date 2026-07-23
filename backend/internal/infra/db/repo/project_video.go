package repo

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/novel2av/backend/internal/domain"
)

type ProjectVideo struct {
	ProjectID   string    `json:"project_id"`
	VideoKey    string    `json:"video_key"`
	DurationSec float64   `json:"duration_sec"`
	Status      string    `json:"status"`
	Error       string    `json:"error"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type ProjectVideoRepo struct{ db *pgxpool.Pool }

func NewProjectVideoRepo(db *pgxpool.Pool) *ProjectVideoRepo { return &ProjectVideoRepo{db: db} }

func (r *ProjectVideoRepo) Upsert(ctx context.Context, v ProjectVideo) (ProjectVideo, error) {
	row := r.db.QueryRow(ctx, `
		INSERT INTO project_videos (project_id, video_key, duration_sec, status, error, updated_at)
		VALUES ($1,$2,$3,$4,$5, now())
		ON CONFLICT (project_id) DO UPDATE
		SET video_key    = COALESCE(NULLIF(EXCLUDED.video_key, ''), project_videos.video_key),
		    duration_sec = EXCLUDED.duration_sec,
		    status       = EXCLUDED.status,
		    error        = EXCLUDED.error,
		    updated_at   = now()
		RETURNING project_id, video_key, duration_sec, status, error, created_at, updated_at`,
		v.ProjectID, v.VideoKey, v.DurationSec, v.Status, v.Error,
	)
	return scanProjectVideo(row)
}

func (r *ProjectVideoRepo) Get(ctx context.Context, projectID string) (ProjectVideo, error) {
	row := r.db.QueryRow(ctx, `
		SELECT project_id, video_key, duration_sec, status, error, created_at, updated_at
		FROM project_videos WHERE project_id = $1`, projectID)
	return scanProjectVideo(row)
}

func scanProjectVideo(s scanner) (ProjectVideo, error) {
	var v ProjectVideo
	err := s.Scan(&v.ProjectID, &v.VideoKey, &v.DurationSec, &v.Status, &v.Error,
		&v.CreatedAt, &v.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ProjectVideo{}, domain.ErrNotFound
	}
	return v, err
}
