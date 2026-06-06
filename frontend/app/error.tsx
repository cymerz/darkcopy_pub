'use client';

import { useEffect } from 'react';

/**
 * Root error boundary for the App Router. Error boundaries are always Client
 * Components, so this file starts with the 'use client' directive.
 *
 * This boundary primarily guards the home page paste list. Per Requirement 1.5,
 * when the paste list fails to load we surface "Gagal memuat daftar paste" along
 * with a retry control that re-runs the failed render via `reset()`.
 *
 * The UI here is intentionally self-contained (no shared component dependency)
 * to guarantee the error boundary can always render, even if other modules fail.
 */
export default function Error({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  useEffect(() => {
    // Surface the error for debugging/observability.
    console.error('Root error boundary caught an error:', error);
  }, [error]);

  return (
    <div
      role="alert"
      className="flex min-h-[50vh] flex-col items-center justify-center px-4 text-center"
    >
      <div className="flex w-full max-w-md flex-col items-center gap-5 rounded-xl border border-gray-200 dark:border-dark-700 bg-white dark:bg-dark-800 p-8">
        {/* Illustrative icon */}
        <span
          aria-hidden="true"
          className="flex h-14 w-14 items-center justify-center rounded-full bg-accent/10 text-accent dark:text-accent-hover"
        >
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
        </span>

        <div className="space-y-2">
          <h1 className="text-xl font-semibold text-gray-900 dark:text-gray-100">
            Gagal memuat daftar paste
          </h1>
          <p className="text-sm text-gray-500 dark:text-gray-400">
            Terjadi kesalahan saat memuat konten. Silakan coba lagi.
          </p>
        </div>

        <button
          type="button"
          onClick={() => reset()}
          className="inline-flex min-h-[44px] items-center justify-center gap-2 rounded-md bg-accent px-5 py-2.5 text-sm font-medium text-white transition-colors hover:bg-accent-hover focus:outline-none focus-visible:ring-2 focus-visible:ring-accent-hover focus-visible:ring-offset-2 focus-visible:ring-offset-dark-800"
        >
          <svg
            xmlns="http://www.w3.org/2000/svg"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth={2}
            strokeLinecap="round"
            strokeLinejoin="round"
            className="h-4 w-4"
            aria-hidden="true"
          >
            <path d="M23 4v6h-6" />
            <path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10" />
          </svg>
          Coba Lagi
        </button>
      </div>
    </div>
  );
}
