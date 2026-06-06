// lib/types.ts
//
// Shared TypeScript type definitions for the DarkCopy frontend.
//
// Field names mirror the Go backend's JSON output exactly. The backend
// uses snake_case for most fields (e.g. `created_at`, `expires_at`,
// `highlighted_html`, `remaining_seconds`, `password_required`,
// `expiry_options`), but emits `expiryOptions` (camelCase) for the
// `GET /new` endpoint — both spellings are preserved here so that
// responses can be consumed without remapping.

/**
 * Summary of a paste used in list responses (e.g. recent public pastes).
 */
export interface PasteSummary {
  slug: string;
  title: string;
  language: string;
  created_at: string;
  expires_at: string | null;
}

/**
 * Summary of a file used in list responses (e.g. recent public files).
 */
export interface FileSummary {
  slug: string;
  filename: string;
  mime_type: string;
  size_bytes: number;
  created_at: string;
  expires_at: string | null;
}

/**
 * Response shape for `GET /` returning recent public pastes and files.
 */
export interface PasteListResponse {
  pastes: PasteSummary[];
  files: FileSummary[];
}

/**
 * Programming language option offered by the backend.
 */
export interface Language {
  id: string;
  name: string;
}

/**
 * Expiry option for pastes/files — duration is expressed in minutes.
 */
export interface ExpiryOption {
  label: string;
  /** Duration in minutes. */
  duration: number;
}

/**
 * Response shape for `GET /new` (paste creation form options).
 *
 * Note: backend uses camelCase `expiryOptions` for this endpoint.
 */
export interface NewPasteOptions {
  languages: Language[];
  expiryOptions: ExpiryOption[];
  disable_new_pastes?: boolean;
}

/**
 * Response shape for `GET /{slug}` (paste view).
 */
export interface PasteViewResponse {
  slug: string;
  title: string;
  content: string;
  highlighted_html: string;
  language: string;
  visibility: 'public' | 'unlisted' | 'password_protected';
  created_at: string;
  expires_at: string | null;
  remaining_seconds: number | null;
  views: number;
}

/**
 * Response shape returned with HTTP 401 when a paste/file is password
 * protected and a password is required to access the content.
 */
export interface PasswordRequiredResponse {
  password_required: true;
  slug: string;
  error: string;
  code: string;
  status: number;
}

/**
 * Response shape for `GET /upload` (file upload form options).
 *
 * Note: backend uses snake_case `expiry_options` for this endpoint.
 */
export interface UploadOptions {
  expiry_options: ExpiryOption[];
  visibilities: string[];
  max_file_size?: number;
  disable_file_uploads?: boolean;
}

/**
 * Response shape for a successful `POST /upload` request.
 */
export interface UploadResponse {
  success: boolean;
  slug: string;
  url: string;
}

/**
 * Generic error response shape returned by the backend for non-2xx
 * responses.
 */
export interface APIErrorResponse {
  error: string;
  code: string;
  status: number;
}

// ---------------------------------------------------------------------------
// Admin (hidden) API types
//
// These back the hidden /admin dashboard. They are not linked from the main
// navigation and require an admin token (sent as the `X-Admin-Token` header).
// Field names mirror the Go backend's JSON output (snake_case).
// ---------------------------------------------------------------------------

export interface ProviderStats {
  provider_name: string;
  files_count: number;
  size_bytes: number;
}

/**
 * Aggregate counts returned by `GET /admin/stats`.
 */
export interface AdminStats {
  total_pastes: number;
  total_files: number;
  total_bytes?: number;
  pending_reports: number;
  provider_stats?: ProviderStats[];
  top_pastes?: AdminPasteItem[];
  top_files?: AdminFileItem[];
}

/**
 * Admin-facing view of a paste returned by `GET /admin/pastes`. Includes every
 * paste regardless of visibility or expiry.
 */
export interface AdminPasteItem {
  slug: string;
  title: string;
  language: string;
  visibility: 'public' | 'unlisted' | 'password_protected';
  has_password: boolean;
  created_at: string;
  expires_at: string | null;
  views: number;
}

/**
 * Admin-facing view of an uploaded file returned by `GET /admin/files`.
 */
export interface AdminFileItem {
  slug: string;
  filename: string;
  mime_type: string;
  size_bytes: number;
  visibility: 'public' | 'unlisted' | 'password_protected';
  has_password: boolean;
  created_at: string;
  expires_at: string | null;
  downloads: number;
}

/**
 * Response shape for `GET /admin/pastes`.
 */
export interface AdminPasteListResponse {
  pastes: AdminPasteItem[];
}

/**
 * Response shape for `GET /admin/files`.
 */
export interface AdminFileListResponse {
  files: AdminFileItem[];
}

/**
 * Error class thrown by the API client when the backend returns a
 * non-2xx response. Carries the backend's error code and HTTP status
 * for granular handling in callers.
 */
export class APIError extends Error {
  code: string;
  status: number;

  constructor(message: string, code: string, status: number) {
    super(message);
    this.name = 'APIError';
    this.code = code;
    this.status = status;
  }
}

// ---------------------------------------------------------------------------
// Admin settings types (mirror Go internal/settings.Settings JSON)
// ---------------------------------------------------------------------------

/** A configurable expiry option (duration in minutes; 0 = never for pastes). */
export interface AdminExpiryOption {
  label: string;
  minutes: number;
}

/** Runtime-configurable application settings, editable from the admin panel. */
export interface AdminSettings {
  max_paste_size_bytes: number;
  max_file_size_bytes: number;
  paste_expiry_options: AdminExpiryOption[];
  file_expiry_options: AdminExpiryOption[];
  max_pastes_per_day_per_ip: number;
  max_file_uploads_per_day_per_ip: number;
  disable_new_pastes?: boolean;
  disable_file_uploads?: boolean;
  max_daily_upload_bytes?: number;
  max_daily_upload_bytes_per_ip?: number;
}

// ---------------------------------------------------------------------------
// Report types (mirror Go internal/report.Report JSON)
// ---------------------------------------------------------------------------

/** Resource a report targets. */
export type ReportResourceType = 'paste' | 'file';

/** Review state of a report. */
export type ReportStatus = 'pending' | 'reviewed' | 'dismissed';

/** A single abuse/content report as returned by `GET /admin/reports`. */
export interface AdminReport {
  id: string;
  resource_type: ReportResourceType;
  slug: string;
  reason: string;
  details: string;
  reporter_ip: string;
  status: ReportStatus;
  created_at: string;
  reviewed_at: string | null;
}

/** Response shape for `GET /admin/reports`. */
export interface AdminReportListResponse {
  reports: AdminReport[];
}

/** Canonical report reasons offered to users (must match the backend). */
export const REPORT_REASONS: { value: string; label: string }[] = [
  { value: 'spam', label: 'Spam' },
  { value: 'illegal', label: 'Konten Ilegal' },
  { value: 'malware', label: 'Malware / Berbahaya' },
  { value: 'copyright', label: 'Pelanggaran Hak Cipta' },
  { value: 'personal_info', label: 'Informasi Pribadi' },
  { value: 'other', label: 'Lainnya' },
];
