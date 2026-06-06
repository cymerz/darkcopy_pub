package handler

import (
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gthbn/pastebin/internal/paste"
)

// PasteHandler handles HTTP requests for paste operations.
type PasteHandler struct {
	pasteService     PasteService
	fileService      FileService
	highlighter      SyntaxHighlighter
	accessController AccessController
	// settings supplies runtime-configurable expiry options; may be nil, in
	// which case the built-in paste.ExpiryOptions are used.
	settings SettingsProvider
	// quota enforces per-IP daily paste-creation limits; may be nil (no limit).
	quota DailyQuota
}

// NewPasteHandler creates a new PasteHandler with the given dependencies.
func NewPasteHandler(ps PasteService, hl SyntaxHighlighter, ac AccessController, fs FileService) *PasteHandler {
	return &PasteHandler{
		pasteService:     ps,
		fileService:      fs,
		highlighter:      hl,
		accessController: ac,
	}
}

// SetSettings installs a settings provider used for dynamic expiry options.
func (h *PasteHandler) SetSettings(sp SettingsProvider) { h.settings = sp }

// SetQuota installs a daily quota enforcer for paste creation.
func (h *PasteHandler) SetQuota(q DailyQuota) { h.quota = q }

// RegisterPasteRoutes registers all paste-related routes on the given chi router.
func RegisterPasteRoutes(r chi.Router, h *PasteHandler) {
	r.Get("/", h.HandleIndex)
	r.Get("/new", h.HandleNewForm)
	r.Post("/new", h.HandleCreate)
	r.Get("/{slug}", h.HandleView)
	r.Get("/raw/{slug}", h.HandleRaw)
	r.Post("/{slug}/unlock", h.HandleUnlock)
}

// HandleIndex renders the home page with the list of recent public pastes.
func (h *PasteHandler) HandleIndex(w http.ResponseWriter, r *http.Request) {
	pastes, err := h.pasteService.ListPublicRecent(r.Context(), 20)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Gagal memuat daftar paste", "INTERNAL_ERROR")
		return
	}

	var files []*paste.FileSummary
	if h.fileService != nil {
		files, err = h.fileService.ListPublicRecent(r.Context(), 20)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "Gagal memuat daftar file", "INTERNAL_ERROR")
			return
		}
	}

	if files == nil {
		files = []*paste.FileSummary{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"pastes": pastes,
		"files":  files,
	})
}

// HandleNewForm renders the paste creation form.
func (h *PasteHandler) HandleNewForm(w http.ResponseWriter, r *http.Request) {
	languages := h.highlighter.SupportedLanguages()
	disableNewPastes := false
	if h.settings != nil {
		disableNewPastes = h.settings.Get().DisableNewPastes
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"languages":          languages,
		"expiryOptions":      h.pasteExpiryOptions(),
		"disable_new_pastes": disableNewPastes,
	})
}

// pasteExpiryOptions returns the expiry options for the new-paste form in the
// {label, duration(min)} wire shape the frontend expects. It uses the dynamic
// settings when available, falling back to the built-in paste.ExpiryOptions.
func (h *PasteHandler) pasteExpiryOptions() []map[string]interface{} {
	if h.settings != nil {
		opts := h.settings.Get().PasteExpiryOptions
		if len(opts) > 0 {
			out := make([]map[string]interface{}, 0, len(opts))
			for _, o := range opts {
				out = append(out, map[string]interface{}{"label": o.Label, "duration": o.Minutes})
			}
			return out
		}
	}
	out := make([]map[string]interface{}, 0, len(paste.ExpiryOptions))
	for _, o := range paste.ExpiryOptions {
		out = append(out, map[string]interface{}{"label": o.Label, "duration": int64(o.Duration.Minutes())})
	}
	return out
}

