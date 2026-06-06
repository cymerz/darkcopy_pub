/**
 * @jest-environment jsdom
 */
// components/PasswordGate.property.test.tsx
//
// Property-based tests for the PasswordGate state machine (design.md
// "Property 5: Password Gate State Machine").
//
// Property 5: For any sequence of unlock attempts on a password-protected
// resource, the gate transitions between exactly the valid states
// (idle = form enabled, loading = form disabled, error = message shown / form
// re-enabled, plus the rate_limited cooldown state) and NEVER exposes content
// without a successful (HTTP 200) unlock response that followed an in-flight
// (loading) request.
//
// The observable UI is driven entirely by the pure reducer `nextState`, so we
// verify Property 5 by generating arbitrary sequences of GateEvents and folding
// them through `nextState` from `initialGateState`, asserting the invariants
// after every step.
//
// Validates: Requirements 4.1, 4.4, 4.5, 6.4, 6.5
import fc from 'fast-check';
import {
  nextState,
  isFormEnabled,
  initialGateState,
  MSG_WRONG_PASSWORD,
  MSG_RATE_LIMITED,
  type GateState,
  type GateStatus,
  type GateEvent,
} from './PasswordGate';

// All four observable gate states (Property 5 invariant #1).
const VALID_STATUSES: GateStatus[] = [
  'idle',
  'loading',
  'error',
  'rate_limited',
];

// The states from which the form is interactive (a SUBMIT may start a request).
const ENABLED_STATUSES: GateStatus[] = ['idle', 'error'];

// Backend-outcome events: each represents an HTTP response and is only honored
// while a request is in flight (status === 'loading').
const BACKEND_OUTCOME_TYPES = new Set<GateEvent['type']>([
  'SUCCESS',
  'UNAUTHORIZED',
  'RATE_LIMITED',
  'NOT_FOUND',
  'GONE',
  'ERROR',
]);

/**
 * Smart generator over the full GateEvent space. ERROR sometimes carries a
 * custom message and sometimes omits it (exercising the MSG_GENERIC fallback).
 */
const gateEventArb: fc.Arbitrary<GateEvent> = fc.oneof(
  fc.constant<GateEvent>({ type: 'SUBMIT' }),
  fc.constant<GateEvent>({ type: 'SUCCESS' }),
  fc.constant<GateEvent>({ type: 'UNAUTHORIZED' }),
  fc.constant<GateEvent>({ type: 'RATE_LIMITED' }),
  fc.constant<GateEvent>({ type: 'NOT_FOUND' }),
  fc.constant<GateEvent>({ type: 'GONE' }),
  fc.constant<GateEvent>({ type: 'COOLDOWN_ELAPSED' }),
  fc
    .option(fc.string(), { nil: undefined })
    .map((message): GateEvent => ({ type: 'ERROR', message })),
);

/** A sequence of unlock-attempt events to fold through the reducer. */
const eventSequenceArb = fc.array(gateEventArb, {
  minLength: 0,
  maxLength: 50,
});

