package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gthbn/pastebin/internal/highlight"
	"github.com/gthbn/pastebin/internal/paste"
	"github.com/gthbn/pastebin/internal/settings"
)

// --- Mock implementations ---

type mockPasteService struct {
	createFn           func(ctx context.Context, req paste.CreatePasteRequest) (*paste.Paste, error)
	getBySlugFn        func(ctx context.Context, slug string) (*paste.Paste, error)
	validatePasswordFn func(ctx context.Context, slug, password string) (bool, error)
	listPublicRecentFn func(ctx context.Context, limit int) ([]*paste.PasteSummary, error)
}

func (m *mockPasteService) Create(ctx context.Context, req paste.CreatePasteRequest) (*paste.Paste, error) {
	if m.createFn != nil {
		return m.createFn(ctx, req)
	}
	return nil, nil
}

func (m *mockPasteService) GetBySlug(ctx context.Context, slug string) (*paste.Paste, error) {
	if m.getBySlugFn != nil {
		return m.getBySlugFn(ctx, slug)
	}
	return nil, paste.ErrNotFound
}

func (m *mockPasteService) ValidatePassword(ctx context.Context, slug, password string) (bool, error) {
	if m.validatePasswordFn != nil {
		return m.validatePasswordFn(ctx, slug, password)
	}
	return false, nil
}

func (m *mockPasteService) ListPublicRecent(ctx context.Context, limit int) ([]*paste.PasteSummary, error) {
	if m.listPublicRecentFn != nil {
		return m.listPublicRecentFn(ctx, limit)
	}
	return nil, nil
}

func (m *mockPasteService) IncrementViews(ctx context.Context, slug string) error {
	return nil
}


type mockHighlighter struct {
	highlightFn          func(content, language string) (string, error)
	supportedLanguagesFn func() []highlight.Language
}

func (m *mockHighlighter) Highlight(content, language string) (string, error) {
	if m.highlightFn != nil {
		return m.highlightFn(content, language)
	}
	return "<span>" + content + "</span>", nil
}

func (m *mockHighlighter) SupportedLanguages() []highlight.Language {
	if m.supportedLanguagesFn != nil {
		return m.supportedLanguagesFn()
	}
	return []highlight.Language{{ID: "go", Name: "Go"}, {ID: "python", Name: "Python"}}
}

type mockAccessController struct {
	checkAccessFn        func(ctx context.Context, resourceID, password string) (AccessResult, error)
	recordFailedFn       func(ctx context.Context, ip, resourceID string) error
	isRateLimitedFn      func(ctx context.Context, ip, resourceID string) (bool, error)
	resetRateLimitCalled bool
}

func (m *mockAccessController) CheckAccess(ctx context.Context, resourceID, password string) (AccessResult, error) {
	if m.checkAccessFn != nil {
		return m.checkAccessFn(ctx, resourceID, password)
	}
	return AccessGranted, nil
}

func (m *mockAccessController) RecordFailedAttempt(ctx context.Context, ip, resourceID string) error {
	if m.recordFailedFn != nil {
		return m.recordFailedFn(ctx, ip, resourceID)
	}
	return nil
}

func (m *mockAccessController) IsRateLimited(ctx context.Context, ip, resourceID string) (bool, error) {
	if m.isRateLimitedFn != nil {
		return m.isRateLimitedFn(ctx, ip, resourceID)
	}
	return false, nil
}

func (m *mockAccessController) ResetRateLimit(ctx context.Context, ip, resourceSlug string) {
	m.resetRateLimitCalled = true
}

// --- Helper to create a chi router with paste routes ---

func setupRouter(h *PasteHandler) *chi.Mux {
	r := chi.NewRouter()
	RegisterPasteRoutes(r, h)
	return r
}

// --- Tests ---

