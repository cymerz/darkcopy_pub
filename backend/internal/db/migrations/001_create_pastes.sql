-- Migration: 001_create_pastes
-- Description: Create pastes table with indexes

CREATE TABLE IF NOT EXISTS pastes (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug        VARCHAR(12) NOT NULL UNIQUE,
    title       VARCHAR(255),
    content     TEXT NOT NULL,
    language    VARCHAR(50) NOT NULL DEFAULT 'plaintext',
    visibility  VARCHAR(20) NOT NULL DEFAULT 'public',
    password_hash VARCHAR(72),
    expires_at  TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_visibility CHECK (visibility IN ('public', 'unlisted', 'password_protected')),
    CONSTRAINT chk_slug_length CHECK (char_length(slug) BETWEEN 6 AND 12)
);

CREATE INDEX IF NOT EXISTS idx_pastes_slug ON pastes(slug);
CREATE INDEX IF NOT EXISTS idx_pastes_expires_at ON pastes(expires_at) WHERE expires_at IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_pastes_public_recent ON pastes(created_at DESC) WHERE visibility = 'public';
