import { clsx, type ClassValue } from "clsx"
import { twMerge } from "tailwind-merge"

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

/**
 * Creates a static timestamp for hydration-safe fallback data
 * Use this instead of new Date().toISOString() for server-side fallbacks
 */
export function getStaticTimestamp(offsetMinutes: number = 0): string {
  // Use a fixed base timestamp for consistent server/client renders
  const baseTime = new Date('2024-01-01T12:00:00.000Z').getTime();
  const adjustedTime = baseTime + (offsetMinutes * 60 * 1000);
  return new Date(adjustedTime).toISOString();
}

/**
 * Formats duration between two timestamps
 * Safe for SSR by avoiding Date.now() unless explicitly client-side
 */
export function formatDuration(start: string, end?: string | null, useCurrentTime: boolean = false): string {
  const startTime = new Date(start).getTime();
  let endTime: number;
  
  if (end) {
    endTime = new Date(end).getTime();
  } else if (useCurrentTime) {
    // Only use current time when explicitly requested (client-side)
    endTime = Date.now();
  } else {
    // Fallback to start time for consistent SSR
    endTime = startTime;
  }
  
  const duration = endTime - startTime;
  const minutes = Math.floor(duration / 60000);
  const seconds = Math.floor((duration % 60000) / 1000);
  
  if (minutes > 0) {
    return `${minutes}m ${seconds}s`;
  }
  return `${seconds}s`;
}
