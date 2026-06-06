import Link from 'next/link';
import type { ReactNode } from 'react';

/**
 * Optional call-to-action rendered as a Next.js {@link Link}.
 *
 * Using a link (rather than an `onClick` handler) is what keeps
 * {@link ErrorDisplay} a Server Component — it can be rendered from anywhere,
 * including other Server Components like `app/not-found.tsx`, without shipping
 * any client-side JavaScript.
 */
export interface ErrorDisplayAction {
  /** Visible button label, e.g. "Kembali ke Beranda". */
  label: string;
  /** Destination path passed to the Next.js `Link`, e.g. "/". */
  href: string;
}

export interface ErrorDisplayProps {
  /** Heading shown prominently, e.g. "Tidak Ditemukan". (required) */
  title: string;
  /** Supporting explanation shown beneath the title. */
  message?: string;
  /**
   * Illustrative icon rendered inside the accent circle. Should be a sized SVG
   * (e.g. `className="h-7 w-7"`) to match the surrounding layout. Defaults to a
   * generic alert (triangle) icon when omitted.
   */
  icon?: ReactNode;
  /** Optional primary action rendered as an accent link/button. */
  action?: ErrorDisplayAction;
}

/**
 * Default alert icon (triangle with exclamation) used when no `icon` prop is
 * supplied. Mirrors the icon used by the root `app/error.tsx` boundary.
 */
function DefaultAlertIcon() {
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
      <path d="M10.29 3.86 1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z" />
      <line x1="12" y1="9" x2="12" y2="13" />
      <line x1="12" y1="17" x2="12.01" y2="17" />
    </svg>
  );
}

/**
 * Reusable, dark-themed error/empty-state card (Server Component).
 *
 * Renders a centered card with an accent icon circle, a title, an optional
 * supporting message, and an optional accent action button (a Next.js `Link`).
 * The visual treatment matches the inline error UIs in `app/error.tsx`,
 * `app/[slug]/page.tsx` (`ExpiredPaste`), and `app/f/[slug]/page.tsx`
 * (`ExpiredFile`) so error surfaces stay consistent with the dark theme
 * (Req 7.5). The action link uses a `min-h-[44px]` touch target (Req 9.5).
 *
 * Designed to back the dedicated error surfaces:
 * - `app/not-found.tsx` — "Tidak Ditemukan" + "Kembali ke Beranda" (Req 7.1)
 * - expired content view — "Konten Telah Kadaluarsa" (Req 7.2)
 * - rate-limited / generic error views (Req 7.3, 7.4)
 *
 * Because the only interactivity is a navigational link, this stays a Server
 * Component (no `'use client'`), making it safe to render from other Server
 * Components.
 *
 * @example
 * ```tsx
 * <ErrorDisplay
 *   title="Tidak Ditemukan"
 *   message="Konten yang Anda cari tidak dapat ditemukan."
 *   action={{ label: 'Kembali ke Beranda', href: '/' }}
 * />
 * ```
 */
export function ErrorDisplay({
  title,
  message,
  icon,
  action,
}: ErrorDisplayProps) {
  return (
    <div
      role="alert"
      className="flex min-h-[50vh] flex-col items-center justify-center px-4 text-center"
    >
      <div className="flex w-full max-w-md flex-col items-center gap-5 rounded-xl border border-gray-200 dark:border-dark-700 bg-white dark:bg-dark-800 p-8">
        {/* Illustrative icon inside an accent circle. */}
        <span
          aria-hidden="true"
          className="flex h-14 w-14 items-center justify-center rounded-full bg-accent/10 text-accent dark:text-accent-hover"
        >
          {icon ?? <DefaultAlertIcon />}
        </span>

        <div className="space-y-2">
          <h1 className="text-xl font-semibold text-gray-900 dark:text-gray-100">{title}</h1>
          {message && <p className="text-sm text-gray-500 dark:text-gray-400">{message}</p>}
        </div>

        {action && (
          <Link
            href={action.href}
            className="inline-flex min-h-[44px] items-center justify-center gap-2 rounded-md bg-accent px-5 py-2.5 text-sm font-medium text-white transition-colors hover:bg-accent-hover focus:outline-none focus-visible:ring-2 focus-visible:ring-accent-hover focus-visible:ring-offset-2 focus-visible:ring-offset-dark-800"
          >
            {action.label}
          </Link>
        )}
      </div>
    </div>
  );
}

export default ErrorDisplay;
