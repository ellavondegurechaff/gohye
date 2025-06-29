"use client";

import { useState } from "react";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { DiscordLogoIcon } from "@radix-ui/react-icons";
import { toast } from "sonner";

interface LoginFormProps {
  error?: string;
  message?: string;
}

const errorMessages = {
  invalid_state: "Authentication failed due to security validation. Please try again.",
  state_mismatch: "Authentication failed due to security validation. Please try again.",
  oauth_error: "Discord authentication failed. Please try again.",
  missing_code: "Authentication failed. Please try again.",
  token_exchange_failed: "Failed to complete authentication. Please try again.",
  user_info_failed: "Failed to retrieve user information. Please try again.",
  session_creation_failed: "Failed to create user session. Please try again.",
  insufficient_permissions: "You don't have admin privileges. Contact an administrator.",
  session_cookie_failed: "Failed to save authentication. Please try again.",
};

const successMessages = {
  logged_out: "You have been logged out successfully.",
};

export function LoginForm({ error, message }: LoginFormProps) {
  const [isLoading, setIsLoading] = useState(false);

  const handleDiscordLogin = async () => {
    setIsLoading(true);
    try {
      // Redirect to Go backend Discord OAuth endpoint
      window.location.href = "http://localhost:8080/auth/discord";
    } catch (err) {
      toast.error("Failed to initiate Discord login");
      setIsLoading(false);
    }
  };

  // Show error message if present
  if (error && error in errorMessages) {
    toast.error(errorMessages[error as keyof typeof errorMessages]);
  }

  // Show success message if present
  if (message && message in successMessages) {
    toast.success(successMessages[message as keyof typeof successMessages]);
  }

  return (
    <Card className="w-[400px] bg-zinc-900 border-zinc-800">
      <CardHeader className="text-center">
        <CardTitle className="text-2xl font-semibold text-white">GoHYE Admin</CardTitle>
        <CardDescription className="text-zinc-400">
          Sign in with Discord to access the admin panel
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <Button 
          onClick={handleDiscordLogin}
          disabled={isLoading}
          className="w-full bg-[#5865F2] hover:bg-[#4752C4] text-white"
        >
          <DiscordLogoIcon className="mr-2 h-4 w-4" />
          {isLoading ? "Connecting..." : "Continue with Discord"}
        </Button>
        
        {error && (
          <div className="text-center text-sm text-red-400 bg-red-950/50 p-3 rounded border border-red-800">
            {errorMessages[error as keyof typeof errorMessages] || "An unknown error occurred"}
          </div>
        )}
        
        <div className="text-center text-xs text-zinc-500">
          Only authorized administrators can access this panel
        </div>
      </CardContent>
    </Card>
  );
}