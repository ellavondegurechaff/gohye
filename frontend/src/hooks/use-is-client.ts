import { useState, useEffect } from "react";

/**
 * Hook to detect if code is running on the client side
 * Prevents hydration mismatches by ensuring server and client render the same initially
 */
export function useIsClient() {
  const [isClient, setIsClient] = useState(false);

  useEffect(() => {
    setIsClient(true);
  }, []);

  return isClient;
}