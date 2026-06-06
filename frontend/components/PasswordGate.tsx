'use client';

// components/PasswordGate.tsx
//
// Client Component that renders the password unlock form for a protected
// paste or file (Requirements 4.1–4.6, 6.2–6.5).
//
// Responsibilities:
// - Render a single password input (type="password") and a submit button whose
//   label depends on the resource type: "Buka" for a paste, "Unduh File" for a
//   file (Req 4.1, 6.2).
// - Disable the form while a request is in flight (loading) (Req 4.x, 6.x).
// - On HTTP 401 show "Kata sandi salah" and clear the password input
//   (Req 4.4, 6.4).
// - On HTTP 429 show a rate-limit message and disable the submit button for
//   30 seconds, then re-enable it (Req 4.5, 6.5).
// - On HTTP 404/410 surface an error message (Req 4.6).
// - On HTTP 200 for a paste: invoke the `onUnlock` callback with the decoded
//   paste data (the unlock page renders the viewer) and navigate (Req 4.3).
// - On HTTP 200 for a file: stream the response body into a Blob and trigger a
//   browser download (Req 6.3).
//
// State machine (design.md "Property 5: Password Gate State Machine"):
// the observable UI is driven entirely by a small, pure transition function
// (`nextState`) exported below so it can be unit/property tested in isolation.
// Content is only ever exposed (`contentUnlocked === true`) as the result of a
// SUCCESS event applied while a request is in flight (status === 'loading'),
// which guarantees content is never revealed without a real HTTP 200 unlock.

import { useEffect, useReducer, useRef, useState } from 'react';
import { useRouter } from 'next/navigation';
import { unlockPaste, unlockFile } from '@/lib/api';
import { APIError } from '@/lib/types';
import type { PasteViewResponse } from '@/lib/types';

// ---------------------------------------------------------------------------
// State machine (pure, exported for testing — Property 5)
// ---------------------------------------------------------------------------

/**
 * The four observable gate states:
 * - `idle`        : form enabled, no error shown.
 * - `loading`     : a request is in flight, form disabled.
 * - `error`       : an error message is shown, form re-enabled.
 * - `rate_limited`: too many attempts; submit disabled during cooldown.
 *
 * The form is interactive (enabled) exactly in the `idle` and `error` states.
 */
export type GateStatus = 'idle' | 'loading' | 'error' | 'rate_limited';

export interface GateState {
  status: GateStatus;
  /** Human-readable error message, shown only in the `error`/`rate_limited` states. */
  errorMessage: string | null;
  /**
   * True only after a successful (HTTP 200) unlock. This flag may ONLY be set
   * by a SUCCESS event applied while `status === 'loading'`, so content can
   * never be exposed without a real successful unlock response.
   */
  contentUnlocked: boolean;
}

/**
 * Events that drive the gate. Each maps to a backend outcome (or a UI action),
 * keeping the transition logic free of side effects.
 */
export type GateEvent =
  | { type: 'SUBMIT' } // user submitted the password form
  | { type: 'SUCCESS' } // backend returned HTTP 200
  | { type: 'UNAUTHORIZED' } // backend returned HTTP 401
  | { type: 'RATE_LIMITED' } // backend returned HTTP 429
  | { type: 'NOT_FOUND' } // backend returned HTTP 404
  | { type: 'GONE' } // backend returned HTTP 410
  | { type: 'ERROR'; message?: string } // network/other failure
  | { type: 'COOLDOWN_ELAPSED' }; // rate-limit cooldown timer fired

// User-facing messages (Indonesian) kept as named constants so tests and the
// component share a single source of truth.
export const MSG_WRONG_PASSWORD = 'Kata sandi salah';
export const MSG_RATE_LIMITED =
  'Terlalu banyak percobaan. Silakan coba lagi nanti.';
export const MSG_NOT_FOUND = 'Tidak ditemukan';
export const MSG_GONE = 'Telah kadaluarsa';
export const MSG_GENERIC = 'Terjadi kesalahan. Silakan coba lagi.';

