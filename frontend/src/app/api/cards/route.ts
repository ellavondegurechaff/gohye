import { NextRequest } from 'next/server';
import { getServerSession } from '@/lib/auth';

export async function GET(request: NextRequest) {
  const session = await getServerSession();
  if (!session) {
    return new Response('Unauthorized', { status: 401 });
  }

  const backendUrl = process.env.GO_BACKEND_URL || 'http://localhost:8080';
  const { searchParams } = new URL(request.url);

  try {
    const response = await fetch(
      `${backendUrl}/admin/api/cards?${searchParams}`,
      {
        headers: {
          Cookie: request.headers.get('cookie') || '',
          'Accept': 'application/json',
        },
        credentials: 'include',
      }
    );

    if (!response.ok) {
      console.error('Backend API error:', response.status, response.statusText);
      return Response.json(
        { 
          success: false, 
          error: 'Failed to fetch cards from backend',
          data: {
            cards: [],
            total: 0,
            page: 1,
            limit: 50,
            total_pages: 0,
            has_more: false,
            has_prev: false,
          }
        }, 
        { status: response.status }
      );
    }

    const data = await response.json();
    
    return Response.json(data, {
      headers: {
        'Cache-Control': 'public, max-age=30, s-maxage=60, stale-while-revalidate=300',
      },
    });
  } catch (error) {
    console.error('Failed to fetch cards:', error);
    return Response.json(
      { 
        success: false, 
        error: 'Internal server error',
        data: {
          cards: [],
          total: 0,
          page: 1,
          limit: 50,
          total_pages: 0,
          has_more: false,
          has_prev: false,
        }
      }, 
      { status: 500 }
    );
  }
}

export async function POST(request: NextRequest) {
  const session = await getServerSession();
  if (!session) {
    return new Response('Unauthorized', { status: 401 });
  }

  const backendUrl = process.env.GO_BACKEND_URL || 'http://localhost:8080';

  try {
    const body = await request.formData();
    
    const response = await fetch(
      `${backendUrl}/admin/cards`,
      {
        method: 'POST',
        headers: {
          Cookie: request.headers.get('cookie') || '',
        },
        credentials: 'include',
        body,
      }
    );

    if (!response.ok) {
      const error = await response.json().catch(() => ({ message: 'Unknown error' }));
      return Response.json({ success: false, error: error.message }, { status: response.status });
    }

    const data = await response.json();
    return Response.json(data);
  } catch (error) {
    console.error('Failed to create card:', error);
    return Response.json({ success: false, error: 'Internal server error' }, { status: 500 });
  }
}