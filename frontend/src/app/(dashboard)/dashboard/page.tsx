import { Suspense } from "react";
import { RevolutionaryDashboard } from "@/components/dashboard/revolutionary-dashboard";
import { apiClient } from "@/lib/api";

// Server-side data fetching
async function getDashboardData() {
  try {
    const stats = await apiClient.getDashboardStats();
    return stats;
  } catch (error) {
    console.error('Failed to fetch dashboard stats:', error);
    return undefined;
  }
}

export default async function DashboardPage() {
  const initialStats = await getDashboardData();

  return (
    <Suspense fallback={
      <div className="min-h-screen bg-gradient-to-br from-black via-zinc-900 to-black flex items-center justify-center">
        <div className="h-16 w-16 border-4 border-pink-500 border-t-transparent rounded-full animate-spin" />
      </div>
    }>
      <RevolutionaryDashboard initialStats={initialStats} />
    </Suspense>
  );
}