package db

import (
	"io/fs"
	"sort"
	"strings"
	"testing"
)

// TestMigrationFilesExist verifies that migration SQL files are properly
// embedded and accessible.
func TestMigrationFilesExist(t *testing.T) {
	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		t.Fatalf("failed to read migrations directory: %v", err)
	}

	if len(entries) == 0 {
		t.Fatal("no migration files found")
	}

	// Verify files are sorted and have .sql extension
	var names []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			names = append(names, entry.Name())
		}
	}

	if len(names) < 2 {
		t.Fatalf("expected at least 2 migration files, got %d", len(names))
	}

	// Verify ordering
	sorted := make([]string, len(names))
	copy(sorted, names)
	sort.Strings(sorted)

	for i, name := range names {
		if name != sorted[i] {
			t.Errorf("migration files not in expected order: got %s at position %d, expected %s", name, i, sorted[i])
		}
	}
}

// TestMigrationFilesContent verifies that migration files contain expected
// SQL statements.
func TestMigrationFilesContent(t *testing.T) {
	tests := []struct {
		filename string
		contains []string
	}{
		{
			filename: "001_create_pastes.sql",
			contains: []string{
				"CREATE TABLE",
				"pastes",
				"id",
				"UUID PRIMARY KEY",
				"slug",
				"VARCHAR(12)",
				"content",
				"TEXT NOT NULL",
				"language",
				"visibility",
				"password_hash",
				"expires_at",
				"created_at",
				"chk_visibility",
				"chk_slug_length",
				"idx_pastes_slug",
				"idx_pastes_expires_at",
				"idx_pastes_public_recent",
			},
		},
		{
			filename: "002_create_files.sql",
			contains: []string{
				"CREATE TABLE",
				"files",
				"id",
				"UUID PRIMARY KEY",
				"slug",
				"VARCHAR(12)",
				"filename",
				"mime_type",
				"size_bytes",
				"storage_key",
				"visibility",
				"password_hash",
				"expires_at",
				"created_at",
				"chk_file_visibility",
				"chk_file_size",
				"chk_file_slug_length",
				"idx_files_slug",
				"idx_files_expires_at",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			content, err := fs.ReadFile(migrationsFS, "migrations/"+tt.filename)
			if err != nil {
				t.Fatalf("failed to read %s: %v", tt.filename, err)
			}

			sql := string(content)
			for _, expected := range tt.contains {
				if !strings.Contains(sql, expected) {
					t.Errorf("migration %s missing expected content: %q", tt.filename, expected)
				}
			}
		})
	}
}

// TestMigrationFilesPastesSchema verifies the pastes table schema matches
// the design specification.
func TestMigrationFilesPastesSchema(t *testing.T) {
	content, err := fs.ReadFile(migrationsFS, "migrations/001_create_pastes.sql")
	if err != nil {
		t.Fatalf("failed to read pastes migration: %v", err)
	}

	sql := string(content)

	// Verify UUID primary key with gen_random_uuid()
	if !strings.Contains(sql, "gen_random_uuid()") {
		t.Error("pastes table should use gen_random_uuid() for default ID")
	}

	// Verify slug constraints
	if !strings.Contains(sql, "BETWEEN 6 AND 12") {
		t.Error("pastes table should have slug length constraint between 6 and 12")
	}

	// Verify visibility check constraint
	if !strings.Contains(sql, "'public', 'unlisted', 'password_protected'") {
		t.Error("pastes table should have visibility check constraint")
	}

	// Verify partial index on expires_at
	if !strings.Contains(sql, "WHERE expires_at IS NOT NULL") {
		t.Error("pastes table should have partial index on expires_at")
	}

	// Verify partial index on public recent
	if !strings.Contains(sql, "WHERE visibility = 'public'") {
		t.Error("pastes table should have partial index for public visibility")
	}
}

// TestMigrationFilesFilesSchema verifies the files table schema matches
// the design specification.
func TestMigrationFilesFilesSchema(t *testing.T) {
	content, err := fs.ReadFile(migrationsFS, "migrations/002_create_files.sql")
	if err != nil {
		t.Fatalf("failed to read files migration: %v", err)
	}

	sql := string(content)

	// Verify UUID primary key with gen_random_uuid()
	if !strings.Contains(sql, "gen_random_uuid()") {
		t.Error("files table should use gen_random_uuid() for default ID")
	}

	// Verify file size constraint (100 MB = 104857600 bytes)
	if !strings.Contains(sql, "104857600") {
		t.Error("files table should have size constraint of 104857600 bytes (100 MB)")
	}

	// Verify slug constraints
	if !strings.Contains(sql, "BETWEEN 6 AND 12") {
		t.Error("files table should have slug length constraint between 6 and 12")
	}

	// Verify visibility check constraint
	if !strings.Contains(sql, "'public', 'unlisted', 'password_protected'") {
		t.Error("files table should have visibility check constraint")
	}

	// Verify size_bytes > 0 constraint
	if !strings.Contains(sql, "size_bytes > 0") {
		t.Error("files table should have size_bytes > 0 constraint")
	}
}
