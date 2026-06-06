// Package report provides abuse/content reporting: visitors can flag a paste or
// file, and administrators review the resulting reports. Reports are persisted
// and surfaced in the admin panel.
package report

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ResourceType identifies what a report targets.
type ResourceType string

const (
	ResourcePaste ResourceType = "paste"
	ResourceFile  ResourceType = "file"
)

// Status is the review state of a report.
type Status string

const (
	StatusPending   Status = "pending"
	StatusReviewed  Status = "reviewed"
	StatusDismissed Status = "dismissed"
)

// Maximum lengths for free-text fields to bound storage and abuse.
const (
	MaxReasonLen  = 32
	MaxDetailsLen = 1000
)

// AllowedReasons is the set of canonical report reasons offered to users. Any
// reason outside this set is rejected.
var AllowedReasons = []string{
	"spam",
	"illegal",
	"malware",
	"copyright",
	"personal_info",
	"other",
}

// Errors returned by the report service.
var (
	ErrInvalidResource = errors.New("Jenis konten laporan tidak valid")
	ErrInvalidReason   = errors.New("Alasan laporan tidak valid")
	ErrInvalidSlug     = errors.New("Slug tidak valid")
	ErrNotFound        = errors.New("laporan tidak ditemukan")
	ErrInvalidStatus   = errors.New("Status laporan tidak valid")
)

// Report is a single abuse/content report.
type Report struct {
	ID           uuid.UUID    `json:"id"`
	ResourceType ResourceType `json:"resource_type"`
	Slug         string       `json:"slug"`
	Reason       string       `json:"reason"`
	Details      string       `json:"details"`
	ReporterIP   string       `json:"reporter_ip"`
	Status       Status       `json:"status"`
	CreatedAt    time.Time    `json:"created_at"`
	ReviewedAt   *time.Time   `json:"reviewed_at"`
}

// CreateReportRequest contains the data needed to file a report.
type CreateReportRequest struct {
	ResourceType ResourceType
	Slug         string
	Reason       string
	Details      string
	ReporterIP   string
}

// Repository defines persistence operations for reports.
type Repository interface {
	Insert(ctx context.Context, r *Report) error
	List(ctx context.Context, status string, limit, offset int) ([]*Report, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status Status, reviewedAt *time.Time) (bool, error)
	DeleteByID(ctx context.Context, id uuid.UUID) (bool, error)
	CountByStatus(ctx context.Context, status string) (int, error)
}

// Service implements the report business logic.
type Service struct {
	repo Repository
	now  func() time.Time
}

// NewService creates a new report Service.
func NewService(repo Repository) *Service {
	return &Service{repo: repo, now: time.Now}
}

// validReason reports whether reason is one of the allowed canonical reasons.
func validReason(reason string) bool {
	for _, r := range AllowedReasons {
		if r == reason {
			return true
		}
	}
	return false
}

// Create validates and persists a new report. Details are trimmed and capped.
func (s *Service) Create(ctx context.Context, req CreateReportRequest) (*Report, error) {
	if req.ResourceType != ResourcePaste && req.ResourceType != ResourceFile {
		return nil, ErrInvalidResource
	}
	slug := strings.TrimSpace(req.Slug)
	if slug == "" || len(slug) > 12 {
		return nil, ErrInvalidSlug
	}
	if !validReason(req.Reason) {
		return nil, ErrInvalidReason
	}

	details := strings.TrimSpace(req.Details)
	if len(details) > MaxDetailsLen {
		details = details[:MaxDetailsLen]
	}

	rep := &Report{
		ID:           uuid.New(),
		ResourceType: req.ResourceType,
		Slug:         slug,
		Reason:       req.Reason,
		Details:      details,
		ReporterIP:   req.ReporterIP,
		Status:       StatusPending,
		CreatedAt:    s.now(),
	}

	if err := s.repo.Insert(ctx, rep); err != nil {
		return nil, err
	}
	return rep, nil
}

// List returns reports filtered by status ("" or "all" returns every status).
func (s *Service) List(ctx context.Context, status string, limit, offset int) ([]*Report, error) {
	switch Status(status) {
	case StatusPending, StatusReviewed, StatusDismissed:
		// valid explicit filter
	default:
		status = "" // treat anything else as "all"
	}
	return s.repo.List(ctx, status, limit, offset)
}

// UpdateStatus changes a report's review status. Returns ErrNotFound when no
// report matches the id, and ErrInvalidStatus for an unknown target status.
func (s *Service) UpdateStatus(ctx context.Context, id uuid.UUID, status Status) error {
	if status != StatusReviewed && status != StatusDismissed && status != StatusPending {
		return ErrInvalidStatus
	}

	var reviewedAt *time.Time
	if status != StatusPending {
		t := s.now()
		reviewedAt = &t
	}

	updated, err := s.repo.UpdateStatus(ctx, id, status, reviewedAt)
	if err != nil {
		return err
	}
	if !updated {
		return ErrNotFound
	}
	return nil
}

// Delete removes a report. Returns ErrNotFound when no report matches.
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	deleted, err := s.repo.DeleteByID(ctx, id)
	if err != nil {
		return err
	}
	if !deleted {
		return ErrNotFound
	}
	return nil
}

// CountPending returns the number of reports awaiting review.
func (s *Service) CountPending(ctx context.Context) (int, error) {
	return s.repo.CountByStatus(ctx, string(StatusPending))
}
