// components/CountdownTimer.test.tsx
//
// Property test for the CountdownTimer's display/formatting logic.
//
// The CountdownTimer component (components/CountdownTimer.tsx) renders its value
// through `formatRemainingTime()` from `@/lib/utils` and decrements every 60s.
// The observable output of the timer after `t` elapsed seconds is therefore
// `formatRemainingTime(Math.max(0, remainingSeconds - t))`. We exercise that pure
// decrement-and-format logic across a wide input space with fast-check.
import fc from 'fast-check';
import { formatRemainingTime } from '@/lib/utils';

const SECONDS_PER_MINUTE = 60;
const SECONDS_PER_HOUR = 3600;
const SECONDS_PER_DAY = 86400;
const MAX_SECONDS = 400 * SECONDS_PER_DAY; // ~400 days, comfortably within int32

/**
 * Parses an Indonesian "X hari Y jam Z menit tersisa" countdown string back into
 * seconds, tolerating missing components (any of days/hours/minutes may be absent
 * when that component is zero). Returns 0 for the terminal "Kadaluarsa" string.
 */
function parseRemainingSeconds(output: string): number {
  if (output === 'Kadaluarsa') return 0;
  let total = 0;
  let m: RegExpMatchArray | null;
  if ((m = output.match(/(\d+) hari/))) total += parseInt(m[1], 10) * SECONDS_PER_DAY;
  if ((m = output.match(/(\d+) jam/))) total += parseInt(m[1], 10) * SECONDS_PER_HOUR;
  if ((m = output.match(/(\d+) menit/))) total += parseInt(m[1], 10) * SECONDS_PER_MINUTE;
  return total;
}

/**
 * Models the CountdownTimer's displayed value: the initial `remainingSeconds`
 * reduced by the elapsed wall-clock time `t` (clamped at zero), formatted for
 * display. This mirrors the component's decrement-and-format behavior collapsed
 * to its single observable output string.
 */
function displayedCountdown(remainingSeconds: number, elapsed: number): string {
  return formatRemainingTime(Math.max(0, remainingSeconds - elapsed));
}

describe('CountdownTimer - Property 6: Countdown Timer Accuracy', () => {
  // Validates: Requirements 3.6
  it('shows a value within 60 seconds of (remainingSeconds - t), and "Kadaluarsa" once depleted', () => {
    fc.assert(
      fc.property(
        // Initial remaining time strictly greater than zero.
        fc.integer({ min: 1, max: MAX_SECONDS }),
        // Elapsed wall-clock seconds; allow elapsed to exceed the initial value
        // so we also cover the expiry transition.
        fc.integer({ min: 0, max: MAX_SECONDS + SECONDS_PER_DAY }),
        (remainingSeconds, elapsed) => {
          const computedRemaining = Math.max(0, remainingSeconds - elapsed);
          const display = displayedCountdown(remainingSeconds, elapsed);

          if (computedRemaining <= 0) {
            // Reached zero or below => terminal "Kadaluarsa" state.
            expect(display).toBe('Kadaluarsa');
          } else {
            // Still counting down: a non-empty string ending with "tersisa".
            expect(display.length).toBeGreaterThan(0);
            expect(display.endsWith('tersisa')).toBe(true);

            // Floor-to-minute formatting => the parsed value is within 60
            // seconds of the true computed remaining time.
            const reconstructed = parseRemainingSeconds(display);
            expect(Math.abs(reconstructed - computedRemaining)).toBeLessThanOrEqual(60);
          }
        }
      )
    );
  });

  // Validates: Requirements 3.6
  it('returns "Kadaluarsa" for any non-positive seconds (expiry boundary)', () => {
    fc.assert(
      fc.property(fc.integer({ min: -MAX_SECONDS, max: 0 }), (seconds) => {
        expect(formatRemainingTime(seconds)).toBe('Kadaluarsa');
      })
    );
  });

  // Validates: Requirements 3.6
  it('is monotonic: as more time elapses the displayed remaining never increases', () => {
    fc.assert(
      fc.property(
        fc.integer({ min: 1, max: MAX_SECONDS }),
        fc.integer({ min: 0, max: MAX_SECONDS }),
        fc.integer({ min: 0, max: MAX_SECONDS }),
        (remainingSeconds, e1, e2) => {
          const earlier = Math.min(e1, e2);
          const later = Math.max(e1, e2);

          const earlierRemaining = parseRemainingSeconds(
            displayedCountdown(remainingSeconds, earlier)
          );
          const laterRemaining = parseRemainingSeconds(
            displayedCountdown(remainingSeconds, later)
          );

          expect(laterRemaining).toBeLessThanOrEqual(earlierRemaining);
        }
      )
    );
  });
});

// Complementary unit tests for concrete examples and the expiry transition.
describe('CountdownTimer display logic - unit tests', () => {
  it('formats a multi-component remaining time correctly', () => {
    const remaining = 2 * SECONDS_PER_DAY + 3 * SECONDS_PER_HOUR + 30 * SECONDS_PER_MINUTE;
    expect(displayedCountdown(remaining, 0)).toBe('2 hari 3 jam 30 menit tersisa');
  });

  it('subtracts elapsed time before formatting', () => {
    // Start at 1h 0m, after 30 minutes elapsed -> 30 minutes remaining.
    expect(displayedCountdown(SECONDS_PER_HOUR, 30 * SECONDS_PER_MINUTE)).toBe(
      '30 menit tersisa'
    );
  });

  it('shows "Kadaluarsa" exactly when elapsed reaches the initial remaining', () => {
    expect(displayedCountdown(SECONDS_PER_HOUR, SECONDS_PER_HOUR)).toBe('Kadaluarsa');
    expect(displayedCountdown(SECONDS_PER_HOUR, SECONDS_PER_HOUR + 1)).toBe('Kadaluarsa');
  });

  it('round-trips a formatted string back to within 60s of the source value', () => {
    const remaining = 5 * SECONDS_PER_HOUR + 45 * SECONDS_PER_MINUTE + 37; // includes stray seconds
    const reconstructed = parseRemainingSeconds(formatRemainingTime(remaining));
    expect(Math.abs(reconstructed - remaining)).toBeLessThanOrEqual(60);
  });
});
