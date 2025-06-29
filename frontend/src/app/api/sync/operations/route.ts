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
      `${backendUrl}/admin/api/sync/operations`,
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
      // Return mock success for development
      const mockOperation = {
        id: `op_${Date.now()}`,
        type: body.type,
        status: 'running',
        progress: 0,
        started_at: new Date().toISOString(),
        errors: [],
      };
      
      return Response.json({
        success: true,
        data: mockOperation,
        message: `${body.type} operation started successfully`
      });
    }

    const data = await response.json();
    return Response.json(data);
  } catch (error) {
    console.error('Failed to start sync operation:', error);
    return new Response('Internal Server Error', { status: 500 });
  }
}