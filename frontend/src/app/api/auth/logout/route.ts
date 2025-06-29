import { NextRequest } from "next/server";

export async function POST(request: NextRequest) {
  try {
    // Forward the logout request to the Go backend
    const response = await fetch(`${process.env.GO_BACKEND_URL}/auth/logout`, {
      method: "POST",
      headers: {
        Cookie: request.headers.get("cookie") || "",
      },
    });

    if (!response.ok) {
      return Response.json(
        { success: false, error: "Logout failed" },
        { status: response.status }
      );
    }

    return Response.json(
      { success: true, message: "Logged out successfully" },
      { status: 200 }
    );
  } catch (error) {
    return Response.json(
      { success: false, error: "Internal server error" },
      { status: 500 }
    );
  }
}