// HandleCreate processes the paste creation form submission.
func (h *PasteHandler) HandleCreate(w http.ResponseWriter, r *http.Request) {
	// Enforce temporary disable setting
	if h.settings != nil && h.settings.Get().DisableNewPastes {
		writeJSONError(w, http.StatusForbidden, "Pembuatan paste baru sedang dinonaktifkan sementara oleh administrator.", "PASTES_DISABLED")
		return
	}

	if err := r.ParseForm(); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Form tidak valid", "BAD_REQUEST")
		return
	}

	// Enforce per-IP daily paste-creation limit when configured.
	if h.quota != nil && h.settings != nil {
		limit := h.settings.Get().MaxPastesPerDayPerIP
		if limit > 0 {
			key := "paste:" + extractIP(r)
			if allowed, _ := h.quota.Allow(key, limit); !allowed {
				writeJSONError(w, http.StatusTooManyRequests, "Batas pembuatan paste harian tercapai. Coba lagi besok.", "DAILY_LIMIT_REACHED")
				return
			}
		}
	}

	content := r.FormValue("content")
	language := r.FormValue("language")
	title := r.FormValue("title")
	visibility := r.FormValue("visibility")
	password := r.FormValue("password")
	expiresInStr := r.FormValue("expires_in")
	customSlug := strings.TrimSpace(r.FormValue("custom_slug"))

	// Parse expiry duration (in minutes).
	var expiresIn time.Duration
	if expiresInStr != "" {
		minutes, err := strconv.ParseInt(expiresInStr, 10, 64)
		if err == nil {
			if minutes == 0 || minutes < 0 {
				expiresIn = paste.NeverExpires
			} else {
				expiresIn = time.Duration(minutes) * time.Minute
			}
		}
	}

	// Map visibility string to type.
	vis := paste.VisibilityPublic
	switch visibility {
	case "unlisted":
		vis = paste.VisibilityUnlisted
	case "password_protected":
		vis = paste.VisibilityPasswordProtected
	}

	req := paste.CreatePasteRequest{
		Content:    content,
		Language:   language,
		Title:      title,
		Visibility: vis,
		Password:   password,
		ExpiresIn:  expiresIn,
		CustomSlug: customSlug,
	}

	created, err := h.pasteService.Create(r.Context(), req)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error(), "VALIDATION_ERROR")
		return
	}

	// If the client wants JSON (e.g. fetch API), return 201 with slug and URL.
	if wantsJSON(r) {
		writeJSON(w, http.StatusCreated, map[string]string{
			"slug": created.Slug,
			"url":  "/" + created.Slug,
		})
		return
	}

	// Otherwise redirect to the newly created paste.
	http.Redirect(w, r, "/"+created.Slug, http.StatusSeeOther)
}

// HandleRaw serves the raw paste content as plain text.
func (h *PasteHandler) HandleRaw(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")

	p, err := h.pasteService.GetBySlug(r.Context(), slug)
	if err != nil {
		if errors.Is(err, paste.ErrNotFound) {
			http.Error(w, "Paste tidak ditemukan", http.StatusNotFound)
			return
		}
		if errors.Is(err, paste.ErrExpired) {
			http.Error(w, "Paste ini telah kadaluarsa", http.StatusGone)
			return
		}
		http.Error(w, "Gagal memuat paste", http.StatusInternalServerError)
		return
	}

	if p.Visibility == paste.VisibilityPasswordProtected {
		http.Error(w, "Password diperlukan", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(p.Content))
}

// HandleView displays a paste by its slug.
func (h *PasteHandler) HandleView(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")

	p, err := h.pasteService.GetBySlug(r.Context(), slug)
	if err != nil {
		if errors.Is(err, paste.ErrNotFound) {
			writeJSONError(w, http.StatusNotFound, "Paste tidak ditemukan", "NOT_FOUND")
			return
		}
		if errors.Is(err, paste.ErrExpired) {
			writeJSONError(w, http.StatusGone, "Paste ini telah kadaluarsa", "RESOURCE_EXPIRED")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "Gagal memuat paste", "INTERNAL_ERROR")
		return
	}

	// If paste is password protected, return 401 indicating password is required.
	if p.Visibility == paste.VisibilityPasswordProtected {
		writeJSON(w, http.StatusUnauthorized, map[string]interface{}{
			"error":             "Password diperlukan",
			"code":              "PASSWORD_REQUIRED",
			"status":            http.StatusUnauthorized,
			"password_required": true,
			"slug":              p.Slug,
		})
		return
	}

	// Highlight the content.
	highlighted, err := h.highlighter.Highlight(p.Content, p.Language)
	if err != nil {
		highlighted = p.Content
	}

	// Increment views!
	_ = h.pasteService.IncrementViews(r.Context(), slug)
	p.Views++

	// Calculate remaining time until expiry.
	var remainingSeconds *int64
	if p.ExpiresAt != nil {
		remaining := time.Until(*p.ExpiresAt)
		secs := int64(remaining.Seconds())
		remainingSeconds = &secs
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"slug":              p.Slug,
		"title":             p.Title,
		"content":           p.Content,
		"highlighted_html":  highlighted,
		"language":          p.Language,
		"visibility":        p.Visibility,
		"created_at":        p.CreatedAt,
		"expires_at":        p.ExpiresAt,
		"remaining_seconds": remainingSeconds,
		"views":             p.Views,
	})
}

