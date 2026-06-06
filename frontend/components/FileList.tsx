// components/FileList.tsx
//
// Server Component that renders a list of recent public files. Each file is
// shown as a polished card linking to its view page (`/f/{slug}`) via Next.js
// client-side navigation.
//

import Link from 'next/link';
import type { FileSummary } from '@/lib/types';
import { formatRelativeTime, formatFileSize } from '@/lib/utils';

interface FileListProps {
  files: FileSummary[];
}

/**
 * Renders recent public files as a responsive grid or stack of cards.
 * When the list is empty, a friendly Indonesian empty-state message is shown.
 *
 * Each card displays:
 * - the file name (truncated elegantly if too long)
 * - the formatted file size via `formatFileSize(size_bytes)`
 * - the MIME type or extension badge
 * - the relative creation time via `formatRelativeTime(created_at)`
 *
 * Tapping a card navigates to `/f/{slug}` using Next.js `Link` for fast
 * client-side navigation.
 */
export function FileList({ files }: FileListProps) {
  if (!files || files.length === 0) {
    return (
      <div
        className="rounded-xl border border-dashed border-gray-200 dark:border-dark-700 bg-gray-50 dark:bg-dark-800/50 px-6 py-12 text-center"
        role="status"
      >
        <p className="text-gray-700 dark:text-gray-300">Belum ada file publik</p>
        <p className="mt-1 text-sm text-gray-500 dark:text-gray-500">
          File publik terbaru akan muncul di sini.
        </p>
      </div>
    );
  }

  // Get a simplified display type from MIME type
  const getFileCategory = (mime: string): string => {
    if (!mime) return 'FILE';
    const parts = mime.split('/');
    if (parts.length < 2) return 'FILE';

    const sub = parts[1].toUpperCase();
    if (sub.includes('PDF')) return 'PDF';
    if (sub.includes('PNG') || sub.includes('JPEG') || sub.includes('JPG') || sub.includes('GIF') || sub.includes('WEBP')) return 'IMAGE';
    if (sub.includes('ZIP') || sub.includes('RAR') || sub.includes('TAR') || sub.includes('GZ') || sub.includes('7Z')) return 'ARCHIVE';
    if (sub.includes('JSON') || sub.includes('XML') || sub.includes('YAML') || sub.includes('JAVASCRIPT') || sub.includes('TYPESCRIPT')) return 'CODE';
    if (parts[0] === 'text') return 'TEXT';

    return sub.length > 5 ? sub.substring(0, 5) : sub;
  };

  return (
    <ul className="flex flex-col gap-3">
      {files.map((file) => {
        const displayName = file.filename.trim() || 'Unnamed File';
        const fileCategory = getFileCategory(file.mime_type);

        return (
          <li key={file.slug}>
            <Link
              href={`/f/${file.slug}`}
              className="group block rounded-xl border border-gray-200 dark:border-dark-700 bg-white dark:bg-dark-800 transition-colors hover:border-accent/60 hover:bg-gray-100 dark:hover:bg-dark-700"
            >
              <div className="flex items-center justify-between gap-4 p-4 min-h-[44px]">
                <div className="flex items-center gap-3 min-w-0 flex-1">
                  {/* File Icon placeholder */}
                  <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-gray-100 dark:bg-dark-700 text-gray-500 dark:text-gray-400 group-hover:bg-accent/15 group-hover:text-accent transition-colors">
                    <svg
                      className="h-5 w-5"
                      fill="none"
                      viewBox="0 0 24 24"
                      stroke="currentColor"
                      strokeWidth="2"
                    >
                      <path
                        strokeLinecap="round"
                        strokeLinejoin="round"
                        d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"
                      />
                    </svg>
                  </span>

                  <div className="min-w-0 flex-1">
                    <h2 className="truncate font-medium text-gray-900 dark:text-gray-100 transition-colors group-hover:text-gray-900 dark:group-hover:text-white">
                      {displayName}
                    </h2>
                    <p className="text-xs text-gray-500 dark:text-gray-400 mt-0.5">
                      {formatFileSize(file.size_bytes)}
                    </p>
                  </div>
                </div>

                <div className="flex shrink-0 items-center gap-3">
                  <span className="rounded-full bg-accent/10 dark:bg-accent/15 px-2.5 py-0.5 text-xs font-medium text-accent dark:text-accent-hover">
                    {fileCategory}
                  </span>
                  <time
                    dateTime={file.created_at}
                    className="text-sm text-gray-500 dark:text-gray-400"
                  >
                    {formatRelativeTime(file.created_at)}
                  </time>
                </div>
              </div>
            </Link>
          </li>
        );
      })}
    </ul>
  );
}

export default FileList;
