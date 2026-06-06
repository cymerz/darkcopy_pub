package expiry

import (
	"context"
	"log/slog"
	"time"
)

// DefaultInterval is the default cleanup interval (5 minutes).
const DefaultInterval = 5 * time.Minute

// DefaultBatchSize is the default maximum items per cleanup cycle.
const DefaultBatchSize = 100

// Option is a functional option for configuring the Manager.
type Option func(*Manager)

// WithInterval sets the ticker interval for the cleanup goroutine.
func WithInterval(d time.Duration) Option {
	return func(m *Manager) {
		m.interval = d
	}
}

// WithBatchSize sets the maximum number of items processed per cleanup cycle.
func WithBatchSize(size int) Option {
	return func(m *Manager) {
		m.batchSize = size
	}
}

// WithNowFunc sets the time function used for determining "now" (useful for testing).
func WithNowFunc(fn func() time.Time) Option {
	return func(m *Manager) {
		m.now = fn
	}
}

// WithLogger sets the logger for the Manager.
func WithLogger(logger *slog.Logger) Option {
	return func(m *Manager) {
		m.logger = logger
	}
}

// NewManager creates a new Manager with the given dependencies and options.
func NewManager(store ExpiredItemStore, fileDeleter FileDeleter, opts ...Option) *Manager {
	m := &Manager{
		store:       store,
		fileDeleter: fileDeleter,
		logger:      slog.Default(),
		now:         time.Now,
		interval:    DefaultInterval,
		batchSize:   DefaultBatchSize,
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// Start launches a background goroutine that runs RunCleanup on each tick.
// It performs an initial cleanup immediately (so already-expired items are
// removed at startup instead of waiting for the first tick) and stops when the
// provided context is cancelled.
func (m *Manager) Start(ctx context.Context) {
	ticker := time.NewTicker(m.interval)
	go func() {
		defer ticker.Stop()
		// Initial sweep at startup — closes the gap for items that expired
		// while the server was not running.
		m.runOnce(ctx)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.runOnce(ctx)
			}
		}
	}()
}

// runOnce executes a single cleanup cycle and logs the outcome.
func (m *Manager) runOnce(ctx context.Context) {
	deleted, err := m.RunCleanup(ctx)
	if err != nil {
		m.logger.Error("cleanup cycle failed", "error", err)
	} else if deleted > 0 {
		m.logger.Info("cleanup cycle completed", "deleted", deleted)
	}
}

// RunCleanup queries expired pastes and files and deletes them.
// It processes up to batchSize items total per invocation.
// If deletion of an individual item fails, it logs the error and continues.
// Returns the total number of successfully deleted items.
func (m *Manager) RunCleanup(ctx context.Context) (int, error) {
	now := m.now()
	deleted := 0

	// 1. Query expired pastes (up to batchSize).
	expiredPastes, err := m.store.ListExpiredPastes(ctx, now, m.batchSize)
	if err != nil {
		return 0, err
	}

	// 2. Delete each expired paste.
	for _, p := range expiredPastes {
		if err := m.store.DeletePaste(ctx, p.ID); err != nil {
			m.logger.Error("failed to delete expired paste",
				"paste_id", p.ID,
				"slug", p.Slug,
				"error", err,
			)
			continue
		}
		deleted++
	}

	// 3. Query expired files (up to batchSize - deletedPastes to stay within batch limit).
	remaining := m.batchSize - len(expiredPastes)
	if remaining <= 0 {
		return deleted, nil
	}

	expiredFiles, err := m.store.ListExpiredFiles(ctx, now, remaining)
	if err != nil {
		return deleted, err
	}

	// 4. Delete each expired file: disk first, then DB.
	for _, f := range expiredFiles {
		// Delete from disk first.
		if err := m.fileDeleter.Delete(ctx, f.StorageKey); err != nil {
			m.logger.Error("failed to delete file from storage",
				"file_id", f.ID,
				"slug", f.Slug,
				"storage_key", f.StorageKey,
				"error", err,
			)
			continue
		}

		// Delete from DB.
		if err := m.store.DeleteFile(ctx, f.ID); err != nil {
			m.logger.Error("failed to delete file record from database",
				"file_id", f.ID,
				"slug", f.Slug,
				"error", err,
			)
			continue
		}
		deleted++
	}

	return deleted, nil
}
