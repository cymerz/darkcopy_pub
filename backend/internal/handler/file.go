package handler

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gthbn/pastebin/internal/file"
	"github.com/gthbn/pastebin/internal/paste"
)

// FileService defines the interface for file operations used by the handler.
type FileService interface {
	Upload(ctx context.Context, req paste.UploadFileRequest) (*paste.FileRecord, error)
	GetBySlug(ctx context.Context, slug string) (*paste.FileRecord, error)
	ServeFile(ctx context.Context, slug string, w http.ResponseWriter) error
	ValidatePassword(ctx context.Context, slug, password string) (bool, error)
	ListPublicRecent(ctx context.Context, limit int) ([]*paste.FileSummary, error)
	PresignDownloadURL(ctx context.Context, slug string, inline bool) (string, error)
}

// FileHandler handles HTTP requests for file upload and retrieval.
type FileHandler struct {
	fileService      FileService
	accessController AccessController
	// settings supplies runtime-configurable expiry options; may be nil.
	settings SettingsProvider
	// quota enforces per-IP daily upload limits; may be nil (no limit).
	quota DailyQuota
	// sizeQuota enforces global/per-IP daily upload size limits; may be nil.
	sizeQuota DailySizeQuota
}

// NewFileHandler creates a new FileHandler with the given dependencies.
func NewFileHandler(fs FileService, ac AccessController) *FileHandler {
	return &FileHandler{
		fileService:      fs,
		accessController: ac,
	}
}

// SetSettings installs a settings provider used for dynamic expiry options.
func (h *FileHandler) SetSettings(sp SettingsProvider) { h.settings = sp }

// SetQuota installs a daily quota enforcer for file uploads.
func (h *FileHandler) SetQuota(q DailyQuota) { h.quota = q }

// SetSizeQuota installs a daily size quota enforcer for file uploads.
func (h *FileHandler) SetSizeQuota(q DailySizeQuota) { h.sizeQuota = q }

// maxUploadMemory is the maximum memory used for parsing multipart forms (100 MB).
const maxUploadMemory = 100 << 20

// RegisterFileRoutes registers all file-related routes on the given chi router.
func RegisterFileRoutes(r chi.Router, h *FileHandler) {
	r.Get("/upload", h.ShowUploadForm)
	r.Post("/upload", h.HandleUpload)
	r.Get("/f/{slug}", h.GetFile)
	r.Head("/f/{slug}", h.GetFile)
	r.Get("/f/{slug}/direct", h.DirectDownload)
	r.Post("/f/{slug}/unlock", h.UnlockFile)
}

// ShowUploadForm renders the file upload form with expiry and visibility options.
func (h *FileHandler) ShowUploadForm(w http.ResponseWriter, r *http.Request) {
	maxFileSize := int64(file.MaxFileSize)
	disableFileUploads := false
	if h.settings != nil {
		maxFileSize = h.settings.Get().MaxFileSizeBytes
		disableFileUploads = h.settings.Get().DisableFileUploads
	}
	resp := map[string]interface{}{
		"expiry_options":       h.fileExpiryOptions(),
		"visibilities":         []string{"public", "unlisted", "password_protected"},
		"max_file_size":        maxFileSize,
		"disable_file_uploads": disableFileUploads,
	}
	writeJSON(w, http.StatusOK, resp)
}

// fileExpiryOptions returns the upload form's expiry options in the
// {label, duration(min)} wire shape, using dynamic settings when available and
// falling back to the built-in paste.FileExpiryOptions.
func (h *FileHandler) fileExpiryOptions() []map[string]interface{} {
	if h.settings != nil {
		opts := h.settings.Get().FileExpiryOptions
		if len(opts) > 0 {
			out := make([]map[string]interface{}, 0, len(opts))
			for _, o := range opts {
				out = append(out, map[string]interface{}{"label": o.Label, "duration": o.Minutes})
			}
			return out
		}
	}
	out := make([]map[string]interface{}, 0, len(paste.FileExpiryOptions))
	for _, o := range paste.FileExpiryOptions {
		out = append(out, map[string]interface{}{"label": o.Label, "duration": int64(o.Duration.Minutes())})
	}
	return out
}

