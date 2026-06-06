'use client';

// components/AdminSettingsForm.tsx
//
// Settings editor used by the hidden admin dashboard. Lets an admin change the
// max paste/file size, the expiry options offered on the create/upload forms,
// and per-IP daily limits for pastes and uploads. All values are validated
// again server-side; this form does light client-side validation for UX.

import { useEffect, useState } from 'react';
import { getAdminSettings, updateAdminSettings } from '@/lib/api';
import { APIError } from '@/lib/types';
import type { AdminSettings, AdminExpiryOption } from '@/lib/types';

const MB = 1024 * 1024;

const INPUT_CLASS =
  'w-full min-h-[44px] rounded-lg border border-gray-200 dark:border-dark-700 bg-white dark:bg-dark-900 px-3 py-2.5 text-gray-900 dark:text-gray-100 placeholder-gray-400 dark:placeholder-gray-500 transition-colors focus:border-accent focus:outline-none focus:ring-2 focus:ring-accent/40 disabled:cursor-not-allowed disabled:opacity-60';

function bytesToMB(bytes: number): string {
  return (bytes / MB).toFixed(bytes % MB === 0 ? 0 : 2);
}

interface ExpiryListEditorProps {
  title: string;
  hint: string;
  options: AdminExpiryOption[];
  onChange: (next: AdminExpiryOption[]) => void;
  disabled: boolean;
}

function ExpiryListEditor({
  title,
  hint,
  options,
  onChange,
  disabled,
}: ExpiryListEditorProps) {
  const update = (i: number, patch: Partial<AdminExpiryOption>) => {
    onChange(options.map((o, idx) => (idx === i ? { ...o, ...patch } : o)));
  };
  const remove = (i: number) => onChange(options.filter((_, idx) => idx !== i));
  const add = () => onChange([...options, { label: '', minutes: 60 }]);

  return (
    <div
      role="group"
      aria-label={title}
      className="space-y-3 rounded-xl border border-gray-200 dark:border-dark-700 bg-white dark:bg-dark-800 p-4"
    >
      <div>
        <h3 className="text-sm font-semibold text-gray-800 dark:text-gray-200">{title}</h3>
        <p className="mt-1 text-xs text-gray-500 dark:text-gray-500">{hint}</p>
      </div>
      <ul className="space-y-2">
        {options.map((o, i) => (
          <li key={i} className="flex flex-wrap items-center gap-2">
            <input
              type="text"
              value={o.label}
              onChange={(e) => update(i, { label: e.target.value })}
              placeholder="Label (mis. 1 Jam)"
              disabled={disabled}
              className={`${INPUT_CLASS} max-w-[12rem] flex-1`}
            />
            <input
              type="number"
              min={0}
              value={o.minutes}
              onChange={(e) => update(i, { minutes: Number(e.target.value) })}
              placeholder="Menit"
              disabled={disabled}
              className={`${INPUT_CLASS} max-w-[8rem]`}
            />
            <span className="text-xs text-gray-500 dark:text-gray-500">menit (0 = selamanya)</span>
            <button
              type="button"
              onClick={() => remove(i)}
              disabled={disabled}
              className="ml-auto inline-flex min-h-[40px] items-center rounded-lg border border-red-500/40 bg-red-500/10 px-3 py-1.5 text-sm text-red-600 dark:text-red-300 transition-colors hover:bg-red-500/20 disabled:opacity-60"
            >
              Hapus
            </button>
          </li>
        ))}
      </ul>
      <button
        type="button"
        onClick={add}
        disabled={disabled}
        className="inline-flex min-h-[40px] items-center rounded-lg border border-gray-200 dark:border-dark-700 bg-white dark:bg-dark-900 px-4 py-2 text-sm font-medium text-gray-800 dark:text-gray-200 transition-colors hover:border-accent/60 hover:text-gray-900 dark:hover:text-white disabled:opacity-60"
      >
        + Tambah Pilihan
      </button>
    </div>
  );
}