func TestHandleIndex_Returns200(t *testing.T) {
	ps := &mockPasteService{
		listPublicRecentFn: func(ctx context.Context, limit int) ([]*paste.PasteSummary, error) {
			return []*paste.PasteSummary{
				{Slug: "abc12345", Title: "Test Paste", Language: "go", CreatedAt: time.Now()},
			}, nil
		},
	}
	hl := &mockHighlighter{}
	ac := &mockAccessController{}
	h := NewPasteHandler(ps, hl, ac, nil)
	router := setupRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	pastes, ok := resp["pastes"].([]interface{})
	if !ok {
		t.Fatal("expected 'pastes' field in response")
	}
	if len(pastes) != 1 {
		t.Errorf("expected 1 paste, got %d", len(pastes))
	}
}

func TestHandleCreate_ValidData_Redirects(t *testing.T) {
	ps := &mockPasteService{
		createFn: func(ctx context.Context, req paste.CreatePasteRequest) (*paste.Paste, error) {
			return &paste.Paste{
				Slug:    "newslug1",
				Content: req.Content,
			}, nil
		},
	}
	hl := &mockHighlighter{}
	ac := &mockAccessController{}
	h := NewPasteHandler(ps, hl, ac, nil)
	router := setupRouter(h)

	form := url.Values{}
	form.Set("content", "Hello, World!")
	form.Set("language", "plaintext")
	form.Set("title", "My Paste")
	form.Set("visibility", "public")

	req := httptest.NewRequest(http.MethodPost, "/new", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("expected status 303, got %d", rec.Code)
	}

	location := rec.Header().Get("Location")
	if location != "/newslug1" {
		t.Errorf("expected redirect to /newslug1, got %s", location)
	}
}

func TestHandleCreate_AcceptJSON_Returns201WithJSON(t *testing.T) {
	ps := &mockPasteService{
		createFn: func(ctx context.Context, req paste.CreatePasteRequest) (*paste.Paste, error) {
			return &paste.Paste{
				Slug:    "jsonslug1",
				Content: req.Content,
			}, nil
		},
	}
	hl := &mockHighlighter{}
	ac := &mockAccessController{}
	h := NewPasteHandler(ps, hl, ac, nil)
	router := setupRouter(h)

	form := url.Values{}
	form.Set("content", "Hello, JSON!")
	form.Set("language", "plaintext")
	form.Set("title", "JSON Paste")
	form.Set("visibility", "public")

	req := httptest.NewRequest(http.MethodPost, "/new", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", rec.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["slug"] != "jsonslug1" {
		t.Errorf("expected slug 'jsonslug1', got %q", resp["slug"])
	}
	if resp["url"] != "/jsonslug1" {
		t.Errorf("expected url '/jsonslug1', got %q", resp["url"])
	}
}

func TestHandleCreate_EmptyContent_ReturnsBadRequest(t *testing.T) {
	ps := &mockPasteService{
		createFn: func(ctx context.Context, req paste.CreatePasteRequest) (*paste.Paste, error) {
			return nil, paste.ErrEmptyContent
		},
	}
	hl := &mockHighlighter{}
	ac := &mockAccessController{}
	h := NewPasteHandler(ps, hl, ac, nil)
	router := setupRouter(h)

	form := url.Values{}
	form.Set("content", "")
	form.Set("language", "plaintext")

	req := httptest.NewRequest(http.MethodPost, "/new", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rec.Code)
	}
}

func TestHandleView_ReturnsPasteContent(t *testing.T) {
	now := time.Now()
	expiresAt := now.Add(1 * time.Hour)
	ps := &mockPasteService{
		getBySlugFn: func(ctx context.Context, slug string) (*paste.Paste, error) {
			if slug == "testslug" {
				return &paste.Paste{
					Slug:       "testslug",
					Title:      "Test",
					Content:    "Hello",
					Language:   "go",
					Visibility: paste.VisibilityPublic,
					CreatedAt:  now,
					ExpiresAt:  &expiresAt,
				}, nil
			}
			return nil, paste.ErrNotFound
		},
	}
	hl := &mockHighlighter{
		highlightFn: func(content, language string) (string, error) {
			return "<span class=\"kw\">Hello</span>", nil
		},
	}
	ac := &mockAccessController{}
	h := NewPasteHandler(ps, hl, ac, nil)
	router := setupRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/testslug", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["slug"] != "testslug" {
		t.Errorf("expected slug 'testslug', got %v", resp["slug"])
	}
	if resp["content"] != "Hello" {
		t.Errorf("expected content 'Hello', got %v", resp["content"])
	}
	if resp["highlighted_html"] != "<span class=\"kw\">Hello</span>" {
		t.Errorf("unexpected highlighted_html: %v", resp["highlighted_html"])
	}
}

