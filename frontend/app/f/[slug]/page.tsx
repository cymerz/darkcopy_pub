import Link from 'next/link';
import { notFound, redirect } from 'next/navigation';

import { getFile } from '@/lib/api';
import { APIError } from '@/lib/types';
import { formatFileSize } from '@/lib/utils';
import { ReportButton } from '@/components/ReportButton';

// File access is dynamic (per-slug, may expire or be password protected) and
// the API client fetches with `cache: 'no-store'`. Render on-demand on every
// request so visitors always see the current state of the file, matching the
// behavior of `app/[slug]/page.tsx`.
export const dynamic = 'force-dynamic';

/**
 * File metadata extracted from the backend's `ServeFile` response headers.
 *
 * The Go backend (`internal/handler/file.go` → `GetFile` → `ServeFile`) does
 * NOT perform Accept-based content negotiation: on success it streams the raw
 * file bytes with `Content-Disposition`, `Content-Type` and `Content-Length`
 * headers. There is therefore no JSON body to parse on HTTP 200 — the display
 * metadata below is read from those response headers.
 */
interface FileMetadata {
  filename: string;
  mimeType: string | null;
  sizeBytes: number | null;
  downloads: number;
}

/**
 * Extracts the filename from a `Content-Disposition` header value.
 *
 * Handles both the RFC 5987 extended form (`filename*=UTF-8''encoded`) and the
 * common quoted/bare forms (`filename="name.ext"` / `filename=name.ext`). The
 * backend emits `attachment; filename="<original name>"`.
 */
