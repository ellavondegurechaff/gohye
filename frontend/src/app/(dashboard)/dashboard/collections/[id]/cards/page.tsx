import { Suspense } from 'react';
import { CollectionCardsPageContent } from '@/components/collections/collection-cards-page';
import { Skeleton } from '@/components/ui/skeleton';

interface PageProps {
  params: Promise<{
    id: string;
  }>;
  searchParams: Promise<{
    page?: string;
    limit?: string;
    search?: string;
    level?: string;
    animated?: string;
  }>;
}

export default async function CollectionCardsPage({ params, searchParams }: PageProps) {
  const resolvedParams = await params;
  const resolvedSearchParams = await searchParams;
  
  return (
    <div className="min-h-screen bg-gradient-to-br from-black via-zinc-900/50 to-black">
      <Suspense fallback={<CollectionCardsPageSkeleton />}>
        <CollectionCardsPageContent 
          collectionId={resolvedParams.id}
          searchParams={resolvedSearchParams}
        />
      </Suspense>
    </div>
  );
}

function CollectionCardsPageSkeleton() {
  return (
    <div className="container mx-auto px-6 py-8 space-y-8">
      {/* Header Skeleton */}
      <div className="space-y-4">
        <Skeleton className="h-8 w-64 bg-zinc-800/50" />
        <Skeleton className="h-4 w-96 bg-zinc-800/50" />
      </div>
      
      {/* Stats Skeleton */}
      <div className="grid grid-cols-2 sm:grid-cols-4 gap-4">
        {Array.from({ length: 4 }).map((_, i) => (
          <Skeleton key={i} className="h-20 bg-zinc-800/50 rounded-lg" />
        ))}
      </div>
      
      {/* Filters Skeleton */}
      <div className="flex gap-4">
        <Skeleton className="h-12 flex-1 bg-zinc-800/50" />
        <Skeleton className="h-12 w-32 bg-zinc-800/50" />
        <Skeleton className="h-12 w-24 bg-zinc-800/50" />
      </div>
      
      {/* Cards Grid Skeleton */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-6">
        {Array.from({ length: 12 }).map((_, i) => (
          <Skeleton key={i} className="aspect-[3/4] bg-zinc-800/50 rounded-lg" />
        ))}
      </div>
    </div>
  );
}