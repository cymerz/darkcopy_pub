package paste

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/gthbn/pastebin/internal/access"
	"golang.org/x/crypto/bcrypt"
)

// mockURLGenerator is a test double for urlgen.URLGenerator.
type mockURLGenerator struct {
	slug string
	err  error
}

func (m *mockURLGenerator) GenerateSlug(ctx context.Context) (string, error) {
	return m.slug, m.err
}

// mockRepository is a test double for PasteRepository.
type mockRepository struct {
	insertedPaste *Paste
	insertErr     error

	getBySlugPaste *Paste
	getBySlugErr   error

	listPublicRecentResult []*PasteSummary
	listPublicRecentErr    error
}

func (m *mockRepository) InsertPaste(ctx context.Context, paste *Paste) error {
	m.insertedPaste = paste
	return m.insertErr
}

func (m *mockRepository) GetBySlug(ctx context.Context, slug string) (*Paste, error) {
	if m.getBySlugErr != nil {
		return nil, m.getBySlugErr
	}
	return m.getBySlugPaste, nil
}

func (m *mockRepository) ListPublicRecent(ctx context.Context, limit int) ([]*PasteSummary, error) {
	if m.listPublicRecentErr != nil {
		return nil, m.listPublicRecentErr
	}
	return m.listPublicRecentResult, nil
}

// mockAccessController is a test double for access.AccessController.
type mockAccessController struct {
	checkResult access.AccessResult
	checkErr    error
}

func (m *mockAccessController) CheckAccess(ctx context.Context, passwordHash string, password string) (access.AccessResult, error) {
	return m.checkResult, m.checkErr
}

func (m *mockAccessController) RecordFailedAttempt(ctx context.Context, ip string, resourceID string) error {
	return nil
}

func (m *mockAccessController) IsRateLimited(ctx context.Context, ip string, resourceID string) (bool, error) {
	return false, nil
}


func TestCreate_ValidPublicPaste(t *testing.T) {
	repo := &mockRepository{}
	urlGen := &mockURLGenerator{slug: "abc12345"}
	accessCtl := &mockAccessController{}
	svc := NewService(repo, urlGen, accessCtl)
	fixedNow := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return fixedNow }

	req := CreatePasteRequest{
		Content:    "Hello, World!",
		Language:   "plaintext",
		Title:      "Test Paste",
		Visibility: VisibilityPublic,
		ExpiresIn:  1 * time.Hour,
	}

	paste, err := svc.Create(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if paste.Slug != "abc12345" {
		t.Errorf("expected slug abc12345, got %s", paste.Slug)
	}
	if paste.Content != "Hello, World!" {
		t.Errorf("expected content 'Hello, World!', got %s", paste.Content)
	}
	if paste.Title != "Test Paste" {
		t.Errorf("expected title 'Test Paste', got %s", paste.Title)
	}
	if paste.Language != "plaintext" {
		t.Errorf("expected language 'plaintext', got %s", paste.Language)
	}
	if paste.Visibility != VisibilityPublic {
		t.Errorf("expected visibility public, got %s", paste.Visibility)
	}
	if paste.PasswordHash != "" {
		t.Errorf("expected empty password hash, got %s", paste.PasswordHash)
	}
	if paste.CreatedAt != fixedNow {
		t.Errorf("expected created_at %v, got %v", fixedNow, paste.CreatedAt)
	}

	expectedExpiry := fixedNow.Add(1 * time.Hour)
	if paste.ExpiresAt == nil || !paste.ExpiresAt.Equal(expectedExpiry) {
		t.Errorf("expected expires_at %v, got %v", expectedExpiry, paste.ExpiresAt)
	}

	if repo.insertedPaste == nil {
		t.Fatal("expected paste to be inserted into repository")
	}
}

func TestCreate_EmptyContent(t *testing.T) {
	repo := &mockRepository{}
	urlGen := &mockURLGenerator{slug: "abc12345"}
	accessCtl := &mockAccessController{}
	svc := NewService(repo, urlGen, accessCtl)

	req := CreatePasteRequest{
		Content:    "",
		Visibility: VisibilityPublic,
	}

	_, err := svc.Create(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for empty content")
	}
	if err != ErrEmptyContent {
		t.Errorf("expected ErrEmptyContent, got %v", err)
	}
	if repo.insertedPaste != nil {
		t.Error("expected no paste to be inserted")
	}
}

func TestCreate_WhitespaceOnlyContent(t *testing.T) {
	repo := &mockRepository{}
	urlGen := &mockURLGenerator{slug: "abc12345"}
	accessCtl := &mockAccessController{}
	svc := NewService(repo, urlGen, accessCtl)

	req := CreatePasteRequest{
		Content:    "   \t\n  ",
		Visibility: VisibilityPublic,
	}

	_, err := svc.Create(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for whitespace-only content")
	}
	if err != ErrEmptyContent {
		t.Errorf("expected ErrEmptyContent, got %v", err)
	}
}

