-- Migration: 006_add_stats_indexes
-- Description: Add indexes on views and downloads for fast top performing queries

CREATE INDEX IF NOT EXISTS idx_pastes_views ON pastes(views DESC);
CREATE INDEX IF NOT EXISTS idx_files_downloads ON files(downloads DESC);
