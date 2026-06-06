package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/gthbn/pastebin/internal/file"
	"github.com/gthbn/pastebin/internal/paste"
	"github.com/gthbn/pastebin/internal/quota"
	"github.com/gthbn/pastebin/internal/settings"
)

// mockFileService is a mock implementation of FileService for testing.
type mockFileService struct {
	uploadFn           func(ctx context.Context, req paste.UploadFileRequest) (*paste.FileRecord, error)
	getBySlugFn        func(ctx context.Context, slug string) (*paste.FileRecord, error)
	serveFileFn        func(ctx context.Context, slug string, w http.ResponseWriter) error
	validatePasswordFn func(ctx context.Context, slug, password string) (bool, error)
}

func (m *mockFileService) Upload(ctx context.Context, req paste.UploadFileRequest) (*paste.FileRecord, error) {
	if m.uploadFn != nil {
		return m.uploadFn(ctx, req)
	}
	return nil, nil
}

func (m *mockFileService) GetBySlug(ctx context.Context, slug string) (*paste.FileRecord, error) {
	if m.getBySlugFn != nil {
		return m.getBySlugFn(ctx, slug)
	}
	return nil, file.ErrNotFound
}

func (m *mockFileService) ServeFile(ctx context.Context, slug string, w http.ResponseWriter) error {
	if m.serveFileFn != nil {
		return m.serveFileFn(ctx, slug, w)
	}
	return nil
}

func (m *mockFileService) ValidatePassword(ctx context.Context, slug, password string) (bool, error) {
	if m.validatePasswordFn != nil {
		return m.validatePasswordFn(ctx, slug, password)
	}
	return false, nil
}

func (m *mockFileService) ListPublicRecent(ctx context.Context, limit int) ([]*paste.FileSummary, error) {
	return nil, nil
}

func (m *mockFileService) PresignDownloadURL(ctx context.Context, slug string, inline bool) (string, error) {
	return "", file.ErrPresignUnsupported
}

func (m *mockFileService) IncrementDownloads(ctx context.Context, slug string) error {
	return nil
}


// mockAccessCtrl is a mock implementation of AccessController for testing.
type mockAccessCtrl struct {
	isRateLimitedFn     func(ctx context.Context, ip, resource string) (bool, error)
	recordFailedFn      func(ctx context.Context, ip, resource string) error
	resetRateLimitFn    func(ctx context.Context, ip, resource string)
	checkAccessFn       func(ctx context.Context, resourceID, password string) (AccessResult, error)
}

func (m *mockAccessCtrl) CheckAccess(ctx context.Context, resourceID, password string) (AccessResult, error) {
	if m.checkAccessFn != nil {
		return m.checkAccessFn(ctx, resourceID, password)
	}
	return AccessGranted, nil
}

func (m *mockAccessCtrl) RecordFailedAttempt(ctx context.Context, ip, resource string) error {
	if m.recordFailedFn != nil {
		return m.recordFailedFn(ctx, ip, resource)
	}
	return nil
}

func (m *mockAccessCtrl) IsRateLimited(ctx context.Context, ip, resource string) (bool, error) {
	if m.isRateLimitedFn != nil {
		return m.isRateLimitedFn(ctx, ip, resource)
	}
	return false, nil
}

func (m *mockAccessCtrl) ResetRateLimit(ctx context.Context, ip, resource string) {
	if m.resetRateLimitFn != nil {
		m.resetRateLimitFn(ctx, ip, resource)
	}
}

// newFileTestRouter creates a chi router with file routes registered for testing.
func newFileTestRouter(fs FileService, ac AccessController) *chi.Mux {
	h := NewFileHandler(fs, ac)
	r := chi.NewRouter()
	RegisterFileRoutes(r, h)
	return r
}

// createMultipartRequest creates a multipart form request with a file field.
func createMultipartRequest(t *testing.T, filename, content string, fields map[string]string) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Add file field.
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatal(err)
	}
	_, err = io.WriteString(part, content)
	if err != nil {
		t.Fatal(err)
	}

	// Add other form fields.
	for key, val := range fields {
		if err := writer.WriteField(key, val); err != nil {
			t.Fatal(err)
		}
	}

	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/upload", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

func TestHandleUpload_ValidFile_ReturnsSuccess(t *testing.T) {
	fs := &mockFileService{
		uploadFn: func(ctx context.Context, req paste.UploadFileRequest) (*paste.FileRecord, error) {
			return &paste.FileRecord{
				ID:       uuid.New(),
				Slug:     "abc12345",
				Filename: req.Filename,
				MIMEType: req.MIMEType,
			}, nil
		},
	}
	ac := &mockAccessCtrl{}

	router := newFileTestRouter(fs, ac)

	req := createMultipartRequest(t, "test.txt", "hello world", map[string]string{
		"visibility": "public",
		"expires_in": "24h",
	})
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d", http.StatusCreated, rr.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}

	if resp["success"] != true {
		t.Errorf("expected success=true, got %v", resp["success"])
	}
	if resp["slug"] != "abc12345" {
		t.Errorf("expected slug=abc12345, got %v", resp["slug"])
	}
	if resp["url"] != "/f/abc12345" {
		t.Errorf("expected url=/f/abc12345, got %v", resp["url"])
	}
}

