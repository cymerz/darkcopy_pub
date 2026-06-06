// components/PasteList.tsx
//
// Server Component that renders a list of recent public pastes. Each paste is
// shown as a dark card linking to its view page (`/{slug}`) via Next.js
// client-side navigation. No interactivity is required, so this stays a
// Server Component (no 'use client').

import Link from 'next/link';
import type { PasteSummary } from '@/lib/types';
import { formatRelativeTime } from '@/lib/utils';

interface PasteListProps {
  pastes: PasteSummary[];
}

/**
 * Renders recent public pastes as a vertical stack of cards. When the list is
 * empty, a friendly Indonesian empty-state message is shown instead.
 *
 * Each card displays:
 * - the paste title (falling back to "Untitled" when empty/whitespace)
 * - a language badge styled with the accent color
 * - the relative creation time via `formatRelativeTime(created_at)`
 *
 * Tapping a card navigates to `/{slug}` using Next.js `Link` (client-side
 * navigation, Requirement 1.3).
 */
export function PasteList({ pastes }: PasteListProps) {
  if (!pastes || pastes.length === 0) {
    return (
      <div
        className="rounded-xl border border-dashed border-gray-200 dark:border-dark-700 bg-gray-50 dark:bg-dark-800/50 px-6 py-12 text-center"
        role="status"
      >
        <p className="text-gray-700 dark:text-gray-300">Belum ada paste publik</p>
        <p className="mt-1 text-sm text-gray-500 dark:text-gray-500">
          Paste publik terbaru akan muncul di sini.
        </p>
      </div>
    );
  }

  return (
    <ul className="flex flex-col gap-3">
      {pastes.map((paste) => {
        const title = paste.title.trim() || 'Untitled';

        return (
          <li key={paste.slug}>
            <Link
              href={`/${paste.slug}`}
              className="group block rounded-xl border border-gray-200 dark:border-dark-700 bg-white dark:bg-dark-800 transition-colors hover:border-accent/60 hover:bg-gray-100 dark:hover:bg-dark-700"
            >
              <div className="flex items-center justify-between gap-4 p-4 min-h-[44px]">
                <h2 className="min-w-0 flex-1 truncate font-medium text-gray-900 dark:text-gray-100 transition-colors group-hover:text-gray-900 dark:group-hover:text-white">
                  {title}
                </h2>
                <div className="flex shrink-0 items-center gap-3">
                  <span className="rounded-full bg-accent/10 dark:bg-accent/15 px-2.5 py-0.5 text-xs font-medium text-accent dark:text-accent-hover">
                    {paste.language}
                  </span>
                  <time
                    dateTime={paste.created_at}
                    className="text-sm text-gray-500 dark:text-gray-400"
                  >
                    {formatRelativeTime(paste.created_at)}
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

export default PasteList;
