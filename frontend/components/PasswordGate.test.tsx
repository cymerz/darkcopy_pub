/**
 * @jest-environment jsdom
 */
// components/PasswordGate.test.tsx
//
// Unit tests for the PasswordGate component's pure state-machine helpers
// (`nextState`, `isFormEnabled`, `eventForStatus`,
// `filenameFromContentDisposition`). These exercise concrete transitions and
// edge cases; the universal Property 5 invariants are covered separately by the
// property test in task 9.3.
import {
  nextState,
  isFormEnabled,
  eventForStatus,
  filenameFromContentDisposition,
  initialGateState,
  MSG_WRONG_PASSWORD,
  MSG_RATE_LIMITED,
  MSG_NOT_FOUND,
  MSG_GONE,
  type GateState,
} from './PasswordGate';

const loadingState: GateState = {
  status: 'loading',
  errorMessage: null,
  contentUnlocked: false,
};

describe('PasswordGate state machine - nextState', () => {
  it('starts idle with content locked and no error', () => {
    expect(initialGateState).toEqual({
      status: 'idle',
      errorMessage: null,
      contentUnlocked: false,
    });
  });

  it('SUBMIT from idle transitions to loading', () => {
    expect(nextState(initialGateState, { type: 'SUBMIT' })).toEqual({
      status: 'loading',
      errorMessage: null,
      contentUnlocked: false,
    });
  });

  it('SUBMIT from error (retry) transitions to loading', () => {
    const errorState: GateState = {
      status: 'error',
      errorMessage: MSG_WRONG_PASSWORD,
      contentUnlocked: false,
    };
    expect(nextState(errorState, { type: 'SUBMIT' }).status).toBe('loading');
  });

  it('SUBMIT is ignored while loading (no double submit)', () => {
    expect(nextState(loadingState, { type: 'SUBMIT' })).toBe(loadingState);
  });

  it('SUBMIT is ignored while rate_limited', () => {
    const rl: GateState = {
      status: 'rate_limited',
      errorMessage: MSG_RATE_LIMITED,
      contentUnlocked: false,
    };
    expect(nextState(rl, { type: 'SUBMIT' })).toBe(rl);
  });

  it('SUCCESS while loading exposes content', () => {
    const result = nextState(loadingState, { type: 'SUCCESS' });
    expect(result.contentUnlocked).toBe(true);
    expect(result.status).toBe('idle');
  });

  it('SUCCESS is ignored when not loading (content stays locked)', () => {
    expect(nextState(initialGateState, { type: 'SUCCESS' }).contentUnlocked).toBe(
      false,
    );
  });

  it('UNAUTHORIZED while loading shows "Kata sandi salah"', () => {
    const result = nextState(loadingState, { type: 'UNAUTHORIZED' });
    expect(result.status).toBe('error');
    expect(result.errorMessage).toBe(MSG_WRONG_PASSWORD);
    expect(result.contentUnlocked).toBe(false);
  });

  it('RATE_LIMITED while loading enters rate_limited with message', () => {
    const result = nextState(loadingState, { type: 'RATE_LIMITED' });
    expect(result.status).toBe('rate_limited');
    expect(result.errorMessage).toBe(MSG_RATE_LIMITED);
  });

  it('NOT_FOUND and GONE map to their messages', () => {
    expect(nextState(loadingState, { type: 'NOT_FOUND' }).errorMessage).toBe(
      MSG_NOT_FOUND,
    );
    expect(nextState(loadingState, { type: 'GONE' }).errorMessage).toBe(MSG_GONE);
  });

  it('COOLDOWN_ELAPSED re-enables the form from rate_limited only', () => {
    const rl: GateState = {
      status: 'rate_limited',
      errorMessage: MSG_RATE_LIMITED,
      contentUnlocked: false,
    };
    expect(nextState(rl, { type: 'COOLDOWN_ELAPSED' }).status).toBe('idle');
    // No-op from other states.
    expect(nextState(loadingState, { type: 'COOLDOWN_ELAPSED' })).toBe(
      loadingState,
    );
  });
});

describe('isFormEnabled', () => {
  it('is enabled in idle and error, disabled in loading and rate_limited', () => {
    expect(isFormEnabled('idle')).toBe(true);
    expect(isFormEnabled('error')).toBe(true);
    expect(isFormEnabled('loading')).toBe(false);
    expect(isFormEnabled('rate_limited')).toBe(false);
  });
});

describe('eventForStatus', () => {
  it('maps known HTTP statuses to events', () => {
    expect(eventForStatus(401)).toEqual({ type: 'UNAUTHORIZED' });
    expect(eventForStatus(429)).toEqual({ type: 'RATE_LIMITED' });
    expect(eventForStatus(404)).toEqual({ type: 'NOT_FOUND' });
    expect(eventForStatus(410)).toEqual({ type: 'GONE' });
  });

  it('maps unknown statuses to a generic error event', () => {
    expect(eventForStatus(500)).toEqual({ type: 'ERROR' });
  });
});

describe('filenameFromContentDisposition', () => {
  it('returns null for a missing header', () => {
    expect(filenameFromContentDisposition(null)).toBeNull();
  });

  it('parses a quoted plain filename', () => {
    expect(
      filenameFromContentDisposition('attachment; filename="report.pdf"'),
    ).toBe('report.pdf');
  });

  it('parses an unquoted plain filename', () => {
    expect(filenameFromContentDisposition('attachment; filename=data.bin')).toBe(
      'data.bin',
    );
  });

  it('parses an RFC 5987 extended filename', () => {
    expect(
      filenameFromContentDisposition(
        "attachment; filename*=UTF-8''na%20me.txt",
      ),
    ).toBe('na me.txt');
  });
});
