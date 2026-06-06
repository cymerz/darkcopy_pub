'use client';

// components/PasteForm.tsx
//
// Client Component for creating a new paste (Requirements 2.2–2.7).
//
// Renders a controlled form with:
// - a monospace content textarea backed by a synced line-number gutter (Req 2.7)
// - an optional title input
// - language and expiry dropdowns populated from backend options (props)
// - visibility radio buttons, with a conditional password field shown only when
//   "Dilindungi Kata Sandi" (password_protected) is selected (Req 2.3)
// - a submit button with a loading state that disables the form (Req 2.6)
// - an inline error message that preserves user input on failure (Req 2.5)
//
// On success the browser is navigated to `/{slug}` via `router.push` (Req 2.4).
//
// Backend form field names mirror the Go `HandleCreate` handler exactly:
//   content, title, language, expires_in (minutes), visibility, password.

import { useRef, useState, useSyncExternalStore } from 'react';
import { useRouter } from 'next/navigation';
import { createPaste } from '@/lib/api';
import { APIError } from '@/lib/types';
import type { ExpiryOption, Language } from '@/lib/types';

interface PasteFormProps {
  languages: Language[];
  expiryOptions: ExpiryOption[];
  disabled?: boolean;
}

type Visibility = 'public' | 'unlisted' | 'password_protected';

const VISIBILITY_OPTIONS: { value: Visibility; label: string }[] = [
  { value: 'public', label: 'Publik' },
  { value: 'unlisted', label: 'Unlisted' },
  { value: 'password_protected', label: 'Dilindungi Kata Sandi' },
];

// Shared Tailwind classes for text inputs / selects to keep the dark theme
// consistent and ensure >= 44px touch targets (Req 9.5).
const FIELD_CLASS =
  'w-full min-h-[44px] rounded-lg border border-gray-200 dark:border-dark-700 bg-white dark:bg-dark-800 px-3 py-2.5 ' +
  'text-gray-900 dark:text-gray-100 placeholder-gray-400 dark:placeholder-gray-500 transition-colors ' +
  'focus:border-accent focus:outline-none focus:ring-2 focus:ring-accent/40';

/**
 * Derives a slug from the createPaste response. Prefers the explicit `slug`
 * field and, defensively, falls back to the trailing segment of `url` (e.g.
 * "/abc123" -> "abc123") in case the backend only returns a location.
 */
function resolveSlug(result: { slug?: string; url?: string }): string | null {
  if (result.slug && result.slug.trim()) {
    return result.slug.trim();
  }
  if (result.url && result.url.trim()) {
    const trimmed = result.url.trim().replace(/\/+$/, '');
    const segment = trimmed.split('/').filter(Boolean).pop();
    if (segment) return segment;
  }
  return null;
}

