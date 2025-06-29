import { redirect } from "next/navigation";
import { getServerSession } from "@/lib/auth";
import { LoginForm } from "@/components/auth/login-form";

export default async function LoginPage({
  searchParams,
}: {
  searchParams: Promise<{ error?: string; message?: string }>;
}) {
  // Check if user is already authenticated
  const session = await getServerSession();
  if (session) {
    redirect("/dashboard");
  }

  const params = await searchParams;

  return (
    <div className="min-h-screen bg-black flex items-center justify-center p-4">
      <LoginForm error={params.error} message={params.message} />
    </div>
  );
}