// components/CountdownTimer.tsx
//
// Client Component that displays the remaining time before a paste expires and
// keeps it up to date. The backend provides an initial `remaining_seconds`
// value; this component decrements it locally every 60 seconds so the viewer
// sees a live countdown without re-fetching (Requirement 3.6).

'use client';

import { useEffect, useState } from 'react';
import { formatRemainingTime } from '@/lib/utils';

interface CountdownTimerProps {
  remainingSeconds: number;
}

/**
 * Renders a live expiry countdown.
 *
 * Behaviour:
 * - Internal state is seeded from `remainingSeconds` and reset whenever the
 *   prop changes (e.g. when a different paste is rendered). The re-seed is done
 *   during render via the "adjust state when a prop changes" pattern
 *   (https://react.dev/learn/you-might-not-need-an-effect#adjusting-some-state-when-a-prop-changes)
 *   rather than synchronously inside an effect, which avoids cascading renders.
 * - A `setInterval` decrements the value by 60 every 60_000ms (one minute),
 *   matching the display granularity of `formatRemainingTime`.
 * - The interval is cleared on unmount and once the countdown reaches zero, at
 *   which point `formatRemainingTime` renders "Kadaluarsa".
 *
 * The text is exposed via an `aria-live="polite"` region so assistive
 * technology announces the expiry transition.
 */
export function CountdownTimer({ remainingSeconds }: CountdownTimerProps) {
  const [seconds, setSeconds] = useState(remainingSeconds);

  // Re-seed local state during render when the prop changes (e.g. a different
  // paste is rendered). Tracking the previous prop value and setting state
  // during render is React's recommended alternative to syncing in an effect.
  const [prevRemaining, setPrevRemaining] = useState(remainingSeconds);
  if (prevRemaining !== remainingSeconds) {
    setPrevRemaining(remainingSeconds);
    setSeconds(remainingSeconds);
  }

  useEffect(() => {
    // Nothing to count down for an already-expired paste.
    if (remainingSeconds <= 0) return;

    const intervalId = setInterval(() => {
      setSeconds((prev) => {
        const next = prev - 60;
        if (next <= 0) {
          clearInterval(intervalId);
          return 0;
        }
        return next;
      });
    }, 60000);

    return () => clearInterval(intervalId);
  }, [remainingSeconds]);

  return (
    <span
      className="inline-flex items-center gap-1.5 text-sm text-gray-500 dark:text-gray-400"
      aria-live="polite"
    >
      <svg
        xmlns="http://www.w3.org/2000/svg"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        strokeWidth={2}
        strokeLinecap="round"
        strokeLinejoin="round"
        className="h-4 w-4 shrink-0"
        aria-hidden="true"
      >
        <circle cx="12" cy="12" r="9" />
        <path d="M12 7v5l3 2" />
      </svg>
      {formatRemainingTime(seconds)}
    </span>
  );
}

export default CountdownTimer;
