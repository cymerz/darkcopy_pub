// Package settings provides runtime-configurable application settings that an
// administrator can change without redeploying. Settings are held in a
// thread-safe Provider (read on the hot path by services/handlers) and
// persisted through a Store. A Manager ties the two together, validating and
// applying updates atomically.
package settings

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// Hard ceilings that admin-configured values may not exceed. They bound the
// blast radius of a misconfiguration (e.g. an enormous max upload size).
const (
	// MaxPasteSizeCeiling is the largest configurable paste size (50 MB).
	MaxPasteSizeCeiling int64 = 50 * 1024 * 1024
	// MaxFileSizeCeiling is the largest configurable file size (500 MB).
	MaxFileSizeCeiling int64 = 500 * 1024 * 1024
	// MaxExpiryOptions caps how many expiry choices may be configured.
	MaxExpiryOptions = 20
	// MaxDailyLimit caps the per-day quota values.
	MaxDailyLimit = 1_000_000
)

// ExpiryOption is a selectable expiry duration expressed in minutes. A value of
// 0 means "never expires" (only meaningful for pastes).
type ExpiryOption struct {
	Label   string `json:"label"`
	Minutes int64  `json:"minutes"`
}

// Settings holds all runtime-configurable values.
type Settings struct {
	// MaxPasteSizeBytes is the maximum allowed paste content size in bytes.
	MaxPasteSizeBytes int64 `json:"max_paste_size_bytes"`
	// MaxFileSizeBytes is the maximum allowed uploaded file size in bytes.
	MaxFileSizeBytes int64 `json:"max_file_size_bytes"`
	// PasteExpiryOptions are the expiry choices offered on the new-paste form.
	PasteExpiryOptions []ExpiryOption `json:"paste_expiry_options"`
	// FileExpiryOptions are the expiry choices offered on the upload form.
	FileExpiryOptions []ExpiryOption `json:"file_expiry_options"`
	// MaxPastesPerDayPerIP limits paste creations per client IP per day.
	// 0 means unlimited.
	MaxPastesPerDayPerIP int `json:"max_pastes_per_day_per_ip"`
	// MaxFileUploadsPerDayPerIP limits file uploads per client IP per day.
	// 0 means unlimited.
	MaxFileUploadsPerDayPerIP int `json:"max_file_uploads_per_day_per_ip"`
	// DisableNewPastes temporarily disables creation of new pastes.
	DisableNewPastes bool `json:"disable_new_pastes"`
	// DisableFileUploads temporarily disables uploading new files.
	DisableFileUploads bool `json:"disable_file_uploads"`
	// MaxDailyUploadBytes limits the total uploaded size (in bytes) across all IPs combined per day.
	// 0 means unlimited.
	MaxDailyUploadBytes int64 `json:"max_daily_upload_bytes"`
	// MaxDailyUploadBytesPerIP limits the total uploaded size (in bytes) per individual IP per day.
	// 0 means unlimited.
	MaxDailyUploadBytesPerIP int64 `json:"max_daily_upload_bytes_per_ip"`
	// UseDirectUpload enables direct client-to-S3 uploads using pre-signed PUT URLs.
	// Falls back to proxy upload if the storage provider does not support it.
	UseDirectUpload bool `json:"use_direct_upload"`
}

// Defaults returns the built-in default settings, mirroring the values that
// were previously hard-coded as constants.
func Defaults() Settings {
	return Settings{
		MaxPasteSizeBytes: 10 * 1024 * 1024,  // 10 MB
		MaxFileSizeBytes:  100 * 1024 * 1024, // 100 MB
		UseDirectUpload:   false,
		PasteExpiryOptions: []ExpiryOption{
			{Label: "1 Jam", Minutes: 60},
			{Label: "6 Jam", Minutes: 360},
			{Label: "24 Jam", Minutes: 1440},
			{Label: "7 Hari", Minutes: 10080},
			{Label: "30 Hari", Minutes: 43200},
			{Label: "Selamanya", Minutes: 0},
		},
		FileExpiryOptions: []ExpiryOption{
			{Label: "1 Jam", Minutes: 60},
			{Label: "6 Jam", Minutes: 360},
			{Label: "24 Jam", Minutes: 1440},
			{Label: "7 Hari", Minutes: 10080},
			{Label: "30 Hari", Minutes: 43200},
		},
	}
}

