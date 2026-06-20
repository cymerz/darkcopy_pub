'use client';

// components/FileUploader.tsx
//
// Client Component for uploading a file (Requirements 5.2–5.10).
//
// Provides:
// - a drag-and-drop zone with visual feedback (border change + "Lepaskan file
//   di sini" text on dragover) plus a "Pilih File" button that opens the native
//   file dialog (Req 5.2, 5.3)
// - file info display after selection: name, human-readable size, type icon
//   (Req 5.4)
// - an expiry dropdown, visibility radio buttons, and a conditional password
//   field shown only for password-protected uploads (Req 5.5)
// - upload via XMLHttpRequest so upload progress can be tracked and rendered as
//   a progress bar with percentage (Req 5.6, 5.7)
// - a success state that displays a copyable file URL (Req 5.8)
// - error handling: HTTP 413 shows the size-limit message (Req 5.9); any other
//   error surfaces the backend-provided message (Req 5.10)
//
// Backend multipart field names mirror the Go `HandleUpload` handler exactly:
//   file, visibility, expires_in (minutes), password.

import { useEffect, useRef, useState } from 'react';
import { formatFileSize } from '@/lib/utils';
import type { ExpiryOption, UploadResponse } from '@/lib/types';
import { CopyButton } from '@/components/CopyButton';
import { presignUpload, registerUploadedFile } from '@/lib/api';

interface FileUploaderProps {
  expiryOptions: ExpiryOption[];
  visibilities: string[];
  maxFileSize?: number;
  disabled?: boolean;
  useDirectUpload?: boolean;
}

type UploadStatus = 'idle' | 'uploading' | 'success' | 'error';

// Maps a backend visibility value to its Indonesian display label.
const VISIBILITY_LABELS: Record<string, string> = {
  public: 'Publik',
  unlisted: 'Unlisted',
  password_protected: 'Dilindungi Kata Sandi',
};

/**
 * Returns the Indonesian display label for a backend visibility value,
 * falling back to the raw value when unknown.
 */
export function visibilityLabel(value: string): string {
  return VISIBILITY_LABELS[value] ?? value;
}

// Shared Tailwind classes for selects to keep the dark theme consistent and
// ensure >= 44px touch targets (Req 9.5).
const FIELD_CLASS =
  'w-full min-h-[44px] rounded-lg border border-gray-200 dark:border-dark-700 bg-white dark:bg-dark-800 px-3 py-2.5 ' +
  'text-gray-900 dark:text-gray-100 placeholder-gray-400 dark:placeholder-gray-500 transition-colors ' +
  'focus:border-accent focus:outline-none focus:ring-2 focus:ring-accent/40';

/** Generic document icon shown next to a selected file (Req 5.4). */
function FileTypeIcon() {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth={1.75}
      strokeLinecap="round"
      strokeLinejoin="round"
      className="h-8 w-8 shrink-0 text-accent dark:text-accent-hover"
      aria-hidden="true"
    >
      <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" />
      <path d="M14 2v6h6" />
      <line x1="8" y1="13" x2="16" y2="13" />
      <line x1="8" y1="17" x2="16" y2="17" />
    </svg>
  );
}

