import { NextRequest } from 'next/server';
import { getServerSession } from '@/lib/auth';

export async function PATCH(
  request: NextRequest,
  { params }: { params: Promise<{ id: string }> }
) {
  const session = await getServerSession();
  if (!session) {
    return new Response('Unauthorized', { status: 401 });
  }

  const { id } = await params;
  const backendUrl = process.env.GO_BACKEND_URL || 'http://localhost:8080';

  try {
    const body = await request.json();
    
    const response = await fetch(
      `${backendUrl}/admin/api/users/${id}/admin`,
      {
        method: 'PATCH',
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
      const error = await response.json().catch(() => ({ message: 'Failed to update user admin status' }));
      return new Response(error.message, { status: response.status });
    }

    const data = await response.json();
    return Response.json(data);
  } catch (error) {
    console.error('Failed to update user admin status:', error);
    return new Response('Internal Server Error', { status: 500 });
  }
}