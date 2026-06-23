-- XMECO Migration 007: Add last_online_at for delayed offline detection
ALTER TABLE device ADD COLUMN IF NOT EXISTS last_online_at TIMESTAMPTZ;
-- Seed existing rows so they won't all go offline immediately
UPDATE device SET last_online_at = NOW() WHERE last_online_at IS NULL;
