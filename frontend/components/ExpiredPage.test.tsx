/**
 * @jest-environment jsdom
 */
// components/ExpiredPage.test.tsx
import '@testing-library/jest-dom';
import { render, screen } from '@testing-library/react';
import { ExpiredPage } from './ExpiredPage';

describe('ExpiredPage', () => {
  it('renders the default "Konten Telah Kadaluarsa" heading (Req 7.2)', () => {
    render(<ExpiredPage />);
    expect(
      screen.getByRole('heading', { name: 'Konten Telah Kadaluarsa' }),
    ).toBeInTheDocument();
  });

  it('explains that the content was deleted automatically (Req 7.2)', () => {
    render(<ExpiredPage />);
    expect(
      screen.getByText(/dihapus otomatis oleh sistem/i),
    ).toBeInTheDocument();
  });

  it('renders a "Kembali ke Beranda" action linking to the home page (Req 7.2)', () => {
    render(<ExpiredPage />);
    const link = screen.getByRole('link', { name: 'Kembali ke Beranda' });
    expect(link).toBeInTheDocument();
    expect(link).toHaveAttribute('href', '/');
  });

  it('exposes the card as an alert region', () => {
    render(<ExpiredPage />);
    expect(screen.getByRole('alert')).toBeInTheDocument();
  });

  it('allows overriding the title for resource-specific wording (Req 3.9)', () => {
    render(<ExpiredPage title="Paste Telah Kadaluarsa" />);
    expect(
      screen.getByRole('heading', { name: 'Paste Telah Kadaluarsa' }),
    ).toBeInTheDocument();
  });

  it('allows overriding the explanation message', () => {
    render(<ExpiredPage message="File ini sudah tidak tersedia." />);
    expect(
      screen.getByText('File ini sudah tidak tersedia.'),
    ).toBeInTheDocument();
  });

  it('renders an icon inside the card', () => {
    const { container } = render(<ExpiredPage />);
    expect(container.querySelector('svg')).toBeInTheDocument();
  });
});