/** Duration (ms) the submit button stays disabled after an HTTP 429. */
export const RATE_LIMIT_COOLDOWN_MS = 30000;

/** The initial gate state: idle, no error, content locked. */
export const initialGateState: GateState = {
  status: 'idle',
  errorMessage: null,
  contentUnlocked: false,
};

/**
 * Returns whether the form is interactive for a given status. The form is only
 * enabled while idle or while showing an error (so the user can retry).
 */
export function isFormEnabled(status: GateStatus): boolean {
  return status === 'idle' || status === 'error';
}

/**
 * Pure state transition for the password gate (Property 5).
 *
 * Invariants enforced here:
 * - A request can only start (`SUBMIT` -> `loading`) from an interactive state
 *   (`idle` or `error`); submitting while `loading` or `rate_limited` is a no-op.
 * - Every backend-outcome event (`SUCCESS`, `UNAUTHORIZED`, `RATE_LIMITED`,
 *   `NOT_FOUND`, `GONE`, `ERROR`) is only honored while `loading`; otherwise it
 *   is ignored. This keeps outcomes paired with an in-flight request.
 * - `contentUnlocked` is set to `true` ONLY by a `SUCCESS` event applied while
 *   `loading`. No other event can expose content.
 * - `COOLDOWN_ELAPSED` only re-enables the form from `rate_limited`.
 */
export function nextState(current: GateState, event: GateEvent): GateState {
  switch (event.type) {
    case 'SUBMIT':
      if (!isFormEnabled(current.status)) return current;
      return { status: 'loading', errorMessage: null, contentUnlocked: false };

    case 'SUCCESS':
      if (current.status !== 'loading') return current;
      return { status: 'idle', errorMessage: null, contentUnlocked: true };

    case 'UNAUTHORIZED':
      if (current.status !== 'loading') return current;
      return {
        status: 'error',
        errorMessage: MSG_WRONG_PASSWORD,
        contentUnlocked: false,
      };

    case 'RATE_LIMITED':
      if (current.status !== 'loading') return current;
      return {
        status: 'rate_limited',
        errorMessage: MSG_RATE_LIMITED,
        contentUnlocked: false,
      };

    case 'NOT_FOUND':
      if (current.status !== 'loading') return current;
      return {
        status: 'error',
        errorMessage: MSG_NOT_FOUND,
        contentUnlocked: false,
      };

    case 'GONE':
      if (current.status !== 'loading') return current;
      return { status: 'error', errorMessage: MSG_GONE, contentUnlocked: false };

    case 'ERROR':
      if (current.status !== 'loading') return current;
      return {
        status: 'error',
        errorMessage: event.message ?? MSG_GENERIC,
        contentUnlocked: false,
      };

    case 'COOLDOWN_ELAPSED':
      if (current.status !== 'rate_limited') return current;
      return { status: 'idle', errorMessage: null, contentUnlocked: false };

    default:
      return current;
  }
}

/**
 * Maps an HTTP status code to the corresponding gate event. Any status other
 * than the explicitly handled ones is treated as a generic error.
 */
export function eventForStatus(status: number): GateEvent {
  switch (status) {
    case 401:
      return { type: 'UNAUTHORIZED' };
    case 429:
      return { type: 'RATE_LIMITED' };
    case 404:
      return { type: 'NOT_FOUND' };
    case 410:
      return { type: 'GONE' };
    default:
      return { type: 'ERROR' };
  }
}

// ---------------------------------------------------------------------------
// File download helpers
// ---------------------------------------------------------------------------

/**
 * Extracts a filename from a `Content-Disposition` header value, supporting
 * both the plain `filename="..."` form and the RFC 5987 `filename*=UTF-8''...`
 * form. Returns `null` when no filename can be derived.
 */
