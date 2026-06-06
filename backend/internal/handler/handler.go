// Package handler provides HTTP handlers for the pastebin application.
package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gthbn/pastebin/internal/highlight"
	"github.com/gthbn/pastebin/internal/paste"
	"github.com/gthbn/pastebin/internal/settings"
)

// PasteService defines the interface for paste operations used by the handler.
type PasteService interface {
	Create(ctx context.Context, req paste.CreatePasteRequest) (*paste.Paste, error)
	GetBySlug(ctx context.Context, slug string) (*paste.Paste, error)
	ValidatePassword(ctx context.Context, slug, password string) (bool, error)
	ListPublicRecent(ctx context.Context, limit int) ([]*paste.PasteSummary, error)
	IncrementViews(ctx context.Context, slug string) error
}

// SyntaxHighlighter defines the interface for syntax highlighting used by the handler.
type SyntaxHighlighter interface {
	Highlight(content, language string) (html string, err error)
	SupportedLanguages() []highlight.Language
}

// AccessController defines the interface for access control and rate limiting used by the handler.
type AccessController interface {
	CheckAccess(ctx context.Context, resourceID string, password string) (AccessResult, error)
	RecordFailedAttempt(ctx context.Context, ip string, resourceID string) error
	IsRateLimited(ctx context.Context, ip string, resourceID string) (bool, error)
	ResetRateLimit(ctx context.Context, ip string, resourceSlug string)
}

// SettingsProvider supplies runtime-configurable settings to the handlers.
// It is satisfied by *settings.Provider.
type SettingsProvider interface {
	Get() settings.Settings
}

// DailyQuota enforces per-key daily action limits. It is satisfied by
// *quota.Counter. Allow records an action and reports whether it was permitted.
type DailyQuota interface {
	Allow(key string, limit int) (allowed bool, remaining int)
}

// DailySizeQuota enforces daily total upload size limits. It is satisfied by
// *quota.SizeCounter. Allow records an action size and reports whether it was permitted.
type DailySizeQuota interface {
	Allow(key string, size int64, limitBytes int64) (allowed bool, remaining int64)
}

// AccessResult represents the outcome of an access check.
type AccessResult int

const (
	AccessGranted AccessResult = iota
	AccessDenied
)

// errorResponse is a standard error response structure.
type errorResponse struct {
	Error  string `json:"error"`
	Code   string `json:"code"`
	Status int    `json:"status"`
}

// writeJSON writes a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// wantsJSON returns true if the request's Accept header contains "application/json".
func wantsJSON(r *http.Request) bool {
	return strings.Contains(r.Header.Get("Accept"), "application/json")
}

// writeJSONError writes a JSON error response.
func writeJSONError(w http.ResponseWriter, status int, message, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(errorResponse{
		Error:  message,
		Code:   code,
		Status: status,
	})
}
