/**
 * @jest-environment jsdom
 */
// components/NavigationProgress.test.tsx
//
// Unit tests for the route-transition progress bar (Req 10.5). The component
// detects client-side navigation by watching `usePathname()` /
// `useSearchParams()` from next/navigation, so we mock those hooks and drive
// "navigation" by re-rendering with different return values.
import '@testing-library/jest-dom';
import { act, render } from '@testing-library/react';
import { NavigationProgress } from './NavigationProgress';

// Mutable values returned by the mocked next/navigation hooks.
let mockPathname = '/';
let mockSearch = new URLSearchParams('');

jest.mock('next/navigation', () => ({
  usePathname: () => mockPathname,
  useSearchParams: () => mockSearch,
}));

// The progress bar is the inner div whose width/opacity is driven by inline
// style. The outer wrapper is a positioned, full-width container.
function getWrapper(container: HTMLElement): HTMLElement {
  const wrapper = container.querySelector('.fixed') as HTMLElement | null;
  if (!wrapper) throw new Error('progress bar wrapper not found');
  return wrapper;
}

function getBar(container: HTMLElement): HTMLElement {
  const bar = getWrapper(container).firstElementChild as HTMLElement | null;
  if (!bar) throw new Error('progress bar element not found');
  return bar;
}

describe('NavigationProgress', () => {
  beforeEach(() => {
    jest.useFakeTimers();
    mockPathname = '/';
    mockSearch = new URLSearchParams('');
  });

  afterEach(() => {
    act(() => {
      jest.runOnlyPendingTimers();
    });
    jest.useRealTimers();
    jest.clearAllMocks();
  });

  it('renders a fixed, high z-index, full-width thin bar container', () => {
    const { container } = render(<NavigationProgress />);
    const wrapper = getWrapper(container);
    expect(wrapper).toHaveClass('fixed', 'top-0', 'left-0', 'w-full');
    // Thin height (h-0.5) so it reads as a slim progress strip.
    expect(wrapper.className).toContain('h-0.5');
  });

  it('stays hidden (0% width, hidden) on the initial render / deep link', () => {
    const { container } = render(<NavigationProgress />);
    const bar = getBar(container);
    expect(bar.style.width).toBe('0%');
    expect(bar.style.opacity).toBe('0');
  });

  it('shows and animates the bar when the pathname changes', () => {
    const { container, rerender } = render(<NavigationProgress />);

    // Simulate a client-side navigation to a new path.
    act(() => {
      mockPathname = '/new';
      rerender(<NavigationProgress />);
    });

    let bar = getBar(container);
    // Becomes visible and starts advancing immediately.
    expect(bar.style.opacity).toBe('1');
    expect(parseFloat(bar.style.width)).toBeGreaterThan(0);

    // Advances toward ~90% shortly after.
    act(() => {
      jest.advanceTimersByTime(60);
    });
    bar = getBar(container);
    expect(parseFloat(bar.style.width)).toBeGreaterThanOrEqual(90);
  });

  it('completes to 100% then fades out and resets after the transition', () => {
    const { container, rerender } = render(<NavigationProgress />);

    act(() => {
      mockPathname = '/upload';
      rerender(<NavigationProgress />);
    });

    // After the completion timer the bar reaches 100%.
    act(() => {
      jest.advanceTimersByTime(350);
    });
    expect(parseFloat(getBar(container).style.width)).toBe(100);

    // Then it fades out and resets back to 0% width.
    act(() => {
      jest.advanceTimersByTime(450);
    });
    const bar = getBar(container);
    expect(bar.style.opacity).toBe('0');
    expect(bar.style.width).toBe('0%');
  });

  it('re-triggers when the search params change on the same path', () => {
    const { container, rerender } = render(<NavigationProgress />);

    act(() => {
      mockSearch = new URLSearchParams('lang=go');
      rerender(<NavigationProgress />);
    });

    expect(getBar(container).style.opacity).toBe('1');
  });
});