// HandleUpload processes a multipart file upload form submission.
func (h *FileHandler) HandleUpload(w http.ResponseWriter, r *http.Request) {
	// Enforce temporary disable setting
	if h.settings != nil && h.settings.Get().DisableFileUploads {
		writeJSON(w, http.StatusForbidden, errorResponse{
			Error:  "Unggah file sedang dinonaktifkan sementara oleh administrator.",
			Code:   "UPLOADS_DISABLED",
			Status: http.StatusForbidden,
		})
		return
	}

	// Enforce per-IP daily upload limit when configured.
	if h.quota != nil && h.settings != nil {
		limit := h.settings.Get().MaxFileUploadsPerDayPerIP
		if limit > 0 {
			key := "upload:" + fileClientIP(r)
			if allowed, _ := h.quota.Allow(key, limit); !allowed {
				writeJSON(w, http.StatusTooManyRequests, errorResponse{
					Error:  "Batas unggah file harian tercapai. Coba lagi besok.",
					Code:   "DAILY_LIMIT_REACHED",
					Status: http.StatusTooManyRequests,
				})
				return
			}
		}
	}

	if err := r.ParseMultipartForm(maxUploadMemory); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{
			Error:  "Gagal memproses form upload",
			Code:   "INVALID_FORM",
			Status: http.StatusBadRequest,
		})
		return
	}

	// Extract file from form field "file".
	f, header, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{
			Error:  "File tidak ditemukan dalam form",
			Code:   "FILE_MISSING",
			Status: http.StatusBadRequest,
		})
		return
	}
	defer f.Close()

	// Enforce daily size limits (global and per-IP) when configured.
	if h.sizeQuota != nil && h.settings != nil {
		set := h.settings.Get()
		clientIP := fileClientIP(r)

		// 1. Enforce global daily upload size limit
		if set.MaxDailyUploadBytes > 0 {
			allowed, _ := h.sizeQuota.Allow("global_size", header.Size, set.MaxDailyUploadBytes)
			if !allowed {
				writeJSON(w, http.StatusTooManyRequests, errorResponse{
					Error:  "Batas total ukuran unggah berkas harian sistem telah tercapai.",
					Code:   "GLOBAL_DAILY_SIZE_LIMIT_REACHED",
					Status: http.StatusTooManyRequests,
				})
				return
			}
		}

		// 2. Enforce per-IP daily upload size limit
		if set.MaxDailyUploadBytesPerIP > 0 {
			allowed, _ := h.sizeQuota.Allow("ip_size:"+clientIP, header.Size, set.MaxDailyUploadBytesPerIP)
			if !allowed {
				writeJSON(w, http.StatusTooManyRequests, errorResponse{
					Error:  "Batas ukuran unggah berkas harian Anda telah tercapai. Coba lagi besok.",
					Code:   "IP_DAILY_SIZE_LIMIT_REACHED",
					Status: http.StatusTooManyRequests,
				})
				return
			}
		}
	}

	// Extract form values.
	visibility := paste.Visibility(r.FormValue("visibility"))
	if visibility == "" {
		visibility = paste.VisibilityPublic
	}
	password := r.FormValue("password")
	expiresInStr := r.FormValue("expires_in")

	// Parse expires_in as duration string.
	expiresIn, err := parseExpiryDuration(expiresInStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{
			Error:  "Format durasi kadaluarsa tidak valid",
			Code:   "INVALID_EXPIRY",
			Status: http.StatusBadRequest,
		})
		return
	}

	// Detect MIME type from file header.
	mimeType := header.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	req := paste.UploadFileRequest{
		Filename:   header.Filename,
		MIMEType:   mimeType,
		Size:       header.Size,
		Reader:     f,
		Visibility: visibility,
		Password:   password,
		ExpiresIn:  expiresIn,
	}

	record, err := h.fileService.Upload(r.Context(), req)
	if err != nil {
		handleFileServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"success": true,
		"slug":    record.Slug,
		"url":     fmt.Sprintf("/f/%s", record.Slug),
	})
}

