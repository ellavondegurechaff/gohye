import { NextRequest } from 'next/server';
import { getServerSession } from '@/lib/auth';

export async function POST(request: NextRequest) {
  const session = await getServerSession();
  if (!session) {
    return new Response('Unauthorized', { status: 401 });
  }

  const backendUrl = process.env.GO_BACKEND_URL || 'http://localhost:8080';

  try {
    const body = await request.json();
    
    const response = await fetch(
      `${backendUrl}/admin/cards/bulk`,
      {
        method: 'POST',
        headers: {
          Cookie: request.headers.get('cookie') || '',
          'Content-Type': 'application/json',
          'Accept': 'application/json',
        },
        credentials: 'include',
        body: JSON.stringify(body),
      }
    );

    if (!response.ok) {
      const error = await response.json().catch(() => ({ message: 'Bulk operation failed' }));
      return Response.json({ success: false, error: error.message }, { status: response.status });
    }

    const data = await response.json();
    return Response.json(data);
  } catch (error) {
    console.error('Failed to perform bulk operation:', error);
    return Response.json({ success: false, error: 'Internal server error' }, { status: 500 });
  }
}