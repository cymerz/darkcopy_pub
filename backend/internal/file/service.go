package file

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gthbn/pastebin/internal/access"
	"github.com/gthbn/pastebin/internal/paste"
	"github.com/gthbn/pastebin/internal/urlgen"
	"golang.org/x/crypto/bcrypt"
)

// MaxFileSize is the maximum allowed file size (100 MB).
const MaxFileSize = 100 * 1024 * 1024

// DefaultExpiryDuration is the default expiry when none is specified.
const DefaultExpiryDuration = 24 * time.Hour

// NeverExpires is a sentinel value for ExpiresIn indicating the file should never expire.
const NeverExpires = time.Duration(-1)

// DefaultPresignExpiry is the default expiry for presigned download URLs.
const DefaultPresignExpiry = 1 * time.Hour

// ErrPresignUnsupported is returned when the storage backend does not support presigning.
var ErrPresignUnsupported = errors.New("storage backend does not support presigned URLs")

// Errors returned by the file service.
var (
	ErrFileTooLarge     = errors.New("Ukuran file melebihi batas maksimum 100 MB")
	ErrPasswordRequired = errors.New("Kata sandi wajib diisi untuk visibilitas ini")
	ErrNotFound         = errors.New("file tidak ditemukan")
	ErrExpired          = errors.New("File ini telah kadaluarsa")
	ErrInvalidSlug      = errors.New("Format slug tidak valid")
)

// FileRepository defines the interface for file persistence operations.
type FileRepository interface {
	InsertFile(ctx context.Context, file *paste.FileRecord) error
	GetBySlug(ctx context.Context, slug string) (*paste.FileRecord, error)
	ListPublicRecent(ctx context.Context, limit int) ([]*paste.FileSummary, error)
	IncrementDownloads(ctx context.Context, slug string) error
}


// FileStorage defines the interface for file disk operations.
type FileStorage interface {
	Save(ctx context.Context, storageKey string, reader io.Reader) error
	Open(ctx context.Context, storageKey string) (io.ReadCloser, error)
	Delete(ctx context.Context, storageKey string) error
	Head(ctx context.Context, storageKey string) error
}

// UploadPresigner defines the interface for generating presigned upload URLs.
type UploadPresigner interface {
	PresignUploadURL(ctx context.Context, storageKey string, expires time.Duration, contentType string) (string, error)
}

// Service is the concrete implementation of FileService.
type Service struct {
	repo    FileRepository
	storage FileStorage
	urlGen  urlgen.URLGenerator
	now     func() time.Time
	// maxFileSizeFn, when set and returning > 0, overrides MaxFileSize.
	maxFileSizeFn func() int64
}

// NewService creates a new file Service with the given dependencies.
func NewService(repo FileRepository, storage FileStorage, urlGen urlgen.URLGenerator) *Service {
	return &Service{
		repo:    repo,
		storage: storage,
		urlGen:  urlGen,
		now:     time.Now,
	}
}

// SetMaxFileSizeFunc installs a function returning the current maximum file size
// in bytes. When unset (or it returns <= 0), the compile-time MaxFileSize
// constant is used. Keeps the constructor backward compatible.
func (s *Service) SetMaxFileSizeFunc(fn func() int64) {
	s.maxFileSizeFn = fn
}

// maxFileSize returns the effective maximum file size in bytes.
func (s *Service) maxFileSize() int64 {
	if s.maxFileSizeFn != nil {
		if v := s.maxFileSizeFn(); v > 0 {
			return v
		}
	}
	return MaxFileSize
}