func TestHandleView_NotFound_Returns404(t *testing.T) {
	ps := &mockPasteService{
		getBySlugFn: func(ctx context.Context, slug string) (*paste.Paste, error) {
			return nil, paste.ErrNotFound
		},
	}
	hl := &mockHighlighter{}
	ac := &mockAccessController{}
	h := NewPasteHandler(ps, hl, ac, nil)
	router := setupRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/nonexistent", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rec.Code)
	}
}

func TestHandleView_Expired_Returns410(t *testing.T) {
	ps := &mockPasteService{
		getBySlugFn: func(ctx context.Context, slug string) (*paste.Paste, error) {
			return nil, paste.ErrExpired
		},
	}
	hl := &mockHighlighter{}
	ac := &mockAccessController{}
	h := NewPasteHandler(ps, hl, ac, nil)
	router := setupRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/expiredslug", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusGone {
		t.Errorf("expected status 410, got %d", rec.Code)
	}
}

func TestHandleView_PasswordProtected_Returns401(t *testing.T) {
	ps := &mockPasteService{
		getBySlugFn: func(ctx context.Context, slug string) (*paste.Paste, error) {
			return &paste.Paste{
				Slug:       "protected",
				Title:      "Secret",
				Content:    "Hidden content",
				Language:   "plaintext",
				Visibility: paste.VisibilityPasswordProtected,
				CreatedAt:  time.Now(),
			}, nil
		},
	}
	hl := &mockHighlighter{}
	ac := &mockAccessController{}
	h := NewPasteHandler(ps, hl, ac, nil)
	router := setupRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rec.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["password_required"] != true {
		t.Error("expected password_required to be true")
	}
}

func TestHandleUnlock_RateLimited_Returns429(t *testing.T) {
	ps := &mockPasteService{}
	hl := &mockHighlighter{}
	ac := &mockAccessController{
		isRateLimitedFn: func(ctx context.Context, ip, resourceID string) (bool, error) {
			return true, nil
		},
	}
	h := NewPasteHandler(ps, hl, ac, nil)
	router := setupRouter(h)

	form := url.Values{}
	form.Set("password", "wrong")

	req := httptest.NewRequest(http.MethodPost, "/someslug/unlock", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.RemoteAddr = "192.168.1.1:12345"
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("expected status 429, got %d", rec.Code)
	}
}

func TestHandleUnlock_WrongPassword_Returns401(t *testing.T) {
	ps := &mockPasteService{
		validatePasswordFn: func(ctx context.Context, slug, password string) (bool, error) {
			return false, nil
		},
	}
	hl := &mockHighlighter{}
	recordCalled := false
	ac := &mockAccessController{
		isRateLimitedFn: func(ctx context.Context, ip, resourceID string) (bool, error) {
			return false, nil
		},
		recordFailedFn: func(ctx context.Context, ip, resourceID string) error {
			recordCalled = true
			return nil
		},
	}
	h := NewPasteHandler(ps, hl, ac, nil)
	router := setupRouter(h)

	form := url.Values{}
	form.Set("password", "wrongpass")

	req := httptest.NewRequest(http.MethodPost, "/protslug/unlock", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.RemoteAddr = "10.0.0.1:9999"
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rec.Code)
	}
	if !recordCalled {
		t.Error("expected RecordFailedAttempt to be called")
	}
}

