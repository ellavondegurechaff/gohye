import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";

export async function middleware(request: NextRequest) {
  const { pathname } = request.nextUrl;
  
  // Protect dashboard routes
  if (pathname.startsWith("/dashboard")) {
    const sessionCookie = request.cookies.get("gohye_session");
    
    if (!sessionCookie) {
      return NextResponse.redirect(new URL("/login", request.url));
    }
    
    // Validate with Go backend
    try {
      const response = await fetch(`${process.env.GO_BACKEND_URL}/api/auth/validate`, {
        headers: {
          Cookie: `gohye_session=${sessionCookie.value}`,
        },
      });
      
      if (!response.ok) {
        return NextResponse.redirect(new URL("/login", request.url));
      }
    } catch {
      return NextResponse.redirect(new URL("/login", request.url));
    }
  }
  
  return NextResponse.next();
}

export const config = {
  matcher: ["/dashboard/:path*"],
};