/**
 * @jest-environment jsdom
 */
// components/PasteForm.test.tsx
import '@testing-library/jest-dom';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { PasteForm } from './PasteForm';
import type { ExpiryOption, Language } from '@/lib/types';
import { APIError } from '@/lib/types';

// --- Mocks -----------------------------------------------------------------

const pushMock = jest.fn();
jest.mock('next/navigation', () => ({
  useRouter: () => ({ push: pushMock }),
}));

const createPasteMock = jest.fn();
jest.mock('@/lib/api', () => ({
  createPaste: (data: FormData) => createPasteMock(data),
}));

// --- Fixtures --------------------------------------------------------------

const LANGUAGES: Language[] = [
  { id: 'plaintext', name: 'Plain Text' },
  { id: 'go', name: 'Go' },
];

const EXPIRY_OPTIONS: ExpiryOption[] = [
  { label: '1 Jam', duration: 60 },
  { label: '24 Jam', duration: 1440 },
];

function renderForm() {
  return render(
    <PasteForm languages={LANGUAGES} expiryOptions={EXPIRY_OPTIONS} />,
  );
}

beforeEach(() => {
  pushMock.mockReset();
  createPasteMock.mockReset();
});

// --- Tests -----------------------------------------------------------------

describe('PasteForm', () => {
  it('renders content, title, language, expiry, and visibility controls', () => {
    renderForm();
    expect(screen.getByLabelText(/Konten/)).toBeRequired();
    expect(screen.getByLabelText(/Judul/)).toBeInTheDocument();
    expect(screen.getByLabelText('Bahasa')).toBeInTheDocument();
    expect(screen.getByLabelText('Waktu Kadaluarsa')).toBeInTheDocument();
    expect(screen.getByLabelText('Publik')).toBeInTheDocument();
    expect(screen.getByLabelText('Unlisted')).toBeInTheDocument();
    expect(screen.getByLabelText('Dilindungi Kata Sandi')).toBeInTheDocument();
  });

  it('hides the password field until password_protected is selected (Req 2.3)', () => {
    renderForm();
    expect(screen.queryByLabelText('Kata Sandi')).not.toBeInTheDocument();

    fireEvent.click(screen.getByLabelText('Dilindungi Kata Sandi'));
    expect(screen.getByLabelText('Kata Sandi')).toBeInTheDocument();
  });

  it('renders a line number for each content line (Req 2.7)', () => {
    renderForm();
    const textarea = screen.getByLabelText(/Konten/);
    fireEvent.change(textarea, { target: { value: 'line1\nline2\nline3' } });
    expect(screen.getByText('1')).toBeInTheDocument();
    expect(screen.getByText('2')).toBeInTheDocument();
    expect(screen.getByText('3')).toBeInTheDocument();
  });

  it('submits the expected backend field names and redirects on success (Req 2.4)', async () => {
    createPasteMock.mockResolvedValue({ slug: 'abc123', url: '/abc123' });
    renderForm();

    fireEvent.change(screen.getByLabelText(/Konten/), {
      target: { value: 'hello world' },
    });
    fireEvent.change(screen.getByLabelText(/Judul/), {
      target: { value: 'My Title' },
    });
    fireEvent.change(screen.getByLabelText('Bahasa'), {
      target: { value: 'go' },
    });
    fireEvent.change(screen.getByLabelText('Waktu Kadaluarsa'), {
      target: { value: '1440' },
    });

    fireEvent.submit(screen.getByRole('button', { name: /Buat Paste/ }));

    await waitFor(() => expect(pushMock).toHaveBeenCalledWith('/abc123'));

    const formData = createPasteMock.mock.calls[0][0] as FormData;
    expect(formData.get('content')).toBe('hello world');
    expect(formData.get('title')).toBe('My Title');
    expect(formData.get('language')).toBe('go');
    expect(formData.get('expires_in')).toBe('1440');
    expect(formData.get('visibility')).toBe('public');
    // Password omitted when not password_protected.
    expect(formData.get('password')).toBeNull();
  });

  it('includes the password field when password_protected is selected', async () => {
    createPasteMock.mockResolvedValue({ slug: 'pw1', url: '/pw1' });
    renderForm();

    fireEvent.change(screen.getByLabelText(/Konten/), {
      target: { value: 'secret' },
    });
    fireEvent.click(screen.getByLabelText('Dilindungi Kata Sandi'));
    fireEvent.change(screen.getByLabelText('Kata Sandi'), {
      target: { value: 's3cr3t' },
    });

    fireEvent.submit(screen.getByRole('button', { name: /Buat Paste/ }));

    await waitFor(() => expect(createPasteMock).toHaveBeenCalled());
    const formData = createPasteMock.mock.calls[0][0] as FormData;
    expect(formData.get('visibility')).toBe('password_protected');
    expect(formData.get('password')).toBe('s3cr3t');
  });

  it('shows the backend error message and preserves input on failure (Req 2.5)', async () => {
    createPasteMock.mockRejectedValue(
      new APIError('Konten tidak boleh kosong', 'VALIDATION_ERROR', 400),
    );
    renderForm();

    const textarea = screen.getByLabelText(/Konten/) as HTMLTextAreaElement;
    fireEvent.change(textarea, { target: { value: 'keep me' } });

    fireEvent.submit(screen.getByRole('button', { name: /Buat Paste/ }));

    expect(await screen.findByRole('alert')).toHaveTextContent(
      'Konten tidak boleh kosong',
    );
    // Input preserved, no navigation.
    expect(textarea.value).toBe('keep me');
    expect(pushMock).not.toHaveBeenCalled();
  });
});
