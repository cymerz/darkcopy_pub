import type { Metadata } from 'next';
import { AdminDashboard } from '@/components/AdminDashboard';

// Hidden admin route.
//
// This page is intentionally NOT linked from the main navigation (Header). It
// is reachable only by navigating directly to /admin. Access to the underlying
// data still requires a valid admin token, which is entered in the UI and
// verified against the backend — the page shell itself contains no secrets.

// Keep the admin route out of search engines and crawlers.
export const metadata: Metadata = {
  title: 'Panel Admin',
  robots: {
    index: false,
    follow: false,
    nocache: true,
    googleBot: { index: false, follow: false },
  },
};

// The dashboard reads/writes sessionStorage and is fully interactive, so render
// it on demand rather than statically.
export const dynamic = 'force-dynamic';

export default function AdminPage() {
  return <AdminDashboard />;
}
