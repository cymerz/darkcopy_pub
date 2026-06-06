'use client';

// components/ReportButton.tsx
//
// A "Laporkan" (report) button that opens a modal letting a visitor flag a
// paste or file for admin review. Submits to the public POST /report endpoint.

import { useState } from 'react';
import { submitReport } from '@/lib/api';
import { APIError, REPORT_REASONS } from '@/lib/types';
import type { ReportResourceType } from '@/lib/types';

interface ReportButtonProps {
  resourceType: ReportResourceType;
  slug: string;
  /** When true, render a compact icon+label button matching the viewer toolbar. */
  compact?: boolean;
}

type SubmitState = 'idle' | 'submitting' | 'done' | 'error';

export function ReportButton({ resourceType, slug, compact }: ReportButtonProps) {
  const [open, setOpen] = useState(false);
  const [reason, setReason] = useState(REPORT_REASONS[0].value);
  const [details, setDetails] = useState('');
  const [state, setState] = useState<SubmitState>('idle');
  const [message, setMessage] = useState<string | null>(null);

  const close = () => {
    if (state === 'submitting') return;
    setOpen(false);
    // Reset after closing so a reopen starts fresh.
    setTimeout(() => {
      setReason(REPORT_REASONS[0].value);
      setDetails('');
      setState('idle');
      setMessage(null);
    }, 150);
  };

  const handleSubmit = async (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    if (state === 'submitting') return;
    setState('submitting');
    setMessage(null);
    try {
      const res = await submitReport({ resourceType, slug, reason, details });
      setState('done');
      setMessage(res.message || 'Laporan terkirim. Terima kasih.');
    } catch (err) {
      setState('error');
      if (err instanceof APIError) {
        setMessage(
          err.status === 429
            ? 'Terlalu banyak laporan. Coba lagi besok.'
            : err.message,
        );
      } else {
        setMessage('Gagal mengirim laporan. Silakan coba lagi.');
      }
    }
  };

  return (
    <>
      <button
        type="button"
        onClick={() => setOpen(true)}
        title="Laporkan konten ini"
        className={
          compact
            ? 'inline-flex min-h-[36px] items-center justify-center gap-1.5 rounded-md border border-gray-200 dark:border-dark-600 bg-white dark:bg-dark-800 px-3 py-1.5 text-xs font-medium text-gray-700 dark:text-gray-300 transition-all hover:border-red-400 hover:text-red-600 dark:hover:text-red-400'
            : 'inline-flex min-h-[44px] items-center justify-center gap-2 rounded-lg border border-gray-200 dark:border-dark-700 bg-white dark:bg-dark-800 px-3.5 py-2 text-sm font-medium text-gray-700 dark:text-gray-300 transition-colors hover:border-red-400 hover:text-red-600 dark:hover:text-red-400'
        }
      >
        <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" className="h-3.5 w-3.5" aria-hidden="true">
          <path d="M4 15s1-1 4-1 5 2 8 2 4-1 4-1V3s-1 1-4 1-5-2-8-2-4 1-4 1z" />
          <line x1="4" y1="22" x2="4" y2="15" />
        </svg>
        Laporkan
      </button>

      {open && (
        <div
          className="fixed inset-0 z-[100] flex items-end sm:items-center justify-center bg-black/50 p-0 sm:p-4"
          role="dialog"
          aria-modal="true"
          aria-labelledby="report-title"
          onClick={close}
        >
          <div
            className="w-full sm:max-w-md rounded-t-3xl sm:rounded-xl border-t sm:border border-gray-200 dark:border-dark-700 bg-white dark:bg-dark-800 p-5 sm:p-6 pb-8 sm:pb-6 shadow-2xl transition-all duration-300"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="mb-4 flex items-start justify-between gap-4">
              <h2 id="report-title" className="text-lg font-bold text-gray-900 dark:text-gray-100">
                Laporkan Konten
              </h2>
              <button
                type="button"
                onClick={close}
                aria-label="Tutup"
                className="text-gray-400 transition-colors hover:text-gray-700 dark:hover:text-gray-200"
              >
                <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" className="h-5 w-5" aria-hidden="true">
                  <line x1="18" y1="6" x2="6" y2="18" />
                  <line x1="6" y1="6" x2="18" y2="18" />
                </svg>
              </button>
            </div>

            {state === 'done' ? (
              <div className="space-y-4">
                <div
                  role="status"
                  className="rounded-lg border border-emerald-500/40 bg-emerald-500/10 px-4 py-3 text-sm text-emerald-600 dark:text-emerald-300"
                >
                  {message}
                </div>
                <button
                  type="button"
                  onClick={close}
                  className="inline-flex min-h-[44px] w-full items-center justify-center rounded-lg bg-accent px-6 py-2.5 font-medium text-white transition-colors hover:bg-accent-hover"
                >
                  Tutup
                </button>
              </div>
            ) : (
              <form onSubmit={handleSubmit} className="space-y-4">
                <p className="text-sm text-gray-500 dark:text-gray-400">
                  Bantu kami menjaga platform tetap aman. Laporan akan ditinjau oleh admin.
                </p>

                <div className="space-y-2">
                  <label htmlFor="report-reason" className="block text-sm font-medium text-gray-800 dark:text-gray-200">
                    Alasan
                  </label>
                  <select
                    id="report-reason"
                    value={reason}
                    onChange={(e) => setReason(e.target.value)}
                    disabled={state === 'submitting'}
                    className="w-full min-h-[44px] rounded-lg border border-gray-200 dark:border-dark-700 bg-white dark:bg-dark-900 px-3 py-2.5 text-gray-900 dark:text-gray-100 transition-colors focus:border-accent focus:outline-none focus:ring-2 focus:ring-accent/40"
                  >
                    {REPORT_REASONS.map((r) => (
                      <option key={r.value} value={r.value}>
                        {r.label}
                      </option>
                    ))}
                  </select>
                </div>

                <div className="space-y-2">
                  <label htmlFor="report-details" className="block text-sm font-medium text-gray-800 dark:text-gray-200">
                    Detail (opsional)
                  </label>
                  <textarea
                    id="report-details"
                    value={details}
                    onChange={(e) => setDetails(e.target.value.slice(0, 1000))}
                    disabled={state === 'submitting'}
                    rows={4}
                    placeholder="Jelaskan masalahnya secara singkat..."
                    className="w-full resize-y rounded-lg border border-gray-200 dark:border-dark-700 bg-white dark:bg-dark-900 px-3 py-2.5 text-sm text-gray-900 dark:text-gray-100 placeholder-gray-400 dark:placeholder-gray-500 transition-colors focus:border-accent focus:outline-none focus:ring-2 focus:ring-accent/40"
                  />
                  <p className="text-right text-xs text-gray-400 dark:text-gray-500">
                    {details.length}/1000
                  </p>
                </div>

                {state === 'error' && message && (
                  <div
                    role="alert"
                    className="rounded-lg border border-red-500/40 bg-red-500/10 px-4 py-3 text-sm text-red-600 dark:text-red-300"
                  >
                    {message}
                  </div>
                )}

                <div className="flex flex-col-reverse sm:flex-row gap-2.5 pt-2">
                  <button
                    type="button"
                    onClick={close}
                    disabled={state === 'submitting'}
                    className="inline-flex min-h-[44px] w-full sm:flex-1 items-center justify-center rounded-lg border border-gray-200 dark:border-dark-700 bg-white dark:bg-dark-800 px-4 py-2.5 text-sm font-medium text-gray-800 dark:text-gray-200 transition-colors hover:border-accent/60 disabled:opacity-60"
                  >
                    Batal
                  </button>
                  <button
                    type="submit"
                    disabled={state === 'submitting'}
                    className="inline-flex min-h-[44px] w-full sm:flex-1 items-center justify-center rounded-lg bg-red-600 px-4 py-2.5 text-sm font-medium text-white transition-colors hover:bg-red-700 disabled:cursor-not-allowed disabled:opacity-60"
                  >
                    {state === 'submitting' ? 'Mengirim...' : 'Kirim Laporan'}
                  </button>
                </div>
              </form>
            )}
          </div>
        </div>
      )}
    </>
  );
}

export default ReportButton;
