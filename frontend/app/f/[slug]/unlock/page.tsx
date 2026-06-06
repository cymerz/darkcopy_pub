// app/f/[slug]/unlock/page.tsx
//
// File unlock page (Server Component) — Requirements 6.1–6.5.
//
// Reached when the file download flow encounters a protected file (backend
// returns HTTP 401 with `password_required: true`) and the router navigates
// here (Req 6.1). It renders the shared {@link PasswordGate} configured for the
// file resource type so the visitor can enter the password and download the
// file (Req 6.2, 6.3, 6.4, 6.5).
//
// Approach: unlike the paste unlock page (which is a Client Component using
// React's `use()` because it renders the unlocked paste inline via PasteViewer),
// a protected file has no inline content to render — on a successful unlock
// PasswordGate streams the response body into a Blob and triggers a browser
// download itself (Req 6.3). There is therefore nothing for this page to manage
// in client state, so it is a plain Server Component that simply awaits the
// async route params and renders the (Client Component) PasswordGate.
//
// Next.js 16: dynamic route `params` is a Promise and is awaited directly in
// this async Server Component.

import { PasswordGate } from '@/components/PasswordGate';

export default async function FileUnlockPage({
  params,
}: {
  params: Promise<{ slug: string }>;
}) {
  // Next.js 16: await the async params Promise in the Server Component.
  const { slug } = await params;

  return (
    <section>
      <PasswordGate slug={slug} resourceType="file" />
    </section>
  );
}