export function AdminSettingsForm({
  token,
  onUnauthorized,
}: {
  token: string;
  onUnauthorized: () => void;
}) {
  const [settings, setSettings] = useState<AdminSettings | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  // Load current settings on mount. State is only set after the awaited fetch.
  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const s = await getAdminSettings(token);
        if (!cancelled) setSettings(s);
      } catch (err) {
        if (cancelled) return;
        if (err instanceof APIError && (err.status === 401 || err.status === 404)) {
          onUnauthorized();
          return;
        }
        setError('Gagal memuat pengaturan.');
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [token, onUnauthorized]);

  const patch = (p: Partial<AdminSettings>) =>
    setSettings((prev) => (prev ? { ...prev, ...p } : prev));

  const handleSave = async (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    if (!settings || saving) return;
    setSaving(true);
    setError(null);
    setNotice(null);
    try {
      const saved = await updateAdminSettings(token, settings);
      setSettings(saved);
      setNotice('Pengaturan berhasil disimpan.');
    } catch (err) {
      if (err instanceof APIError) {
        if (err.status === 401 || err.status === 404) {
          onUnauthorized();
          return;
        }
        setError(err.message || 'Gagal menyimpan pengaturan.');
      } else {
        setError('Gagal menyimpan pengaturan.');
      }
    } finally {
      setSaving(false);
    }
  };

  if (loading) {
    return <p className="py-8 text-center text-gray-500 dark:text-gray-400">Memuat pengaturan...</p>;
  }

  if (!settings) {
    return (
      <div
        role="alert"
        className="rounded-lg border border-red-500/40 bg-red-500/10 px-4 py-3 text-sm text-red-600 dark:text-red-300"
      >
        {error ?? 'Pengaturan tidak tersedia.'}
      </div>
    );
  }

  return (
    <form onSubmit={handleSave} className="space-y-5">
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

      {/* Size limits */}
      <div className="grid gap-4 sm:grid-cols-2">
        <label className="space-y-2">
          <span className="block text-sm font-medium text-gray-800 dark:text-gray-200">
            Ukuran Paste Maksimum (MB)
          </span>
          <input
            type="number"
            min={0}
            step="0.5"
            value={bytesToMB(settings.max_paste_size_bytes)}
            onChange={(e) =>
              patch({ max_paste_size_bytes: Math.round(Number(e.target.value) * MB) })
            }
            disabled={saving}
            className={INPUT_CLASS}
          />
        </label>
        <label className="space-y-2">
          <span className="block text-sm font-medium text-gray-800 dark:text-gray-200">
            Ukuran File Maksimum (MB)
          </span>
          <input
            type="number"
            min={0}
            step="1"
            value={bytesToMB(settings.max_file_size_bytes)}
            onChange={(e) =>
              patch({ max_file_size_bytes: Math.round(Number(e.target.value) * MB) })
            }
            disabled={saving}
            className={INPUT_CLASS}
          />
        </label>
      </div>

      {/* Daily limits */}
      <div className="grid gap-4 sm:grid-cols-2">
        <label className="space-y-2">
          <span className="block text-sm font-medium text-gray-800 dark:text-gray-200">
            Maks. Paste / IP / Hari
          </span>
          <input
            type="number"
            min={0}
            value={settings.max_pastes_per_day_per_ip}
            onChange={(e) =>
              patch({ max_pastes_per_day_per_ip: Number(e.target.value) })
            }
            disabled={saving}
            className={INPUT_CLASS}
          />
          <span className="block text-xs text-gray-500 dark:text-gray-500">0 = tanpa batas</span>
        </label>
        <label className="space-y-2">
          <span className="block text-sm font-medium text-gray-800 dark:text-gray-200">
            Maks. Unggah File / IP / Hari
          </span>
          <input
            type="number"
            min={0}
            value={settings.max_file_uploads_per_day_per_ip}
            onChange={(e) =>
              patch({ max_file_uploads_per_day_per_ip: Number(e.target.value) })
            }
            disabled={saving}
            className={INPUT_CLASS}
          />
          <span className="block text-xs text-gray-500 dark:text-gray-500">0 = tanpa batas</span>
        </label>
      </div>

      {/* Daily size limits */}
      <div className="grid gap-4 sm:grid-cols-2">
        <label className="space-y-2">
          <span className="block text-sm font-medium text-gray-800 dark:text-gray-200">
            Batas Total Ukuran Upload Harian Global (MB)
          </span>
          <input
            type="number"
            min={0}
            value={settings.max_daily_upload_bytes ? bytesToMB(settings.max_daily_upload_bytes) : 0}
            onChange={(e) =>
              patch({ max_daily_upload_bytes: Math.round(Number(e.target.value) * MB) })
            }
            disabled={saving}
            className={INPUT_CLASS}
          />
          <span className="block text-xs text-gray-500 dark:text-gray-500">0 = tanpa batas harian global</span>
        </label>
        <label className="space-y-2">
          <span className="block text-sm font-medium text-gray-800 dark:text-gray-200">
            Batas Total Ukuran Upload Harian Per IP (MB)
          </span>
          <input
            type="number"
            min={0}
            value={settings.max_daily_upload_bytes_per_ip ? bytesToMB(settings.max_daily_upload_bytes_per_ip) : 0}
            onChange={(e) =>
              patch({ max_daily_upload_bytes_per_ip: Math.round(Number(e.target.value) * MB) })
            }
            disabled={saving}
            className={INPUT_CLASS}
          />
          <span className="block text-xs text-gray-500 dark:text-gray-500">0 = tanpa batas harian per IP</span>
        </label>
      </div>

      {/* Temporary Toggles */}
      <div className="grid gap-4 sm:grid-cols-2 rounded-xl border border-gray-200 dark:border-dark-700 bg-white dark:bg-dark-800 p-4">
        <label className="flex items-center space-x-3 cursor-pointer">
          <input
            type="checkbox"
            checked={settings.disable_new_pastes ?? false}
            onChange={(e) => patch({ disable_new_pastes: e.target.checked })}
            disabled={saving}
            className="h-4 w-4 rounded border-gray-300 text-accent focus:ring-accent bg-transparent"
          />
          <div>
            <span className="block text-sm font-medium text-gray-800 dark:text-gray-200">
              Nonaktifkan Sementara New Paste
            </span>
            <span className="block text-xs text-gray-500 dark:text-gray-500">
              Mencegah pengguna membuat paste baru.
            </span>
          </div>
        </label>
        <label className="flex items-center space-x-3 cursor-pointer">
          <input
            type="checkbox"
            checked={settings.disable_file_uploads ?? false}
            onChange={(e) => patch({ disable_file_uploads: e.target.checked })}
            disabled={saving}
            className="h-4 w-4 rounded border-gray-300 text-accent focus:ring-accent bg-transparent"
          />
          <div>
            <span className="block text-sm font-medium text-gray-800 dark:text-gray-200">
              Nonaktifkan Sementara Unggah File
            </span>
            <span className="block text-xs text-gray-500 dark:text-gray-500">
              Mencegah pengguna mengunggah berkas baru.
            </span>
          </div>
        </label>
      </div>

      <ExpiryListEditor
        title="Pilihan Kadaluarsa Paste"
        hint="Ditampilkan di form pembuatan paste."
        options={settings.paste_expiry_options}
        onChange={(next) => patch({ paste_expiry_options: next })}
        disabled={saving}
      />

      <ExpiryListEditor
        title="Pilihan Kadaluarsa File"
        hint="Ditampilkan di form unggah file."
        options={settings.file_expiry_options}
        onChange={(next) => patch({ file_expiry_options: next })}
        disabled={saving}
      />

      <button
        type="submit"
        disabled={saving}
        className="inline-flex min-h-[44px] items-center justify-center rounded-lg bg-accent px-6 py-2.5 font-medium text-white shadow-sm shadow-accent/30 transition-colors hover:bg-accent-hover focus:outline-none focus:ring-2 focus:ring-accent/50 disabled:cursor-not-allowed disabled:opacity-60"
      >
        {saving ? 'Menyimpan...' : 'Simpan Pengaturan'}
      </button>
    </form>
  );
}

export default AdminSettingsForm;
