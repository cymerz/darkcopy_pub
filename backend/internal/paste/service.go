package paste

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gthbn/pastebin/internal/access"
	"github.com/gthbn/pastebin/internal/urlgen"
)

// slugPattern allows lowercase letters, digits, and hyphens (3–64 chars).
var slugPattern = regexp.MustCompile(`^[a-z0-9-]{3,64}$`)

// MaxContentSize is the maximum allowed paste content size (10 MB).
const MaxContentSize = 10 * 1024 * 1024

// NeverExpires is a sentinel value for ExpiresIn indicating the paste should never expire.
const NeverExpires = time.Duration(-1)

// Errors returned by the paste service.
var (
	ErrEmptyContent     = errors.New("Konten paste tidak boleh kosong")
	ErrContentTooLarge  = errors.New("Ukuran konten melebihi batas maksimum 10 MB")
	ErrPasswordRequired = errors.New("Kata sandi wajib diisi untuk visibilitas ini")
	ErrSlugTaken        = errors.New("Slug sudah digunakan, pilih yang lain")
	ErrSlugInvalid      = errors.New("Slug hanya boleh mengandung huruf, angka, dan tanda hubung")
	ErrNotFound         = errors.New("paste tidak ditemukan")
	ErrExpired          = errors.New("Paste ini telah kadaluarsa")
)

// PasteRepository defines the interface for paste persistence operations.
type PasteRepository interface {
	InsertPaste(ctx context.Context, paste *Paste) error
	GetBySlug(ctx context.Context, slug string) (*Paste, error)
	ListPublicRecent(ctx context.Context, limit int) ([]*PasteSummary, error)
}

// Service is the concrete implementation of PasteService.
type Service struct {
	repo      PasteRepository
	urlGen    urlgen.URLGenerator
	accessCtl access.AccessController
	now       func() time.Time
	// maxContentSize, when > 0, overrides MaxContentSize for the size check.
	// Set via SetMaxContentSizeFunc to support runtime-configurable limits.
	maxContentSizeFn func() int64
}

// NewService creates a new paste Service with the given dependencies.
func NewService(repo PasteRepository, urlGen urlgen.URLGenerator, accessCtl access.AccessController) *Service {
	return &Service{
		repo:      repo,
		urlGen:    urlGen,
		accessCtl: accessCtl,
		now:       time.Now,
	}
}

// SetMaxContentSizeFunc installs a function that returns the current maximum
// paste content size in bytes. When unset (or it returns <= 0), the compile-time
// MaxContentSize constant is used. This keeps the constructor backward
// compatible while allowing runtime-configurable limits.
func (s *Service) SetMaxContentSizeFunc(fn func() int64) {
	s.maxContentSizeFn = fn
}

// maxContentSize returns the effective maximum paste size in bytes.
func (s *Service) maxContentSize() int64 {
	if s.maxContentSizeFn != nil {
		if v := s.maxContentSizeFn(); v > 0 {
			return v
		}
	}
	return MaxContentSize
}

// GetBySlug retrieves a paste by its slug. Returns ErrNotFound if the paste
// does not exist, and ErrExpired if the paste has passed its expiry time.
func (s *Service) GetBySlug(ctx context.Context, slug string) (*Paste, error) {
	paste, err := s.repo.GetBySlug(ctx, slug)
	if err != nil {
		return nil, ErrNotFound
	}

	if paste.ExpiresAt != nil && paste.ExpiresAt.Before(s.now()) {
		return nil, ErrExpired
	}

	return paste, nil
}

// ListPublicRecent returns the most recent public pastes up to the given limit.
func (s *Service) ListPublicRecent(ctx context.Context, limit int) ([]*PasteSummary, error) {
	return s.repo.ListPublicRecent(ctx, limit)
}

// Create validates the request, generates a unique slug, hashes the password
// if needed, computes the expiry time, and persists the paste.
func (s *Service) Create(ctx context.Context, req CreatePasteRequest) (*Paste, error) {
	// Validate content is not empty or whitespace-only.
	if strings.TrimSpace(req.Content) == "" {
		return nil, ErrEmptyContent
	}

	// Validate content size does not exceed the configured maximum.
	if int64(len(req.Content)) > s.maxContentSize() {
		return nil, ErrContentTooLarge
	}

	// Validate password is required for password_protected visibility.
	if req.Visibility == VisibilityPasswordProtected {
		if strings.TrimSpace(req.Password) == "" {
			return nil, ErrPasswordRequired
		}
	}

	// Hash password using bcrypt cost factor 10 if visibility is password_protected.
	var passwordHash string
	if req.Visibility == VisibilityPasswordProtected {
		hash, err := access.HashPassword(req.Password)
		if err != nil {
			return nil, err
		}
		passwordHash = hash
	}

	// Generate or validate slug.
	var slug string
	if req.CustomSlug != "" {
		custom := strings.ToLower(strings.TrimSpace(req.CustomSlug))
		if !slugPattern.MatchString(custom) {
			return nil, ErrSlugInvalid
		}
		// Check availability by attempting a lookup.
		if _, err := s.repo.GetBySlug(ctx, custom); err == nil {
			return nil, ErrSlugTaken
		}
		slug = custom
	} else {
		var err error
		slug, err = s.urlGen.GenerateSlug(ctx)
		if err != nil {
			return nil, err
		}
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

	paste := &Paste{
		ID:           uuid.New(),
		Slug:         slug,
		Title:        req.Title,
		Content:      req.Content,
		Language:     req.Language,
		Visibility:   req.Visibility,
		PasswordHash: passwordHash,
		ExpiresAt:    expiresAt,
		CreatedAt:    now,
	}

	if err := s.repo.InsertPaste(ctx, paste); err != nil {
		return nil, err
	}

	return paste, nil
}

// ValidatePassword checks whether the given password grants access to the paste
// identified by slug. Returns true if access is granted, false otherwise.
// Returns ErrNotFound if the paste does not exist, ErrExpired if it has expired.
// For public/unlisted pastes (no password hash), returns true immediately.
func (s *Service) ValidatePassword(ctx context.Context, slug, password string) (bool, error) {
	paste, err := s.repo.GetBySlug(ctx, slug)
	if err != nil {
		return false, ErrNotFound
	}

	if paste.ExpiresAt != nil && paste.ExpiresAt.Before(s.now()) {
		return false, ErrExpired
	}

	// Public/unlisted pastes have no password hash — access is always granted.
	if paste.PasswordHash == "" {
		return true, nil
	}

	result, err := s.accessCtl.CheckAccess(ctx, paste.PasswordHash, password)
	if err != nil {
		return false, err
	}

	return result == access.AccessGranted, nil
}
