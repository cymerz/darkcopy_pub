'use client';

import { useEffect, useRef, useState } from 'react';

interface CopyButtonProps {
  content: string;
}

type CopyState = 'idle' | 'copied' | 'error';

const FEEDBACK_DURATION_MS = 2000;

export function CopyButton({ content }: CopyButtonProps) {
  const [state, setState] = useState<CopyState>('idle');
  const timeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    return () => {
      if (timeoutRef.current) clearTimeout(timeoutRef.current);
    };
  }, []);

  const scheduleReset = () => {
    if (timeoutRef.current) clearTimeout(timeoutRef.current);
    timeoutRef.current = setTimeout(() => {
      setState('idle');
      timeoutRef.current = null;
    }, FEEDBACK_DURATION_MS);
  };

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(content);
      setState('copied');
    } catch {
      setState('error');
    }
    scheduleReset();
  };

  return (
    <button
      type="button"
      onClick={handleCopy}
      aria-live="polite"
      aria-label={state === 'copied' ? 'Berhasil disalin' : state === 'error' ? 'Gagal menyalin' : 'Salin ke clipboard'}
      title={state === 'copied' ? 'Berhasil disalin!' : state === 'error' ? 'Gagal menyalin' : 'Salin'}
      className={`inline-flex min-h-[36px] items-center justify-center gap-1.5 rounded-md border px-3 py-1.5 text-xs font-medium transition-all ${
        state === 'copied'
          ? 'border-emerald-500/50 bg-emerald-500/10 text-emerald-400 dark:text-emerald-300'
          : state === 'error'
            ? 'border-red-500/50 bg-red-500/10 text-red-400 dark:text-red-300'
            : 'border-gray-200 dark:border-dark-600 bg-white dark:bg-dark-800 text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-dark-700 hover:border-gray-300 dark:hover:border-dark-500'
      }`}
    >
      {state === 'copied' ? (
        <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" className="w-3.5 h-3.5" aria-hidden="true">
          <polyline points="20 6 9 17 4 12" />
        </svg>
      ) : state === 'error' ? (
        <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" className="w-3.5 h-3.5" aria-hidden="true">
          <line x1="18" y1="6" x2="6" y2="18" />
          <line x1="6" y1="6" x2="18" y2="18" />
        </svg>
      ) : (
        <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" className="w-3.5 h-3.5" aria-hidden="true">
          <rect x="9" y="9" width="13" height="13" rx="2" ry="2" />
          <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1" />
        </svg>
      )}
      {state === 'copied' ? 'Berhasil disalin' : state === 'error' ? 'Gagal menyalin' : 'Salin'}
    </button>
  );
}

export default CopyButton;