// GetFile retrieves a file by slug and serves it or indicates password protection.
func (h *FileHandler) GetFile(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")

	record, err := h.fileService.GetBySlug(r.Context(), slug)
	if err != nil {
		handleFileServiceError(w, err)
		return
	}

	// If file is password protected, return 401 indicating password required.
	if record.Visibility == paste.VisibilityPasswordProtected {
		writeJSON(w, http.StatusUnauthorized, map[string]interface{}{
			"error":             "File ini dilindungi kata sandi",
			"code":              "PASSWORD_REQUIRED",
			"status":            http.StatusUnauthorized,
			"password_required": true,
			"slug":              record.Slug,
		})
		return
	}

	// Check if preview/inline is requested via query param
	inline := r.URL.Query().Get("preview") == "true" || r.URL.Query().Get("inline") == "true"
	ctx := r.Context()
	if inline {
		ctx = context.WithValue(ctx, "serve_inline", true)
	}

	// Capture and forward HTTP Range headers for media seeking
	if rangeHeader := r.Header.Get("Range"); rangeHeader != "" {
		ctx = context.WithValue(ctx, "range_header", rangeHeader)
	}

	// Serve the file.
	if r.Method == "HEAD" {
		disposition := "attachment"
		if inline {
			disposition = "inline"
		}
		w.Header().Set("Content-Type", record.MIMEType)
		w.Header().Set("Content-Disposition", fmt.Sprintf(`%s; filename="%s"`, disposition, record.Filename))
		w.Header().Set("Content-Length", strconv.FormatInt(record.SizeBytes, 10))
		w.WriteHeader(http.StatusOK)
		return
	}

	if err := h.fileService.ServeFile(ctx, slug, w); err != nil {
		log.Printf("ERROR: failed to serve file %s: %v", slug, err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{
			Error:  "Gagal menyajikan file",
			Code:   "SERVE_ERROR",
			Status: http.StatusInternalServerError,
		})
	}
}

// DirectDownload generates a presigned S3 URL and redirects the client directly
// to S3 for downloading the file, bypassing the backend streaming proxy entirely.
// Supports ?preview=true for inline display (images, videos, audio).
func (h *FileHandler) DirectDownload(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")

	record, err := h.fileService.GetBySlug(r.Context(), slug)
	if err != nil {
		handleFileServiceError(w, err)
		return
	}

	// Password-protected files cannot use direct download without unlocking.
	if record.Visibility == paste.VisibilityPasswordProtected {
		writeJSON(w, http.StatusUnauthorized, map[string]interface{}{
			"error":             "File ini dilindungi kata sandi",
			"code":              "PASSWORD_REQUIRED",
			"status":            http.StatusUnauthorized,
			"password_required": true,
			"slug":              record.Slug,
		})
		return
	}

	inline := r.URL.Query().Get("preview") == "true" || r.URL.Query().Get("inline") == "true"

	presignedURL, err := h.fileService.PresignDownloadURL(r.Context(), slug, inline)
	if err != nil {
		// Fallback: presigning failed (local storage, S3 error, etc.).
		// Serve the file directly via the streaming proxy so it still works.
		ctx := r.Context()
		if inline {
			ctx = context.WithValue(ctx, "serve_inline", true)
		}
		if rangeHeader := r.Header.Get("Range"); rangeHeader != "" {
			ctx = context.WithValue(ctx, "range_header", rangeHeader)
		}
		if serveErr := h.fileService.ServeFile(ctx, slug, w); serveErr != nil {
			log.Printf("ERROR: fallback serve failed for %s: %v", slug, serveErr)
			handleFileServiceError(w, serveErr)
		}
		return
	}

	// Redirect browser directly to the presigned S3 URL.
	http.Redirect(w, r, presignedURL, http.StatusTemporaryRedirect)
}

