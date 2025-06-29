import { NextRequest } from 'next/server';
import { getServerSession } from '@/lib/auth';

interface RouteContext {
  params: Promise<{ id: string }>;
}

export async function GET(request: NextRequest, context: RouteContext) {
  const session = await getServerSession();
  if (!session) {
    return new Response('Unauthorized', { status: 401 });
  }

  const { id } = await context.params;
  const backendUrl = process.env.GO_BACKEND_URL || 'http://localhost:8080';

  try {
    const response = await fetch(
      `${backendUrl}/admin/cards/${id}`,
      {
        headers: {
          Cookie: request.headers.get('cookie') || '',
          'Accept': 'application/json',
        },
        credentials: 'include',
      }
    );

    if (!response.ok) {
      const error = await response.json().catch(() => ({ message: 'Card not found' }));
      return Response.json({ success: false, error: error.message }, { status: response.status });
    }

    const data = await response.json();
    
    return Response.json(data, {
      headers: {
        'Cache-Control': 'public, max-age=300, s-maxage=600',
      },
    });
  } catch (error) {
    console.error('Failed to fetch card:', error);
    return Response.json({ success: false, error: 'Internal server error' }, { status: 500 });
  }
}

export async function PUT(request: NextRequest, context: RouteContext) {
  const session = await getServerSession();
  if (!session) {
    return new Response('Unauthorized', { status: 401 });
  }

  const { id } = await context.params;
  const backendUrl = process.env.GO_BACKEND_URL || 'http://localhost:8080';

  try {
    const body = await request.formData();
    
    const response = await fetch(
      `${backendUrl}/admin/cards/${id}`,
      {
        method: 'PUT',
        headers: {
          Cookie: request.headers.get('cookie') || '',
        },
        credentials: 'include',
        body,
      }
    );

    if (!response.ok) {
      const error = await response.json().catch(() => ({ message: 'Failed to update card' }));
      return Response.json({ success: false, error: error.message }, { status: response.status });
    }

    const data = await response.json();
    return Response.json(data);
  } catch (error) {
    console.error('Failed to update card:', error);
    return Response.json({ success: false, error: 'Internal server error' }, { status: 500 });
  }
}

export async function DELETE(request: NextRequest, context: RouteContext) {
  const session = await getServerSession();
  if (!session) {
    return new Response('Unauthorized', { status: 401 });
  }

  const { id } = await context.params;
  const backendUrl = process.env.GO_BACKEND_URL || 'http://localhost:8080';

  try {
    const response = await fetch(
      `${backendUrl}/admin/cards/${id}`,
      {
        method: 'DELETE',
        headers: {
          Cookie: request.headers.get('cookie') || '',
          'Accept': 'application/json',
        },
        credentials: 'include',
      }
    );

    if (!response.ok) {
      const error = await response.json().catch(() => ({ message: 'Failed to delete card' }));
      return Response.json({ success: false, error: error.message }, { status: response.status });
    }

    const data = await response.json();
    return Response.json(data);
  } catch (error) {
    console.error('Failed to delete card:', error);
    return Response.json({ success: false, error: 'Internal server error' }, { status: 500 });
  }
}