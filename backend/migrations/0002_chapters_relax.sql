-- 0002_chapters_relax.sql — allow chapters without content_key yet.
ALTER TABLE chapters ALTER COLUMN content_key DROP NOT NULL;
