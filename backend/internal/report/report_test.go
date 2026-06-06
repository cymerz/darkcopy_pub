package report

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

// stubRepo is an in-memory Repository for testing the service.
type stubRepo struct {
	inserted    *Report
	insertErr   error
	updated     bool
	updateErr   error
	deleted     bool
	deleteErr   error
	pendingN    int
	lastStatus  Status
	lastReview  *time.Time
}

func (s *stubRepo) Insert(_ context.Context, r *Report) error {
	if s.insertErr != nil {
		return s.insertErr
	}
	s.inserted = r
	return nil
}
func (s *stubRepo) List(_ context.Context, _ string, _, _ int) ([]*Report, error) {
	return nil, nil
}
func (s *stubRepo) UpdateStatus(_ context.Context, _ uuid.UUID, status Status, reviewedAt *time.Time) (bool, error) {
	s.lastStatus = status
	s.lastReview = reviewedAt
	return s.updated, s.updateErr
}
func (s *stubRepo) DeleteByID(_ context.Context, _ uuid.UUID) (bool, error) {
	return s.deleted, s.deleteErr
}
func (s *stubRepo) CountByStatus(_ context.Context, _ string) (int, error) {
	return s.pendingN, nil
}

func TestCreate_Valid(t *testing.T) {
	repo := &stubRepo{}
	svc := NewService(repo)

	rep, err := svc.Create(context.Background(), CreateReportRequest{
		ResourceType: ResourcePaste,
		Slug:         "abc123",
		Reason:       "spam",
		Details:      "  lots of spam  ",
		ReporterIP:   "1.2.3.4",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rep.Status != StatusPending {
		t.Errorf("expected pending status, got %s", rep.Status)
	}
	if repo.inserted == nil || repo.inserted.Details != "lots of spam" {
		t.Errorf("expected details trimmed and persisted, got %q", repo.inserted.Details)
	}
}

func TestCreate_InvalidResource(t *testing.T) {
	svc := NewService(&stubRepo{})
	_, err := svc.Create(context.Background(), CreateReportRequest{
		ResourceType: "comment",
		Slug:         "abc123",
		Reason:       "spam",
	})
	if err != ErrInvalidResource {
		t.Errorf("expected ErrInvalidResource, got %v", err)
	}
}

func TestCreate_InvalidReason(t *testing.T) {
	svc := NewService(&stubRepo{})
	_, err := svc.Create(context.Background(), CreateReportRequest{
		ResourceType: ResourceFile,
		Slug:         "abc123",
		Reason:       "because",
	})
	if err != ErrInvalidReason {
		t.Errorf("expected ErrInvalidReason, got %v", err)
	}
}

func TestCreate_InvalidSlug(t *testing.T) {
	svc := NewService(&stubRepo{})
	_, err := svc.Create(context.Background(), CreateReportRequest{
		ResourceType: ResourcePaste,
		Slug:         "",
		Reason:       "spam",
	})
	if err != ErrInvalidSlug {
		t.Errorf("expected ErrInvalidSlug, got %v", err)
	}
}

func TestCreate_DetailsCapped(t *testing.T) {
	repo := &stubRepo{}
	svc := NewService(repo)
	long := make([]byte, MaxDetailsLen+500)
	for i := range long {
		long[i] = 'x'
	}
	_, err := svc.Create(context.Background(), CreateReportRequest{
		ResourceType: ResourcePaste,
		Slug:         "abc123",
		Reason:       "other",
		Details:      string(long),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repo.inserted.Details) != MaxDetailsLen {
		t.Errorf("expected details capped to %d, got %d", MaxDetailsLen, len(repo.inserted.Details))
	}
}

func TestUpdateStatus_SetsReviewedAt(t *testing.T) {
	repo := &stubRepo{updated: true}
	svc := NewService(repo)
	if err := svc.UpdateStatus(context.Background(), uuid.New(), StatusReviewed); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.lastStatus != StatusReviewed {
		t.Errorf("expected reviewed status, got %s", repo.lastStatus)
	}
	if repo.lastReview == nil {
		t.Error("expected reviewed_at to be set for reviewed status")
	}
}

func TestUpdateStatus_PendingClearsReviewedAt(t *testing.T) {
	repo := &stubRepo{updated: true}
	svc := NewService(repo)
	if err := svc.UpdateStatus(context.Background(), uuid.New(), StatusPending); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.lastReview != nil {
		t.Error("expected reviewed_at to be nil when reverting to pending")
	}
}

func TestUpdateStatus_NotFound(t *testing.T) {
	repo := &stubRepo{updated: false}
	svc := NewService(repo)
	if err := svc.UpdateStatus(context.Background(), uuid.New(), StatusDismissed); err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestUpdateStatus_Invalid(t *testing.T) {
	svc := NewService(&stubRepo{})
	if err := svc.UpdateStatus(context.Background(), uuid.New(), Status("bogus")); err != ErrInvalidStatus {
		t.Errorf("expected ErrInvalidStatus, got %v", err)
	}
}

func TestDelete_NotFound(t *testing.T) {
	svc := NewService(&stubRepo{deleted: false})
	if err := svc.Delete(context.Background(), uuid.New()); err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}
