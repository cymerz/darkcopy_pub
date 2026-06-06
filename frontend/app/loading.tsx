import { PasteListSkeleton, Skeleton } from '@/components/LoadingSkeleton';

/**
 * Route-level loading UI for the home page (Req 1.4). Next.js renders this
 * automatically while the home page Server Component fetches the recent paste
 * list. The layout mirrors the real home page: a heading area followed by a
 * list of card-shaped skeleton placeholders.
 */
export default function Loading() {
  return (
    <div className="space-y-6">
      {/* Heading area placeholder (matches the "Paste Publik Terbaru" title) */}
      <Skeleton className="h-8 w-64" />

      {/* Paste list placeholders */}
      <PasteListSkeleton />
    </div>
  );
}
