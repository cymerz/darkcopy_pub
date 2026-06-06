package file

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/gthbn/pastebin/internal/paste"
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

// mockFileRepository is a test double for FileRepository.
type mockFileRepository struct {
	insertedFile *paste.FileRecord
	insertErr    error
	getBySlugRec *paste.FileRecord
	getBySlugErr error
}

func (m *mockFileRepository) InsertFile(ctx context.Context, file *paste.FileRecord) error {
	m.insertedFile = file
	return m.insertErr
}

func (m *mockFileRepository) GetBySlug(ctx context.Context, slug string) (*paste.FileRecord, error) {
	if m.getBySlugErr != nil {
		return nil, m.getBySlugErr
	}
	return m.getBySlugRec, nil
}

func (m *mockFileRepository) ListPublicRecent(ctx context.Context, limit int) ([]*paste.FileSummary, error) {
	return nil, nil
}

func (m *mockFileRepository) IncrementDownloads(ctx context.Context, slug string) error {
	return nil
}


// mockFileStorage is a test double for FileStorage.
type mockFileStorage struct {
	savedKey    string
	savedData   []byte
	saveErr     error
	openReader  io.ReadCloser
	openErr     error
	deletedKey  string
	deleteErr   error
}

func (m *mockFileStorage) Save(ctx context.Context, storageKey string, reader io.Reader) error {
	m.savedKey = storageKey
	if m.saveErr != nil {
		return m.saveErr
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	m.savedData = data
	return nil
}

func (m *mockFileStorage) Open(ctx context.Context, storageKey string) (io.ReadCloser, error) {
	return m.openReader, m.openErr
}

func (m *mockFileStorage) Delete(ctx context.Context, storageKey string) error {
	m.deletedKey = storageKey
	return m.deleteErr
}

func TestUpload_ValidPublicFile(t *testing.T) {
	repo := &mockFileRepository{}
	storage := &mockFileStorage{}
	urlGen := &mockURLGenerator{slug: "abc12345"}
	svc := NewService(repo, storage, urlGen)
	fixedNow := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return fixedNow }

	content := []byte("file content here")
	req := paste.UploadFileRequest{
		Filename:   "test.txt",
		MIMEType:   "text/plain",
		Size:       int64(len(content)),
		Reader:     bytes.NewReader(content),
		Visibility: paste.VisibilityPublic,
		ExpiresIn:  1 * time.Hour,
	}

	record, err := svc.Upload(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if record.Slug != "abc12345" {
		t.Errorf("expected slug abc12345, got %s", record.Slug)
	}
	if record.Filename != "test.txt" {
		t.Errorf("expected filename test.txt, got %s", record.Filename)
	}
	if record.MIMEType != "text/plain" {
		t.Errorf("expected mime type text/plain, got %s", record.MIMEType)
	}
	if record.SizeBytes != int64(len(content)) {
		t.Errorf("expected size %d, got %d", len(content), record.SizeBytes)
	}
	if record.StorageKey != "uploads/abc12345/test.txt" {
		t.Errorf("expected storage key uploads/abc12345/test.txt, got %s", record.StorageKey)
	}
	if record.Visibility != paste.VisibilityPublic {
		t.Errorf("expected visibility public, got %s", record.Visibility)
	}
	if record.PasswordHash != "" {
		t.Errorf("expected empty password hash, got %s", record.PasswordHash)
	}
	if record.CreatedAt != fixedNow {
		t.Errorf("expected created_at %v, got %v", fixedNow, record.CreatedAt)
	}

	expectedExpiry := fixedNow.Add(1 * time.Hour)
	if record.ExpiresAt == nil || !record.ExpiresAt.Equal(expectedExpiry) {
		t.Errorf("expected expires_at %v, got %v", expectedExpiry, record.ExpiresAt)
	}

	if repo.insertedFile == nil {
		t.Fatal("expected file to be inserted into repository")
	}
	if storage.savedKey != "uploads/abc12345/test.txt" {
		t.Errorf("expected storage save key uploads/abc12345/test.txt, got %s", storage.savedKey)
	}
	if !bytes.Equal(storage.savedData, content) {
		t.Errorf("expected saved data to match content")
	}
}

func TestUpload_FileTooLarge(t *testing.T) {
	repo := &mockFileRepository{}
	storage := &mockFileStorage{}
	urlGen := &mockURLGenerator{slug: "abc12345"}
	svc := NewService(repo, storage, urlGen)

	req := paste.UploadFileRequest{
		Filename:   "large.bin",
		MIMEType:   "application/octet-stream",
		Size:       MaxFileSize + 1,
		Reader:     bytes.NewReader([]byte("x")),
		Visibility: paste.VisibilityPublic,
	}

	_, err := svc.Upload(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for file too large")
	}
	if err != ErrFileTooLarge {
		t.Errorf("expected ErrFileTooLarge, got %v", err)
	}
	if repo.insertedFile != nil {
		t.Error("expected no file to be inserted")
	}
	if storage.savedKey != "" {
		t.Error("expected no file to be saved to storage")
	}
}

func TestUpload_PasswordProtected_MissingPassword(t *testing.T) {
	repo := &mockFileRepository{}
	storage := &mockFileStorage{}
	urlGen := &mockURLGenerator{slug: "abc12345"}
	svc := NewService(repo, storage, urlGen)

	req := paste.UploadFileRequest{
		Filename:   "secret.pdf",
		MIMEType:   "application/pdf",
		Size:       1024,
		Reader:     bytes.NewReader([]byte("data")),
		Visibility: paste.VisibilityPasswordProtected,
		Password:   "",
	}

	_, err := svc.Upload(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for missing password")
	}
	if err != ErrPasswordRequired {
		t.Errorf("expected ErrPasswordRequired, got %v", err)
	}
	if repo.insertedFile != nil {
		t.Error("expected no file to be inserted")
	}
}

func TestUpload_PasswordProtected_WhitespacePassword(t *testing.T) {
	repo := &mockFileRepository{}
	storage := &mockFileStorage{}
	urlGen := &mockURLGenerator{slug: "abc12345"}
	svc := NewService(repo, storage, urlGen)

	req := paste.UploadFileRequest{
		Filename:   "secret.pdf",
		MIMEType:   "application/pdf",
		Size:       1024,
		Reader:     bytes.NewReader([]byte("data")),
		Visibility: paste.VisibilityPasswordProtected,
		Password:   "   \t  ",
	}

	_, err := svc.Upload(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for whitespace-only password")
	}
	if err != ErrPasswordRequired {
		t.Errorf("expected ErrPasswordRequired, got %v", err)
	}
}

func TestUpload_PasswordProtected_Valid(t *testing.T) {
	repo := &mockFileRepository{}
	storage := &mockFileStorage{}
	urlGen := &mockURLGenerator{slug: "abc12345"}
	svc := NewService(repo, storage, urlGen)

	req := paste.UploadFileRequest{
		Filename:   "secret.pdf",
		MIMEType:   "application/pdf",
		Size:       1024,
		Reader:     bytes.NewReader([]byte("secret data")),
		Visibility: paste.VisibilityPasswordProtected,
		Password:   "mypassword",
		ExpiresIn:  1 * time.Hour,
	}

	record, err := svc.Upload(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if record.PasswordHash == "" {
		t.Fatal("expected password hash to be set")
	}

	// Verify bcrypt hash is valid and matches the password.
	err = bcrypt.CompareHashAndPassword([]byte(record.PasswordHash), []byte("mypassword"))
	if err != nil {
		t.Errorf("password hash does not match: %v", err)
	}

	// Verify bcrypt cost factor is 10.
	cost, err := bcrypt.Cost([]byte(record.PasswordHash))
	if err != nil {
		t.Fatalf("failed to get bcrypt cost: %v", err)
	}
	if cost != 10 {
		t.Errorf("expected bcrypt cost 10, got %d", cost)
	}
}

func TestUpload_DefaultExpiry(t *testing.T) {
	repo := &mockFileRepository{}
	storage := &mockFileStorage{}
	urlGen := &mockURLGenerator{slug: "abc12345"}
	svc := NewService(repo, storage, urlGen)
	fixedNow := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return fixedNow }

	req := paste.UploadFileRequest{
		Filename:   "doc.txt",
		MIMEType:   "text/plain",
		Size:       100,
		Reader:     bytes.NewReader([]byte("content")),
		Visibility: paste.VisibilityPublic,
		ExpiresIn:  0, // not set → should default to 24 hours
	}

	record, err := svc.Upload(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedExpiry := fixedNow.Add(24 * time.Hour)
	if record.ExpiresAt == nil {
		t.Fatal("expected expires_at to be set (default 24h)")
	}
	if !record.ExpiresAt.Equal(expectedExpiry) {
		t.Errorf("expected expires_at %v, got %v", expectedExpiry, *record.ExpiresAt)
	}
}

func TestUpload_NeverExpires(t *testing.T) {
	repo := &mockFileRepository{}
	storage := &mockFileStorage{}
	urlGen := &mockURLGenerator{slug: "abc12345"}
	svc := NewService(repo, storage, urlGen)

	req := paste.UploadFileRequest{
		Filename:   "permanent.bin",
		MIMEType:   "application/octet-stream",
		Size:       512,
		Reader:     bytes.NewReader([]byte("permanent")),
		Visibility: paste.VisibilityPublic,
		ExpiresIn:  NeverExpires, // sentinel for "never expires"
	}

	record, err := svc.Upload(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if record.ExpiresAt != nil {
		t.Errorf("expected nil expires_at for never-expires, got %v", record.ExpiresAt)
	}
}

func TestUpload_StorageError(t *testing.T) {
	repo := &mockFileRepository{}
	storage := &mockFileStorage{saveErr: errors.New("disk full")}
	urlGen := &mockURLGenerator{slug: "abc12345"}
	svc := NewService(repo, storage, urlGen)

	req := paste.UploadFileRequest{
		Filename:   "file.txt",
		MIMEType:   "text/plain",
		Size:       100,
		Reader:     bytes.NewReader([]byte("data")),
		Visibility: paste.VisibilityPublic,
		ExpiresIn:  1 * time.Hour,
	}

	_, err := svc.Upload(context.Background(), req)
	if err == nil {
		t.Fatal("expected error when storage save fails")
	}
	if repo.insertedFile != nil {
		t.Error("expected no file to be inserted when storage fails")
	}
}

func TestUpload_RepositoryError(t *testing.T) {
	repo := &mockFileRepository{insertErr: errors.New("db error")}
	storage := &mockFileStorage{}
	urlGen := &mockURLGenerator{slug: "abc12345"}
	svc := NewService(repo, storage, urlGen)

	req := paste.UploadFileRequest{
		Filename:   "file.txt",
		MIMEType:   "text/plain",
		Size:       100,
		Reader:     bytes.NewReader([]byte("data")),
		Visibility: paste.VisibilityPublic,
		ExpiresIn:  1 * time.Hour,
	}

	_, err := svc.Upload(context.Background(), req)
	if err == nil {
		t.Fatal("expected error when repository insert fails")
	}
}

func TestUpload_SlugGenerationError(t *testing.T) {
	repo := &mockFileRepository{}
	storage := &mockFileStorage{}
	urlGen := &mockURLGenerator{slug: "", err: errors.New("slug generation failed")}
	svc := NewService(repo, storage, urlGen)

	req := paste.UploadFileRequest{
		Filename:   "file.txt",
		MIMEType:   "text/plain",
		Size:       100,
		Reader:     bytes.NewReader([]byte("data")),
		Visibility: paste.VisibilityPublic,
		ExpiresIn:  1 * time.Hour,
	}

	_, err := svc.Upload(context.Background(), req)
	if err == nil {
		t.Fatal("expected error when slug generation fails")
	}
	if storage.savedKey != "" {
		t.Error("expected no file to be saved when slug generation fails")
	}
	if repo.insertedFile != nil {
		t.Error("expected no file to be inserted when slug generation fails")
	}
}

func TestGetBySlug_ValidFile(t *testing.T) {
	fixedNow := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	expiresAt := fixedNow.Add(1 * time.Hour)

	repo := &mockFileRepository{
		getBySlugRec: &paste.FileRecord{
			Slug:       "abc12345",
			Filename:   "test.txt",
			MIMEType:   "text/plain",
			SizeBytes:  100,
			StorageKey: "uploads/abc12345/test.txt",
			Visibility: paste.VisibilityPublic,
			ExpiresAt:  &expiresAt,
			CreatedAt:  fixedNow,
		},
	}
	storage := &mockFileStorage{}
	urlGen := &mockURLGenerator{slug: "abc12345"}
	svc := NewService(repo, storage, urlGen)
	svc.now = func() time.Time { return fixedNow }

	record, err := svc.GetBySlug(context.Background(), "abc12345")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if record.Slug != "abc12345" {
		t.Errorf("expected slug abc12345, got %s", record.Slug)
	}
	if record.Filename != "test.txt" {
		t.Errorf("expected filename test.txt, got %s", record.Filename)
	}
}

func TestGetBySlug_NotFound(t *testing.T) {
	repo := &mockFileRepository{
		getBySlugErr: errors.New("not found"),
	}
	storage := &mockFileStorage{}
	urlGen := &mockURLGenerator{slug: "abc12345"}
	svc := NewService(repo, storage, urlGen)

	_, err := svc.GetBySlug(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for non-existent slug")
	}
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestGetBySlug_Expired(t *testing.T) {
	fixedNow := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	expiredAt := fixedNow.Add(-1 * time.Hour) // expired 1 hour ago

	repo := &mockFileRepository{
		getBySlugRec: &paste.FileRecord{
			Slug:       "abc12345",
			Filename:   "old.txt",
			MIMEType:   "text/plain",
			SizeBytes:  50,
			StorageKey: "uploads/abc12345/old.txt",
			Visibility: paste.VisibilityPublic,
			ExpiresAt:  &expiredAt,
			CreatedAt:  fixedNow.Add(-2 * time.Hour),
		},
	}
	storage := &mockFileStorage{}
	urlGen := &mockURLGenerator{slug: "abc12345"}
	svc := NewService(repo, storage, urlGen)
	svc.now = func() time.Time { return fixedNow }

	_, err := svc.GetBySlug(context.Background(), "abc12345")
	if err == nil {
		t.Fatal("expected error for expired file")
	}
	if err != ErrExpired {
		t.Errorf("expected ErrExpired, got %v", err)
	}
}

func TestServeFile_SetsHeadersAndStreamsContent(t *testing.T) {
	fixedNow := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	expiresAt := fixedNow.Add(1 * time.Hour)
	fileContent := []byte("hello world file content")

	repo := &mockFileRepository{
		getBySlugRec: &paste.FileRecord{
			Slug:       "abc12345",
			Filename:   "report.pdf",
			MIMEType:   "application/pdf",
			SizeBytes:  int64(len(fileContent)),
			StorageKey: "uploads/abc12345/report.pdf",
			Visibility: paste.VisibilityPublic,
			ExpiresAt:  &expiresAt,
			CreatedAt:  fixedNow,
		},
	}
	storage := &mockFileStorage{
		openReader: io.NopCloser(bytes.NewReader(fileContent)),
	}
	urlGen := &mockURLGenerator{slug: "abc12345"}
	svc := NewService(repo, storage, urlGen)
	svc.now = func() time.Time { return fixedNow }

	rw := &fakeResponseWriter{headers: make(map[string][]string)}

	err := svc.ServeFile(context.Background(), "abc12345", rw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify Content-Disposition header
	cd := rw.headers.Get("Content-Disposition")
	expectedCD := `attachment; filename="report.pdf"`
	if cd != expectedCD {
		t.Errorf("expected Content-Disposition %q, got %q", expectedCD, cd)
	}

	// Verify Content-Type header
	ct := rw.headers.Get("Content-Type")
	if ct != "application/pdf" {
		t.Errorf("expected Content-Type application/pdf, got %s", ct)
	}

	// Verify Content-Length header
	cl := rw.headers.Get("Content-Length")
	expectedCL := fmt.Sprintf("%d", len(fileContent))
	if cl != expectedCL {
		t.Errorf("expected Content-Length %s, got %s", expectedCL, cl)
	}

	// Verify body content
	if !bytes.Equal(rw.body.Bytes(), fileContent) {
		t.Errorf("expected body %q, got %q", string(fileContent), rw.body.String())
	}
}

// fakeResponseWriter implements http.ResponseWriter for testing.
type fakeResponseWriter struct {
	headers    http.Header
	body       bytes.Buffer
	statusCode int
}

func (f *fakeResponseWriter) Header() http.Header {
	return f.headers
}

func (f *fakeResponseWriter) Write(data []byte) (int, error) {
	return f.body.Write(data)
}

func (f *fakeResponseWriter) WriteHeader(statusCode int) {
	f.statusCode = statusCode
}
