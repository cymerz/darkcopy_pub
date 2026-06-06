-- Migration: 003_create_settings
-- Description: Single-row table holding runtime-configurable application
-- settings as a JSONB document. The CHECK constraint pins it to one row.

CREATE TABLE IF NOT EXISTS app_settings (
    id          INTEGER PRIMARY KEY DEFAULT 1,
    data        JSONB NOT NULL,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_settings_singleton CHECK (id = 1)
);