func TestHandleUnlock_CorrectPassword_Returns200(t *testing.T) {
	now := time.Now()
	ps := &mockPasteService{
		validatePasswordFn: func(ctx context.Context, slug, password string) (bool, error) {
			return true, nil
		},
		getBySlugFn: func(ctx context.Context, slug string) (*paste.Paste, error) {
			return &paste.Paste{
				Slug:       slug,
				Title:      "Secret Paste",
				Content:    "Secret content",
				Language:   "go",
				Visibility: paste.VisibilityPasswordProtected,
				CreatedAt:  now,
			}, nil
		},
	}
	hl := &mockHighlighter{}
	ac := &mockAccessController{
		isRateLimitedFn: func(ctx context.Context, ip, resourceID string) (bool, error) {
			return false, nil
		},
	}
	h := NewPasteHandler(ps, hl, ac, nil)
	router := setupRouter(h)

	form := url.Values{}
	form.Set("password", "correctpass")

	req := httptest.NewRequest(http.MethodPost, "/secretslug/unlock", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.RemoteAddr = "10.0.0.1:9999"
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["content"] != "Secret content" {
		t.Errorf("expected content 'Secret content', got %v", resp["content"])
	}

	if !ac.resetRateLimitCalled {
		t.Error("expected ResetRateLimit to be called")
	}
}

func TestHandleUnlock_PasteNotFound_Returns404(t *testing.T) {
	ps := &mockPasteService{
		validatePasswordFn: func(ctx context.Context, slug, password string) (bool, error) {
			return false, paste.ErrNotFound
		},
	}
	hl := &mockHighlighter{}
	ac := &mockAccessController{
		isRateLimitedFn: func(ctx context.Context, ip, resourceID string) (bool, error) {
			return false, nil
		},
	}
	h := NewPasteHandler(ps, hl, ac, nil)
	router := setupRouter(h)

	form := url.Values{}
	form.Set("password", "test")

	req := httptest.NewRequest(http.MethodPost, "/missing/unlock", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.RemoteAddr = "10.0.0.1:9999"
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rec.Code)
	}
}

func TestHandleIndex_ServiceError_Returns500(t *testing.T) {
	ps := &mockPasteService{
		listPublicRecentFn: func(ctx context.Context, limit int) ([]*paste.PasteSummary, error) {
			return nil, errors.New("database error")
		},
	}
	hl := &mockHighlighter{}
	ac := &mockAccessController{}
	h := NewPasteHandler(ps, hl, ac, nil)
	router := setupRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rec.Code)
	}
}

func TestHandleNewForm_Returns200(t *testing.T) {
	ps := &mockPasteService{}
	hl := &mockHighlighter{}
	ac := &mockAccessController{}
	h := NewPasteHandler(ps, hl, ac, nil)
	router := setupRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/new", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if _, ok := resp["languages"]; !ok {
		t.Error("expected 'languages' field in response")
	}
	if _, ok := resp["expiryOptions"]; !ok {
		t.Error("expected 'expiryOptions' field in response")
	}
}

// --- Settings/quota stubs for the dynamic-limit tests ---

type stubSettingsProvider struct {
	s settings.Settings
}

func (p stubSettingsProvider) Get() settings.Settings { return p.s }

type stubQuota struct {
	allow     bool
	remaining int
	gotKey    string
	gotLimit  int
}

func (q *stubQuota) Allow(key string, limit int) (bool, int) {
	q.gotKey = key
	q.gotLimit = limit
	return q.allow, q.remaining
}

func TestHandleCreate_DailyLimitReached_Returns429(t *testing.T) {
	ps := &mockPasteService{
		createFn: func(ctx context.Context, req paste.CreatePasteRequest) (*paste.Paste, error) {
			t.Fatal("Create must not be called when the daily limit is reached")
			return nil, nil
		},
	}
	h := NewPasteHandler(ps, &mockHighlighter{}, &mockAccessController{}, nil)
	sp := stubSettingsProvider{s: settings.Defaults()}
	sp.s.MaxPastesPerDayPerIP = 2
	h.SetSettings(sp)
	q := &stubQuota{allow: false, remaining: 0}
	h.SetQuota(q)
	router := setupRouter(h)

	form := url.Values{}
	form.Set("content", "hello")
	req := httptest.NewRequest(http.MethodPost, "/new", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.RemoteAddr = "203.0.113.7:5555"
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("expected status 429, got %d", rec.Code)
	}
	if q.gotLimit != 2 {
		t.Errorf("expected quota checked with limit 2, got %d", q.gotLimit)
	}
	if q.gotKey != "paste:203.0.113.7" {
		t.Errorf("expected quota key 'paste:203.0.113.7', got %q", q.gotKey)
	}
}