export function filenameFromContentDisposition(
  header: string | null,
): string | null {
  if (!header) return null;

  // RFC 5987 extended form: filename*=UTF-8''encoded-name
  const extended = header.match(/filename\*=(?:UTF-8'')?([^;]+)/i);
  if (extended?.[1]) {
    try {
      return decodeURIComponent(extended[1].trim().replace(/^"|"$/g, ''));
    } catch {
      // fall through to the plain form
    }
  }

  // Plain form: filename="name" or filename=name
  const plain = header.match(/filename="?([^"\n;]+)"?/i);
  if (plain?.[1]) return plain[1].trim();

  return null;
}

/**
 * Triggers a browser download of `blob` using a temporary anchor element and an
 * object URL, which is revoked once the click has been dispatched.
 */
function triggerBlobDownload(blob: Blob, filename: string): void {
  const url = URL.createObjectURL(blob);
  const anchor = document.createElement('a');
  anchor.href = url;
  anchor.download = filename;
  document.body.appendChild(anchor);
  anchor.click();
  document.body.removeChild(anchor);
  URL.revokeObjectURL(url);
}

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

interface PasswordGateProps {
  slug: string;
  resourceType: 'paste' | 'file';
  onUnlock?: (data: PasteViewResponse) => void;
}

export function PasswordGate({
  slug,
  resourceType,
  onUnlock,
}: PasswordGateProps) {
  const router = useRouter();

  const [state, dispatch] = useReducer(nextState, initialGateState);
  const [password, setPassword] = useState('');

  // Holds the pending rate-limit cooldown timer so it can be cleared on unmount.
  const cooldownTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    return () => {
      if (cooldownTimer.current) clearTimeout(cooldownTimer.current);
    };
  }, []);

  const formEnabled = isFormEnabled(state.status);
  const isLoading = state.status === 'loading';
  const isRateLimited = state.status === 'rate_limited';
  const submitDisabled = !formEnabled;

  const submitLabel = resourceType === 'file' ? 'Unduh File' : 'Buka';
  const loadingLabel = resourceType === 'file' ? 'Mengunduh...' : 'Membuka...';

  const handleRateLimited = (): void => {
    dispatch({ type: 'RATE_LIMITED' });
    if (cooldownTimer.current) clearTimeout(cooldownTimer.current);
    cooldownTimer.current = setTimeout(() => {
      dispatch({ type: 'COOLDOWN_ELAPSED' });
      cooldownTimer.current = null;
    }, RATE_LIMIT_COOLDOWN_MS);
  };

  const handlePasteUnlock = async (): Promise<void> => {
    try {
      const data = await unlockPaste(slug, password);
      // HTTP 200: expose content via the callback / navigation (Req 4.3).
      dispatch({ type: 'SUCCESS' });
      if (onUnlock) {
        onUnlock(data);
      } else {
        // Fallback navigation when no inline handler is supplied.
        router.push(`/${slug}`);
      }
    } catch (err) {
      if (err instanceof APIError) {
        if (err.status === 401) setPassword(''); // clear input (Req 4.4)
        if (err.status === 429) {
          handleRateLimited();
        } else {
          dispatch(eventForStatus(err.status));
        }
      } else {
        dispatch({ type: 'ERROR' });
      }
    }
  };

  const handleFileUnlock = async (): Promise<void> => {
    try {
      const response = await unlockFile(slug, password);

      if (response.ok) {
        // HTTP 200: stream the file body and trigger a download (Req 6.3).
        const blob = await response.blob();
        const filename =
          filenameFromContentDisposition(
            response.headers.get('Content-Disposition'),
          ) ?? slug;
        triggerBlobDownload(blob, filename);
        dispatch({ type: 'SUCCESS' });
        return;
      }

      if (response.status === 401) setPassword(''); // clear input (Req 6.4)
      if (response.status === 429) {
        handleRateLimited();
      } else {
        dispatch(eventForStatus(response.status));
      }
    } catch {
      dispatch({ type: 'ERROR' });
    }
  };

  const handleSubmit = async (
    e: React.FormEvent<HTMLFormElement>,
  ): Promise<void> => {
    e.preventDefault();
    if (!formEnabled) return;

    dispatch({ type: 'SUBMIT' });

    if (resourceType === 'file') {
      await handleFileUnlock();
    } else {
      await handlePasteUnlock();
    }
  };

  return (
    <div className="mx-auto flex w-full max-w-md flex-col items-center px-4 py-12">
      <div className="w-full rounded-xl border border-gray-200 dark:border-dark-700 bg-white dark:bg-dark-800 p-6 shadow-lg sm:p-8">
        {/* Lock icon */}
        <div className="mb-5 flex justify-center">
          <span className="flex h-12 w-12 items-center justify-center rounded-full bg-accent/10 dark:bg-accent/15 text-accent dark:text-accent-hover">
            <svg
              xmlns="http://www.w3.org/2000/svg"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth={2}
              strokeLinecap="round"
              strokeLinejoin="round"
              className="h-6 w-6"
              aria-hidden="true"
            >
              <rect x="3" y="11" width="18" height="11" rx="2" ry="2" />
              <path d="M7 11V7a5 5 0 0 1 10 0v4" />
            </svg>
          </span>
        </div>

        <h1 className="mb-1 text-center text-xl font-bold text-gray-900 dark:text-gray-100">
          {resourceType === 'file'
            ? 'File Dilindungi Kata Sandi'
            : 'Paste Dilindungi Kata Sandi'}
        </h1>
        <p className="mb-6 text-center text-sm text-gray-500 dark:text-gray-400">
          Masukkan kata sandi untuk{' '}
          {resourceType === 'file' ? 'mengunduh file ini' : 'melihat paste ini'}
          .
        </p>

        <form onSubmit={handleSubmit} className="space-y-4" noValidate>
          <div className="space-y-2">
            <label
              htmlFor="password"
              className="block text-sm font-medium text-gray-800 dark:text-gray-200"
            >
              Kata Sandi
            </label>
            <input
              type="password"
              id="password"
              name="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              disabled={!formEnabled}
              autoComplete="off"
              autoFocus
              placeholder="Masukkan kata sandi"
              className="w-full min-h-[44px] rounded-lg border border-gray-200 dark:border-dark-700 bg-white dark:bg-dark-900 px-3 py-2.5 text-gray-900 dark:text-gray-100 placeholder-gray-400 dark:placeholder-gray-500 transition-colors focus:border-accent focus:outline-none focus:ring-2 focus:ring-accent/40 disabled:cursor-not-allowed disabled:opacity-60"
            />
          </div>

          {/* Error / rate-limit message (Req 4.4, 4.5, 4.6, 6.4, 6.5) */}
          {state.errorMessage && (
            <div
              role="alert"
              className="rounded-lg border border-red-500/40 bg-red-500/10 px-4 py-3 text-sm text-red-600 dark:text-red-300"
            >
              {state.errorMessage}
            </div>
          )}

          <button
            type="submit"
            disabled={submitDisabled}
            className="inline-flex min-h-[44px] w-full items-center justify-center gap-2 rounded-lg bg-accent px-6 py-2.5 font-medium text-white shadow-sm shadow-accent/30 transition-colors hover:bg-accent-hover focus:outline-none focus:ring-2 focus:ring-accent/50 disabled:cursor-not-allowed disabled:opacity-60"
          >
            {isLoading && (
              <svg
                className="h-4 w-4 animate-spin"
                xmlns="http://www.w3.org/2000/svg"
                fill="none"
                viewBox="0 0 24 24"
                aria-hidden="true"
              >
                <circle
                  className="opacity-25"
                  cx="12"
                  cy="12"
                  r="10"
                  stroke="currentColor"
                  strokeWidth="4"
                />
                <path
                  className="opacity-75"
                  fill="currentColor"
                  d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"
                />
              </svg>
            )}
            {isLoading
              ? loadingLabel
              : isRateLimited
                ? 'Tunggu sebentar...'
                : submitLabel}
          </button>
        </form>
      </div>
    </div>
  );
}

export default PasswordGate;
