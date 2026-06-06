'use client';

import { useState } from 'react';
import { CopyButton } from '@/components/CopyButton';
import { CountdownTimer } from '@/components/CountdownTimer';
import { ReportButton } from '@/components/ReportButton';
import type { PasteViewResponse } from '@/lib/types';
import { formatRelativeTime, getFileExtension } from '@/lib/utils';

interface PasteViewerProps {
  paste: PasteViewResponse;
}

function buildDownloadFilename(title: string, slug: string, extension: string): string {
  const base = title.trim() || `paste-${slug}`;
  const sanitized = base
    .replace(/[\\/:*?"<>|]+/g, '')
    .replace(/\s+/g, '_')
    .replace(/^[._]+|[._]+$/g, '');
  const safeBase = sanitized || `paste-${slug}`;
  return `${safeBase}${extension}`;
}

export function PasteViewer({ paste }: PasteViewerProps) {
  const title = paste.title.trim() || 'Untitled';
  const [showHighlighting, setShowHighlighting] = useState(true);

  const lineCount = Math.max(1, paste.content.split('\n').length);
  const lineNumbers = Array.from({ length: lineCount }, (_, i) => i + 1);

  const handleDownload = (): void => {
    const extension = getFileExtension(paste.language);
    const filename = buildDownloadFilename(paste.title, paste.slug, extension);
    const blob = new Blob([paste.content], { type: 'text/plain;charset=utf-8' });
    const url = URL.createObjectURL(blob);
    const anchor = document.createElement('a');
    anchor.href = url;
    anchor.download = filename;
    document.body.appendChild(anchor);
    anchor.click();
    document.body.removeChild(anchor);
    URL.revokeObjectURL(url);
  };

  return (
    <article className="space-y-5">
      {/* Metadata header */}
      <header className="space-y-3">
        <h1 className="break-words text-2xl md:text-3xl font-bold text-gray-900 dark:text-gray-100">
          {title}
        </h1>

        <div className="flex flex-wrap items-center gap-x-3 gap-y-2 text-sm text-gray-500 dark:text-gray-400">
          <span className="inline-flex items-center rounded-md border border-accent/30 bg-accent/10 px-2 py-0.5 text-xs font-medium text-accent dark:text-accent-hover">
            {paste.language}
          </span>

          <time dateTime={paste.created_at} title={paste.created_at}>
            {formatRelativeTime(paste.created_at)}
          </time>

          {paste.remaining_seconds != null ? (
            <CountdownTimer remainingSeconds={paste.remaining_seconds} />
          ) : paste.expires_at ? (
            <span title={paste.expires_at}>
              Kadaluarsa: {formatRelativeTime(paste.expires_at)}
            </span>
          ) : (
            <span>Tidak pernah kadaluarsa</span>
          )}

          <span className="text-gray-300 dark:text-dark-600" aria-hidden="true">•</span>

          <span className="inline-flex items-center gap-1.5">
            <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" className="w-3.5 h-3.5" aria-hidden="true">
              <path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z" />
              <circle cx="12" cy="12" r="3" />
            </svg>
            <span>Dilihat {paste.views ?? 0} kali</span>
          </span>
        </div>
      </header>

      {/* Action toolbar — grouped buttons in a clean bar */}
      <div className="flex flex-wrap items-center gap-1.5 rounded-lg border border-gray-200 dark:border-dark-700 bg-gray-50 dark:bg-dark-800/50 p-1.5">
        <CopyButton content={paste.content} />

        <button
          type="button"
          onClick={handleDownload}
          title="Unduh"
          className="inline-flex min-h-[36px] items-center justify-center gap-1.5 rounded-md border border-gray-200 dark:border-dark-600 bg-white dark:bg-dark-800 px-3 py-1.5 text-xs font-medium text-gray-700 dark:text-gray-300 transition-all hover:bg-gray-50 dark:hover:bg-dark-700 hover:border-gray-300 dark:hover:border-dark-500"
        >
          <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" className="w-3.5 h-3.5" aria-hidden="true">
            <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" />
            <polyline points="7 10 12 15 17 10" />
            <line x1="12" y1="15" x2="12" y2="3" />
          </svg>
          Unduh
        </button>

        <a
          href={`/api/raw/${paste.slug}`}
          target="_blank"
          rel="noopener noreferrer"
          title="Lihat raw"
          className="inline-flex min-h-[36px] items-center justify-center gap-1.5 rounded-md border border-gray-200 dark:border-dark-600 bg-white dark:bg-dark-800 px-3 py-1.5 text-xs font-medium text-gray-700 dark:text-gray-300 transition-all hover:bg-gray-50 dark:hover:bg-dark-700 hover:border-gray-300 dark:hover:border-dark-500"
        >
          <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" className="w-3.5 h-3.5" aria-hidden="true">
            <polyline points="4 7 4 4 20 4 20 7" />
            <line x1="9" y1="20" x2="15" y2="20" />
            <line x1="12" y1="4" x2="12" y2="20" />
          </svg>
          Raw
        </a>

        {/* Separator */}
        <div className="hidden sm:block mx-1 h-5 w-px bg-gray-200 dark:bg-dark-600" aria-hidden="true" />

        {/* Syntax highlighting toggle */}
        <button
          type="button"
          onClick={() => setShowHighlighting((v) => !v)}
          title={showHighlighting ? 'Matikan syntax highlighting' : 'Nyalakan syntax highlighting'}
          aria-pressed={showHighlighting}
          className={`inline-flex min-h-[36px] items-center justify-center gap-1.5 rounded-md border px-3 py-1.5 text-xs font-medium transition-all ${
            showHighlighting
              ? 'border-accent/40 bg-accent/10 text-accent dark:text-accent-hover'
              : 'border-gray-200 dark:border-dark-600 bg-white dark:bg-dark-800 text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-dark-700'
          }`}
        >
          <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" className="w-3.5 h-3.5" aria-hidden="true">
            <polyline points="16 18 22 12 16 6" />
            <polyline points="8 6 2 12 8 18" />
          </svg>
          {showHighlighting ? 'Highlighted' : 'Plain Text'}
        </button>

        {/* Push the report button to the far right */}
        <div className="sm:ml-auto">
          <ReportButton resourceType="paste" slug={paste.slug} compact />
        </div>
      </div>

      {/* Code content with line-number gutter.
          When syntax highlighting is ON, the panel stays dark in BOTH themes so
          the backend's dark-tuned (dracula) colors remain readable on light
          pages. In plain-text mode the panel follows the active theme. */}
      <div
        className={`flex overflow-hidden rounded-lg border ${
          showHighlighting
            ? 'border-dark-700 bg-dark-800'
            : 'border-gray-200 dark:border-dark-700 bg-white dark:bg-dark-800'
        }`}
      >
        <div
          aria-hidden="true"
          className={`shrink-0 select-none border-r px-3 py-4 text-right font-mono text-xs leading-6 ${
            showHighlighting
              ? 'border-dark-700 bg-dark-900/60 text-gray-500'
              : 'border-gray-200 dark:border-dark-700 bg-gray-50 dark:bg-dark-900/60 text-gray-400 dark:text-gray-500'
          }`}
        >
          {lineNumbers.map((n) => (
            <div key={n}>{n}</div>
          ))}
        </div>

        <div
          className={`min-w-0 flex-1 overflow-x-auto px-4 py-4 font-mono text-sm leading-6 ${
            showHighlighting
              ? 'text-gray-100'
              : 'text-gray-900 dark:text-gray-100'
          }`}
        >
          {showHighlighting ? (
            <div className="darkcopy-code" dangerouslySetInnerHTML={{ __html: paste.highlighted_html }} />
          ) : (
            <pre className="whitespace-pre">{paste.content}</pre>
          )}
        </div>
      </div>
    </article>
  );
}

export default PasteViewer;
