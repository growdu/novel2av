-- 0004_project_videos.sql — full-book composite video per project.

CREATE TABLE IF NOT EXISTS project_videos (
    project_id     uuid PRIMARY KEY REFERENCES projects(id) ON DELETE CASCADE,
    video_key      text NOT NULL DEFAULT '',
    duration_sec   real NOT NULL DEFAULT 0,
    status         text NOT NULL DEFAULT 'PENDING',  -- PENDING/COMPOSING/READY/FAILED
    error          text NOT NULL DEFAULT '',
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now()
);
