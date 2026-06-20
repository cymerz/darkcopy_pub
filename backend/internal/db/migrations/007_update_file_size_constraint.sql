-- Migration: 007_update_file_size_constraint
-- Description: Drop the hardcoded upper limit in chk_file_size and only enforce size_bytes > 0. Capping is managed dynamically by the Go application.

ALTER TABLE files DROP CONSTRAINT IF EXISTS chk_file_size;

ALTER TABLE files ADD CONSTRAINT chk_file_size CHECK (size_bytes > 0);
