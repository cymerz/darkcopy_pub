import { getUploadOptions } from '@/lib/api';
import { FileUploader } from '@/components/FileUploader';

// The upload options (expiry choices, visibilities) are fetched with
// `cache: 'no-store'` by the API client. Render on-demand to stay consistent
// with that fetch behavior and with `app/page.tsx` / `app/new/page.tsx`.
export const dynamic = 'force-dynamic';

/**
 * File upload page (Server Component).
 *
 * Fetches the available expiry options and visibility choices from the backend
 * via {@link getUploadOptions} (Req 5.1) and passes them to the
 * {@link FileUploader} client component, which handles the interactive upload.
 *
 * Field-name mapping: the backend's `GET /upload` response uses snake_case
 * (`UploadOptions.expiry_options`), while {@link FileUploader}'s props use
 * camelCase (`expiryOptions`). The values are remapped at the call site below.
 *
 * Error handling mirrors `app/page.tsx`: the fetch is wrapped in a try/catch
 * that logs the failure server-side and re-throws so the nearest
 * `app/error.tsx` boundary can render the user-facing retry UI.
 */
export default async function UploadPage() {
  let data;
  try {
    data = await getUploadOptions();
  } catch (error) {
    // Log for server-side observability, then propagate to the error boundary
    // (app/error.tsx) which provides the user-facing retry UI. Only the data
    // fetch is wrapped: JSX is constructed outside the try/catch so child
    // render errors reach the error boundary instead of being swallowed.
    console.error('Gagal memuat opsi unggah file:', error);
    throw error;
  }

  return (
    <section>
      <h1 className="text-2xl md:text-3xl font-bold text-gray-900 dark:text-gray-100 mb-6">Unggah File</h1>
      <FileUploader
        expiryOptions={data.expiry_options}
        visibilities={data.visibilities}
        maxFileSize={data.max_file_size}
        disabled={data.disable_file_uploads ?? false}
      />
    </section>
  );
}