func TestHandleCreate_UnderDailyLimit_Succeeds(t *testing.T) {
	ps := &mockPasteService{
		createFn: func(ctx context.Context, req paste.CreatePasteRequest) (*paste.Paste, error) {
			return &paste.Paste{Slug: "okslug12"}, nil
		},
	}
	h := NewPasteHandler(ps, &mockHighlighter{}, &mockAccessController{}, nil)
	sp := stubSettingsProvider{s: settings.Defaults()}
	sp.s.MaxPastesPerDayPerIP = 5
	h.SetSettings(sp)
	h.SetQuota(&stubQuota{allow: true, remaining: 4})
	router := setupRouter(h)

	form := url.Values{}
	form.Set("content", "hello")
	req := httptest.NewRequest(http.MethodPost, "/new", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.RemoteAddr = "203.0.113.7:5555"
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", rec.Code)
	}
}

func TestHandleNewForm_UsesDynamicExpiryOptions(t *testing.T) {
	h := NewPasteHandler(&mockPasteService{}, &mockHighlighter{}, &mockAccessController{}, nil)
	sp := stubSettingsProvider{s: settings.Settings{
		PasteExpiryOptions: []settings.ExpiryOption{
			{Label: "Tes 5 Menit", Minutes: 5},
		},
	}}
	h.SetSettings(sp)
	router := setupRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/new", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp struct {
		ExpiryOptions []struct {
			Label    string `json:"label"`
			Duration int64  `json:"duration"`
		} `json:"expiryOptions"`
		DisableNewPastes bool `json:"disable_new_pastes"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.ExpiryOptions) != 1 || resp.ExpiryOptions[0].Label != "Tes 5 Menit" || resp.ExpiryOptions[0].Duration != 5 {
		t.Errorf("expected dynamic expiry option, got %+v", resp.ExpiryOptions)
	}
}

func TestHandleCreate_NeverExpires(t *testing.T) {
	var capturedExpiresIn time.Duration
	ps := &mockPasteService{
		createFn: func(ctx context.Context, req paste.CreatePasteRequest) (*paste.Paste, error) {
			capturedExpiresIn = req.ExpiresIn
			return &paste.Paste{
				Slug:    "newslug1",
				Content: req.Content,
			}, nil
		},
	}
	hl := &mockHighlighter{}
	ac := &mockAccessController{}
	h := NewPasteHandler(ps, hl, ac, nil)
	router := setupRouter(h)

	form := url.Values{}
	form.Set("content", "Hello, World!")
	form.Set("language", "plaintext")
	form.Set("title", "My Paste")
	form.Set("visibility", "public")
	form.Set("expires_in", "0") // sentinel for "never expires"

	req := httptest.NewRequest(http.MethodPost, "/new", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther && rec.Code != http.StatusCreated {
		t.Errorf("expected status 303 or 201, got %d", rec.Code)
	}

	if capturedExpiresIn != paste.NeverExpires {
		t.Errorf("expected ExpiresIn to be paste.NeverExpires (%v), got %v", paste.NeverExpires, capturedExpiresIn)
	}
}

func TestHandleCreate_Disabled(t *testing.T) {
	ps := &mockPasteService{}
	hl := &mockHighlighter{}
	ac := &mockAccessController{}
	h := NewPasteHandler(ps, hl, ac, nil)
	
	// Create mock settings with DisableNewPastes = true
	s := settings.Defaults()
	s.DisableNewPastes = true
	h.SetSettings(settings.NewProvider(s))
	
	router := setupRouter(h)

	form := url.Values{}
	form.Set("content", "Hello, World!")
	form.Set("language", "plaintext")

	req := httptest.NewRequest(http.MethodPost, "/new", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected status 403 Forbidden, got %d", rec.Code)
	}
}
