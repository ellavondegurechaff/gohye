import { Suspense } from "react";
import { RevolutionaryCardsPage } from "@/components/cards/revolutionary-cards-page";
import { CardsTableSkeleton } from "@/components/cards/cards-table-skeleton";
import { CardSearchParams } from "@/lib/types";

interface PageProps {
  searchParams: Promise<{
    search?: string;
    collection?: string;
    level?: string;
    animated?: string;
    page?: string;
    limit?: string;
  }>;
}

// Server component to fetch initial data
async function getInitialData(searchParams: CardSearchParams) {
  const backendUrl = process.env.GO_BACKEND_URL || 'http://localhost:8080';
  
  try {
    const urlSearchParams = new URLSearchParams();
    
    if (searchParams.search) urlSearchParams.append("search", searchParams.search);
    if (searchParams.collection) urlSearchParams.append("collection", searchParams.collection);
    if (searchParams.level) urlSearchParams.append("level", searchParams.level.toString());
    if (searchParams.animated !== undefined) urlSearchParams.append("animated", searchParams.animated.toString());
    if (searchParams.page) urlSearchParams.append("page", searchParams.page.toString());
    if (searchParams.limit) urlSearchParams.append("limit", searchParams.limit.toString());

    const [cardsResponse, collectionsResponse] = await Promise.all([
      fetch(`${backendUrl}/admin/api/cards?${urlSearchParams}`, {
        headers: { 'Accept': 'application/json' },
        credentials: 'include',
      }),
      fetch(`${backendUrl}/admin/api/collections`, {
        headers: { 'Accept': 'application/json' },
        credentials: 'include',
      })
    ]);
    
    if (!cardsResponse.ok || !collectionsResponse.ok) {
      return {
        cards: [],
        collections: [],
        pagination: {
          total: 0,
          page: 1,
          limit: 50,
          total_pages: 0,
          has_more: false,
          has_prev: false,
        },
      };
    }
    
    const cardsResult = await cardsResponse.json();
    const collectionsResult = await collectionsResponse.json();
    
    return {
      cards: cardsResult.data?.cards || [],
      collections: collectionsResult.data || [],
      pagination: {
        total: cardsResult.data?.total || 0,
        page: cardsResult.data?.page || 1,
        limit: cardsResult.data?.limit || 50,
        total_pages: cardsResult.data?.total_pages || 0,
        has_more: cardsResult.data?.has_more || false,
        has_prev: cardsResult.data?.has_prev || false,
      },
    };
  } catch (error) {
    console.error('Failed to fetch initial data:', error);
    return {
      cards: [],
      collections: [],
      pagination: {
        total: 0,
        page: 1,
        limit: 50,
        total_pages: 0,
        has_more: false,
        has_prev: false,
      },
    };
  }
}

export default async function CardsPage({ searchParams }: PageProps) {
  const params = await searchParams;
  
  const searchQuery: CardSearchParams = {
    search: params.search || undefined,
    collection: params.collection || undefined,
    level: params.level ? parseInt(params.level) : undefined,
    animated: params.animated === "true" ? true : params.animated === "false" ? false : undefined,
    page: params.page ? parseInt(params.page) : 1,
    limit: params.limit ? parseInt(params.limit) : 50,
  };

  const initialData = await getInitialData(searchQuery);

  return (
    <Suspense fallback={<CardsTableSkeleton />}>
      <RevolutionaryCardsPage
        initialCards={initialData.cards}
        initialCollections={initialData.collections}
        initialPagination={initialData.pagination}
        initialSearchParams={searchQuery}
      />
    </Suspense>
  );
}