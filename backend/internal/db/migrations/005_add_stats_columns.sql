-- Migration: 005_add_stats_columns
-- Description: Add views to pastes and downloads to files

ALTER TABLE pastes ADD COLUMN IF NOT EXISTS views INTEGER NOT NULL DEFAULT 0;
ALTER TABLE files ADD COLUMN IF NOT EXISTS downloads INTEGER NOT NULL DEFAULT 0;
