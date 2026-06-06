// lib/utils.test.ts
import fc from 'fast-check';
import {
  formatRelativeTime,
  formatRemainingTime,
  formatFileSize,
  getFileExtension,
} from './utils';

const SECONDS_PER_MINUTE = 60;
const SECONDS_PER_HOUR = 3600;
const SECONDS_PER_DAY = 86400;
const MAX_SECONDS = 400 * SECONDS_PER_DAY; // ~400 days, comfortably within int32

/**
 * Maps a `formatRelativeTime` output back to a monotonic "age" rank in seconds.
 * More recent dates must yield a smaller-or-equal rank. Each bucket maps to the
 * lower bound (representative value) of that bucket so the rank is non-decreasing
 * as the real age grows across bucket boundaries.
 */
function relativeAgeRank(output: string): number {
  if (output === 'baru saja') return 0;
  let m: RegExpMatchArray | null;
  if ((m = output.match(/^(\d+) menit yang lalu$/))) {
    return parseInt(m[1], 10) * SECONDS_PER_MINUTE;
  }
  if ((m = output.match(/^(\d+) jam yang lalu$/))) {
    return parseInt(m[1], 10) * SECONDS_PER_HOUR;
  }
  if ((m = output.match(/^(\d+) hari yang lalu$/))) {
    return parseInt(m[1], 10) * SECONDS_PER_DAY;
  }
  // Fallback bucket: a locale date string for ages >= 30 days. All such ages
  // share the lower-bound rank of 30 days.
  return 30 * SECONDS_PER_DAY;
}

/** Reconstructs the total seconds encoded by a `formatRemainingTime` output. */
function parseRemainingSeconds(output: string): number {
  let total = 0;
  let m: RegExpMatchArray | null;
  if ((m = output.match(/(\d+) hari/))) total += parseInt(m[1], 10) * SECONDS_PER_DAY;
  if ((m = output.match(/(\d+) jam/))) total += parseInt(m[1], 10) * SECONDS_PER_HOUR;
  if ((m = output.match(/(\d+) menit/))) total += parseInt(m[1], 10) * SECONDS_PER_MINUTE;
  return total;
}

describe('formatRelativeTime - Property 1: Relative Time Formatting Monotonicity', () => {
  // Freeze "now" so formatRelativeTime is deterministic across the property run.
  const FIXED_NOW = new Date('2025-06-15T12:00:00.000Z').getTime();

  beforeAll(() => {
    jest.useFakeTimers();
    jest.setSystemTime(FIXED_NOW);
  });

  afterAll(() => {
    jest.useRealTimers();
  });

  // Validates: Requirements 1.2
  it('a more recent date never reports a longer duration, and output is always non-empty', () => {
    fc.assert(
      fc.property(
        fc.integer({ min: 0, max: MAX_SECONDS }),
        fc.integer({ min: 0, max: MAX_SECONDS }),
        (s1, s2) => {
          const olderOffset = Math.max(s1, s2); // larger age = date B (older)
          const recentOffset = Math.min(s1, s2); // smaller age = date A (more recent)

          const dateA = new Date(FIXED_NOW - recentOffset * 1000).toISOString();
          const dateB = new Date(FIXED_NOW - olderOffset * 1000).toISOString();

          const outA = formatRelativeTime(dateA);
          const outB = formatRelativeTime(dateB);

          // Always a non-empty (Indonesian) string.
          expect(outA.length).toBeGreaterThan(0);
          expect(outB.length).toBeGreaterThan(0);

          // A is more recent than (or equal to) B => shorter-or-equal duration.
          expect(relativeAgeRank(outA)).toBeLessThanOrEqual(relativeAgeRank(outB));
        }
      )
    );
  });
});

describe('formatRemainingTime - Property 2: Remaining Time Formatting Completeness', () => {
  // Validates: Requirements 3.2, 3.6
  it('positive seconds produce a non-empty string ending with "tersisa" reconstructible within 60s', () => {
    fc.assert(
      fc.property(fc.integer({ min: 1, max: MAX_SECONDS }), (seconds) => {
        const out = formatRemainingTime(seconds);

        expect(out.length).toBeGreaterThan(0);
        expect(out.endsWith('tersisa')).toBe(true);

        const reconstructed = parseRemainingSeconds(out);
        expect(Math.abs(reconstructed - seconds)).toBeLessThanOrEqual(60);
      })
    );
  });

  // Validates: Requirements 3.2, 3.6
  it('non-positive seconds always return "Kadaluarsa"', () => {
    fc.assert(
      fc.property(fc.integer({ min: -MAX_SECONDS, max: 0 }), (seconds) => {
        expect(formatRemainingTime(seconds)).toBe('Kadaluarsa');
      })
    );
  });
});

describe('formatFileSize - Property 3: File Size Formatting Consistency', () => {
  const ONE_KB = 1024;
  const ONE_MB = 1024 * 1024;
  const FIVE_GB = 5 * 1024 * 1024 * 1024;

  // Smart generator: cover each unit regime (B / KB / MB) so the property
  // exercises every branch and the boundaries between them.
  const bytesArb = fc.oneof(
    fc.integer({ min: 0, max: ONE_KB - 1 }), // B regime
    fc.integer({ min: ONE_KB, max: ONE_MB - 1 }), // KB regime
    fc.integer({ min: ONE_MB, max: FIVE_GB }) // MB regime
  );

  // Validates: Requirements 5.4
  it('produces a numeric value with exactly one unit; B and KB numeric values are < 1024', () => {
    fc.assert(
      fc.property(bytesArb, (bytes) => {
        const out = formatFileSize(bytes);

        const match = out.match(/^(\d+(?:\.\d+)?) (B|KB|MB)$/);
        // Exactly one numeric value followed by exactly one unit.
        expect(match).not.toBeNull();

        const value = parseFloat(match![1]);
        const unit = match![2];

        if (unit === 'B' || unit === 'KB') {
          expect(value).toBeLessThan(1024);
        }
      })
    );
  });
});

// Complementary unit tests for concrete examples and boundaries.
describe('utility functions - unit tests', () => {
  it('formatRemainingTime composes non-zero day/hour/minute components', () => {
    expect(formatRemainingTime(SECONDS_PER_DAY + SECONDS_PER_HOUR + SECONDS_PER_MINUTE)).toBe(
      '1 hari 1 jam 1 menit tersisa'
    );
    expect(formatRemainingTime(0)).toBe('Kadaluarsa');
    expect(formatRemainingTime(-10)).toBe('Kadaluarsa');
  });

  it('formatFileSize uses the expected unit thresholds', () => {
    expect(formatFileSize(0)).toBe('0 B');
    expect(formatFileSize(1023)).toBe('1023 B');
    expect(formatFileSize(1024)).toBe('1.0 KB');
    expect(formatFileSize(1024 * 1024)).toBe('1.0 MB');
  });

  it('getFileExtension maps known languages and falls back to .txt', () => {
    expect(getFileExtension('typescript')).toBe('.ts');
    expect(getFileExtension('go')).toBe('.go');
    expect(getFileExtension('unknown-language')).toBe('.txt');
  });
});
