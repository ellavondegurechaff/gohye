import { NextRequest } from 'next/server';
import { cookies } from 'next/headers';
import type { CardDTO, APIResponse } from '@/lib/types';

export async function GET(
  request: NextRequest,
  { params }: { params: Promise<{ id: string }> }
) {
  const resolvedParams = await params;
  const cookieStore = await cookies();
  const sessionCookie = await cookieStore.get("gohye_session");
  
  if (!sessionCookie) {
    return new Response('Unauthorized', { status: 401 });
  }

  const backendUrl = process.env.GO_BACKEND_URL || 'http://localhost:8080';

  try {
    const response = await fetch(
      `${backendUrl}/admin/api/collections/${resolvedParams.id}/cards`,
      {
        headers: {
          Cookie: `gohye_session=${sessionCookie.value}`,
          'Accept': 'application/json',
        },
        cache: 'no-store',
      }
    );

    if (!response.ok) {
      console.error('Backend API error:', response.status, response.statusText);
      return Response.json(
        { 
          success: false, 
          error: 'Failed to fetch cards from backend',
          data: []
        }, 
        { status: response.status }
      );
    }

    const result: APIResponse<CardDTO[]> = await response.json();
    
    return Response.json(result, {
      headers: {
        'Cache-Control': 'public, max-age=30, s-maxage=60, stale-while-revalidate=300',
      },
    });
  } catch (error) {
    console.error('Failed to fetch collection cards:', error);
    return Response.json(
      { 
        success: false, 
        error: 'Internal server error',
        data: []
      }, 
      { status: 500 }
    );
  }
}