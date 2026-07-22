// Package repo holds pgx-backed data access for each domain entity.
package repo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/novel2av/backend/internal/domain"
)

type ProjectRepo struct{ db *pgxpool.Pool }

func NewProjectRepo(db *pgxpool.Pool) *ProjectRepo { return &ProjectRepo{db: db} }

// Create inserts a new project and returns the populated row (with generated id/timestamps).
func (r *ProjectRepo) Create(ctx context.Context, p domain.Project) (domain.Project, error) {
	cfg, _ := json.Marshal(p.Config)
	row := r.db.QueryRow(ctx, `
		INSERT INTO projects (user_id, title, author, source_key, status, word_count, config)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		RETURNING id, created_at, updated_at`,
		p.UserID, p.Title, p.Author, p.SourceKey, p.Status, p.WordCount, cfg,
	)
	if err := row.Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt); err != nil {
		return domain.Project{}, err
	}
	return p, nil
}

func (r *ProjectRepo) Get(ctx context.Context, id string) (domain.Project, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, user_id, title, author, source_key, status, word_count, config, created_at, updated_at
		FROM projects WHERE id = $1`, id)
	return scanProject(row)
}

func (r *ProjectRepo) Delete(ctx context.Context, id string) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM projects WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

type ListParams struct {
	UserID string
	Limit  int
	Offset int
}

func (r *ProjectRepo) List(ctx context.Context, p ListParams) ([]domain.Project, error) {
	if p.Limit <= 0 || p.Limit > 100 {
		p.Limit = 20
	}
	rows, err := r.db.Query(ctx, `
		SELECT id, user_id, title, author, source_key, status, word_count, config, created_at, updated_at
		FROM projects
		WHERE user_id = $1
		ORDER BY updated_at DESC
		LIMIT $2 OFFSET $3`, p.UserID, p.Limit, p.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Project
	for rows.Next() {
		p, err := scanProject(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// --- helpers ---------------------------------------------------------------

type scanner interface {
	Scan(dest ...any) error
}

func scanProject(s scanner) (domain.Project, error) {
	var (
		p      domain.Project
		cfgRaw []byte
	)
	err := s.Scan(
		&p.ID, &p.UserID, &p.Title, &p.Author, &p.SourceKey,
		&p.Status, &p.WordCount, &cfgRaw, &p.CreatedAt, &p.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Project{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Project{}, err
	}
	if len(cfgRaw) > 0 {
		_ = json.Unmarshal(cfgRaw, &p.Config)
	}
	if p.Status == "" {
		p.Status = domain.ProjectCreated
	}
	return p, nil
}

// CountByStatus is useful for /readyz + dashboards.
func (r *ProjectRepo) CountByStatus(ctx context.Context, userID string) (map[domain.ProjectStatus]int, error) {
	rows, err := r.db.Query(ctx,
		`SELECT status, COUNT(*) FROM projects WHERE user_id = $1 GROUP BY status`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[domain.ProjectStatus]int{}
	for rows.Next() {
		var st domain.ProjectStatus
		var n int
		if err := rows.Scan(&st, &n); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		out[st] = n
	}
	return out, rows.Err()
}

// Touch updates updated_at; used as a cheap "exists" probe for some pipelines.
func (r *ProjectRepo) Touch(ctx context.Context, id string, t time.Time) error {
	_, err := r.db.Exec(ctx, `UPDATE projects SET updated_at = $2 WHERE id = $1`, id, t)
	return err
}