export function PasteForm({ languages, expiryOptions, disabled }: PasteFormProps) {
  const router = useRouter();

  const [content, setContent] = useState('');
  const [title, setTitle] = useState('');
  const [language, setLanguage] = useState(languages[0]?.id ?? '');
  const [expiresIn, setExpiresIn] = useState(
    expiryOptions[0] ? String(expiryOptions[0].duration) : '',
  );
  const [visibility, setVisibility] = useState<Visibility>('public');
  const [password, setPassword] = useState('');
  const [customSlug, setCustomSlug] = useState('');

  // Read window.location.origin in a hydration-safe way: server snapshot is ''
  // (matching SSR), client snapshot reads the real origin after hydration.
  const origin = useSyncExternalStore(
    () => () => { },
    () => window.location.origin,
    () => '',
  );

  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const gutterRef = useRef<HTMLDivElement>(null);

  const isFormDisabled = isSubmitting || disabled;

  // Line numbers for the content gutter. At least one line is always shown.
  const lineCount = Math.max(1, content.split('\n').length);
  const lineNumbers = Array.from({ length: lineCount }, (_, i) => i + 1);

  // Keep the line-number gutter scroll position in sync with the textarea.
  const handleTextareaScroll = (
    e: React.UIEvent<HTMLTextAreaElement>,
  ): void => {
    if (gutterRef.current) {
      gutterRef.current.scrollTop = e.currentTarget.scrollTop;
    }
  };

  const handleSubmit = async (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    if (isSubmitting) return;

    setError(null);
    setIsSubmitting(true);

    try {
      const formData = new URLSearchParams();
      formData.append('content', content);
      formData.append('title', title);
      formData.append('language', language);
      formData.append('expires_in', expiresIn);
      formData.append('visibility', visibility);
      if (customSlug.trim()) formData.append('custom_slug', customSlug.trim().toLowerCase());
      if (visibility === 'password_protected') {
        formData.append('password', password);
      }

      const result = await createPaste(formData);
      const slug = resolveSlug(result);

      if (!slug) {
        setError('Gagal membuat paste. Silakan coba lagi.');
        setIsSubmitting(false);
        return;
      }

      // Success: navigate to the newly created paste (Req 2.4).
      router.push(`/${slug}`);
    } catch (err) {
      // Preserve form data; only surface the backend error message (Req 2.5).
      if (err instanceof APIError) {
        setError(err.message);
      } else {
        setError('Terjadi kesalahan saat membuat paste. Silakan coba lagi.');
      }
      setIsSubmitting(false);
    }
  };

  return (
    <form onSubmit={handleSubmit} className="space-y-6" noValidate>
      {disabled && (
        <div role="alert" className="rounded-xl border border-amber-500/40 bg-amber-500/10 p-4 text-amber-600 dark:text-amber-300 backdrop-blur-md animate-pulse">
          <div className="flex items-center space-x-3">
            <svg className="h-6 w-6 text-amber-500 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth="2.5">
              <path strokeLinecap="round" strokeLinejoin="round" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
            </svg>
            <div>
              <h4 className="font-bold text-sm">Fitur Pembuatan Paste Ditangguhkan Sementara</h4>
              <p className="text-xs mt-0.5 opacity-90">Administrator telah menonaktifkan pembuatan paste baru sementara waktu untuk pemeliharaan sistem.</p>
            </div>
          </div>
        </div>
      )}

      {/* Content with line numbers (Req 2.2, 2.7) */}
      <div className="space-y-2">
        <label
          htmlFor="content"
          className="block text-sm font-medium text-gray-800 dark:text-gray-200"
        >
          Konten <span className="text-accent dark:text-accent-hover">*</span>
        </label>
        <div className="flex overflow-hidden rounded-lg border border-gray-200 dark:border-dark-700 bg-white dark:bg-dark-800 transition-colors focus-within:border-accent focus-within:ring-2 focus-within:ring-accent/40">
          <div
            ref={gutterRef}
            aria-hidden="true"
            className="select-none overflow-hidden bg-gray-50 dark:bg-dark-900/60 px-3 py-3 text-right font-mono text-sm leading-6 text-gray-500 dark:text-gray-500"
          >
            {lineNumbers.map((n) => (
              <div key={n}>{n}</div>
            ))}
          </div>
          <textarea
            id="content"
            name="content"
            required
            value={content}
            onChange={(e) => setContent(e.target.value)}
            onScroll={handleTextareaScroll}
            placeholder="Tempel kode atau teks di sini..."
            spellCheck={false}
            rows={14}
            disabled={isFormDisabled}
            className="min-h-[280px] flex-1 resize-y bg-transparent px-3 py-3 font-mono text-sm leading-6 text-gray-900 dark:text-gray-100 placeholder-gray-400 dark:placeholder-gray-500 focus:outline-none disabled:cursor-not-allowed"
          />
        </div>
      </div>

      {/* Title (optional) */}
      <div className="space-y-2">
        <label
          htmlFor="title"
          className="block text-sm font-medium text-gray-800 dark:text-gray-200"
        >
          Judul (opsional)
        </label>
        <input
          type="text"
          id="title"
          name="title"
          value={title}
          onChange={(e) => setTitle(e.target.value)}
          placeholder="Judul paste"
          disabled={isFormDisabled}
          className={FIELD_CLASS}
        />
      </div>

      {/* Custom slug (optional) */}
      <div className="space-y-2">
        <label
          htmlFor="custom_slug"
          className="block text-sm font-medium text-gray-800 dark:text-gray-200"
        >
          Custom URL (opsional)
        </label>
        <div className="flex items-center overflow-hidden rounded-lg border border-gray-200 dark:border-dark-700 bg-white dark:bg-dark-800 transition-colors focus-within:border-accent focus-within:ring-2 focus-within:ring-accent/40">
          <span className="shrink-0 select-none border-r border-gray-200 dark:border-dark-700 bg-gray-50 dark:bg-dark-900/60 px-3 py-2.5 text-sm text-gray-500 dark:text-gray-500 whitespace-nowrap">
            {origin}/
          </span>
          <input
            type="text"
            id="custom_slug"
            name="custom_slug"
            value={customSlug}
            onChange={(e) => setCustomSlug(e.target.value.toLowerCase().replace(/[^a-z0-9-]/g, ''))}
            placeholder="slug-kustom (min. 3 karakter)"
            disabled={isFormDisabled}
            className="min-h-[44px] flex-1 bg-transparent px-3 py-2.5 text-sm text-gray-900 dark:text-gray-100 placeholder-gray-400 dark:placeholder-gray-500 focus:outline-none disabled:cursor-not-allowed"
          />
        </div>
        <p className="text-xs text-gray-500 dark:text-gray-500">Hanya huruf kecil, angka, dan tanda hubung. Kosongkan untuk slug otomatis.</p>
      </div>

      <div className="grid gap-6 md:grid-cols-2">
        {/* Language */}
        <div className="space-y-2">
          <label
            htmlFor="language"
            className="block text-sm font-medium text-gray-800 dark:text-gray-200"
          >
            Bahasa
          </label>
          <select
            id="language"
            name="language"
            value={language}
            onChange={(e) => setLanguage(e.target.value)}
            disabled={isFormDisabled}
            className={FIELD_CLASS}
          >
            {languages.map((lang) => (
              <option key={lang.id} value={lang.id}>
                {lang.name}
              </option>
            ))}
          </select>
        </div>

        {/* Expiry */}
        <div className="space-y-2">
          <label
            htmlFor="expires_in"
            className="block text-sm font-medium text-gray-800 dark:text-gray-200"
          >
            Waktu Kadaluarsa
          </label>
          <select
            id="expires_in"
            name="expires_in"
            value={expiresIn}
            onChange={(e) => setExpiresIn(e.target.value)}
            disabled={isFormDisabled}
            className={FIELD_CLASS}
          >
            {expiryOptions.map((option) => (
              <option key={option.label} value={String(option.duration)}>
                {option.label}
              </option>
            ))}
          </select>
        </div>
      </div>

      {/* Visibility (Req 2.2) */}
      <fieldset className="space-y-2">
        <legend className="text-sm font-medium text-gray-800 dark:text-gray-200">
          Visibilitas
        </legend>
        <div className="flex flex-col gap-2 md:flex-row md:flex-wrap md:gap-4">
          {VISIBILITY_OPTIONS.map((option) => (
            <label
              key={option.value}
              className="flex min-h-[44px] cursor-pointer items-center gap-2.5 rounded-lg border border-gray-200 dark:border-dark-700 bg-white dark:bg-dark-800 px-3.5 py-2.5 text-sm text-gray-800 dark:text-gray-200 transition-colors hover:border-accent/60 has-[:checked]:border-accent has-[:checked]:bg-accent/10 disabled:cursor-not-allowed disabled:opacity-60"
            >
              <input
                type="radio"
                name="visibility"
                value={option.value}
                checked={visibility === option.value}
                onChange={() => setVisibility(option.value)}
                disabled={isFormDisabled}
                className="h-4 w-4 accent-accent"
              />
              {option.label}
            </label>
          ))}
        </div>
      </fieldset>

      {/* Conditional password field (Req 2.3) */}
      {visibility === 'password_protected' && (
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
            placeholder="Masukkan kata sandi"
            autoComplete="new-password"
            disabled={isFormDisabled}
            className={FIELD_CLASS}
          />
        </div>
      )}

      {/* Error display (Req 2.5) — shown below the form, input preserved */}
      {error && (
        <div
          role="alert"
          className="rounded-lg border border-red-500/40 bg-red-500/10 px-4 py-3 text-sm text-red-600 dark:text-red-300"
        >
          {error}
        </div>
      )}

      {/* Submit with loading state (Req 2.6) */}
      <div>
        <button
          type="submit"
          disabled={isFormDisabled}
          className="inline-flex min-h-[44px] items-center justify-center gap-2 rounded-lg bg-accent px-6 py-2.5 font-medium text-white shadow-sm shadow-accent/30 transition-colors hover:bg-accent-hover focus:outline-none focus:ring-2 focus:ring-accent/50 disabled:cursor-not-allowed disabled:opacity-60"
        >
          {isSubmitting && (
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
          {isSubmitting ? 'Membuat...' : 'Buat Paste'}
        </button>
      </div>
    </form>
  );
}

export default PasteForm;
