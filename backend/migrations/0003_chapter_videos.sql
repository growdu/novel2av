-- 0003_chapter_videos.sql — per-chapter composed videos.

CREATE TABLE IF NOT EXISTS chapter_videos (
    chapter_id     uuid PRIMARY KEY REFERENCES chapters(id) ON DELETE CASCADE,
    video_key      text NOT NULL DEFAULT '',
    duration_sec   real NOT NULL DEFAULT 0,
    status         text NOT NULL DEFAULT 'PENDING',  -- PENDING/COMPOSING/READY/FAILED
    error          text NOT NULL DEFAULT '',
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_chapter_videos_status ON chapter_videos(status);
