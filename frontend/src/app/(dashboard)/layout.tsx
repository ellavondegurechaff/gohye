import { redirect } from "next/navigation";
import { getServerSession } from "@/lib/auth";
import { DashboardHeader } from "@/components/layout/dashboard-header";
import { DashboardSidebar } from "@/components/layout/dashboard-sidebar";

export default async function DashboardLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const session = await getServerSession();
  
  if (!session) {
    redirect("/login");
  }

  return (
    <div className="min-h-screen bg-black">
      <DashboardHeader user={session} />
      <DashboardSidebar />
      <main className="md:ml-64">
        {children}
      </main>
    </div>
  );
}