// Package db provides database connectivity and migration support using pgx.
package db

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Connect establishes a connection pool to PostgreSQL using the provided
// connection string. It returns a *pgxpool.Pool ready for use.
func Connect(ctx context.Context, connString string) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("db: failed to parse connection string: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("db: failed to create connection pool: %w", err)
	}

	// Verify connectivity
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("db: failed to ping database: %w", err)
	}

	return pool, nil
}

// RunMigrations reads all SQL migration files from the embedded migrations
// directory and executes them in alphabetical order. It uses a migrations
// tracking table to ensure each migration is only applied once.
func RunMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	// Create migrations tracking table if it doesn't exist
	_, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			id          SERIAL PRIMARY KEY,
			filename    VARCHAR(255) NOT NULL UNIQUE,
			applied_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("db: failed to create schema_migrations table: %w", err)
	}

	// Read all migration files
	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("db: failed to read migrations directory: %w", err)
	}

	// Sort entries by name to ensure consistent ordering
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}

		filename := entry.Name()

		// Check if migration has already been applied
		var exists bool
		err := pool.QueryRow(ctx,
			"SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE filename = $1)",
			filename,
		).Scan(&exists)
		if err != nil {
			return fmt.Errorf("db: failed to check migration status for %s: %w", filename, err)
		}

		if exists {
			continue
		}

		// Read and execute migration
		content, err := fs.ReadFile(migrationsFS, "migrations/"+filename)
		if err != nil {
			return fmt.Errorf("db: failed to read migration file %s: %w", filename, err)
		}

		_, err = pool.Exec(ctx, string(content))
		if err != nil {
			return fmt.Errorf("db: failed to execute migration %s: %w", filename, err)
		}

		// Record migration as applied
		_, err = pool.Exec(ctx,
			"INSERT INTO schema_migrations (filename) VALUES ($1)",
			filename,
		)
		if err != nil {
			return fmt.Errorf("db: failed to record migration %s: %w", filename, err)
		}
	}

	return nil
}