func TestGetFile_NonExistent_Returns404(t *testing.T) {
	fs := &mockFileService{
		getBySlugFn: func(ctx context.Context, slug string) (*paste.FileRecord, error) {
			return nil, file.ErrNotFound
		},
	}
	ac := &mockAccessCtrl{}

	router := newFileTestRouter(fs, ac)

	req := httptest.NewRequest(http.MethodGet, "/f/nonexist1", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, rr.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}

	if resp["code"] != "NOT_FOUND" {
		t.Errorf("expected code=NOT_FOUND, got %v", resp["code"])
	}
}

func TestGetFile_Expired_Returns410(t *testing.T) {
	fs := &mockFileService{
		getBySlugFn: func(ctx context.Context, slug string) (*paste.FileRecord, error) {
			return nil, file.ErrExpired
		},
	}
	ac := &mockAccessCtrl{}

	router := newFileTestRouter(fs, ac)

	req := httptest.NewRequest(http.MethodGet, "/f/expired1", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusGone {
		t.Errorf("expected status %d, got %d", http.StatusGone, rr.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}

	if resp["code"] != "RESOURCE_EXPIRED" {
		t.Errorf("expected code=RESOURCE_EXPIRED, got %v", resp["code"])
	}
}

func TestGetFile_PasswordProtected_Returns401(t *testing.T) {
	fs := &mockFileService{
		getBySlugFn: func(ctx context.Context, slug string) (*paste.FileRecord, error) {
			return &paste.FileRecord{
				ID:         uuid.New(),
				Slug:       slug,
				Filename:   "secret.pdf",
				Visibility: paste.VisibilityPasswordProtected,
			}, nil
		},
	}
	ac := &mockAccessCtrl{}

	router := newFileTestRouter(fs, ac)

	req := httptest.NewRequest(http.MethodGet, "/f/protect1", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}

	if resp["code"] != "PASSWORD_REQUIRED" {
		t.Errorf("expected code=PASSWORD_REQUIRED, got %v", resp["code"])
	}
}

func TestGetFile_Public_ServesFile(t *testing.T) {
	fs := &mockFileService{
		getBySlugFn: func(ctx context.Context, slug string) (*paste.FileRecord, error) {
			return &paste.FileRecord{
				ID:         uuid.New(),
				Slug:       slug,
				Filename:   "hello.txt",
				MIMEType:   "text/plain",
				Visibility: paste.VisibilityPublic,
			}, nil
		},
		serveFileFn: func(ctx context.Context, slug string, w http.ResponseWriter) error {
			w.Header().Set("Content-Type", "text/plain")
			w.Header().Set("Content-Disposition", `attachment; filename="hello.txt"`)
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("file content"))
			return nil
		},
	}
	ac := &mockAccessCtrl{}

	router := newFileTestRouter(fs, ac)

	req := httptest.NewRequest(http.MethodGet, "/f/public12", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	if rr.Body.String() != "file content" {
		t.Errorf("expected body 'file content', got %q", rr.Body.String())
	}
}

func TestUnlockFile_RateLimited_Returns429(t *testing.T) {
	fs := &mockFileService{}
	ac := &mockAccessCtrl{
		isRateLimitedFn: func(ctx context.Context, ip, resource string) (bool, error) {
			return true, nil
		},
	}

	router := newFileTestRouter(fs, ac)

	form := url.Values{}
	form.Set("password", "test123")
	req := httptest.NewRequest(http.MethodPost, "/f/ratelim1/unlock", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusTooManyRequests {
		t.Errorf("expected status %d, got %d", http.StatusTooManyRequests, rr.Code)
	}
}

func TestUnlockFile_WrongPassword_Returns401(t *testing.T) {
	fs := &mockFileService{
		validatePasswordFn: func(ctx context.Context, slug, password string) (bool, error) {
			return false, nil
		},
	}
	failedRecorded := false
	ac := &mockAccessCtrl{
		recordFailedFn: func(ctx context.Context, ip, resource string) error {
			failedRecorded = true
			return nil
		},
	}

	router := newFileTestRouter(fs, ac)

	form := url.Values{}
	form.Set("password", "wrongpass")
	req := httptest.NewRequest(http.MethodPost, "/f/protect1/unlock", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}

	if !failedRecorded {
		t.Error("expected RecordFailedAttempt to be called")
	}
}

func TestUnlockFile_CorrectPassword_ServesFile(t *testing.T) {
	fs := &mockFileService{
		validatePasswordFn: func(ctx context.Context, slug, password string) (bool, error) {
			return true, nil
		},
		serveFileFn: func(ctx context.Context, slug string, w http.ResponseWriter) error {
			w.Header().Set("Content-Type", "application/pdf")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("pdf content"))
			return nil
		},
	}
	resetCalled := false
	ac := &mockAccessCtrl{
		resetRateLimitFn: func(ctx context.Context, ip, resource string) {
			resetCalled = true
		},
	}

	router := newFileTestRouter(fs, ac)

	form := url.Values{}
	form.Set("password", "correct")
	req := httptest.NewRequest(http.MethodPost, "/f/protect1/unlock", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	if !resetCalled {
		t.Error("expected ResetRateLimit to be called")
	}

	if rr.Body.String() != "pdf content" {
		t.Errorf("expected body 'pdf content', got %q", rr.Body.String())
	}
}

func TestHandleUpload_FileTooLarge_Returns413(t *testing.T) {
	fs := &mockFileService{
		uploadFn: func(ctx context.Context, req paste.UploadFileRequest) (*paste.FileRecord, error) {
			return nil, file.ErrFileTooLarge
		},
	}
	ac := &mockAccessCtrl{}

	router := newFileTestRouter(fs, ac)

	req := createMultipartRequest(t, "big.bin", "data", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("expected status %d, got %d", http.StatusRequestEntityTooLarge, rr.Code)
	}
}

func TestShowUploadForm_ReturnsOptions(t *testing.T) {
	fs := &mockFileService{}
	ac := &mockAccessCtrl{}

	router := newFileTestRouter(fs, ac)

	req := httptest.NewRequest(http.MethodGet, "/upload", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}

	if resp["visibilities"] == nil {
		t.Error("expected visibilities in response")
	}
	if resp["expiry_options"] == nil {
		t.Error("expected expiry_options in response")
	}
	if resp["max_file_size"] == nil {
		t.Error("expected max_file_size in response")
	}
	if resp["disable_file_uploads"] == nil {
		t.Error("expected disable_file_uploads in response")
	}
}

func TestParseExpiryDuration(t *testing.T) {
	tests := []struct {
		input    string
		expected time.Duration
		wantErr  bool
	}{
		{"", 0, false},
		{"-1", file.NeverExpires, false},
		{"1h", time.Hour, false},
		{"24h", 24 * time.Hour, false},
		{"168h", 168 * time.Hour, false},
		{"720h", 720 * time.Hour, false},
		{"invalid", 0, true},
		{"-2h", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseExpiryDuration(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseExpiryDuration(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got != tt.expected {
				t.Errorf("parseExpiryDuration(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestHandleUpload_Disabled(t *testing.T) {
	fs := &mockFileService{}
	ac := &mockAccessCtrl{}
	h := NewFileHandler(fs, ac)

	s := settings.Defaults()
	s.DisableFileUploads = true
	h.SetSettings(settings.NewProvider(s))

	r := chi.NewRouter()
	RegisterFileRoutes(r, h)

	req := createMultipartRequest(t, "test.txt", "some content", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected status 403 Forbidden, got %d", rec.Code)
	}
}

func TestHandleUpload_DailySizeLimits(t *testing.T) {
	fs := &mockFileService{
		uploadFn: func(ctx context.Context, req paste.UploadFileRequest) (*paste.FileRecord, error) {
			return &paste.FileRecord{}, nil
		},
	}
	ac := &mockAccessCtrl{}
	h := NewFileHandler(fs, ac)

	s := settings.Defaults()
	s.MaxDailyUploadBytes = 20
	s.MaxDailyUploadBytesPerIP = 15
	h.SetSettings(settings.NewProvider(s))

	sc := quota.NewSizeCounter()
	h.SetSizeQuota(sc)

	r := chi.NewRouter()
	RegisterFileRoutes(r, h)

	// 1. First upload of 10 bytes: should be allowed (under IP limit 15, and global limit 20)
	req1 := createMultipartRequest(t, "test.txt", "1234567890", nil)
	rec1 := httptest.NewRecorder()
	r.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusOK && rec1.Code != http.StatusCreated {
		t.Errorf("expected status OK, got %d", rec1.Code)
	}

	// 2. Second upload of 10 bytes from same IP: should be blocked by IP limit (total 20 > 15)
	req2 := createMultipartRequest(t, "test2.txt", "1234567890", nil)
	rec2 := httptest.NewRecorder()
	r.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusTooManyRequests {
		t.Errorf("expected status 429 Too Many Requests, got %d", rec2.Code)
	}
}