export function FileUploader({ expiryOptions, visibilities, maxFileSize = 100 * 1024 * 1024, disabled, useDirectUpload }: FileUploaderProps) {
  const [file, setFile] = useState<File | null>(null);
  const [isDragging, setIsDragging] = useState(false);
  const [expiresIn, setExpiresIn] = useState(
    expiryOptions[0] ? String(expiryOptions[0].duration) : '',
  );
  const [visibility, setVisibility] = useState(
    visibilities.includes('public') ? 'public' : (visibilities[0] ?? 'public'),
  );
  const [password, setPassword] = useState('');

  const [status, setStatus] = useState<UploadStatus>('idle');
  const [progress, setProgress] = useState(0);
  const [error, setError] = useState<string | null>(null);
  const [uploadUrl, setUploadUrl] = useState<string>('');

  const fileInputRef = useRef<HTMLInputElement>(null);
  const xhrRef = useRef<XMLHttpRequest | null>(null);

  const isUploading = status === 'uploading';
  const isFormDisabled = isUploading || disabled;

  // Abort any in-flight upload if the component unmounts.
  useEffect(() => {
    return () => {
      xhrRef.current?.abort();
    };
  }, []);

  /** Selects a file and resets any prior result/error state. */
  const selectFile = (selected: File) => {
    setFile(selected);
    setStatus('idle');
    setProgress(0);
    setUploadUrl('');
    if (selected.size > maxFileSize) {
      setError(`Ukuran file melebihi batas maksimum ${formatFileSize(maxFileSize)}`);
    } else {
      setError(null);
    }
  };

  const handleDragOver = (e: React.DragEvent<HTMLDivElement>) => {
    e.preventDefault();
    if (!isFormDisabled) setIsDragging(true);
  };

  const handleDragLeave = (e: React.DragEvent<HTMLDivElement>) => {
    e.preventDefault();
    setIsDragging(false);
  };

  const handleDrop = (e: React.DragEvent<HTMLDivElement>) => {
    e.preventDefault();
    setIsDragging(false);
    if (isFormDisabled) return;
    const dropped = e.dataTransfer.files;
    if (dropped && dropped.length > 0) {
      selectFile(dropped[0]);
    }
  };

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    if (isFormDisabled) return;
    const picked = e.target.files;
    if (picked && picked.length > 0) {
      selectFile(picked[0]);
    }
  };

  const openFileDialog = () => {
    if (!isFormDisabled) fileInputRef.current?.click();
  };

  // Touch fallback (Req 9.4): native HTML5 drag-and-drop is unreliable on touch
  // devices, so a tap on the drop zone opens the native file picker (which on
  // mobile offers camera/gallery/files). preventDefault suppresses the
  // synthesized click so the dialog opens exactly once per tap.
  const handleTouchEnd = (e: React.TouchEvent<HTMLDivElement>) => {
    e.preventDefault();
    if (!isFormDisabled) openFileDialog();
  };

  const handleUpload = () => {
    if (!file || isUploading || file.size > maxFileSize) return;

    setStatus('uploading');
    setProgress(0);
    setError(null);
    setUploadUrl('');

    if (useDirectUpload) {
      // Direct-to-S3 pre-signed URL workflow
      presignUpload({
        filename: file.name,
        size_bytes: file.size,
        mime_type: file.type || 'application/octet-stream',
        visibility: visibility,
        password: visibility === 'password_protected' ? password : '',
        expires_in: expiresIn,
      })
        .then((presignData) => {
          const xhr = new XMLHttpRequest();
          xhrRef.current = xhr;

          xhr.upload.onprogress = (e) => {
            if (e.lengthComputable) {
              setProgress(Math.round((e.loaded / e.total) * 100));
            }
          };

          xhr.onload = () => {
            xhrRef.current = null;

            if (xhr.status === 200 || xhr.status === 201 || xhr.status === 204) {
              // Successfully uploaded to S3, register it with the backend database
              registerUploadedFile({
                slug: presignData.slug,
                filename: file.name,
                size_bytes: file.size,
                mime_type: file.type || 'application/octet-stream',
                storage_key: presignData.storage_key,
                visibility: visibility,
                password: visibility === 'password_protected' ? password : '',
                expires_in: expiresIn,
              })
                .then((result) => {
                  const origin = typeof window !== 'undefined' ? window.location.origin : '';
                  const rawUrl = result.url ?? '';
                  const fullUrl = /^https?:\/\//.test(rawUrl)
                    ? rawUrl
                    : `${origin}${rawUrl}`;
                  setUploadUrl(fullUrl);
                  setStatus('success');
                })
                .catch((err) => {
                  setError(err.message || 'Gagal meregistrasi file.');
                  setStatus('error');
                });
            } else {
              setError('Gagal mengunggah file ke storage.');
              setStatus('error');
            }
          };

          xhr.onerror = () => {
            xhrRef.current = null;
            setError('Terjadi kesalahan jaringan saat mengunggah. Silakan coba lagi.');
            setStatus('error');
          };

          xhr.open('PUT', presignData.upload_url);
          xhr.setRequestHeader('Content-Type', file.type || 'application/octet-stream');
          xhr.send(file);
        })
        .catch((err) => {
          xhrRef.current = null;
          setError(err.message || 'Gagal memproses pre-signed URL.');
          setStatus('error');
        });
    } else {
      // Normal proxied multipart form data workflow
      const formData = new FormData();
      formData.append('file', file);
      formData.append('visibility', visibility);
      formData.append('expires_in', expiresIn);
      if (visibility === 'password_protected') {
        formData.append('password', password);
      }

      const xhr = new XMLHttpRequest();
      xhrRef.current = xhr;

      xhr.upload.onprogress = (e) => {
        if (e.lengthComputable) {
          setProgress(Math.round((e.loaded / e.total) * 100));
        }
      };

      xhr.onload = () => {
        xhrRef.current = null;

        if (xhr.status === 201) {
          try {
            const result: UploadResponse = JSON.parse(xhr.responseText);
            const origin = typeof window !== 'undefined' ? window.location.origin : '';
            const rawUrl = result.url ?? '';
            const fullUrl = /^https?:\/\//.test(rawUrl)
              ? rawUrl
              : `${origin}${rawUrl}`;
            setUploadUrl(fullUrl);
            setStatus('success');
          } catch {
            setError('Respons server tidak valid.');
            setStatus('error');
          }
          return;
        }

        if (xhr.status === 413) {
          setError(`Ukuran file melebihi batas maksimum ${formatFileSize(maxFileSize)}`);
          setStatus('error');
          return;
        }

        let message = 'Gagal mengunggah file. Silakan coba lagi.';
        try {
          const parsed = JSON.parse(xhr.responseText);
          if (parsed?.error) message = parsed.error;
        } catch {}
        setError(message);
        setStatus('error');
      };

      xhr.onerror = () => {
        xhrRef.current = null;
        setError('Terjadi kesalahan jaringan saat mengunggah. Silakan coba lagi.');
        setStatus('error');
      };

      xhr.open('POST', '/api/upload');
      xhr.setRequestHeader('Accept', 'application/json');
      xhr.send(formData);
    }
  };

  const resetForAnother = () => {
    setFile(null);
    setStatus('idle');
    setProgress(0);
    setError(null);
    setUploadUrl('');
    if (fileInputRef.current) fileInputRef.current.value = '';
  };

  // Success view: show the copyable file URL (Req 5.8).
  if (status === 'success') {
    return (
      <div className="space-y-6">
        <div
          role="status"
          className="rounded-lg border border-accent/40 bg-accent/10 px-4 py-3 text-sm text-accent dark:text-accent-hover"
        >
          File berhasil diunggah!
        </div>

        <div className="space-y-2">
          <label
            htmlFor="upload-url"
            className="block text-sm font-medium text-gray-800 dark:text-gray-200"
          >
            URL File
          </label>
          <div className="flex flex-col gap-3 md:flex-row md:items-center">
            <input
              id="upload-url"
              type="text"
              readOnly
              value={uploadUrl}
              onFocus={(e) => e.currentTarget.select()}
              className={`${FIELD_CLASS} font-mono text-sm`}
            />
            <CopyButton content={uploadUrl} />
          </div>
          <a
            href={uploadUrl}
            className="inline-flex min-h-[44px] items-center text-sm text-accent dark:text-accent-hover underline-offset-2 hover:underline"
          >
            Buka file
          </a>
        </div>

        <button
          type="button"
          onClick={resetForAnother}
          className="inline-flex min-h-[44px] items-center justify-center rounded-lg border border-gray-200 dark:border-dark-700 bg-white dark:bg-dark-800 px-6 py-2.5 font-medium text-gray-800 dark:text-gray-200 transition-colors hover:border-accent/60 hover:text-gray-900 dark:hover:text-white focus:outline-none focus:ring-2 focus:ring-accent/40"
        >
          Unggah file lain
        </button>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {disabled && (
        <div role="alert" className="rounded-xl border border-amber-500/40 bg-amber-500/10 p-4 text-amber-600 dark:text-amber-300 backdrop-blur-md animate-pulse">
          <div className="flex items-center space-x-3">
            <svg className="h-6 w-6 text-amber-500 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth="2.5">
              <path strokeLinecap="round" strokeLinejoin="round" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
            </svg>
            <div>
              <h4 className="font-bold text-sm">Fitur Unggah File Ditangguhkan Sementara</h4>
              <p className="text-xs mt-0.5 opacity-90">Administrator telah menonaktifkan unggah file baru sementara waktu untuk pemeliharaan sistem.</p>
            </div>
          </div>
        </div>
      )}

      {/* Drag-and-drop zone (Req 5.2, 5.3) */}
      <div className="space-y-2">
        <span className="block text-sm font-medium text-gray-800 dark:text-gray-200">
          File (maks. {formatFileSize(maxFileSize)})
        </span>
        <div
          onDragOver={handleDragOver}
          onDragLeave={handleDragLeave}
          onDrop={handleDrop}
          onClick={openFileDialog}
          onTouchEnd={handleTouchEnd}
          role="button"
          tabIndex={0}
          aria-label="Area unggah file: seret file ke sini atau ketuk untuk memilih"
          onKeyDown={(e) => {
            if (e.key === 'Enter' || e.key === ' ') {
              e.preventDefault();
              openFileDialog();
            }
          }}
          className={`flex min-h-[160px] cursor-pointer touch-manipulation flex-col items-center justify-center rounded-xl border-2 border-dashed px-6 py-10 text-center transition-colors ${isDragging
              ? 'border-accent bg-accent/10'
              : 'border-gray-300 dark:border-dark-600 bg-gray-50 dark:bg-dark-800/60 hover:border-accent/60 hover:bg-gray-100 dark:hover:bg-dark-800'
            }`}
        >
          {isDragging ? (
            <p className="text-base font-medium text-accent dark:text-accent-hover">
              Lepaskan file di sini
            </p>
          ) : file ? (
            <div className="flex items-center gap-3 text-left w-full min-w-0 justify-center px-4">
              <FileTypeIcon />
              <div className="min-w-0 max-w-full">
                <p className="truncate font-medium text-gray-900 dark:text-gray-100">
                  {file.name}
                </p>
                <p className="text-sm text-gray-500 dark:text-gray-400">
                  {formatFileSize(file.size)}
                  {file.type ? ` · ${file.type}` : ''}
                </p>
              </div>
            </div>
          ) : (
            <>
              <svg
                xmlns="http://www.w3.org/2000/svg"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                strokeWidth={1.5}
                strokeLinecap="round"
                strokeLinejoin="round"
                className="mb-3 h-10 w-10 text-gray-500 dark:text-gray-500"
                aria-hidden="true"
              >
                <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" />
                <polyline points="17 8 12 3 7 8" />
                <line x1="12" y1="3" x2="12" y2="15" />
              </svg>
              <p className="text-base text-gray-700 dark:text-gray-300">
                Seret file ke sini atau ketuk untuk memilih
              </p>
            </>
          )}
        </div>

        {/* Hidden native file input + "Pilih File" button (Req 5.2) */}
        <input
          ref={fileInputRef}
          type="file"
          onChange={handleFileChange}
          className="hidden"
          aria-hidden="true"
        />
        <button
          type="button"
          onClick={openFileDialog}
          disabled={isFormDisabled}
          className="inline-flex min-h-[44px] touch-manipulation items-center justify-center gap-2 rounded-lg border border-gray-200 dark:border-dark-700 bg-white dark:bg-dark-800 px-4 py-2.5 text-sm font-medium text-gray-800 dark:text-gray-200 transition-colors hover:border-accent/60 hover:text-gray-900 dark:hover:text-white focus:outline-none focus:ring-2 focus:ring-accent/40 disabled:cursor-not-allowed disabled:opacity-60"
        >
          Pilih File
        </button>
      </div>

      {/* Expiry dropdown (Req 5.5) */}
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

      {/* Visibility radio buttons (Req 5.5) */}
      <fieldset className="space-y-2" disabled={isFormDisabled}>
        <legend className="text-sm font-medium text-gray-800 dark:text-gray-200">
          Visibilitas
        </legend>
        <div className="flex flex-col gap-2 md:flex-row md:flex-wrap md:gap-4">
          {visibilities.map((value) => (
            <label
              key={value}
              className="flex min-h-[44px] cursor-pointer items-center gap-2.5 rounded-lg border border-gray-200 dark:border-dark-700 bg-white dark:bg-dark-800 px-3.5 py-2.5 text-sm text-gray-800 dark:text-gray-200 transition-colors hover:border-accent/60 has-[:checked]:border-accent has-[:checked]:bg-accent/10"
            >
              <input
                type="radio"
                name="visibility"
                value={value}
                checked={visibility === value}
                onChange={() => setVisibility(value)}
                className="h-4 w-4 accent-accent"
              />
              {visibilityLabel(value)}
            </label>
          ))}
        </div>
      </fieldset>

      {/* Conditional password field (Req 5.5) */}
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

      {/* Progress bar (Req 5.7) */}
      {isUploading && (
        <div className="space-y-2" aria-live="polite">
          <div className="flex items-center justify-between text-sm text-gray-700 dark:text-gray-300">
            <span>Mengunggah...</span>
            <span>{progress}%</span>
          </div>
          <div
            className="h-2.5 w-full overflow-hidden rounded-full bg-gray-200 dark:bg-dark-700"
            role="progressbar"
            aria-valuenow={progress}
            aria-valuemin={0}
            aria-valuemax={100}
          >
            <div
              className="h-full rounded-full bg-accent transition-[width] duration-150 ease-out"
              style={{ width: `${progress}%` }}
            />
          </div>
        </div>
      )}

      {/* Error display (Req 5.9, 5.10) */}
      {error && (
        <div
          role="alert"
          className="rounded-lg border border-red-500/40 bg-red-500/10 px-4 py-3 text-sm text-red-600 dark:text-red-300"
        >
          {error}
        </div>
      )}

      {/* Upload button (Req 5.6) — disabled while uploading or with no file */}
      <div>
        <button
          type="button"
          onClick={handleUpload}
          disabled={!file || isFormDisabled || file.size > maxFileSize}
          className="inline-flex min-h-[44px] items-center justify-center gap-2 rounded-lg bg-accent px-6 py-2.5 font-medium text-white shadow-sm shadow-accent/30 transition-colors hover:bg-accent-hover focus:outline-none focus:ring-2 focus:ring-accent/50 disabled:cursor-not-allowed disabled:opacity-60"
        >
          {isUploading && (
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
          {isUploading ? 'Mengunggah...' : 'Unggah'}
        </button>
      </div>
    </div>
  );
}

export default FileUploader;