// Upload validates the request, generates a unique slug, saves the file to disk,
// computes the expiry time, and persists the file metadata.
func (s *Service) Upload(ctx context.Context, req paste.UploadFileRequest) (*paste.FileRecord, error) {
	// Validate file size does not exceed the configured maximum.
	if req.Size > s.maxFileSize() {
		return nil, ErrFileTooLarge
	}

	// Validate password is required for password_protected visibility.
	if req.Visibility == paste.VisibilityPasswordProtected {
		if strings.TrimSpace(req.Password) == "" {
			return nil, ErrPasswordRequired
		}
	}

	// Hash password using bcrypt cost factor 10 if visibility is password_protected.
	var passwordHash string
	if req.Visibility == paste.VisibilityPasswordProtected {
		hash, err := access.HashPassword(req.Password)
		if err != nil {
			return nil, err
		}
		passwordHash = hash
	}

	// Generate a unique slug.
	slug, err := s.urlGen.GenerateSlug(ctx)
	if err != nil {
		return nil, err
	}

	// Sanitize filename to prevent path traversal
	filename := sanitizeFilename(req.Filename)

	// Build storage key: uploads/{slug}/{filename}
	storageKey := fmt.Sprintf("uploads/%s/%s", slug, filename)

	// Save file to disk.
	if err := s.storage.Save(ctx, storageKey, req.Reader); err != nil {
		return nil, err
	}

	now := s.now()

	// Default ExpiresIn to 24 hours if not set (zero value).
	expiresIn := req.ExpiresIn
	if expiresIn == 0 {
		expiresIn = DefaultExpiryDuration
	}

	// Calculate expires_at. NULL (nil) if ExpiresIn is negative (NeverExpires sentinel).
	var expiresAt *time.Time
	if expiresIn > 0 {
		t := now.Add(expiresIn)
		expiresAt = &t
	}

	record := &paste.FileRecord{
		ID:           uuid.New(),
		Slug:         slug,
		Filename:     filename,
		MIMEType:     req.MIMEType,
		SizeBytes:    req.Size,
		StorageKey:   storageKey,
		Visibility:   req.Visibility,
		PasswordHash: passwordHash,
		ExpiresAt:    expiresAt,
		CreatedAt:    now,
	}

	if err := s.repo.InsertFile(ctx, record); err != nil {
		return nil, err
	}

	return record, nil
}

// GetBySlug retrieves a file record by its slug.
// Returns ErrNotFound if the slug does not exist, or ErrExpired if the file has expired.
func (s *Service) GetBySlug(ctx context.Context, slug string) (*paste.FileRecord, error) {
	record, err := s.repo.GetBySlug(ctx, slug)
	if err != nil {
		return nil, ErrNotFound
	}

	if record.ExpiresAt != nil && record.ExpiresAt.Before(s.now()) {
		return nil, ErrExpired
	}

	return record, nil
}

// ListPublicRecent returns the most recent public files up to the given limit.
func (s *Service) ListPublicRecent(ctx context.Context, limit int) ([]*paste.FileSummary, error) {
	return s.repo.ListPublicRecent(ctx, limit)
}