describe('PasswordGate state machine - Property 5: Password Gate State Machine', () => {
  // Validates: Requirements 4.1, 4.4, 4.5, 6.4, 6.5
  it('holds all Property 5 invariants across arbitrary event sequences', () => {
    fc.assert(
      fc.property(eventSequenceArb, (events) => {
        let prevState: GateState = initialGateState;
        // Tracks whether a legitimate unlock (SUCCESS while loading) has ever
        // occurred; content must never be exposed without one.
        let sawLegitUnlock = false;

        for (const event of events) {
          const newState = nextState(prevState, event);

          // --- Invariant 1: status is always a valid state. -----------------
          expect(VALID_STATUSES).toContain(newState.status);

          const isLegitSuccess =
            event.type === 'SUCCESS' && prevState.status === 'loading';

          // --- Invariant 2: CONTENT SAFETY (the core of Property 5). --------
          // contentUnlocked transitions false -> true at a step IFF that step's
          // event is SUCCESS applied while the previous state was 'loading'.
          const becameUnlocked =
            !prevState.contentUnlocked && newState.contentUnlocked;
          expect(becameUnlocked).toBe(isLegitSuccess);

          // A legit success always exposes content...
          if (isLegitSuccess) {
            expect(newState.contentUnlocked).toBe(true);
            sawLegitUnlock = true;
          }
          // ...and content is NEVER exposed without some prior/just-now legit
          // unlock response.
          if (newState.contentUnlocked) {
            expect(sawLegitUnlock).toBe(true);
          }

          // --- Invariant 3: form-enabled correctness. -----------------------
          // Enabled exactly in idle/error; disabled in loading/rate_limited.
          const expectedEnabled = ENABLED_STATUSES.includes(newState.status);
          expect(isFormEnabled(newState.status)).toBe(expectedEnabled);

          // --- Invariant 4: SUBMIT semantics. -------------------------------
          if (event.type === 'SUBMIT') {
            if (isFormEnabled(prevState.status)) {
              expect(newState.status).toBe('loading');
              expect(newState.errorMessage).toBeNull();
              expect(newState.contentUnlocked).toBe(false);
            } else {
              // No-op while loading or rate_limited.
              expect(newState).toEqual(prevState);
            }
          }

          // --- Invariant 5: backend outcomes only apply while loading. ------
          if (
            BACKEND_OUTCOME_TYPES.has(event.type) &&
            prevState.status !== 'loading'
          ) {
            expect(newState).toEqual(prevState);
          }

          // --- Invariant 6: specific error messages while loading. ----------
          if (event.type === 'UNAUTHORIZED' && prevState.status === 'loading') {
            expect(newState.status).toBe('error');
            expect(newState.errorMessage).toBe(MSG_WRONG_PASSWORD);
            expect(newState.contentUnlocked).toBe(false);
          }
          if (event.type === 'RATE_LIMITED' && prevState.status === 'loading') {
            expect(newState.status).toBe('rate_limited');
            expect(newState.errorMessage).toBe(MSG_RATE_LIMITED);
            expect(newState.contentUnlocked).toBe(false);
          }

          prevState = newState;
        }
      }),
      { numRuns: 500 },
    );
  });

  // Validates: Requirements 4.4, 4.5
  it('never exposes content unless the immediately preceding state was loading via SUCCESS', () => {
    fc.assert(
      fc.property(eventSequenceArb, (events) => {
        let prevState: GateState = initialGateState;
        for (const event of events) {
          const newState = nextState(prevState, event);
          if (
            !prevState.contentUnlocked &&
            newState.contentUnlocked === true
          ) {
            // The only legitimate path to exposing content.
            expect(event.type).toBe('SUCCESS');
            expect(prevState.status).toBe('loading');
          }
          prevState = newState;
        }
      }),
      { numRuns: 500 },
    );
  });
});

describe('PasswordGate state machine - realistic unlock cycle', () => {
  // Validates: Requirements 4.1, 4.3, 4.4
  it('idle -> SUBMIT -> loading -> UNAUTHORIZED -> error -> SUBMIT -> loading -> SUCCESS -> unlocked', () => {
    // Start idle: form enabled, content locked.
    let state = initialGateState;
    expect(state.status).toBe('idle');
    expect(isFormEnabled(state.status)).toBe(true);
    expect(state.contentUnlocked).toBe(false);

    // First attempt -> loading (form disabled).
    state = nextState(state, { type: 'SUBMIT' });
    expect(state.status).toBe('loading');
    expect(isFormEnabled(state.status)).toBe(false);

    // Wrong password -> error (message shown, form re-enabled).
    state = nextState(state, { type: 'UNAUTHORIZED' });
    expect(state.status).toBe('error');
    expect(state.errorMessage).toBe(MSG_WRONG_PASSWORD);
    expect(isFormEnabled(state.status)).toBe(true);
    expect(state.contentUnlocked).toBe(false);

    // Retry -> loading again.
    state = nextState(state, { type: 'SUBMIT' });
    expect(state.status).toBe('loading');
    expect(isFormEnabled(state.status)).toBe(false);

    // Correct password -> content exposed.
    state = nextState(state, { type: 'SUCCESS' });
    expect(state.contentUnlocked).toBe(true);
    expect(state.status).toBe('idle');
    expect(state.errorMessage).toBeNull();
  });

  // Validates: Requirements 4.5, 6.5
  it('rate-limit cooldown cycle: loading -> RATE_LIMITED -> rate_limited -> COOLDOWN_ELAPSED -> idle', () => {
    let state = nextState(initialGateState, { type: 'SUBMIT' });
    expect(state.status).toBe('loading');

    state = nextState(state, { type: 'RATE_LIMITED' });
    expect(state.status).toBe('rate_limited');
    expect(state.errorMessage).toBe(MSG_RATE_LIMITED);
    expect(isFormEnabled(state.status)).toBe(false);
    expect(state.contentUnlocked).toBe(false);

    // SUBMIT during cooldown is ignored.
    expect(nextState(state, { type: 'SUBMIT' })).toEqual(state);

    // Cooldown elapses -> back to idle, form re-enabled.
    state = nextState(state, { type: 'COOLDOWN_ELAPSED' });
    expect(state.status).toBe('idle');
    expect(isFormEnabled(state.status)).toBe(true);
    expect(state.contentUnlocked).toBe(false);
  });
});
