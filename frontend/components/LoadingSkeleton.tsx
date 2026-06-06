import type { HTMLAttributes } from 'react';

/**
 * Generic skeleton primitive. Renders a pulsing dark block that callers can
 * size and shape via Tailwind utility classes.
 */
export function Skeleton({
  className = '',
  ...props
}: HTMLAttributes<HTMLDivElement>) {
  const base = 'animate-pulse bg-gray-200 dark:bg-dark-700 rounded';
  return <div className={`${base} ${className}`.trim()} {...props} />;
}

/**
 * Skeleton placeholder for the recent-paste list shown on the home page.
 * Renders five card-shaped placeholders that mirror the real PasteList items:
 * title bar, language badge, and timestamp shape.
 */
export function PasteListSkeleton() {
  // Vary widths slightly so the placeholder feels organic rather than uniform.
  const titleWidths = ['w-1/2', 'w-3/5', 'w-2/5', 'w-2/3', 'w-1/3'];

  return (
    <div
      className="space-y-3"
      role="status"
      aria-label="Memuat daftar paste"
    >
      {titleWidths.map((width, idx) => (
        <div
          key={idx}
          className="rounded-xl border border-gray-200 dark:border-dark-700 bg-white dark:bg-dark-800 p-4"
        >
          <div className="flex items-center justify-between gap-4">
            <Skeleton className={`h-5 ${width}`} />
            <div className="flex items-center gap-3 shrink-0">
              <Skeleton className="h-5 w-16 rounded-full bg-gray-300 dark:bg-dark-800" />
              <Skeleton className="h-4 w-24" />
            </div>
          </div>
        </div>
      ))}
      <span className="sr-only">Memuat...</span>
    </div>
  );
}

/**
 * Skeleton placeholder for the paste view page. Renders a metadata header
 * (title, language badge, timestamp) followed by a stack of varied-width
 * code-line skeletons that approximate syntax-highlighted content.
 */
export function PasteViewSkeleton() {
  // Hand-picked widths to mimic real code: indented blocks, short statements,
  // longer expressions, blank-ish lines, etc.
  const lineWidths = [
    'w-3/4',
    'w-1/2',
    'w-5/6',
    'w-2/3',
    'w-11/12',
    'w-1/3',
    'w-3/5',
    'w-4/5',
    'w-1/2',
    'w-2/3',
    'w-5/6',
    'w-1/4',
    'w-3/4',
    'w-2/5',
  ];

  return (
    <div
      className="space-y-4"
      role="status"
      aria-label="Memuat paste"
    >
      {/* Metadata header */}
      <div className="rounded-xl border border-gray-200 dark:border-dark-700 bg-white dark:bg-dark-800 p-4">
        <div className="flex items-center justify-between gap-4">
          <Skeleton className="h-6 w-1/2" />
          <Skeleton className="h-7 w-24 rounded-full bg-gray-300 dark:bg-dark-800" />
        </div>
        <div className="mt-3 flex items-center gap-3">
          <Skeleton className="h-4 w-32" />
          <Skeleton className="h-4 w-28" />
        </div>
      </div>

      {/* Code-line block */}
      <div className="rounded-xl border border-gray-200 dark:border-dark-700 bg-white dark:bg-dark-800 p-4">
        <div className="space-y-2 font-mono">
          {lineWidths.map((width, idx) => (
            <div key={idx} className="flex items-center gap-3">
              <Skeleton className="h-4 w-6 shrink-0 bg-gray-300 dark:bg-dark-800" />
              <Skeleton className={`h-4 ${width}`} />
            </div>
          ))}
        </div>
      </div>
      <span className="sr-only">Memuat...</span>
    </div>
  );
}

/**
 * Skeleton placeholder for forms (create paste, upload, unlock).
 * Renders labeled input bars and a dropdown bar inside a card-shaped wrapper.
 */
export function FormSkeleton() {
  return (
    <div
      className="rounded-xl border border-gray-200 dark:border-dark-700 bg-white dark:bg-dark-800 p-6 space-y-5"
      role="status"
      aria-label="Memuat formulir"
    >
      <div className="space-y-2">
        <Skeleton className="h-4 w-20" />
        <Skeleton className="h-10 w-full" />
      </div>
      <div className="space-y-2">
        <Skeleton className="h-4 w-24" />
        <Skeleton className="h-10 w-full" />
      </div>
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <div className="space-y-2">
          <Skeleton className="h-4 w-20" />
          <Skeleton className="h-10 w-full" />
        </div>
        <div className="space-y-2">
          <Skeleton className="h-4 w-28" />
          <Skeleton className="h-10 w-full" />
        </div>
      </div>
      <div className="space-y-2">
        <Skeleton className="h-4 w-32" />
        <Skeleton className="h-32 w-full" />
      </div>
      <Skeleton className="h-10 w-32" />
      <span className="sr-only">Memuat...</span>
    </div>
  );
}