// UnlockFile processes a password submission to access a protected file.
func (h *FileHandler) UnlockFile(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	clientIP := fileClientIP(r)

	// Check rate limiting first.
	limited, err := h.accessController.IsRateLimited(r.Context(), clientIP, slug)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{
			Error:  "Kesalahan internal",
			Code:   "INTERNAL_ERROR",
			Status: http.StatusInternalServerError,
		})
		return
	}
	if limited {
		writeJSON(w, http.StatusTooManyRequests, errorResponse{
			Error:  "Terlalu banyak percobaan. Coba lagi nanti.",
			Code:   "RATE_LIMITED",
			Status: http.StatusTooManyRequests,
		})
		return
	}

	// Parse password from form.
	if err := r.ParseForm(); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{
			Error:  "Form tidak valid",
			Code:   "BAD_REQUEST",
			Status: http.StatusBadRequest,
		})
		return
	}

	password := r.FormValue("password")
	if password == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{
			Error:  "Kata sandi tidak boleh kosong",
			Code:   "PASSWORD_EMPTY",
			Status: http.StatusBadRequest,
		})
		return
	}

	// Validate password.
	valid, err := h.fileService.ValidatePassword(r.Context(), slug, password)
	if err != nil {
		handleFileServiceError(w, err)
		return
	}

	if !valid {
		// Record failed attempt.
		_ = h.accessController.RecordFailedAttempt(r.Context(), clientIP, slug)
		writeJSON(w, http.StatusUnauthorized, errorResponse{
			Error:  "Kata sandi salah",
			Code:   "INVALID_PASSWORD",
			Status: http.StatusUnauthorized,
		})
		return
	}

	// Password correct — reset rate limit and serve file.
	h.accessController.ResetRateLimit(r.Context(), clientIP, slug)

	ctx := r.Context()
	if rangeHeader := r.Header.Get("Range"); rangeHeader != "" {
		ctx = context.WithValue(ctx, "range_header", rangeHeader)
	}

	if err := h.fileService.ServeFile(ctx, slug, w); err != nil {
		log.Printf("ERROR: failed to serve unlocked file %s: %v", slug, err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{
			Error:  "Gagal menyajikan file",
			Code:   "SERVE_ERROR",
			Status: http.StatusInternalServerError,
		})
	}
}

// parseExpiryDuration parses an expiry duration string.
// Accepts:
//   - ""   → 0 (use service-layer default)
//   - "0"  → NeverExpires sentinel
//   - plain integer string → treated as minutes (e.g. "60" = 1 hour)
//   - Go duration string   → parsed directly (e.g. "1h", "24h")
func parseExpiryDuration(s string) (time.Duration, error) {
	if s == "" {
		return 0, nil // use default in service layer
	}

	// Try parsing as a plain integer (minutes), which is what the frontend sends.
	if minutes, err := strconv.ParseInt(s, 10, 64); err == nil {
		if minutes == 0 {
			return file.NeverExpires, nil // "Selamanya"
		}
		if minutes < 0 {
			return file.NeverExpires, nil // legacy "-1" sentinel
		}
		return time.Duration(minutes) * time.Minute, nil
	}

	// Fall back to Go duration string format (e.g. "1h", "24h").
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, err
	}
	if d <= 0 {
		return 0, fmt.Errorf("durasi harus positif atau 0 untuk selamanya")
	}
	return d, nil
}

// handleFileServiceError maps file service errors to appropriate HTTP responses.
func handleFileServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, file.ErrNotFound):
		writeJSON(w, http.StatusNotFound, errorResponse{
			Error:  "File tidak ditemukan",
			Code:   "NOT_FOUND",
			Status: http.StatusNotFound,
		})
	case errors.Is(err, file.ErrExpired):
		writeJSON(w, http.StatusGone, errorResponse{
			Error:  "File ini telah kadaluarsa",
			Code:   "RESOURCE_EXPIRED",
			Status: http.StatusGone,
		})
	case errors.Is(err, file.ErrFileTooLarge):
		writeJSON(w, http.StatusRequestEntityTooLarge, errorResponse{
			Error:  err.Error(),
			Code:   "FILE_TOO_LARGE",
			Status: http.StatusRequestEntityTooLarge,
		})
	case errors.Is(err, file.ErrPasswordRequired):
		writeJSON(w, http.StatusBadRequest, errorResponse{
			Error:  err.Error(),
			Code:   "PASSWORD_REQUIRED",
			Status: http.StatusBadRequest,
		})
	default:
		log.Printf("ERROR: unexpected internal server error: %v", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{
			Error:  "Kesalahan internal server",
			Code:   "INTERNAL_ERROR",
			Status: http.StatusInternalServerError,
		})
	}
}

// fileClientIP extracts the client IP address from the request.
func fileClientIP(r *http.Request) string {
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
