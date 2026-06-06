/**
 * @jest-environment jsdom
 */
// components/CopyButton.test.tsx
import '@testing-library/jest-dom';
import { render, screen, act } from '@testing-library/react';
import { CopyButton } from './CopyButton';

describe('CopyButton', () => {
  let writeText: jest.Mock;

  beforeEach(() => {
    jest.useFakeTimers();
    writeText = jest.fn().mockResolvedValue(undefined);
    Object.assign(navigator, {
      clipboard: { writeText },
    });
  });

  afterEach(() => {
    act(() => {
      jest.runOnlyPendingTimers();
    });
    jest.useRealTimers();
    jest.clearAllMocks();
  });

  it('renders the default "Salin" label', () => {
    render(<CopyButton content="hello" />);
    expect(screen.getByRole('button')).toHaveTextContent('Salin');
  });

  it('copies the raw content to the clipboard on click', async () => {
    render(<CopyButton content="the raw content" />);
    await act(async () => {
      screen.getByRole('button').click();
    });
    expect(writeText).toHaveBeenCalledWith('the raw content');
  });

  it('shows "Berhasil disalin" feedback after a successful copy', async () => {
    render(<CopyButton content="x" />);
    await act(async () => {
      screen.getByRole('button').click();
    });
    expect(screen.getByRole('button')).toHaveTextContent('Berhasil disalin');
  });

  it('reverts to the default label after 2 seconds', async () => {
    render(<CopyButton content="x" />);
    await act(async () => {
      screen.getByRole('button').click();
    });
    expect(screen.getByRole('button')).toHaveTextContent('Berhasil disalin');

    act(() => {
      jest.advanceTimersByTime(2000);
    });
    expect(screen.getByRole('button')).toHaveTextContent('Salin');
    expect(screen.getByRole('button')).not.toHaveTextContent('Berhasil disalin');
  });

  it('shows an error state when the clipboard write fails', async () => {
    writeText.mockRejectedValueOnce(new Error('denied'));
    render(<CopyButton content="x" />);
    await act(async () => {
      screen.getByRole('button').click();
    });
    expect(screen.getByRole('button')).toHaveTextContent('Gagal menyalin');
  });
});