// ServeFile retrieves a file by slug and streams it to the HTTP response writer
// with appropriate Content-Disposition, Content-Type, and Content-Length headers.
func (s *Service) ServeFile(ctx context.Context, slug string, w http.ResponseWriter) error {
	record, err := s.GetBySlug(ctx, slug)
	if err != nil {
		return err
	}

	inline := false
	if v := ctx.Value("serve_inline"); v != nil {
		if b, ok := v.(bool); ok {
			inline = b
		}
	}

	disposition := "attachment"
	if inline {
		disposition = "inline"
	}

	// Stream the file directly from S3 or local storage to the response writer.
	// This is 100% reliable, same-origin, and handles all browser players flawlessly.
	reader, err := s.storage.Open(ctx, record.StorageKey)
	if err != nil {
		return err
	}
	defer reader.Close()

	// Increment downloads!
	_ = s.repo.IncrementDownloads(ctx, slug)
	record.Downloads++

	w.Header().Set("Content-Disposition", fmt.Sprintf(`%s; filename="%s"`, disposition, record.Filename))
	w.Header().Set("Content-Type", record.MIMEType)
	w.Header().Set("X-Downloads-Count", strconv.Itoa(record.Downloads))

	// Enable seeking in browser video players by announcing byte-range support.
	w.Header().Set("Accept-Ranges", "bytes")

	var start, end int64
	isRange := false
	rangeHeader := ""
	if val := ctx.Value("range_header"); val != nil {
		if rStr, ok := val.(string); ok && rStr != "" {
			rangeHeader = rStr
		}
	}

	if rangeHeader != "" {
		if strings.HasPrefix(rangeHeader, "bytes=") {
			parts := strings.Split(rangeHeader[6:], "-")
			if len(parts) == 2 {
				var parseErr error
				start, parseErr = strconv.ParseInt(parts[0], 10, 64)
				if parseErr != nil {
					start = 0
				}
				if parts[1] == "" {
					end = record.SizeBytes - 1
				} else {
					end, parseErr = strconv.ParseInt(parts[1], 10, 64)
					if parseErr != nil {
						end = record.SizeBytes - 1
					}
				}
				if start >= 0 && start < record.SizeBytes && end >= start && end < record.SizeBytes {
					isRange = true
				}
			}
		}
	}
	var streamReader io.Reader = reader

	if isRange {
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, record.SizeBytes))
		w.Header().Set("Content-Length", strconv.FormatInt(end-start+1, 10))
		w.WriteHeader(http.StatusPartialContent) // HTTP 206 Partial Content

		// If reader is seekable (local filesystem os.File), seek to the start offset.
		if seeker, ok := reader.(io.ReadSeeker); ok {
			var seekErr error
			if _, seekErr = seeker.Seek(start, io.SeekStart); seekErr != nil {
				return seekErr
			}
		}
		// Read only the requested range slice size.
		streamReader = io.LimitReader(reader, end-start+1)
	} else {
		w.Header().Set("Content-Length", strconv.FormatInt(record.SizeBytes, 10))
		w.WriteHeader(http.StatusOK) // HTTP 200 OK
	}

	// Wrap the S3 network stream in a buffered reader to minimize network I/O system calls
	// and drastically speed up downloads from distant cloud regions.
	bufferedStream := bufio.NewReaderSize(streamReader, 256*1024) // 256 KB buffer

	// Optimize streaming performance for media (videos/audio) by using a massive 4MB buffer.
	// This reduces small TCP/I/O system call overhead, preventing video buffering lags.
	buf := make([]byte, 4*1024*1024) // 4 MB buffer
	_, err = io.CopyBuffer(w, bufferedStream, buf)
	return err
}

// Presigner is an optional interface that storage backends can implement
// to support generating presigned download URLs (e.g. S3).
type Presigner interface {
	PresignURL(ctx context.Context, storageKey string, expires time.Duration, inline bool) (string, error)
}

// PresignDownloadURL generates a temporary presigned URL for direct S3 download.
// Returns ErrPresignUnsupported if the storage backend does not support presigning.
func (s *Service) PresignDownloadURL(ctx context.Context, slug string, inline bool) (string, error) {
	record, err := s.GetBySlug(ctx, slug)
	if err != nil {
		return "", err
	}

	presigner, ok := s.storage.(Presigner)
	if !ok {
		return "", ErrPresignUnsupported
	}

	return presigner.PresignURL(ctx, record.StorageKey, DefaultPresignExpiry, inline)
}

// ValidatePassword verifies a password against the stored hash for a file.
// Returns true if the password is correct or if the file is not password-protected.
func (s *Service) ValidatePassword(ctx context.Context, slug, password string) (bool, error) {
	record, err := s.GetBySlug(ctx, slug)
	if err != nil {
		return false, err
	}
	if record.PasswordHash == "" {
		return true, nil
	}
	// Use bcrypt to compare
	err = bcrypt.CompareHashAndPassword([]byte(record.PasswordHash), []byte(password))
	if err == nil {
		return true, nil
	}
	if err == bcrypt.ErrMismatchedHashAndPassword {
		return false, nil
	}
	return false, err
}

// IncrementDownloads increments the download count of a file by its slug.
func (s *Service) IncrementDownloads(ctx context.Context, slug string) error {
	return s.repo.IncrementDownloads(ctx, slug)
}

// SupportsUploadPresigning returns true if the underlying storage supports upload presigning.
func (s *Service) SupportsUploadPresigning() bool {
	_, ok := s.storage.(UploadPresigner)
	return ok
}