// HandleUnlock processes the password submission for a protected paste.
func (h *PasteHandler) HandleUnlock(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")

	if err := r.ParseForm(); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Form tidak valid", "BAD_REQUEST")
		return
	}

	password := r.FormValue("password")
	clientIP := extractIP(r)

	// Check rate limiting first.
	limited, err := h.accessController.IsRateLimited(r.Context(), clientIP, slug)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Gagal memeriksa rate limit", "INTERNAL_ERROR")
		return
	}
	if limited {
		writeJSONError(w, http.StatusTooManyRequests, "Terlalu banyak percobaan. Silakan coba lagi nanti.", "RATE_LIMITED")
		return
	}

	// Validate password.
	valid, err := h.pasteService.ValidatePassword(r.Context(), slug, password)
	if err != nil {
		if errors.Is(err, paste.ErrNotFound) {
			writeJSONError(w, http.StatusNotFound, "Paste tidak ditemukan", "NOT_FOUND")
			return
		}
		if errors.Is(err, paste.ErrExpired) {
			writeJSONError(w, http.StatusGone, "Paste ini telah kadaluarsa", "RESOURCE_EXPIRED")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "Gagal memvalidasi password", "INTERNAL_ERROR")
		return
	}

	if !valid {
		// Record failed attempt.
		_ = h.accessController.RecordFailedAttempt(r.Context(), clientIP, slug)
		writeJSONError(w, http.StatusUnauthorized, "Password salah", "INVALID_PASSWORD")
		return
	}

	// Password correct — reset rate limit and return paste content.
	h.accessController.ResetRateLimit(r.Context(), clientIP, slug)

	// Increment views!
	_ = h.pasteService.IncrementViews(r.Context(), slug)

	// Fetch the paste to return its content.
	p, err := h.pasteService.GetBySlug(r.Context(), slug)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Gagal memuat paste", "INTERNAL_ERROR")
		return
	}

	// Highlight the content.
	highlighted, err := h.highlighter.Highlight(p.Content, p.Language)
	if err != nil {
		highlighted = p.Content
	}

	var remainingSeconds *int64
	if p.ExpiresAt != nil {
		remaining := time.Until(*p.ExpiresAt)
		secs := int64(remaining.Seconds())
		remainingSeconds = &secs
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"slug":              p.Slug,
		"title":             p.Title,
		"content":           p.Content,
		"highlighted_html":  highlighted,
		"language":          p.Language,
		"visibility":        p.Visibility,
		"created_at":        p.CreatedAt,
		"expires_at":        p.ExpiresAt,
		"remaining_seconds": remainingSeconds,
		"views":             p.Views,
	})
}

// extractIP extracts the client IP address from the request, stripping the port.
func extractIP(r *http.Request) string {
	// 1. Check Cloudflare-specific header
	if cfip := r.Header.Get("CF-Connecting-IP"); cfip != "" {
		return cfip
	}
	// 2. Check X-Forwarded-For header (set by OpenResty/Nginx)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		if clientIP := strings.TrimSpace(ips[0]); clientIP != "" {
			return clientIP
		}
	}
	// 3. Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	// 4. Fallback to RemoteAddr
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