function parseContentDispositionFilename(header: string | null): string | null {
  if (!header) return null;

  // RFC 5987 extended form takes precedence when present.
  const extended = header.match(/filename\*\s*=\s*[^']*''([^;]+)/i);
  if (extended?.[1]) {
    try {
      return decodeURIComponent(extended[1].trim());
    } catch {
      // Fall through to the non-extended forms on malformed encoding.
    }
  }

  const quoted = header.match(/filename\s*=\s*"([^"]*)"/i);
  if (quoted?.[1]) return quoted[1];

  const bare = header.match(/filename\s*=\s*([^;]+)/i);
  if (bare?.[1]) return bare[1].trim();

  return null;
}

/**
 * Builds {@link FileMetadata} from a successful `getFile` response, reading the
 * filename, MIME type and size from the streaming-download response headers.
 */
function readFileMetadata(response: Response, slug: string): FileMetadata {
  const filename =
    parseContentDispositionFilename(
      response.headers.get('content-disposition'),
    ) ?? slug;

  const mimeType = response.headers.get('content-type');

  const contentLength = response.headers.get('content-length');
  const parsedSize = contentLength ? Number.parseInt(contentLength, 10) : NaN;
  const sizeBytes = Number.isFinite(parsedSize) ? parsedSize : null;

  const downloadsHeader = response.headers.get('x-downloads-count');
  const downloads = downloadsHeader ? Number.parseInt(downloadsHeader, 10) : 0;

  return { filename, mimeType, sizeBytes, downloads };
}

/**
 * Discards an unread response body. On HTTP 200 the body is the full file
 * payload; since this Server Component only needs the headers, the stream is
 * cancelled so the proxied connection is released instead of being buffered
 * into the Next.js server. The browser re-fetches the bytes via the download
 * link below.
 */
function discardBody(response: Response): void {
  void response.body?.cancel().catch(() => {
    /* Body already consumed or unavailable — nothing to clean up. */
  });
}

/**
 * Self-contained "expired file" view rendered for HTTP 410 responses.
 *
 * Kept inline (no shared component dependency) so this page can always render
 * the expired state even before the dedicated error components (task 13.x)
 * exist. Mirrors the messaging of `ExpiredPaste` in `app/[slug]/page.tsx` and
 * Req 7.2 (explanation that the content was auto-deleted, plus a "Kembali ke
 * Beranda" action).
 */
function ExpiredFile() {
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
            File Telah Kadaluarsa
          </h1>
          <p className="text-sm text-gray-500 dark:text-gray-400">
            File ini telah kadaluarsa dan dihapus otomatis oleh sistem, sehingga
            tidak lagi tersedia untuk diunduh.
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
 * Presentational file-info card with a download action (dark themed, styled to
 * match {@link PasteViewer}).
 *
 * A Server Component cannot trigger a browser blob download directly, so the
 * file metadata is displayed and the download is offered as a plain anchor
 * pointing at the `/api/f/{slug}/direct` path. That endpoint generates a
 * presigned S3 URL and returns a 302 redirect, so the browser downloads
 * directly from S3 — bypassing the backend streaming proxy entirely.
 */
/**
 * Helper to identify file category (image, video, audio) based on mime-type
 * and/or file extension for reliable preview generation.
 */
function getFileTypeCategory(filename: string, mimeType: string | null): 'image' | 'video' | 'audio' | null {
  const mime = mimeType?.toLowerCase() || '';
  if (mime.startsWith('image/')) return 'image';
  if (mime.startsWith('video/')) return 'video';
  if (mime.startsWith('audio/')) return 'audio';

  const ext = filename.split('.').pop()?.toLowerCase() || '';
  const imageExts = ['png', 'jpg', 'jpeg', 'gif', 'webp', 'svg', 'bmp', 'ico'];
  const videoExts = ['mp4', 'webm', 'ogg', 'mov', 'mkv', 'avi', 'wmv'];
  const audioExts = ['mp3', 'wav', 'ogg', 'aac', 'flac', 'm4a'];

  if (imageExts.includes(ext)) return 'image';
  if (videoExts.includes(ext)) return 'video';
  if (audioExts.includes(ext)) return 'audio';

  return null;
}

/**
 * Presentational file-info card with a download action (dark themed, styled to
 * match {@link PasteViewer}).
 *
 * A Server Component cannot trigger a browser blob download directly, so the
 * file metadata is displayed and the download is offered as a plain anchor
 * pointing at `/api/f/{slug}/direct`. That endpoint returns a 302 redirect to a
 * presigned S3 URL, so the browser downloads directly from S3.
 */
function FileInfo({
  slug,
  metadata,
}: {
  slug: string;
  metadata: FileMetadata;
}) {
  const { filename, mimeType, sizeBytes, downloads } = metadata;
  const downloadHref = `/api/f/${slug}/direct`;
  const previewHref = `/api/f/${slug}/direct?preview=true`;
  const category = getFileTypeCategory(filename, mimeType);

  return (
    <article className="mx-auto max-w-xl space-y-4">
      <header className="space-y-1">
        <h1 className="break-words text-2xl font-bold text-gray-900 dark:text-gray-100">
          {filename}
        </h1>
        <p className="text-sm text-gray-500 dark:text-gray-400">File siap diunduh.</p>
      </header>

      <div className="space-y-4 rounded-xl border border-gray-200 dark:border-dark-700 bg-white dark:bg-dark-800 p-6">
        <div className="flex items-start gap-4">
          {/* File type icon */}
          <span
            aria-hidden="true"
            className="flex h-12 w-12 shrink-0 items-center justify-center rounded-lg bg-accent/10 text-accent dark:text-accent-hover"
          >
            <svg
              xmlns="http://www.w3.org/2000/svg"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth={2}
              strokeLinecap="round"
              strokeLinejoin="round"
              className="h-6 w-6"
            >
              <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" />
              <polyline points="14 2 14 8 20 8" />
            </svg>
          </span>

          <dl className="min-w-0 flex-1 space-y-2 text-sm">
            <div className="flex flex-wrap gap-x-2">
              <dt className="text-gray-500 dark:text-gray-500">Nama:</dt>
              <dd className="min-w-0 break-words text-gray-800 dark:text-gray-200">{filename}</dd>
            </div>
            {sizeBytes != null && (
              <div className="flex flex-wrap gap-x-2">
                <dt className="text-gray-500 dark:text-gray-500">Ukuran:</dt>
                <dd className="text-gray-800 dark:text-gray-200">{formatFileSize(sizeBytes)}</dd>
              </div>
            )}
            {mimeType && (
              <div className="flex flex-wrap gap-x-2">
                <dt className="text-gray-500 dark:text-gray-500">Tipe:</dt>
                <dd className="min-w-0 break-words text-gray-800 dark:text-gray-200">
                  {mimeType}
                </dd>
              </div>
            )}
            <div className="flex flex-wrap gap-x-2">
              <dt className="text-gray-500 dark:text-gray-500">Unduhan:</dt>
              <dd className="text-gray-800 dark:text-gray-200">{downloads} kali</dd>
            </div>
          </dl>
        </div>

        {/* File Preview Section */}
        {category === 'image' && (
          <div className="overflow-hidden rounded-lg border border-gray-200 dark:border-dark-700 bg-gray-50/50 dark:bg-dark-900/40 p-2 flex justify-center items-center">
            <img
              src={previewHref}
              alt={filename}
              className="max-h-[350px] w-auto max-w-full rounded-md object-contain shadow-sm transition-transform duration-300 hover:scale-[1.01]"
              loading="lazy"
            />
          </div>
        )}

        {category === 'video' && (
          <div className="overflow-hidden rounded-lg border border-gray-200 dark:border-dark-700 bg-gray-50/50 dark:bg-dark-900/40 p-2">
            <video
              src={previewHref}
              controls
              className="w-full max-h-[350px] rounded-md object-contain bg-black"
              preload="metadata"
            >
              Browser Anda tidak mendukung preview video.
            </video>
          </div>
        )}

        {category === 'audio' && (
          <div className="overflow-hidden rounded-lg border border-gray-200 dark:border-dark-700 bg-gray-50/50 dark:bg-dark-900/40 p-4">
            <audio src={previewHref} controls className="w-full">
              Browser Anda tidak mendukung preview audio.
            </audio>
          </div>
        )}

        <a
          href={downloadHref}
          download={filename}
          className="inline-flex min-h-[44px] w-full items-center justify-center gap-2 rounded-lg bg-accent px-4 py-2.5 text-sm font-medium text-white transition-colors hover:bg-accent-hover focus:outline-none focus-visible:ring-2 focus-visible:ring-accent-hover focus-visible:ring-offset-2 focus-visible:ring-offset-dark-800"
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
            <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" />
            <polyline points="7 10 12 15 17 10" />
            <line x1="12" y1="15" x2="12" y2="3" />
          </svg>
          Unduh File
        </a>
      </div>

      {/* Report this file for admin review */}
      <div className="flex justify-center">
        <ReportButton resourceType="file" slug={slug} />
      </div>
    </article>
  );
}

/**
 * File view page (Server Component, Next.js 16 async params).
 *
 * Fetches the file resource by slug via {@link getFile} (Req 6.1) and maps the
 * backend's HTTP status to the right UX:
 *
 * - 404 → `notFound()` renders the "Tidak Ditemukan" page.
 * - 410 → renders the inline {@link ExpiredFile} view (Req 7.2).
 * - 401 → `redirect()` to the password gate `/f/${slug}/unlock` (Req 6.1). The
 *   backend returns `password_required: true` for protected files; the body is
 *   parsed for completeness but any 401 triggers the redirect.
 * - 200 → the backend streams the raw file bytes (no JSON). File metadata is
 *   read from the response headers and rendered via {@link FileInfo} with a
 *   download link to the `/api/f/${slug}/direct` path (presigned S3 redirect).
 * - anything else → re-thrown as an {@link APIError} to the nearest
 *   `app/error.tsx` boundary.
 *
 * `getFile` returns the raw {@link Response} and does NOT throw on non-2xx, so
 * status branching happens OUTSIDE the try/catch — only the network call is
 * wrapped, and `notFound()`/`redirect()` are invoked in the main control flow
 * so their NEXT_* control-flow throws propagate normally instead of being
 * swallowed.
 */
export default async function FileViewPage({
  params,
}: {
  params: Promise<{ slug: string }>;
}) {
  // Next.js 16: dynamic route params are async and must be awaited.
  const { slug } = await params;

  let response: Response;
  try {
    response = await getFile(slug);
  } catch (error) {
    // Connection failures (backend unreachable, timeout) surface to the error
    // boundary, which renders the user-facing retry UI.
    console.error(`Gagal memuat file "${slug}":`, error);
    throw error;
  }

  // Branch on status OUTSIDE the try/catch so `notFound()`/`redirect()`
  // control-flow throws are not swallowed.
  if (response.status === 404) {
    discardBody(response);
    notFound();
  }

  if (response.status === 410) {
    discardBody(response);
    return <ExpiredFile />;
  }

  if (response.status === 401) {
    // Password-protected file: gate via the unlock page (Req 6.1). The backend
    // sends `password_required: true`; consume the JSON body before redirecting.
    await response.json().catch(() => null);
    redirect(`/f/${slug}/unlock`);
  }

  if (response.status === 200) {
    // Success: the backend streams the raw file bytes (no JSON). Read display
    // metadata from the headers, release the streamed body, and render the
    // download UI.
    const metadata = readFileMetadata(response, slug);
    discardBody(response);
    return <FileInfo slug={slug} metadata={metadata} />;
  }

  // Unexpected status (e.g. 500) → surface to the error boundary with whatever
  // detail the backend provided in its JSON error body.
  const errorBody = await response.json().catch(() => null);
  throw new APIError(
    errorBody?.error ?? `Gagal memuat file (HTTP ${response.status})`,
    errorBody?.code ?? 'UNKNOWN',
    response.status,
  );
}
