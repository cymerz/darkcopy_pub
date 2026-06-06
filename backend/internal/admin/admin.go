// Package admin provides administrative operations for managing pastes and
// files. It exposes list, delete, and aggregate-stat capabilities that are not
// available through the public API and are intended to be guarded by an admin
// token at the HTTP layer.
package admin

import (
	"context"
	"errors"
	"time"

	"github.com/gthbn/pastebin/internal/paste"
)

// ErrNotFound is returned when the requested item does not exist.
var ErrNotFound = errors.New("item tidak ditemukan")

// ErrPurgeUnavailable is returned by PurgeExpired when no purger is configured.
var ErrPurgeUnavailable = errors.New("pembersihan kadaluarsa tidak tersedia")

// PasteItem is an admin-facing view of a paste. Unlike paste.PasteSummary it
// includes the visibility and a password flag so an administrator can see every
// paste regardless of visibility or expiry state.
type PasteItem struct {
	Slug        string     `json:"slug"`
	Title       string     `json:"title"`
	Language    string     `json:"language"`
	Visibility  string     `json:"visibility"`
	HasPassword bool       `json:"has_password"`
	CreatedAt   time.Time  `json:"created_at"`
	ExpiresAt   *time.Time `json:"expires_at"`
}

// FileItem is an admin-facing view of an uploaded file.
type FileItem struct {
	Slug        string     `json:"slug"`
	Filename    string     `json:"filename"`
	MIMEType    string     `json:"mime_type"`
	SizeBytes   int64      `json:"size_bytes"`
	Visibility  string     `json:"visibility"`
	HasPassword bool       `json:"has_password"`
	CreatedAt   time.Time  `json:"created_at"`
	ExpiresAt   *time.Time `json:"expires_at"`
}

// ProviderStats holds sharding distribution stats for a single S3 provider.
type ProviderStats struct {
	ProviderName string `json:"provider_name"`
	FilesCount   int    `json:"files_count"`
	SizeBytes    int64  `json:"size_bytes"`
}

// Stats holds aggregate counts across the system.
type Stats struct {
	TotalPastes   int             `json:"total_pastes"`
	TotalFiles    int             `json:"total_files"`
	TotalBytes    int64           `json:"total_bytes"`
	ProviderStats []ProviderStats `json:"provider_stats"`
}

// PasteRepository defines the paste persistence operations needed by the admin
// service.
type PasteRepository interface {
	ListAllPastes(ctx context.Context, limit, offset int) ([]*PasteItem, error)
	DeletePasteBySlug(ctx context.Context, slug string) (bool, error)
	CountPastes(ctx context.Context) (int, error)
}

// FileRepository defines the file persistence operations needed by the admin
// service.
type FileRepository interface {
	ListAllFiles(ctx context.Context, limit, offset int) ([]*FileItem, error)
	GetBySlug(ctx context.Context, slug string) (*paste.FileRecord, error)
	DeleteFileBySlug(ctx context.Context, slug string) (bool, error)
	CountFiles(ctx context.Context) (int, error)
}

// FileStorage removes file blobs from the underlying storage.
type FileStorage interface {
	Delete(ctx context.Context, storageKey string) error
}

// Purger triggers an on-demand cleanup of expired items. It is satisfied by the
// expiry manager's RunCleanup, which deletes a batch of expired pastes/files
// (DB rows and, for files, the blobs on disk) and returns how many it removed.
type Purger interface {
	RunCleanup(ctx context.Context) (int, error)
}

// Service is the concrete implementation of the admin operations.
type Service struct {
	pasteRepo PasteRepository
	fileRepo  FileRepository
	storage   FileStorage
	purger    Purger
}

// NewService creates a new admin Service with the given dependencies. The
// purger may be nil, in which case PurgeExpired returns ErrPurgeUnavailable.
func NewService(pasteRepo PasteRepository, fileRepo FileRepository, storage FileStorage, purger Purger) *Service {
	return &Service{
		pasteRepo: pasteRepo,
		fileRepo:  fileRepo,
		storage:   storage,
		purger:    purger,
	}
}

// ListPastes returns all pastes (any visibility, including expired) ordered by
// most recent first.
func (s *Service) ListPastes(ctx context.Context, limit, offset int) ([]*PasteItem, error) {
	return s.pasteRepo.ListAllPastes(ctx, limit, offset)
}