// Validation errors.
var (
	ErrInvalidPasteSize   = errors.New("ukuran paste maksimum tidak valid")
	ErrInvalidFileSize    = errors.New("ukuran file maksimum tidak valid")
	ErrNoExpiryOptions    = errors.New("minimal satu pilihan waktu kadaluarsa diperlukan")
	ErrTooManyExpiry      = fmt.Errorf("terlalu banyak pilihan waktu kadaluarsa (maks %d)", MaxExpiryOptions)
	ErrInvalidExpiry      = errors.New("pilihan waktu kadaluarsa tidak valid")
	ErrInvalidDailyLimit  = errors.New("batas harian tidak valid")
)

// Validate checks the settings for consistency and safe bounds. It returns a
// descriptive error when a value is out of range.
func (s Settings) Validate() error {
	if s.MaxPasteSizeBytes <= 0 || s.MaxPasteSizeBytes > MaxPasteSizeCeiling {
		return ErrInvalidPasteSize
	}
	if s.MaxFileSizeBytes <= 0 || s.MaxFileSizeBytes > MaxFileSizeCeiling {
		return ErrInvalidFileSize
	}
	if err := validateExpiryOptions(s.PasteExpiryOptions); err != nil {
		return err
	}
	if err := validateExpiryOptions(s.FileExpiryOptions); err != nil {
		return err
	}
	if s.MaxPastesPerDayPerIP < 0 || s.MaxPastesPerDayPerIP > MaxDailyLimit {
		return ErrInvalidDailyLimit
	}
	if s.MaxFileUploadsPerDayPerIP < 0 || s.MaxFileUploadsPerDayPerIP > MaxDailyLimit {
		return ErrInvalidDailyLimit
	}
	if s.MaxDailyUploadBytes < 0 {
		return errors.New("batas ukuran upload harian global tidak boleh negatif")
	}
	if s.MaxDailyUploadBytesPerIP < 0 {
		return errors.New("batas ukuran upload harian per IP tidak boleh negatif")
	}
	return nil
}

func validateExpiryOptions(opts []ExpiryOption) error {
	if len(opts) == 0 {
		return ErrNoExpiryOptions
	}
	if len(opts) > MaxExpiryOptions {
		return ErrTooManyExpiry
	}
	for _, o := range opts {
		if o.Label == "" || o.Minutes < 0 {
			return ErrInvalidExpiry
		}
	}
	return nil
}

// Provider holds the current settings behind a read/write mutex so it can be
// read concurrently on the request hot path and updated atomically by an admin.
type Provider struct {
	mu  sync.RWMutex
	cur Settings
}

// NewProvider creates a Provider seeded with the given settings.
func NewProvider(s Settings) *Provider {
	return &Provider{cur: s}
}

// Get returns a copy of the current settings.
func (p *Provider) Get() Settings {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.cur.clone()
}

// Set atomically replaces the current settings.
func (p *Provider) Set(s Settings) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cur = s.clone()
}

// MaxPasteSize returns the configured maximum paste size in bytes.
func (p *Provider) MaxPasteSize() int64 {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.cur.MaxPasteSizeBytes
}

// MaxFileSize returns the configured maximum file size in bytes.
func (p *Provider) MaxFileSize() int64 {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.cur.MaxFileSizeBytes
}

// clone returns a deep copy so callers can't mutate the Provider's slices.
func (s Settings) clone() Settings {
	out := s
	out.PasteExpiryOptions = append([]ExpiryOption(nil), s.PasteExpiryOptions...)
	out.FileExpiryOptions = append([]ExpiryOption(nil), s.FileExpiryOptions...)
	return out
}

// Store persists settings. Load returns (nil, nil) when no settings have been
// saved yet (first run).
type Store interface {
	Load(ctx context.Context) (*Settings, error)
	Save(ctx context.Context, s Settings) error
}

// Manager ties a Provider to a Store, validating and applying updates.
type Manager struct {
	provider *Provider
	store    Store
}

// NewManager creates a Manager backed by the given provider and store.
func NewManager(provider *Provider, store Store) *Manager {
	return &Manager{provider: provider, store: store}
}

// Get returns the current settings.
func (m *Manager) Get() Settings {
	return m.provider.Get()
}

// Update validates, persists, then applies the new settings. On a persistence
// failure the in-memory settings are left unchanged.
func (m *Manager) Update(ctx context.Context, s Settings) error {
	if err := s.Validate(); err != nil {
		return err
	}
	if m.store != nil {
		if err := m.store.Save(ctx, s); err != nil {
			return err
		}
	}
	m.provider.Set(s)
	return nil
}
