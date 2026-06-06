package handler

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/gthbn/pastebin/internal/report"
)

// ReportService defines the report operations used by the public handler.
type ReportService interface {
	Create(ctx context.Context, req report.CreateReportRequest) (*report.Report, error)
}

// ReportHandler handles the public report-submission endpoint.
type ReportHandler struct {
	service ReportService
	// quota optionally limits reports per IP per day; may be nil (no limit).
	quota DailyQuota
	// dailyLimit is the per-IP report cap when quota is set (0 disables it).
	dailyLimit int
}

// NewReportHandler creates a new ReportHandler.
func NewReportHandler(s ReportService) *ReportHandler {
	return &ReportHandler{service: s}
}

// SetQuota installs a daily quota enforcer with the given per-IP daily limit.
func (h *ReportHandler) SetQuota(q DailyQuota, dailyLimit int) {
	h.quota = q
	h.dailyLimit = dailyLimit
}

// RegisterReportRoutes registers the public report route.
func (h *ReportHandler) registerRoutes(r chi.Router) {
	r.Post("/report", h.HandleCreate)
}

// RegisterReportRoutes mounts the public report endpoint on the router.
func RegisterReportRoutes(r chi.Router, h *ReportHandler) {
	h.registerRoutes(r)
}

// HandleCreate accepts an abuse/content report for a paste or file.
func (h *ReportHandler) HandleCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Form tidak valid", "BAD_REQUEST")
		return
	}

	clientIP := extractIP(r)

	// Per-IP daily limit to curb report spam.
	if h.quota != nil && h.dailyLimit > 0 {
		if allowed, _ := h.quota.Allow("report:"+clientIP, h.dailyLimit); !allowed {
			writeJSONError(w, http.StatusTooManyRequests, "Batas laporan harian tercapai. Coba lagi besok.", "DAILY_LIMIT_REACHED")
			return
		}
	}

	resourceType := report.ResourceType(r.FormValue("resource_type"))
	req := report.CreateReportRequest{
		ResourceType: resourceType,
		Slug:         r.FormValue("slug"),
		Reason:       r.FormValue("reason"),
		Details:      r.FormValue("details"),
		ReporterIP:   clientIP,
	}

	if _, err := h.service.Create(r.Context(), req); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error(), "VALIDATION_ERROR")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"success": true,
		"message": "Laporan terkirim. Terima kasih.",
	})
}
