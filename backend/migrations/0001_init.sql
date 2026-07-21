-- 0001_init.sql — initial schema for novel2av-backend (Go is the source of truth).

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS projects (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      uuid NOT NULL,
    title        text NOT NULL,
    author       text NOT NULL DEFAULT '',
    source_key   text NOT NULL,
    status       text NOT NULL DEFAULT 'CREATED',
    word_count   int  NOT NULL DEFAULT 0,
    config       jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS chapters (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id  uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    index       int  NOT NULL,
    title       text NOT NULL,
    content_key text NOT NULL,
    word_count  int  NOT NULL DEFAULT 0,
    status      text NOT NULL DEFAULT 'PENDING',
    created_at  timestamptz NOT NULL DEFAULT now(),
    UNIQUE(project_id, index)
);

CREATE TABLE IF NOT EXISTS characters (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id   uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name         text NOT NULL,
    aliases      text[] NOT NULL DEFAULT '{}',
    role         text NOT NULL DEFAULT 'supporting',
    appearance   text NOT NULL DEFAULT '',
    personality  text NOT NULL DEFAULT '',
    voice        text NOT NULL DEFAULT '',
    ref_image_key text NOT NULL DEFAULT '',
    meta         jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at   timestamptz NOT NULL DEFAULT now(),
    UNIQUE(project_id, name)
);

CREATE TABLE IF NOT EXISTS shots (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    chapter_id   uuid NOT NULL REFERENCES chapters(id) ON DELETE CASCADE,
    scene_idx    int  NOT NULL,
    shot_idx     int  NOT NULL,
    type         text NOT NULL DEFAULT 'wide',
    description  text NOT NULL DEFAULT '',
    narration    text NOT NULL DEFAULT '',
    mood         text NOT NULL DEFAULT '',
    duration_sec real NOT NULL DEFAULT 0,
    status       text NOT NULL DEFAULT 'PENDING',
    image_key    text NOT NULL DEFAULT '',
    tts_key      text NOT NULL DEFAULT '',
    bgm_key      text NOT NULL DEFAULT '',
    subtitle_key text NOT NULL DEFAULT '',
    meta         jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at   timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS jobs (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id  uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    parent_id   uuid REFERENCES jobs(id) ON DELETE SET NULL,
    type        text NOT NULL,
    status      text NOT NULL DEFAULT 'queued',
    attempts    int  NOT NULL DEFAULT 0,
    meta        jsonb NOT NULL DEFAULT '{}'::jsonb,
    error       jsonb,
    started_at  timestamptz,
    finished_at timestamptz,
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_jobs_project ON jobs(project_id, status);
CREATE INDEX IF NOT EXISTS idx_jobs_parent  ON jobs(parent_id);
CREATE INDEX IF NOT EXISTS idx_chapters    ON chapters(project_id, index);
