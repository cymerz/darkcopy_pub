import { getNewPasteOptions } from '@/lib/api';
import { PasteForm } from '@/components/PasteForm';

// The option lists (languages, expiry choices) are relatively static, but the
// API client fetches them with `cache: 'no-store'`. Render on-demand to keep the
// page consistent with that fetch behavior and with `app/page.tsx`.
export const dynamic = 'force-dynamic';

/**
 * Create paste page (Server Component).
 *
 * Fetches the available programming languages and expiry options from the
 * backend via {@link getNewPasteOptions} (Req 2.1) and passes them to the
 * {@link PasteForm} client component, which handles the interactive form.
 *
 * Error handling mirrors `app/page.tsx`: the fetch is wrapped in a try/catch
 * that logs the failure server-side and re-throws so the nearest
 * `app/error.tsx` boundary can render the user-facing retry UI.
 */
export default async function NewPastePage() {
  let languages;
  let expiryOptions;
  let disableNewPastes = false;
  try {
    const data = await getNewPasteOptions();
    languages = data.languages;
    expiryOptions = data.expiryOptions;
    disableNewPastes = data.disable_new_pastes ?? false;
  } catch (error) {
    // Log for server-side observability, then propagate to the error boundary
    // (app/error.tsx) which provides the user-facing retry UI. Only the data
    // fetch is wrapped: JSX is constructed outside the try/catch so child
    // render errors reach the error boundary instead of being swallowed.
    console.error('Gagal memuat opsi pembuatan paste:', error);
    throw error;
  }

  return (
    <section>
      <h1 className="text-2xl md:text-3xl font-bold text-gray-900 dark:text-gray-100 mb-6">
        Buat Paste Baru
      </h1>
      <PasteForm languages={languages} expiryOptions={expiryOptions} disabled={disableNewPastes} />
    </section>
  );
}
