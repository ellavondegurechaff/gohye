import { NextRequest } from 'next/server';
import { getServerSession } from '@/lib/auth';

export async function GET(request: NextRequest) {
  const session = await getServerSession();
  if (!session) {
    return new Response('Unauthorized', { status: 401 });
  }

  const { searchParams } = new URL(request.url);
  const backendUrl = process.env.GO_BACKEND_URL || 'http://localhost:8080';

  try {
    const response = await fetch(
      `${backendUrl}/admin/api/users?${searchParams}`,
      {
        headers: {
          Cookie: request.headers.get('cookie') || '',
          'Accept': 'application/json',
        },
        credentials: 'include',
      }
    );

    if (!response.ok) {
      return new Response('Failed to fetch users', { status: response.status });
    }

    const data = await response.json();

    return Response.json(data, {
      headers: {
        'Cache-Control': 'public, max-age=30, s-maxage=60',
      },
    });
  } catch (error) {
    console.error('Failed to fetch users:', error);
    return new Response('Internal Server Error', { status: 500 });
  }
}