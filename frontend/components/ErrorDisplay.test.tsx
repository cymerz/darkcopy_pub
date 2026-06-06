/**
 * @jest-environment jsdom
 */
// components/ErrorDisplay.test.tsx
import '@testing-library/jest-dom';
import { render, screen } from '@testing-library/react';
import { ErrorDisplay } from './ErrorDisplay';

describe('ErrorDisplay', () => {
  it('renders the required title as a heading', () => {
    render(<ErrorDisplay title="Tidak Ditemukan" />);
    expect(
      screen.getByRole('heading', { name: 'Tidak Ditemukan' }),
    ).toBeInTheDocument();
  });

  it('exposes the card as an alert region', () => {
    render(<ErrorDisplay title="Terjadi Kesalahan" />);
    expect(screen.getByRole('alert')).toBeInTheDocument();
  });

  it('renders the optional message when provided', () => {
    render(
      <ErrorDisplay
        title="Konten Telah Kadaluarsa"
        message="Konten ini telah dihapus otomatis."
      />,
    );
    expect(
      screen.getByText('Konten ini telah dihapus otomatis.'),
    ).toBeInTheDocument();
  });

  it('omits the message paragraph when no message is provided', () => {
    const { container } = render(<ErrorDisplay title="Tidak Ditemukan" />);
    expect(container.querySelector('p')).toBeNull();
  });

  it('renders the action as a link pointing at the given href', () => {
    render(
      <ErrorDisplay
        title="Tidak Ditemukan"
        action={{ label: 'Kembali ke Beranda', href: '/' }}
      />,
    );
    const link = screen.getByRole('link', { name: 'Kembali ke Beranda' });
    expect(link).toBeInTheDocument();
    expect(link).toHaveAttribute('href', '/');
  });

  it('does not render a link when no action is provided', () => {
    render(<ErrorDisplay title="Tidak Ditemukan" />);
    expect(screen.queryByRole('link')).not.toBeInTheDocument();
  });

  it('renders a provided custom icon instead of the default', () => {
    render(
      <ErrorDisplay
        title="Konten Telah Kadaluarsa"
        icon={<svg data-testid="custom-icon" />}
      />,
    );
    expect(screen.getByTestId('custom-icon')).toBeInTheDocument();
  });

  it('renders a default icon when none is provided', () => {
    const { container } = render(<ErrorDisplay title="Terjadi Kesalahan" />);
    // The default alert icon is the only <svg> rendered in this case.
    expect(container.querySelector('svg')).toBeInTheDocument();
  });
});