func TestCreate_ContentTooLarge(t *testing.T) {
	repo := &mockRepository{}
	urlGen := &mockURLGenerator{slug: "abc12345"}
	accessCtl := &mockAccessController{}
	svc := NewService(repo, urlGen, accessCtl)

	largeContent := strings.Repeat("x", MaxContentSize+1)
	req := CreatePasteRequest{
		Content:    largeContent,
		Visibility: VisibilityPublic,
	}

	_, err := svc.Create(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for content too large")
	}
	if err != ErrContentTooLarge {
		t.Errorf("expected ErrContentTooLarge, got %v", err)
	}
}

func TestCreate_PasswordProtected_Valid(t *testing.T) {
	repo := &mockRepository{}
	urlGen := &mockURLGenerator{slug: "abc12345"}
	accessCtl := &mockAccessController{}
	svc := NewService(repo, urlGen, accessCtl)

	req := CreatePasteRequest{
		Content:    "secret content",
		Visibility: VisibilityPasswordProtected,
		Password:   "mypassword",
		ExpiresIn:  1 * time.Hour,
	}

	paste, err := svc.Create(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if paste.PasswordHash == "" {
		t.Fatal("expected password hash to be set")
	}

	// Verify bcrypt hash is valid and matches the password.
	err = bcrypt.CompareHashAndPassword([]byte(paste.PasswordHash), []byte("mypassword"))
	if err != nil {
		t.Errorf("password hash does not match: %v", err)
	}

	// Verify bcrypt cost factor is 10.
	cost, err := bcrypt.Cost([]byte(paste.PasswordHash))
	if err != nil {
		t.Fatalf("failed to get bcrypt cost: %v", err)
	}
	if cost != 10 {
		t.Errorf("expected bcrypt cost 10, got %d", cost)
	}
}

func TestCreate_PasswordProtected_MissingPassword(t *testing.T) {
	repo := &mockRepository{}
	urlGen := &mockURLGenerator{slug: "abc12345"}
	accessCtl := &mockAccessController{}
	svc := NewService(repo, urlGen, accessCtl)

	req := CreatePasteRequest{
		Content:    "secret content",
		Visibility: VisibilityPasswordProtected,
		Password:   "",
	}

	_, err := svc.Create(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for missing password")
	}
	if err != ErrPasswordRequired {
		t.Errorf("expected ErrPasswordRequired, got %v", err)
	}
}

func TestCreate_PasswordProtected_WhitespacePassword(t *testing.T) {
	repo := &mockRepository{}
	urlGen := &mockURLGenerator{slug: "abc12345"}
	accessCtl := &mockAccessController{}
	svc := NewService(repo, urlGen, accessCtl)

	req := CreatePasteRequest{
		Content:    "secret content",
		Visibility: VisibilityPasswordProtected,
		Password:   "   ",
	}

	_, err := svc.Create(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for whitespace-only password")
	}
	if err != ErrPasswordRequired {
		t.Errorf("expected ErrPasswordRequired, got %v", err)
	}
}

func TestCreate_DefaultExpiry(t *testing.T) {
	repo := &mockRepository{}
	urlGen := &mockURLGenerator{slug: "abc12345"}
	accessCtl := &mockAccessController{}
	svc := NewService(repo, urlGen, accessCtl)
	fixedNow := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return fixedNow }

	req := CreatePasteRequest{
		Content:    "some content",
		Visibility: VisibilityPublic,
		ExpiresIn:  0, // not set → should default to 24 hours
	}

	paste, err := svc.Create(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedExpiry := fixedNow.Add(24 * time.Hour)
	if paste.ExpiresAt == nil {
		t.Fatal("expected expires_at to be set (default 24h)")
	}
	if !paste.ExpiresAt.Equal(expectedExpiry) {
		t.Errorf("expected expires_at %v, got %v", expectedExpiry, *paste.ExpiresAt)
	}
}

func TestCreate_NeverExpires(t *testing.T) {
	repo := &mockRepository{}
	urlGen := &mockURLGenerator{slug: "abc12345"}
	accessCtl := &mockAccessController{}
	svc := NewService(repo, urlGen, accessCtl)

	req := CreatePasteRequest{
		Content:    "permanent content",
		Visibility: VisibilityPublic,
		ExpiresIn:  NeverExpires, // sentinel for "never expires"
	}

	paste, err := svc.Create(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if paste.ExpiresAt != nil {
		t.Errorf("expected nil expires_at for never-expires, got %v", paste.ExpiresAt)
	}
}

func TestCreate_UnlistedVisibility(t *testing.T) {
	repo := &mockRepository{}
	urlGen := &mockURLGenerator{slug: "xyz98765"}
	accessCtl := &mockAccessController{}
	svc := NewService(repo, urlGen, accessCtl)

	req := CreatePasteRequest{
		Content:    "unlisted content",
		Visibility: VisibilityUnlisted,
		ExpiresIn:  6 * time.Hour,
	}

	paste, err := svc.Create(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if paste.Visibility != VisibilityUnlisted {
		t.Errorf("expected visibility unlisted, got %s", paste.Visibility)
	}
	if paste.PasswordHash != "" {
		t.Errorf("expected empty password hash for unlisted, got %s", paste.PasswordHash)
	}
}

func TestCreate_SlugGenerationError(t *testing.T) {
	repo := &mockRepository{}
	urlGen := &mockURLGenerator{slug: "", err: errors.New("slug generation failed")}
	accessCtl := &mockAccessController{}
	svc := NewService(repo, urlGen, accessCtl)

	req := CreatePasteRequest{
		Content:    "some content",
		Visibility: VisibilityPublic,
		ExpiresIn:  1 * time.Hour,
	}

	_, err := svc.Create(context.Background(), req)
	if err == nil {
		t.Fatal("expected error when slug generation fails")
	}
}

func TestCreate_RepositoryError(t *testing.T) {
	repo := &mockRepository{insertErr: errors.New("db error")}
	urlGen := &mockURLGenerator{slug: "abc12345"}
	accessCtl := &mockAccessController{}
	svc := NewService(repo, urlGen, accessCtl)

	req := CreatePasteRequest{
		Content:    "some content",
		Visibility: VisibilityPublic,
		ExpiresIn:  1 * time.Hour,
	}

	_, err := svc.Create(context.Background(), req)
	if err == nil {
		t.Fatal("expected error when repository insert fails")
	}
}

func TestGetBySlug_ValidPaste(t *testing.T) {
	fixedNow := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	futureExpiry := fixedNow.Add(1 * time.Hour)

	repo := &mockRepository{
		getBySlugPaste: &Paste{
			Slug:       "abc12345",
			Title:      "Test Paste",
			Content:    "Hello, World!",
			Language:   "plaintext",
			Visibility: VisibilityPublic,
			ExpiresAt:  &futureExpiry,
			CreatedAt:  fixedNow,
		},
	}
	urlGen := &mockURLGenerator{slug: "abc12345"}
	accessCtl := &mockAccessController{}
	svc := NewService(repo, urlGen, accessCtl)
	svc.now = func() time.Time { return fixedNow }

	paste, err := svc.GetBySlug(context.Background(), "abc12345")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if paste.Slug != "abc12345" {
		t.Errorf("expected slug abc12345, got %s", paste.Slug)
	}
	if paste.Content != "Hello, World!" {
		t.Errorf("expected content 'Hello, World!', got %s", paste.Content)
	}
}

func TestGetBySlug_NotFound(t *testing.T) {
	repo := &mockRepository{
		getBySlugErr: errors.New("not found"),
	}
	urlGen := &mockURLGenerator{slug: "abc12345"}
	accessCtl := &mockAccessController{}
	svc := NewService(repo, urlGen, accessCtl)

	_, err := svc.GetBySlug(context.Background(), "nonexist1")
	if err == nil {
		t.Fatal("expected error for non-existent slug")
	}
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestGetBySlug_Expired(t *testing.T) {
	fixedNow := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	pastExpiry := fixedNow.Add(-1 * time.Hour) // expired 1 hour ago

	repo := &mockRepository{
		getBySlugPaste: &Paste{
			Slug:       "abc12345",
			Title:      "Expired Paste",
			Content:    "old content",
			Language:   "plaintext",
			Visibility: VisibilityPublic,
			ExpiresAt:  &pastExpiry,
			CreatedAt:  fixedNow.Add(-2 * time.Hour),
		},
	}
	urlGen := &mockURLGenerator{slug: "abc12345"}
	accessCtl := &mockAccessController{}
	svc := NewService(repo, urlGen, accessCtl)
	svc.now = func() time.Time { return fixedNow }

	_, err := svc.GetBySlug(context.Background(), "abc12345")
	if err == nil {
		t.Fatal("expected error for expired paste")
	}
	if err != ErrExpired {
		t.Errorf("expected ErrExpired, got %v", err)
	}
}

func TestListPublicRecent_ReturnsResults(t *testing.T) {
	fixedNow := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	repo := &mockRepository{
		listPublicRecentResult: []*PasteSummary{
			{
				Slug:      "paste001",
				Title:     "First Paste",
				Language:  "go",
				CreatedAt: fixedNow,
			},
			{
				Slug:      "paste002",
				Title:     "Second Paste",
				Language:  "python",
				CreatedAt: fixedNow.Add(-1 * time.Hour),
			},
		},
	}
	urlGen := &mockURLGenerator{slug: "abc12345"}
	accessCtl := &mockAccessController{}
	svc := NewService(repo, urlGen, accessCtl)

	results, err := svc.ListPublicRecent(context.Background(), 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].Slug != "paste001" {
		t.Errorf("expected first slug paste001, got %s", results[0].Slug)
	}
	if results[1].Slug != "paste002" {
		t.Errorf("expected second slug paste002, got %s", results[1].Slug)
	}
}

// --- ValidatePassword tests ---

func TestValidatePassword_CorrectPassword(t *testing.T) {
	fixedNow := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	futureExpiry := fixedNow.Add(1 * time.Hour)

	hash, _ := access.HashPassword("secret123")
	repo := &mockRepository{
		getBySlugPaste: &Paste{
			Slug:         "abc12345",
			Visibility:   VisibilityPasswordProtected,
			PasswordHash: hash,
			ExpiresAt:    &futureExpiry,
			CreatedAt:    fixedNow,
		},
	}
	urlGen := &mockURLGenerator{}
	accessCtl := &mockAccessController{checkResult: access.AccessGranted}
	svc := NewService(repo, urlGen, accessCtl)
	svc.now = func() time.Time { return fixedNow }

	granted, err := svc.ValidatePassword(context.Background(), "abc12345", "secret123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !granted {
		t.Error("expected access to be granted with correct password")
	}
}

func TestValidatePassword_WrongPassword(t *testing.T) {
	fixedNow := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	futureExpiry := fixedNow.Add(1 * time.Hour)

	hash, _ := access.HashPassword("secret123")
	repo := &mockRepository{
		getBySlugPaste: &Paste{
			Slug:         "abc12345",
			Visibility:   VisibilityPasswordProtected,
			PasswordHash: hash,
			ExpiresAt:    &futureExpiry,
			CreatedAt:    fixedNow,
		},
	}
	urlGen := &mockURLGenerator{}
	accessCtl := &mockAccessController{checkResult: access.AccessDenied}
	svc := NewService(repo, urlGen, accessCtl)
	svc.now = func() time.Time { return fixedNow }

	granted, err := svc.ValidatePassword(context.Background(), "abc12345", "wrongpass")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if granted {
		t.Error("expected access to be denied with wrong password")
	}
}

func TestValidatePassword_NotFound(t *testing.T) {
	repo := &mockRepository{
		getBySlugErr: errors.New("not found"),
	}
	urlGen := &mockURLGenerator{}
	accessCtl := &mockAccessController{}
	svc := NewService(repo, urlGen, accessCtl)

	granted, err := svc.ValidatePassword(context.Background(), "nonexist1", "anypass")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
	if granted {
		t.Error("expected access to be denied for non-existent paste")
	}
}

func TestValidatePassword_Expired(t *testing.T) {
	fixedNow := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	pastExpiry := fixedNow.Add(-1 * time.Hour)

	hash, _ := access.HashPassword("secret123")
	repo := &mockRepository{
		getBySlugPaste: &Paste{
			Slug:         "abc12345",
			Visibility:   VisibilityPasswordProtected,
			PasswordHash: hash,
			ExpiresAt:    &pastExpiry,
			CreatedAt:    fixedNow.Add(-2 * time.Hour),
		},
	}
	urlGen := &mockURLGenerator{}
	accessCtl := &mockAccessController{}
	svc := NewService(repo, urlGen, accessCtl)
	svc.now = func() time.Time { return fixedNow }

	granted, err := svc.ValidatePassword(context.Background(), "abc12345", "secret123")
	if err != ErrExpired {
		t.Errorf("expected ErrExpired, got %v", err)
	}
	if granted {
		t.Error("expected access to be denied for expired paste")
	}
}

func TestValidatePassword_PublicPaste_NoPassword(t *testing.T) {
	fixedNow := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	futureExpiry := fixedNow.Add(1 * time.Hour)

	repo := &mockRepository{
		getBySlugPaste: &Paste{
			Slug:         "abc12345",
			Visibility:   VisibilityPublic,
			PasswordHash: "", // no password
			ExpiresAt:    &futureExpiry,
			CreatedAt:    fixedNow,
		},
	}
	urlGen := &mockURLGenerator{}
	accessCtl := &mockAccessController{}
	svc := NewService(repo, urlGen, accessCtl)
	svc.now = func() time.Time { return fixedNow }

	granted, err := svc.ValidatePassword(context.Background(), "abc12345", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !granted {
		t.Error("expected access to be granted for public paste without password")
	}
}
