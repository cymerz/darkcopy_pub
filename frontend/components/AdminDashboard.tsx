'use client';

// components/AdminDashboard.tsx
//
// Hidden admin dashboard (Client Component). It is intentionally NOT linked
// from the main navigation (Header) — it is reachable only by navigating
// directly to /admin.
//
// Flow:
// 1. Token gate: the admin enters a token which is verified against the backend
//    by calling GET /admin/stats. On success the token is kept in
//    sessionStorage (cleared on tab close / logout) and the dashboard loads.
// 2. Dashboard: shows stats, a list of all pastes and all files with delete
//    actions. Each list call carries the token via the X-Admin-Token header.
//
// The token only ever lives in component state + sessionStorage on the client;
// it is never baked into the bundle.

import { useCallback, useEffect, useState, useSyncExternalStore } from 'react';
import {
  getAdminStats,
  getAdminPastes,
  getAdminFiles,
  deleteAdminPaste,
  deleteAdminFile,
  purgeExpired,
  getAdminReports,
  updateAdminReportStatus,
  deleteAdminReport,
} from '@/lib/api';
import { APIError, REPORT_REASONS } from '@/lib/types';
import type {
  AdminStats,
  AdminPasteItem,
  AdminFileItem,
  AdminReport,
  ReportStatus,
} from '@/lib/types';
import { formatRelativeTime, formatFileSize } from '@/lib/utils';
import { AdminSettingsForm } from '@/components/AdminSettingsForm';

const TOKEN_STORAGE_KEY = 'darkcopy_admin_token';

// ---------------------------------------------------------------------------
// Token store (hydration-safe via useSyncExternalStore)
//
// The admin token lives in sessionStorage. Reading it during render directly
// would cause an SSR/client hydration mismatch (server has no sessionStorage),
// so we expose it as an external store: the server snapshot is always null, and
// the client snapshot is read from sessionStorage after hydration.
// ---------------------------------------------------------------------------

const tokenListeners = new Set<() => void>();

function readStoredToken(): string | null {
  if (typeof window === 'undefined') return null;
  return sessionStorage.getItem(TOKEN_STORAGE_KEY);
}

function subscribeToken(callback: () => void): () => void {
  tokenListeners.add(callback);
  const onStorage = (e: StorageEvent) => {
    if (e.key === TOKEN_STORAGE_KEY) callback();
  };
  window.addEventListener('storage', onStorage);
  return () => {
    tokenListeners.delete(callback);
    window.removeEventListener('storage', onStorage);
  };
}

function notifyTokenListeners(): void {
  tokenListeners.forEach((l) => l());
}

function setStoredToken(token: string): void {
  sessionStorage.setItem(TOKEN_STORAGE_KEY, token);
  notifyTokenListeners();
}

function clearStoredToken(): void {
  sessionStorage.removeItem(TOKEN_STORAGE_KEY);
  notifyTokenListeners();
}

const VISIBILITY_LABELS: Record<string, string> = {
  public: 'Publik',
  unlisted: 'Tidak Terdaftar',
  password_protected: 'Dilindungi Sandi',
};

function VisibilityBadge({ visibility }: { visibility: string }) {
  return (
    <span className="rounded-full bg-gray-100 dark:bg-dark-700 px-2.5 py-0.5 text-xs font-medium text-gray-700 dark:text-gray-300">
      {VISIBILITY_LABELS[visibility] ?? visibility}
    </span>
  );
}

function isExpired(expiresAt: string | null): boolean {
  if (!expiresAt) return false;
  return new Date(expiresAt).getTime() < Date.now();
}

const REASON_LABELS: Record<string, string> = Object.fromEntries(
  REPORT_REASONS.map((r) => [r.value, r.label]),
);

function reasonLabel(reason: string): string {
  return REASON_LABELS[reason] ?? reason;
}

// ---------------------------------------------------------------------------
// Token gate
// ---------------------------------------------------------------------------

