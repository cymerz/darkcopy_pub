import { ErrorDisplay } from '@/components/ErrorDisplay';

/**
 * Props for {@link ExpiredPage}.
 *
 * Both fields are optional so the component can be dropped in with zero config
 * for the default "konten" (content) wording (Req 7.2), or customized for a
 * specific resource type — e.g. "Paste Telah Kadaluarsa" (Req 3.9) or a
 * file-specific variant — without re-wiring the underlying {@link ErrorDisplay}.
 */
export interface ExpiredPageProps {
  /**
   * Heading shown to the user. Defaults to "Konten Telah Kadaluarsa" per
   * Req 7.2.
   */
  title?: string;
  /**
   * Supporting explanation that the content was deleted automatically by the
   * system. Defaults to a generic Indonesian explanation per Req 7.2.
   */
  message?: string;
}

/** Default heading required by Req 7.2. */
const DEFAULT_TITLE = 'Konten Telah Kadaluarsa';

/**
 * Default explanation that the content has expired and was deleted
 * automatically by the system (Req 7.2).
 */
const DEFAULT_MESSAGE =
  'Konten ini telah kadaluarsa dan dihapus otomatis oleh sistem, sehingga tidak lagi tersedia.';

/**
 * Clock icon illustrating expiry. Sized to match the accent circle used by
 * {@link ErrorDisplay} (`h-7 w-7`).
 */
function ClockIcon() {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth={2}
      strokeLinecap="round"
      strokeLinejoin="round"
      className="h-7 w-7"
    >
      <circle cx="12" cy="12" r="9" />
      <polyline points="12 7 12 12 16 14" />
    </svg>
  );
}

/**
 * Error surface shown when the backend returns HTTP 410 (resource expired).
 *
 * This is a thin, presentational wrapper around {@link ErrorDisplay} that
 * supplies the expiry-specific copy and a clock icon. Per Req 7.2 it shows the
 * title "Konten Telah Kadaluarsa", an explanation that the content was deleted
 * automatically, and a "Kembali ke Beranda" action link.
 *
 * Like {@link ErrorDisplay}, this stays a Server Component (no `'use client'`)
 * because the only interactivity is a navigational link — so it is safe to
 * render directly from the paste/file view Server Components on a 410 response
 * (e.g. `app/[slug]/page.tsx` and `app/f/[slug]/page.tsx`, which currently use
 * inline expired components and could later be refactored to render this).
 *
 * @example
 * ```tsx
 * // Default content wording (Req 7.2)
 * <ExpiredPage />
 *
 * // Paste-specific wording (Req 3.9)
 * <ExpiredPage title="Paste Telah Kadaluarsa" />
 * ```
 */
export function ExpiredPage({
  title = DEFAULT_TITLE,
  message = DEFAULT_MESSAGE,
}: ExpiredPageProps = {}) {
  return (
    <ErrorDisplay
      title={title}
      message={message}
      icon={<ClockIcon />}
      action={{ label: 'Kembali ke Beranda', href: '/' }}
    />
  );
}

export default ExpiredPage;
