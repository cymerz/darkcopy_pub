import { getRecentPastes } from '@/lib/api';
import { PasteList } from '@/components/PasteList';
import { FileList } from '@/components/FileList';
import Link from 'next/link';

// Paste and file data is dynamic and changes frequently, so always render on-demand and
// fetch fresh data on every request (the API client also uses `cache: 'no-store'`).
export const dynamic = 'force-dynamic';

/**
 * Home page (Server Component).
 *
 * Fetches the list of recent public pastes and files from the backend via
 * {@link getRecentPastes} and renders them through {@link PasteList} and {@link FileList}.
 *
 * Error handling (Req 1.5): the fetch is wrapped in a try/catch that logs the
 * failure server-side and re-throws so the nearest `app/error.tsx` boundary can
 * render the "Gagal memuat" message with a retry button.
 */
export default async function HomePage() {
  let pastes = [];
  let files = [];
  try {
    const result = await getRecentPastes();
    // Go encodes a nil slice as JSON null; normalise to an empty array.
    pastes = result.pastes ?? [];
    files = result.files ?? [];
  } catch (error) {
    // Log for server-side observability, then propagate to the error boundary
    // (app/error.tsx) which provides the user-facing retry UI per Req 1.5.
    console.error('Gagal memuat daftar paste dan file:', error);
    throw error;
  }

  return (
    <div className="space-y-12 md:space-y-16">
      {/* Stunning Premium Hero Section */}
      <div className="text-center space-y-4 max-w-3xl mx-auto py-8 md:py-12 animate-fade-in">
        <h1 className="text-4xl md:text-6xl font-extrabold tracking-tight">
          <span className="bg-gradient-to-r from-accent via-indigo-400 to-purple-500 bg-clip-text text-transparent">
            DarkCopy
          </span>
        </h1>
        <p className="text-lg md:text-xl text-gray-600 dark:text-gray-400 font-medium">
          Bagi Teks & File Secara Anonim, Aman, dan Instan.
        </p>
        <p className="text-sm md:text-base text-gray-400 dark:text-gray-500 max-w-xl mx-auto leading-relaxed">
          Platform tanpa registrasi untuk membagikan paste terenkripsi dan file sementara dengan kadaluarsa otomatis & pengaman kata sandi.
        </p>
        <div className="flex flex-wrap items-center justify-center gap-4 pt-4">
          <Link
            href="/new"
            className="inline-flex min-h-[44px] items-center justify-center gap-2.5 rounded-lg bg-accent text-white px-6 py-2.5 font-semibold shadow-lg shadow-accent/25 hover:bg-accent-hover hover:scale-[1.02] active:scale-[0.98] transition-all"
          >
            <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth="2.5">
              <path strokeLinecap="round" strokeLinejoin="round" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
            </svg>
            Buat Paste Baru
          </Link>
          <Link
            href="/upload"
            className="inline-flex min-h-[44px] items-center justify-center gap-2.5 rounded-lg border border-gray-200 dark:border-dark-700 bg-white/50 dark:bg-dark-800/40 backdrop-blur-md text-gray-800 dark:text-gray-200 px-6 py-2.5 font-semibold hover:bg-gray-50 dark:hover:bg-dark-700/60 hover:scale-[1.02] active:scale-[0.98] transition-all"
          >
            <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth="2.5">
              <path strokeLinecap="round" strokeLinejoin="round" d="M3 16.5v2.25A2.25 2.25 0 005.25 21h13.5A2.25 2.25 0 0021 18.75V16.5m-13.5-9L12 3m0 0l4.5 4.5M12 3v13.5" />
            </svg>
            Unggah File
          </Link>
        </div>
      </div>
 
      <div className="grid grid-cols-1 xl:grid-cols-2 gap-8 md:gap-10">
        <section className="space-y-6">
          <div className="flex items-center justify-between border-b border-gray-100 dark:border-dark-800 pb-4">
            <h2 className="text-xl md:text-2xl font-bold text-gray-900 dark:text-gray-100 flex items-center gap-2">
              <svg className="h-5 w-5 text-accent" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth="2.5">
                <path strokeLinecap="round" strokeLinejoin="round" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
              </svg>
              Paste Publik Terbaru
            </h2>
          </div>
          <PasteList pastes={pastes} />
        </section>
 
        <section className="space-y-6">
          <div className="flex items-center justify-between border-b border-gray-100 dark:border-dark-800 pb-4">
            <h2 className="text-xl md:text-2xl font-bold text-gray-900 dark:text-gray-100 flex items-center gap-2">
              <svg className="h-5 w-5 text-accent" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth="2.5">
                <path strokeLinecap="round" strokeLinejoin="round" d="M2.25 12.75V12A2.25 2.25 0 014.5 9.75h15A2.25 2.25 0 0121.75 12v.75m-8.69-6.44l-2.12-2.12a1.5 1.5 0 00-1.061-.44H4.5A2.25 2.25 0 002.25 6v12a2.25 2.25 0 002.25 2.25h15A2.25 2.25 0 0021.75 18V9a2.25 2.25 0 00-2.25-2.25h-5.379a1.5 1.5 0 01-1.06-.44z" />
              </svg>
              File Publik Terbaru
            </h2>
          </div>
          <FileList files={files} />
        </section>
      </div>
    </div>
  );
}
