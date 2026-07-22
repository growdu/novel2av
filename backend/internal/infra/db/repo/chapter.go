package repo

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/novel2av/backend/internal/domain"
)

type ChapterRepo struct{ db *pgxpool.Pool }

func NewChapterRepo(db *pgxpool.Pool) *ChapterRepo { return &ChapterRepo{db: db} }

// Upsert inserts a chapter row or updates title/word_count/content_key if it already exists.
func (r *ChapterRepo) Upsert(ctx context.Context, c domain.Chapter) (domain.Chapter, error) {
	row := r.db.QueryRow(ctx, `
		INSERT INTO chapters (project_id, "index", title, content_key, word_count, status)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (project_id, "index") DO UPDATE
		SET title = EXCLUDED.title,
		    word_count = EXCLUDED.word_count,
		    content_key = COALESCE(NULLIF(EXCLUDED.content_key, ''), chapters.content_key),
		    status = EXCLUDED.status
		RETURNING id, created_at`,
		c.ProjectID, c.Index, c.Title, c.ContentKey, c.WordCount, c.Status,
	)
	if err := row.Scan(&c.ID, &c.CreatedAt); err != nil {
		return domain.Chapter{}, err
	}
	return c, nil
}

func (r *ChapterRepo) ListByProject(ctx context.Context, projectID string) ([]domain.Chapter, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, project_id, "index", title, word_count, status, content_key, created_at
		FROM chapters WHERE project_id = $1 ORDER BY "index" ASC`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Chapter
	for rows.Next() {
		c, err := scanChapter(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *ChapterRepo) Get(ctx context.Context, id string) (domain.Chapter, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, project_id, "index", title, word_count, status, content_key, created_at
		FROM chapters WHERE id = $1`, id)
	return scanChapter(row)
}

func (r *ChapterRepo) Patch(ctx context.Context, id string, title *string, status *string) (domain.Chapter, error) {
	sets := []string{}
	args := []any{id}
	idx := 2
	if title != nil {
		sets = append(sets, "title = $"+itoa(idx))
		args = append(args, *title)
		idx++
	}
	if status != nil {
		sets = append(sets, "status = $"+itoa(idx))
		args = append(args, *status)
		idx++
	}
	if len(sets) == 0 {
		return r.Get(ctx, id)
	}
	q := "UPDATE chapters SET " + strings.Join(sets, ", ") + " WHERE id = $1 " +
		`RETURNING id, project_id, "index", title, word_count, status, content_key, created_at`
	row := r.db.QueryRow(ctx, q, args...)
	return scanChapter(row)
}

func (r *ChapterRepo) DeleteByProject(ctx context.Context, projectID string) error {
	_, err := r.db.Exec(ctx, `DELETE FROM chapters WHERE project_id = $1`, projectID)
	return err
}

func scanChapter(s scanner) (domain.Chapter, error) {
	var c domain.Chapter
	err := s.Scan(&c.ID, &c.ProjectID, &c.Index, &c.Title, &c.WordCount, &c.Status, &c.ContentKey, &c.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Chapter{}, domain.ErrNotFound
	}
	return c, err
}

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