// DeletePaste permanently removes a paste by its slug. Returns ErrNotFound when
// no paste matches the slug.
func (s *Service) DeletePaste(ctx context.Context, slug string) error {
	deleted, err := s.pasteRepo.DeletePasteBySlug(ctx, slug)
	if err != nil {
		return err
	}
	if !deleted {
		return ErrNotFound
	}
	return nil
}

// ListFiles returns all uploaded files ordered by most recent first.
func (s *Service) ListFiles(ctx context.Context, limit, offset int) ([]*FileItem, error) {
	return s.fileRepo.ListAllFiles(ctx, limit, offset)
}

// DeleteFile permanently removes a file record and its blob on disk. The DB row
// is deleted first; the blob is then removed on a best-effort basis (a missing
// blob does not fail the operation). Returns ErrNotFound when no file matches.
func (s *Service) DeleteFile(ctx context.Context, slug string) error {
	record, err := s.fileRepo.GetBySlug(ctx, slug)
	if err != nil {
		return ErrNotFound
	}

	deleted, err := s.fileRepo.DeleteFileBySlug(ctx, slug)
	if err != nil {
		return err
	}
	if !deleted {
		return ErrNotFound
	}

	// Best-effort blob removal — the DB record is already gone, so a storage
	// error here should not surface as a failure to the caller.
	_ = s.storage.Delete(ctx, record.StorageKey)
	return nil
}

// Stats returns aggregate counts of pastes and files.
func (s *Service) Stats(ctx context.Context) (*Stats, error) {
	pasteCount, err := s.pasteRepo.CountPastes(ctx)
	if err != nil {
		return nil, err
	}
	fileCount, err := s.fileRepo.CountFiles(ctx)
	if err != nil {
		return nil, err
	}

	// Fetch all files metadata to calculate exact byte distribution
	// Limit is set to 100,000 as a reasonable upper bound for fully-in-memory aggregation.
	files, err := s.fileRepo.ListAllFiles(ctx, 100000, 0)
	if err != nil {
		return nil, err
	}

	var totalBytes int64
	for _, f := range files {
		totalBytes += f.SizeBytes
	}

	var providerStats []ProviderStats

	// Check if storage is MultiS3Storage to calculate provider sharding stats
	if multiStorage, ok := s.storage.(interface {
		GetProviderNames() []string
		GetProviderIndex(storageKey string) int
	}); ok {
		names := multiStorage.GetProviderNames()
		if len(names) > 0 {
			counts := make([]int, len(names))
			sizes := make([]int64, len(names))

			for _, f := range files {
				// Rebuild storage key: uploads/{slug}/{filename}
				storageKey := "uploads/" + f.Slug + "/" + f.Filename
				idx := multiStorage.GetProviderIndex(storageKey)
				if idx >= 0 && idx < len(names) {
					counts[idx]++
					sizes[idx] += f.SizeBytes
				}
			}

			for i, name := range names {
				providerStats = append(providerStats, ProviderStats{
					ProviderName: name,
					FilesCount:   counts[i],
					SizeBytes:    sizes[i],
				})
			}
		}
	} else {
		// If single S3 bucket is active
		providerName := "SINGLE BUCKET"
		if fileCount > 0 {
			providerStats = append(providerStats, ProviderStats{
				ProviderName: providerName,
				FilesCount:   fileCount,
				SizeBytes:    totalBytes,
			})
		}
	}

	return &Stats{
		TotalPastes:   pasteCount,
		TotalFiles:    fileCount,
		TotalBytes:    totalBytes,
		ProviderStats: providerStats,
	}, nil
}

// PurgeExpired triggers an immediate cleanup of expired pastes and files,
// returning the number of items removed. It runs the cleanup repeatedly until a
// cycle removes nothing, so it drains backlogs larger than a single batch.
// Returns ErrPurgeUnavailable when no purger is configured.
func (s *Service) PurgeExpired(ctx context.Context) (int, error) {
	if s.purger == nil {
		return 0, ErrPurgeUnavailable
	}

	total := 0
	for {
		deleted, err := s.purger.RunCleanup(ctx)
		total += deleted
		if err != nil {
			return total, err
		}
		if deleted == 0 {
			return total, nil
		}
	}
}
