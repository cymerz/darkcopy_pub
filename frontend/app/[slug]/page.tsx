import Link from 'next/link';
import { notFound, redirect } from 'next/navigation';

import { getPaste } from '@/lib/api';
import { APIError, type PasteViewResponse } from '@/lib/types';
import { PasteViewer } from '@/components/PasteViewer';

// Paste content is dynamic (per-slug, may expire), and the API client fetches
// with `cache: 'no-store'`. Render on-demand on every request so visitors always
// see the current state of the paste rather than a cached snapshot.
export const dynamic = 'force-dynamic';

/**
 * Self-contained "expired paste" view rendered for HTTP 410 responses.
 *
 * Kept inline (no shared component dependency) so this page can always render
 * the expired state even before the dedicated error components (task 13.x)
 * exist. Task 13.3 will refine the canonical expired page; the messaging here
 * mirrors Req 3.9 ("Paste Telah Kadaluarsa") and Req 7.2 (explanation that the
 * content was auto-deleted, plus a "Kembali ke Beranda" action).
 */
function ExpiredPaste() {
  return (
    <div
      role="alert"
      className="flex min-h-[50vh] flex-col items-center justify-center px-4 text-center"
    >
      <div className="flex w-full max-w-md flex-col items-center gap-5 rounded-xl border border-gray-200 dark:border-dark-700 bg-white dark:bg-dark-800 p-8">
        {/* Illustrative clock icon */}
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
            <circle cx="12" cy="12" r="10" />
            <polyline points="12 6 12 12 16 14" />
          </svg>
        </span>

        <div className="space-y-2">
          <h1 className="text-xl font-semibold text-gray-900 dark:text-gray-100">
            Paste Telah Kadaluarsa
          </h1>
          <p className="text-sm text-gray-500 dark:text-gray-400">
            Konten ini telah kadaluarsa dan dihapus otomatis oleh sistem,
            sehingga tidak lagi tersedia.
          </p>
        </div>

        <Link
          href="/"
          className="inline-flex min-h-[44px] items-center justify-center gap-2 rounded-md bg-accent px-5 py-2.5 text-sm font-medium text-white transition-colors hover:bg-accent-hover focus:outline-none focus-visible:ring-2 focus-visible:ring-accent-hover focus-visible:ring-offset-2 focus-visible:ring-offset-dark-800"
        >
          Kembali ke Beranda
        </Link>
      </div>
    </div>
  );
}

/**
 * Paste view page (Server Component, Next.js 16 async params).
 *
 * Fetches a paste by slug via {@link getPaste} (Req 3.1) and renders it through
 * {@link PasteViewer}. Error handling maps backend HTTP statuses to the right
 * UX per the requirements:
 *
 * - 404 → `notFound()` renders the "Tidak Ditemukan" page (Req 3.8).
 * - 410 → renders the inline expired view (Req 3.9).
 * - 401 → `redirect()` to the password gate `/${slug}/unlock` (Req 3.10). The
 *   API client throws {@link APIError} on non-2xx (discarding the body), so the
 *   401 status alone is treated as the `password_required` signal here.
 * - anything else → re-thrown to the nearest `app/error.tsx` boundary.
 *
 * The fetch is the only thing wrapped in try/catch. `notFound()`/`redirect()`
 * are invoked inside the catch (not the try) so the NEXT_* control-flow errors
 * they throw propagate normally instead of being swallowed; the catch only
 * handles {@link APIError} and re-throws everything else.
 */
export default async function PasteViewPage({
  params,
}: {
  params: Promise<{ slug: string }>;
}) {
  // Next.js 16: dynamic route params are async and must be awaited.
  const { slug } = await params;

  let paste: PasteViewResponse;
  try {
    paste = await getPaste(slug);
  } catch (error) {
    if (error instanceof APIError) {
      if (error.status === 404) notFound();
      if (error.status === 410) return <ExpiredPaste />;
      // 401 indicates a password-protected paste (Req 3.10). apiFetch discards
      // the response body, so the status is the signal to gate via /unlock.
      if (error.status === 401) redirect(`/${slug}/unlock`);
    }
    // Non-API errors (network failures) and unexpected statuses (e.g. 500)
    // surface to the error boundary.
    throw error;
  }

  return <PasteViewer paste={paste} />;
}
