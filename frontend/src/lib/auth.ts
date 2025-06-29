import { cookies } from "next/headers";
import type { UserSession, APIResponse } from "./types";

export async function getServerSession(): Promise<UserSession | null> {
  const cookieStore = await cookies();
  const sessionCookie = await cookieStore.get("gohye_session");
  
  if (!sessionCookie) return null;
  
  const backendUrl = process.env.GO_BACKEND_URL || 'http://localhost:8080';
  
  try {
    const response = await fetch(`${backendUrl}/api/auth/validate`, {
      headers: {
        Cookie: `gohye_session=${sessionCookie.value}`,
      },
      next: { revalidate: 300 }, // Cache for 5 minutes
    });
    
    if (!response.ok) return null;
    
    const result: APIResponse<{ user: UserSession }> = await response.json();
    return result.success ? result.data?.user || null : null;
  } catch (error) {
    console.warn('Backend not available, auth check failed:', error);
    return null;
  }
}

export async function validateSession(): Promise<boolean> {
  const session = await getServerSession();
  return session !== null && session.is_admin;
}