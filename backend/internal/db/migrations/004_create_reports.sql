-- Migration: 004_create_reports
-- Description: Abuse/content reports submitted by visitors against a paste or
-- file, reviewed by an administrator.

CREATE TABLE IF NOT EXISTS reports (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    resource_type VARCHAR(10) NOT NULL,
    slug          VARCHAR(12) NOT NULL,
    reason        VARCHAR(32) NOT NULL,
    details       TEXT NOT NULL DEFAULT '',
    reporter_ip   VARCHAR(64) NOT NULL DEFAULT '',
    status        VARCHAR(12) NOT NULL DEFAULT 'pending',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    reviewed_at   TIMESTAMPTZ,

    CONSTRAINT chk_report_resource_type CHECK (resource_type IN ('paste', 'file')),
    CONSTRAINT chk_report_status CHECK (status IN ('pending', 'reviewed', 'dismissed'))
);

CREATE INDEX IF NOT EXISTS idx_reports_status_created ON reports(status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_reports_resource ON reports(resource_type, slug);
