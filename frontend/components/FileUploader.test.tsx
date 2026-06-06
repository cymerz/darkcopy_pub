/**
 * @jest-environment jsdom
 */
// components/FileUploader.test.tsx
import '@testing-library/jest-dom';
import { render, screen, fireEvent, waitFor, act } from '@testing-library/react';
import { FileUploader, visibilityLabel } from './FileUploader';
import { formatFileSize } from '@/lib/utils';
import type { ExpiryOption } from '@/lib/types';

// --- Fixtures --------------------------------------------------------------

const EXPIRY_OPTIONS: ExpiryOption[] = [
  { label: '1 Jam', duration: 60 },
  { label: '24 Jam', duration: 1440 },
];

const VISIBILITIES = ['public', 'unlisted', 'password_protected'];

function renderUploader() {
  return render(
    <FileUploader expiryOptions={EXPIRY_OPTIONS} visibilities={VISIBILITIES} />,
  );
}

// --- Tests -----------------------------------------------------------------

describe('visibilityLabel', () => {
  it('maps known backend visibility values to Indonesian labels', () => {
    expect(visibilityLabel('public')).toBe('Publik');
    expect(visibilityLabel('unlisted')).toBe('Unlisted');
    expect(visibilityLabel('password_protected')).toBe('Dilindungi Kata Sandi');
  });

  it('falls back to the raw value for unknown visibilities', () => {
    expect(visibilityLabel('something_else')).toBe('something_else');
  });
});