function TokenGate() {
  const [token, setToken] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const handleSubmit = async (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    const trimmed = token.trim();
    if (!trimmed || loading) return;

    setLoading(true);
    setError(null);
    try {
      // Verify the token by hitting a protected endpoint, then persist it.
      // Persisting notifies the token store, which re-renders the dashboard.
      await getAdminStats(trimmed);
      setStoredToken(trimmed);
    } catch (err) {
      if (err instanceof APIError) {
        setError(
          err.status === 404
            ? 'Admin API tidak aktif di server.'
            : 'Token admin tidak valid.',
        );
      } else {
        setError('Terjadi kesalahan. Silakan coba lagi.');
      }
      setLoading(false);
    }
  };

  return (
    <div className="mx-auto flex w-full max-w-md flex-col items-center px-4 py-12">
      <div className="w-full rounded-xl border border-gray-200 dark:border-dark-700 bg-white dark:bg-dark-800 p-6 shadow-lg sm:p-8">
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
          Panel Admin
        </h1>
        <p className="mb-6 text-center text-sm text-gray-500 dark:text-gray-400">
          Masukkan token admin untuk melanjutkan.
        </p>

        <form onSubmit={handleSubmit} className="space-y-4" noValidate>
          <div className="space-y-2">
            <label
              htmlFor="admin-token"
              className="block text-sm font-medium text-gray-800 dark:text-gray-200"
            >
              Token Admin
            </label>
            <input
              type="password"
              id="admin-token"
              name="admin-token"
              value={token}
              onChange={(e) => setToken(e.target.value)}
              disabled={loading}
              autoComplete="off"
              autoFocus
              placeholder="Masukkan token"
              className="w-full min-h-[44px] rounded-lg border border-gray-200 dark:border-dark-700 bg-white dark:bg-dark-900 px-3 py-2.5 text-gray-900 dark:text-gray-100 placeholder-gray-400 dark:placeholder-gray-500 transition-colors focus:border-accent focus:outline-none focus:ring-2 focus:ring-accent/40 disabled:cursor-not-allowed disabled:opacity-60"
            />
          </div>

          {error && (
            <div
              role="alert"
              className="rounded-lg border border-red-500/40 bg-red-500/10 px-4 py-3 text-sm text-red-600 dark:text-red-300"
            >
              {error}
            </div>
          )}

          <button
            type="submit"
            disabled={loading}
            className="inline-flex min-h-[44px] w-full items-center justify-center gap-2 rounded-lg bg-accent px-6 py-2.5 font-medium text-white shadow-sm shadow-accent/30 transition-colors hover:bg-accent-hover focus:outline-none focus:ring-2 focus:ring-accent/50 disabled:cursor-not-allowed disabled:opacity-60"
          >
            {loading ? 'Memverifikasi...' : 'Masuk'}
          </button>
        </form>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Dashboard
// ---------------------------------------------------------------------------

type Tab = 'overview' | 'pastes' | 'files' | 'reports' | 'settings';

function Dashboard({ token, onLogout }: { token: string; onLogout: () => void }) {
  const [stats, setStats] = useState<AdminStats | null>(null);
  const [pastes, setPastes] = useState<AdminPasteItem[]>([]);
  const [files, setFiles] = useState<AdminFileItem[]>([]);
  const [reports, setReports] = useState<AdminReport[]>([]);
  const [tab, setTab] = useState<Tab>('overview');
  const [loading, setLoading] = useState(true);
  const [loadingPastes, setLoadingPastes] = useState(false);
  const [loadingFiles, setLoadingFiles] = useState(false);
  const [loadingReports, setLoadingReports] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [busySlug, setBusySlug] = useState<string | null>(null);
  const [busyReportId, setBusyReportId] = useState<string | null>(null);
  const [purging, setPurging] = useState(false);
  const [notice, setNotice] = useState<string | null>(null);
  // Bumping this key re-runs the load effect (used by the "Muat Ulang" button).
  const [reloadKey, setReloadKey] = useState(0);

  const reload = useCallback(() => {
    setLoading(true);
    setReloadKey((k) => k + 1);
  }, []);

  const topPastes = stats?.top_pastes || [];
  const topFiles = stats?.top_files || [];

  // 1. Fetch Stats on mount / reload (always needed for overview & badges)
  useEffect(() => {
    let cancelled = false;

    (async () => {
      try {
        const statsRes = await getAdminStats(token);
        if (cancelled) return;
        setStats(statsRes);
        setError(null);
      } catch (err) {
        if (cancelled) return;
        if (
          err instanceof APIError &&
          (err.status === 401 || err.status === 404)
        ) {
          // Token became invalid (or admin API disabled) — force re-auth.
          onLogout();
          return;
        }
        setError('Gagal memuat statistik admin.');
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [token, onLogout, reloadKey]);

  // 2. Fetch Pastes lazily when on pastes tab
  useEffect(() => {
    if (tab !== 'pastes') return;
    let cancelled = false;

    (async () => {
      try {
        setLoadingPastes(true);
        const pastesRes = await getAdminPastes(token);
        if (cancelled) return;
        setPastes(pastesRes.pastes ?? []);
        setError(null);
      } catch (err) {
        if (cancelled) return;
        if (
          err instanceof APIError &&
          (err.status === 401 || err.status === 404)
        ) {
          onLogout();
          return;
        }
        setError('Gagal memuat daftar paste.');
      } finally {
        if (!cancelled) setLoadingPastes(false);
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [token, onLogout, tab, reloadKey]);

  // 3. Fetch Files lazily when on files tab
  useEffect(() => {
    if (tab !== 'files') return;
    let cancelled = false;

    (async () => {
      try {
        setLoadingFiles(true);
        const filesRes = await getAdminFiles(token);
        if (cancelled) return;
        setFiles(filesRes.files ?? []);
        setError(null);
      } catch (err) {
        if (cancelled) return;
        if (
          err instanceof APIError &&
          (err.status === 401 || err.status === 404)
        ) {
          onLogout();
          return;
        }
        setError('Gagal memuat daftar file.');
      } finally {
        if (!cancelled) setLoadingFiles(false);
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [token, onLogout, tab, reloadKey]);

  // 4. Fetch Reports lazily when on reports tab
  useEffect(() => {
    if (tab !== 'reports') return;
    let cancelled = false;

    (async () => {
      try {
        setLoadingReports(true);
        const reportsRes = await getAdminReports(token);
        if (cancelled) return;
        setReports(reportsRes.reports ?? []);
        setError(null);
      } catch (err) {
        if (cancelled) return;
        if (
          err instanceof APIError &&
          (err.status === 401 || err.status === 404)
        ) {
          onLogout();
          return;
        }
        setError('Gagal memuat daftar laporan.');
      } finally {
        if (!cancelled) setLoadingReports(false);
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [token, onLogout, tab, reloadKey]);

  const handleDeletePaste = async (slug: string) => {
    if (!window.confirm(`Hapus paste "${slug}"? Tindakan ini tidak dapat dibatalkan.`)) {
      return;
    }
    setBusySlug(slug);
    try {
      await deleteAdminPaste(token, slug);
      setPastes((prev) => prev.filter((p) => p.slug !== slug));
      setStats((prev) =>
        prev ? { ...prev, total_pastes: Math.max(0, prev.total_pastes - 1) } : prev,
      );
    } catch {
      setError(`Gagal menghapus paste "${slug}".`);
    } finally {
      setBusySlug(null);
    }
  };

  const handleDeleteFile = async (slug: string) => {
    if (!window.confirm(`Hapus file "${slug}"? Tindakan ini tidak dapat dibatalkan.`)) {
      return;
    }
    setBusySlug(slug);
    try {
      await deleteAdminFile(token, slug);
      setFiles((prev) => prev.filter((f) => f.slug !== slug));
      setStats((prev) =>
        prev ? { ...prev, total_files: Math.max(0, prev.total_files - 1) } : prev,
      );
    } catch {
      setError(`Gagal menghapus file "${slug}".`);
    } finally {
      setBusySlug(null);
    }
  };

  const handleReportStatus = async (id: string, status: ReportStatus) => {
    setBusyReportId(id);
    try {
      await updateAdminReportStatus(token, id, status);
      setReports((prev) =>
        prev.map((r) => (r.id === id ? { ...r, status } : r)),
      );
    } catch {
      setError('Gagal memperbarui status laporan.');
    } finally {
      setBusyReportId(null);
    }
  };

  // Delete the actual reported content (paste or file). The report itself is
  // then removed too, since the content it points to no longer exists.
  const handleDeleteReportedContent = async (
    id: string,
    resourceType: 'paste' | 'file',
    slug: string,
  ) => {
    const label = resourceType === 'file' ? 'file' : 'paste';
    if (
      !window.confirm(
        `Hapus ${label} "${slug}" yang dilaporkan? Konten akan dihapus permanen.`,
      )
    ) {
      return;
    }
    setBusyReportId(id);
    try {
      if (resourceType === 'file') {
        await deleteAdminFile(token, slug);
        setFiles((prev) => prev.filter((f) => f.slug !== slug));
        setStats((prev) =>
          prev ? { ...prev, total_files: Math.max(0, prev.total_files - 1) } : prev,
        );
      } else {
        await deleteAdminPaste(token, slug);
        setPastes((prev) => prev.filter((p) => p.slug !== slug));
        setStats((prev) =>
          prev ? { ...prev, total_pastes: Math.max(0, prev.total_pastes - 1) } : prev,
        );
      }
      // Remove the report record once its target content is gone.
      await deleteAdminReport(token, id).catch(() => {
        /* Content is already deleted; a leftover report row is harmless. */
      });
      setReports((prev) => prev.filter((r) => r.id !== id));
    } catch (err) {
      if (err instanceof APIError && err.status === 404) {
        // Content was already deleted — drop the report from the list too.
        await deleteAdminReport(token, id).catch(() => {});
        setReports((prev) => prev.filter((r) => r.id !== id));
        return;
      }
      setError(`Gagal menghapus ${label} "${slug}".`);
    } finally {
      setBusyReportId(null);
    }
  };

  const handleDeleteReport = async (id: string) => {
    if (!window.confirm('Hapus laporan ini?')) return;
    setBusyReportId(id);
    try {
      await deleteAdminReport(token, id);
      setReports((prev) => prev.filter((r) => r.id !== id));
    } catch {
      setError('Gagal menghapus laporan.');
    } finally {
      setBusyReportId(null);
    }
  };

  const handlePurge = async () => {
    if (
      !window.confirm(
        'Bersihkan semua paste dan file yang sudah kadaluarsa sekarang? Tindakan ini tidak dapat dibatalkan.',
      )
    ) {
      return;
    }
    setPurging(true);
    setError(null);
    setNotice(null);
    try {
      const { deleted } = await purgeExpired(token);
      setNotice(
        deleted > 0
          ? `${deleted} item kadaluarsa telah dibersihkan.`
          : 'Tidak ada item kadaluarsa untuk dibersihkan.',
      );
      reload();
    } catch (err) {
      if (err instanceof APIError && (err.status === 401 || err.status === 404)) {
        onLogout();
        return;
      }
      setError('Gagal membersihkan item kadaluarsa.');
    } finally {
      setPurging(false);
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex flex-col md:flex-row md:items-center md:justify-between gap-4">
        <h1 className="text-2xl font-bold text-gray-900 dark:text-gray-100 md:text-3xl">
          Panel Admin
        </h1>
        <div className="flex flex-col sm:flex-row gap-2 w-full md:w-auto">
          <button
            type="button"
            onClick={handlePurge}
            disabled={loading || purging}
            className="inline-flex min-h-[40px] w-full sm:w-auto items-center justify-center rounded-lg border border-amber-500/40 bg-amber-500/10 px-4 py-2 text-sm font-medium text-amber-600 dark:text-amber-300 transition-colors hover:bg-amber-500/20 disabled:opacity-60"
          >
            {purging ? 'Membersihkan...' : 'Bersihkan Kadaluarsa'}
          </button>
          <button
            type="button"
            onClick={reload}
            disabled={loading}
            className="inline-flex min-h-[40px] w-full sm:w-auto items-center justify-center rounded-lg border border-gray-200 dark:border-dark-700 bg-white dark:bg-dark-800 px-4 py-2 text-sm font-medium text-gray-800 dark:text-gray-200 transition-colors hover:border-accent/60 hover:text-gray-900 dark:hover:text-white disabled:opacity-60"
          >
            Muat Ulang
          </button>
          <button
            type="button"
            onClick={onLogout}
            className="inline-flex min-h-[40px] w-full sm:w-auto items-center justify-center rounded-lg border border-gray-200 dark:border-dark-700 bg-white dark:bg-dark-800 px-4 py-2 text-sm font-medium text-gray-800 dark:text-gray-200 transition-colors hover:border-red-500/60 hover:text-red-600 dark:hover:text-red-300"
          >
            Keluar
          </button>
        </div>
      </div>

      {notice && (
        <div
          role="status"
          className="rounded-lg border border-emerald-500/40 bg-emerald-500/10 px-4 py-3 text-sm text-emerald-600 dark:text-emerald-300"
        >
          {notice}
        </div>
      )}

      {error && (
        <div
          role="alert"
          className="rounded-lg border border-red-500/40 bg-red-500/10 px-4 py-3 text-sm text-red-600 dark:text-red-300"
        >
          {error}
        </div>
      )}

      {/* Tabs */}
      <div className="flex gap-2 border-b border-gray-200 dark:border-dark-700 overflow-x-auto scrollbar-none whitespace-nowrap -mx-4 px-4 sm:mx-0 sm:px-0">
        <button
          type="button"
          onClick={() => setTab('overview')}
          className={`shrink-0 min-h-[44px] px-4 py-2 text-sm font-medium transition-colors ${
            tab === 'overview'
              ? 'border-b-2 border-accent text-gray-900 dark:text-white'
              : 'text-gray-500 dark:text-gray-400 hover:text-gray-800 dark:hover:text-gray-200'
          }`}
        >
          Ringkasan
        </button>
        <button
          type="button"
          onClick={() => setTab('pastes')}
          className={`shrink-0 min-h-[44px] px-4 py-2 text-sm font-medium transition-colors ${
            tab === 'pastes'
              ? 'border-b-2 border-accent text-gray-900 dark:text-white'
              : 'text-gray-500 dark:text-gray-400 hover:text-gray-800 dark:hover:text-gray-200'
          }`}
        >
          Paste ({stats ? stats.total_pastes : 0})
        </button>
        <button
          type="button"
          onClick={() => setTab('files')}
          className={`shrink-0 min-h-[44px] px-4 py-2 text-sm font-medium transition-colors ${
            tab === 'files'
              ? 'border-b-2 border-accent text-gray-900 dark:text-white'
              : 'text-gray-500 dark:text-gray-400 hover:text-gray-800 dark:hover:text-gray-200'
          }`}
        >
          File ({stats ? stats.total_files : 0})
        </button>
        <button
          type="button"
          onClick={() => setTab('reports')}
          className={`shrink-0 relative min-h-[44px] px-4 py-2 text-sm font-medium transition-colors ${
            tab === 'reports'
              ? 'border-b-2 border-accent text-gray-900 dark:text-white'
              : 'text-gray-500 dark:text-gray-400 hover:text-gray-800 dark:hover:text-gray-200'
          }`}
        >
          Laporan ({stats ? stats.pending_reports : 0})
          {stats && stats.pending_reports > 0 && (
            <span className="ml-1.5 inline-flex items-center justify-center rounded-full bg-red-600 px-1.5 py-0.5 text-[10px] font-bold leading-none text-white">
              {stats.pending_reports}
            </span>
          )}
        </button>
        <button
          type="button"
          onClick={() => setTab('settings')}
          className={`shrink-0 min-h-[44px] px-4 py-2 text-sm font-medium transition-colors ${
            tab === 'settings'
              ? 'border-b-2 border-accent text-gray-900 dark:text-white'
              : 'text-gray-500 dark:text-gray-400 hover:text-gray-800 dark:hover:text-gray-200'
          }`}
        >
          Pengaturan
        </button>
      </div>

      {tab === 'settings' ? (
        <AdminSettingsForm token={token} onUnauthorized={onLogout} />
      ) : loading ? (
        <p className="py-8 text-center text-gray-500 dark:text-gray-400">Memuat...</p>
      ) : tab === 'overview' ? (
        <div className="space-y-6">
          {/* Stats */}
          <div className="grid grid-cols-2 gap-3 sm:grid-cols-4 sm:max-w-4xl">
            <div className="rounded-xl border border-gray-200 dark:border-dark-700 bg-white dark:bg-dark-800 p-4 shadow-sm">
              <p className="text-sm text-gray-500 dark:text-gray-400">Total Paste</p>
              <p className="mt-1 text-2xl font-bold text-gray-900 dark:text-gray-100">
                {stats ? stats.total_pastes : '—'}
              </p>
            </div>
            <div className="rounded-xl border border-gray-200 dark:border-dark-700 bg-white dark:bg-dark-800 p-4 shadow-sm">
              <p className="text-sm text-gray-500 dark:text-gray-400">Total File</p>
              <p className="mt-1 text-2xl font-bold text-gray-900 dark:text-gray-100">
                {stats ? stats.total_files : '—'}
              </p>
            </div>
            <div className="rounded-xl border border-gray-200 dark:border-dark-700 bg-white dark:bg-dark-800 p-4 shadow-sm">
              <p className="text-sm text-gray-500 dark:text-gray-400">Ukuran Penyimpanan</p>
              <p className="mt-1 text-2xl font-bold text-gray-900 dark:text-gray-100">
                {stats && stats.total_bytes !== undefined ? formatFileSize(stats.total_bytes) : '—'}
              </p>
            </div>
            <div className="rounded-xl border border-gray-200 dark:border-dark-700 bg-white dark:bg-dark-800 p-4 shadow-sm">
              <p className="text-sm text-gray-500 dark:text-gray-400">Laporan Tertunda</p>
              <p
                className={`mt-1 text-2xl font-bold ${
                  stats && stats.pending_reports > 0
                    ? 'text-red-600 dark:text-red-400'
                    : 'text-gray-900 dark:text-gray-100'
                }`}
              >
                {stats ? stats.pending_reports : '—'}
              </p>
            </div>
          </div>

          {/* S3 Storage Sharding Statistics */}
          {stats && stats.provider_stats && stats.provider_stats.length > 0 && (
            <div className="rounded-xl border border-gray-200 dark:border-dark-700 bg-white dark:bg-dark-800/80 p-5 shadow-lg backdrop-blur-sm">
              <h2 className="text-sm font-semibold uppercase tracking-wider text-gray-400 dark:text-gray-500 mb-4 flex items-center gap-2">
                <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="h-4 w-4 text-accent">
                  <path d="M12 2v20M17 5H9.5a3.5 3.5 0 0 0 0 7h5a3.5 3.5 0 0 1 0 7H6" />
                </svg>
                Status Distribusi Sharding S3 Cloud
              </h2>
              <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
                {stats.provider_stats.map((p) => {
                  const totalBytes = stats.total_bytes || 1;
                  const percentage = Math.min(100, Math.round((p.size_bytes / totalBytes) * 100));
                  
                  // Dynamic brand coloring for popular cloud providers!
                  const isB2 = p.provider_name.includes("B2") || p.provider_name.includes("BLACKBLAZE") || p.provider_name.includes("BACKBLAZE");
                  const isFilebase = p.provider_name.includes("FILEBASE");
                  const isR2 = p.provider_name.includes("R2") || p.provider_name.includes("CLOUDFLARE");
                  
                  let brandColorCls = "bg-accent shadow-accent/20";
                  let bgTagCls = "bg-accent/10 text-accent";
                  
                  if (isB2) {
                    brandColorCls = "bg-red-500 shadow-red-500/20";
                    bgTagCls = "bg-red-500/10 text-red-600 dark:text-red-300";
                  } else if (isFilebase) {
                    brandColorCls = "bg-blue-500 shadow-blue-500/20";
                    bgTagCls = "bg-blue-500/10 text-blue-600 dark:text-blue-300";
                  } else if (isR2) {
                    brandColorCls = "bg-orange-500 shadow-orange-500/20";
                    bgTagCls = "bg-orange-500/10 text-orange-600 dark:text-orange-300";
                  }

                  return (
                    <div key={p.provider_name} className="flex flex-col justify-between rounded-xl border border-gray-150 dark:border-dark-700 bg-gray-50/50 dark:bg-dark-900/40 p-4 transition-all hover:scale-[1.01] hover:border-gray-200 dark:hover:border-dark-600 shadow-sm">
                      <div>
                        <div className="flex items-center justify-between gap-2 mb-3">
                          <span className="font-mono font-bold text-sm text-gray-900 dark:text-gray-100 flex items-center gap-1.5">
                            <span className={`inline-block h-2.5 w-2.5 rounded-full ${brandColorCls}`} />
                            {p.provider_name}
                          </span>
                          <span className={`rounded-full px-2 py-0.5 text-xs font-semibold tracking-wide uppercase ${bgTagCls}`}>
                            {percentage}%
                          </span>
                        </div>
                        <div className="space-y-1.5">
                          <div className="flex justify-between text-xs text-gray-500 dark:text-gray-400">
                            <span>Penyimpanan Terpakai</span>
                            <span className="font-medium text-gray-800 dark:text-gray-200">{formatFileSize(p.size_bytes)}</span>
                          </div>
                          <div className="flex justify-between text-xs text-gray-500 dark:text-gray-400">
                            <span>Berkas Tersimpan</span>
                            <span className="font-medium text-gray-800 dark:text-gray-200">{p.files_count} file</span>
                          </div>
                        </div>
                      </div>
                      
                      {/* Progress bar */}
                      <div className="mt-4">
                        <div className="h-1.5 w-full rounded-full bg-gray-200 dark:bg-dark-800 overflow-hidden">
                          <div 
                            className={`h-full rounded-full transition-all duration-500 ${brandColorCls}`}
                            style={{ width: `${percentage}%` }}
                          />
                        </div>
                      </div>
                    </div>
                  );
                })}
              </div>
            </div>
          )}

          {/* Top Performing Content Panel */}
          <div className="grid gap-6 md:grid-cols-2">
            {/* Top Pastes Card */}
            <div className="rounded-xl border border-gray-200 dark:border-dark-700 bg-white dark:bg-dark-800 p-5 shadow-lg">
              <h2 className="text-sm font-semibold uppercase tracking-wider text-gray-400 dark:text-gray-500 mb-4 flex items-center gap-2">
                <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="h-4 w-4 text-accent">
                  <path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z" />
                  <circle cx="12" cy="12" r="3" />
                </svg>
                Top 5 Paste Terpopuler (Views)
              </h2>
              {topPastes.length === 0 ? (
                <p className="text-sm text-gray-500 dark:text-gray-400 py-4 text-center">Belum ada data</p>
              ) : (
                <div className="divide-y divide-gray-100 dark:divide-dark-700/60">
                  {topPastes.map((p, idx) => (
                    <div key={p.slug} className="flex items-center justify-between py-3 gap-3">
                      <div className="min-w-0 flex-1">
                        <div className="flex items-center gap-2">
                          <span className="text-xs font-bold text-gray-400 dark:text-gray-500 font-mono w-4">
                            #{idx + 1}
                          </span>
                          <a
                            href={`/${p.slug}`}
                            target="_blank"
                            rel="noopener noreferrer"
                            className="truncate text-sm font-medium text-gray-900 dark:text-gray-100 hover:text-accent-hover"
                          >
                            {p.title.trim() || 'Untitled'}
                          </a>
                        </div>
                        <div className="mt-1 flex items-center gap-2 text-xs text-gray-500 dark:text-gray-500 pl-6">
                          <code>{p.slug}</code>
                          <span>•</span>
                          <span>{p.language}</span>
                        </div>
                      </div>
                      <span className="shrink-0 inline-flex items-center gap-1 rounded-full bg-accent/10 px-2.5 py-0.5 text-xs font-semibold text-accent dark:text-accent-hover">
                        {p.views || 0} views
                      </span>
                    </div>
                  ))}
                </div>
              )}
            </div>

            {/* Top Files Card */}
            <div className="rounded-xl border border-gray-200 dark:border-dark-700 bg-white dark:bg-dark-800 p-5 shadow-lg">
              <h2 className="text-sm font-semibold uppercase tracking-wider text-gray-400 dark:text-gray-500 mb-4 flex items-center gap-2">
                <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="h-4 w-4 text-accent">
                  <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" />
                  <polyline points="7 10 12 15 17 10" />
                  <line x1="12" y1="15" x2="12" y2="3" />
                </svg>
                Top 5 File Terpopuler (Unduhan)
              </h2>
              {topFiles.length === 0 ? (
                <p className="text-sm text-gray-500 dark:text-gray-400 py-4 text-center">Belum ada data</p>
              ) : (
                <div className="divide-y divide-gray-100 dark:divide-dark-700/60">
                  {topFiles.map((f, idx) => (
                    <div key={f.slug} className="flex items-center justify-between py-3 gap-3">
                      <div className="min-w-0 flex-1">
                        <div className="flex items-center gap-2">
                          <span className="text-xs font-bold text-gray-400 dark:text-gray-500 font-mono w-4">
                            #{idx + 1}
                          </span>
                          <a
                            href={`/f/${f.slug}`}
                            target="_blank"
                            rel="noopener noreferrer"
                            className="truncate text-sm font-medium text-gray-900 dark:text-gray-100 hover:text-accent-hover"
                          >
                            {f.filename}
                          </a>
                        </div>
                        <div className="mt-1 flex items-center gap-2 text-xs text-gray-500 dark:text-gray-500 pl-6">
                          <code>{f.slug}</code>
                          <span>•</span>
                          <span>{formatFileSize(f.size_bytes)}</span>
                        </div>
                      </div>
                      <span className="shrink-0 inline-flex items-center gap-1 rounded-full bg-emerald-500/10 px-2.5 py-0.5 text-xs font-semibold text-emerald-600 dark:text-emerald-300">
                        {f.downloads || 0} unduhan
                      </span>
                    </div>
                  ))}
                </div>
              )}
            </div>
          </div>
        </div>
      ) : tab === 'pastes' ? (
        loadingPastes ? (
          <p className="py-8 text-center text-gray-500 dark:text-gray-400">Memuat daftar paste...</p>
        ) : (
          <PasteTable
            pastes={pastes}
            busySlug={busySlug}
            onDelete={handleDeletePaste}
          />
        )
      ) : tab === 'files' ? (
        loadingFiles ? (
          <p className="py-8 text-center text-gray-500 dark:text-gray-400">Memuat daftar file...</p>
        ) : (
          <FileTable files={files} busySlug={busySlug} onDelete={handleDeleteFile} />
        )
      ) : (
        loadingReports ? (
          <p className="py-8 text-center text-gray-500 dark:text-gray-400">Memuat daftar laporan...</p>
        ) : (
          <ReportsTable
            reports={reports}
            busyId={busyReportId}
            onStatus={handleReportStatus}
            onDeleteContent={handleDeleteReportedContent}
            onDelete={handleDeleteReport}
          />
        )
      )}
    </div>
  );
}

function PasteTable({
  pastes,
  busySlug,
  onDelete,
}: {
  pastes: AdminPasteItem[];
  busySlug: string | null;
  onDelete: (slug: string) => void;
}) {
  if (pastes.length === 0) {
    return (
      <div className="rounded-xl border border-dashed border-gray-200 dark:border-dark-700 bg-gray-50 dark:bg-dark-800/50 px-6 py-12 text-center text-gray-700 dark:text-gray-300">
        Belum ada paste.
      </div>
    );
  }

  return (
    <ul className="space-y-2">
      {pastes.map((p) => {
        const expired = isExpired(p.expires_at);
        return (
          <li
            key={p.slug}
            className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 rounded-xl border border-gray-200 dark:border-dark-700 bg-white dark:bg-dark-800 p-4"
          >
            <div className="min-w-0 flex-1">
              <div className="flex items-center gap-2">
                <a
                  href={`/${p.slug}`}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="truncate font-medium text-gray-900 dark:text-gray-100 hover:text-accent-hover"
                >
                  {p.title.trim() || 'Untitled'}
                </a>
                {expired && (
                  <span className="rounded-full bg-red-500/15 px-2 py-0.5 text-xs font-medium text-red-600 dark:text-red-300">
                    Kadaluarsa
                  </span>
                )}
              </div>
              <div className="mt-1 flex flex-wrap items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
                <code className="text-gray-500 dark:text-gray-500">{p.slug}</code>
                <span className="rounded-full bg-accent/10 dark:bg-accent/15 px-2 py-0.5 font-medium text-accent dark:text-accent-hover">
                  {p.language}
                </span>
                <VisibilityBadge visibility={p.visibility} />
                <span>{formatRelativeTime(p.created_at)}</span>
                <span>• Dilihat {p.views ?? 0} kali</span>
              </div>
            </div>
            <button
              type="button"
              onClick={() => onDelete(p.slug)}
              disabled={busySlug === p.slug}
              className="inline-flex min-h-[40px] w-full sm:w-auto shrink-0 items-center justify-center rounded-lg border border-red-500/40 bg-red-500/10 px-4 py-2 text-sm font-medium text-red-600 dark:text-red-300 transition-colors hover:bg-red-500/20 disabled:opacity-60"
            >
              {busySlug === p.slug ? 'Menghapus...' : 'Hapus'}
            </button>
          </li>
        );
      })}
    </ul>
  );
}

function FileTable({
  files,
  busySlug,
  onDelete,
}: {
  files: AdminFileItem[];
  busySlug: string | null;
  onDelete: (slug: string) => void;
}) {
  if (files.length === 0) {
    return (
      <div className="rounded-xl border border-dashed border-gray-200 dark:border-dark-700 bg-gray-50 dark:bg-dark-800/50 px-6 py-12 text-center text-gray-700 dark:text-gray-300">
        Belum ada file.
      </div>
    );
  }

  return (
    <ul className="space-y-2">
      {files.map((f) => {
        const expired = isExpired(f.expires_at);
        return (
          <li
            key={f.slug}
            className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 rounded-xl border border-gray-200 dark:border-dark-700 bg-white dark:bg-dark-800 p-4"
          >
            <div className="min-w-0 flex-1">
              <div className="flex items-center gap-2">
                <a
                  href={`/f/${f.slug}`}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="truncate font-medium text-gray-900 dark:text-gray-100 hover:text-accent-hover"
                >
                  {f.filename}
                </a>
                {expired && (
                  <span className="rounded-full bg-red-500/15 px-2 py-0.5 text-xs font-medium text-red-600 dark:text-red-300">
                    Kadaluarsa
                  </span>
                )}
              </div>
              <div className="mt-1 flex flex-wrap items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
                <code className="text-gray-500 dark:text-gray-500">{f.slug}</code>
                <span>{formatFileSize(f.size_bytes)}</span>
                <VisibilityBadge visibility={f.visibility} />
                <span>{formatRelativeTime(f.created_at)}</span>
                <span>• Diunduh {f.downloads ?? 0} kali</span>
              </div>
            </div>
            <button
              type="button"
              onClick={() => onDelete(f.slug)}
              disabled={busySlug === f.slug}
              className="inline-flex min-h-[40px] w-full sm:w-auto shrink-0 items-center justify-center rounded-lg border border-red-500/40 bg-red-500/10 px-4 py-2 text-sm font-medium text-red-600 dark:text-red-300 transition-colors hover:bg-red-500/20 disabled:opacity-60"
            >
              {busySlug === f.slug ? 'Menghapus...' : 'Hapus'}
            </button>
          </li>
        );
      })}
    </ul>
  );
}

const REPORT_STATUS_LABELS: Record<string, string> = {
  pending: 'Tertunda',
  reviewed: 'Ditinjau',
  dismissed: 'Diabaikan',
};

function ReportStatusBadge({ status }: { status: ReportStatus }) {
  const cls =
    status === 'pending'
      ? 'bg-red-500/15 text-red-600 dark:text-red-300'
      : status === 'reviewed'
        ? 'bg-emerald-500/15 text-emerald-600 dark:text-emerald-300'
        : 'bg-gray-100 dark:bg-dark-700 text-gray-600 dark:text-gray-400';
  return (
    <span className={`rounded-full px-2 py-0.5 text-xs font-medium ${cls}`}>
      {REPORT_STATUS_LABELS[status] ?? status}
    </span>
  );
}

function ReportsTable({
  reports,
  busyId,
  onStatus,
  onDeleteContent,
  onDelete,
}: {
  reports: AdminReport[];
  busyId: string | null;
  onStatus: (id: string, status: ReportStatus) => void;
  onDeleteContent: (
    id: string,
    resourceType: 'paste' | 'file',
    slug: string,
  ) => void;
  onDelete: (id: string) => void;
}) {
  if (reports.length === 0) {
    return (
      <div className="rounded-xl border border-dashed border-gray-200 dark:border-dark-700 bg-gray-50 dark:bg-dark-800/50 px-6 py-12 text-center text-gray-700 dark:text-gray-300">
        Belum ada laporan.
      </div>
    );
  }

  return (
    <ul className="space-y-2">
      {reports.map((r) => {
        const href = r.resource_type === 'file' ? `/f/${r.slug}` : `/${r.slug}`;
        const busy = busyId === r.id;
        return (
          <li
            key={r.id}
            className="space-y-3 rounded-xl border border-gray-200 dark:border-dark-700 bg-white dark:bg-dark-800 p-4"
          >
            <div className="flex flex-wrap items-center gap-2">
              <span className="rounded-full bg-accent/10 dark:bg-accent/15 px-2 py-0.5 text-xs font-medium text-accent dark:text-accent-hover">
                {r.resource_type === 'file' ? 'File' : 'Paste'}
              </span>
              <span className="rounded-full border border-gray-200 dark:border-dark-600 px-2 py-0.5 text-xs font-medium text-gray-700 dark:text-gray-300">
                {reasonLabel(r.reason)}
              </span>
              <ReportStatusBadge status={r.status} />
              <a
                href={href}
                target="_blank"
                rel="noopener noreferrer"
                className="font-mono text-xs text-accent hover:underline dark:text-accent-hover"
              >
                {r.slug}
              </a>
              <span className="ml-auto text-xs text-gray-500 dark:text-gray-400">
                {formatRelativeTime(r.created_at)}
              </span>
            </div>

            {r.details && (
              <p className="whitespace-pre-wrap break-words rounded-lg bg-gray-50 dark:bg-dark-900/60 px-3 py-2 text-sm text-gray-700 dark:text-gray-300">
                {r.details}
              </p>
            )}

            <div className="grid grid-cols-2 gap-2 sm:flex sm:flex-wrap sm:items-center sm:gap-2 w-full">
              {r.status !== 'reviewed' && (
                <button
                  type="button"
                  onClick={() => onStatus(r.id, 'reviewed')}
                  disabled={busy}
                  className="inline-flex min-h-[36px] items-center justify-center rounded-lg border border-emerald-500/40 bg-emerald-500/10 px-3 py-1.5 text-xs font-medium text-emerald-600 dark:text-emerald-300 transition-colors hover:bg-emerald-500/20 disabled:opacity-60"
                >
                  Tandai Ditinjau
                </button>
              )}
              {r.status !== 'dismissed' && (
                <button
                  type="button"
                  onClick={() => onStatus(r.id, 'dismissed')}
                  disabled={busy}
                  className="inline-flex min-h-[36px] items-center justify-center rounded-lg border border-gray-200 dark:border-dark-600 bg-white dark:bg-dark-800 px-3 py-1.5 text-xs font-medium text-gray-700 dark:text-gray-300 transition-colors hover:border-accent/60 disabled:opacity-60"
                >
                  Abaikan
                </button>
              )}
              {r.status !== 'pending' && (
                <button
                  type="button"
                  onClick={() => onStatus(r.id, 'pending')}
                  disabled={busy}
                  className="inline-flex min-h-[36px] items-center justify-center rounded-lg border border-gray-200 dark:border-dark-600 bg-white dark:bg-dark-800 px-3 py-1.5 text-xs font-medium text-gray-700 dark:text-gray-300 transition-colors hover:border-accent/60 disabled:opacity-60"
                >
                  Kembalikan ke Tertunda
                </button>
              )}
              <a
                href={href}
                target="_blank"
                rel="noopener noreferrer"
                className="inline-flex min-h-[36px] items-center justify-center rounded-lg border border-gray-200 dark:border-dark-600 bg-white dark:bg-dark-800 px-3 py-1.5 text-xs font-medium text-gray-700 dark:text-gray-300 transition-colors hover:border-accent/60"
              >
                Tinjau Konten
              </a>
              <button
                type="button"
                onClick={() => onDeleteContent(r.id, r.resource_type, r.slug)}
                disabled={busy}
                className="inline-flex min-h-[36px] items-center justify-center rounded-lg border border-red-600 bg-red-600 px-3 py-1.5 text-xs font-medium text-white transition-colors hover:bg-red-700 disabled:opacity-60"
              >
                {busy ? '...' : r.resource_type === 'file' ? 'Hapus File' : 'Hapus Paste'}
              </button>
              <button
                type="button"
                onClick={() => onDelete(r.id)}
                disabled={busy}
                className="col-span-2 sm:col-span-1 sm:ml-auto inline-flex min-h-[36px] items-center justify-center rounded-lg border border-red-500/40 bg-red-500/10 px-3 py-1.5 text-xs font-medium text-red-600 dark:text-red-300 transition-colors hover:bg-red-500/20 disabled:opacity-60"
              >
                {busy ? '...' : 'Hapus Laporan'}
              </button>
            </div>
          </li>
        );
      })}
    </ul>
  );
}

// ---------------------------------------------------------------------------
// Root
// ---------------------------------------------------------------------------

export function AdminDashboard() {
  // Read the token from sessionStorage in a hydration-safe way: the server
  // snapshot is always null (matching SSR output), and React swaps to the real
  // client value after hydration — avoiding an SSR/client markup mismatch.
  const token = useSyncExternalStore(
    subscribeToken,
    readStoredToken,
    () => null,
  );

  const handleLogout = useCallback(() => {
    clearStoredToken();
  }, []);

  if (!token) {
    return <TokenGate />;
  }

  return <Dashboard token={token} onLogout={handleLogout} />;
}

export default AdminDashboard;
