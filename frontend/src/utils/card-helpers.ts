import { CARD_COLORS } from "@/constants/cards";
import type { CardDTO } from "@/lib/types";

/**
 * Get the CSS classes for a card level badge
 */
export function getLevelBadgeClasses(level: number): string {
  const levelKey = `LEVEL_${level}` as keyof typeof CARD_COLORS;
  return CARD_COLORS[levelKey] || CARD_COLORS.LEVEL_1;
}

/**
 * Format card count for display
 */
export function formatCardCount(count: number): string {
  return count.toLocaleString();
}

/**
 * Generate search params from filter values
 */
export function buildSearchParams(filters: {
  search?: string;
  collection?: string; 
  level?: string;
  type?: string;
  page?: number;
  limit?: number;
}) {
  const params: Record<string, string | number> = {};
  
  if (filters.search) params.search = filters.search;
  if (filters.collection) params.collection = filters.collection;
  if (filters.level) params.level = parseInt(filters.level);
  if (filters.type === "animated") params.animated = "true";
  if (filters.type === "normal") params.animated = "false";
  if (filters.page) params.page = filters.page;
  if (filters.limit) params.limit = filters.limit;
  
  return params;
}

/**
 * Check if a card matches search criteria
 */
export function cardMatchesSearch(card: CardDTO, searchTerm: string): boolean {
  if (!searchTerm) return true;
  
  const term = searchTerm.toLowerCase();
  return (
    card.name.toLowerCase().includes(term) ||
    card.collection_name.toLowerCase().includes(term) ||
    (card.tags && card.tags.some(tag => tag.toLowerCase().includes(term)))
  );
}