describe('FileUploader', () => {
  it('renders the drop zone, "Pilih File" button, expiry, and visibility controls', () => {
    renderUploader();
    expect(screen.getByText(/Seret file ke sini/)).toBeInTheDocument();
    expect(
      screen.getByRole('button', { name: 'Pilih File' }),
    ).toBeInTheDocument();
    expect(screen.getByLabelText('Waktu Kadaluarsa')).toBeInTheDocument();
    expect(screen.getByLabelText('Publik')).toBeInTheDocument();
    expect(screen.getByLabelText('Unlisted')).toBeInTheDocument();
    expect(screen.getByLabelText('Dilindungi Kata Sandi')).toBeInTheDocument();
  });

  it('shows "Lepaskan file di sini" on dragover (Req 5.3)', () => {
    renderUploader();
    const dropZone = screen.getByLabelText(/Area unggah file/);
    fireEvent.dragOver(dropZone);
    expect(screen.getByText('Lepaskan file di sini')).toBeInTheDocument();

    fireEvent.dragLeave(dropZone);
    expect(screen.queryByText('Lepaskan file di sini')).not.toBeInTheDocument();
  });

  it('displays file name and formatted size after a drop (Req 5.4)', () => {
    renderUploader();
    const dropZone = screen.getByLabelText(/Area unggah file/);

    const bytes = 2048;
    const file = new File(['x'.repeat(bytes)], 'report.pdf', {
      type: 'application/pdf',
    });

    fireEvent.drop(dropZone, { dataTransfer: { files: [file] } });

    expect(screen.getByText('report.pdf')).toBeInTheDocument();
    // File size is rendered via formatFileSize (2048 bytes -> "2.0 KB").
    expect(
      screen.getByText(new RegExp(formatFileSize(bytes))),
    ).toBeInTheDocument();
  });

  it('reveals the password field only for password_protected (Req 5.5)', () => {
    renderUploader();
    expect(screen.queryByLabelText('Kata Sandi')).not.toBeInTheDocument();

    fireEvent.click(screen.getByLabelText('Dilindungi Kata Sandi'));
    expect(screen.getByLabelText('Kata Sandi')).toBeInTheDocument();
  });

  it('uploads via XMLHttpRequest and shows a copyable URL on 201 (Req 5.6, 5.8)', async () => {
    // Minimal XHR mock capturing the upload lifecycle.
    const sendMock = jest.fn();
    const openMock = jest.fn();
    const setRequestHeaderMock = jest.fn();

    class MockXHR {
      static instances: MockXHR[] = [];
      upload = { onprogress: null as ((e: ProgressEvent) => void) | null };
      onload: (() => void) | null = null;
      onerror: (() => void) | null = null;
      status = 0;
      responseText = '';
      open = openMock;
      setRequestHeader = setRequestHeaderMock;
      abort = jest.fn();
      send = (body: FormData) => {
        sendMock(body);
        MockXHR.instances.push(this);
      };
    }

    const original = global.XMLHttpRequest;
    // @ts-expect-error - assigning a minimal mock for the test
    global.XMLHttpRequest = MockXHR;

    try {
      renderUploader();
      const dropZone = screen.getByLabelText(/Area unggah file/);
      const file = new File(['hello'], 'note.txt', { type: 'text/plain' });
      fireEvent.drop(dropZone, { dataTransfer: { files: [file] } });

      fireEvent.click(screen.getByRole('button', { name: 'Unggah' }));

      expect(openMock).toHaveBeenCalledWith('POST', '/api/upload');
      expect(setRequestHeaderMock).toHaveBeenCalledWith(
        'Accept',
        'application/json',
      );

      // Verify the exact backend multipart field names.
      const sentForm = sendMock.mock.calls[0][0] as FormData;
      expect(sentForm.get('file')).toBeInstanceOf(File);
      expect(sentForm.get('visibility')).toBe('public');
      expect(sentForm.get('expires_in')).toBe('60');

      // Simulate a successful response.
      const xhr = MockXHR.instances[0];
      xhr.status = 201;
      xhr.responseText = JSON.stringify({
        success: true,
        slug: 'abc123',
        url: '/f/abc123',
      });
      act(() => xhr.onload?.());

      const urlInput = (await screen.findByLabelText(
        'URL File',
      )) as HTMLInputElement;
      expect(urlInput.value).toContain('/f/abc123');
    } finally {
      global.XMLHttpRequest = original;
    }
  });

  it('shows the size-limit message on HTTP 413 (Req 5.9)', async () => {
    class MockXHR {
      static instances: MockXHR[] = [];
      upload = { onprogress: null as ((e: ProgressEvent) => void) | null };
      onload: (() => void) | null = null;
      onerror: (() => void) | null = null;
      status = 0;
      responseText = '';
      open = jest.fn();
      setRequestHeader = jest.fn();
      abort = jest.fn();
      send = () => {
        MockXHR.instances.push(this);
      };
    }

    const original = global.XMLHttpRequest;
    // @ts-expect-error - assigning a minimal mock for the test
    global.XMLHttpRequest = MockXHR;

    try {
      renderUploader();
      const dropZone = screen.getByLabelText(/Area unggah file/);
      const file = new File(['big'], 'big.bin', {
        type: 'application/octet-stream',
      });
      fireEvent.drop(dropZone, { dataTransfer: { files: [file] } });
      fireEvent.click(screen.getByRole('button', { name: 'Unggah' }));

      const xhr = MockXHR.instances[0];
      xhr.status = 413;
      xhr.responseText = '';
      act(() => xhr.onload?.());

      await waitFor(() =>
        expect(screen.getByRole('alert')).toHaveTextContent(
          'Ukuran file melebihi batas maksimum 100 MB',
        ),
      );
    } finally {
      global.XMLHttpRequest = original;
    }
  });

  it('shows the backend error message on other errors (Req 5.10)', async () => {
    class MockXHR {
      static instances: MockXHR[] = [];
      upload = { onprogress: null as ((e: ProgressEvent) => void) | null };
      onload: (() => void) | null = null;
      onerror: (() => void) | null = null;
      status = 0;
      responseText = '';
      open = jest.fn();
      setRequestHeader = jest.fn();
      abort = jest.fn();
      send = () => {
        MockXHR.instances.push(this);
      };
    }

    const original = global.XMLHttpRequest;
    // @ts-expect-error - assigning a minimal mock for the test
    global.XMLHttpRequest = MockXHR;

    try {
      renderUploader();
      const dropZone = screen.getByLabelText(/Area unggah file/);
      const file = new File(['x'], 'x.txt', { type: 'text/plain' });
      fireEvent.drop(dropZone, { dataTransfer: { files: [file] } });
      fireEvent.click(screen.getByRole('button', { name: 'Unggah' }));

      const xhr = MockXHR.instances[0];
      xhr.status = 400;
      xhr.responseText = JSON.stringify({
        error: 'Format durasi kadaluarsa tidak valid',
        code: 'INVALID_EXPIRY',
        status: 400,
      });
      act(() => xhr.onload?.());

      await waitFor(() =>
        expect(screen.getByRole('alert')).toHaveTextContent(
          'Format durasi kadaluarsa tidak valid',
        ),
      );
    } finally {
      global.XMLHttpRequest = original;
    }
  });
});
