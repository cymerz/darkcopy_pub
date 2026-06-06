// lib/api.test.ts
/**
 * Property 4: API Error Propagation
 *
 * For any HTTP error response (status >= 400) with a valid JSON body containing
 * `error`, `code`, and `status` fields, `apiFetch` must throw an `APIError`
 * with matching `message`, `code`, and `status` properties.
 *
 * Validates: Requirements 1.5, 3.8, 3.9, 7.1, 7.2, 7.3, 7.4
 */

import fc from 'fast-check';

// ---------------------------------------------------------------------------
// We use jest.resetModules() + dynamic require so that the APIError class
// imported in the test is always the SAME module instance as the one used
// inside api.ts — avoiding cross-realm instanceof failures.
// ---------------------------------------------------------------------------

let getPaste: (slug: string) => Promise<unknown>;
// eslint-disable-next-line @typescript-eslint/no-explicit-any
let APIErrorClass: new (message: string, code: string, status: number) => any;

beforeEach(() => {
  jest.resetModules();
  // eslint-disable-next-line @typescript-eslint/no-require-imports
  ({ getPaste } = require('./api'));
  // eslint-disable-next-line @typescript-eslint/no-require-imports
  ({ APIError: APIErrorClass } = require('./types'));
});

afterEach(() => {
  jest.restoreAllMocks();
});

// ---------------------------------------------------------------------------
// Property 4a: Any status >= 400 with a well-formed JSON error body throws
//              APIError with matching fields.
// ---------------------------------------------------------------------------
describe('apiFetch - Property 4: API Error Propagation', () => {
  it('throws APIError with matching fields for any HTTP status >= 400 with valid JSON error body', async () => {
    await fc.assert(
      fc.asyncProperty(
        // HTTP error status codes (400–599)
        fc.integer({ min: 400, max: 599 }),
        // Arbitrary non-empty error message
        fc.string({ minLength: 1, maxLength: 200 }),
        // Arbitrary non-empty error code (printable ASCII, no whitespace-only)
        fc
          .string({ minLength: 1, maxLength: 50 })
          .filter((s) => s.trim().length > 0),
        async (status, errorMessage, errorCode) => {
          const body = JSON.stringify({
            error: errorMessage,
            code: errorCode,
            status,
          });

          global.fetch = jest.fn().mockResolvedValueOnce(
            new Response(body, {
              status,
              headers: { 'Content-Type': 'application/json' },
            }),
          );

          let thrown: unknown;
          try {
            await getPaste('test-slug');
          } catch (e) {
            thrown = e;
          }

          // Must throw
          expect(thrown).toBeDefined();
          // Must be an APIError (same module instance via resetModules)
          expect(thrown).toBeInstanceOf(APIErrorClass);

          // Fields must match the response body
          expect((thrown as { status: number }).status).toBe(status);
          expect((thrown as { code: string }).code).toBe(errorCode);
          expect((thrown as Error).message).toBe(errorMessage);
        },
      ),
      { numRuns: 100 },
    );
  });

  // ---------------------------------------------------------------------------
  // Property 4b: When the response body is not valid JSON, apiFetch still throws
  //              APIError with the fallback values.
  // ---------------------------------------------------------------------------
  it('throws APIError with fallback fields when response body is not valid JSON', async () => {
    await fc.assert(
      fc.asyncProperty(
        fc.integer({ min: 400, max: 599 }),
        async (status) => {
          global.fetch = jest.fn().mockResolvedValueOnce(
            new Response('not json at all', {
              status,
              headers: { 'Content-Type': 'text/plain' },
            }),
          );

          let thrown: unknown;
          try {
            await getPaste('test-slug');
          } catch (e) {
            thrown = e;
          }

          expect(thrown).toBeInstanceOf(APIErrorClass);
          // Status must still match the HTTP status
          expect((thrown as { status: number }).status).toBe(status);
          // Code must fall back to UNKNOWN
          expect((thrown as { code: string }).code).toBe('UNKNOWN');
          // Message must fall back to 'Unknown error'
          expect((thrown as Error).message).toBe('Unknown error');
        },
      ),
      { numRuns: 50 },
    );
  });

  // ---------------------------------------------------------------------------
  // Property 4c: Successful responses (200, 201) never throw.
  // Restrict to statuses that the Response constructor accepts with a body.
  // ---------------------------------------------------------------------------
  it('never throws for 200/201 responses with valid JSON body', async () => {
    await fc.assert(
      fc.asyncProperty(
        fc.constantFrom(200, 201),
        async (status) => {
          // Minimal valid PasteViewResponse shape
          const body = JSON.stringify({
            slug: 'abc',
            title: '',
            content: '',
            highlighted_html: '',
            language: 'plaintext',
            visibility: 'public',
            created_at: new Date().toISOString(),
            expires_at: null,
            remaining_seconds: null,
          });

          global.fetch = jest.fn().mockResolvedValueOnce(
            new Response(body, {
              status,
              headers: { 'Content-Type': 'application/json' },
            }),
          );

          let thrown: unknown;
          try {
            await getPaste('abc');
          } catch (e) {
            thrown = e;
          }

          expect(thrown).toBeUndefined();
        },
      ),
      { numRuns: 20 },
    );
  });

  // ---------------------------------------------------------------------------
  // Concrete unit tests for specific status codes mentioned in requirements.
  // ---------------------------------------------------------------------------
  describe('concrete error status codes', () => {
    const cases: Array<{ status: number; code: string; message: string }> = [
      { status: 400, code: 'BAD_REQUEST', message: 'Form tidak valid' },
      { status: 401, code: 'PASSWORD_REQUIRED', message: 'Password diperlukan' },
      { status: 404, code: 'NOT_FOUND', message: 'Paste tidak ditemukan' },
      { status: 410, code: 'RESOURCE_EXPIRED', message: 'Paste ini telah kadaluarsa' },
      { status: 429, code: 'RATE_LIMITED', message: 'Terlalu banyak percobaan' },
      { status: 500, code: 'INTERNAL_ERROR', message: 'Gagal memuat paste' },
    ];

    test.each(cases)(
      'HTTP $status ($code) throws APIError with correct fields',
      async ({ status, code, message }) => {
        const makeResponse = () =>
          new Response(JSON.stringify({ error: message, code, status }), {
            status,
            headers: { 'Content-Type': 'application/json' },
          });

        global.fetch = jest.fn().mockResolvedValueOnce(makeResponse());

        let thrown: unknown;
        try {
          await getPaste('some-slug');
        } catch (e) {
          thrown = e;
        }

        expect(thrown).toBeInstanceOf(APIErrorClass);
        expect((thrown as { status: number }).status).toBe(status);
        expect((thrown as { code: string }).code).toBe(code);
        expect((thrown as Error).message).toBe(message);
      },
    );
  });
});
