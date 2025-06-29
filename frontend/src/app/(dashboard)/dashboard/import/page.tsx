import { Suspense } from "react";
import { cookies } from "next/headers";
import { RevolutionaryImport } from "@/components/import/revolutionary-import";
import type { CollectionDTO, APIResponse } from "@/lib/types";

// Server-side data fetching with proper cookie forwarding
async function getCollectionsData(): Promise<CollectionDTO[]> {
  try {
    const cookieStore = await cookies();
    const sessionCookie = await cookieStore.get("gohye_session");
    
    if (!sessionCookie) {
      console.warn('No session cookie found for import collections fetch');
      return [];
    }
    
    const backendUrl = process.env.GO_BACKEND_URL || 'http://localhost:8080';
    const response = await fetch(`${backendUrl}/admin/api/collections`, {
      headers: {
        Cookie: `gohye_session=${sessionCookie.value}`,
        'Accept': 'application/json',
      },
      cache: 'no-store', // Ensure fresh data
    });
    
    if (!response.ok) {
      console.error('Backend API error:', response.status, response.statusText);
      return [];
    }
    
    const result: APIResponse<CollectionDTO[]> = await response.json();
    const collections = result.success ? result.data : [];
    console.log('Successfully fetched collections for import:', collections?.length || 0);
    return collections || [];
  } catch (error) {
    console.error('Failed to fetch collections for import - detailed error:', {
      error: error instanceof Error ? error.message : 'Unknown error',
      stack: error instanceof Error ? error.stack : undefined,
    });
    
    // Return empty array for graceful degradation
    return [];
  }
}

export default async function ImportPage() {
  const collections = await getCollectionsData();

  return (
    <Suspense fallback={
      <div className="min-h-screen bg-gradient-to-br from-black via-zinc-900 to-black flex items-center justify-center">
        <div className="h-16 w-16 border-4 border-purple-500 border-t-transparent rounded-full animate-spin" />
      </div>
    }>
      <RevolutionaryImport collections={collections} />
    </Suspense>
  );
}