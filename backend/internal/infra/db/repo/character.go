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

type CharacterRepo struct{ db *pgxpool.Pool }

func NewCharacterRepo(db *pgxpool.Pool) *CharacterRepo { return &CharacterRepo{db: db} }

// UpsertByName creates or updates a character row keyed by (project_id, name).
// `aliases`, `ref_image_key`, `voice`, `meta` may be nil; existing values are kept.
func (r *CharacterRepo) UpsertByName(ctx context.Context, c domain.Character) (domain.Character, error) {
	metaRaw, _ := json.Marshal(c.Meta)
	if c.Role == "" {
		c.Role = "supporting"
	}
	row := r.db.QueryRow(ctx, `
		INSERT INTO characters (project_id, name, aliases, role, appearance, personality, voice, ref_image_key, meta)
		VALUES ($1,$2,$3,$4,$5,$6,$7,COALESCE(NULLIF($8,''),''),$9)
		ON CONFLICT (project_id, name) DO UPDATE
		SET aliases      = EXCLUDED.aliases,
		    role         = EXCLUDED.role,
		    appearance   = EXCLUDED.appearance,
		    personality  = EXCLUDED.personality,
		    voice        = EXCLUDED.voice,
		    ref_image_key = COALESCE(NULLIF(EXCLUDED.ref_image_key, ''), characters.ref_image_key),
		    meta         = EXCLUDED.meta
		RETURNING id, ref_image_key, created_at`,
		c.ProjectID, c.Name, c.Aliases, c.Role,
		c.Appearance, c.Personality, c.Voice, c.RefImageKey, metaRaw)
	if err := row.Scan(&c.ID, &c.RefImageKey, &c.CreatedAt); err != nil {
		return domain.Character{}, err
	}
	return c, nil
}

func (r *CharacterRepo) ListByProject(ctx context.Context, projectID string) ([]domain.Character, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, project_id, name, aliases, role, appearance, personality, voice, ref_image_key, meta, created_at
		FROM characters WHERE project_id = $1 ORDER BY role ASC, name ASC`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Character
	for rows.Next() {
		c, err := scanCharacter(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *CharacterRepo) Get(ctx context.Context, id string) (domain.Character, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, project_id, name, aliases, role, appearance, personality, voice, ref_image_key, meta, created_at
		FROM characters WHERE id = $1`, id)
	return scanCharacter(row)
}

func (r *CharacterRepo) SetRefImage(ctx context.Context, id, key string) error {
	_, err := r.db.Exec(ctx, `UPDATE characters SET ref_image_key = $2 WHERE id = $1`, id, key)
	return err
}

// Patch allows updating any subset of fields except id/project_id/created_at.
func (r *CharacterRepo) Patch(ctx context.Context, id string, p domain.CharacterPatch) (domain.Character, error) {
	sets := []string{}
	args := []any{id}
	idx := 2
	if p.Name != nil {
		sets = append(sets, "name = $"+itoa(idx))
		args = append(args, *p.Name)
		idx++
	}
	if p.Aliases != nil {
		sets = append(sets, "aliases = $"+itoa(idx))
		args = append(args, *p.Aliases)
		idx++
	}
	if p.Role != nil {
		sets = append(sets, "role = $"+itoa(idx))
		args = append(args, *p.Role)
		idx++
	}
	if p.Appearance != nil {
		sets = append(sets, "appearance = $"+itoa(idx))
		args = append(args, *p.Appearance)
		idx++
	}
	if p.Personality != nil {
		sets = append(sets, "personality = $"+itoa(idx))
		args = append(args, *p.Personality)
		idx++
	}
	if p.Voice != nil {
		sets = append(sets, "voice = $"+itoa(idx))
		args = append(args, *p.Voice)
		idx++
	}
	if p.RefImageKey != nil {
		sets = append(sets, "ref_image_key = $"+itoa(idx))
		args = append(args, *p.RefImageKey)
		idx++
	}
	if len(sets) == 0 {
		return r.Get(ctx, id)
	}
	q := "UPDATE characters SET " + strings.Join(sets, ", ") + " WHERE id = $1 " +
		`RETURNING id, project_id, name, aliases, role, appearance, personality, voice, ref_image_key, meta, created_at`
	row := r.db.QueryRow(ctx, q, args...)
	return scanCharacter(row)
}

func (r *CharacterRepo) Delete(ctx context.Context, id string) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM characters WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func scanCharacter(s scanner) (domain.Character, error) {
	var (
		c       domain.Character
		metaRaw []byte
	)
	err := s.Scan(&c.ID, &c.ProjectID, &c.Name, &c.Aliases, &c.Role,
		&c.Appearance, &c.Personality, &c.Voice, &c.RefImageKey, &metaRaw, &c.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Character{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Character{}, err
	}
	if len(metaRaw) > 0 {
		_ = json.Unmarshal(metaRaw, &c.Meta)
	}
	if c.Role == "" {
		c.Role = "supporting"
	}
	return c, nil
}
