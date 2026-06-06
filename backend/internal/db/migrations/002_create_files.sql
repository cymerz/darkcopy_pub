-- Migration: 002_create_files
-- Description: Create files table with indexes

CREATE TABLE IF NOT EXISTS files (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug        VARCHAR(12) NOT NULL UNIQUE,
    filename    VARCHAR(255) NOT NULL,
    mime_type   VARCHAR(100) NOT NULL,
    size_bytes  BIGINT NOT NULL,
    storage_key VARCHAR(512) NOT NULL,
    visibility  VARCHAR(20) NOT NULL DEFAULT 'public',
    password_hash VARCHAR(72),
    expires_at  TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_file_visibility CHECK (visibility IN ('public', 'unlisted', 'password_protected')),
    CONSTRAINT chk_file_size CHECK (size_bytes > 0 AND size_bytes <= 104857600),
    CONSTRAINT chk_file_slug_length CHECK (char_length(slug) BETWEEN 6 AND 12)
);

CREATE INDEX IF NOT EXISTS idx_files_slug ON files(slug);
CREATE INDEX IF NOT EXISTS idx_files_expires_at ON files(expires_at) WHERE expires_at IS NOT NULL;
