// Package paste provides the paste service interface and domain types.
package paste

import (
	"context"
	"encoding/json"
	"io"
	"time"

	"github.com/google/uuid"
)

// Visibility represents the access level of a paste or file.
type Visibility string

const (
	VisibilityPublic            Visibility = "public"
	VisibilityUnlisted          Visibility = "unlisted"
	VisibilityPasswordProtected Visibility = "password_protected"
)

// Paste represents a stored text paste.
type Paste struct {
	ID           uuid.UUID
	Slug         string
	Title        string
	Content      string
	Language     string
	Visibility   Visibility
	PasswordHash string
	ExpiresAt    *time.Time
	CreatedAt    time.Time
	Views        int
}


// PasteSummary is a lightweight representation of a paste for listing.
type PasteSummary struct {
	Slug      string     `json:"slug"`
	Title     string     `json:"title"`
	Language  string     `json:"language"`
	CreatedAt time.Time  `json:"created_at"`
	ExpiresAt *time.Time `json:"expires_at"`
}

// FileSummary is a lightweight representation of a file for listing.
type FileSummary struct {
	Slug      string     `json:"slug"`
	Filename  string     `json:"filename"`
	MIMEType  string     `json:"mime_type"`
	SizeBytes int64      `json:"size_bytes"`
	CreatedAt time.Time  `json:"created_at"`
	ExpiresAt *time.Time `json:"expires_at"`
}

// CreatePasteRequest contains the data needed to create a new paste.
type CreatePasteRequest struct {
	Content    string
	Language   string
	Title      string
	Visibility Visibility
	Password   string        // required if Visibility == VisibilityPasswordProtected
	ExpiresIn  time.Duration // 0 = use default (24h)
	CustomSlug string        // optional; if empty a random slug is generated
}

// FileRecord represents a stored file's metadata.
type FileRecord struct {
	ID           uuid.UUID
	Slug         string
	Filename     string
	MIMEType     string
	SizeBytes    int64
	StorageKey   string
	Visibility   Visibility
	PasswordHash string
	ExpiresAt    *time.Time
	CreatedAt    time.Time
	Downloads    int
}


// UploadFileRequest contains the data needed to upload a new file.
type UploadFileRequest struct {
	Filename   string
	MIMEType   string
	Size       int64
	Reader     io.Reader
	Visibility Visibility
	Password   string
	ExpiresIn  time.Duration
}

// RegisterFileRequest contains the metadata needed to register a direct-uploaded file.
type RegisterFileRequest struct {
	Slug       string
	Filename   string
	MIMEType   string
	Size       int64
	StorageKey string
	Visibility Visibility
	Password   string
	ExpiresIn  time.Duration
}

// ExpiryOption represents a selectable expiry duration.
type ExpiryOption struct {
	Label    string
	Duration time.Duration
}

// MarshalJSON emits ExpiryOption as { "label": "...", "duration": <minutes> }
// so the frontend receives duration in minutes (matching the `expires_in` form field).
func (e ExpiryOption) MarshalJSON() ([]byte, error) {
	type wire struct {
		Label    string `json:"label"`
		Duration int64  `json:"duration"`
	}
	return json.Marshal(wire{
		Label:    e.Label,
		Duration: int64(e.Duration.Minutes()),
	})
}

// ExpiryOptions defines the available expiry durations for pastes (includes "Selamanya").
var ExpiryOptions = []ExpiryOption{
	{Label: "1 Jam", Duration: 1 * time.Hour},
	{Label: "6 Jam", Duration: 6 * time.Hour},
	{Label: "24 Jam", Duration: 24 * time.Hour},
	{Label: "7 Hari", Duration: 7 * 24 * time.Hour},
	{Label: "30 Hari", Duration: 30 * 24 * time.Hour},
	{Label: "Selamanya", Duration: 0},
}

// FileExpiryOptions defines the available expiry durations for file uploads (no "Selamanya").
var FileExpiryOptions = []ExpiryOption{
	{Label: "1 Jam", Duration: 1 * time.Hour},
	{Label: "6 Jam", Duration: 6 * time.Hour},
	{Label: "24 Jam", Duration: 24 * time.Hour},
	{Label: "7 Hari", Duration: 7 * 24 * time.Hour},
	{Label: "30 Hari", Duration: 30 * 24 * time.Hour},
}

// DefaultExpiryDuration is the default expiry when none is specified.
const DefaultExpiryDuration = 24 * time.Hour

// PasteService defines the interface for paste operations.
type PasteService interface {
	Create(ctx context.Context, req CreatePasteRequest) (*Paste, error)
	GetBySlug(ctx context.Context, slug string) (*Paste, error)
	ValidatePassword(ctx context.Context, slug, password string) (bool, error)
	ListPublicRecent(ctx context.Context, limit int) ([]*PasteSummary, error)
	IncrementViews(ctx context.Context, slug string) error
}