// PresignUploadURL generates a unique slug, constructs a storage key, and returns
// a pre-signed S3 upload URL for PUT request.
func (s *Service) PresignUploadURL(ctx context.Context, filename, contentType string) (slug, storageKey, uploadURL string, err error) {
	presigner, ok := s.storage.(UploadPresigner)
	if !ok {
		return "", "", "", ErrPresignUnsupported
	}

	// Sanitize filename to prevent path traversal
	sanitizedFilename := sanitizeFilename(filename)

	slug, err = s.urlGen.GenerateSlug(ctx)
	if err != nil {
		return "", "", "", err
	}

	storageKey = fmt.Sprintf("uploads/%s/%s", slug, sanitizedFilename)
	uploadURL, err = presigner.PresignUploadURL(ctx, storageKey, DefaultPresignExpiry, contentType)
	if err != nil {
		return "", "", "", err
	}

	return slug, storageKey, uploadURL, nil
}

// RegisterUploadedFile verifies the uploaded file exists in storage and records its metadata in PostgreSQL.
func (s *Service) RegisterUploadedFile(ctx context.Context, req paste.RegisterFileRequest) (*paste.FileRecord, error) {
	// Validate slug format to prevent directory traversal or access bypass.
	// Slugs should be alphanumeric only.
	for _, r := range req.Slug {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
			return nil, ErrInvalidSlug
		}
	}

	// Sanitize filename to prevent directory traversal
	filename := sanitizeFilename(req.Filename)

	// Reconstruct expected StorageKey strictly on backend using Slug and Sanitized Filename
	// to prevent clients from registering arbitrary keys or accessing other users' files.
	expectedStorageKey := fmt.Sprintf("uploads/%s/%s", req.Slug, filename)

	// 1. Verify file exists in storage.
	if err := s.storage.Head(ctx, expectedStorageKey); err != nil {
		return nil, fmt.Errorf("file tidak ditemukan di storage: %w", err)
	}

	// 2. Validate password is required for password_protected visibility.
	if req.Visibility == paste.VisibilityPasswordProtected {
		if strings.TrimSpace(req.Password) == "" {
			return nil, ErrPasswordRequired
		}
	}

	// 3. Hash password using bcrypt.
	var passwordHash string
	if req.Visibility == paste.VisibilityPasswordProtected {
		hash, err := access.HashPassword(req.Password)
		if err != nil {
			return nil, err
		}
		passwordHash = hash
	}

	now := s.now()

	// 4. Default ExpiresIn to 24 hours if not set (zero value).
	expiresIn := req.ExpiresIn
	if expiresIn == 0 {
		expiresIn = DefaultExpiryDuration
	}

	// Calculate expires_at. NULL (nil) if ExpiresIn is negative (NeverExpires sentinel).
	var expiresAt *time.Time
	if expiresIn > 0 {
		t := now.Add(expiresIn)
		expiresAt = &t
	}

	record := &paste.FileRecord{
		ID:           uuid.New(),
		Slug:         req.Slug,
		Filename:     filename,
		MIMEType:     req.MIMEType,
		SizeBytes:    req.Size,
		StorageKey:   expectedStorageKey,
		Visibility:   req.Visibility,
		PasswordHash: passwordHash,
		ExpiresAt:    expiresAt,
		CreatedAt:    now,
	}

	if err := s.repo.InsertFile(ctx, record); err != nil {
		return nil, err
	}

	return record, nil
}

// sanitizeFilename strips directory components and traversal sequences from a filename.
func sanitizeFilename(filename string) string {
	// Convert backslashes to forward slashes
	filename = strings.ReplaceAll(filename, "\\", "/")
	
	// Remove all ".." to prevent any directory traversal
	for strings.Contains(filename, "..") {
		filename = strings.ReplaceAll(filename, "..", "")
	}

	// Get the base name using path package (which understands forward slashes)
	filename = path.Base(filename)

	// Remove remaining slashes to be absolutely safe
	filename = strings.ReplaceAll(filename, "/", "")
	filename = strings.ReplaceAll(filename, "\\", "")

	if filename == "" || filename == "." || filename == ".." {
		filename = "uploaded_file"
	}
	return filename
}

