'use client';

// app/[slug]/unlock/page.tsx
//
// Paste unlock page (Client Component) — Requirements 3.10, 4.1.
//
// Reached when the paste view page (`app/[slug]/page.tsx`) redirects here after
// the backend returns HTTP 401 with `password_required` for a protected paste
// (Req 3.10). It renders the shared {@link PasswordGate} so the visitor can
// enter the password (Req 4.1).
//
// Approach (design task 9.1, option b): rather than navigating away after a
// successful unlock, this page supplies an `onUnlock` callback to PasswordGate.
// On a successful (HTTP 200) unlock the gate hands back the decoded
// {@link PasteViewResponse}, which we store in state and render inline via
// {@link PasteViewer} — so the unlocked content displays immediately without a
// second round-trip to the backend.
//
// Next.js 16: dynamic route `params` is a Promise even in a Client Component.
// It is unwrapped synchronously with React's `use()` hook.

import { use, useState } from 'react';

import { PasswordGate } from '@/components/PasswordGate';
import { PasteViewer } from '@/components/PasteViewer';
import type { PasteViewResponse } from '@/lib/types';

export default function PasteUnlockPage({
  params,
}: {
  params: Promise<{ slug: string }>;
}) {
  // Next.js 16: unwrap the async params Promise in the Client Component.
  const { slug } = use(params);

  // Holds the paste once it has been successfully unlocked (HTTP 200). While
  // null, the password gate is shown; once set, the viewer renders inline.
  const [paste, setPaste] = useState<PasteViewResponse | null>(null);

  return (
    <section>
      {paste ? (
        <PasteViewer paste={paste} />
      ) : (
        <PasswordGate slug={slug} resourceType="paste" onUnlock={setPaste} />
      )}
    </section>
  );
}
