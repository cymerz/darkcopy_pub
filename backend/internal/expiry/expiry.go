// Package expiry provides the expiry manager for cleaning up expired pastes and files.
package expiry

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
)

// ExpiryManager defines the interface for managing resource expiration.
type ExpiryManager interface {
	Start(ctx context.Context)
	RunCleanup(ctx context.Context) (deleted int, err error)
}

// ExpiredPaste represents a paste that has expired.
type ExpiredPaste struct {
	ID   uuid.UUID
	Slug string
}

// ExpiredFile represents a file that has expired.
type ExpiredFile struct {
	ID         uuid.UUID
	Slug       string
	StorageKey string
}

// ExpiredItemStore defines the interface for querying and deleting expired items.
type ExpiredItemStore interface {
	// ListExpiredPastes returns up to `limit` pastes with expires_at < now.
	ListExpiredPastes(ctx context.Context, now time.Time, limit int) ([]ExpiredPaste, error)
	// DeletePaste deletes a paste by its ID.
	DeletePaste(ctx context.Context, id uuid.UUID) error
	// ListExpiredFiles returns up to `limit` files with expires_at < now.
	ListExpiredFiles(ctx context.Context, now time.Time, limit int) ([]ExpiredFile, error)
	// DeleteFile deletes a file record by its ID.
	DeleteFile(ctx context.Context, id uuid.UUID) error
}

// Locker defines the interface for distributed locks.
type Locker interface {
	AcquireLock(ctx context.Context, key string, ttl time.Duration) (bool, error)
	ReleaseLock(ctx context.Context, key string) error
}

// FileDeleter defines the interface for deleting files from storage.
type FileDeleter interface {
	Delete(ctx context.Context, storageKey string) error
}

// Manager is the concrete implementation of ExpiryManager.
type Manager struct {
	store       ExpiredItemStore
	fileDeleter FileDeleter
	locker      Locker
	logger      *slog.Logger
	now         func() time.Time
	interval    time.Duration
	batchSize   int
}

