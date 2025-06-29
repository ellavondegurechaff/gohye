import { NextRequest } from 'next/server';
import { getServerSession } from '@/lib/auth';

export async function GET(request: NextRequest) {
  const session = await getServerSession();
  if (!session) {
    return new Response('Unauthorized', { status: 401 });
  }

  const backendUrl = process.env.GO_BACKEND_URL || 'http://localhost:8080';

  try {
    const response = await fetch(
      `${backendUrl}/admin/api/collections`,
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
          error: 'Failed to fetch collections from backend',
          data: []
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
    console.error('Failed to fetch collections:', error);
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

export async function POST(request: NextRequest) {
  const session = await getServerSession();
  if (!session) {
    return new Response('Unauthorized', { status: 401 });
  }

  const backendUrl = process.env.GO_BACKEND_URL || 'http://localhost:8080';

  try {
    const body = await request.json();
    
    const response = await fetch(
      `${backendUrl}/admin/collections`,
      {
        method: 'POST',
        headers: {
          Cookie: request.headers.get('cookie') || '',
          'Content-Type': 'application/json',
        },
        credentials: 'include',
        body: JSON.stringify(body),
      }
    );

    if (!response.ok) {
      const error = await response.json().catch(() => ({ message: 'Unknown error' }));
      return Response.json({ success: false, error: error.message }, { status: response.status });
    }

    const data = await response.json();
    return Response.json(data);
  } catch (error) {
    console.error('Failed to create collection:', error);
    return Response.json({ success: false, error: 'Internal server error' }, { status: 500 });
  }
}