'use client';

import { Suspense, useEffect, useRef, useState } from 'react';
import { usePathname, useSearchParams } from 'next/navigation';

/**
 * Inner implementation of the navigation progress bar (Req 10.5).
 *
 * The App Router does not expose router transition events, so we detect
 * client-side navigation by watching `usePathname()` and `useSearchParams()`.
 * Whenever either changes we briefly show a thin animated bar fixed at the very
 * top of the page (above the sticky header), then fade it out.
 *
 * `useSearchParams()` requires a Suspense boundary, which the exported
 * `NavigationProgress` wrapper provides.
 */
function NavigationProgressInner() {
  const pathname = usePathname();
  const searchParams = useSearchParams();

  const [visible, setVisible] = useState(false);
  const [width, setWidth] = useState(0);

  // The bar reflects client-side navigation transitions only, so we skip the
  // very first render (initial page load / deep link), which is handled by the
  // route-level `loading.tsx` UI instead.
  const isFirstRender = useRef(true);
  const timers = useRef<ReturnType<typeof setTimeout>[]>([]);

  useEffect(() => {
    if (isFirstRender.current) {
      isFirstRender.current = false;
      return;
    }

    // Cancel any timers still pending from a previous transition.
    timers.current.forEach(clearTimeout);
    timers.current = [];

    // Show the bar and animate it quickly toward ~90% to signal progress.
    setVisible(true);
    setWidth(10);
    timers.current.push(setTimeout(() => setWidth(90), 50));

    // Complete the bar, then fade it out and reset its width.
    timers.current.push(setTimeout(() => setWidth(100), 350));
    timers.current.push(setTimeout(() => setVisible(false), 600));
    timers.current.push(setTimeout(() => setWidth(0), 800));

    return () => {
      timers.current.forEach(clearTimeout);
      timers.current = [];
    };
  }, [pathname, searchParams]);

  return (
    <div
      aria-hidden="true"
      className="fixed top-0 left-0 z-[60] h-0.5 w-full pointer-events-none"
    >
      <div
        className="h-full bg-accent shadow-sm shadow-accent/50 transition-[width,opacity] duration-300 ease-out"
        style={{ width: `${width}%`, opacity: visible ? 1 : 0 }}
      />
    </div>
  );
}

/**
 * Thin route-transition progress bar shown at the top of every page.
 * Wraps the inner component in a Suspense boundary because it relies on
 * `useSearchParams()`.
 */
export function NavigationProgress() {
  return (
    <Suspense fallback={null}>
      <NavigationProgressInner />
    </Suspense>
  );
}

export default NavigationProgress;
