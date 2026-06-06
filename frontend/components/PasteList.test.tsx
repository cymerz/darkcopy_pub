/**
 * @jest-environment jsdom
 */
// components/PasteList.test.tsx
import '@testing-library/jest-dom';
import { render, screen } from '@testing-library/react';
import { PasteList } from './PasteList';
import type { PasteSummary } from '@/lib/types';

function makePaste(overrides: Partial<PasteSummary> = {}): PasteSummary {
  return {
    slug: 'abc123',
    title: 'My Paste',
    language: 'typescript',
    // Far in the past so relative time is deterministic ("hari yang lalu").
    created_at: '2020-01-01T00:00:00.000Z',
    expires_at: null,
    ...overrides,
  };
}

describe('PasteList', () => {
  it('shows the empty-state message when there are no pastes', () => {
    render(<PasteList pastes={[]} />);
    expect(screen.getByText('Belum ada paste publik')).toBeInTheDocument();
  });

  it('renders a card linking to /{slug}', () => {
    render(<PasteList pastes={[makePaste({ slug: 'xyz789' })]} />);
    const link = screen.getByRole('link', { name: /My Paste/ });
    expect(link).toHaveAttribute('href', '/xyz789');
  });

  it('falls back to "Untitled" when the title is empty or whitespace', () => {
    render(
      <PasteList
        pastes={[
          makePaste({ slug: 'empty', title: '' }),
          makePaste({ slug: 'spaces', title: '   ' }),
        ]}
      />
    );
    expect(screen.getAllByText('Untitled')).toHaveLength(2);
  });

  it('displays the language badge', () => {
    render(<PasteList pastes={[makePaste({ language: 'go' })]} />);
    expect(screen.getByText('go')).toBeInTheDocument();
  });

  it('displays a relative creation time', () => {
    render(<PasteList pastes={[makePaste()]} />);
    // Old dates collapse to "{n} hari yang lalu" (days ago).
    expect(screen.getByText(/yang lalu|\d/)).toBeInTheDocument();
  });

  it('renders one card per paste', () => {
    render(
      <PasteList
        pastes={[
          makePaste({ slug: 'a', title: 'A' }),
          makePaste({ slug: 'b', title: 'B' }),
          makePaste({ slug: 'c', title: 'C' }),
        ]}
      />
    );
    expect(screen.getAllByRole('link')).toHaveLength(3);
  });
});
